package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

// DB wraps the SQL database connection
type DB struct {
	*sql.DB
	domain string
}

// User represents a user in the system
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	Role         string
	CreatedAt    time.Time
	LastLogin    *time.Time
	IsActive     bool
}

// EmailMapping represents a mapping between an email address and an API endpoint
type EmailMapping struct {
	ID             int64
	UserID         int64
	GeneratedEmail string
	EndpointURL    string
	Description    string
	Headers        map[string]string
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	UserEmail      string
}

// RegistrationToken represents a token for user registration
type RegistrationToken struct {
	ID        int64
	UserID    int64
	Token     string
	ExpiresAt time.Time
	UsedAt    *time.Time
}

// New creates a new database connection
func New(dbPath, domain string) (*DB, error) {
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db, domain}, nil
}

// CreateUser creates a new user
func (db *DB) CreateUser(email, role string) (*User, error) {
	// Validate role
	role = strings.ToLower(role)
	if role != "admin" && role != "user" {
		return nil, fmt.Errorf("invalid role: %s", role)
	}

	// Create user without password (will be set during registration)
	query := `
		INSERT INTO users (email, role, created_at, is_active)
		VALUES (?, ?, ?, TRUE)
		RETURNING id, email, role, created_at, is_active
	`

	user := &User{}
	err := db.QueryRow(
		query,
		strings.ToLower(email),
		role,
		time.Now(),
	).Scan(
		&user.ID,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
		&user.IsActive,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return user, nil
}

// CreateRegistrationToken creates a new registration token for a user
func (db *DB) CreateRegistrationToken(userID int64) (*RegistrationToken, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Insert token with 24-hour expiry
	query := `
		INSERT INTO registration_tokens (user_id, token, expires_at)
		VALUES (?, ?, ?)
		RETURNING id, user_id, token, expires_at
	`

	rt := &RegistrationToken{}
	expiresAt := time.Now().Add(24 * time.Hour)
	err := db.QueryRow(
		query,
		userID,
		token,
		expiresAt,
	).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.Token,
		&rt.ExpiresAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create registration token: %w", err)
	}

	return rt, nil
}

// SetPassword sets a user's password using their registration token
func (db *DB) SetPassword(token, password string) error {
	// Get and validate token
	query := `
		SELECT id, user_id, expires_at, used_at
		FROM registration_tokens
		WHERE token = ?
	`

	var rt RegistrationToken
	err := db.QueryRow(query, token).Scan(
		&rt.ID,
		&rt.UserID,
		&rt.ExpiresAt,
		&rt.UsedAt,
	)

	if err == sql.ErrNoRows {
		return fmt.Errorf("invalid token")
	}
	if err != nil {
		return fmt.Errorf("failed to get token: %w", err)
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
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update password
	_, err = tx.Exec(
		"UPDATE users SET password_hash = ? WHERE id = ?",
		string(hash),
		rt.UserID,
	)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Mark token as used
	_, err = tx.Exec(
		"UPDATE registration_tokens SET used_at = ? WHERE id = ?",
		time.Now(),
		rt.ID,
	)
	if err != nil {
		return fmt.Errorf("failed to update token: %w", err)
	}

	return tx.Commit()
}

// CreateEmailMapping creates a new email mapping for a user
func (db *DB) CreateEmailMapping(userID int64, endpoint, description string, headers map[string]string) (*EmailMapping, error) {
	// Try up to 3 times to generate a unique email address
	var generatedEmail string
	for attempts := 0; attempts < 3; attempts++ {
		// Generate random email address
		randomBytes := make([]byte, 16)
		if _, err := rand.Read(randomBytes); err != nil {
			return nil, fmt.Errorf("failed to generate random email: %w", err)
		}
		randomPart := strings.ToLower(base64.URLEncoding.EncodeToString(randomBytes)[:12])
		generatedEmail = fmt.Sprintf("%s@%s", randomPart, db.domain)

		// Check if this email already exists
		var exists bool
		err := db.QueryRow("SELECT 1 FROM email_mappings WHERE generated_email = ?", generatedEmail).Scan(&exists)
		if err == sql.ErrNoRows {
			// Email is unique, proceed with creation
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to check email uniqueness: %w", err)
		}
		// If we get here, the email exists, try again
		if attempts == 2 {
			return nil, fmt.Errorf("failed to generate unique email address after 3 attempts")
		}
	}

	// Convert headers to JSON
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal headers: %w", err)
	}

	// Insert mapping
	query := `
		INSERT INTO email_mappings (
			user_id, generated_email, endpoint_url, description,
			headers, is_active, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, TRUE, ?, ?)
		RETURNING id, user_id, generated_email, endpoint_url,
			description, headers, is_active, created_at, updated_at
	`

	now := time.Now()
	mapping := &EmailMapping{Headers: make(map[string]string)}
	var headersStr string

	err = db.QueryRow(
		query,
		userID,
		generatedEmail,
		endpoint,
		description,
		string(headersJSON),
		now,
		now,
	).Scan(
		&mapping.ID,
		&mapping.UserID,
		&mapping.GeneratedEmail,
		&mapping.EndpointURL,
		&mapping.Description,
		&headersStr,
		&mapping.IsActive,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create mapping: %w", err)
	}

	if err := json.Unmarshal([]byte(headersStr), &mapping.Headers); err != nil {
		return nil, fmt.Errorf("failed to parse headers: %w", err)
	}

	return mapping, nil
}

// GetEmailMapping retrieves the API endpoint for a given email address
func (db *DB) GetEmailMapping(emailAddress string) (*EmailMapping, error) {
	query := `
		SELECT id, user_id, generated_email, endpoint_url,
			description, headers, is_active, created_at, updated_at
		FROM email_mappings 
		WHERE generated_email = ? AND is_active = TRUE
	`

	mapping := &EmailMapping{Headers: make(map[string]string)}
	var headersJSON string
	err := db.QueryRow(query, emailAddress).Scan(
		&mapping.ID,
		&mapping.UserID,
		&mapping.GeneratedEmail,
		&mapping.EndpointURL,
		&mapping.Description,
		&headersJSON,
		&mapping.IsActive,
		&mapping.CreatedAt,
		&mapping.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get email mapping: %w", err)
	}

	if err := json.Unmarshal([]byte(headersJSON), &mapping.Headers); err != nil {
		return nil, fmt.Errorf("failed to parse headers: %w", err)
	}

	return mapping, nil
}

// LogEmailProcessing logs the email processing attempt
func (db *DB) LogEmailProcessing(emailAddress, subject, status, errorMsg, apiEndpoint string, headers map[string]string, userID int64) error {
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}

	// Get the mapping ID for this email address
	var mappingID int64
	err = db.QueryRow("SELECT id FROM email_mappings WHERE generated_email = ? AND user_id = ?", emailAddress, userID).Scan(&mappingID)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to get mapping ID: %w", err)
	}

	query := `
		INSERT INTO email_logs (
			mapping_id, from_address, subject, body,
			processed_at, status, error_message, headers
		)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?)
	`

	_, err = db.Exec(query,
		mappingID,
		emailAddress,
		subject,
		"", // body is optional
		status,
		errorMsg,
		string(headersJSON),
	)
	if err != nil {
		return fmt.Errorf("failed to log email processing: %w", err)
	}

	return nil
}

// UpdateEmailMapping updates an existing email-to-API mapping
func (db *DB) UpdateEmailMapping(emailAddress, endpointURL string, headers map[string]string, userID int64) error {
	headersJSON, err := json.Marshal(headers)
	if err != nil {
		return fmt.Errorf("failed to marshal headers: %w", err)
	}

	query := `
		UPDATE email_mappings 
		SET endpoint_url = ?, headers = ?, updated_at = CURRENT_TIMESTAMP
		WHERE generated_email = ? AND user_id = ?
	`

	result, err := db.Exec(query, endpointURL, string(headersJSON), emailAddress, userID)
	if err != nil {
		return fmt.Errorf("failed to update email mapping: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no mapping found for email: %s", emailAddress)
	}

	return nil
}

// DeleteEmailMapping permanently deletes an email mapping
func (db *DB) DeleteEmailMapping(emailAddress string, userID int64) error {
	query := `
		DELETE FROM email_mappings 
		WHERE generated_email = ? AND user_id = ?
	`

	result, err := db.Exec(query, emailAddress, userID)
	if err != nil {
		return fmt.Errorf("failed to delete email mapping: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no mapping found for email: %s", emailAddress)
	}

	return nil
}

// ToggleEmailMapping toggles whether an email mapping is active for receiving emails
func (db *DB) ToggleEmailMapping(emailAddress string, userID int64) (bool, error) {
	query := `
		UPDATE email_mappings 
		SET is_active = NOT is_active, updated_at = CURRENT_TIMESTAMP
		WHERE generated_email = ? AND user_id = ?
		RETURNING is_active
	`

	var isActive bool
	err := db.QueryRow(query, emailAddress, userID).Scan(&isActive)
	if err != nil {
		return false, fmt.Errorf("failed to toggle email mapping: %w", err)
	}

	return isActive, nil
}

// GetUserByEmail retrieves a user by their email address
func (db *DB) GetUserByEmail(email string) (*User, error) {
	// fmt.Printf("DEBUG: GetUserByEmail searching for: %q\n", strings.ToLower(email))

	// Print all emails in the users table for debugging
	rows, err := db.Query("SELECT email FROM users")
	if err == nil {
		// fmt.Println("DEBUG: Emails in users table:")
		for rows.Next() {
			var e string
			if err := rows.Scan(&e); err == nil {
				// fmt.Printf("  - %q\n", e)
			}
		}
		rows.Close()
	}

	query := `
		SELECT id, email, password_hash, role, created_at, last_login, is_active
		FROM users
		WHERE email = ? AND is_active = 1
	`
	// fmt.Printf("DEBUG: Executing query: %s with email=%q\n", query, strings.ToLower(email))

	user := &User{}
	row := db.QueryRow(query, strings.ToLower(email))
	err = row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.LastLogin,
		&user.IsActive,
	)

	if err == sql.ErrNoRows {
		fmt.Printf("DEBUG: No rows returned from query\n")
		return nil, nil
	}
	if err != nil {
		fmt.Printf("DEBUG: Error scanning row: %v\n", err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// fmt.Printf("DEBUG: Found user with ID=%d email=%q role=%q is_active=%v\n",
	// 	user.ID, user.Email, user.Role, user.IsActive)

	return user, nil
}

// GetUsers retrieves all users
func (db *DB) GetUsers() ([]User, error) {
	query := `
		SELECT id, email, role, created_at, last_login, is_active
		FROM users
		ORDER BY created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Role,
			&user.CreatedAt,
			&user.LastLogin,
			&user.IsActive,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after scanning users: %w", err)
	}

	return users, nil
}

// UpdateLastLogin updates a user's last login timestamp
func (db *DB) UpdateLastLogin(userID int64) error {
	query := `UPDATE users SET last_login = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to update last login: %w", err)
	}
	return nil
}

// GetUserByID retrieves a user by their ID
func (db *DB) GetUserByID(userID int64) (*User, error) {
	query := `
		SELECT id, email, password_hash, role, created_at, last_login, is_active
		FROM users
		WHERE id = ? AND is_active = 1
	`
	user := &User{}
	row := db.QueryRow(query, userID)
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.LastLogin,
		&user.IsActive,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ValidateRegistrationToken checks if a registration token is valid
func (db *DB) ValidateRegistrationToken(token string) (bool, error) {
	query := `
		SELECT expires_at, used_at
		FROM registration_tokens
		WHERE token = ?
	`

	var expiresAt time.Time
	var usedAt *time.Time
	err := db.QueryRow(query, token).Scan(&expiresAt, &usedAt)

	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to validate token: %w", err)
	}

	// Check if token is expired or used
	if usedAt != nil || time.Now().After(expiresAt) {
		return false, nil
	}

	return true, nil
}

// ChangePassword changes a user's password
func (db *DB) ChangePassword(userID int64, currentPassword, newPassword string) error {
	// Get the user to verify current password
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

	// Update password
	_, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hash), userID)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// ToggleUserStatus toggles a user's active status
func (db *DB) ToggleUserStatus(userID int64) (bool, error) {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return false, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Toggle user status
	query := `
		UPDATE users 
		SET is_active = NOT is_active
		WHERE id = ?
		RETURNING is_active
	`

	var isActive bool
	err = tx.QueryRow(query, userID).Scan(&isActive)
	if err != nil {
		return false, fmt.Errorf("failed to toggle user status: %w", err)
	}

	// If user is deactivated, deactivate all their mappings
	// If user is reactivated, reactivate all their mappings
	query = `
		UPDATE email_mappings 
		SET is_active = ?, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = ?
	`
	if _, err := tx.Exec(query, isActive, userID); err != nil {
		return false, fmt.Errorf("failed to update user mappings: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return isActive, nil
}

// UpdateUserRole updates a user's role
func (db *DB) UpdateUserRole(userID int64, newRole string) error {
	// Validate role
	newRole = strings.ToLower(newRole)
	if newRole != "admin" && newRole != "user" {
		return fmt.Errorf("invalid role: %s", newRole)
	}

	query := `
		UPDATE users 
		SET role = ?
		WHERE id = ?
	`

	result, err := db.Exec(query, newRole, userID)
	if err != nil {
		return fmt.Errorf("failed to update user role: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no user found with ID: %d", userID)
	}

	return nil
}

// GetMappingsWithUsers retrieves all email mappings with their associated user information
func (db *DB) GetMappingsWithUsers() ([]struct {
	EmailMapping
	UserEmail string
}, error) {
	query := `
		SELECT 
			m.id, m.user_id, m.generated_email, m.endpoint_url,
			m.description, m.headers, m.is_active, m.created_at, m.updated_at,
			u.email as user_email
		FROM email_mappings m
		JOIN users u ON m.user_id = u.id
		ORDER BY m.created_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query mappings: %w", err)
	}
	defer rows.Close()

	var mappings []struct {
		EmailMapping
		UserEmail string
	}

	for rows.Next() {
		var mapping struct {
			EmailMapping
			UserEmail string
		}
		var headersJSON string

		err := rows.Scan(
			&mapping.ID,
			&mapping.UserID,
			&mapping.GeneratedEmail,
			&mapping.EndpointURL,
			&mapping.Description,
			&headersJSON,
			&mapping.IsActive,
			&mapping.CreatedAt,
			&mapping.UpdatedAt,
			&mapping.UserEmail,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan mapping: %w", err)
		}

		mapping.Headers = make(map[string]string)
		if err := json.Unmarshal([]byte(headersJSON), &mapping.Headers); err != nil {
			return nil, fmt.Errorf("failed to parse headers: %w", err)
		}

		mappings = append(mappings, mapping)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after scanning mappings: %w", err)
	}

	return mappings, nil
}

// GetLogsWithUsers retrieves all email logs with their associated user and mapping information
func (db *DB) GetLogsWithUsers() ([]struct {
	ID             int64
	EmailAddress   string
	Subject        string
	ProcessedAt    time.Time
	Status         string
	ErrorMessage   string
	APIEndpoint    string
	Headers        map[string]string
	UserID         int64
	UserEmail      string
	GeneratedEmail string
}, error) {
	query := `
		SELECT 
			l.id,
			l.from_address,
			l.subject,
			l.processed_at,
			l.status,
			l.error_message,
			m.endpoint_url,
			m.headers,
			m.user_id,
			u.email as user_email,
			m.generated_email
		FROM email_logs l
		JOIN email_mappings m ON l.mapping_id = m.id
		JOIN users u ON m.user_id = u.id
		ORDER BY l.processed_at DESC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []struct {
		ID             int64
		EmailAddress   string
		Subject        string
		ProcessedAt    time.Time
		Status         string
		ErrorMessage   string
		APIEndpoint    string
		Headers        map[string]string
		UserID         int64
		UserEmail      string
		GeneratedEmail string
	}

	for rows.Next() {
		var log struct {
			ID             int64
			EmailAddress   string
			Subject        string
			ProcessedAt    time.Time
			Status         string
			ErrorMessage   string
			APIEndpoint    string
			Headers        map[string]string
			UserID         int64
			UserEmail      string
			GeneratedEmail string
		}
		var headersJSON string

		err := rows.Scan(
			&log.ID,
			&log.EmailAddress,
			&log.Subject,
			&log.ProcessedAt,
			&log.Status,
			&log.ErrorMessage,
			&log.APIEndpoint,
			&headersJSON,
			&log.UserID,
			&log.UserEmail,
			&log.GeneratedEmail,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log: %w", err)
		}

		log.Headers = make(map[string]string)
		if err := json.Unmarshal([]byte(headersJSON), &log.Headers); err != nil {
			return nil, fmt.Errorf("failed to parse headers: %w", err)
		}

		logs = append(logs, log)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error after scanning logs: %w", err)
	}

	return logs, nil
}
