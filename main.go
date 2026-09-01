package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var db *sql.DB

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests by method, path, and status code",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	schedulerRunsTotal = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "scheduler_runs_total",
			Help: "Total number of scheduler check cycles executed",
		},
	)

	healthCheckResultsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "health_check_results_total",
			Help: "Total health check results by status (up/down)",
		},
		[]string{"status"},
	)
)

func init() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(schedulerRunsTotal)
	prometheus.MustRegister(healthCheckResultsTotal)
}

func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start).Seconds()
		status := fmt.Sprintf("%d", c.Writer.Status())
		httpRequestsTotal.WithLabelValues(c.Request.Method, c.FullPath(), status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, c.FullPath()).Observe(duration)
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
	router.Use(metricsMiddleware())                       // record metrics on every request
	router.GET("/metrics", gin.WrapH(promhttp.Handler())) // expose metrics for Prometheus to scrape
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

	log.Printf("🚀 Server starting on port %s\n", port) // #nosec G706 -- port is operator-set env var, not user input
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func initDB() (*sql.DB, error) {
	sslmode := os.Getenv("DB_SSLMODE")
	if sslmode == "" {
		sslmode = "require"
	}

	password, err := getDBPassword() // <-- fetch via env-var-or-Secrets-Manager
	if err != nil {
		return nil, fmt.Errorf("getting DB password: %w", err)
	}

	dsn := fmt.Sprintf(
		"user=%s password=%s dbname=%s host=%s port=%s sslmode=%s",
		os.Getenv("DB_USER"),
		password, // <-- use the fetched value
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

func getDBPassword() (string, error) {
	if pw := os.Getenv("DB_PASSWORD"); pw != "" {
		log.Println("using DB password from DB_PASSWORD env var") // <-- ADD (env-var path)
		return pw, nil
	}

	secretName := os.Getenv("DB_SECRET_NAME")
	if secretName == "" {
		return "", fmt.Errorf("neither DB_PASSWORD nor DB_SECRET_NAME is set")
	}

	log.Printf("fetching DB password from Secrets Manager (secret: %s)", secretName) // #nosec G706 -- secretName is operator-set env var, not user input

	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		return "", fmt.Errorf("loading AWS config: %w", err)
	}

	client := secretsmanager.NewFromConfig(cfg)
	out, err := client.GetSecretValue(context.TODO(), &secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return "", fmt.Errorf("fetching secret %s: %w", secretName, err)
	}

	var secret struct {
		Password string `json:"password"`
	}
	if err := json.Unmarshal([]byte(*out.SecretString), &secret); err != nil {
		return "", fmt.Errorf("parsing secret JSON: %w", err)
	}

	return secret.Password, nil
}
