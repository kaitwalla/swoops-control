package models

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
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
