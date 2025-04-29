package models

import "time"

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	LastLogin    time.Time `json:"last_login,omitempty"`
	IsActive     bool      `json:"is_active"`
}

type RegistrationToken struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	UsedAt    time.Time `json:"used_at,omitempty"`
}

type EmailMapping struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	GeneratedEmail string    `json:"generated_email"`
	EndpointURL    string    `json:"endpoint_url"`
	Description    string    `json:"description,omitempty"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
	LastUsedAt     time.Time `json:"last_used_at,omitempty"`
}

type ReceivedEmail struct {
	ID          int64     `json:"id"`
	MappingID   int64     `json:"mapping_id"`
	FromAddress string    `json:"from_address"`
	Subject     string    `json:"subject,omitempty"`
	Body        string    `json:"body"`
	ReceivedAt  time.Time `json:"received_at"`
	ProcessedAt time.Time `json:"processed_at,omitempty"`
	Status      string    `json:"status"`
}

// Request/Response structures

type CreateUserRequest struct {
	Email string `json:"email" binding:"required,email"`
	Role  string `json:"role" binding:"required,oneof=admin user"`
}

type CreateMappingRequest struct {
	EndpointURL string `json:"endpoint_url" binding:"required,url"`
	Description string `json:"description"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type SetPasswordRequest struct {
	Token    string `json:"token" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type ListMappingsResponse struct {
	Mappings []EmailMapping `json:"mappings"`
	Total    int            `json:"total"`
}

type ListUsersResponse struct {
	Users []User `json:"users"`
	Total int    `json:"total"`
}
