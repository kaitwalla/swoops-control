package api

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
)

// contextKey for agent authentication
const contextKeyHostID contextKey = "host_id"

// HostIDFromContext extracts the host ID from the request context.
func HostIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKeyHostID).(string)
	return id, ok
}

// AgentAuth returns middleware that validates agent requests using bearer tokens.
// The token is matched against host.agent_auth_token in the database.
func (s *Server) AgentAuth() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing Authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				writeError(w, http.StatusUnauthorized, "invalid Authorization header format")
				return
			}

			token := parts[1]
			if token == "" {
				writeError(w, http.StatusUnauthorized, "empty bearer token")
				return
			}

			// Look up host by auth token
			host, err := s.store.GetHostByAuthToken(token)
			if err != nil {
				if err == sql.ErrNoRows {
					writeError(w, http.StatusUnauthorized, "invalid auth token")
					return
				}
				writeInternalError(w, err)
				return
			}

			// Add host_id to context
			ctx := context.WithValue(r.Context(), contextKeyHostID, host.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
