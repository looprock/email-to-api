package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/golang-migrate/migrate/v4/source/file"
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
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize database
	dbConfig := &database.Config{
		Driver:     cfg.Database.Driver,
		DSN:        cfg.Database.Path,                              // For SQLite
		MigrateURL: fmt.Sprintf("sqlite3://%s", cfg.Database.Path), // Database URL for migrations
		Domain:     cfg.MailServer.Domain,
	}
	if cfg.Database.Driver == "postgres" {
		dbConfig.DSN = fmt.Sprintf("host=%s port=%d user=%s dbname=%s password=%s sslmode=%s",
			cfg.Database.Host, cfg.Database.Port, cfg.Database.User,
			cfg.Database.Name, cfg.Database.Password, cfg.Database.SSLMode)
		dbConfig.MigrateURL = fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			cfg.Database.User, cfg.Database.Password, cfg.Database.Host,
			cfg.Database.Port, cfg.Database.Name, cfg.Database.SSLMode)
	}

	db, err := database.New(dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Run database migrations
	if err := db.Migrate(); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	// Initialize email processor
	processor := email.New(db, email.ProcessorConfig{
		MaxSize:       cfg.MailServer.MaxEmailSize,
		RetryAttempts: cfg.MailServer.MaxRetries,
		RetryDelay:    cfg.MailServer.RetryDelay,
	})

	// Start the appropriate email receiver based on configuration
	switch cfg.MailServer.ReceiveMethod {
	case "smtp":
		go func() {
			if err := email.StartSMTPServer(processor, cfg.MailServer.SMTPHost, cfg.MailServer.SMTPPort); err != nil {
				log.Printf("SMTP server error: %v", err)
				stop()
			}
		}()
		log.Printf("Started SMTP server on %s:%d", cfg.MailServer.SMTPHost, cfg.MailServer.SMTPPort)

	case "webhook":
		// TODO: Implement webhook receiver
		log.Fatal("Webhook receiver not yet implemented")

	default:
		log.Fatalf("Unknown email receive method: %s", cfg.MailServer.ReceiveMethod)
	}

	// Keep the application running until we receive an interrupt signal
	<-ctx.Done()
	log.Println("Shutting down mail server...")
}
