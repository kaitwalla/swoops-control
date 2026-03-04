package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/models"
	"golang.org/x/crypto/bcrypt"
)

// CreateUser creates a new user with hashed password.
func (s *Store) CreateUser(req *models.CreateUserRequest) (*models.User, error) {
	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &models.User{
		ID:           models.NewID(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hashedPassword),
		FullName:     req.FullName,
		IsActive:     true,
		IsAdmin:      req.IsAdmin,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = s.db.Exec(`
		INSERT INTO users (id, username, email, password_hash, full_name, is_active, is_admin, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.Email, user.PasswordHash, user.FullName, user.IsActive, user.IsAdmin, user.CreatedAt, user.UpdatedAt)

	if err != nil {
		// Check for UNIQUE constraint violation
		if isSQLiteDuplicateColumnError(err) || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrDuplicateUser
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}

	return user, nil
}

// GetUserByUsername retrieves a user by username.
func (s *Store) GetUserByUsername(username string) (*models.User, error) {
	user := &models.User{}
	var lastLoginAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, username, email, password_hash, full_name, is_active, is_admin, last_login_at, created_at, updated_at
		FROM users
		WHERE username = ?
	`, username).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.FullName, &user.IsActive, &user.IsAdmin,
		&lastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}

	return user, nil
}

// GetUserByID retrieves a user by ID.
func (s *Store) GetUserByID(id string) (*models.User, error) {
	user := &models.User{}
	var lastLoginAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, username, email, password_hash, full_name, is_active, is_admin, last_login_at, created_at, updated_at
		FROM users
		WHERE id = ?
	`, id).Scan(
		&user.ID, &user.Username, &user.Email, &user.PasswordHash,
		&user.FullName, &user.IsActive, &user.IsAdmin,
		&lastLoginAt, &user.CreatedAt, &user.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user: %w", err)
	}

	if lastLoginAt.Valid {
		user.LastLoginAt = &lastLoginAt.Time
	}

	return user, nil
}

// VerifyPassword checks if the provided password matches the user's hashed password.
func (s *Store) VerifyPassword(user *models.User, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
}

// UpdateUserLastLogin updates the user's last login timestamp.
func (s *Store) UpdateUserLastLogin(userID string) error {
	now := time.Now()
	_, err := s.db.Exec(`
		UPDATE users
		SET last_login_at = ?, updated_at = ?
		WHERE id = ?
	`, now, now, userID)

	if err != nil {
		return fmt.Errorf("update last login: %w", err)
	}

	return nil
}

// ResetUserPassword resets a user's password by username.
func (s *Store) ResetUserPassword(username, newPassword string) error {
	// Hash the new password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	// Update the password
	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE users
		SET password_hash = ?, updated_at = ?
		WHERE username = ?
	`, string(hashedPassword), now, username)

	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	// Check if user was found
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// CreateUserSession creates a new session token for a user.
func (s *Store) CreateUserSession(userID, userAgent, ipAddress string) (*models.UserSession, string, error) {
	// Generate session token
	token, err := models.GenerateAuthToken()
	if err != nil {
		return nil, "", fmt.Errorf("generate token: %w", err)
	}

	// Hash token for storage
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	session := &models.UserSession{
		ID:        models.NewID(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour), // 7 days
		UserAgent: userAgent,
		IPAddress: ipAddress,
		CreatedAt: time.Now(),
	}

	_, err = s.db.Exec(`
		INSERT INTO user_sessions (id, user_id, token_hash, expires_at, user_agent, ip_address, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.UserID, session.TokenHash, session.ExpiresAt, session.UserAgent, session.IPAddress, session.CreatedAt)

	if err != nil {
		return nil, "", fmt.Errorf("insert session: %w", err)
	}

	return session, token, nil
}

// GetUserSessionByToken retrieves a session by its token and validates expiry.
func (s *Store) GetUserSessionByToken(token string) (*models.UserSession, error) {
	// Hash token to look up
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	session := &models.UserSession{}

	err := s.db.QueryRow(`
		SELECT id, user_id, token_hash, expires_at, user_agent, ip_address, created_at
		FROM user_sessions
		WHERE token_hash = ? AND expires_at > ?
	`, tokenHash, time.Now()).Scan(
		&session.ID, &session.UserID, &session.TokenHash,
		&session.ExpiresAt, &session.UserAgent, &session.IPAddress, &session.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}

	return session, nil
}

// DeleteUserSession deletes a session (logout).
func (s *Store) DeleteUserSession(token string) error {
	// Hash token to delete
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	_, err := s.db.Exec(`DELETE FROM user_sessions WHERE token_hash = ?`, tokenHash)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

// CleanupExpiredSessions removes all expired session tokens.
func (s *Store) CleanupExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM user_sessions WHERE expires_at < ?`, time.Now())
	if err != nil {
		return fmt.Errorf("cleanup sessions: %w", err)
	}

	return nil
}

// ListUsers retrieves all users (for admin purposes).
// Password hashes are excluded for security.
// Default limit is 100 users to prevent loading entire table.
func (s *Store) ListUsers() ([]*models.User, error) {
	rows, err := s.db.Query(`
		SELECT id, username, email, full_name, is_active, is_admin, last_login_at, created_at, updated_at
		FROM users
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		user := &models.User{}
		var lastLoginAt sql.NullTime

		err := rows.Scan(
			&user.ID, &user.Username, &user.Email,
			&user.FullName, &user.IsActive, &user.IsAdmin,
			&lastLoginAt, &user.CreatedAt, &user.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}

		if lastLoginAt.Valid {
			user.LastLoginAt = &lastLoginAt.Time
		}

		users = append(users, user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}

	return users, nil
}
