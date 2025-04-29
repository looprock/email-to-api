package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the application
type Config struct {
	Database DatabaseConfig
	Email    EmailConfig
	Server   ServerConfig
}

// DatabaseConfig holds database-specific configuration
type DatabaseConfig struct {
	Path   string
	Domain string
}

// EmailConfig holds email-specific configuration
type EmailConfig struct {
	SMTPHost      string
	SMTPPort      int
	MaxSize       int64
	RetryAttempts int
	RetryDelay    int
	ReceiveMethod string
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Admin AdminServerConfig
}

// AdminServerConfig holds admin server configuration
type AdminServerConfig struct {
	Host string
	Port int
}

// Load creates a new Config instance from environment variables
func Load() (*Config, error) {
	// Database configuration
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "mailreader.db"
	}

	domain := os.Getenv("MAILREADER_DOMAIN")
	if domain == "" {
		return nil, fmt.Errorf("MAILREADER_DOMAIN environment variable is required")
	}

	// Email configuration
	smtpHost := os.Getenv("MAILREADER_SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "0.0.0.0"
	}

	smtpPort := 25
	if portStr := os.Getenv("MAILREADER_SMTP_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			smtpPort = port
		}
	}

	maxEmailSize := int64(10 * 1024 * 1024) // 10MB default
	if sizeStr := os.Getenv("MAILREADER_MAX_EMAIL_SIZE"); sizeStr != "" {
		if size, err := strconv.ParseInt(sizeStr, 10, 64); err == nil {
			maxEmailSize = size
		}
	}

	maxRetries := 10 // Default to 10 retries
	if retriesStr := os.Getenv("MAILREADER_MAX_RETRIES"); retriesStr != "" {
		if retries, err := strconv.Atoi(retriesStr); err == nil {
			maxRetries = retries
		}
	}

	retryDelay := 5
	if delayStr := os.Getenv("MAILREADER_RETRY_DELAY"); delayStr != "" {
		if delay, err := strconv.Atoi(delayStr); err == nil {
			retryDelay = delay
		}
	}

	receiveMethod := os.Getenv("MAILREADER_RECEIVE_METHOD")
	if receiveMethod == "" {
		receiveMethod = "smtp"
	}

	// Admin server configuration
	adminHost := os.Getenv("ADMIN_SERVER_HOST")
	if adminHost == "" {
		adminHost = "0.0.0.0"
	}

	adminPort := 8080
	if portStr := os.Getenv("ADMIN_SERVER_PORT"); portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			adminPort = port
		}
	}

	return &Config{
		Database: DatabaseConfig{
			Path:   dbPath,
			Domain: domain,
		},
		Email: EmailConfig{
			SMTPHost:      smtpHost,
			SMTPPort:      smtpPort,
			MaxSize:       maxEmailSize,
			RetryAttempts: maxRetries,
			RetryDelay:    retryDelay,
			ReceiveMethod: receiveMethod,
		},
		Server: ServerConfig{
			Admin: AdminServerConfig{
				Host: adminHost,
				Port: adminPort,
			},
		},
	}, nil
}

// getEnvOrDefault returns the environment variable value or the default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
