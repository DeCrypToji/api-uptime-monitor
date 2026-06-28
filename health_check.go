package main

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type HealthCheckResult struct {
	StatusCode     int
	ResponseTimeMs int64
	IsHealthy      bool
	ErrorMessage   string
}

func PerformHealthCheck(endpoint Endpoint) (*HealthCheckResult, error) {
	startTime := time.Now()
	result := &HealthCheckResult{}

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest(endpoint.HTTPMethod, endpoint.URL, nil)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to create request: %v", err)
		result.IsHealthy = false
		result.ResponseTimeMs = int64(time.Since(startTime).Milliseconds())
		return result, nil
	}

	resp, err := client.Do(req)
	responseTime := time.Since(startTime)

	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Request failed: %v", err)
		result.IsHealthy = false
		result.ResponseTimeMs = responseTime.Milliseconds()
		return result, nil
	}
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		result.ErrorMessage = fmt.Sprintf("Failed to read response: %v", err)
		result.IsHealthy = false
		result.ResponseTimeMs = responseTime.Milliseconds()
		return result, nil
	}

	result.StatusCode = resp.StatusCode
	result.ResponseTimeMs = responseTime.Milliseconds()
	result.IsHealthy = (resp.StatusCode == endpoint.ExpectedStatusCode)

	return result, nil
}

func SaveHealthCheck(userID, endpointID string, result *HealthCheckResult) error {
	query := `
		INSERT INTO health_checks (
			id,
			user_id,
			endpoint_id,
			status_code,
			response_time_ms,
			is_healthy,
			error_message,
			checked_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
	`

	_, err := db.Exec(
		query,
		uuid.New().String(),
		userID,
		endpointID,
		result.StatusCode,
		result.ResponseTimeMs,
		result.IsHealthy,
		result.ErrorMessage,
	)

	return err
}

func UpdateEndpointStatus(endpointID string, result *HealthCheckResult) error {
	query := `
		UPDATE endpoints
		SET 
			last_is_healthy = $1,
			last_response_time_ms = $2,
			last_checked_at = NOW(),
			last_status_code = $3,
			updated_at = NOW()
		WHERE id = $4
	`

	_, err := db.Exec(
		query,
		result.IsHealthy,
		result.ResponseTimeMs,
		result.StatusCode,
		endpointID,
	)

	return err
}

func CheckEndpointHealth(userID string, endpoint Endpoint) error {
	result, err := PerformHealthCheck(endpoint)
	if err != nil {
		return fmt.Errorf("health check failed: %v", err)
	}

	if err := SaveHealthCheck(userID, endpoint.ID, result); err != nil {
		return fmt.Errorf("failed to save health check: %v", err)
	}

	if err := UpdateEndpointStatus(endpoint.ID, result); err != nil {
		return fmt.Errorf("failed to update endpoint status: %v", err)
	}

	return nil
}
