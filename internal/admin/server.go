package admin

import (
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/looprock/email-to-api/internal/config"
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
	ID             int64     `gorm:"column:id"`
	EmailAddress   string    `gorm:"column:from_address"`
	Subject        string    `gorm:"column:subject"`
	ProcessedAt    time.Time `gorm:"column:processed_at"`
	Status         string    `gorm:"column:status"`
	ErrorMessage   string    `gorm:"column:error_message"`
	APIEndpoint    string    `gorm:"column:endpoint_url"`
	GeneratedEmail string    `gorm:"column:generated_email"`
	Headers        string    `gorm:"column:headers"`
	UserEmail      string    `gorm:"column:user_email"`
}

// TableName specifies the table name for GORM
func (LogEntry) TableName() string {
	return "email_logs"
}

// UsersData represents the data for users page
type UsersData struct {
	Users       []database.User
	Error       string
	Success     string
	CurrentPage string
	UserRole    string
	UserEmail   string
	Token       string
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
	UserID      uint
	UserRole    string
	IsAdmin     bool
	CurrentPage string
	UserEmail   string
}

// New creates a new admin server
func New(db *database.DB, cfg *config.Config) (*Server, error) {
	// Parse both templates with a base template
	tmpl := template.New("").Funcs(template.FuncMap{
		"eq": func(a, b string) bool { return a == b },
	})

	tmpl, err := tmpl.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	emailer, err := email.NewMailgunSender(cfg.Mailgun.SiteDomain)
	if err != nil {
		return nil, fmt.Errorf("failed to create email sender: %w", err)
	}

	// Note: emailer can be nil if Mailgun is not configured
	server := &Server{
		db:       db,
		tmpl:     tmpl,
		sessions: NewSessionManager(),
		emailer:  emailer,
	}

	if emailer == nil {
		log.Println("Warning: Email sending is not configured. Users will need to be configured manually.")
	}

	return server, nil
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
	userID := r.Context().Value(userIDKey).(uint)
	userRole := r.Context().Value(userRoleKey).(string)

	var mappings []database.EmailMapping
	query := s.db.DB.Preload("User") // Preload the User relationship

	if userRole != "admin" {
		// Regular users only see their own mappings
		query = query.Where("user_id = ?", userID)
	}

	// Get mappings with user information
	err := query.Order("created_at DESC").Find(&mappings).Error

	if err != nil {
		log.Printf("Database error fetching mappings: %v", err)
		data.Error = fmt.Sprintf("Failed to fetch mappings: %v", err)
		s.tmpl.ExecuteTemplate(w, "layout.html", data)
		return
	}

	data.Mappings = mappings
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
	userID := r.Context().Value(userIDKey).(uint)
	userRole := r.Context().Value(userRoleKey).(string)

	var logs []LogEntry
	query := s.db.DB.
		Table("email_logs l").
		Select(`l.id, l.from_address, l.subject, l.processed_at, l.status, l.error_message, 
			l.headers, m.endpoint_url, m.generated_email, u.email as user_email`).
		Joins("LEFT JOIN email_mappings m ON l.mapping_id = m.id").
		Joins("LEFT JOIN users u ON m.user_id = u.id")

	if userRole != "admin" {
		// Regular users only see their own logs
		query = query.Where("m.user_id = ?", userID)
	}

	err := query.
		Order("l.processed_at DESC").
		Limit(100).
		Find(&logs).Error

	if err != nil {
		log.Printf("Failed to fetch logs: %v", err)
		data.Error = "Failed to fetch logs"
		s.tmpl.ExecuteTemplate(w, "layout.html", data)
		return
	}

	data.Logs = logs
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
	userID := r.Context().Value(userIDKey).(uint)

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
		Token:       s.sessions.GenerateCSRFToken(),
	}

	if r.Method == "POST" {
		// Validate CSRF token
		token := r.FormValue("token")
		if !s.sessions.ValidateCSRFToken(token) {
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		email := r.FormValue("email")
		role := r.FormValue("role")

		user, err := s.db.CreateUser(email, role)
		if err != nil {
			data.Error = fmt.Sprintf("Failed to create user: %v", err)
		} else {
			// Check if we need to create a registration token
			if user.PasswordHash == "" {
				if s.emailer == nil {
					data.Error = "Email sending is not configured. Please configure email sending or manually set password."
				} else {
					// Create registration token
					regToken, err := s.db.CreateRegistrationToken(user.ID)
					if err != nil {
						data.Error = fmt.Sprintf("Failed to create registration token: %v", err)
					} else {
						// Send registration email
						if err := s.emailer.SendRegistrationEmail(email, regToken.Token); err != nil {
							log.Printf("Failed to send registration email: %v", err)
							data.Error = fmt.Sprintf("User created but failed to send registration email: %v", err)
						} else {
							data.Success = fmt.Sprintf("User created successfully. Registration email sent to %s", email)
						}
					}
				}
			} else {
				data.Success = fmt.Sprintf("User %s already exists", email)
			}
		}
	}

	// Get all users
	users, err := s.db.GetUsers()
	if err != nil {
		data.Error = fmt.Sprintf("Failed to fetch users: %v", err)
	} else {
		data.Users = users
	}

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
	var targetUserID uint
	var isAdminChangingOther bool

	if userIDStr := r.URL.Query().Get("user_id"); userIDStr != "" {
		// Admin changing another user's password
		var err error
		parsed, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusBadRequest)
			return
		}
		targetUserID = uint(parsed)
		// Verify admin role
		if r.Context().Value(userRoleKey).(string) != "admin" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		isAdminChangingOther = true
	} else {
		// User changing their own password
		targetUserID = r.Context().Value(userIDKey).(uint)
	}

	data := PasswordData{
		UserID:      targetUserID,
		UserRole:    r.Context().Value(userRoleKey).(string),
		CurrentPage: "change_password",
		UserEmail:   r.Context().Value("userEmail").(string),
		IsAdmin:     r.Context().Value(userRoleKey).(string) == "admin",
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

		// Only verify current password if user is changing their own password
		// or if a non-admin is trying to change a password
		if !isAdminChangingOther {
			currentUserID := r.Context().Value(userIDKey).(uint)
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

	parsed, err := strconv.ParseUint(r.FormValue("user_id"), 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	userID := uint(parsed)

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

	parsed, err := strconv.ParseUint(r.FormValue("user_id"), 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}
	userID := uint(parsed)

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
