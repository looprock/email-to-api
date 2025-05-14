package database

import (
	"fmt"
	"log"
	
	"gorm.io/gorm"
)

// GetMappingByEmail finds an email mapping by its email address without requiring a user ID
// This is useful for admin operations that need to work with mappings across users
func (db *DB) GetMappingByEmail(emailAddress string) (*EmailMapping, error) {
	log.Printf("Looking up mapping for email: %s", emailAddress)
	
	var mapping EmailMapping
	err := db.Where("generated_email = ?", emailAddress).First(&mapping).Error
	if err != nil {
		return nil, fmt.Errorf("failed to find mapping for email %s: %w", emailAddress, err)
	}
	
	log.Printf("Found mapping ID %d for email %s (owned by user ID: %d)", 
		mapping.ID, emailAddress, mapping.UserID)
	
	return &mapping, nil
}

// AdminDeleteEmailMapping allows admins to delete any mapping by email address
// without requiring the admin to own the mapping
func (db *DB) AdminDeleteEmailMapping(emailAddress string) error {
	log.Printf("Admin attempting to delete mapping for email: %s", emailAddress)
	
	mapping, err := db.GetMappingByEmail(emailAddress)
	if err != nil {
		return err
	}
	
	// Use a transaction to ensure all related records are deleted
	return db.Transaction(func(tx *gorm.DB) error {
		// First delete associated logs
		if result := tx.Where("mapping_id = ?", mapping.ID).Delete(&EmailLog{}); result.Error != nil {
			log.Printf("Error deleting logs: %v", result.Error)
			return fmt.Errorf("failed to delete associated email logs: %w", result.Error)
		}
		
		// Then delete the mapping itself
		if result := tx.Delete(mapping); result.Error != nil {
			log.Printf("Error deleting mapping: %v", result.Error)
			return fmt.Errorf("failed to delete email mapping: %w", result.Error)
		}
		
		log.Printf("Successfully deleted mapping ID %d for email %s (owned by user ID: %d)", 
			mapping.ID, emailAddress, mapping.UserID)
		
		return nil
	})
}