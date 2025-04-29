package email

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/emersion/go-smtp"
)

// The Backend implements SMTP server methods
type Backend struct {
	processor *Processor
}

// NewBackend creates a new SMTP backend
func NewBackend(processor *Processor) *Backend {
	return &Backend{processor: processor}
}

// NewSession implements smtp.Backend interface
func (bkd *Backend) NewSession(c *smtp.Conn) (smtp.Session, error) {
	remoteAddr := c.Conn().RemoteAddr().String()
	log.Printf("New SMTP session started from %s", remoteAddr)
	return &Session{
		processor:  bkd.processor,
		remoteAddr: remoteAddr,
	}, nil
}

// A Session is returned after EHLO
type Session struct {
	processor  *Processor
	from       string
	to         []string
	subject    string
	body       string
	remoteAddr string
	username   string
}

func (s *Session) AuthPlain(username, password string) error {
	log.Printf("Auth attempt with username: %s", username)
	s.username = username
	// For this implementation, we'll accept all auth
	return nil
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	log.Printf("MAIL FROM: %s", from)
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	log.Printf("RCPT TO: %s", to)
	s.to = append(s.to, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	log.Printf("Starting to receive email data")
	// Read the email data
	data, err := io.ReadAll(r)
	if err != nil {
		log.Printf("Error reading email data: %v", err)
		return fmt.Errorf("failed to read email data: %w", err)
	}
	log.Printf("Received email data of length: %d bytes", len(data))

	// Parse the email data
	emailStr := string(data)
	lines := strings.Split(emailStr, "\r\n")

	// Initialize headers map
	headers := make(map[string][]string)
	var currentHeader string
	var bodyStart int

	// Parse headers
	for i, line := range lines {
		if line == "" {
			bodyStart = i + 1
			break
		}

		// Handle header continuation lines
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if currentHeader != "" {
				lastVal := headers[currentHeader][len(headers[currentHeader])-1]
				headers[currentHeader][len(headers[currentHeader])-1] = lastVal + "\n" + strings.TrimSpace(line)
			}
			continue
		}

		// Parse header
		if idx := strings.Index(line, ":"); idx > 0 {
			currentHeader = strings.TrimSpace(line[:idx])
			value := strings.TrimSpace(line[idx+1:])
			headers[currentHeader] = append(headers[currentHeader], value)

			// Capture subject specifically
			if strings.EqualFold(currentHeader, "Subject") {
				s.subject = value
				log.Printf("Found Subject header: %q", value)
			}
		}
	}

	// Parse Content-Type and boundaries
	contentType := ""
	if ctHeaders := headers["Content-Type"]; len(ctHeaders) > 0 {
		contentType = ctHeaders[0]
	}

	// Parse message ID and references
	messageID := ""
	if msgIDs := headers["Message-ID"]; len(msgIDs) > 0 {
		messageID = msgIDs[0]
	}

	inReplyTo := ""
	if replies := headers["In-Reply-To"]; len(replies) > 0 {
		inReplyTo = replies[0]
	}

	references := []string{}
	if refs := headers["References"]; len(refs) > 0 {
		references = strings.Fields(refs[0])
	}

	// Parse CC and BCC
	cc := []string{}
	if ccHeaders := headers["Cc"]; len(ccHeaders) > 0 {
		cc = parseAddressList(ccHeaders[0])
	}

	bcc := []string{}
	if bccHeaders := headers["Bcc"]; len(bccHeaders) > 0 {
		bcc = parseAddressList(bccHeaders[0])
	}

	// Parse Date
	receivedTime := time.Now()
	if dateHeaders := headers["Date"]; len(dateHeaders) > 0 {
		if parsedTime, err := time.Parse(time.RFC1123Z, dateHeaders[0]); err == nil {
			receivedTime = parsedTime
		}
	}

	// Join the body lines back together
	body := strings.Join(lines[bodyStart:], "\r\n")

	// Process for each recipient
	for _, recipient := range s.to {
		email := Email{
			// Basic fields
			From:    s.from,
			To:      recipient,
			Subject: s.subject,
			Body:    body,

			// Additional recipients
			Cc:  cc,
			Bcc: bcc,

			// Message metadata
			MessageID:  messageID,
			InReplyTo:  inReplyTo,
			References: references,
			Date:       receivedTime,

			// Content details
			ContentType:             contentType,
			ContentTransferEncoding: getFirstHeader(headers, "Content-Transfer-Encoding"),
			PlainBody:               body, // For now, treating all as plain

			// Connection info
			ReceivedFrom:    s.remoteAddr,
			ReceivedAt:      time.Now(),
			AuthenticatedAs: s.username,

			// All headers
			Headers: headers,
		}

		log.Printf("Processing email to: %s", recipient)
		log.Printf("Email details: MessageID=%s, ContentType=%s, Date=%v",
			email.MessageID, email.ContentType, email.Date)

		// Process the email
		if err := s.processor.Process(email); err != nil {
			log.Printf("Failed to process email for recipient %s: %v", recipient, err)
			return fmt.Errorf("failed to process email for %s: %w", recipient, err)
		}
		log.Printf("Successfully processed email for recipient: %s", recipient)
	}

	return nil
}

// Helper function to get first header value
func getFirstHeader(headers map[string][]string, key string) string {
	if values := headers[key]; len(values) > 0 {
		return values[0]
	}
	return ""
}

// Helper function to parse address lists
func parseAddressList(addresses string) []string {
	// Simple splitting by comma for now
	parts := strings.Split(addresses, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		addr := strings.TrimSpace(part)
		if addr != "" {
			result = append(result, addr)
		}
	}
	return result
}

func (s *Session) Reset() {
	log.Printf("Resetting SMTP session")
	s.from = ""
	s.to = []string{}
	s.subject = ""
	s.body = ""
	s.username = ""
}

func (s *Session) Logout() error {
	log.Printf("SMTP session logout")
	return nil
}

// loggingListener wraps a net.Listener to log connections
type loggingListener struct {
	net.Listener
}

func (l *loggingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		log.Printf("Failed to accept connection: %v", err)
		return conn, err
	}

	log.Printf("New TCP connection from: %s", conn.RemoteAddr())
	return &loggingConn{Conn: conn}, nil
}

// loggingConn wraps a net.Conn to log disconnections
type loggingConn struct {
	net.Conn
}

func (c *loggingConn) Close() error {
	log.Printf("TCP connection closed from: %s", c.RemoteAddr())
	return c.Conn.Close()
}

// StartSMTPServer starts the SMTP server
func StartSMTPServer(processor *Processor, host string, port int) error {
	be := NewBackend(processor)
	s := smtp.NewServer(be)

	// Force dual-stack (IPv4 + IPv6) by setting specific listener options
	addr := fmt.Sprintf("%s:%d", host, port)
	config := &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			var opErr error
			if err := c.Control(func(fd uintptr) {
				opErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
				if opErr != nil {
					return
				}
				// Force dual-stack
				opErr = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 0)
			}); err != nil {
				return err
			}
			return opErr
		},
	}

	// Create a TCP listener with dual-stack support
	listener, err := config.Listen(context.Background(), "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to create listener: %w", err)
	}

	s.Addr = addr
	s.Domain = host
	s.ReadTimeout = 30 * time.Second  // Increased timeout
	s.WriteTimeout = 30 * time.Second // Increased timeout
	s.MaxMessageBytes = 1024 * 1024
	s.MaxRecipients = 50
	s.AllowInsecureAuth = true
	s.Debug = log.Writer() // Enable SMTP protocol debugging

	log.Printf("Starting SMTP server at %s", s.Addr)
	log.Printf("Server configuration:")
	log.Printf("- Domain: %s", s.Domain)
	log.Printf("- Read Timeout: %d seconds", s.ReadTimeout/time.Second)
	log.Printf("- Write Timeout: %d seconds", s.WriteTimeout/time.Second)
	log.Printf("- Max Message Size: %d bytes", s.MaxMessageBytes)
	log.Printf("- Max Recipients: %d", s.MaxRecipients)
	log.Printf("- Allow Insecure Auth: %v", s.AllowInsecureAuth)

	// Wrap the listener with logging
	loggingListener := &loggingListener{Listener: listener}

	// Use the logging listener instead of ListenAndServe
	return s.Serve(loggingListener)
}
