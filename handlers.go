package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type SignupRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type AuthResponse struct {
	UserID   string `json:"user_id"`
	JWTToken string `json:"jwt_token"`
}

func signupHandler(c *gin.Context) {
	var req SignupRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to hash password"})
		return
	}

	var userID string
	err = db.QueryRow(
		"INSERT INTO users (email, password_hash, created_at) VALUES ($1, $2, NOW()) RETURNING id",
		req.Email, string(hashedPassword),
	).Scan(&userID)

	if err != nil {
		if err.Error() == "pq: duplicate key value violates unique constraint \"users_email_key\"" {
			c.JSON(http.StatusConflict, gin.H{"error": "email already registered"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create user"})
		}
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		UserID: userID,
		Email:  req.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, AuthResponse{
		UserID:   userID,
		JWTToken: tokenString,
	})
}

func loginHandler(c *gin.Context) {
	var req LoginRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var userID, hashedPassword string
	err := db.QueryRow(
		"SELECT id, password_hash FROM users WHERE email = $1",
		req.Email,
	).Scan(&userID, &hashedPassword)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password))
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
		UserID: userID,
		Email:  req.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	})

	tokenString, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, AuthResponse{
		UserID:   userID,
		JWTToken: tokenString,
	})
}

func logoutHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "logged out successfully"})
}

// ============================================================================
// ENDPOINTS HANDLERS
// ============================================================================

type Endpoint struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	URL                string `json:"url"`
	HTTPMethod         string `json:"http_method"`
	ExpectedStatusCode int    `json:"expected_status_code"`
	LastIsHealthy      bool   `json:"last_is_healthy"`
	LastResponseTimeMs int    `json:"last_response_time_ms"`
	LastCheckedAt      string `json:"last_checked_at"`
}

type CreateEndpointRequest struct {
	Name               string `json:"name"`
	URL                string `json:"url" binding:"required,url"`
	HTTPMethod         string `json:"http_method"`
	ExpectedStatusCode int    `json:"expected_status_code"`
}

func getEndpointsHandler(c *gin.Context) {
	userID := GetUserID(c)
	log.Printf("Fetching endpoints for user: %s", userID)

	rows, err := db.Query(
		`SELECT id, COALESCE(name, ''), url, COALESCE(http_method, 'GET'), COALESCE(expected_status_code, 200), 
		        COALESCE(last_is_healthy, false), COALESCE(last_response_time_ms, 0), 
		        COALESCE(last_checked_at::text, '') 
		 FROM endpoints 
		 WHERE user_id = $1 AND is_active = true 
		 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		log.Printf("Error querying endpoints: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch endpoints"})
		return
	}
	defer rows.Close()

	endpoints := []Endpoint{}
	for rows.Next() {
		var ep Endpoint
		if err := rows.Scan(&ep.ID, &ep.Name, &ep.URL, &ep.HTTPMethod, &ep.ExpectedStatusCode, &ep.LastIsHealthy, &ep.LastResponseTimeMs, &ep.LastCheckedAt); err != nil {
			log.Printf("Error scanning endpoint: %v", err)
			continue
		}
		endpoints = append(endpoints, ep)
	}

	if endpoints == nil {
		endpoints = []Endpoint{}
	}

	log.Printf("Returning %d endpoints", len(endpoints))
	c.JSON(http.StatusOK, endpoints)
}

func createEndpointHandler(c *gin.Context) {
	userID := GetUserID(c)

	var req CreateEndpointRequest
	if err := c.BindJSON(&req); err != nil {
		log.Printf("Bind error: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	if req.HTTPMethod == "" {
		req.HTTPMethod = "GET"
	}
	if req.ExpectedStatusCode == 0 {
		req.ExpectedStatusCode = 200
	}

	log.Printf("Creating endpoint for user %s: url=%s, method=%s, status=%d", userID, req.URL, req.HTTPMethod, req.ExpectedStatusCode)

	var endpointID string
	err := db.QueryRow(
		"INSERT INTO endpoints (user_id, name, url, http_method, expected_status_code, is_active, created_at, updated_at) VALUES ($1, $2, $3, $4, $5, true, NOW(), NOW()) RETURNING id",
		userID, req.Name, req.URL, req.HTTPMethod, req.ExpectedStatusCode,
	).Scan(&endpointID)

	if err != nil {
		log.Printf("Database error creating endpoint: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create endpoint: " + err.Error()})
		return
	}

	log.Printf("Endpoint created successfully: %s", endpointID)
	c.JSON(http.StatusCreated, gin.H{"id": endpointID})
}

func getEndpointHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	var ep Endpoint
	err := db.QueryRow(
		`SELECT id, COALESCE(name, ''), url, COALESCE(http_method, 'GET'), COALESCE(expected_status_code, 200),
		        COALESCE(last_is_healthy, false), COALESCE(last_response_time_ms, 0), 
		        COALESCE(last_checked_at::text, '')
		 FROM endpoints 
		 WHERE id = $1 AND user_id = $2 AND is_active = true`,
		endpointID, userID,
	).Scan(&ep.ID, &ep.Name, &ep.URL, &ep.HTTPMethod, &ep.ExpectedStatusCode, &ep.LastIsHealthy, &ep.LastResponseTimeMs, &ep.LastCheckedAt)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
		return
	}
	if err != nil {
		log.Printf("Error fetching endpoint: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch endpoint"})
		return
	}

	c.JSON(http.StatusOK, ep)
}

func updateEndpointHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	var req CreateEndpointRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	_, err := db.Exec(
		"UPDATE endpoints SET name = $1, url = $2, http_method = $3, expected_status_code = $4, updated_at = NOW() WHERE id = $5 AND user_id = $6",
		req.Name, req.URL, req.HTTPMethod, req.ExpectedStatusCode, endpointID, userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update endpoint"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "endpoint updated"})
}

func deleteEndpointHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	_, err := db.Exec(
		"UPDATE endpoints SET is_active = false, updated_at = NOW() WHERE id = $1 AND user_id = $2",
		endpointID, userID,
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete endpoint"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "endpoint deleted"})
}

func getHealthChecksHandler(c *gin.Context) {
	userID := GetUserID(c)
	endpointID := c.Param("id")

	rows, err := db.Query(
		"SELECT status_code, response_time_ms, is_healthy, error_message, checked_at FROM health_checks WHERE endpoint_id = $1 AND endpoint_id IN (SELECT id FROM endpoints WHERE user_id = $2) ORDER BY checked_at DESC LIMIT 100",
		endpointID, userID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch health checks"})
		return
	}
	defer rows.Close()

	checks := []gin.H{}
	for rows.Next() {
		var statusCode int
		var responseTime int
		var isHealthy bool
		var errorMsg sql.NullString
		var checkedAt string

		if err := rows.Scan(&statusCode, &responseTime, &isHealthy, &errorMsg, &checkedAt); err != nil {
			continue
		}

		checks = append(checks, gin.H{
			"status_code":      statusCode,
			"response_time_ms": responseTime,
			"is_healthy":       isHealthy,
			"error_message":    errorMsg.String,
			"checked_at":       checkedAt,
		})
	}

	c.JSON(http.StatusOK, checks)
}

func getStatusPageHandler(c *gin.Context) {
	slug := c.Param("slug")

	var statusPageID string
	err := db.QueryRow(
		"SELECT id FROM status_pages WHERE slug = $1",
		slug,
	).Scan(&statusPageID)

	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "status page not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch status page"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "status page", "id": statusPageID})
}
