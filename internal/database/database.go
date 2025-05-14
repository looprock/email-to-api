package database

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DB wraps the database connection and provides additional functionality
type DB struct {
	*gorm.DB
	config *Config
}

// New creates a new database connection
func New(config *Config) (*DB, error) {
	var dialector gorm.Dialector

	switch config.Driver {
	case "postgres":
		dialector = postgres.Open(config.DSN)
	case "sqlite", "sqlite3": // Accept both "sqlite" and "sqlite3"
		dialector = sqlite.Open(config.DSN)
	default:
		return nil, fmt.Errorf("unsupported database driver: %s", config.Driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{
		DB:     db,
		config: config,
	}, nil
}

// Migrate runs database migrations
func (db *DB) Migrate() error {
	m, err := migrate.New("file://migrations", db.config.MigrateURL)
	if err != nil {
		return fmt.Errorf("failed to create migrate instance: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("failed to run migrations: %w", err)
		}
		log.Println("No migrations to run")
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// CreateUser creates a new user
func (db *DB) CreateUser(email, role string) (*User, error) {
	// Validate role
	role = strings.ToLower(role)
	if role != "admin" && role != "user" {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	// Check if user already exists
	email = strings.ToLower(email)
	var existingUser User
	err := db.Where("email = ?", email).First(&existingUser).Error
	if err == nil {
		// User already exists
		return &existingUser, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		// Some other error occurred
		return nil, fmt.Errorf("failed to check existing user: %w", err)
	}

	// Create new user
	user := &User{
		Email:    email,
		Role:     role,
		IsActive: true,
	}

	if err := db.Create(user).Error; err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// CreateRegistrationToken creates a new registration token for a user
func (db *DB) CreateRegistrationToken(userID uint) (*RegistrationToken, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	rt := &RegistrationToken{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := db.Create(rt).Error; err != nil {
		return nil, fmt.Errorf("failed to create registration token: %w", err)
	}

	return rt, nil
}

// SetPassword sets a user's password using their registration token
func (db *DB) SetPassword(token, password string) error {
	var rt RegistrationToken
	if err := db.Where("token = ?", token).First(&rt).Error; err != nil {
		return fmt.Errorf("invalid token")
	}

	if rt.UsedAt != nil {
		return fmt.Errorf("token already used")
	}
	if time.Now().After(rt.ExpiresAt) {
		return fmt.Errorf("token expired")
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update user's password and mark token as used
	return db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		if err := tx.Model(&User{}).Where("id = ?", rt.UserID).Update("password_hash", string(hash)).Error; err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}

		if err := tx.Model(&rt).Update("used_at", now).Error; err != nil {
			return fmt.Errorf("failed to update token: %w", err)
		}

		return nil
	})
}

// CreateEmailMapping creates a new email mapping for a user
func (db *DB) CreateEmailMapping(userID uint, endpoint, description string, headers map[string]string) (*EmailMapping, error) {
	// Try up to 3 times to generate a unique email address
	var generatedEmail string
	for attempts := 0; attempts < 3; attempts++ {
		// Generate random email address
		randomBytes := make([]byte, 16)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("failed to generate random email: %w", err)
		}
		randomPart := strings.ToLower(base64.URLEncoding.EncodeToString(randomBytes)[:12])
		generatedEmail = fmt.Sprintf("%s@%s", randomPart, db.config.Domain)

		// Check if this email already exists
		var exists bool
		if err := db.Model(&EmailMapping{}).Select("1").Where("generated_email = ?", generatedEmail).Scan(&exists).Error; err != nil {
			return nil, fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		if !exists {
			break
		}
		if attempts == 2 {
			return nil, fmt.Errorf("failed to generate unique email address after 3 attempts")
		}
	}

	mapping := &EmailMapping{
		UserID:         userID,
		GeneratedEmail: generatedEmail,
		EndpointURL:    endpoint,
		Description:    description,
		Headers:        headers,
		IsActive:       true,
	}

	if err := db.Create(mapping).Error; err != nil {
		return nil, fmt.Errorf("failed to create mapping: %w", err)
	}

	return mapping, nil
}

// GetEmailMapping retrieves the API endpoint for a given email address
func (db *DB) GetEmailMapping(emailAddress string) (*EmailMapping, error) {
	var mapping EmailMapping
	err := db.Where("generated_email = ? AND is_active = ?", emailAddress, true).First(&mapping).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email mapping: %w", err)
	}
	return &mapping, nil
}

// LogEmailProcessing logs the email processing attempt
func (db *DB) LogEmailProcessing(emailAddress, subject, status, errorMsg string, headers map[string]string, userID uint) error {
	var mapping EmailMapping
	if err := db.Where("generated_email = ? AND user_id = ?", emailAddress, userID).First(&mapping).Error; err != nil {
		return fmt.Errorf("failed to get mapping: %w", err)
	}

	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}

	log := &EmailLog{
		MappingID:    mapping.ID,
		FromAddress:  emailAddress,
		Subject:      subject,
		Status:       status,
		ErrorMessage: errorMsg,
		Headers:      string(headersJSON),
	}

	if err := db.Create(log).Error; err != nil {
		return fmt.Errorf("failed to create log: %w", err)
	}

	return nil
}

// UpdateEmailMapping updates an existing email-to-API mapping
func (db *DB) UpdateEmailMapping(emailAddress string, endpointURL string, headers map[string]string, userID uint) error {
	result := db.Model(&EmailMapping{}).
		Where("generated_email = ? AND user_id = ?", emailAddress, userID).
		Updates(map[string]interface{}{
			"endpoint_url": endpointURL,
			"headers":      headers,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update email mapping: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no mapping found for email: %s", emailAddress)
	}

	return nil
}

// DeleteEmailMapping permanently deletes an email mapping and its associated logs
func (db *DB) DeleteEmailMapping(emailAddress string, userID uint) error {
	log.Printf("Attempting to delete email mapping for %s (userID: %d)", emailAddress, userID)
	
	// First, find the mapping to get its ID
	var mapping EmailMapping
	if err := db.Where("generated_email = ? AND user_id = ?", emailAddress, userID).First(&mapping).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("No mapping found for email: %s (userID: %d)", emailAddress, userID)
			return fmt.Errorf("no mapping found for email: %s", emailAddress)
		}
		log.Printf("Error finding email mapping: %v", err)
		return fmt.Errorf("failed to find email mapping: %w", err)
	}
	
	log.Printf("Found mapping ID %d for email %s (userID: %d)", mapping.ID, emailAddress, userID)

	// Execute the deletion using raw SQL to directly handle foreign key constraints
	// Use transaction for consistency
	return db.Transaction(func(tx *gorm.DB) error {
		// Use raw SQL to delete logs first
		if result := tx.Exec("DELETE FROM email_logs WHERE mapping_id = ?", mapping.ID); result.Error != nil {
			log.Printf("Error deleting logs with SQL: %v", result.Error)
			return fmt.Errorf("failed to delete associated email logs: %w", result.Error)
		} else {
			log.Printf("Deleted %d log entries for mapping ID %d", result.RowsAffected, mapping.ID)
		}

		// Then delete the mapping with raw SQL
		if result := tx.Exec("DELETE FROM email_mappings WHERE id = ?", mapping.ID); result.Error != nil {
			log.Printf("Error deleting mapping with SQL: %v", result.Error)
			return fmt.Errorf("failed to delete email mapping: %w", result.Error)
		} else {
			log.Printf("Successfully deleted mapping ID %d for email %s", mapping.ID, emailAddress)
		}

		return nil
	})
}

// ToggleEmailMapping toggles whether an email mapping is active
func (db *DB) ToggleEmailMapping(emailAddress string, userID uint) (bool, error) {
	var mapping EmailMapping
	if err := db.Where("generated_email = ? AND user_id = ?", emailAddress, userID).First(&mapping).Error; err != nil {
		return false, fmt.Errorf("failed to get mapping: %w", err)
	}

	mapping.IsActive = !mapping.IsActive
	if err := db.Save(&mapping).Error; err != nil {
		return false, fmt.Errorf("failed to toggle mapping: %w", err)
	}

	return mapping.IsActive, nil
}

// GetUserByEmail retrieves a user by their email address
func (db *DB) GetUserByEmail(email string) (*User, error) {
	var user User
	err := db.Where("email = ? AND is_active = ?", strings.ToLower(email), true).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetUsers retrieves all users
func (db *DB) GetUsers() ([]User, error) {
	var users []User
	err := db.DB.Raw(`
		SELECT id, email, password_hash, role, 
			   created_at, updated_at, last_login, is_active 
		FROM users 
		ORDER BY created_at DESC
	`).Scan(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}
	return users, nil
}

// UpdateLastLogin updates a user's last login timestamp
func (db *DB) UpdateLastLogin(userID uint) error {
	if err := db.Model(&User{}).Where("id = ?", userID).Update("last_login", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by their ID
func (db *DB) GetUserByID(userID uint) (*User, error) {
	var user User
	err := db.Where("id = ? AND is_active = ?", userID, true).First(&user).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// ValidateRegistrationToken checks if a registration token is valid
func (db *DB) ValidateRegistrationToken(token string) (bool, error) {
	var rt RegistrationToken
	err := db.Where("token = ?", token).First(&rt).Error
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to validate token: %w", err)
	}

	// Check if token is expired or used
	if rt.UsedAt != nil || time.Now().After(rt.ExpiresAt) {
		return false, nil
	}

	return true, nil
}

// ChangePassword changes a user's password
func (db *DB) ChangePassword(userID uint, currentPassword, newPassword string) error {
	user, err := db.GetUserByID(userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return fmt.Errorf("user not found")
	}

	// If current password is provided, verify it
	if currentPassword != "" {
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
			return fmt.Errorf("invalid current password")
		}
	}

	// Hash new password
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	if err := db.Model(user).Update("password_hash", string(hash)).Error; err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// ToggleUserStatus toggles a user's active status
func (db *DB) ToggleUserStatus(userID uint) (bool, error) {
	var isActive bool
	err := db.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := tx.First(&user, userID).Error; err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		user.IsActive = !user.IsActive
		if err := tx.Save(&user).Error; err != nil {
			return fmt.Errorf("failed to update user: %w", err)
		}

		// Update all user's mappings
		if err := tx.Model(&EmailMapping{}).Where("user_id = ?", userID).Update("is_active", user.IsActive).Error; err != nil {
			return fmt.Errorf("failed to update mappings: %w", err)
		}

		isActive = user.IsActive
		return nil
	})

	return isActive, err
}

// UpdateUserRole updates a user's role
func (db *DB) UpdateUserRole(userID uint, newRole string) error {
	// Validate role
	newRole = strings.ToLower(newRole)
	if newRole != "admin" && newRole != "user" {
		return fmt.Errorf("invalid role: %s", newRole)
	}

	result := db.Model(&User{}).Where("id = ?", userID).Update("role", newRole)
	if result.Error != nil {
		return fmt.Errorf("failed to update user role: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no user found with ID: %d", userID)
	}

	return nil
}

// GetMappingsWithUsers retrieves all email mappings with their associated user information
func (db *DB) GetMappingsWithUsers() ([]EmailMapping, error) {
	var mappings []EmailMapping
	err := db.Preload("User").Order("created_at DESC").Find(&mappings).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get mappings: %w", err)
	}
	return mappings, nil
}

// GetLogsWithUsers retrieves all email logs with their associated user and mapping information
func (db *DB) GetLogsWithUsers() ([]EmailLog, error) {
	var logs []EmailLog
	err := db.Preload("Mapping.User").Order("processed_at DESC").Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}
	return logs, nil
}
