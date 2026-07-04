package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// determineAlertType is a pure function: given the previous health state and the
// new one, it returns the alert event_type to fire, or "" for no transition.
//
// oldHealthy == nil means the endpoint has never been checked before.
//
//	nil   -> down     : alert (it's genuinely down on first observation)
//	nil   -> healthy  : silent (no need to announce "it came up" on boot)
//	true  -> false    : alert endpoint_down
//	false -> true      : alert endpoint_recovered
//	same  -> same     : silent (this is what prevents alert spam)
func determineAlertType(oldHealthy *bool, newHealthy bool) string {
	if oldHealthy == nil {
		if !newHealthy {
			return "endpoint_down"
		}
		return ""
	}
	if *oldHealthy && !newHealthy {
		return "endpoint_down"
	}
	if !*oldHealthy && newHealthy {
		return "endpoint_recovered"
	}
	return ""
}

// buildAlertMessage produces the human-readable Slack text for an event.
func buildAlertMessage(endpoint Endpoint, eventType string, result *HealthCheckResult) string {
	name := endpoint.Name
	if name == "" {
		name = endpoint.URL
	}

	if eventType == "endpoint_down" {
		if result.ErrorMessage != "" {
			return fmt.Sprintf("🔴 DOWN: %s is unreachable — %s", name, result.ErrorMessage)
		}
		return fmt.Sprintf("🔴 DOWN: %s returned status %d (expected %d)",
			name, result.StatusCode, endpoint.ExpectedStatusCode)
	}

	return fmt.Sprintf("🟢 RECOVERED: %s is back — status %d in %dms",
		name, result.StatusCode, result.ResponseTimeMs)
}

// getPreviousHealth reads last_is_healthy BEFORE it gets overwritten by this check.
// Returns a *bool: nil means the endpoint has never been checked.
func getPreviousHealth(endpointID string) (*bool, error) {
	var isHealthy *bool
	err := db.QueryRow(`SELECT last_is_healthy FROM endpoints WHERE id = $1`, endpointID).Scan(&isHealthy)
	if err != nil {
		return nil, err
	}
	return isHealthy, nil
}

// getUserWebhook reads the user's Slack webhook URL (nullable).
func getUserWebhook(userID string) (*string, error) {
	var webhook *string
	err := db.QueryRow(`SELECT slack_webhook_url FROM users WHERE id = $1`, userID).Scan(&webhook)
	if err != nil {
		return nil, err
	}
	return webhook, nil
}

// recordAlertEvent inserts a pending (is_sent=false) alert row and returns its id.
// We record FIRST so there's an audit trail even if the Slack send later fails.
func recordAlertEvent(userID, endpointID, eventType, message string) (string, error) {
	var id string
	err := db.QueryRow(`
		INSERT INTO alert_events (user_id, endpoint_id, event_type, message, is_sent)
		VALUES ($1, $2, $3, $4, false)
		RETURNING id
	`, userID, endpointID, eventType, message).Scan(&id)
	return id, err
}

// markAlertSent flips a recorded alert to sent once Slack accepts it.
func markAlertSent(alertID string) error {
	_, err := db.Exec(`UPDATE alert_events SET is_sent = true, sent_at = NOW() WHERE id = $1`, alertID)
	return err
}

// sendSlackMessage POSTs a simple {"text": ...} payload. Works with both real
// Slack incoming webhooks and webhook.site test URLs.
func sendSlackMessage(webhookURL, message string) error {
	payload := map[string]string{"text": message}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(webhookURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// MaybeSendAlert is the orchestrator. It NEVER returns an error, by design:
// a failed alert must never fail the health check that produced it. Every
// failure path is logged and swallowed.
func MaybeSendAlert(userID string, endpoint Endpoint, oldHealthy *bool, result *HealthCheckResult) {
	eventType := determineAlertType(oldHealthy, result.IsHealthy)
	if eventType == "" {
		return // no state change, nothing to alert
	}

	message := buildAlertMessage(endpoint, eventType, result)

	// Record first (is_sent=false) — audit trail regardless of send outcome.
	alertID, err := recordAlertEvent(userID, endpoint.ID, eventType, message)
	if err != nil {
		log.Printf("Alert: failed to record alert_event for %s: %v", endpoint.URL, err)
		return
	}

	webhook, err := getUserWebhook(userID)
	if err != nil {
		log.Printf("Alert: failed to read webhook for user %s: %v", userID, err)
		return
	}
	if webhook == nil || *webhook == "" {
		log.Printf("Alert: %s recorded for %s but no webhook configured", eventType, endpoint.URL)
		return
	}

	if err := sendSlackMessage(*webhook, message); err != nil {
		log.Printf("Alert: failed to send Slack for %s: %v", endpoint.URL, err)
		return // row stays is_sent=false; a retry job could pick it up later
	}

	if err := markAlertSent(alertID); err != nil {
		log.Printf("Alert: sent but failed to mark sent for %s: %v", endpoint.URL, err)
		return
	}

	log.Printf("Alert: %s notification sent for %s", eventType, endpoint.URL)
}
