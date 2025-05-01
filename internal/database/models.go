package database

import (
	"time"
)

// User represents a user in the system
type User struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"`
	Email        string    `gorm:"uniqueIndex;not null"`
	PasswordHash string    `gorm:"not null"`
	Role         string    `gorm:"not null;default:'user'"`
	IsActive     bool      `gorm:"not null;default:true"`
	CreatedAt    time.Time `gorm:"not null;autoCreateTime"`
	UpdatedAt    time.Time `gorm:"not null;autoUpdateTime"`
	LastLogin    *time.Time
}

// RegistrationToken represents a token used for user registration
type RegistrationToken struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Token     string    `gorm:"uniqueIndex;not null"`
	UserID    uint      `gorm:"not null"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time `gorm:"not null;autoCreateTime"`
	User      User      `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// EmailMapping represents an email forwarding mapping
type EmailMapping struct {
	ID             uint   `gorm:"primaryKey;autoIncrement"`
	UserID         uint   `gorm:"not null"`
	GeneratedEmail string `gorm:"uniqueIndex;not null"`
	EndpointURL    string `gorm:"not null"`
	Description    string
	Headers        map[string]string `gorm:"serializer:json"`
	IsActive       bool              `gorm:"not null;default:true"`
	CreatedAt      time.Time         `gorm:"not null;autoCreateTime"`
	UpdatedAt      time.Time         `gorm:"not null;autoUpdateTime"`
	User           User              `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE"`
}

// EmailLog represents a log of processed emails
type EmailLog struct {
	ID           uint   `gorm:"primaryKey;autoIncrement"`
	MappingID    uint   `gorm:"not null;index"`
	Subject      string `gorm:"not null"`
	FromAddress  string `gorm:"not null"`
	Status       string `gorm:"not null"`
	ErrorMessage string
	Headers      string       `gorm:"type:text"`
	ProcessedAt  time.Time    `gorm:"not null;autoCreateTime"`
	Mapping      EmailMapping `gorm:"foreignKey:MappingID;constraint:OnDelete:CASCADE"`
}
