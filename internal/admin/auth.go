package admin

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Session represents a user session
type Session struct {
	Token     string
	UserID    uint
	Role      string
	ExpiresAt time.Time
}

// SessionManager handles user sessions
type SessionManager struct {
	sessions   map[string]Session
	csrfTokens map[string]time.Time
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:   make(map[string]Session),
		csrfTokens: make(map[string]time.Time),
	}
}

// CreateSession creates a new session for a user
func (sm *SessionManager) CreateSession(userID uint, role string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("failed to generate session token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store session
	sm.sessions[token] = Session{
		Token:     token,
		UserID:    userID,
		Role:      role,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	return token, nil
}

// GetSession retrieves a session by token
func (sm *SessionManager) GetSession(token string) *Session {
	if session, exists := sm.sessions[token]; exists {
		if time.Now().Before(session.ExpiresAt) {
			return &session
		}
		delete(sm.sessions, token)
	}
	return nil
}

// ClearSession removes a session
func (sm *SessionManager) ClearSession(token string) {
	delete(sm.sessions, token)
}

// GenerateCSRFToken generates a new CSRF token
func (sm *SessionManager) GenerateCSRFToken() string {
	// Generate random token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return ""
	}
	token := base64.URLEncoding.EncodeToString(tokenBytes)

	// Store token with expiration
	sm.csrfTokens[token] = time.Now().Add(1 * time.Hour)

	return token
}

// ValidateCSRFToken validates a CSRF token
func (sm *SessionManager) ValidateCSRFToken(token string) bool {
	if expiresAt, exists := sm.csrfTokens[token]; exists {
		if time.Now().Before(expiresAt) {
			return true
		}
		delete(sm.csrfTokens, token)
	}
	return false
}

// RequireAuth middleware ensures the user is authenticated
func (s *Server) RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie("session")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Validate session
		session := s.sessions.GetSession(cookie.Value)
		if session == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Fetch user email from DB
		user, err := s.db.GetUserByID(session.UserID)
		userEmail := ""
		if err == nil && user != nil {
			userEmail = user.Email
		}

		// Add user info to context
		ctx := r.Context()
		ctx = context.WithValue(ctx, userIDKey, session.UserID)
		ctx = context.WithValue(ctx, userRoleKey, session.Role)
		ctx = context.WithValue(ctx, "userEmail", userEmail)
		next(w, r.WithContext(ctx))
	}
}

// RequireAdmin middleware ensures the user is an admin
func (s *Server) RequireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := r.Context().Value(userRoleKey).(string)
		if role != "admin" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// HandleLogin handles user login
func (s *Server) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		s.tmpl.ExecuteTemplate(w, "login.html", nil)
		return
	}

	// Handle POST (login attempt)
	email := r.FormValue("email")
	password := r.FormValue("password")

	fmt.Printf("INFO: Login attempt email=%q\n", email)

	// Get user from database
	user, err := s.db.GetUserByEmail(email)
	if user != nil {
		fmt.Printf("INFO: Found user: email=%q, role=%q, is_active=%v\n", user.Email, user.Role, user.IsActive)
	} else {
		fmt.Println("INFO: No user found for email")
	}
	if err != nil || user == nil {
		s.tmpl.ExecuteTemplate(w, "login.html", map[string]string{
			"Error": "Invalid email or password",
		})
		return
	}

	// fmt.Printf("DEBUG: Comparing hash=%q with password=%q\n", user.PasswordHash, password)

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		// fmt.Printf("DEBUG: Password check failed: %v\n", err)
		s.tmpl.ExecuteTemplate(w, "login.html", map[string]string{
			"Error": "Invalid email or password",
		})
		return
	}

	// Create session
	token, err := s.sessions.CreateSession(user.ID, user.Role)
	if err != nil {
		s.tmpl.ExecuteTemplate(w, "login.html", map[string]string{
			"Error": "Failed to create session",
		})
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 hours
	})

	// Redirect to home
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// HandleLogout handles user logout
func (s *Server) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	// Clear session from memory
	if cookie, err := r.Cookie("session"); err == nil {
		s.sessions.ClearSession(cookie.Value)
	}

	// Redirect to login
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
