// Amazon Pinpoint SMS integration (#36) — scaffolding.
// Production path replaces direct SNS with Pinpoint for delivery tracking,
// templates, and opt-out handling. The SNS fallback is in main.go:sendSMS().
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// Pinpoint scaffolding — uses env vars for future configuration.
// When PINPOINT_APP_ID is set + aws-sdk-go-v2/service/pinpoint is added,
// replace snsClient.Publish with Pinpoint SendMessages.
var (
	pinpointAppID   = os.Getenv("PINPOINT_APP_ID")
	pinpointOrigNum = getenv("PINPOINT_ORIG_NUMBER", "+33800000000")
)

// pinpointTemplates maps journey steps to SMS templates.
var pinpointTemplates = map[string]string{
	"dispatch":  "Insucar: %s assigned — %s (%s), ETA ~%d min. Track: %s",
	"arriving":  "Insucar: %s is arriving in ~5 minutes. %s (%s)",
	"arrived":   "Insucar: %s has arrived at your location. %s",
	"resolved":  "Insucar: Case %s resolved. Thank you for choosing Insucar.",
	"rate":      "Insucar: How was your experience? Rate: %s",
}

func initPinpoint() {
	if pinpointAppID == "" {
		log.Printf("pinpoint: PINPOINT_APP_ID not set — SNS fallback active")
		return
	}
	log.Printf("pinpoint: scaffolding ready (app=%s, origin=%s) — add aws-sdk-go-v2/service/pinpoint to enable",
		pinpointAppID[:min(8, len(pinpointAppID))], pinpointOrigNum)
}

// handlePinpointCallback receives SMS delivery status updates from Pinpoint SNS topic.
// Accepts: event_type (_SMS.SUCCESS/_SMS.FAILURE/_SMS.OPTOUT), destination, messageId
func handlePinpointCallback(w http.ResponseWriter, r *http.Request) {
	var data map[string]any
	if json.NewDecoder(r.Body).Decode(&data) != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid"})
		return
	}

	eventType, _ := data["event_type"].(string)
	phone, _ := data["destination"].(string)

	statusMap := map[string]string{
		"_SMS.SUCCESS": "delivered",
		"_SMS.FAILURE": "failed",
		"_SMS.OPTOUT":  "opted_out",
	}
	status, ok := statusMap[eventType]
	if !ok {
		status = "unknown"
	}

	if phone != "" {
		db.Exec(r.Context(),
			`UPDATE notifications SET status=$1, updated_at=now() WHERE recipient=$2 AND channel='sms' AND status='sent'`,
			status, phone)
	}

	log.Printf(`{"stream":"pinpoint","event":"callback","type":%q,"dest":%q,"status":%q}`,
		eventType, phone[:min(6, len(phone))], status)

	writeJSON(w, 200, map[string]string{"status": "acknowledged"})
}

// formatPinpointTemplate formats a template with the given args, adding opt-out notice.
func formatPinpointTemplate(step string, args ...interface{}) string {
	tmpl, ok := pinpointTemplates[step]
	if !ok {
		tmpl = pinpointTemplates["dispatch"]
	}
	msg := fmt.Sprintf(tmpl, args...)
	msg += "\n\nReply STOP to opt out."
	return msg
}

// Ensure time import retained
var _ = time.Now
