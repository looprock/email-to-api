package main

import (
	"flag"
	"fmt"
	"log"
	"net/smtp"
	"os"
)

func main() {
	// Configure logging
	log.SetFlags(log.Ldate | log.Ltime)
	log.SetPrefix("[test-email] ")

	// Parse command line arguments
	host := flag.String("host", "localhost", "SMTP server host")
	port := flag.Int("port", 25, "SMTP server port")
	fromAddr := flag.String("from", "sender@example.com", "From email address")
	toAddr := flag.String("to", "foo@localhost.localdomain", "To email address")
	subject := flag.String("subject", "Test Subject Word1 Word2", "Email subject")
	body := flag.String("body", "This is a test email body.", "Email body")
	flag.Parse()

	// Build email message
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"\r\n"+
		"%s\r\n", *fromAddr, *toAddr, *subject, *body)

	// Prepare server address
	addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("Attempting to connect to %s...", addr)

	// Create SMTP client
	client, err := smtp.Dial(addr)
	if err != nil {
		if os.IsTimeout(err) {
			log.Fatalf("Connection timeout - Is the SMTP server running on %s?", addr)
		}
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()
	log.Println("Connected to server")

	// Set sender and recipient
	if err := client.Mail(*fromAddr); err != nil {
		log.Fatalf("Failed to set sender: %v", err)
	}
	if err := client.Rcpt(*toAddr); err != nil {
		log.Fatalf("Failed to set recipient: %v", err)
	}

	// Send the email body
	log.Println("Attempting to send message...")
	writer, err := client.Data()
	if err != nil {
		log.Fatalf("Failed to start data transaction: %v", err)
	}

	_, err = writer.Write([]byte(msg))
	if err != nil {
		log.Fatalf("Failed to write message: %v", err)
	}

	err = writer.Close()
	if err != nil {
		log.Fatalf("Failed to close data transaction: %v", err)
	}

	// Quit the connection
	err = client.Quit()
	if err != nil {
		log.Printf("Warning: Failed to close connection cleanly: %v", err)
	}

	// Print success message
	log.Println("\nEmail sent successfully!")
	log.Printf("From: %s", *fromAddr)
	log.Printf("To: %s", *toAddr)
	log.Printf("Subject: %s", *subject)
	log.Printf("Body: %s", *body)
}
