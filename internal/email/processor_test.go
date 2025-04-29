package email

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/looprock/email-to-api/internal/database"
)

func TestProcessor_Process(t *testing.T) {
	// Create a test API server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		var data ProcessedData
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		// Verify the processed data
		expectedEmail := EmailData{
			From:    "sender@example.com",
			To:      "test@example.com",
			Subject: "test subject",
			Body:    "Test email body",
		}

		if data.Data.From != expectedEmail.From {
			t.Errorf("Expected From = %s, got %s", expectedEmail.From, data.Data.From)
		}
		if data.Data.To != expectedEmail.To {
			t.Errorf("Expected To = %s, got %s", expectedEmail.To, data.Data.To)
		}
		if data.Data.Subject != expectedEmail.Subject {
			t.Errorf("Expected Subject = %s, got %s", expectedEmail.Subject, data.Data.Subject)
		}
		if data.Data.Body != expectedEmail.Body {
			t.Errorf("Expected Body = %s, got %s", expectedEmail.Body, data.Data.Body)
		}

		if data.Source != "email" {
			t.Errorf("Expected source 'email', got '%s'", data.Source)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	// Create a test database
	db, err := database.New(":memory:", "example.com")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create test tables
	_, err = db.Exec(`
		CREATE TABLE email_mappings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			generated_email TEXT NOT NULL UNIQUE,
			endpoint_url TEXT NOT NULL,
			description TEXT,
			headers TEXT,
			is_active BOOLEAN DEFAULT TRUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE email_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			mapping_id INTEGER,
			from_address TEXT NOT NULL,
			subject TEXT,
			body TEXT,
			processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			status TEXT NOT NULL,
			error_message TEXT,
			headers TEXT
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create test tables: %v", err)
	}

	// Insert test mapping
	mapping, err := db.CreateEmailMapping(1, ts.URL, "Test Mapping", map[string]string{"Content-Type": "application/json"})
	if err != nil {
		t.Fatalf("Failed to create test mapping: %v", err)
	}

	// Create processor with test configuration
	processor := New(db, ProcessorConfig{
		MaxSize:       1024 * 1024,
		RetryAttempts: 3,
		RetryDelay:    1,
	})

	// Test processing an email
	email := Email{
		From:    "sender@example.com",
		To:      mapping.GeneratedEmail,
		Subject: "test subject",
		Body:    "Test email body",
	}

	if err := processor.Process(email); err != nil {
		t.Errorf("Failed to process email: %v", err)
	}
}
