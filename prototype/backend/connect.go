// Amazon Connect + Lex telephony integration (#35).
// Handles Connect contact events, Lex NLU triage, and PSAP warm-transfer coordination.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// Connect event types from Amazon Connect contact flows.
type connectEvent struct {
	ContactID        string `json:"contactId"`
	InstanceARN      string `json:"instanceArn"`
	EventType        string `json:"eventType"` // CONNECTED, DISCONNECTED, QUEUED, etc.
	ANI              string `json:"ani"`       // caller phone number
	DNIS             string `json:"dnis"`      // called number
	InitialContactID string `json:"initialContactId"`
	QueueName        string `json:"queueName"`
	Timestamp        string `json:"timestamp"`
}

// Lex NLU result from Connect contact flow.
type lexResult struct {
	Intent     string  `json:"intent"`      // breakdown, accident, medical, flat_tyre, lockout
	Confidence float64 `json:"confidence"`
	Slots      map[string]string `json:"slots"`
}

// handleConnectEvent receives Amazon Connect contact events via webhook.
// Maps Connect lifecycle to Insucar case states.
func handleConnectEvent(w http.ResponseWriter, r *http.Request) {
	var evt connectEvent
	if json.NewDecoder(r.Body).Decode(&evt) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid event"})
		return
	}

	log.Printf(`{"stream":"connect","event":%q,"contact":%q,"ani":%q}`, evt.EventType, evt.ContactID, evt.ANI)

	switch evt.EventType {
	case "CONNECTED":
		// Call answered — create or link case
		handleConnectConnected(r, &evt)
	case "DISCONNECTED":
		// Call ended — log wrap-up
		db.Exec(r.Context(),
			`INSERT INTO interaction_log(event_type,note) VALUES('call_ended',$1)`,
			fmt.Sprintf("Connect call %s disconnected, ANI=%s", evt.ContactID, evt.ANI))
	case "QUEUED":
		// Caller waiting in queue
		publishSSE("connect.queued", map[string]any{
			"contact_id": evt.ContactID, "ani": evt.ANI, "queue": evt.QueueName,
		})
	}

	writeJSON(w, 200, map[string]string{"status": "acknowledged", "contact_id": evt.ContactID})
}

// handleConnectConnected creates or finds a case when a call connects.
func handleConnectConnected(r *http.Request, evt *connectEvent) {
	// ANI lookup to find existing customer
	m, cid, _, matched := lookupByPhone(r.Context(), evt.ANI)

	if matched && cid != "" {
		// Check for existing open case
		var caseID string
		db.QueryRow(r.Context(),
			`SELECT id FROM cases WHERE customer_id=$1 AND status NOT IN ('closed','resolved','cancelled')
			 ORDER BY created_at DESC LIMIT 1`, cid).Scan(&caseID)
		if caseID != "" {
			// Link call to existing case (duplicate call detection — #32)
			db.Exec(r.Context(),
				`UPDATE cases SET connect_contact_id=$1, channel='phone', updated_at=now() WHERE id=$2`,
				evt.ContactID, caseID)
			publishSSE("connect.screen_pop", map[string]any{
				"contact_id": evt.ContactID, "case_id": caseID,
				"customer":  m["customer"], "policy": m["policy"],
				"duplicate": true,
			})
			return
		}
	}

	// New call — create triaging case
	caseNo := fmt.Sprintf("CASE-%d", time.Now().Unix())
	db.QueryRow(r.Context(),
		`INSERT INTO cases(case_number,customer_id,channel,status,priority,incident,incident_at,connect_contact_id)
		 VALUES($1,$2,'phone','triaging','high','breakdown',now(),$3) RETURNING id`,
		caseNo, nz(cid, ""), evt.ContactID)

	if matched {
		publishSSE("connect.screen_pop", map[string]any{
			"contact_id": evt.ContactID, "case_number": caseNo,
			"customer": m["customer"], "policy": m["policy"],
			"duplicate": false,
		})
	}
}

// handleLexTriage receives Lex NLU results from the Connect contact flow.
// Maps Lex intents to incident types and priority.
func handleLexTriage(w http.ResponseWriter, r *http.Request) {
	var result lexResult
	if json.NewDecoder(r.Body).Decode(&result) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid lex result"})
		return
	}

	// Map Lex intent to incident type
	intentToIncident := map[string]string{
		"breakdown":       "breakdown",
		"accident":        "accident",
		"flat_tyre":       "flat_tyre",
		"medical":         "medical_emergency",
		"lockout":         "lockout",
		"out_of_fuel":     "out_of_fuel",
		"battery_dead":    "battery",
		"ev_no_charge":    "ev_no_charge",
	}

	incidentType := intentToIncident[result.Intent]
	if incidentType == "" {
		incidentType = "other"
	}

	// Priority: low confidence → high priority (escalate for review)
	priority := "high"
	if result.Confidence > 0.8 {
		priority = "normal"
	}
	if result.Intent == "accident" || result.Intent == "medical" {
		priority = "emergency"
	}

	writeJSON(w, 200, map[string]any{
		"intent":         result.Intent,
		"confidence":     result.Confidence,
		"incident_type":  incidentType,
		"priority":       priority,
		"slots":          result.Slots,
		"needs_review":   result.Confidence < 0.7,
	})
}

// handlePsapTransfer handles the PSAP warm-transfer flow from Connect.
// Receives the contact ID being transferred and logs the audit trail.
func handlePsapTransfer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ContactID string `json:"contact_id"`
		CaseID    string `json:"case_id"`
		Reason    string `json:"reason"`
		PSAPRef   string `json:"psap_reference"`
	}
	json.NewDecoder(r.Body).Decode(&in)

	if in.CaseID != "" {
		db.Exec(r.Context(),
			`INSERT INTO interaction_log(case_id,event_type,note) VALUES($1,'psap_transfer',$2)`,
			in.CaseID, fmt.Sprintf("Connect PSAP transfer: %s, ref=%s", in.Reason, in.PSAPRef))
		db.Exec(r.Context(),
			`UPDATE cases SET priority='emergency' WHERE id=$1 AND priority!='emergency'`, in.CaseID)
	}

	publishSSE("connect.psap", map[string]any{
		"contact_id":    in.ContactID,
		"case_id":       in.CaseID,
		"psap_reference": in.PSAPRef,
	})

	writeJSON(w, 200, map[string]string{"status": "transferred", "psap_reference": in.PSAPRef})
}

// initConnect registers Connect-related routes.
func initConnect() {
	// Called from main() to register routes — actual registration in main.go
	log.Printf("connect: Lex + PSAP handlers ready")
}

// Ensure unused imports are retained
var _ = strings.TrimPrefix("", "")
