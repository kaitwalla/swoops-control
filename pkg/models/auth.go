package models

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// GenerateAuthToken generates a cryptographically secure random token.
// Returns a URL-safe base64 encoded string of 32 bytes (256 bits).
func GenerateAuthToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// User represents a web frontend user account.
type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"` // Never serialize password hash
	FullName     string     `json:"full_name"`
	IsActive     bool       `json:"is_active"`
	IsAdmin      bool       `json:"is_admin"`
	GitHubToken  string     `json:"-"` // Never serialize GitHub token
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// UserSession represents an authenticated web session.
type UserSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // Never serialize token hash
	ExpiresAt time.Time `json:"expires_at"`
	UserAgent string    `json:"user_agent,omitempty"`
	IPAddress string    `json:"ip_address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUserRequest is the request payload for creating a new user.
type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
	IsAdmin  bool   `json:"is_admin"`
}

// LoginRequest is the request payload for user login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse is the response payload for successful login.
type LoginResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token"`
}
