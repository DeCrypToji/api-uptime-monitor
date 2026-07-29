package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

var db *sql.DB

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
}

func main() {
	var err error
	db, err = initDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("✅ Database connected successfully")

	if os.Getenv("RUN_SCHEDULER") != "false" {
		go StartScheduler()
	}
	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"time":   time.Now().Unix(),
		})
	})

	v1 := router.Group("/api/v1")
	{
		v1.POST("/auth/signup", signupHandler)
		v1.POST("/auth/login", loginHandler)
		v1.POST("/auth/logout", authMiddleware(), logoutHandler)

		v1.GET("/endpoints", authMiddleware(), getEndpointsHandler)
		v1.POST("/endpoints", authMiddleware(), createEndpointHandler)
		v1.GET("/endpoints/:id", authMiddleware(), getEndpointHandler)
		v1.PATCH("/endpoints/:id", authMiddleware(), updateEndpointHandler)
		v1.DELETE("/endpoints/:id", authMiddleware(), deleteEndpointHandler)

		v1.POST("/endpoints/:id/check", authMiddleware(), checkEndpointHealthHandler)
		v1.GET("/endpoints/:id/health", authMiddleware(), getHealthChecksHandler)

		v1.GET("/status/:slug", getStatusPageHandler)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("🚀 Server starting on port %s\n", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() (*sql.DB, error) {
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "require" // RDS enforces TLS; local dev can override with DB_SSLMODE=disable
	}

	dsn := fmt.Sprintf(
		"user=%s password=%s dbname=%s host=%s port=%s sslmode=%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		sslmode,
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	return db, nil
}
