package database

import (
	"database/sql"
	"encoding/json"
	"example-api/internal/models"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
}

func New(dbPath string) (*Database, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create emails table if it doesn't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS emails (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tags TEXT NOT NULL,
			body TEXT NOT NULL,
			source TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

// LogEmailStatus logs the status of an email operation
func (d *Database) LogEmailStatus(emailID int64, status string, errorMessage string) error {
	_, err := d.db.Exec(
		"INSERT INTO email_logs (email_id, status, error_message) VALUES (?, ?, ?)",
		emailID,
		status,
		errorMessage,
	)
	if err != nil {
		return fmt.Errorf("failed to log email status: %w", err)
	}
	return nil
}

func (d *Database) StoreEmail(email *models.EmailRequest) (*models.Email, error) {
	// Convert tags slice to JSON string for storage
	tagsJSON, err := json.Marshal(email.Tags)
	if err != nil {
		// Log the error
		if logErr := d.LogEmailStatus(0, "error", fmt.Sprintf("failed to marshal tags: %v", err)); logErr != nil {
			log.Printf("Failed to log error: %v", logErr)
		}
		return nil, err
	}

	// Strip trailing newlines from body
	cleanBody := strings.TrimRight(email.Body, "\r\n")

	result, err := d.db.Exec(
		"INSERT INTO emails (tags, body, source, created_at) VALUES (?, ?, ?, ?)",
		string(tagsJSON),
		cleanBody,
		email.Source,
		time.Now(),
	)
	if err != nil {
		// Log the error
		if logErr := d.LogEmailStatus(0, "error", fmt.Sprintf("failed to insert email: %v", err)); logErr != nil {
			log.Printf("Failed to log error: %v", logErr)
		}
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		// Log the error
		if logErr := d.LogEmailStatus(0, "error", fmt.Sprintf("failed to get last insert ID: %v", err)); logErr != nil {
			log.Printf("Failed to log error: %v", logErr)
		}
		return nil, err
	}

	// Log success
	if err := d.LogEmailStatus(id, "success", ""); err != nil {
		log.Printf("Failed to log success: %v", err)
	}

	return &models.Email{
		ID:        id,
		Tags:      email.Tags,
		Body:      cleanBody,
		Source:    email.Source,
		CreatedAt: time.Now(),
	}, nil
}

// GetEmailByID retrieves an email by its ID
func (d *Database) GetEmailByID(id int64) (*models.Email, error) {
	var email models.Email
	var tagsJSON string
	var createdAt string

	err := d.db.QueryRow(
		"SELECT id, tags, body, source, created_at FROM emails WHERE id = ?",
		id,
	).Scan(&email.ID, &tagsJSON, &email.Body, &email.Source, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// Parse the created_at timestamp
	email.CreatedAt, err = time.Parse("2006-01-02 15:04:05", createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to parse timestamp: %w", err)
	}

	// Parse the tags JSON
	if err := json.Unmarshal([]byte(tagsJSON), &email.Tags); err != nil {
		return nil, fmt.Errorf("failed to parse tags: %w", err)
	}

	return &email, nil
}

// GetEmailsByTag retrieves all emails that contain a specific tag
func (d *Database) GetEmailsByTag(tag string) ([]models.Email, error) {
	// Use a simpler JSON array contains check
	rows, err := d.db.Query(`
		SELECT id, tags, body, source, created_at 
		FROM emails 
		WHERE tags LIKE ?
		ORDER BY created_at DESC`,
		fmt.Sprintf("%%\"%s\"%%", tag))
	if err != nil {
		return nil, fmt.Errorf("failed to query emails: %w", err)
	}
	defer rows.Close()

	var emails []models.Email
	for rows.Next() {
		var email models.Email
		var tagsJSON string
		var createdAt string

		if err := rows.Scan(&email.ID, &tagsJSON, &email.Body, &email.Source, &createdAt); err != nil {
			return nil, fmt.Errorf("failed to scan email row: %w", err)
		}

		// Clean up the timestamp by removing newlines
		createdAt = strings.ReplaceAll(createdAt, "\n", "")

		// Parse the created_at timestamp, trying multiple formats
		parsed := false
		for _, format := range []string{
			"2006-01-02 15:04:05.999999-07:00", // SQLite format with microseconds and timezone
			time.RFC3339,                       // RFC3339
			"2006-01-02 15:04:05",              // Basic format
		} {
			if t, err := time.Parse(format, createdAt); err == nil {
				email.CreatedAt = t
				parsed = true
				break
			}
		}

		if !parsed {
			log.Printf("Warning: Could not parse timestamp %q, using current time", createdAt)
			email.CreatedAt = time.Now()
		}

		// Parse the tags JSON
		if err := json.Unmarshal([]byte(tagsJSON), &email.Tags); err != nil {
			return nil, fmt.Errorf("failed to parse tags: %w", err)
		}

		emails = append(emails, email)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return emails, nil
}
