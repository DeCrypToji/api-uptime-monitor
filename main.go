package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/joho/godotenv"
)

var db *sql.DB

func init() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
}

func main() {
	// Initialize database
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Test database connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("✅ Database connected successfully")

	// Initialize Gin router
	router := gin.Default()

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Unix(),
		})
	})

	// API v1 routes
	v1 := router.Group("/api/v1")
	{
		// Auth routes
		v1.POST("/auth/signup", signupHandler)
		v1.POST("/auth/login", loginHandler)
		v1.POST("/auth/logout", authMiddleware(), logoutHandler)

		// Endpoints routes (protected)
		v1.GET("/endpoints", authMiddleware(), getEndpointsHandler)
		v1.POST("/endpoints", authMiddleware(), createEndpointHandler)
		v1.GET("/endpoints/:id", authMiddleware(), getEndpointHandler)
		v1.PATCH("/endpoints/:id", authMiddleware(), updateEndpointHandler)
		v1.DELETE("/endpoints/:id", authMiddleware(), deleteEndpointHandler)

		// Health checks routes
		v1.GET("/endpoints/:id/health", authMiddleware(), getHealthChecksHandler)

		// Status pages routes
		v1.GET("/status/:slug", getStatusPageHandler) // public
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("🚀 Server starting on port %s\n", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// initDB initializes the PostgreSQL database connection
func initDB() (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"user=%s password=%s dbname=%s host=%s port=%s sslmode=disable",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	// Connection pool settings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}

// ============================================================================
// PLACEHOLDER HANDLERS (To be implemented)
// ============================================================================

func signupHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "signup endpoint",
		"TODO":    "Implement user registration",
	})
}

func loginHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "login endpoint",
		"TODO":    "Implement user login with JWT",
	})
}

func logoutHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "logout endpoint",
	})
}

func getEndpointsHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "get endpoints",
		"TODO":    "Fetch user's endpoints",
	})
}

func createEndpointHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "create endpoint",
		"TODO":    "Create new endpoint",
	})
}

func getEndpointHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "get endpoint",
		"TODO":    "Fetch single endpoint",
	})
}

func updateEndpointHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "update endpoint",
		"TODO":    "Update endpoint settings",
	})
}

func deleteEndpointHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "delete endpoint",
		"TODO":    "Delete endpoint",
	})
}

func getHealthChecksHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "get health checks",
		"TODO":    "Fetch health check history",
	})
}

func getStatusPageHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"message": "get status page",
		"TODO":    "Fetch public status page",
	})
}
