package database

import (
	"fmt"
	"os"
	"path/filepath"
)

// Config holds database configuration
type Config struct {
	Driver     string
	DSN        string
	MigrateURL string
	Domain     string // Domain for generated email addresses
}

// LoadConfig loads database configuration from environment variables
func LoadConfig() (*Config, error) {
	driver := os.Getenv("DB_DRIVER")
	if driver == "" {
		driver = "sqlite" // Default to SQLite
	}

	var dsn string
	var migrateURL string

	switch driver {
	case "postgres":
		host := os.Getenv("DB_HOST")
		if host == "" {
			host = "localhost"
		}
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "5432"
		}
		user := os.Getenv("DB_USER")
		if user == "" {
			user = "postgres"
		}
		password := os.Getenv("DB_PASSWORD")
		dbname := os.Getenv("DB_NAME")
		if dbname == "" {
			dbname = "emailtoapi"
		}
		sslmode := os.Getenv("DB_SSLMODE")
		if sslmode == "" {
			sslmode = "disable"
		}

		dsn = fmt.Sprintf("host=%s port=%s user=%s dbname=%s sslmode=%s",
			host, port, user, dbname, sslmode)
		if password != "" {
			dsn += fmt.Sprintf(" password=%s", password)
		}

		// Construct migrate URL for postgres
		migrateURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			user, password, host, port, dbname, sslmode)

	case "sqlite":
		dbPath := os.Getenv("DB_PATH")
		if dbPath == "" {
			dbPath = "emailtoapi.db"
		}

		// Ensure the directory exists
		dbDir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}

		dsn = dbPath
		migrateURL = fmt.Sprintf("sqlite3://%s", dbPath)

	default:
		return nil, fmt.Errorf("unsupported database driver: %s", driver)
	}

	return &Config{
		Driver:     driver,
		DSN:        dsn,
		MigrateURL: migrateURL,
	}, nil
}
