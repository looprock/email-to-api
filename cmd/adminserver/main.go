package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/looprock/email-to-api/internal/admin"
	"github.com/looprock/email-to-api/internal/config"
	"github.com/looprock/email-to-api/internal/database"
)

func main() {
	// Create context that listens for the interrupt signal from the OS
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	dbConfig := &database.Config{
		Driver:     cfg.Database.Driver,
		DSN:        cfg.Database.Path, // For SQLite
		MigrateURL: "file://migrations",
		Domain:     cfg.MailServer.Domain,
	}
	if cfg.Database.Driver == "postgres" {
		dbConfig.DSN = fmt.Sprintf("host=%s port=%d user=%s dbname=%s password=%s sslmode=%s",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
			cfg.Database.Name, cfg.Database.Password, cfg.Database.SSLMode)
		log.Printf("[INFO] Admin server using PostgreSQL database: %s@%s:%d/%s", 
			cfg.Database.User, cfg.Database.Host, cfg.Database.Port, cfg.Database.Name)
	} else {
		log.Printf("[INFO] Admin server using SQLite database: %s", cfg.Database.Path)
	}

	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Start admin interface
	adminServer, err := admin.New(db, cfg)
	if err != nil {
		log.Fatalf("Failed to initialize admin server: %v", err)
	}

	go func() {
		adminAddr := fmt.Sprintf("%s:%d", cfg.AdminServer.Host, cfg.AdminServer.Port)
		if err := adminServer.Start(adminAddr); err != nil {
			log.Printf("Admin server error: %v", err)
			stop()
		}
	}()
	log.Printf("Started admin server on %s:%d", cfg.AdminServer.Host, cfg.AdminServer.Port)

	// Keep the application running until we receive an interrupt signal
	<-ctx.Done()
	log.Println("Shutting down admin server...")
}
