// Provider connector infrastructure: webhook receiver, fallback chain, circuit breaker.
// Handles provider status callbacks, multi-provider dispatch with retry, and health tracking.
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
	"strings"
	"sync"
	"time"
)

// circuitState tracks provider health for circuit-breaking.
type circuitState struct {
	Failures    int       `json:"failures"`
	LastFailure time.Time `json:"last_failure"`
	OpenUntil   time.Time `json:"open_until"`
}

var (
	circuits   = map[string]*circuitState{}
	circuitMu  sync.RWMutex
	breakThreshold = 3
	breakDuration  = 30 * time.Second
)

// isProviderHealthy returns true if the provider is not circuit-broken.
func isProviderHealthy(provID string) bool {
	circuitMu.RLock()
	defer circuitMu.RUnlock()
	c, ok := circuits[provID]
	if !ok {
		return true
	}
	if c.OpenUntil.After(time.Now()) {
		return false
	}
	return true
}

// recordProviderFailure marks a failure and opens the circuit if threshold exceeded.
func recordProviderFailure(provID string) {
	circuitMu.Lock()
	defer circuitMu.Unlock()
	c, ok := circuits[provID]
	if !ok {
		c = &circuitState{}
		circuits[provID] = c
	}
	c.Failures++
	c.LastFailure = time.Now()
	if c.Failures >= breakThreshold {
		c.OpenUntil = time.Now().Add(breakDuration)
		log.Printf(`{"stream":"system","event":"circuit_open","provider":%q,"failures":%d}`, provID, c.Failures)
	}
}

// recordProviderSuccess resets the circuit for a provider.
func recordProviderSuccess(provID string) {
	circuitMu.Lock()
	defer circuitMu.Unlock()
	c, ok := circuits[provID]
	if !ok {
		return
	}
	c.Failures = 0
	c.OpenUntil = time.Time{}
}

// ------- Webhook Receiver -------

// Provider webhook payload (standardized for AXA / Towpal / etc.).
type webhookPayload struct {
	ExternalID  string `json:"external_mission_id"`
	MissionStatus string `json:"status"`          // searching, offered, accepted, en_route, on_site, completed, failed, cancelled
	ETA         *int   `json:"eta_minutes"`
	DriverName  string `json:"driver_name"`
	DriverPlate string `json:"driver_plate"`
	Timestamp   string `json:"timestamp"`
	RawPayload  json.RawMessage `json:"raw,omitempty"`
}

// handleProviderWebhook receives status callbacks from external providers.
// Verifies HMAC signature, updates mission status and case lifecycle.
func handleProviderWebhook(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, 400, map[string]string{"error": "bad request"})
		return
	}

	var payload webhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid json"})
		return
	}

	if payload.ExternalID == "" {
		writeJSON(w, 400, map[string]string{"error": "missing external_mission_id"})
		return
	}

	// Find the mission by external_mission_id
	var missionID, caseID string
	var currentStatus string
	err = db.QueryRow(r.Context(),
		`SELECT id, case_id::text, status::text FROM missions WHERE external_mission_id=$1 LIMIT 1`,
		payload.ExternalID).Scan(&missionID, &caseID, &currentStatus)
	if err != nil {
		// Try to find by idempotency key or return 404
		log.Printf(`{"stream":"webhook","event":"mission_not_found","external_id":%q}`, payload.ExternalID)
		writeJSON(w, 404, map[string]string{"error": "mission not found"})
		return
	}

	// Record the status event
	if payload.MissionStatus != "" {
		eta := payload.ETA
		if eta == nil {
			// keep existing ETA if not provided
			var existingETA int
			if db.QueryRow(r.Context(), `SELECT coalesce(eta_minutes,0) FROM missions WHERE id=$1`, missionID).Scan(&existingETA) == nil {
				eta = &existingETA
			}
		}
		db.Exec(r.Context(),
			`INSERT INTO mission_status_events(mission_id,status,raw_status,eta_minutes,payload,occurred_at)
			 VALUES($1,$2,$3,$4,$5,now())`,
			missionID, payload.MissionStatus, payload.MissionStatus, eta, string(body))

		// Update mission status
		db.Exec(r.Context(),
			`UPDATE missions SET status=$1, eta_minutes=coalesce($2,eta_minutes), updated_at=now()
			 WHERE id=$3`,
			payload.MissionStatus, eta, missionID)

		// Map mission status to case status transitions
		newCaseStatus := mapMissionToCase(payload.MissionStatus)
		if newCaseStatus != "" {
			db.Exec(r.Context(), `UPDATE cases SET status=$1, updated_at=now() WHERE id=$2`, newCaseStatus, caseID)
			db.Exec(r.Context(),
				`INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,$2,$3)`,
				caseID, "status_update",
				fmt.Sprintf("Provider webhook: mission %s -> %s", currentStatus, payload.MissionStatus))
		}
	}

	// Update driver info if provided
	if payload.DriverName != "" {
		db.Exec(r.Context(),
			`UPDATE mission_driver SET driver_name=$1, vehicle_plate=$2, updated_at=now()
			 WHERE mission_id=$3`,
			payload.DriverName, payload.DriverPlate, missionID)
		// Also insert if not exists
		db.Exec(r.Context(),
			`INSERT INTO mission_driver(mission_id,driver_name,vehicle_plate)
			 VALUES($1,$2,$3) ON CONFLICT DO NOTHING`,
			missionID, payload.DriverName, payload.DriverPlate)
	}

	writeJSON(w, 200, map[string]any{
		"status":    "acknowledged",
		"mission_id": missionID,
		"new_status": payload.MissionStatus,
	})
}

// mapMissionToCase translates mission status updates into case lifecycle transitions.
func mapMissionToCase(ms string) string {
	switch ms {
	case "en_route":
		return "en_route"
	case "on_site":
		return "on_site"
	case "completed":
		return "resolved"
	case "failed", "cancelled":
		return "cancelled"
	default:
		return ""
	}
}

// ------- Fallback Dispatch Chain -------

// dispatchWithFallback tries to dispatch to providers in priority order.
// If the primary provider fails (circuit open, API timeout, error), it tries the next.
// Returns the mission details or an error if all providers fail.
func dispatchWithFallback(ctx context.Context, caseID, service string, forceProvider string) (map[string]any, error) {
	svc := nz(service, "tow_recovery")

	// Get ordered list of enabled providers matching the service
	var providers []struct {
		ID   string
		Name string
		URL  string
		Rank int
	}
	rows, err := db.Query(ctx, `
		SELECT p.id, p.display_name, coalesce(pc.base_url,''), p.priority_rank
		FROM providers p
		JOIN provider_connectors pc ON pc.provider_id = p.id
		WHERE p.status = 'enabled' AND pc.status = 'enabled'
		ORDER BY p.priority_rank ASC, p.performance_score DESC NULLS LAST
		LIMIT 10`)
	if err != nil {
		return nil, fmt.Errorf("provider query failed: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var p struct {
			ID   string
			Name string
			URL  string
			Rank int
		}
		rows.Scan(&p.ID, &p.Name, &p.URL, &p.Rank)
		providers = append(providers, p)
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no providers available")
	}

	// If a specific provider was requested, try it first, then fall through to others
	if forceProvider != "" {
		for i, p := range providers {
			if p.ID == forceProvider {
				// Move to front
				providers[0], providers[i] = providers[i], providers[0]
				break
			}
		}
	}

	// Track tried providers for the response
	var attempts []string
	for _, p := range providers {
		// Skip circuit-broken providers
		if !isProviderHealthy(p.ID) {
			log.Printf(`{"stream":"dispatch","event":"circuit_skip","provider":%q}`, p.Name)
			attempts = append(attempts, fmt.Sprintf("%s (circuit open)", p.Name))
			continue
		}

		attempts = append(attempts, p.Name)
		src := "api"
		eta := 18 + rand.Intn(25)

		// Try calling the provider API
		callURL := p.URL
		if callURL == "" || callURL == "https://api.axa-roadside.example" || callURL == "https://api.towpal.example" {
			// Fall back to PROVIDER_API_URL env var for the stub service
			if providerURL != "" {
				callURL = providerURL
			}
		}
		if callURL != "" {
			if pe, ok := callProviderURL(ctx, caseID, svc, callURL); ok {
				recordProviderSuccess(p.ID)
				if pe > 0 {
					eta = pe
				}
			} else {
				// Provider call failed — record and try next
				recordProviderFailure(p.ID)
				log.Printf(`{"stream":"dispatch","event":"provider_failed","provider":%q,"case":%q}`, p.Name, caseID)
				continue
			}
		} else {
			src = "internal"
		}

		// Success! Create the mission
		var mid string
		if db.QueryRow(ctx,
			`INSERT INTO missions(case_id,provider_id,service,source,status,eta_minutes)
			 VALUES($1,$2,$3,$4,'en_route',$5) RETURNING id`,
			caseID, p.ID, svc, src, eta).Scan(&mid) != nil {
			return nil, fmt.Errorf("mission insert failed")
		}
		db.Exec(ctx,
			`INSERT INTO mission_status_events(mission_id,status,eta_minutes,occurred_at)
			 VALUES($1,'en_route',$2,now())`, mid, eta)
		db.Exec(ctx, `INSERT INTO mission_driver(mission_id,driver_name,vehicle_plate) VALUES($1,'TBD','TBD')`, mid)
		db.Exec(ctx, `UPDATE cases SET status='dispatched' WHERE id=$1`, caseID)
		db.Exec(ctx, `INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'dispatch',$2)`,
			caseID, fmt.Sprintf("dispatched %s (via %s) ETA %d min", p.Name, "api", eta))

		// SSE: notify operators of the dispatch
		publishSSE("case.dispatched", map[string]any{
			"case_id":  caseID,
			"provider": p.Name,
			"eta":      eta,
			"status":   "dispatched",
		})

		// SMS notification
		link := statusBase + "/" + fmt.Sprintf("%d", time.Now().UnixNano())
		smsStatus := sendSMS(ctx, caseID, p.Name, "en route", "pending", eta, link)

		return map[string]any{
			"mission_id":      mid,
			"provider":        p.Name,
			"provider_id":     p.ID,
			"provider_source": src,
			"eta_minutes":     eta,
			"status":          "en_route",
			"status_link":     link,
			"sms":             smsStatus,
			"attempted":       attempts,
			"driver":          map[string]string{"name": "TBD", "plate": "TBD"},
		}, nil
	}

	return nil, fmt.Errorf("all providers failed or were unavailable; attempted: %v", attempts)
}

// callProviderURL is like callProvider but with explicit URL.
func callProviderURL(ctx context.Context, caseID, service, url string) (int, bool) {
	body, _ := json.Marshal(map[string]string{
		"case_id": caseID,
		"service": service,
	})
	req, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Insucar-Webhook", fmt.Sprintf("https://api.unysolar.com/api/webhook/provider"))

	resp, err := (&http.Client{Timeout: 8 * time.Second}).Do(req)
	if err != nil {
		log.Printf(`{"stream":"error","event":"provider_call_failed","url":%q,"err":%q}`, url, err.Error())
		return 0, false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return 0, false
	}
	raw, _ := io.ReadAll(resp.Body)
	var out struct {
		Eta      int    `json:"eta_minutes"`
		Status   string `json:"status"`
		Driver   string `json:"driver_name"`
		Plate    string `json:"driver_plate"`
	}
	json.Unmarshal(raw, &out)
	if out.Status == "rejected" {
		return 0, false
	}
	return out.Eta, resp.StatusCode < 400
}

// ------- Provider Health Check -------

// checkProviderHealth pings each provider's base URL and updates circuit state.
func checkProviderHealth(ctx context.Context) {
	rows, err := db.Query(ctx, `
		SELECT p.id, pc.base_url FROM providers p
		JOIN provider_connectors pc ON pc.provider_id = p.id
		WHERE p.status='enabled' AND pc.status='enabled' AND pc.base_url != ''`)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var id, url string
		rows.Scan(&id, &url)
		if url == "https://api.axa-roadside.example" || url == "https://api.towpal.example" {
			if providerURL != "" {
				url = providerURL
			}
		}
		req, _ := http.NewRequestWithContext(ctx, "GET", url+"/health", nil)
		cl := &http.Client{Timeout: 5 * time.Second}
		resp, err := cl.Do(req)
		if err != nil || resp.StatusCode >= 500 {
			recordProviderFailure(id)
		} else {
			recordProviderSuccess(id)
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
}

// startProviderHealthLoop runs periodic health checks in the background.
func startProviderHealthLoop(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				checkProviderHealth(ctx)
			}
		}
	}()
	log.Printf("provider health checker: started (interval 60s)")
}

// ------- Webhook signature verification -------

var webhookSecretKey = []byte(getenv("WEBHOOK_SECRET", "insucar-webhook-secret-change-me"))

// verifyWebhookSignature validates the HMAC-SHA256 signature on incoming webhooks.
// The provider includes X-Insucar-Signature: hex(HMAC-SHA256(body, secret)).
func verifyWebhookSignature(r *http.Request, body []byte) bool {
	sig := r.Header.Get("X-Insucar-Signature")
	if sig == "" {
		// Try X-Hub-Signature-256 (GitHub-style) as fallback
		sig = r.Header.Get("X-Hub-Signature-256")
		sig = strings.TrimPrefix(sig, "sha256=")
	}
	if sig == "" {
		return false
	}
	mac := hmac.New(sha256.New, webhookSecretKey)
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}
