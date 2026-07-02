package main

import (
	"log"
	"os"
	"strconv"
	"time"
)

// StartScheduler runs health checks on all active endpoints at a fixed interval.
// It runs one pass immediately, then repeats every CHECK_INTERVAL_SECONDS.
func StartScheduler() {
	interval := getCheckInterval()
	log.Printf("⏱️  Scheduler started — interval %s", interval)

	// Immediate first pass so we don't wait a full interval for the first check.
	runCheckPass()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		runCheckPass()
	}
}

// getCheckInterval reads CHECK_INTERVAL_SECONDS from the environment,
// defaulting to 60 seconds if unset or invalid.
func getCheckInterval() time.Duration {
	seconds := 60
	if v := os.Getenv("CHECK_INTERVAL_SECONDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
			seconds = parsed
		}
	}
	return time.Duration(seconds) * time.Second
}

// runCheckPass queries every active endpoint and runs a health check on each.
func runCheckPass() {
	rows, err := db.Query(`
		SELECT id, user_id, url, http_method, expected_status_code, name
		FROM endpoints
		WHERE is_active = true
	`)
	if err != nil {
		log.Printf("Scheduler: failed to query endpoints: %v", err)
		return
	}
	defer rows.Close()

	// Read everything first, THEN run checks — don't hold the read cursor
	// open while doing INSERT/UPDATE writes.
	type job struct {
		userID   string
		endpoint Endpoint
	}
	var jobs []job

	for rows.Next() {
		var userID string
		var ep Endpoint
		if err := rows.Scan(&ep.ID, &userID, &ep.URL, &ep.HTTPMethod, &ep.ExpectedStatusCode, &ep.Name); err != nil {
			log.Printf("Scheduler: scan error: %v", err)
			continue
		}
		jobs = append(jobs, job{userID: userID, endpoint: ep})
	}

	if len(jobs) == 0 {
		log.Printf("Scheduler: no active endpoints to check")
		return
	}

	log.Printf("Scheduler: checking %d endpoint(s)", len(jobs))

	for _, j := range jobs {
		if err := CheckEndpointHealth(j.userID, j.endpoint); err != nil {
			log.Printf("Scheduler: check FAILED for %s: %v", j.endpoint.URL, err)
			continue
		}
		log.Printf("Scheduler: checked %s", j.endpoint.URL)
	}
}
