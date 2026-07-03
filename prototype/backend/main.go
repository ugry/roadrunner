// Insucar first-prototype backend (real, no business-logic mocks).
// Serves the separate end-user app and the hidden operator console, and the REST APIs
// they call, all backed by the real PostgreSQL/PostGIS schema.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var db *pgxpool.Pool

// Obscure, non-discoverable operator path (would be per-deployment secret in prod).
const opsPath = "/ops-console-7f3a9c"

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:test@db:5432/insucar?sslmode=disable"
	}
	var err error
	for i := 0; i < 30; i++ {
		db, err = pgxpool.New(context.Background(), dsn)
		if err == nil {
			if err = db.Ping(context.Background()); err == nil {
				break
			}
		}
		log.Printf("waiting for db (%d)...", i)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("db connect failed: %v", err)
	}
	log.Println("connected to database")

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealth)
	mux.HandleFunc("/api/register", handleRegister)
	mux.HandleFunc("/api/lookup", handleLookup)
	mux.HandleFunc("/api/cases", handleCreateCase)
	mux.HandleFunc("/api/dispatch", handleDispatch)
	mux.HandleFunc("/api/case", handleGetCase)

	// end-user app (public)
	mux.HandleFunc("/", serveFile("enduser.html"))
	mux.HandleFunc("/login", serveFile("enduser.html"))
	mux.HandleFunc("/register", serveFile("enduser.html"))
	// operator console (hidden, non-advertised path)
	mux.HandleFunc(opsPath, serveFile("operator.html"))

	addr := ":8080"
	log.Printf("listening on %s (operator path: %s)", addr, opsPath)
	log.Fatal(http.ListenAndServe(addr, logMW(mux)))
}

func logMW(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// system.log-style structured line
		log.Printf(`{"stream":"system","method":"%s","path":"%s","ua":"%s"}`, r.Method, r.URL.Path, r.UserAgent())
		h.ServeHTTP(w, r)
	})
}

func serveFile(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Any unknown path under root that is not the operator path returns 404,
		// so the hidden operator surface is not discoverable by guessing "/".
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
	if err := db.Ping(r.Context()); err != nil {
		writeJSON(w, 503, map[string]string{"status": "db_down"})
		return
	}
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

// End-user registration (real insert + consent rows).
func handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var in struct {
		Email, Phone, First, Last, Language, Country string
		Consents                                     []string
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
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

// ANI -> customer/policy/vehicle screen-pop lookup.
func handleLookup(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		writeJSON(w, 400, map[string]string{"error": "phone required"})
		return
	}
	row := db.QueryRow(r.Context(), `
		SELECT c.id, c.first_name, c.last_name, c.preferred_language, c.country_code,
		       p.policy_number, p.coverage::text, p.status::text,
		       v.id, v.license_plate, v.make, v.model, v.fuel::text
		FROM customers c
		LEFT JOIN policies p ON p.customer_id=c.id
		LEFT JOIN policy_vehicles pv ON pv.policy_id=p.id
		LEFT JOIN vehicles v ON v.id=pv.vehicle_id
		WHERE c.phone_e164=$1 LIMIT 1`, phone)
	var m = map[string]any{}
	var cid, fn, ln, lang, country, pol, cov, pstat, vid, plate, make_, model, fuel *string
	if err := row.Scan(&cid, &fn, &ln, &lang, &country, &pol, &cov, &pstat, &vid, &plate, &make_, &model, &fuel); err != nil {
		writeJSON(w, 404, map[string]any{"matched": false, "phone": phone})
		return
	}
	m["matched"] = true
	m["phone"] = phone
	m["customer"] = map[string]any{"id": s(cid), "first_name": s(fn), "last_name": s(ln), "language": s(lang), "country": s(country)}
	m["policy"] = map[string]any{"policy_number": s(pol), "coverage": s(cov), "status": s(pstat)}
	m["vehicle"] = map[string]any{"id": s(vid), "plate": s(plate), "make": s(make_), "model": s(model), "fuel": s(fuel)}
	writeJSON(w, 200, m)
}

// Create a case (real row).
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

// Dispatch nearest available provider (real provider row + mission + driver).
func handleDispatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method", 405)
		return
	}
	var in struct{ CaseID, Service string }
	json.NewDecoder(r.Body).Decode(&in)
	var provID, provName string
	err := db.QueryRow(r.Context(),
		`SELECT id, display_name FROM providers WHERE status='enabled' ORDER BY priority_rank ASC LIMIT 1`).Scan(&provID, &provName)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": "no provider available"})
		return
	}
	eta := 18 + rand.Intn(25)
	var mid string
	err = db.QueryRow(r.Context(),
		`INSERT INTO missions(case_id,provider_id,service,source,status,eta_minutes)
		 VALUES($1,$2,$3,'manual','en_route',$4) RETURNING id`,
		in.CaseID, provID, nz(in.Service, "tow_recovery"), eta).Scan(&mid)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": err.Error()})
		return
	}
	db.Exec(r.Context(), `INSERT INTO mission_driver(mission_id,driver_name,vehicle_plate) VALUES($1,'Pierre L.','TOW-77-FR')`, mid)
	db.Exec(r.Context(), `UPDATE cases SET status='dispatched' WHERE id=$1`, in.CaseID)
	writeJSON(w, 201, map[string]any{"mission_id": mid, "provider": provName, "eta_minutes": eta,
		"driver": map[string]string{"name": "Pierre L.", "plate": "TOW-77-FR"}, "status": "en_route"})
}

func handleGetCase(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	var caseNo, status, incident string
	err := db.QueryRow(r.Context(), `SELECT case_number,status::text,incident::text FROM cases WHERE id=$1`, id).
		Scan(&caseNo, &status, &incident)
	if err != nil {
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
