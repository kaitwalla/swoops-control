package api

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
)

// APIKeyAuth returns middleware that validates requests against a pre-shared API key.
// The key is expected in the Authorization header as "Bearer <key>".
// If apiKey is empty, authentication is disabled (for development only).
func APIKeyAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if apiKey == "" {
				log.Println("WARNING: API authentication is disabled — set auth.api_key in config")
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(w, http.StatusUnauthorized, "invalid Authorization header format, expected: Bearer <key>")
				return
			}

			if subtle.ConstantTimeCompare([]byte(parts[1]), []byte(apiKey)) != 1 {
				writeError(w, http.StatusForbidden, "invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
