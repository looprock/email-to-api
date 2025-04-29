package utils

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// GenerateRandomString creates a random string of specified length
func GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// GenerateEmailAddress creates a random email address for the given domain
func GenerateEmailAddress(randomLength int, domain string) (string, error) {
	random, err := GenerateRandomString(randomLength)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s@%s", strings.ToLower(random), domain), nil
}

// HashPassword creates a bcrypt hash of the password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword compares a password against a hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateJWT creates a new JWT token for a user
func GenerateJWT(userID int64, email, role string, secret string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"role":    role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})
	return token.SignedString([]byte(secret))
}

// GenerateRegistrationToken creates a new registration token
func GenerateRegistrationToken() (string, error) {
	return GenerateRandomString(64)
}

// ValidateEndpointURL ensures the endpoint URL is valid and has required format
func ValidateEndpointURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// SanitizeEmail removes whitespace and converts to lowercase
func SanitizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// IsValidRole checks if a role is valid
func IsValidRole(role string) bool {
	return role == "admin" || role == "user"
}
