// Insucar prototype backend.
// Telephony: MOCK Amazon Connect adapter (emits Connect-shaped screen-pop events).
// Providers: REAL HTTP connector (calls a real provider API when PROVIDER_API_URL is configured).
// SMS: REAL via AWS SNS (best-effort; subject to SNS SMS sandbox).
package main

import (
	"bytes"
	"context"
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
	db        *pgxpool.Pool
	snsClient *sns.Client
	statusBase = getenv("STATUS_LINK_BASE", "https://app.unysolar.com/status")
	providerURL = os.Getenv("PROVIDER_API_URL") // real provider endpoint (registry-driven in prod)
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
		log.Println("AWS SNS client initialized")
	} else {
		log.Printf("AWS config not available, SMS disabled: %v", e)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/api/register", handleRegister)
	mux.HandleFunc("/api/lookup", handleLookup)
	mux.HandleFunc("/api/cases", handleCreateCase)
	mux.HandleFunc("/api/dispatch", handleDispatch)
	mux.HandleFunc("/api/case", handleGetCase)
	// MOCK Amazon Connect adapter: simulates an inbound call + Streams screen-pop payload
	mux.HandleFunc("/api/telephony/mock/incoming", handleMockIncoming)

	mux.HandleFunc("/", serveFile("enduser.html"))
	mux.HandleFunc("/login", serveFile("enduser.html"))
	mux.HandleFunc("/register", serveFile("enduser.html"))
	mux.HandleFunc(opsPath, serveFile("operator.html"))

	log.Printf("listening on :8080 (ops path %s; provider=%q; sms=%v)", opsPath, providerURL, snsClient != nil)
	log.Fatal(http.ListenAndServe(":8080", logMW(mux)))
}

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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	if db.Ping(r.Context()) != nil {
		writeJSON(w, 503, map[string]string{"status": "db_down"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var in struct {
		Email, Phone, First, Last, Language, Country string
		Consents                                     []string
	}
	if json.NewDecoder(r.Body).Decode(&in) != nil {
		writeJSON(w, 400, map[string]string{"error": "bad json"})
		return
	}
	var id string
	err := db.QueryRow(r.Context(),
		`INSERT INTO customers(email,phone_e164,first_name,last_name,preferred_language,country_code,status)
		 VALUES($1,$2,$3,$4,$5,$6,'active') RETURNING id`,
		in.Email, in.Phone, in.First, in.Last, nz(in.Language, "en"), nz(in.Country, "FR")).Scan(&id)
	if err != nil {
		writeJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	for _, c := range in.Consents {
		db.Exec(r.Context(), `INSERT INTO consents(customer_id,purpose,granted,basis) VALUES($1,$2,true,'consent')`, id, c)
	}
	db.Exec(r.Context(), `INSERT INTO audit_ledger(event_type,actor,payload) VALUES('customer.register',$1,$2)`,
		in.Email, fmt.Sprintf(`{"customer_id":"%s"}`, id))
	writeJSON(w, 201, map[string]string{"customer_id": id, "status": "active"})
}

func lookupByPhone(ctx context.Context, phone string) (map[string]any, string, string, bool) {
	row := db.QueryRow(ctx, `
		SELECT c.id, c.first_name, c.last_name, c.preferred_language, c.country_code,
		       p.policy_number, p.coverage::text, p.status::text,
		       v.license_plate, v.make, v.model, v.fuel::text
		FROM customers c
		LEFT JOIN policies p ON p.customer_id=c.id
		LEFT JOIN policy_vehicles pv ON pv.policy_id=p.id
		LEFT JOIN vehicles v ON v.id=pv.vehicle_id
		WHERE c.phone_e164=$1 LIMIT 1`, phone)
	var cid, fn, ln, lang, country, pol, cov, pstat, plate, mk, model, fuel *string
	if row.Scan(&cid, &fn, &ln, &lang, &country, &pol, &cov, &pstat, &plate, &mk, &model, &fuel) != nil {
		return nil, "", "", false
	}
	m := map[string]any{
		"customer": map[string]any{"id": s(cid), "first_name": s(fn), "last_name": s(ln), "language": s(lang), "country": s(country)},
		"policy":   map[string]any{"policy_number": s(pol), "coverage": s(cov), "status": s(pstat)},
		"vehicle":  map[string]any{"plate": s(plate), "make": s(mk), "model": s(model), "fuel": s(fuel)},
	}
	return m, s(cid), s(lang), true
}

func handleLookup(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		writeJSON(w, 400, map[string]string{"error": "phone required"})
		return
	}
	m, _, _, ok := lookupByPhone(r.Context(), phone)
	if !ok {
		writeJSON(w, 404, map[string]any{"matched": false, "phone": phone})
		return
	}
	m["matched"] = true
	m["phone"] = phone
	writeJSON(w, 200, m)
}

// MOCK Amazon Connect: simulate an inbound call -> return the screen-pop contact attributes.
func handleMockIncoming(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var in struct{ Phone string }
	json.NewDecoder(r.Body).Decode(&in)
	contactID := fmt.Sprintf("mock-%d", time.Now().UnixNano())
	m, _, lang, ok := lookupByPhone(r.Context(), in.Phone)
	pop := map[string]any{
		"connect_contact_id": contactID, "ani": in.Phone, "dnis": "+33800000000",
		"language": lang, "matched": ok, "priority": "high", "queue": "mobility",
	}
	if ok {
		pop["screen_pop"] = m
	}
	db.Exec(r.Context(), `INSERT INTO audit_ledger(event_type,actor,payload) VALUES('telephony.mock.incoming',$1,$2)`,
		in.Phone, fmt.Sprintf(`{"contact_id":"%s","matched":%v}`, contactID, ok))
	writeJSON(w, 200, pop)
}

func handleCreateCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var in struct{ CustomerID, Incident, Priority string }
	json.NewDecoder(r.Body).Decode(&in)
	caseNo := fmt.Sprintf("CASE-%d", time.Now().Unix())
	var id string
	var custArg any
	if in.CustomerID != "" {
		custArg = in.CustomerID
	}
	err := db.QueryRow(r.Context(),
		`INSERT INTO cases(case_number,customer_id,channel,status,priority,incident,incident_at)
		 VALUES($1,$2,'phone','triaging',$3,$4,now()) RETURNING id`,
		caseNo, custArg, nz(in.Priority, "high"), nz(in.Incident, "breakdown")).Scan(&id)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 201, map[string]string{"case_id": id, "case_number": caseNo, "status": "triaging"})
}

func handleDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var in struct{ CaseID, Service string }
	json.NewDecoder(r.Body).Decode(&in)

	var provID, provName string
	if db.QueryRow(r.Context(),
		`SELECT id, display_name FROM providers WHERE status='enabled' ORDER BY priority_rank ASC LIMIT 1`).
		Scan(&provID, &provName) != nil {
		writeJSON(w, 404, map[string]string{"error": "no provider available"})
		return
	}

	// REAL provider connection: call the external provider API if configured (registry-driven).
	providerSource := "internal"
	eta := 18 + rand.Intn(25)
	if providerURL != "" {
		if pe, ok := callProvider(r.Context(), in.CaseID, nz(in.Service, "tow_recovery")); ok {
			providerSource = "api"
			if pe > 0 {
				eta = pe
			}
		}
	}

	var mid string
	if db.QueryRow(r.Context(),
		`INSERT INTO missions(case_id,provider_id,service,source,status,eta_minutes)
		 VALUES($1,$2,$3,$4,'en_route',$5) RETURNING id`,
		in.CaseID, provID, nz(in.Service, "tow_recovery"), providerSource, eta).Scan(&mid) != nil {
		writeJSON(w, 400, map[string]string{"error": "dispatch failed"})
		return
	}
	db.Exec(r.Context(), `INSERT INTO mission_driver(mission_id,driver_name,vehicle_plate) VALUES($1,'Pierre L.','TOW-77-FR')`, mid)
	db.Exec(r.Context(), `UPDATE cases SET status='dispatched' WHERE id=$1`, in.CaseID)

	// REAL SMS: notify the customer with status link + driver identity.
	token := fmt.Sprintf("%d", time.Now().UnixNano())
	link := statusBase + "/" + token
	smsStatus := sendSMS(r.Context(), in.CaseID, provName, "Pierre L.", "TOW-77-FR", eta, link)

	writeJSON(w, 201, map[string]any{
		"mission_id": mid, "provider": provName, "provider_source": providerSource,
		"eta_minutes": eta, "driver": map[string]string{"name": "Pierre L.", "plate": "TOW-77-FR"},
		"status": "en_route", "status_link": link, "sms": smsStatus,
	})
}

// callProvider performs a REAL outbound HTTP POST to the configured provider API.
func callProvider(ctx context.Context, caseID, service string) (int, bool) {
	body, _ := json.Marshal(map[string]string{"case_id": caseID, "service": service})
	req, _ := http.NewRequestWithContext(ctx, "POST", providerURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: 8 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		log.Printf(`{"stream":"error","event":"provider_call_failed","err":%q}`, err.Error())
		return 0, false
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	log.Printf(`{"stream":"system","event":"provider_call","status":%d,"bytes":%d}`, resp.StatusCode, len(raw))
	var out struct{ Eta int `json:"eta_minutes"` }
	json.Unmarshal(raw, &out)
	return out.Eta, resp.StatusCode < 400
}

// sendSMS sends a REAL SMS via AWS SNS to the case's customer (best-effort).
func sendSMS(ctx context.Context, caseID, provider, driver, plate string, eta int, link string) string {
	if snsClient == nil {
		return "disabled"
	}
	var phone *string
	db.QueryRow(ctx, `SELECT c.phone_e164 FROM cases ca JOIN customers c ON c.id=ca.customer_id WHERE ca.id=$1`, caseID).Scan(&phone)
	if phone == nil || *phone == "" {
		return "no_phone"
	}
	msg := fmt.Sprintf("Insucar: help is on the way. %s, driver %s (%s), ETA ~%d min. Track: %s",
		provider, driver, plate, eta, link)
	_, err := snsClient.Publish(ctx, &sns.PublishInput{PhoneNumber: phone, Message: &msg})
	if err != nil {
		log.Printf(`{"stream":"error","event":"sms_failed","err":%q}`, err.Error())
		return "failed:" + err.Error()
	}
	db.Exec(ctx, `INSERT INTO notifications(case_id,channel,recipient,template,status,status_link_token)
		VALUES($1,'sms',$2,'dispatch',$3,$4)`, caseID, *phone, "sent", link)
	return "sent"
}

func handleGetCase(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	var caseNo, status, incident string
	if db.QueryRow(r.Context(), `SELECT case_number,status::text,incident::text FROM cases WHERE id=$1`, id).
		Scan(&caseNo, &status, &incident) != nil {
		writeJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	var eta *int
	var provName *string
	db.QueryRow(r.Context(), `SELECT m.eta_minutes, p.display_name FROM missions m
		JOIN providers p ON p.id=m.provider_id WHERE m.case_id=$1 ORDER BY m.created_at DESC LIMIT 1`, id).
		Scan(&eta, &provName)
	writeJSON(w, 200, map[string]any{"case_number": caseNo, "status": status, "incident": incident,
		"eta_minutes": eta, "provider": provName})
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
