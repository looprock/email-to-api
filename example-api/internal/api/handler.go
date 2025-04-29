package api

import (
	"example-api/internal/database"
	"example-api/internal/models"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	db *database.Database
}

func New(db *database.Database) *Handler {
	return &Handler{db: db}
}

// AuthMiddleware checks for a valid token in the Authorization header
func AuthMiddleware(validToken string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.GetHeader("Authorization")
		if token == "" {
			log.Printf("Auth failed: No token provided for %s %s", c.Request.Method, c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No authorization token provided"})
			c.Abort()
			return
		}

		// Remove "Bearer " prefix if present
		if len(token) > 7 && token[:7] == "Bearer " {
			token = token[7:]
		}

		if token != validToken {
			log.Printf("Auth failed: Invalid token provided for %s %s", c.Request.Method, c.Request.URL.Path)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		log.Printf("Auth successful for %s %s", c.Request.Method, c.Request.URL.Path)
		c.Next()
	}
}

// HandleEmailReceive processes incoming email data
func (h *Handler) HandleEmailReceive(c *gin.Context) {
	log.Printf("Received email request with Content-Type: %s", c.GetHeader("Content-Type"))

	var incoming struct {
		Data struct {
			From                    string              `json:"from"`
			To                      string              `json:"to"`
			Subject                 string              `json:"subject"`
			Body                    string              `json:"body"`
			Cc                      []string            `json:"cc,omitempty"`
			Bcc                     []string            `json:"bcc,omitempty"`
			MessageID               string              `json:"message_id,omitempty"`
			InReplyTo               string              `json:"in_reply_to,omitempty"`
			References              []string            `json:"references,omitempty"`
			Date                    time.Time           `json:"date"`
			ContentType             string              `json:"content_type,omitempty"`
			ContentTransferEncoding string              `json:"content_transfer_encoding,omitempty"`
			HTMLBody                string              `json:"html_body,omitempty"`
			PlainBody               string              `json:"plain_body,omitempty"`
			ReceivedFrom            string              `json:"received_from,omitempty"`
			ReceivedAt              time.Time           `json:"received_at"`
			AuthenticatedAs         string              `json:"authenticated_as,omitempty"`
			Headers                 map[string][]string `json:"headers,omitempty"`
		} `json:"data"`
		Source string `json:"source"`
	}

	if err := c.ShouldBindJSON(&incoming); err != nil {
		log.Printf("Failed to decode JSON: %v", err)
		log.Printf("Raw request body: %+v", c.Request.Body)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	log.Printf("Decoded incoming data: %+v", incoming)

	// Extract tags from subject
	tags := strings.Fields(incoming.Data.Subject)
	if len(tags) == 0 {
		tags = []string{"untagged"}
	}
	log.Printf("Extracted tags: %v", tags)

	// Store in database
	email := &models.EmailRequest{
		Tags:   tags,
		Body:   incoming.Data.Body,
		Source: incoming.Source,
	}

	log.Printf("Storing email: %+v", email)
	storedEmail, err := h.db.StoreEmail(email)
	if err != nil {
		log.Printf("Failed to store email: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to store email"})
		return
	}

	log.Printf("Successfully stored email with ID: %d", storedEmail.ID)
	c.JSON(http.StatusCreated, storedEmail)
}

// HandleGetEmailByID handles GET requests to retrieve an email by ID
func (h *Handler) HandleGetEmailByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ID format"})
		return
	}

	email, err := h.db.GetEmailByID(id)
	if err != nil {
		log.Printf("Failed to get email: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve email"})
		return
	}

	if email == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Email not found"})
		return
	}

	c.JSON(http.StatusOK, email)
}

// HandleGetEmailsByTag handles GET requests to retrieve emails by tag
func (h *Handler) HandleGetEmailsByTag(c *gin.Context) {
	tag := c.Query("tag")
	if tag == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Tag parameter is required"})
		return
	}

	log.Printf("Searching for emails with tag: %s", tag)
	emails, err := h.db.GetEmailsByTag(tag)
	if err != nil {
		log.Printf("Failed to get emails by tag %q: %+v", tag, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to retrieve emails: %v", err)})
		return
	}

	log.Printf("Found %d emails with tag %q", len(emails), tag)
	response := models.EmailResponse{
		Emails: emails,
		Total:  len(emails),
	}

	c.JSON(http.StatusOK, response)
}
