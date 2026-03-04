package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

// isSecureRequest determines if the request is over HTTPS, checking both
// direct TLS and reverse proxy headers.
func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	// Check for reverse proxy headers
	proto := r.Header.Get("X-Forwarded-Proto")
	return strings.EqualFold(proto, "https")
}

// handleLogin authenticates a user and returns a session token.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Password == "" {
		http.Error(w, "username and password required", http.StatusBadRequest)
		return
	}

	// Get user by username
	user, err := s.store.GetUserByUsername(req.Username)
	if err == store.ErrNotFound {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Check if user is active
	if !user.IsActive {
		http.Error(w, "account is disabled", http.StatusForbidden)
		return
	}

	// Verify password
	if err := s.store.VerifyPassword(user, req.Password); err != nil {
		http.Error(w, "invalid username or password", http.StatusUnauthorized)
		return
	}

	// Update last login
	if err := s.store.UpdateUserLastLogin(user.ID); err != nil {
		// Log error but don't fail the login
		_ = err
	}

	// Create session
	userAgent := r.Header.Get("User-Agent")
	ipAddress := getClientIP(r)
	session, token, err := s.store.CreateUserSession(user.ID, userAgent, ipAddress)
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "swoops_session",
		Value:    token,
		Path:     "/",
		MaxAge:   int(session.ExpiresAt.Sub(session.CreatedAt).Seconds()),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})

	// Return response
	resp := models.LoginResponse{
		User:  user,
		Token: token,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// handleLogout invalidates the current session.
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Get token from cookie or Authorization header
	token := ""
	if cookie, err := r.Cookie("swoops_session"); err == nil {
		token = cookie.Value
	} else if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token = strings.TrimPrefix(auth, "Bearer ")
	}

	if token != "" {
		// Delete session
		if err := s.store.DeleteUserSession(token); err != nil {
			// Log error but don't fail the logout
			_ = err
		}
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "swoops_session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteStrictMode,
	})

	w.WriteHeader(http.StatusNoContent)
}

// handleGetCurrentUser returns the currently authenticated user.
func (s *Server) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	// Get user from context (set by auth middleware)
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	user, err := s.store.GetUserByID(userID)
	if err == store.ErrNotFound {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

// handleCreateUser creates a new user (admin only).
// Note: Admin check is enforced by RequireAdmin middleware
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {

	var req models.CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	if req.Username == "" || req.Email == "" || req.Password == "" {
		http.Error(w, "username, email, and password required", http.StatusBadRequest)
		return
	}

	// Create user
	user, err := s.store.CreateUser(&req)
	if err != nil {
		if errors.Is(err, store.ErrDuplicateUser) {
			http.Error(w, "username or email already exists", http.StatusConflict)
			return
		}
		http.Error(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// handleListUsers returns all users (admin only).
// Note: Admin check is enforced by RequireAdmin middleware
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {

	users, err := s.store.ListUsers()
	if err != nil {
		http.Error(w, "failed to list users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(users)
}
