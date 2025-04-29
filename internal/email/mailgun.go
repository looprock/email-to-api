package email

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mailgun/mailgun-go/v4"
)

// Sender handles sending emails via Mailgun
type Sender struct {
	mg          mailgun.Mailgun
	domain      string
	fromAddress string
	siteDomain  string
}

// NewMailgunSender creates a new Mailgun email sender
func NewMailgunSender() (*Sender, error) {
	apiKey := os.Getenv("MAILGUN_API_KEY")
	if apiKey == "" {
		return nil, nil // Mailgun not configured, return nil without error
	}

	domain := os.Getenv("MAILGUN_DOMAIN")
	if domain == "" {
		return nil, fmt.Errorf("MAILGUN_DOMAIN environment variable is required when MAILGUN_API_KEY is set")
	}

	siteDomain := os.Getenv("SITE_DOMAIN")
	if siteDomain == "" {
		return nil, fmt.Errorf("SITE_DOMAIN environment variable is required when MAILGUN_API_KEY is set")
	}

	fromAddress := os.Getenv("MAILGUN_FROM_ADDRESS")
	if fromAddress == "" {
		return nil, fmt.Errorf("MAILGUN_FROM_ADDRESS environment variable is required when MAILGUN_API_KEY is set")
	}

	// Validate that from address matches domain
	if !strings.HasSuffix(fromAddress, "@"+domain) {
		return nil, fmt.Errorf("MAILGUN_FROM_ADDRESS (%s) must use the same domain as MAILGUN_DOMAIN (%s)", fromAddress, domain)
	}

	log.Printf("Initializing Mailgun with domain: %s, from address: %s", domain, fromAddress)
	mg := mailgun.NewMailgun(domain, apiKey)

	// Test the API key by getting sending stats
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := mg.GetStats(ctx, []string{"accepted", "delivered"}, &mailgun.GetStatOptions{})
	if err != nil {
		if strings.Contains(err.Error(), "401") {
			return nil, fmt.Errorf("authentication failed - please verify your API key and domain settings in the Mailgun dashboard")
		}
		return nil, fmt.Errorf("failed to validate Mailgun credentials: %w", err)
	}

	return &Sender{
		mg:          mg,
		domain:      domain,
		fromAddress: fromAddress,
		siteDomain:  siteDomain,
	}, nil
}

// SendRegistrationEmail sends a registration email with the provided token
func (s *Sender) SendRegistrationEmail(email, token string) error {
	subject := "Complete Your Registration"
	body := fmt.Sprintf(`Hello!

You have been invited to use the Email API Management System. To complete your registration, please click the link below:

http://%s/register?token=%s

This link will expire in 24 hours.

If you did not request this invitation, please ignore this email.

Best regards,
Email API Management System`, s.siteDomain, token)

	log.Printf("Attempting to send registration email to %s using domain %s", email, s.domain)
	message := mailgun.NewMessage(s.fromAddress, subject, body, email)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, id, err := s.mg.Send(ctx, message)
	if err != nil {
		if strings.Contains(err.Error(), "401") {
			return fmt.Errorf("unauthorized: please verify your Mailgun API key and domain settings")
		}
		return fmt.Errorf("failed to send registration email: %w", err)
	}
	log.Printf("Successfully sent registration email to %s with message ID: %s", email, id)

	return nil
}
