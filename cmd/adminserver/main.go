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
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("[INFO] Admin server using database: %s", cfg.Database.Path)

	// Initialize database
	db, err := database.New(cfg.Database.Path, cfg.Database.Domain)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Start admin interface
	adminServer, err := admin.New(db)
	if err != nil {
		log.Fatalf("Failed to initialize admin server: %v", err)
	}

	go func() {
		adminAddr := fmt.Sprintf("%s:%d", cfg.Server.Admin.Host, cfg.Server.Admin.Port)
		if err := adminServer.Start(adminAddr); err != nil {
			log.Printf("Admin server error: %v", err)
			stop()
		}
	}()
	log.Printf("Started admin server on %s:%d", cfg.Server.Admin.Host, cfg.Server.Admin.Port)

	// Keep the application running until we receive an interrupt signal
	<-ctx.Done()
	log.Println("Shutting down admin server...")
}
