package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/looprock/email-to-api/internal/config"
	"github.com/looprock/email-to-api/internal/database"
	"github.com/looprock/email-to-api/internal/email"
)

func main() {
	// Configure logging
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.SetPrefix("[mailserver] ")

	// Create context that listens for the interrupt signal from the OS
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Log the database path being used
	log.Printf("Using database at path: %s", cfg.Database.Path)

	// Initialize database
	db, err := database.New(cfg.Database.Path, cfg.Database.Domain)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize email processor
	processor := email.New(db, email.ProcessorConfig{
		MaxSize:       cfg.Email.MaxSize,
		RetryAttempts: cfg.Email.RetryAttempts,
		RetryDelay:    cfg.Email.RetryDelay,
	})

	// Start the appropriate email receiver based on configuration
	switch cfg.Email.ReceiveMethod {
	case "smtp":
		go func() {
			if err := email.StartSMTPServer(processor, cfg.Email.SMTPHost, cfg.Email.SMTPPort); err != nil {
				log.Printf("SMTP server error: %v", err)
				stop()
			}
		}()
		log.Printf("Started SMTP server on %s:%d", cfg.Email.SMTPHost, cfg.Email.SMTPPort)

	case "webhook":
		// TODO: Implement webhook receiver
		log.Fatal("Webhook receiver not yet implemented")

	default:
		log.Fatalf("Unknown email receive method: %s", cfg.Email.ReceiveMethod)
	}

	// Keep the application running until we receive an interrupt signal
	<-ctx.Done()
	log.Println("Shutting down mail server...")
}
