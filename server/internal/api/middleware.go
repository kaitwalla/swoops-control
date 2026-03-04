package api

import (
	"context"
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	contextKeyUserID  contextKey = "user_id"
	contextKeyIsAdmin contextKey = "is_admin"
)

// UserIDFromContext extracts the user ID from the request context
func UserIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKeyUserID).(string)
	return id, ok
}

// IsAdminFromContext extracts the admin status from the request context
func IsAdminFromContext(ctx context.Context) bool {
	isAdmin, _ := ctx.Value(contextKeyIsAdmin).(bool)
	return isAdmin
}

// RequireAdmin returns middleware that ensures the request is from an admin user
func (s *Server) RequireAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAdminFromContext(r.Context()) {
				writeError(w, http.StatusForbidden, "admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

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

			// Extract token from Authorization header or ?token= query param (WebSocket only)
			var token string
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					token = parts[1]
				}
			}
			// Only allow query param auth for WebSocket upgrade requests to avoid token leakage
			// in logs, browser history, and referrer headers
			if token == "" && strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				token = r.URL.Query().Get("token")
			}
			if token == "" {
				writeError(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}

			if subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) != 1 {
				writeError(w, http.StatusForbidden, "invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HybridAuth returns middleware that validates requests with either:
// 1. Session token (from cookie or Authorization header)
// 2. API key (from Authorization header)
// This allows both user-based auth and API key auth to coexist.
func (s *Server) HybridAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try session auth first (cookie or Bearer token)
			var sessionToken string
			if cookie, err := r.Cookie("swoops_session"); err == nil {
				sessionToken = cookie.Value
			}

			// If no cookie, check Authorization header
			if sessionToken == "" {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					parts := strings.SplitN(authHeader, " ", 2)
					if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
						sessionToken = parts[1]
					}
				}
			}

			// WebSocket support: allow token query param
			if sessionToken == "" && strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				sessionToken = r.URL.Query().Get("token")
			}

			// Try to authenticate with session token
			if sessionToken != "" {
				session, err := s.store.GetUserSessionByToken(sessionToken)
				if err == nil {
					// Valid session - get user and add to context
					user, err := s.store.GetUserByID(session.UserID)
					if err == nil && user.IsActive {
						// Add user info to context
						ctx := context.WithValue(r.Context(), contextKeyUserID, user.ID)
						ctx = context.WithValue(ctx, contextKeyIsAdmin, user.IsAdmin)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}

				// If session lookup failed, try API key as fallback
			}

			// Fall back to API key auth
			if s.config.Auth.APIKey == "" {
				log.Println("WARNING: API authentication is disabled")
				next.ServeHTTP(w, r)
				return
			}

			// Check if the token matches the API key
			if sessionToken != "" && subtle.ConstantTimeCompare([]byte(sessionToken), []byte(s.config.Auth.APIKey)) == 1 {
				// Valid API key - no user context, but authenticated
				next.ServeHTTP(w, r)
				return
			}

			// No valid authentication
			writeError(w, http.StatusUnauthorized, "authentication required")
		})
	}
}
