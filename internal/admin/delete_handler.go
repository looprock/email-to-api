package admin

import (
	"log"
	"net/http"
)

// handleDeleteMapping is a handler for the DELETE /api/mappings/delete endpoint
// It properly handles permissions for both regular users and admins
func (s *Server) handleDeleteMapping(w http.ResponseWriter, r *http.Request) {
	// Only allow DELETE method
	if r.Method != "DELETE" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user information from context
	userID := r.Context().Value(userIDKey).(uint)
	userRole := r.Context().Value(userRoleKey).(string)

	// Validate CSRF token
	token := r.URL.Query().Get("token")
	if !s.sessions.ValidateCSRFToken(token) {
		http.Error(w, "Invalid CSRF token", http.StatusForbidden)
		return
	}

	// Get email address to delete
	emailAddress := r.URL.Query().Get("email")
	if emailAddress == "" {
		http.Error(w, "Email address required", http.StatusBadRequest)
		return
	}

	if userRole == "admin" {
		// Admin can delete any mapping
		log.Printf("Admin (user ID %d) attempting to delete mapping: %s", userID, emailAddress)
		
		// Get the mapping first to find its owner
		mapping, err := s.db.GetMappingByEmail(emailAddress)
		if err != nil {
			log.Printf("Error getting mapping: %v", err)
			http.Error(w, "Mapping not found", http.StatusNotFound)
			return
		}
		
		// Use admin function to delete the mapping
		if err := s.db.AdminDeleteEmailMapping(emailAddress); err != nil {
			log.Printf("Error deleting mapping: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		log.Printf("Admin successfully deleted mapping for email %s (owned by user ID %d)", 
			emailAddress, mapping.UserID)
	} else {
		// Regular user can only delete their own mappings
		log.Printf("User (ID %d) attempting to delete their mapping: %s", userID, emailAddress)
		if err := s.db.DeleteEmailMapping(emailAddress, userID); err != nil {
			log.Printf("Error deleting mapping: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to mappings page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}