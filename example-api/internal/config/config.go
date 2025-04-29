package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	// Server settings
	ServerPort int
	Domain     string

	// Database settings
	DBPath string

	// Email settings
	MailgunAPIKey    string
	MailgunDomain    string
	FromEmailAddress string

	// Security settings
	JWTSecret      string
	AdminPassword  string
	TokenExpiry    int // in hours
	RandomEmailLen int // length of random part in generated emails
}

func New() (*Config, error) {
	domain := os.Getenv("MAILREADER_DOMAIN")
	if domain == "" {
		return nil, fmt.Errorf("MAILREADER_DOMAIN environment variable is required")
	}

	adminPassword := os.Getenv("MAILREADER_ADMIN_PASSWORD")
	if adminPassword == "" {
		return nil, fmt.Errorf("MAILREADER_ADMIN_PASSWORD environment variable is required")
	}

	// Optional settings with defaults
	port := 8809
	if portStr := os.Getenv("MAILREADER_PORT"); portStr != "" {
		fmt.Sscanf(portStr, "%d", &port)
	}

	dbPath := "mailreader.db"
	if path := os.Getenv("MAILREADER_DB_PATH"); path != "" {
		dbPath = path
	}

	mailgunAPIKey := os.Getenv("MAILGUN_API_KEY")
	mailgunDomain := os.Getenv("MAILGUN_DOMAIN")
	fromEmail := fmt.Sprintf("no-reply@%s", domain)
	if email := os.Getenv("MAILREADER_FROM_EMAIL"); email != "" {
		fromEmail = email
	}

	jwtSecret := os.Getenv("MAILREADER_JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "default-secret-change-me-in-production"
	}

	return &Config{
		ServerPort:       port,
		Domain:           strings.TrimSpace(domain),
		DBPath:           dbPath,
		MailgunAPIKey:    mailgunAPIKey,
		MailgunDomain:    mailgunDomain,
		FromEmailAddress: fromEmail,
		JWTSecret:        jwtSecret,
		AdminPassword:    adminPassword,
		TokenExpiry:      24, // 24 hours
		RandomEmailLen:   12, // 12 characters for random email part
	}, nil
}
