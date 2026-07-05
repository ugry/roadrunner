// Insucar backend — roadside assistance platform.
// Auth: app-level sessions (HMAC cookie). Users log in by email; agents by agent_id.
// Telephony: MOCK Amazon Connect. Providers: REAL HTTP connector. SMS: REAL AWS SNS.
// Multi-tenant: host-based resolution + RLS. Status tracking: live ETA + Leaflet map.
package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	db          *pgxpool.Pool
	snsClient   *sns.Client
	statusBase  = getenv("STATUS_LINK_BASE", "https://app.unysolar.com/status")
	providerURL = os.Getenv("PROVIDER_API_URL")
	sessionKey  = []byte(getenv("SESSION_SECRET", "insucar-demo-session-key-change-me"))
)

const opsPath = "/ops-console-7f3a9c"

func main() {
	dsn := getenv("DATABASE_URL", "postgres://postgres:test@db:5432/insucar?sslmode=disable")
	var err error
	for i := 0; i < 30; i++ {
		db, err = pgxpool.New(context.Background(), dsn)
		if err == nil && db.Ping(context.Background()) == nil {
			break
		}
		log.Printf("waiting for db (%d)...", i)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	log.Println("connected to database")
	loadRateLimits()
	if cfg, e := awscfg.LoadDefaultConfig(context.Background()); e == nil {
		snsClient = sns.NewFromConfig(cfg)
		initEvents(cfg)
	}
	initRedis()
	initTenants(context.Background())
	initPinpoint()
	cognito = newCognitoVerifier()
	if cognito != nil {
		if err := cognito.refresh(context.Background()); err != nil {
			log.Printf("cognito jwks prefetch failed (will retry lazily): %v", err)
		}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealth)

	// auth
	mux.HandleFunc("/api/user/login", handleUserLogin)
	mux.HandleFunc("/api/agent/login", handleAgentLogin)
	mux.HandleFunc("/api/logout", handleLogout)
	mux.HandleFunc("/api/me", handleMe)

	// public
	mux.HandleFunc("/api/register", handleRegister)
	mux.HandleFunc("/api/upload/photo", handlePhotoUpload)
	mux.HandleFunc("/api/photo/", handlePhotoServe)
	mux.HandleFunc("/api/telephony/mock/incoming", handleMockIncoming)
	mux.HandleFunc("/api/status/", handleStatusPage)
	mux.HandleFunc("/api/telephony/mock/psap", handleMockPsap)
	mux.HandleFunc("/api/telephony/mock/call-state", handleMockCallState)
	mux.HandleFunc("/api/webhook/provider", handleProviderWebhook)
	mux.HandleFunc("/api/events", handleSSE)
	mux.HandleFunc("/api/connect/events", handleConnectEvent)
	mux.HandleFunc("/api/connect/lex", handleLexTriage)
	mux.HandleFunc("/api/connect/psap", handlePsapTransfer)
	mux.HandleFunc("/api/pinpoint/callback", handlePinpointCallback)
	mux.HandleFunc("/api/push/send", requireRole("agent", handlePushSend))

	// user (customer) — requires user session
	mux.HandleFunc("/api/user/incident", requireRole("user", handleUserIncident))
	mux.HandleFunc("/api/user/cases", requireRole("user", handleUserCases))

	// agent (operator) — requires staff session
	mux.HandleFunc("/api/agent/cases", requireRole("agent", handleAgentCases))
	mux.HandleFunc("/api/agent/case", requireRole("agent", handleAgentCase))
	mux.HandleFunc("/api/agent/lookup", requireRole("agent", handleLookup))
	mux.HandleFunc("/api/agent/dispatch", requireRole("agent", handleDispatch))
	mux.HandleFunc("/api/agent/providers", requireRole("agent", handleAgentProviders))
	mux.HandleFunc("/api/agent/stats", requireRole("agent", handleAgentStats))
	mux.HandleFunc("/api/agent/sms-journey", requireRole("agent", handleSmsJourney))
	mux.HandleFunc("/api/case/rate", requireRole("user", handleCaseRate))
	mux.HandleFunc("/api/case/arrived", requireRole("user", handleCaseArrived))

	// admin — requires staff session with admin/supervisor/product_owner role
	mux.HandleFunc("/api/admin/rate-limits", requireAdmin(handleAdminRateLimits))
	mux.HandleFunc("/api/admin/api-access", requireAdmin(handleAdminApiAccess))
	mux.HandleFunc("/api/admin/operators", requireAdmin(handleAdminOperators))
	mux.HandleFunc("/api/admin/stats", requireAdmin(handleAdminStats))
	mux.HandleFunc("/api/agent/predict-eta", requireRole("agent", handlePredictEta))
	mux.HandleFunc("/api/agent/safety-triage", requireRole("agent", handleSafetyTriage))

	// auth config (Cognito setup exposed to frontends)
	mux.HandleFunc("/api/auth/config", handleAuthConfig)

	// pages (host-based: op.* -> operators only, apex -> users only)
	mux.HandleFunc("/", handleRoot)
	mux.HandleFunc(opsPath, serveFile("operator.html"))
	mux.HandleFunc("/admin-8f2a4d", serveFile("admin.html"))
	mux.HandleFunc("/register-page", serveFile("register.html"))

	startProviderHealthLoop(context.Background())

	log.Printf("listening on :8080 (ops path %s; provider=%q; sms=%v; events=%v; redis=%v; cognito=%v; tenant=%s)",
		opsPath, providerURL, snsClient != nil, ebClient != nil && eventBus != "", rdb != nil, cognito != nil, defaultTenantID[:min(8, len(defaultTenantID))])
	log.Fatal(http.ListenAndServe(":8080", tenantMiddleware(logMW(mux))))
}

// ---------- infra helpers ----------
func logMW(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf(`{"stream":"system","method":"%s","path":"%s"}`, r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
}
// custom404 returns JSON for API routes, HTML for web routes (#43 / E7).
func custom404(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(404)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(404)
	w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>404 — Insucar</title><style>body{font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;background:#f4f8f6;color:#0b1f2a}div{text-align:center}h1{font-size:3rem;color:#0a7d5a}a{color:#0a7d5a}</style></head><body><div><h1>404</h1><p>Page not found</p><a href="/">Home</a> · <a href="/app">Sign in</a> · <a href="tel:+33800000000">Call us</a></div></body></html>`))
}

func handleRoot(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if i := strings.IndexByte(host, ':'); i >= 0 { host = host[:i] }
	if strings.HasPrefix(host, "op.") {
		if r.URL.Path == "/callback" { http.ServeFile(w, r, "/app/web/cognito-callback.html"); return }
		if strings.HasPrefix(r.URL.Path, "/api/") { return } // let mux handle API
		http.ServeFile(w, r, "/app/web/operator.html")
		return
	}
	switch r.URL.Path {
	case "/":
		http.ServeFile(w, r, "/app/web/landing.html")
	case "/app", "/login", "/register":
		http.ServeFile(w, r, "/app/web/enduser.html")
	case "/app/callback":
		http.ServeFile(w, r, "/app/web/cognito-callback.html")
	default:
		if strings.HasPrefix(r.URL.Path, "/api/") { return } // let mux handle
		custom404(w, r)
	}
}

func serveFile(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if name == "enduser.html" && r.URL.Path != "/" && r.URL.Path != "/login" && r.URL.Path != "/register" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "/app/web/"+name)
	}
}
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
func sha256hex(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// ---------- sessions (HMAC-signed cookie: role|id|name|exp|sig) ----------
func makeToken(role, id, name string) string {
	exp := time.Now().Add(8 * time.Hour).Unix()
	body := fmt.Sprintf("%s|%s|%s|%d", role, id, name, exp)
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(body))
	return body + "|" + hex.EncodeToString(mac.Sum(nil))
}
func parseToken(tok string) (role, id, name string, ok bool) {
	parts := strings.Split(tok, "|")
	if len(parts) != 5 {
		return
	}
	body := strings.Join(parts[:4], "|")
	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(body))
	if !hmac.Equal([]byte(parts[4]), []byte(hex.EncodeToString(mac.Sum(nil)))) {
		return
	}
	var exp int64
	fmt.Sscanf(parts[3], "%d", &exp)
	if time.Now().Unix() > exp {
		return
	}
	return parts[0], parts[1], parts[2], true
}
func setSession(w http.ResponseWriter, role, id, name string) {
	http.SetCookie(w, &http.Cookie{Name: "insucar_session", Value: makeToken(role, id, name),
		Path: "/", HttpOnly: true, MaxAge: 8 * 3600, SameSite: http.SameSiteLaxMode})
}
func currentSession(r *http.Request) (role, id, name string, ok bool) {
	c, err := r.Cookie("insucar_session")
	if err != nil {
		return
	}
	return parseToken(c.Value)
}
func requireRole(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Identity from a Cognito Bearer token (managed auth) or the demo cookie.
		c, ok := resolveCaller(r)
		if !ok {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		if c.Role != role {
			writeJSON(w, 403, map[string]string{"error": "forbidden"})
			return
		}
		next(w, withCaller(r, c))
	}
}

func handleAuthConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]any{
		"cognito":          cognito != nil,
		"region":           getenv("AWS_REGION", "eu-west-1"),
		"customerDomain":   getenv("COGNITO_CUSTOMER_DOMAIN", ""),
		"customerClientId": getenv("COGNITO_CUSTOMER_CLIENT_ID", ""),
		"staffDomain":      getenv("COGNITO_STAFF_DOMAIN", ""),
		"staffClientId":    getenv("COGNITO_STAFF_CLIENT_ID", ""),
	})
}

// ---------- handlers ----------
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if db.Ping(r.Context()) != nil {
		writeJSON(w, 503, map[string]string{"status": "db_down"})
		return
	}
	if !redisHealthy(r.Context()) {
		writeJSON(w, 503, map[string]string{"status": "redis_down"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func handleUserLogin(w http.ResponseWriter, r *http.Request) {
	var in struct{ Email, Password string }
	json.NewDecoder(r.Body).Decode(&in)
	var id, first, last string
	var hash *string
	err := db.QueryRow(r.Context(),
		`SELECT id,first_name,last_name,password_hash FROM customers WHERE email=$1`, in.Email).
		Scan(&id, &first, &last, &hash)
	if err != nil || hash == nil || *hash != sha256hex(in.Password) {
		writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
		return
	}
	setSession(w, "user", id, first+" "+last)
	db.Exec(r.Context(), `INSERT INTO audit_ledger(event_type,actor,payload) VALUES('auth.user.login',$1,'{}')`, in.Email)
	writeJSON(w, 200, map[string]any{"role": "user", "id": id, "name": first + " " + last})
}

func handleAgentLogin(w http.ResponseWriter, r *http.Request) {
	var in struct{ AgentID, Password string }
	json.NewDecoder(r.Body).Decode(&in)
	var id, name, role string
	var hash *string
	err := db.QueryRow(r.Context(),
		`SELECT id,display_name,role::text,password_hash FROM staff WHERE agent_id=$1 AND active`, in.AgentID).
		Scan(&id, &name, &role, &hash)
	if err != nil || hash == nil || *hash != sha256hex(in.Password) {
		writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
		return
	}
	setSession(w, "agent", id, name)
	db.Exec(r.Context(), `INSERT INTO audit_ledger(event_type,actor,payload) VALUES('auth.agent.login',$1,$2)`,
		in.AgentID, fmt.Sprintf(`{"role":"%s"}`, role))
	writeJSON(w, 200, map[string]any{"role": "agent", "id": id, "name": name, "staff_role": role, "agent_id": in.AgentID})
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{Name: "insucar_session", Value: "", Path: "/", MaxAge: -1})
	writeJSON(w, 200, map[string]string{"status": "logged_out"})
}

func handleMe(w http.ResponseWriter, r *http.Request) {
	c, ok := resolveCaller(r)
	if !ok {
		writeJSON(w, 200, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, 200, map[string]any{"authenticated": true, "role": c.Role, "id": c.ID, "name": c.Name})
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email, Phone, First, Last, Language, Country, Password string
		Consents                                              []string
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	// E1 + E8: Server-side validation
	first := strings.TrimSpace(in.First)
	last := strings.TrimSpace(in.Last)
	email := strings.TrimSpace(in.Email)
	phone := strings.TrimSpace(in.Phone)
	password := in.Password

	var errs []string
	if first == "" { errs = append(errs, "first name required") }
	if last == "" { errs = append(errs, "last name required") }
	if len(first) > 100 { errs = append(errs, "first name too long (max 100)") }
	if email == "" || !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		errs = append(errs, "valid email required")
	}
	if phone == "" || !strings.HasPrefix(phone, "+") {
		errs = append(errs, "phone required (E.164 format, e.g. +33600000009)")
	}
	if len(password) < 8 {
		errs = append(errs, "password must be at least 8 characters")
	}
	// E2: GDPR consent enforcement
	hasTerms := false
	for _, c := range in.Consents {
		if c == "terms" { hasTerms = true; break }
	}
	if !hasTerms {
		errs = append(errs, "consent to terms of service required (GDPR Art.7)")
	}
	if len(errs) > 0 {
		writeJSON(w, 400, map[string]any{"error": "validation failed", "fields": errs})
		return
	}

	var id string
	err := db.QueryRow(r.Context(),
		`INSERT INTO customers(email,phone_e164,first_name,last_name,preferred_language,country_code,status,password_hash)
		 VALUES($1,$2,$3,$4,$5,$6,'active',$7) RETURNING id`,
		email, phone, first, last, nz(in.Language, "en"), nz(in.Country, "FR"), sha256hex(password)).Scan(&id)
	if err != nil {
		writeJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	for _, c := range in.Consents {
		db.Exec(r.Context(), `INSERT INTO consents(customer_id,purpose,granted,basis) VALUES($1,$2,true,'consent')`, id, c)
	}
	db.Exec(r.Context(), `INSERT INTO audit_ledger(event_type,actor,payload) VALUES('customer.register',$1,$2)`,
		in.Email, fmt.Sprintf(`{"customer_id":"%s"}`, id))
	setSession(w, "user", id, in.First+" "+in.Last)
	publishEvent(r.Context(), "customer.registered", map[string]any{"customer_id": id, "email": in.Email})
	writeJSON(w, 201, map[string]string{"customer_id": id, "status": "active"})
}

// USER: submit an incident (creates a case for the logged-in customer)
func handleUserIncident(w http.ResponseWriter, r *http.Request) {
	uid := ""
	if c, ok := callerFrom(r); ok {
		uid = c.ID
	}
	var in struct{ Incident, Description, Address, Lat, Lng, PhotoID string }
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	// E3: Validate required fields
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		writeJSON(w, 400, map[string]string{"error": "description required"})
		return
	}
	caseNo := fmt.Sprintf("CASE-%d", time.Now().Unix())
	var cid string
	err := db.QueryRow(r.Context(),
		`INSERT INTO cases(case_number,customer_id,channel,status,priority,incident,incident_at,symptom_description)
		 VALUES($1,$2,'app','triaging','high',$3,now(),$4) RETURNING id`,
		caseNo, uid, nz(in.Incident, "breakdown"), desc).Scan(&cid)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "failed to create case: " + err.Error()})
		return
	}
	if in.Address != "" || in.Lat != "" {
		db.Exec(r.Context(), `INSERT INTO case_locations(case_id,address_text,capture_method) VALUES($1,$2,'app')`, cid, in.Address)
	}
	note := "customer submitted via app: " + in.Description
	if in.PhotoID != "" {
		note += " [photo: /api/photo/" + in.PhotoID + "]"
	}
	db.Exec(r.Context(), `INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'note',$2)`, cid, note)
	publishEvent(r.Context(), "case.created", map[string]any{"case_id": cid, "case_number": caseNo, "incident": nz(in.Incident, "breakdown"), "customer_id": uid})
	publishSSE("case.created", map[string]any{"case_id": cid, "case_number": caseNo, "incident": nz(in.Incident, "breakdown")})
	writeJSON(w, 201, map[string]string{"case_id": cid, "case_number": caseNo, "status": "triaging"})
}

// USER: list own cases
func handleUserCases(w http.ResponseWriter, r *http.Request) {
	uid := ""
	if c, ok := callerFrom(r); ok {
		uid = c.ID
	}
	rows, _ := db.Query(r.Context(),
		`SELECT id, case_number,status::text,incident::text,coalesce(ca.symptom_description,''),ca.created_at,
		        coalesce(ca.satisfaction_score,0), coalesce(ca.resolution_notes,''),
		        coalesce(m.eta_minutes,0)
		 FROM cases ca
		 LEFT JOIN missions m ON m.case_id = ca.id AND m.status NOT IN ('failed','cancelled')
		 WHERE ca.customer_id=$1 ORDER BY ca.created_at DESC`, uid)
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, no, st, inc, desc, notes string
		var score, etaMin int
		var ts time.Time
		rows.Scan(&id, &no, &st, &inc, &desc, &ts, &score, &notes, &etaMin)
		out = append(out, map[string]any{"id": id, "case_number": no, "status": st, "incident": inc,
			"description": desc, "created_at": ts, "score": score, "notes": notes, "eta_minutes": etaMin})
	}
	writeJSON(w, 200, map[string]any{"cases": out})
}

// AGENT: list all active cases (queue)
func handleAgentCases(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(r.Context(), `
		SELECT ca.id, ca.case_number, ca.status::text, ca.priority::text, ca.incident::text,
		       coalesce(c.first_name||' '||c.last_name,'(unknown)'), coalesce(c.phone_e164,''), ca.created_at
		FROM cases ca LEFT JOIN customers c ON c.id=ca.customer_id
		WHERE ca.status NOT IN ('closed','resolved','cancelled')
		ORDER BY (ca.priority='emergency') DESC, ca.created_at DESC LIMIT 100`)
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, no, st, pr, inc, who, phone string
		var ts time.Time
		rows.Scan(&id, &no, &st, &pr, &inc, &who, &phone, &ts)
		out = append(out, map[string]any{"id": id, "case_number": no, "status": st, "priority": pr,
			"incident": inc, "customer": who, "phone": phone, "created_at": ts})
	}
	writeJSON(w, 200, map[string]any{"cases": out})
}

// AGENT: case detail (customer + vehicle + latest mission + coverage + timeline)
func handleAgentCase(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	var no, st, pr, inc, desc, who, phone string
	var createdAt time.Time
	err := db.QueryRow(r.Context(), `
		SELECT ca.case_number, ca.status::text, ca.priority::text, ca.incident::text, coalesce(ca.symptom_description,''),
		       coalesce(c.first_name||' '||c.last_name,'(unknown)'), coalesce(c.phone_e164,''), ca.created_at
		FROM cases ca LEFT JOIN customers c ON c.id=ca.customer_id WHERE ca.id=$1`, id).
		Scan(&no, &st, &pr, &inc, &desc, &who, &phone, &createdAt)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	var plate, mk, model, pol, cov *string
	var excess *float64
	var calloutLimit *int
	db.QueryRow(r.Context(), `
		SELECT v.license_plate,v.make,v.model,p.policy_number,p.coverage::text,p.excess_amount,p.callout_limit
		FROM cases ca JOIN customers c ON c.id=ca.customer_id
		LEFT JOIN policies p ON p.customer_id=c.id
		LEFT JOIN policy_vehicles pv ON pv.policy_id=p.id
		LEFT JOIN vehicles v ON v.id=pv.vehicle_id WHERE ca.id=$1 LIMIT 1`, id).
		Scan(&plate, &mk, &model, &pol, &cov, &excess, &calloutLimit)
	var eta *int
	var prov *string
	var mid *string
	db.QueryRow(r.Context(), `SELECT m.id,m.eta_minutes,p.display_name FROM missions m JOIN providers p ON p.id=m.provider_id
		WHERE m.case_id=$1 ORDER BY m.created_at DESC LIMIT 1`, id).Scan(&mid, &eta, &prov)
	// driver info for active mission
	var drvName, drvPlate *string
	if mid != nil && *mid != "" {
		db.QueryRow(r.Context(),
			`SELECT driver_name, vehicle_plate FROM mission_driver WHERE mission_id=$1 LIMIT 1`, *mid).
			Scan(&drvName, &drvPlate)
	}
	// mission timeline events
	var timeline []map[string]any
	if mid != nil && *mid != "" {
		trows, _ := db.Query(r.Context(),
			`SELECT status::text, eta_minutes, occurred_at, coalesce(raw_status,status::text)
			 FROM mission_status_events WHERE mission_id=$1 ORDER BY occurred_at ASC`, *mid)
		defer trows.Close()
		for trows.Next() {
			var tStatus, rawStatus string
			var tEta *int
			var tOccurred time.Time
			trows.Scan(&tStatus, &tEta, &tOccurred, &rawStatus)
			timeline = append(timeline, map[string]any{
				"status": tStatus, "eta_minutes": tEta,
				"occurred_at": tOccurred, "label": rawStatus,
			})
		}
	}
	// case safety info
	var safe, inTraffic, onShoulder, vulnerable, isDark *bool
	var weather *string
	db.QueryRow(r.Context(),
		`SELECT is_everyone_safe,in_live_traffic,on_hard_shoulder,vulnerable_occupants,is_dark,weather::text
		 FROM case_safety WHERE case_id=$1`, id).
		Scan(&safe, &inTraffic, &onShoulder, &vulnerable, &isDark, &weather)

	writeJSON(w, 200, map[string]any{
		"case_number": no, "status": st, "priority": pr, "incident": inc, "description": desc,
		"customer": who, "phone": phone, "created_at": createdAt,
		"policy": map[string]any{
			"number": s(pol), "coverage": s(cov),
			"excess": excess, "callout_limit": calloutLimit,
		},
		"vehicle": map[string]any{"plate": s(plate), "make": s(mk), "model": s(model)},
		"mission": map[string]any{"id": s(mid), "provider": s(prov), "eta_minutes": eta, "timeline": timeline, "driver": map[string]any{"name": s(drvName), "plate": s(drvPlate)}},
		"safety": map[string]any{
			"everyone_safe": safe, "in_live_traffic": inTraffic,
			"on_hard_shoulder": onShoulder, "vulnerable_occupants": vulnerable,
			"is_dark": isDark, "weather": weather,
		},
	})
}

func lookupByPhone(ctx context.Context, phone string) (map[string]any, string, string, bool) {
	// Serve the screen-pop from ElastiCache when warm (short TTL).
	cacheKey := "pop:" + phone
	if b, ok := cacheGet(ctx, cacheKey); ok {
		var m map[string]any
		if json.Unmarshal(b, &m) == nil {
			cust, _ := m["customer"].(map[string]any)
			cid, _ := cust["id"].(string)
			lang, _ := cust["language"].(string)
			return m, cid, lang, true
		}
	}
	row := db.QueryRow(ctx, `
		SELECT c.id,c.first_name,c.last_name,c.preferred_language,c.country_code,
		       p.policy_number,p.coverage::text,p.status::text,v.license_plate,v.make,v.model,v.fuel::text
		FROM customers c
		LEFT JOIN policies p ON p.customer_id=c.id
		LEFT JOIN policy_vehicles pv ON pv.policy_id=p.id
		LEFT JOIN vehicles v ON v.id=pv.vehicle_id WHERE c.phone_e164=$1 LIMIT 1`, phone)
	var cid, fn, ln, lang, country, pol, cov, pstat, plate, mk, model, fuel *string
	if row.Scan(&cid, &fn, &ln, &lang, &country, &pol, &cov, &pstat, &plate, &mk, &model, &fuel) != nil {
		return nil, "", "", false
	}
	m := map[string]any{
		"customer": map[string]any{"id": s(cid), "first_name": s(fn), "last_name": s(ln), "language": s(lang), "country": s(country)},
		"policy":   map[string]any{"policy_number": s(pol), "coverage": s(cov), "status": s(pstat)},
		"vehicle":  map[string]any{"plate": s(plate), "make": s(mk), "model": s(model), "fuel": s(fuel)},
	}
	if b, err := json.Marshal(m); err == nil {
		cacheSet(ctx, cacheKey, b, 60*time.Second)
	}
	return m, s(cid), s(lang), true
}

func handleLookup(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	m, _, _, ok := lookupByPhone(r.Context(), phone)
	if !ok {
		writeJSON(w, 404, map[string]any{"matched": false, "phone": phone})
		return
	}
	m["matched"] = true
	m["phone"] = phone
	writeJSON(w, 200, m)
}

// handlePhotoUpload accepts multipart photo uploads, saves to disk, returns photo_id.
func handlePhotoUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, 400, map[string]string{"error": "file too large (max 10 MB)"})
		return
	}
	file, header, err := r.FormFile("photo")
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "no photo file provided"})
		return
	}
	defer file.Close()

	photoID := fmt.Sprintf("photo-%d-%s", time.Now().UnixNano(),
		strings.ToLower(strings.ReplaceAll(header.Filename, " ", "-")))
	os.MkdirAll("/tmp/photos", 0755)
	dst, _ := os.Create("/tmp/photos/" + photoID)
	if dst != nil {
		defer dst.Close()
		io.Copy(dst, file)
	}
	writeJSON(w, 201, map[string]any{
		"photo_id": photoID, "filename": header.Filename,
		"size_bytes": header.Size, "url": "/api/photo/" + photoID,
	})
}

// handlePhotoServe serves uploaded photos by ID.
func handlePhotoServe(w http.ResponseWriter, r *http.Request) {
	photoID := strings.TrimPrefix(r.URL.Path, "/api/photo/")
	if photoID == "" || strings.Contains(photoID, "..") {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, "/tmp/photos/"+photoID)
}

func handleMockIncoming(w http.ResponseWriter, r *http.Request) {
	var in struct{ Phone string }
	json.NewDecoder(r.Body).Decode(&in)
	cidc := fmt.Sprintf("mock-%d", time.Now().UnixNano())
	m, _, lang, ok := lookupByPhone(r.Context(), in.Phone)
	pop := map[string]any{"connect_contact_id": cidc, "ani": in.Phone, "dnis": "+33800000000",
		"language": lang, "matched": ok, "priority": "high", "queue": "mobility"}
	if ok {
		pop["screen_pop"] = m
	}
	writeJSON(w, 200, pop)
}

// Wire safety triage to database + auto-escalate priority (#31)
func handleSafetyTriage(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CaseID          string `json:"case_id"`
		EveryoneSafe    bool   `json:"everyone_safe"`
		InLiveTraffic   bool   `json:"in_live_traffic"`
		OnHardShoulder  bool   `json:"on_hard_shoulder"`
		Vulnerable      bool   `json:"vulnerable_occupants"`
		IsDark          bool   `json:"is_dark"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	db.Exec(r.Context(), `INSERT INTO case_safety(case_id,is_everyone_safe,in_live_traffic,on_hard_shoulder,vulnerable_occupants,is_dark)
		VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(case_id) DO UPDATE SET is_everyone_safe=$2,in_live_traffic=$3,on_hard_shoulder=$4,vulnerable_occupants=$5,is_dark=$6`,
		in.CaseID, in.EveryoneSafe, in.InLiveTraffic, in.OnHardShoulder, in.Vulnerable, in.IsDark)
	// Auto-escalate: any safety concern → priority=emergency
	if !in.EveryoneSafe || in.InLiveTraffic || in.Vulnerable {
		db.Exec(r.Context(), `UPDATE cases SET priority='emergency' WHERE id=$1`, in.CaseID)
		publishSSE("case.updated", map[string]any{"case_id": in.CaseID, "priority": "emergency"})
	}
	writeJSON(w, 200, map[string]any{"case_id": in.CaseID, "priority_escalated": !in.EveryoneSafe || in.InLiveTraffic || in.Vulnerable})
}

// handleMockPsap performs a simulated PSAP (112) warm-transfer with audit trail.
func handleMockPsap(w http.ResponseWriter, r *http.Request) {
	var in struct {
		CaseID string `json:"case_id"`
		Reason string `json:"reason"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	if in.CaseID == "" {
		writeJSON(w, 400, map[string]string{"error": "case_id required"})
		return
	}
	// Audit the PSAP transfer
	db.Exec(r.Context(),
		`INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'psap_transfer',$2)`,
		in.CaseID, fmt.Sprintf("PSAP 112 warm-transfer initiated. Reason: %s", nz(in.Reason, "emergency")))
	db.Exec(r.Context(),
		`INSERT INTO case_safety(case_id,emergency_services_called,emergency_reference)
		 VALUES($1,true,$2) ON CONFLICT(case_id) DO UPDATE SET emergency_services_called=true`,
		in.CaseID, fmt.Sprintf("PSAP-%d", time.Now().Unix()))
	// Update case priority to emergency
	db.Exec(r.Context(), `UPDATE cases SET priority='emergency' WHERE id=$1 AND priority!='emergency'`, in.CaseID)

	writeJSON(w, 200, map[string]any{
		"status":         "transferred",
		"case_id":        in.CaseID,
		"reference":      fmt.Sprintf("PSAP-%d", time.Now().Unix()),
		"timestamp":      time.Now(),
		"instructions":   "Stay on the line. Operator will hand over key details to 112 dispatcher.",
	})
}

// mockCallState tracks simulated call lifecycle state.
var mockCallStates = map[string]map[string]any{} // phone -> call state
var mockCallMu sync.RWMutex

// handleMockCallState manages the mock call lifecycle: ringing → answered → connected → wrap-up → ended.
func handleMockCallState(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Return current call state
		phone := r.URL.Query().Get("phone")
		mockCallMu.RLock()
		state, ok := mockCallStates[phone]
		mockCallMu.RUnlock()
		if !ok {
			writeJSON(w, 200, map[string]any{"phone": phone, "call_state": "idle"})
			return
		}
		// Calculate live duration
		if started, ok2 := state["started_at"].(time.Time); ok2 {
			state["duration"] = time.Since(started).Truncate(time.Second).String()
		}
		writeJSON(w, 200, state)
	case "POST":
		// Advance call state
		var in struct {
			Phone string `json:"phone"`
			State string `json:"state"` // ringing, answered, connected, wrapup, ended
		}
		json.NewDecoder(r.Body).Decode(&in)
		valid := map[string]bool{
			"ringing": true, "answered": true, "connected": true, "wrapup": true, "ended": true,
		}
		if !valid[in.State] {
			writeJSON(w, 400, map[string]string{"error": "invalid state: use ringing/answered/connected/wrapup/ended"})
			return
		}
		mockCallMu.Lock()
		mockCallStates[in.Phone] = map[string]any{
			"phone":      in.Phone,
			"call_state": in.State,
			"started_at": time.Now(),
		}
		mockCallMu.Unlock()

		// Log call state change to interaction log if case context available
		cid := r.URL.Query().Get("case_id")
		if cid != "" {
			db.Exec(r.Context(),
				`INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'call',$2)`,
				cid, fmt.Sprintf("Call state: %s (ANI: %s)", in.State, in.Phone))
		}

		writeJSON(w, 200, map[string]any{
			"phone":      in.Phone,
			"call_state": in.State,
			"timestamp":  time.Now(),
		})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func handleDispatch(w http.ResponseWriter, r *http.Request) {
	var in struct{ CaseID, Service, ProviderID string }
	json.NewDecoder(r.Body).Decode(&in)
	svc := nz(in.Service, "tow_recovery")
	publishEvent(r.Context(), "case.dispatch.requested", map[string]any{"case_id": in.CaseID, "service": svc})

	result, err := dispatchWithFallback(r.Context(), in.CaseID, svc, in.ProviderID)
	if err != nil {
		writeJSON(w, 500, map[string]string{"error": err.Error()})
		return
	}

	mid, _ := result["mission_id"].(string)
	provName, _ := result["provider"].(string)
	eta, _ := result["eta_minutes"].(int)
	src, _ := result["provider_source"].(string)
	link, _ := result["status_link"].(string)
	sms, _ := result["sms"].(string)
	drv, _ := result["driver"].(map[string]string)

	publishEvent(r.Context(), "case.dispatched", map[string]any{
		"case_id": in.CaseID, "mission_id": mid, "provider": provName,
		"eta_minutes": eta, "source": src, "attempted": result["attempted"],
	})

	status := "en_route"
	writeJSON(w, 201, map[string]any{
		"mission_id": mid, "provider": provName, "provider_source": src,
		"eta_minutes": eta, "status": status, "status_link": link, "sms": sms,
		"driver": drv, "attempted": result["attempted"],
	})
}

// AGENT: list enabled providers with availability, capabilities, SLA
func handleAgentProviders(w http.ResponseWriter, r *http.Request) {
	rows, _ := db.Query(r.Context(), `
		SELECT p.id, p.display_name, p.priority_rank, p.performance_score,
		       pc.capabilities::text[], pc.sla_uptime, pc.base_url, pc.status::text,
		       coalesce(p.categories::text[], '{}'::text[])
		FROM providers p
		JOIN provider_connectors pc ON pc.provider_id = p.id
		WHERE p.status = 'enabled' AND pc.status = 'enabled'
		ORDER BY p.priority_rank ASC, p.performance_score DESC NULLS LAST
		LIMIT 20`)
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var id, name string
		var rank, score *int
		var caps []string
		var sla *float64
		var url, status *string
		var cats []string
		rows.Scan(&id, &name, &rank, &score, &caps, &sla, &url, &status, &cats)
		p := map[string]any{
			"id": id, "name": name, "priority_rank": rank,
			"performance_score": score, "capabilities": caps,
			"sla_uptime": sla, "categories": cats, "status": status,
		}
		// fetch availability windows for this provider
		arows, _ := db.Query(r.Context(), `SELECT day_of_week,open_time,close_time FROM provider_availability WHERE provider_id=$1 ORDER BY day_of_week`, id)
		var avail []map[string]any
		for arows.Next() {
			var dow int
			var open, close string
			arows.Scan(&dow, &open, &close)
			avail = append(avail, map[string]any{"day": dow, "open": open, "close": close})
		}
		arows.Close()
		p["availability"] = avail
		out = append(out, p)
	}
	writeJSON(w, 200, map[string]any{"providers": out})
}

// AGENT: queue & SLA statistics (elapsed times, counts)
func handleAgentStats(w http.ResponseWriter, r *http.Request) {
	// counts by status
	var waiting, active, total int
	db.QueryRow(r.Context(),
		`SELECT count(1) FROM cases WHERE status IN ('new','triaging')`).Scan(&waiting)
	db.QueryRow(r.Context(),
		`SELECT count(1) FROM cases WHERE status IN ('dispatched','en_route','on_site')`).Scan(&active)
	db.QueryRow(r.Context(),
		`SELECT count(1) FROM cases WHERE status NOT IN ('closed','resolved','cancelled')`).Scan(&total)

	// longest waiting case (in seconds)
	var longestSecs *float64
	db.QueryRow(r.Context(),
		`SELECT EXTRACT(EPOCH FROM (now()-created_at))::float
		 FROM cases WHERE status IN ('new','triaging')
		 ORDER BY created_at ASC LIMIT 1`).Scan(&longestSecs)

	// average time to dispatch (cases dispatched in last 24h, uses interaction_log as fallback)
	var avgDispatchSecs *float64
	db.QueryRow(r.Context(), `
		SELECT coalesce(avg(EXTRACT(EPOCH FROM (coalesce(ca.dispatched_at, il.occurred_at)-ca.created_at))), 0)::float
		FROM cases ca
		LEFT JOIN LATERAL (SELECT occurred_at FROM interaction_log WHERE case_id=ca.id AND event_type='dispatch' ORDER BY occurred_at ASC LIMIT 1) il ON true
		WHERE ca.status='dispatched' AND ca.created_at > now()-interval '24 hours'`).Scan(&avgDispatchSecs)

	// emergency count
	var emergency int
	db.QueryRow(r.Context(),
		`SELECT count(1) FROM cases WHERE priority='emergency' AND status NOT IN ('closed','resolved','cancelled')`).Scan(&emergency)

	// total resolved today
	var resolvedToday int
	db.QueryRow(r.Context(),
		`SELECT count(1) FROM cases WHERE status IN ('resolved','closed') AND resolved_at > current_date`).Scan(&resolvedToday)

	writeJSON(w, 200, map[string]any{
		"waiting": waiting, "active": active, "total": total,
		"longest_wait_secs": longestSecs, "avg_dispatch_secs": avgDispatchSecs,
		"emergency": emergency, "resolved_today": resolvedToday,
	})
}

// AGENT: update operator status (on-call, ACW, offline)
func handleAgentStatus(w http.ResponseWriter, r *http.Request) {
	var in struct{ Status string } // on_call, acw, offline
	json.NewDecoder(r.Body).Decode(&in)
	valid := map[string]bool{"on_call": true, "acw": true, "offline": true}
	if !valid[in.Status] {
		writeJSON(w, 400, map[string]string{"error": "invalid status: use on_call, acw, or offline"})
		return
	}
	c, _ := callerFrom(r)
	if c.ID != "" {
		db.Exec(r.Context(), `UPDATE staff SET status=$1 WHERE id=$2`, in.Status, c.ID)
	}
	writeJSON(w, 200, map[string]any{"status": in.Status, "operator_id": c.ID})
}

// SMS journey: sends progressive notifications to the customer.
// Steps: assigned, arriving, arrived, resolved, rate
var smsJourneySteps = []struct{ Key, Template string }{
	{"assigned", "Insucar: %s assigned — %s (%s), ETA ~%d min. Track: %s"},
	{"arriving", "Insucar: %s is arriving in ~5 minutes. %s (%s)"},
	{"arrived", "Insucar: %s has arrived at your location. %s"},
	{"resolved", "Insucar: Your case %s has been resolved. Thank you for choosing Insucar."},
	{"rate", "Insucar: How was your experience? Rate us: %s"},
}

func getSmsStep(idx int) (string, string) {
	if idx >= 0 && idx < len(smsJourneySteps) {
		return smsJourneySteps[idx].Key, smsJourneySteps[idx].Template
	}
	return "", ""
}

func handleSmsJourney(w http.ResponseWriter, r *http.Request) {
	var in struct{ CaseID, Step string }
	json.NewDecoder(r.Body).Decode(&in)
	if in.CaseID == "" || in.Step == "" {
		writeJSON(w, 400, map[string]string{"error": "caseID and step required"})
		return
	}
	// Find the step index
	stepIdx := -1
	for i, s := range smsJourneySteps {
		if s.Key == in.Step { stepIdx = i; break }
	}
	if stepIdx < 0 {
		writeJSON(w, 400, map[string]string{"error": "invalid step: use assigned/arriving/arrived/resolved/rate"})
		return
	}
	// Get case + mission + driver info
	var phone, prov, drv, plate, caseNo string
	var eta int
	db.QueryRow(r.Context(), `
		SELECT c.phone_e164, p.display_name, COALESCE(md.driver_name,''), COALESCE(md.vehicle_plate,''),
		       ca.case_number, COALESCE(m.eta_minutes,0)
		FROM cases ca JOIN customers c ON c.id=ca.customer_id
		LEFT JOIN missions m ON m.case_id=ca.id
		LEFT JOIN providers p ON p.id=m.provider_id
		LEFT JOIN mission_driver md ON md.mission_id=m.id
		WHERE ca.id=$1 ORDER BY m.created_at DESC LIMIT 1`, in.CaseID).
		Scan(&phone, &prov, &drv, &plate, &caseNo, &eta)

	if phone == "" {
		writeJSON(w, 404, map[string]string{"error": "case or phone not found"})
		return
	}

	link := statusBase + "/" + caseNo
	var msg string
	switch in.Step {
	case "assigned":
		msg = fmt.Sprintf(smsJourneySteps[0].Template, prov, drv, plate, eta, link)
	case "arriving":
		msg = fmt.Sprintf(smsJourneySteps[1].Template, prov, drv, plate)
	case "arrived":
		msg = fmt.Sprintf(smsJourneySteps[2].Template, prov, drv)
	case "resolved":
		msg = fmt.Sprintf(smsJourneySteps[3].Template, caseNo)
	case "rate":
		rateLink := statusBase + "/rate/" + in.CaseID
		msg = fmt.Sprintf(smsJourneySteps[4].Template, rateLink)
	}

	if snsClient != nil {
		snsClient.Publish(r.Context(), &sns.PublishInput{PhoneNumber: &phone, Message: &msg})
	}
	db.Exec(r.Context(), `INSERT INTO notifications(case_id,channel,recipient,template,status,status_link_token)
		VALUES($1,'sms',$2,$3,'sent',$4)`, in.CaseID, phone, in.Step, link)
	db.Exec(r.Context(), `INSERT INTO interaction_log(case_id,event_type,note)
		VALUES($1,'sms_journey',$2)`, in.CaseID, "sent step: "+in.Step)

	// Update case status on resolved
	if in.Step == "resolved" {
		db.Exec(r.Context(), `UPDATE cases SET status='resolved', resolved_at=now() WHERE id=$1`, in.CaseID)
	}

	writeJSON(w, 200, map[string]any{
		"step": in.Step, "case_id": in.CaseID, "recipient": phone, "status": "sent",
	})
}

// Case rating: customer rates service after case resolved.
func handleCaseRate(w http.ResponseWriter, r *http.Request) {
	var in struct{ CaseID string; Score int; Comment string }
	json.NewDecoder(r.Body).Decode(&in)
	if in.CaseID == "" || in.Score < 1 || in.Score > 5 {
		writeJSON(w, 400, map[string]string{"error": "caseID and score (1-5) required"})
		return
	}
	uid := ""
	if c, ok := callerFrom(r); ok { uid = c.ID }

	// Verify case belongs to this user
	var owner string
	if db.QueryRow(r.Context(), `SELECT customer_id FROM cases WHERE id=$1`, in.CaseID).Scan(&owner) != nil || owner != uid {
		writeJSON(w, 403, map[string]string{"error": "not your case"})
		return
	}

	db.Exec(r.Context(), `UPDATE cases SET satisfaction_score=$1, resolution_notes=COALESCE(resolution_notes,'')||' [rated: '||$2||'/5]' WHERE id=$3`,
		in.Score, in.Score, in.CaseID)
	if in.Comment != "" {
		db.Exec(r.Context(), `INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'rating',$2)`,
			in.CaseID, fmt.Sprintf("score=%d/5 — %s", in.Score, in.Comment))
	}

	writeJSON(w, 200, map[string]any{"case_id": in.CaseID, "score": in.Score, "status": "rated"})
}

// G5: Push notification sender — stores subscription + sends via SSE fallback
var pushSubscriptions []map[string]any

func handlePushSend(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		URL   string `json:"url"`
	}
	json.NewDecoder(r.Body).Decode(&in)
	// Store subscription for web-push (future)
	if r.Method == "PUT" {
		var sub map[string]any
		json.NewDecoder(r.Body).Decode(&sub)
		if sub["endpoint"] != nil {
			pushSubscriptions = append(pushSubscriptions, sub)
		}
		writeJSON(w, 200, map[string]string{"status": "subscribed"})
		return
	}
	// Broadcast push event via SSE to all operator consoles
	publishSSE("push.notification", map[string]any{
		"title": nz(in.Title, "Insucar"),
		"body":  nz(in.Body, "Update on your case"),
		"url":   nz(in.URL, "/app"),
	})
	writeJSON(w, 200, map[string]string{"status": "broadcast", "subscribers": fmt.Sprintf("%d", len(pushSubscriptions))})
}

// Case arrived: motorist confirms provider has arrived on scene.
func handleCaseArrived(w http.ResponseWriter, r *http.Request) {
	var in struct{ CaseID string }
	json.NewDecoder(r.Body).Decode(&in)
	uid := ""
	if c, ok := callerFrom(r); ok { uid = c.ID }
	var owner string
	if db.QueryRow(r.Context(), `SELECT customer_id FROM cases WHERE id=$1`, in.CaseID).Scan(&owner) != nil || owner != uid {
		writeJSON(w, 403, map[string]string{"error": "not your case"})
		return
	}
	db.Exec(r.Context(), `UPDATE cases SET status='on_site' WHERE id=$1`, in.CaseID)
	db.Exec(r.Context(), `UPDATE missions SET status='on_site' WHERE case_id=$1`, in.CaseID)
	db.Exec(r.Context(), `INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'arrived','Motorist confirmed provider arrival')`, in.CaseID)
	publishSSE("case.updated", map[string]any{"case_id": in.CaseID, "status": "on_site"})
	writeJSON(w, 200, map[string]any{"case_id": in.CaseID, "status": "on_site"})
}

// Predictive ETA: adjusts based on time of day (rush hour penalty) and provider performance.
func handlePredictEta(w http.ResponseWriter, r *http.Request) {
	caseID := r.URL.Query().Get("case_id")
	if caseID == "" {
		writeJSON(w, 400, map[string]string{"error": "case_id required"})
		return
	}
	var baseETA int
	db.QueryRow(r.Context(), `SELECT coalesce(eta_minutes,20) FROM missions WHERE case_id=$1 ORDER BY created_at DESC LIMIT 1`, caseID).Scan(&baseETA)

	// Traffic multiplier: rush hour (7-9am, 4-7pm) = 1.5x, weekend = 0.8x
	now := time.Now()
	hour := now.Hour()
	mult := 1.0
	if (hour >= 7 && hour <= 9) || (hour >= 16 && hour <= 19) { mult = 1.5 }
	if now.Weekday() == time.Saturday || now.Weekday() == time.Sunday { mult = 0.8 }
	adjusted := int(float64(baseETA) * mult)

	writeJSON(w, 200, map[string]any{
		"case_id":    caseID,
		"base_eta":   baseETA,
		"adjusted":   adjusted,
		"multiplier": mult,
		"rush_hour":  (hour >= 7 && hour <= 9) || (hour >= 16 && hour <= 19),
	})
}

func callProvider(ctx context.Context, caseID, service string) (int, bool) {
	body, _ := json.Marshal(map[string]string{"case_id": caseID, "service": service})
	req, _ := http.NewRequestWithContext(ctx, "POST", providerURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		log.Printf(`{"stream":"error","event":"provider_call_failed","err":%q}`, err.Error())
		return 0, false
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Eta int `json:"eta_minutes"`
	}
	json.Unmarshal(raw, &out)
	return out.Eta, resp.StatusCode < 400
}

func sendSMS(ctx context.Context, caseID, provider, driver, plate string, eta int, link string) string {
	if snsClient == nil {
		return "disabled"
	}
	var phone *string
	db.QueryRow(ctx, `SELECT c.phone_e164 FROM cases ca JOIN customers c ON c.id=ca.customer_id WHERE ca.id=$1`, caseID).Scan(&phone)
	if phone == nil || *phone == "" {
		return "no_phone"
	}
	msg := fmt.Sprintf("Insucar: help is on the way. %s, driver %s (%s), ETA ~%d min. Track: %s", provider, driver, plate, eta, link)
	if _, err := snsClient.Publish(ctx, &sns.PublishInput{PhoneNumber: phone, Message: &msg}); err != nil {
		return "failed"
	}
	db.Exec(ctx, `INSERT INTO notifications(case_id,channel,recipient,template,status,status_link_token)
		VALUES($1,'sms',$2,'dispatch','sent',$3)`, caseID, *phone, link)
	return "sent"
}

// handleStatusPage serves the live tracking page + JSON API for customer status links
func handleStatusPage(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.URL.Path, "/api/status/")
	token = strings.TrimSuffix(token, "/")
	if token == "" {
		http.NotFound(w, r)
		return
	}
	if strings.Contains(r.Header.Get("Accept"), "text/html") || !strings.Contains(r.Header.Get("Accept"), "application/json") {
		http.ServeFile(w, r, "/app/web/status.html")
		return
	}
	var prov, drv, plate, svc string
	var etaMin int
	var lat, lng *float64
	var provPhoto, provRating *string
	err := db.QueryRow(r.Context(), `
		SELECT p.display_name, COALESCE(md.driver_name,''), COALESCE(md.vehicle_plate,''),
		       COALESCE(m.service,'tow_recovery'), COALESCE(m.eta_minutes,0),
		       ST_Y(cl.geog::geometry) as lat, ST_X(cl.geog::geometry) as lng,
		       md.photo_url, p.performance_score::text
		FROM notifications n
		JOIN missions m ON m.case_id = n.case_id
		JOIN providers p ON p.id = m.provider_id
		LEFT JOIN mission_driver md ON md.mission_id = m.id
		LEFT JOIN case_locations cl ON cl.case_id = n.case_id
		WHERE n.status_link_token = $1
		   OR n.status_link_token LIKE '%' || $1
		ORDER BY m.created_at DESC LIMIT 1`, token).
		Scan(&prov, &drv, &plate, &svc, &etaMin, &lat, &lng, &provPhoto, &provRating)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "status link not found"})
		return
	}
	resp := map[string]any{
		"provider":        prov,
		"driver_name":     drv,
		"vehicle_plate":   plate,
		"service":         svc,
		"eta_minutes":     etaMin,
		"eta":             time.Now().Add(time.Duration(etaMin) * time.Minute).Format(time.RFC3339),
		"provider_photo":  s(provPhoto),
		"provider_rating": s(provRating),
	}
	if lat != nil {
		resp["lat"] = *lat
		resp["lng"] = *lng
	}
	writeJSON(w, 200, resp)
}

// ===== ADMIN HANDLERS =====

type rateLimitConfig struct {
	Endpoint string `json:"endpoint"`
	RPM      int    `json:"rpm"`
	Burst    int    `json:"burst"`
	Enabled  bool   `json:"enabled"`
}

var rateLimits = map[string]rateLimitConfig{}

// GAP-1: Persistent rate limits loaded from DB on startup
func loadRateLimits() {
	if db == nil { return }
	rows, err := db.Query(context.Background(), `SELECT endpoint, rpm, burst, enabled FROM rate_limits`)
	if err != nil { return }
	defer rows.Close()
	for rows.Next() {
		var rl rateLimitConfig
		rows.Scan(&rl.Endpoint, &rl.RPM, &rl.Burst, &rl.Enabled)
		rateLimits[rl.Endpoint] = rl
	}
}

func requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, ok := resolveCaller(r)
		if !ok || c.Role != "agent" {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		var staffRole string
		db.QueryRow(r.Context(), `SELECT COALESCE(role::text,'operator') FROM staff WHERE id=$1`, c.ID).Scan(&staffRole)
		if staffRole != "admin" && staffRole != "product_owner" && staffRole != "supervisor" {
			writeJSON(w, 403, map[string]string{"error": "admin access required"})
			return
		}
		next(w, withCaller(r, c))
	}
}

func handleAdminRateLimits(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		rows, err := db.Query(r.Context(), `SELECT endpoint, rpm, burst, enabled FROM rate_limits ORDER BY endpoint`)
		if err != nil { writeJSON(w, 500, map[string]string{"error": "db error"}); return }
		defer rows.Close()
		var all []rateLimitConfig
		for rows.Next() {
			var rl rateLimitConfig
			rows.Scan(&rl.Endpoint, &rl.RPM, &rl.Burst, &rl.Enabled)
			all = append(all, rl)
		}
		if all == nil { all = []rateLimitConfig{} }
		writeJSON(w, 200, map[string]any{"rate_limits": all})
	case "PUT":
		var in rateLimitConfig
		json.NewDecoder(r.Body).Decode(&in)
		if in.Endpoint == "" { writeJSON(w, 400, map[string]string{"error": "endpoint required"}); return }
		db.Exec(r.Context(), `INSERT INTO rate_limits(endpoint,rpm,burst,enabled) VALUES($1,$2,$3,$4)
			ON CONFLICT(endpoint) DO UPDATE SET rpm=$2,burst=$3,enabled=$4,updated_at=now()`,
			in.Endpoint, in.RPM, in.Burst, in.Enabled)
		writeJSON(w, 200, in)
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func handleAdminApiAccess(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		rows, _ := db.Query(r.Context(), `
			SELECT endpoint, methods, min_role, description, is_active
			FROM api_endpoints ORDER BY endpoint`)
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			var ep, methods, role, desc string
			var active bool
			rows.Scan(&ep, &methods, &role, &desc, &active)
			out = append(out, map[string]any{
				"endpoint": ep, "methods": methods, "min_role": role, "description": desc, "is_active": active,
			})
		}
		writeJSON(w, 200, map[string]any{"endpoints": out})
	case "PUT":
		var in struct {
			Endpoint string `json:"endpoint"`
			IsActive bool   `json:"is_active"`
			MinRole  string `json:"min_role"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		db.Exec(r.Context(), `UPDATE api_endpoints SET is_active=$1, min_role=$2 WHERE endpoint=$3`,
			in.IsActive, in.MinRole, in.Endpoint)
		writeJSON(w, 200, map[string]string{"status": "updated"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func handleAdminOperators(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		rows, _ := db.Query(r.Context(), `
			SELECT id, email, display_name, agent_id, active, cognito_subject IS NOT NULL as has_cognito
			FROM staff ORDER BY display_name`)
		defer rows.Close()
		var out []map[string]any
		for rows.Next() {
			var id, email, name, agent string
			var active, hasCog bool
			rows.Scan(&id, &email, &name, &agent, &active, &hasCog)
			out = append(out, map[string]any{
				"id": id, "email": email, "display_name": name,
				"agent_id": agent, "active": active, "has_cognito": hasCog,
			})
		}
		writeJSON(w, 200, map[string]any{"operators": out})
	case "POST":
		var in struct {
			Email       string `json:"email"`
			DisplayName string `json:"display_name"`
			AgentID     string `json:"agent_id"`
			Password    string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&in)
		var id string
		err := db.QueryRow(r.Context(),
			`INSERT INTO staff(email,display_name,agent_id,password_hash,active) VALUES($1,$2,$3,$4,true) RETURNING id`,
			in.Email, in.DisplayName, in.AgentID, sha256hex(in.Password)).Scan(&id)
		if err != nil {
			writeJSON(w, 409, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 201, map[string]any{"id": id, "status": "created"})
	case "DELETE":
		id := r.URL.Query().Get("id")
		db.Exec(r.Context(), `UPDATE staff SET active=false WHERE id=$1`, id)
		writeJSON(w, 200, map[string]string{"status": "deactivated"})
	default:
		writeJSON(w, 405, map[string]string{"error": "method not allowed"})
	}
}

func handleAdminStats(w http.ResponseWriter, r *http.Request) {
	var totalCustomers, totalStaff, activeCases, totalMissions int
	db.QueryRow(r.Context(), `SELECT COUNT(*) FROM customers`).Scan(&totalCustomers)
	db.QueryRow(r.Context(), `SELECT COUNT(*) FROM staff`).Scan(&totalStaff)
	db.QueryRow(r.Context(), `SELECT COUNT(*) FROM cases WHERE status NOT IN ('closed','resolved','cancelled')`).Scan(&activeCases)
	db.QueryRow(r.Context(), `SELECT COUNT(*) FROM missions`).Scan(&totalMissions)
	writeJSON(w, 200, map[string]any{
		"total_customers": totalCustomers,
		"total_staff":     totalStaff,
		"active_cases":    activeCases,
		"total_missions":  totalMissions,
		"rate_limits":     len(rateLimits),
	})
}

func nz(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}
func s(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
