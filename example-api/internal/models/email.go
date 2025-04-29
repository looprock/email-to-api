package models

import "time"

type Email struct {
	ID        int64     `json:"id"`
	Tags      []string  `json:"tags"`
	Body      string    `json:"body"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

type EmailRequest struct {
	Tags   []string `json:"tags"`
	Body   string   `json:"body"`
	Source string   `json:"source"`
}

// EmailResponse represents a list of emails
type EmailResponse struct {
	Emails []Email `json:"emails"`
	Total  int     `json:"total"`
}
