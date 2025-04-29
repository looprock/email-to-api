package main

import (
	"example-api/internal/api"
	"example-api/internal/database"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Configure logging
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	log.SetPrefix("[example-api] ")

	log.Println("Starting example API server...")

	// Get configuration from environment variables
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/emails.db"
	}
	log.Printf("Using database at: %s", dbPath)

	apiToken := os.Getenv("API_TOKEN")
	if apiToken == "" {
		log.Fatal("API_TOKEN environment variable is required")
	}
	log.Println("API token configured successfully")

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// Ensure data directory exists
	if err := os.MkdirAll("./data", 0755); err != nil {
		log.Printf("Warning: Failed to create data directory: %v", err)
	}

	// Initialize database
	log.Println("Initializing database...")
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	log.Println("Database initialized successfully")

	// Initialize router and handler
	gin.SetMode(gin.ReleaseMode)
	router := gin.New() // Use New() instead of Default() for custom logging

	// Add custom logging middleware
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return fmt.Sprintf("[example-api] %s | %d | %s | %s %s | %s\n",
			param.TimeStamp.Format(time.RFC3339),
			param.StatusCode,
			param.Latency,
			param.Method,
			param.Path,
			param.ErrorMessage,
		)
	}))
	router.Use(gin.Recovery())

	handler := api.New(db)

	// Set up routes
	router.POST("/api/emails", api.AuthMiddleware(apiToken), handler.HandleEmailReceive)
	router.GET("/api/emails/:id", handler.HandleGetEmailByID)
	router.GET("/api/emails", handler.HandleGetEmailsByTag)

	// Start server
	address := ":" + port
	log.Printf("Server initialization complete. Listening on %s", address)
	if err := router.Run(address); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
