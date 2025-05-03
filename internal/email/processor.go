package email

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/looprock/email-to-api/internal/database"
)

// Processor handles email processing and forwarding
type Processor struct {
	db     *database.DB
	config ProcessorConfig
}

// BackoffConfig holds configuration for exponential backoff
type BackoffConfig struct {
	InitialDelay  time.Duration
	MaxDelay      time.Duration
	Multiplier    float64
	Randomization float64
}

// ProcessorConfig holds configuration for the email processor
type ProcessorConfig struct {
	MaxSize       int64
	RetryAttempts int
	RetryDelay    int
	Backoff       BackoffConfig
}

// New creates a new email processor
func New(db *database.DB, config ProcessorConfig) *Processor {
	// Set default backoff values if not configured
	if config.Backoff.InitialDelay == 0 {
		config.Backoff.InitialDelay = 1 * time.Second
	}
	if config.Backoff.MaxDelay == 0 {
		config.Backoff.MaxDelay = 30 * time.Second
	}
	if config.Backoff.Multiplier == 0 {
		config.Backoff.Multiplier = 2.0
	}
	if config.Backoff.Randomization == 0 {
		config.Backoff.Randomization = 0.2 // 20% randomization
	}

	return &Processor{
		db:     db,
		config: config,
	}
}

// Email represents a processed email
type Email struct {
	// Basic email fields
	From    string
	To      string
	Subject string
	Body    string

	// Additional recipients
	Cc  []string
	Bcc []string

	// Message metadata
	MessageID  string
	InReplyTo  string
	References []string
	Date       time.Time

	// Content details
	ContentType             string
	ContentTransferEncoding string
	HTMLBody                string
	PlainBody               string
	Attachments             []Attachment

	// Connection info
	ReceivedFrom    string
	ReceivedAt      time.Time
	AuthenticatedAs string

	// All headers in raw form
	Headers map[string][]string
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

// EmailData represents a processed email
type EmailData struct {
	// Basic fields
	From    string `json:"from"`
	To      string `json:"to"`
	Subject string `json:"subject"`
	Body    string `json:"body"`

	// Additional recipients
	Cc  []string `json:"cc,omitempty"`
	Bcc []string `json:"bcc,omitempty"`

	// Message metadata
	MessageID  string    `json:"message_id,omitempty"`
	InReplyTo  string    `json:"in_reply_to,omitempty"`
	References []string  `json:"references,omitempty"`
	Date       time.Time `json:"date"`

	// Content details
	ContentType             string `json:"content_type,omitempty"`
	ContentTransferEncoding string `json:"content_transfer_encoding,omitempty"`
	HTMLBody                string `json:"html_body,omitempty"`
	PlainBody               string `json:"plain_body,omitempty"`

	// Connection info
	ReceivedFrom    string    `json:"received_from,omitempty"`
	ReceivedAt      time.Time `json:"received_at"`
	AuthenticatedAs string    `json:"authenticated_as,omitempty"`

	// All headers
	Headers map[string][]string `json:"headers,omitempty"`
}

// ProcessedData represents the JSON payload to be sent to the API
type ProcessedData struct {
	Data   EmailData `json:"data"`
	Source string    `json:"source"`
}

// calculateBackoff calculates the next backoff duration with jitter
func (p *Processor) calculateBackoff(attempt int) time.Duration {
	// Calculate base delay using exponential backoff
	multiplier := math.Pow(p.config.Backoff.Multiplier, float64(attempt))
	delay := time.Duration(float64(p.config.Backoff.InitialDelay) * multiplier)

	// Apply maximum delay cap
	if delay > p.config.Backoff.MaxDelay {
		delay = p.config.Backoff.MaxDelay
	}

	// Add randomization/jitter
	jitterRange := float64(delay) * p.config.Backoff.Randomization
	jitter := time.Duration(rand.Float64() * jitterRange)
	delay = delay + jitter

	return delay
}

// Process handles the email processing workflow
func (p *Processor) Process(email Email) error {
	log.Printf("Processing email from %s to %s with subject: %q", email.From, email.To, email.Subject)

	// Check email size immediately
	if int64(len(email.Body)) > p.config.MaxSize {
		log.Printf("Email size %d bytes exceeds maximum allowed size of %d bytes", len(email.Body), p.config.MaxSize)
		// Log the dropped email due to size
		if err := p.db.LogEmailProcessing(
			email.To,
			email.Subject,
			"dropped",
			fmt.Sprintf("email size %d bytes exceeds maximum allowed size of %d bytes", len(email.Body), p.config.MaxSize),
			nil,
			uint(1), // default user ID
		); err != nil {
			log.Printf("Failed to log dropped email: %v", err)
		}
		return fmt.Errorf("email size exceeds maximum allowed size")
	}
	log.Printf("Email size check passed: %d bytes", len(email.Body))

	// Start async processing
	go func() {
		if err := p.processAsync(email); err != nil {
			log.Printf("Async processing failed: %v", err)
		}
	}()

	return nil
}

// processAsync handles the asynchronous email processing workflow
func (p *Processor) processAsync(email Email) error {
	// Get API endpoint mapping for the recipient
	mapping, err := p.db.GetEmailMapping(email.To)
	if err != nil {
		log.Printf("Error getting email mapping for address %q: %v", email.To, err)
		// Log the error in getting mapping
		if logErr := p.db.LogEmailProcessing(
			email.To,
			email.Subject,
			"error",
			fmt.Sprintf("failed to get email mapping: %v", err),
			nil,
			uint(1), // Use default user ID only for logging errors when we can't find the mapping
		); logErr != nil {
			log.Printf("Failed to log error: %v", logErr)
		}
		return fmt.Errorf("failed to get email mapping: %w", err)
	}
	if mapping == nil {
		log.Printf("No mapping found for email address %q - dropping email from %q with subject %q",
			email.To, email.From, email.Subject)
		// Log the dropped email
		if err := p.db.LogEmailProcessing(
			email.To,
			email.Subject,
			"dropped",
			"no mapping found",
			nil,
			uint(1), // Use default user ID only for logging errors when we can't find the mapping
		); err != nil {
			log.Printf("Failed to log dropped email: %v", err)
		}
		return nil
	}

	if !mapping.IsActive {
		log.Printf("Mapping found for %q but it is inactive - dropping email from %q with subject %q",
			email.To, email.From, email.Subject)
		// Log the dropped email
		if err := p.db.LogEmailProcessing(
			email.To,
			email.Subject,
			"dropped",
			"mapping is inactive",
			mapping.Headers,
			mapping.UserID,
		); err != nil {
			log.Printf("Failed to log dropped email: %v", err)
		}
		return nil
	}

	log.Printf("Found active mapping for %q to endpoint %q", email.To, mapping.EndpointURL)

	// Process the subject into array of tags
	tags := strings.Fields(email.Subject)
	if len(tags) == 0 {
		// Ensure we always have at least one tag
		tags = []string{"untagged"}
		log.Printf("No tags found in subject, using default tag: %q", tags[0])
	} else {
		// Convert tags to lowercase
		for i, tag := range tags {
			tags[i] = strings.ToLower(tag)
		}
		log.Printf("Extracted %d tags from subject: %v", len(tags), tags)
	}

	// Convert Email to EmailData
	emailData := EmailData{
		// Basic fields
		From:    email.From,
		To:      email.To,
		Subject: email.Subject,
		Body:    email.Body,

		// Additional recipients
		Cc:  email.Cc,
		Bcc: email.Bcc,

		// Message metadata
		MessageID:  email.MessageID,
		InReplyTo:  email.InReplyTo,
		References: email.References,
		Date:       email.Date,

		// Content details
		ContentType:             email.ContentType,
		ContentTransferEncoding: email.ContentTransferEncoding,
		HTMLBody:                email.HTMLBody,
		PlainBody:               email.PlainBody,

		// Connection info
		ReceivedFrom:    email.ReceivedFrom,
		ReceivedAt:      email.ReceivedAt,
		AuthenticatedAs: email.AuthenticatedAs,

		// All headers
		Headers: email.Headers,
	}

	processedEmail := ProcessedData{
		Data:   emailData,
		Source: "email",
	}

	// Log the payload for debugging
	payloadJSON, _ := json.Marshal(processedEmail)
	log.Printf("Sending payload to API: %s", string(payloadJSON))

	// Send to API with retries and exponential backoff
	var lastErr error
	for attempt := 0; attempt < p.config.RetryAttempts; attempt++ {
		log.Printf("Attempt %d/%d: Sending to endpoint %q", attempt+1, p.config.RetryAttempts, mapping.EndpointURL)
		if err := p.sendToAPI(mapping.EndpointURL, mapping.Headers, processedEmail); err != nil {
			lastErr = err
			backoff := p.calculateBackoff(attempt)
			log.Printf("Attempt %d failed: %v. Retrying in %v...", attempt+1, err, backoff)
			time.Sleep(backoff)
			continue
		}

		log.Printf("Successfully sent email to endpoint %q", mapping.EndpointURL)

		// Log successful processing
		if err := p.db.LogEmailProcessing(
			email.To,
			email.Subject,
			"success",
			"",
			mapping.Headers,
			mapping.UserID, // Use the mapping's UserID for logging
		); err != nil {
			log.Printf("Warning: Failed to log successful processing: %v", err)
			return fmt.Errorf("failed to log success: %w", err)
		}
		log.Printf("Successfully logged email processing in database")

		return nil
	}

	// Log failed processing
	if err := p.db.LogEmailProcessing(
		email.To,
		email.Subject,
		"error",
		lastErr.Error(),
		mapping.Headers,
		mapping.UserID, // Use the mapping's UserID for logging
	); err != nil {
		log.Printf("Warning: Failed to log error processing: %v", err)
		return fmt.Errorf("failed to log error: %w", err)
	}

	return fmt.Errorf("failed to process email after %d attempts: %w",
		p.config.RetryAttempts, lastErr)
}

// sendToAPI sends the processed data to the specified API endpoint
func (p *Processor) sendToAPI(endpoint string, headers map[string]string, payload ProcessedData) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	log.Printf("Sending request to %s with payload: %s", endpoint, string(data))

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set default Content-Type if not specified in headers
	if _, hasContentType := headers["Content-Type"]; !hasContentType {
		req.Header.Set("Content-Type", "application/json")
		log.Printf("Using default Content-Type: application/json")
	}

	// Add custom headers
	for key, value := range headers {
		req.Header.Set(key, value)
		log.Printf("Added custom header: %s: %s", key, value)
	}

	log.Printf("Request headers: %v", req.Header)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read and log response body for debugging
	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("Response status: %d, body: %s", resp.StatusCode, string(respBody))

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API request failed with status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("API request successful (status %d)", resp.StatusCode)
	return nil
}
