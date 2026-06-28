package main

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type SignupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type CreateEndpointRequest struct {
	Name               string `json:"name" binding:"required"`
	URL                string `json:"url" binding:"required"`
	HTTPMethod         string `json:"http_method" binding:"required"`
	ExpectedStatusCode int    `json:"expected_status_code" binding:"required"`
}

type UpdateEndpointRequest struct {
	Name               string `json:"name"`
	HTTPMethod         string `json:"http_method"`
	ExpectedStatusCode int    `json:"expected_status_code"`
}

func signupHandler(c *gin.Context) {
	var req SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), 10)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to hash password"})
		return
	}

	var userID string
	query := `INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id`
	err = db.QueryRow(query, req.Email, string(hashedPassword)).Scan(&userID)
	if err != nil {
		c.JSON(400, gin.H{"error": "email already exists"})
		return
	}

	token := NewJWT(userID, req.Email)
	c.JSON(201, gin.H{"user_id": userID, "jwt_token": token})
}

func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var userID, passwordHash string
	query := `SELECT id, password_hash FROM users WHERE email = $1`
	err := db.QueryRow(query, req.Email).Scan(&userID, &passwordHash)
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid email or password"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password))
	if err != nil {
		c.JSON(401, gin.H{"error": "invalid email or password"})
		return
	}

	token := NewJWT(userID, req.Email)
	c.JSON(200, gin.H{"user_id": userID, "jwt_token": token})
}

func logoutHandler(c *gin.Context) {
	c.JSON(200, gin.H{"message": "logged out"})
}

func createEndpointHandler(c *gin.Context) {
	userID := GetUserID(c)

	var req CreateEndpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var endpointID string
	query := `
		INSERT INTO endpoints (user_id, name, url, http_method, expected_status_code, is_active)
		VALUES ($1, $2, $3, $4, $5, true)
		RETURNING id
	`
	err := db.QueryRow(query, userID, req.Name, req.URL, req.HTTPMethod, req.ExpectedStatusCode).Scan(&endpointID)
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to create endpoint"})
		return
	}

	c.JSON(201, gin.H{"id": endpointID})
}

func getEndpointHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	var id, name, url, method string
	var expectedStatus, statusCode, responseTime int
	var isHealthy bool

	query := `
		SELECT id, name, url, http_method, expected_status_code, last_is_healthy, last_response_time_ms, last_status_code
		FROM endpoints
		WHERE id = $1 AND user_id = $2
	`
	err := db.QueryRow(query, endpointID, userID).Scan(&id, &name, &url, &method, &expectedStatus, &isHealthy, &responseTime, &statusCode)
	if err != nil {
		c.JSON(404, gin.H{"error": "endpoint not found"})
		return
	}

	c.JSON(200, gin.H{
		"id":                    id,
		"name":                  name,
		"url":                   url,
		"http_method":           method,
		"expected_status_code":  expectedStatus,
		"last_is_healthy":       isHealthy,
		"last_response_time_ms": responseTime,
		"last_status_code":      statusCode,
	})
}

func updateEndpointHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	var req UpdateEndpointRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	query := `
		UPDATE endpoints
		SET name = COALESCE(NULLIF($1, ''), name),
		    http_method = COALESCE(NULLIF($2, ''), http_method),
		    expected_status_code = COALESCE(NULLIF($3, 0), expected_status_code),
		    updated_at = NOW()
		WHERE id = $4 AND user_id = $5
	`
	_, err := db.Exec(query, req.Name, req.HTTPMethod, req.ExpectedStatusCode, endpointID, userID)
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to update endpoint"})
		return
	}

	c.JSON(200, gin.H{"message": "endpoint updated"})
}

func deleteEndpointHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	query := `UPDATE endpoints SET is_active = false, updated_at = NOW() WHERE id = $1 AND user_id = $2`
	_, err := db.Exec(query, endpointID, userID)
	if err != nil {
		c.JSON(400, gin.H{"error": "failed to delete endpoint"})
		return
	}

	c.JSON(200, gin.H{"message": "endpoint deleted"})
}

func getHealthChecksHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	rows, err := db.Query(`
		SELECT hc.id, hc.status_code, hc.response_time_ms, hc.is_healthy, hc.error_message, hc.checked_at
		FROM health_checks hc
		JOIN endpoints e ON hc.endpoint_id = e.id
		WHERE e.id = $1 AND e.user_id = $2
		ORDER BY hc.checked_at DESC
		LIMIT 100
	`, endpointID, userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to fetch health checks"})
		return
	}
	defer rows.Close()

	var checks []gin.H
	for rows.Next() {
		var id, errorMsg string
		var statusCode, responseTime int
		var isHealthy bool
		var checkedAt time.Time

		err := rows.Scan(&id, &statusCode, &responseTime, &isHealthy, &errorMsg, &checkedAt)
		if err != nil {
			continue
		}

		checks = append(checks, gin.H{
			"id":               id,
			"status_code":      statusCode,
			"response_time_ms": responseTime,
			"is_healthy":       isHealthy,
			"error_message":    errorMsg,
			"checked_at":       checkedAt,
		})
	}

	c.JSON(200, checks)
}

func getStatusPageHandler(c *gin.Context) {
	slug := c.Param("slug")

	var sslug, name string
	query := `SELECT slug, name FROM status_pages WHERE slug = $1`
	err := db.QueryRow(query, slug).Scan(&sslug, &name)
	if err != nil {
		c.JSON(404, gin.H{"error": "status page not found"})
		return
	}

	c.JSON(200, gin.H{"slug": sslug, "name": name})
}

func checkEndpointHealthHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	var endpoint Endpoint
	query := `SELECT id, url, http_method, expected_status_code, name FROM endpoints WHERE id = $1 AND user_id = $2`
	err := db.QueryRow(query, endpointID, userID).Scan(&endpoint.ID, &endpoint.URL, &endpoint.HTTPMethod, &endpoint.ExpectedStatusCode, &endpoint.Name)
	if err != nil {
		c.JSON(404, gin.H{"error": "endpoint not found"})
		return
	}

	err = CheckEndpointHealth(userID, endpoint)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "health check completed"})
}
func getEndpointsHandler(c *gin.Context) {
	userID := GetUserID(c)
	log.Printf("Fetching endpoints for user: %s", userID)

	rows, err := db.Query(`
		SELECT id, name, url, http_method, expected_status_code, 
		       last_is_healthy, last_response_time_ms, last_checked_at, last_status_code
		FROM endpoints
		WHERE user_id = $1 AND is_active = true
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		log.Printf("Query error: %v", err)
		c.JSON(500, gin.H{"error": "failed to fetch endpoints"})
		return
	}
	defer rows.Close()

	var endpoints []gin.H
	for rows.Next() {
		var id, name, url, method string
		var expectedStatus int
		var isHealthy *bool
		var responseTime *int
		var lastChecked *time.Time
		var statusCode *int

		err := rows.Scan(&id, &name, &url, &method, &expectedStatus, &isHealthy, &responseTime, &lastChecked, &statusCode)
		if err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}

		endpoints = append(endpoints, gin.H{
			"id":                    id,
			"name":                  name,
			"url":                   url,
			"http_method":           method,
			"expected_status_code":  expectedStatus,
			"last_is_healthy":       isHealthy,
			"last_response_time_ms": responseTime,
			"last_checked_at":       lastChecked,
			"last_status_code":      statusCode,
		})
	}

	log.Printf("Returning %d endpoints", len(endpoints))
	c.JSON(200, endpoints)
}
