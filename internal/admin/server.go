package admin

import (
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/looprock/email-to-api/internal/database"
	"github.com/looprock/email-to-api/internal/email"

	"golang.org/x/crypto/bcrypt"
)

//go:embed templates/*.html
var templateFS embed.FS

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// userIDKey is the context key for the user ID
	userIDKey   contextKey = "userID"
	userRoleKey contextKey = "userRole"
)

// Server represents the admin interface server
type Server struct {
	db       *database.DB
	tmpl     *template.Template
	sessions *SessionManager
	emailer  *email.Sender
}

// EmailMappingData represents the data for email mappings page
type EmailMappingData struct {
	Mappings    []database.EmailMapping
	Error       string
	Success     string
	CurrentPage string
	UserRole    string
	UserEmail   string
	Token       string
}

// LogData represents the data for logs page
type LogData struct {
	Logs        []LogEntry
	Error       string
	CurrentPage string
	UserRole    string
	UserEmail   string
}

// LogEntry represents a log entry with formatted time
type LogEntry struct {
	ID             int64
	EmailAddress   string
	Subject        string
	ProcessedAt    string
	Status         string
	ErrorMessage   string
	APIEndpoint    string
	GeneratedEmail string
	Headers        map[string]string
	UserEmail      string
}

// UsersData represents the data for users page
type UsersData struct {
	Users       []database.User
	Error       string
	Success     string
	CurrentPage string
	UserRole    string
	UserEmail   string
}

// RegistrationData represents the data for registration page
type RegistrationData struct {
	Error   string
	Success string
	Token   string
}

// PasswordData represents the data for password change pages
type PasswordData struct {
	Error       string
	Success     string
	UserID      int64
	UserRole    string
	IsAdmin     bool
	CurrentPage string
	UserEmail   string
}

// New creates a new admin server
func New(db *database.DB) (*Server, error) {
	// Parse both templates with a base template
	tmpl := template.New("").Funcs(template.FuncMap{
		"eq": func(a, b string) bool { return a == b },
	})

	tmpl, err := tmpl.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	emailer, err := email.NewMailgunSender()
	if err != nil {
		return nil, fmt.Errorf("failed to create email sender: %w", err)
	}

	return &Server{
		db:       db,
		tmpl:     tmpl,
		sessions: NewSessionManager(),
		emailer:  emailer,
	}, nil
}

// Start starts the admin server
func (s *Server) Start(addr string) error {
	// Register routes
	mux := http.NewServeMux()

	// Auth routes
	mux.HandleFunc("/login", s.HandleLogin)
	mux.HandleFunc("/logout", s.HandleLogout)
	mux.HandleFunc("/register", s.handleRegister)
	mux.HandleFunc("/change-password", s.RequireAuth(s.handleChangePassword))

	// User management routes
	mux.HandleFunc("/users/role", s.RequireAuth(s.RequireAdmin(s.handleUserRole)))
	mux.HandleFunc("/users/toggle", s.RequireAuth(s.RequireAdmin(s.handleUserToggle)))

	// Protected routes
	mux.HandleFunc("/", s.RequireAuth(s.handleMappings))
	mux.HandleFunc("/logs", s.RequireAuth(s.handleLogs))
	mux.HandleFunc("/users", s.RequireAuth(s.RequireAdmin(s.handleUsers)))
	mux.HandleFunc("/api/mappings", s.RequireAuth(s.handleAPIMappings))

	// New HTMX routes
	mux.HandleFunc("/admin/mappings/add-form", s.RequireAuth(s.handleAddMappingForm))
	mux.HandleFunc("/admin/mappings/header-row", s.RequireAuth(s.handleHeaderRow))

	log.Printf("Starting admin server at %s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleMappings handles the email mappings page
func (s *Server) handleMappings(w http.ResponseWriter, r *http.Request) {
	data := EmailMappingData{
		CurrentPage: "mappings",
		UserRole:    r.Context().Value(userRoleKey).(string),
		UserEmail:   r.Context().Value("userEmail").(string),
		Token:       s.sessions.GenerateCSRFToken(),
	}

	// Get user ID from context
	userID := r.Context().Value(userIDKey).(int64)
	userRole := r.Context().Value(userRoleKey).(string)

	var query string
	var args []interface{}

	if userRole == "admin" {
		// Admin sees all mappings
		query = `
			SELECT m.id, m.generated_email, m.endpoint_url, m.headers, m.is_active, m.created_at, m.updated_at,
			       u.email as user_email
			FROM email_mappings m
			JOIN users u ON m.user_id = u.id
			ORDER BY m.created_at DESC
		`
	} else {
		// Regular users only see their own mappings
		query = `
			SELECT m.id, m.generated_email, m.endpoint_url, m.headers, m.is_active, m.created_at, m.updated_at,
			       u.email as user_email
			FROM email_mappings m
			JOIN users u ON m.user_id = u.id
			WHERE m.user_id = ?
			ORDER BY m.created_at DESC
		`
		args = append(args, userID)
	}

	// log.Printf("Fetching mappings with query: %s (user_id=%d, role=%s)", query, userID, userRole)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("Database error fetching mappings: %v", err)
		data.Error = fmt.Sprintf("Failed to fetch mappings: %v", err)
		s.tmpl.ExecuteTemplate(w, "layout.html", data)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var mapping database.EmailMapping
		var headersJSON string
		if err := rows.Scan(
			&mapping.ID,
			&mapping.GeneratedEmail,
			&mapping.EndpointURL,
			&headersJSON,
			&mapping.IsActive,
			&mapping.CreatedAt,
			&mapping.UpdatedAt,
			&mapping.UserEmail,
		); err != nil {
			log.Printf("Error scanning mapping row: %v", err)
			data.Error = fmt.Sprintf("Failed to read mapping: %v", err)
			continue
		}
		mapping.Headers = make(map[string]string)
		if err := json.Unmarshal([]byte(headersJSON), &mapping.Headers); err != nil {
			log.Printf("Error parsing headers JSON: %v", err)
			data.Error = fmt.Sprintf("Failed to parse headers: %v", err)
			continue
		}
		data.Mappings = append(data.Mappings, mapping)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error after scanning rows: %v", err)
		data.Error = fmt.Sprintf("Error reading mappings: %v", err)
	}

	s.tmpl.ExecuteTemplate(w, "layout.html", data)
}

// handleLogs handles the logs page
func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	data := LogData{
		CurrentPage: "logs",
		UserRole:    r.Context().Value(userRoleKey).(string),
		UserEmail:   r.Context().Value("userEmail").(string),
	}

	// Get user ID from context
	userID := r.Context().Value(userIDKey).(int64)
	userRole := r.Context().Value(userRoleKey).(string)

	// Build query based on user role
	var query string
	var args []interface{}

	if userRole == "admin" {
		// Admin sees all logs
		query = `
			SELECT 
				l.id, 
				l.from_address, 
				l.subject, 
				l.processed_at, 
				l.status, 
				l.error_message, 
				l.headers,
				m.endpoint_url,
				m.generated_email,
				u.email as user_email
			FROM email_logs l
			LEFT JOIN email_mappings m ON l.mapping_id = m.id
			LEFT JOIN users u ON m.user_id = u.id
			ORDER BY l.processed_at DESC 
			LIMIT 100
		`
	} else {
		// Regular users only see their own logs
		query = `
			SELECT 
				l.id, 
				l.from_address, 
				l.subject, 
				l.processed_at, 
				l.status, 
				l.error_message, 
				l.headers,
				m.endpoint_url,
				m.generated_email,
				u.email as user_email
			FROM email_logs l
			LEFT JOIN email_mappings m ON l.mapping_id = m.id
			LEFT JOIN users u ON m.user_id = u.id
			WHERE m.user_id = ?
			ORDER BY l.processed_at DESC 
			LIMIT 100
		`
		args = append(args, userID)
	}

	// log.Printf("Fetching logs with query: %s (user_id=%d, role=%s)", query, userID, userRole)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		log.Printf("Failed to fetch logs: %v", err)
		data.Error = "Failed to fetch logs"
		s.tmpl.ExecuteTemplate(w, "layout.html", data)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var logEntry LogEntry
		var processedAt time.Time
		var headersJSON string
		var endpointURL, generatedEmail, userEmail sql.NullString
		if err := rows.Scan(
			&logEntry.ID,
			&logEntry.EmailAddress,
			&logEntry.Subject,
			&processedAt,
			&logEntry.Status,
			&logEntry.ErrorMessage,
			&headersJSON,
			&endpointURL,
			&generatedEmail,
			&userEmail,
		); err != nil {
			log.Printf("Failed to scan log entry: %v", err)
			data.Error = "Failed to read log"
			continue
		}
		logEntry.ProcessedAt = processedAt.Format("2006-01-02 15:04:05")

		// Handle nullable fields
		if endpointURL.Valid {
			logEntry.APIEndpoint = endpointURL.String
		}
		if generatedEmail.Valid {
			logEntry.GeneratedEmail = generatedEmail.String
		}
		if userEmail.Valid {
			logEntry.UserEmail = userEmail.String
		} else {
			logEntry.UserEmail = "System" // For logs without a user (e.g., dropped emails)
		}

		// Parse headers JSON if present
		if headersJSON != "" && headersJSON != "{}" {
			var headers map[string]string
			if err := json.Unmarshal([]byte(headersJSON), &headers); err != nil {
				log.Printf("Error parsing headers JSON for log %d: %v", logEntry.ID, err)
			} else {
				logEntry.Headers = headers
			}
		}

		data.Logs = append(data.Logs, logEntry)
	}

	if err = rows.Err(); err != nil {
		log.Printf("Error after scanning rows: %v", err)
		data.Error = "Error reading logs"
	}

	s.tmpl.ExecuteTemplate(w, "layout.html", data)
}

// handleAddMappingForm renders the add mapping form template
func (s *Server) handleAddMappingForm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	s.tmpl.ExecuteTemplate(w, "add-mapping-form", token)
}

// handleHeaderRow renders a new header row template
func (s *Server) handleHeaderRow(w http.ResponseWriter, r *http.Request) {
	s.tmpl.ExecuteTemplate(w, "header-row", nil)
}

// handleAPIMappings handles API requests for email mappings
func (s *Server) handleAPIMappings(w http.ResponseWriter, r *http.Request) {
	// Get user ID from context for all operations
	userID := r.Context().Value(userIDKey).(int64)

	// Validate CSRF token for all non-GET requests
	if r.Method != "GET" {
		var token string
		if r.Method == "DELETE" {
			// For DELETE requests, get token from URL query
			token = r.URL.Query().Get("token")
		} else {
			// For other methods, get token from form data
			token = r.FormValue("token")
		}
		if !s.sessions.ValidateCSRFToken(token) {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}
	}

	switch r.Method {
	case "POST":
		// Parse form data instead of JSON
		if err := r.ParseForm(); err != nil {
			http.Error(w, fmt.Sprintf("Failed to parse form: %v", err), http.StatusBadRequest)
			return
		}

		// Collect headers from form data
		headers := make(map[string]string)
		headerNames := r.Form["header_name[]"]
		headerValues := r.Form["header_value[]"]
		for i := range headerNames {
			if headerNames[i] != "" && headerValues[i] != "" {
				headers[headerNames[i]] = headerValues[i]
			}
		}

		// Create the mapping
		if _, err := s.db.CreateEmailMapping(
			userID,
			r.FormValue("endpoint_url"),
			r.FormValue("description"),
			headers,
		); err != nil {
			log.Printf("Error creating mapping: %v", err)
			http.Error(w, fmt.Sprintf("Failed to create mapping: %v", err), http.StatusInternalServerError)
			return
		}

		// Redirect back to mappings page
		http.Redirect(w, r, "/", http.StatusSeeOther)

	case "PUT":
		emailAddress := r.FormValue("email")
		if emailAddress == "" {
			http.Error(w, "Email address required", http.StatusBadRequest)
			return
		}

		if _, err := s.db.ToggleEmailMapping(emailAddress, userID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Redirect back to mappings page
		http.Redirect(w, r, "/", http.StatusSeeOther)

	case "DELETE":
		emailAddress := r.URL.Query().Get("email")
		if emailAddress == "" {
			http.Error(w, "Email address required", http.StatusBadRequest)
			return
		}

		if err := s.db.DeleteEmailMapping(emailAddress, userID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Redirect back to mappings page
		http.Redirect(w, r, "/", http.StatusSeeOther)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUsers handles the users management page
func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	data := UsersData{
		CurrentPage: "users",
		UserRole:    r.Context().Value(userRoleKey).(string),
		UserEmail:   r.Context().Value("userEmail").(string),
	}

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			data.Error = "Failed to parse form"
			s.tmpl.ExecuteTemplate(w, "layout.html", data)
			return
		}

		email := r.FormValue("email")
		role := r.FormValue("role")

		user, err := s.db.CreateUser(email, role)
		if err != nil {
			data.Error = fmt.Sprintf("Failed to create user: %v", err)
			s.tmpl.ExecuteTemplate(w, "layout.html", data)
			return
		}

		// Create registration token
		token, err := s.db.CreateRegistrationToken(user.ID)
		if err != nil {
			data.Error = fmt.Sprintf("Failed to create registration token: %v", err)
			s.tmpl.ExecuteTemplate(w, "layout.html", data)
			return
		}

		// Send registration email
		if err := s.emailer.SendRegistrationEmail(email, token.Token); err != nil {
			log.Printf("Failed to send registration email: %v", err)
			data.Error = fmt.Sprintf("User created but failed to send registration email: %v", err)
			s.tmpl.ExecuteTemplate(w, "layout.html", data)
			return
		}

		data.Success = fmt.Sprintf("User created successfully. Registration email sent to %s", email)
	}

	// Get all users
	users, err := s.db.GetUsers()
	if err != nil {
		data.Error = fmt.Sprintf("Failed to fetch users: %v", err)
		s.tmpl.ExecuteTemplate(w, "layout.html", data)
		return
	}
	data.Users = users

	s.tmpl.ExecuteTemplate(w, "layout.html", data)
}

// handleRegister handles user registration with token
func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	data := RegistrationData{
		Token: r.URL.Query().Get("token"),
	}

	if r.Method == "GET" {
		// Verify token exists and is valid
		if data.Token == "" {
			log.Printf("Registration attempt with empty token")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		log.Printf("Validating registration token: %s", data.Token)
		valid, err := s.db.ValidateRegistrationToken(data.Token)
		if err != nil {
			log.Printf("Error validating token: %v", err)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		if !valid {
			log.Printf("Invalid or expired token: %s", data.Token)
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		log.Printf("Token valid, rendering registration form")
		if err := s.tmpl.ExecuteTemplate(w, "register.html", data); err != nil {
			log.Printf("Error rendering template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}

	if r.Method == "POST" {
		if err := r.ParseForm(); err != nil {
			data.Error = "Failed to parse form"
			s.tmpl.ExecuteTemplate(w, "register.html", data)
			return
		}

		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")
		token := r.FormValue("token")

		if password == "" || confirmPassword == "" {
			data.Error = "Password is required"
			s.tmpl.ExecuteTemplate(w, "register.html", data)
			return
		}

		if password != confirmPassword {
			data.Error = "Passwords do not match"
			s.tmpl.ExecuteTemplate(w, "register.html", data)
			return
		}

		if err := s.db.SetPassword(token, password); err != nil {
			data.Error = fmt.Sprintf("Failed to set password: %v", err)
			s.tmpl.ExecuteTemplate(w, "register.html", data)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleChangePassword handles user password changes
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	// Get the target user ID - either from query param (if admin changing other user) or from context (if changing own)
	var targetUserID int64
	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		// Admin changing another user's password
		var err error
		targetUserID, err = strconv.ParseInt(userIDStr, 10, 64)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}
		// Verify admin role
		if r.Context().Value(userRoleKey).(string) != "admin" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
	} else {
		// User changing their own password
		targetUserID = r.Context().Value(userIDKey).(int64)
	}

	data := PasswordData{
		UserID:      targetUserID,
		UserRole:    r.Context().Value(userRoleKey).(string),
		CurrentPage: "change_password",
		UserEmail:   r.Context().Value("userEmail").(string),
	}

	if r.Method == "GET" {
		if err := s.tmpl.ExecuteTemplate(w, "change_password.html", data); err != nil {
			log.Printf("Error rendering change password template: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
		return
	}

	if r.Method == "POST" {
		currentPassword := r.FormValue("current_password")
		newPassword := r.FormValue("new_password")
		confirmPassword := r.FormValue("confirm_password")

		if newPassword != confirmPassword {
			data.Error = "New passwords do not match"
			s.tmpl.ExecuteTemplate(w, "change_password.html", data)
			return
		}

		// Always verify current password of the user making the change
		currentUserID := r.Context().Value(userIDKey).(int64)
		currentUser, err := s.db.GetUserByID(currentUserID)
		if err != nil {
			data.Error = "Failed to verify credentials"
			s.tmpl.ExecuteTemplate(w, "change_password.html", data)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(currentUser.PasswordHash), []byte(currentPassword)); err != nil {
			data.Error = "Invalid current password"
			s.tmpl.ExecuteTemplate(w, "change_password.html", data)
			return
		}

		// Change target user's password
		if err := s.db.ChangePassword(targetUserID, "", newPassword); err != nil {
			data.Error = err.Error()
			s.tmpl.ExecuteTemplate(w, "change_password.html", data)
			return
		}

		data.Success = "Password changed successfully"
		s.tmpl.ExecuteTemplate(w, "change_password.html", data)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleUserRole handles updating a user's role
func (s *Server) handleUserRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	newRole := r.FormValue("role")
	if err := s.db.UpdateUserRole(userID, newRole); err != nil {
		log.Printf("Error updating user role: %v", err)
		http.Error(w, fmt.Sprintf("Failed to update role: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

// handleUserToggle handles toggling a user's active status
func (s *Server) handleUserToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, err := strconv.ParseInt(r.FormValue("user_id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	isActive, err := s.db.ToggleUserStatus(userID)
	if err != nil {
		log.Printf("Error toggling user status: %v", err)
		http.Error(w, fmt.Sprintf("Failed to toggle status: %v", err), http.StatusInternalServerError)
		return
	}

	status := "activated"
	if !isActive {
		status = "deactivated"
	}
	log.Printf("User %d %s", userID, status)

	http.Redirect(w, r, "/users", http.StatusSeeOther)
}
