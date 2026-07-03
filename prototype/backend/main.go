// Insucar prototype backend (partially functional).
// Auth: app-level sessions (HMAC cookie). Users log in by email; agents by agent_id.
// Telephony: MOCK Amazon Connect. Providers: REAL HTTP connector. SMS: REAL AWS SNS.
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
	"math/rand"
	"net/http"
	"os"
	"strings"
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
	if cfg, e := awscfg.LoadDefaultConfig(context.Background()); e == nil {
		snsClient = sns.NewFromConfig(cfg)
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
	mux.HandleFunc("/api/telephony/mock/incoming", handleMockIncoming)

	// user (customer) — requires user session
	mux.HandleFunc("/api/user/incident", requireRole("user", handleUserIncident))
	mux.HandleFunc("/api/user/cases", requireRole("user", handleUserCases))

	// agent (operator) — requires staff session
	mux.HandleFunc("/api/agent/cases", requireRole("agent", handleAgentCases))
	mux.HandleFunc("/api/agent/case", requireRole("agent", handleAgentCase))
	mux.HandleFunc("/api/agent/lookup", requireRole("agent", handleLookup))
	mux.HandleFunc("/api/agent/dispatch", requireRole("agent", handleDispatch))

	// pages
	mux.HandleFunc("/", serveFile("enduser.html"))
	mux.HandleFunc("/login", serveFile("enduser.html"))
	mux.HandleFunc("/register", serveFile("enduser.html"))
	mux.HandleFunc(opsPath, serveFile("operator.html"))

	log.Printf("listening on :8080 (ops path %s; provider=%q; sms=%v)", opsPath, providerURL, snsClient != nil)
	log.Fatal(http.ListenAndServe(":8080", logMW(mux)))
}

// ---------- infra helpers ----------
func logMW(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf(`{"stream":"system","method":"%s","path":"%s"}`, r.Method, r.URL.Path)
		h.ServeHTTP(w, r)
	})
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
		rl, _, _, ok := currentSession(r)
		if !ok || rl != role {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

// ---------- handlers ----------
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if db.Ping(r.Context()) != nil {
		writeJSON(w, 503, map[string]string{"status": "db_down"})
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
	role, id, name, ok := currentSession(r)
	if !ok {
		writeJSON(w, 200, map[string]any{"authenticated": false})
		return
	}
	writeJSON(w, 200, map[string]any{"authenticated": true, "role": role, "id": id, "name": name})
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
	var id string
	err := db.QueryRow(r.Context(),
		`INSERT INTO customers(email,phone_e164,first_name,last_name,preferred_language,country_code,status,password_hash)
		 VALUES($1,$2,$3,$4,$5,$6,'active',$7) RETURNING id`,
		in.Email, in.Phone, in.First, in.Last, nz(in.Language, "en"), nz(in.Country, "FR"), sha256hex(in.Password)).Scan(&id)
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
	writeJSON(w, 201, map[string]string{"customer_id": id, "status": "active"})
}

// USER: submit an incident (creates a case for the logged-in customer)
func handleUserIncident(w http.ResponseWriter, r *http.Request) {
	_, uid, _, _ := currentSession(r)
	var in struct{ Incident, Description, Address, Lat, Lng string }
	json.NewDecoder(r.Body).Decode(&in)
	caseNo := fmt.Sprintf("CASE-%d", time.Now().Unix())
	var cid string
	err := db.QueryRow(r.Context(),
		`INSERT INTO cases(case_number,customer_id,channel,status,priority,incident,incident_at,symptom_description)
		 VALUES($1,$2,'app','triaging','high',$3,now(),$4) RETURNING id`,
		caseNo, uid, nz(in.Incident, "breakdown"), in.Description).Scan(&cid)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	if in.Address != "" || in.Lat != "" {
		db.Exec(r.Context(), `INSERT INTO case_locations(case_id,address_text,capture_method) VALUES($1,$2,'app')`, cid, in.Address)
	}
	db.Exec(r.Context(), `INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'note',$2)`, cid, "customer submitted via app: "+in.Description)
	writeJSON(w, 201, map[string]string{"case_id": cid, "case_number": caseNo, "status": "triaging"})
}

// USER: list own cases
func handleUserCases(w http.ResponseWriter, r *http.Request) {
	_, uid, _, _ := currentSession(r)
	rows, _ := db.Query(r.Context(),
		`SELECT case_number,status::text,incident::text,coalesce(symptom_description,''),created_at
		 FROM cases WHERE customer_id=$1 ORDER BY created_at DESC`, uid)
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var no, st, inc, desc string
		var ts time.Time
		rows.Scan(&no, &st, &inc, &desc, &ts)
		out = append(out, map[string]any{"case_number": no, "status": st, "incident": inc, "description": desc, "created_at": ts})
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

// AGENT: case detail (customer + vehicle + latest mission)
func handleAgentCase(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	var no, st, pr, inc, desc, who, phone string
	err := db.QueryRow(r.Context(), `
		SELECT ca.case_number, ca.status::text, ca.priority::text, ca.incident::text, coalesce(ca.symptom_description,''),
		       coalesce(c.first_name||' '||c.last_name,'(unknown)'), coalesce(c.phone_e164,'')
		FROM cases ca LEFT JOIN customers c ON c.id=ca.customer_id WHERE ca.id=$1`, id).
		Scan(&no, &st, &pr, &inc, &desc, &who, &phone)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	var plate, mk, model, pol, cov *string
	db.QueryRow(r.Context(), `
		SELECT v.license_plate,v.make,v.model,p.policy_number,p.coverage::text
		FROM cases ca JOIN customers c ON c.id=ca.customer_id
		LEFT JOIN policies p ON p.customer_id=c.id
		LEFT JOIN policy_vehicles pv ON pv.policy_id=p.id
		LEFT JOIN vehicles v ON v.id=pv.vehicle_id WHERE ca.id=$1 LIMIT 1`, id).
		Scan(&plate, &mk, &model, &pol, &cov)
	var eta *int
	var prov *string
	db.QueryRow(r.Context(), `SELECT m.eta_minutes,p.display_name FROM missions m JOIN providers p ON p.id=m.provider_id
		WHERE m.case_id=$1 ORDER BY m.created_at DESC LIMIT 1`, id).Scan(&eta, &prov)
	writeJSON(w, 200, map[string]any{
		"case_number": no, "status": st, "priority": pr, "incident": inc, "description": desc,
		"customer": who, "phone": phone,
		"policy": map[string]any{"number": s(pol), "coverage": s(cov)},
		"vehicle": map[string]any{"plate": s(plate), "make": s(mk), "model": s(model)},
		"mission": map[string]any{"provider": s(prov), "eta_minutes": eta},
	})
}

func lookupByPhone(ctx context.Context, phone string) (map[string]any, string, string, bool) {
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
	return map[string]any{
		"customer": map[string]any{"id": s(cid), "first_name": s(fn), "last_name": s(ln), "language": s(lang), "country": s(country)},
		"policy":   map[string]any{"policy_number": s(pol), "coverage": s(cov), "status": s(pstat)},
		"vehicle":  map[string]any{"plate": s(plate), "make": s(mk), "model": s(model), "fuel": s(fuel)},
	}, s(cid), s(lang), true
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

func handleDispatch(w http.ResponseWriter, r *http.Request) {
	var in struct{ CaseID, Service string }
	json.NewDecoder(r.Body).Decode(&in)
	var provID, provName string
	if db.QueryRow(r.Context(), `SELECT id,display_name FROM providers WHERE status='enabled' ORDER BY priority_rank ASC LIMIT 1`).
		Scan(&provID, &provName) != nil {
		writeJSON(w, 404, map[string]string{"error": "no provider available"})
		return
	}
	src := "internal"
	eta := 18 + rand.Intn(25)
	if providerURL != "" {
		if pe, ok := callProvider(r.Context(), in.CaseID, nz(in.Service, "tow_recovery")); ok {
			src = "api"
			if pe > 0 {
				eta = pe
			}
		}
	}
	var mid string
	if db.QueryRow(r.Context(),
		`INSERT INTO missions(case_id,provider_id,service,source,status,eta_minutes)
		 VALUES($1,$2,$3,$4,'en_route',$5) RETURNING id`,
		in.CaseID, provID, nz(in.Service, "tow_recovery"), src, eta).Scan(&mid) != nil {
		writeJSON(w, 400, map[string]string{"error": "dispatch failed"})
		return
	}
	db.Exec(r.Context(), `INSERT INTO mission_driver(mission_id,driver_name,vehicle_plate) VALUES($1,'Pierre L.','TOW-77-FR')`, mid)
	db.Exec(r.Context(), `UPDATE cases SET status='dispatched' WHERE id=$1`, in.CaseID)
	db.Exec(r.Context(), `INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'dispatch',$2)`, in.CaseID,
		fmt.Sprintf("dispatched %s ETA %d min", provName, eta))
	link := statusBase + "/" + fmt.Sprintf("%d", time.Now().UnixNano())
	smsStatus := sendSMS(r.Context(), in.CaseID, provName, "Pierre L.", "TOW-77-FR", eta, link)
	writeJSON(w, 201, map[string]any{"mission_id": mid, "provider": provName, "provider_source": src,
		"eta_minutes": eta, "driver": map[string]string{"name": "Pierre L.", "plate": "TOW-77-FR"},
		"status": "en_route", "status_link": link, "sms": smsStatus})
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
