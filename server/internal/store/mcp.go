package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/swoopsh/swoops/pkg/models"
)

// ---- Agent Status Updates ----

func (s *Store) CreateAgentStatusUpdate(update *models.AgentStatusUpdate) error {
	detailsJSON, _ := json.Marshal(update.Details)
	_, err := s.db.Exec(`
		INSERT INTO agent_status_updates (id, session_id, status_type, message, details_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		update.ID, update.SessionID, update.Type, update.Message, string(detailsJSON), update.CreatedAt,
	)
	return err
}

func (s *Store) ListAgentStatusUpdates(sessionID string, limit int) ([]*models.AgentStatusUpdate, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(`
		SELECT id, session_id, status_type, message, details_json, created_at
		FROM agent_status_updates
		WHERE session_id = ?
		ORDER BY created_at DESC
		LIMIT ?`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var updates []*models.AgentStatusUpdate
	for rows.Next() {
		u, err := scanAgentStatusUpdate(rows)
		if err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}
	return updates, rows.Err()
}

func scanAgentStatusUpdate(row interface{ Scan(...interface{}) error }) (*models.AgentStatusUpdate, error) {
	var u models.AgentStatusUpdate
	var detailsJSON string
	if err := row.Scan(&u.ID, &u.SessionID, &u.Type, &u.Message, &detailsJSON, &u.CreatedAt); err != nil {
		return nil, err
	}
	if detailsJSON != "" {
		_ = json.Unmarshal([]byte(detailsJSON), &u.Details)
	}
	return &u, nil
}

// ---- Session Tasks ----

func (s *Store) CreateSessionTask(task *models.SessionTask) error {
	contextJSON, _ := json.Marshal(task.Context)
	_, err := s.db.Exec(`
		INSERT INTO session_tasks (id, session_id, task_type, priority, title, description, context_json, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.SessionID, task.Type, task.Priority, task.Title, task.Description,
		string(contextJSON), task.Status, task.CreatedAt, task.UpdatedAt,
	)
	return err
}

func (s *Store) GetNextTask(sessionID string) (*models.SessionTask, error) {
	row := s.db.QueryRow(`
		SELECT id, session_id, task_type, priority, title, description, context_json, status, retrieved_at, completed_at, created_at, updated_at
		FROM session_tasks
		WHERE session_id = ? AND status = 'pending'
		ORDER BY priority DESC, created_at ASC
		LIMIT 1`, sessionID)

	task, err := scanSessionTask(row)
	if err == sql.ErrNoRows {
		return nil, nil // No pending tasks
	}
	return task, err
}

func (s *Store) UpdateTaskStatus(taskID string, status models.TaskStatus) error {
	now := time.Now()
	var retrievedAt, completedAt *time.Time
	if status == models.TaskRetrieved {
		retrievedAt = &now
	} else if status == models.TaskCompleted || status == models.TaskFailed {
		completedAt = &now
	}

	res, err := s.db.Exec(`
		UPDATE session_tasks
		SET status = ?, retrieved_at = COALESCE(?, retrieved_at), completed_at = COALESCE(?, completed_at), updated_at = ?
		WHERE id = ?`,
		status, retrievedAt, completedAt, now, taskID,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) ListSessionTasks(sessionID string) ([]*models.SessionTask, error) {
	rows, err := s.db.Query(`
		SELECT id, session_id, task_type, priority, title, description, context_json, status, retrieved_at, completed_at, created_at, updated_at
		FROM session_tasks
		WHERE session_id = ?
		ORDER BY priority DESC, created_at ASC`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []*models.SessionTask
	for rows.Next() {
		t, err := scanSessionTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func scanSessionTask(row interface{ Scan(...interface{}) error }) (*models.SessionTask, error) {
	var t models.SessionTask
	var contextJSON string
	var retrievedAt, completedAt sql.NullTime
	if err := row.Scan(&t.ID, &t.SessionID, &t.Type, &t.Priority, &t.Title, &t.Description,
		&contextJSON, &t.Status, &retrievedAt, &completedAt, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	if contextJSON != "" {
		_ = json.Unmarshal([]byte(contextJSON), &t.Context)
	}
	if retrievedAt.Valid {
		t.RetrievedAt = &retrievedAt.Time
	}
	if completedAt.Valid {
		t.CompletedAt = &completedAt.Time
	}
	return &t, nil
}

// ---- Review Requests ----

func (s *Store) CreateReviewRequest(review *models.ReviewRequest) error {
	filePathsJSON, _ := json.Marshal(review.FilePaths)
	_, err := s.db.Exec(`
		INSERT INTO review_requests (id, session_id, request_type, title, description, file_paths_json, diff, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		review.ID, review.SessionID, review.Type, review.Title, review.Description,
		string(filePathsJSON), review.Diff, review.Status, review.CreatedAt, review.UpdatedAt,
	)
	return err
}

func (s *Store) GetReviewRequest(id string) (*models.ReviewRequest, error) {
	row := s.db.QueryRow(`
		SELECT id, session_id, request_type, title, description, file_paths_json, diff, status, reviewer_notes, reviewed_at, created_at, updated_at
		FROM review_requests
		WHERE id = ?`, id)
	return scanReviewRequest(row)
}

func (s *Store) UpdateReviewRequest(id string, status models.ReviewStatus, notes string) error {
	now := time.Now()
	res, err := s.db.Exec(`
		UPDATE review_requests
		SET status = ?, reviewer_notes = ?, reviewed_at = ?, updated_at = ?
		WHERE id = ?`,
		status, notes, now, now, id,
	)
	if err != nil {
		return err
	}
	return checkRowsAffected(res)
}

func (s *Store) ListReviewRequests(sessionID string) ([]*models.ReviewRequest, error) {
	query := `
		SELECT id, session_id, request_type, title, description, file_paths_json, diff, status, reviewer_notes, reviewed_at, created_at, updated_at
		FROM review_requests`
	args := []interface{}{}

	if sessionID != "" {
		query += ` WHERE session_id = ?`
		args = append(args, sessionID)
	}
	query += ` ORDER BY created_at DESC`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviews []*models.ReviewRequest
	for rows.Next() {
		r, err := scanReviewRequest(rows)
		if err != nil {
			return nil, err
		}
		reviews = append(reviews, r)
	}
	return reviews, rows.Err()
}

func scanReviewRequest(row interface{ Scan(...interface{}) error }) (*models.ReviewRequest, error) {
	var r models.ReviewRequest
	var filePathsJSON string
	var reviewedAt sql.NullTime
	if err := row.Scan(&r.ID, &r.SessionID, &r.Type, &r.Title, &r.Description,
		&filePathsJSON, &r.Diff, &r.Status, &r.ReviewerNotes, &reviewedAt, &r.CreatedAt, &r.UpdatedAt); err != nil {
		return nil, err
	}
	if filePathsJSON != "" && filePathsJSON != "[]" {
		_ = json.Unmarshal([]byte(filePathsJSON), &r.FilePaths)
	}
	if reviewedAt.Valid {
		r.ReviewedAt = &reviewedAt.Time
	}
	return &r, nil
}

// ---- Session Messages ----

func (s *Store) CreateSessionMessage(msg *models.SessionMessage) error {
	contextJSON, _ := json.Marshal(msg.Context)
	_, err := s.db.Exec(`
		INSERT INTO session_messages (id, from_session_id, to_session_id, message_type, subject, body, context_json, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.FromSessionID, msg.ToSessionID, msg.Type, msg.Subject, msg.Body,
		string(contextJSON), msg.Status, msg.CreatedAt,
	)
	return err
}

func (s *Store) GetSessionMessages(sessionID string) ([]*models.SessionMessage, error) {
	rows, err := s.db.Query(`
		SELECT id, from_session_id, to_session_id, message_type, subject, body, context_json, status, read_at, created_at
		FROM session_messages
		WHERE to_session_id = ? OR from_session_id = ?
		ORDER BY created_at DESC`, sessionID, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []*models.SessionMessage
	for rows.Next() {
		m, err := scanSessionMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, m)
	}
	return messages, rows.Err()
}

func (s *Store) MarkMessageRead(messageID, toSessionID string) error {
	now := time.Now()
	result, err := s.db.Exec(`
		UPDATE session_messages
		SET status = ?, read_at = ?
		WHERE id = ? AND to_session_id = ? AND status = 'sent'`,
		models.MessageRead, now, messageID, toSessionID,
	)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrNotFound
	}
	return nil
}

func scanSessionMessage(row interface{ Scan(...interface{}) error }) (*models.SessionMessage, error) {
	var m models.SessionMessage
	var contextJSON string
	var readAt sql.NullTime
	if err := row.Scan(&m.ID, &m.FromSessionID, &m.ToSessionID, &m.Type, &m.Subject, &m.Body,
		&contextJSON, &m.Status, &readAt, &m.CreatedAt); err != nil {
		return nil, err
	}
	if contextJSON != "" {
		_ = json.Unmarshal([]byte(contextJSON), &m.Context)
	}
	if readAt.Valid {
		m.ReadAt = &readAt.Time
	}
	return &m, nil
}
