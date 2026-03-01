package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

// ---- Agent Status Updates ----

type reportStatusRequest struct {
	StatusType string                 `json:"status_type"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details"`
}

func (s *Server) handleReportStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	var req reportStatusRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.StatusType == "" || req.Message == "" {
		writeError(w, http.StatusBadRequest, "status_type and message are required")
		return
	}

	// Validate status type
	if !models.IsValidAgentStatusType(req.StatusType) {
		writeError(w, http.StatusBadRequest, "invalid status_type: must be one of: working, idle, blocked, completed, error")
		return
	}

	// Verify session exists
	if _, err := s.store.GetSession(sessionID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	update := &models.AgentStatusUpdate{
		ID:        models.NewID(),
		SessionID: sessionID,
		Type:      models.AgentStatusType(req.StatusType),
		Message:   req.Message,
		Details:   req.Details,
		CreatedAt: time.Now(),
	}

	if err := s.store.CreateAgentStatusUpdate(update); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, update)
}

func (s *Server) handleListStatusUpdates(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	updates, err := s.store.ListAgentStatusUpdates(sessionID, 50)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if updates == nil {
		updates = []*models.AgentStatusUpdate{}
	}
	writeJSON(w, http.StatusOK, updates)
}

// ---- Session Tasks ----

type createTaskRequest struct {
	TaskType    string                 `json:"task_type"`
	Priority    int                    `json:"priority"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Context     map[string]interface{} `json:"context"`
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	var req createTaskRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TaskType == "" || req.Title == "" || req.Description == "" {
		writeError(w, http.StatusBadRequest, "task_type, title, and description are required")
		return
	}

	// Validate task type
	if !models.IsValidTaskType(req.TaskType) {
		writeError(w, http.StatusBadRequest, "invalid task_type: must be one of: instruction, fix, review, refactor, test")
		return
	}

	// Verify session exists
	if _, err := s.store.GetSession(sessionID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	now := time.Now()
	task := &models.SessionTask{
		ID:          models.NewID(),
		SessionID:   sessionID,
		Type:        models.TaskType(req.TaskType),
		Priority:    req.Priority,
		Title:       req.Title,
		Description: req.Description,
		Context:     req.Context,
		Status:      models.TaskPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.store.CreateSessionTask(task); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

func (s *Server) handleGetNextTask(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	// Verify session exists
	if _, err := s.store.GetSession(sessionID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	task, err := s.store.GetNextTask(sessionID)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	if task == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Mark as retrieved
	if err := s.store.UpdateTaskStatus(task.ID, models.TaskRetrieved); err != nil {
		writeInternalError(w, err)
		return
	}

	task.Status = models.TaskRetrieved
	now := time.Now()
	task.RetrievedAt = &now

	writeJSON(w, http.StatusOK, task)
}

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	tasks, err := s.store.ListSessionTasks(sessionID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if tasks == nil {
		tasks = []*models.SessionTask{}
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) handleUpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "task_id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate task status
	if !models.IsValidTaskStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "invalid status: must be one of: pending, retrieved, completed, failed")
		return
	}

	if err := s.store.UpdateTaskStatus(taskID, models.TaskStatus(req.Status)); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "task not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ---- Review Requests ----

type createReviewRequest struct {
	RequestType string   `json:"request_type"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	FilePaths   []string `json:"file_paths"`
	Diff        string   `json:"diff"`
}

func (s *Server) handleCreateReviewRequest(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	var req createReviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.RequestType == "" || req.Title == "" || req.Description == "" {
		writeError(w, http.StatusBadRequest, "request_type, title, and description are required")
		return
	}

	// Validate request type
	if !models.IsValidReviewType(req.RequestType) {
		writeError(w, http.StatusBadRequest, "invalid request_type: must be one of: code, architecture, security, performance")
		return
	}

	// Verify session exists
	if _, err := s.store.GetSession(sessionID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	now := time.Now()
	review := &models.ReviewRequest{
		ID:          models.NewID(),
		SessionID:   sessionID,
		Type:        models.ReviewType(req.RequestType),
		Title:       req.Title,
		Description: req.Description,
		FilePaths:   req.FilePaths,
		Diff:        req.Diff,
		Status:      models.ReviewPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.store.CreateReviewRequest(review); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, review)
}

func (s *Server) handleListReviewRequests(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session_id")

	reviews, err := s.store.ListReviewRequests(sessionID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if reviews == nil {
		reviews = []*models.ReviewRequest{}
	}
	writeJSON(w, http.StatusOK, reviews)
}

func (s *Server) handleGetReviewRequest(w http.ResponseWriter, r *http.Request) {
	reviewID := chi.URLParam(r, "review_id")
	if reviewID == "" {
		writeError(w, http.StatusBadRequest, "review_id is required")
		return
	}

	review, err := s.store.GetReviewRequest(reviewID)
	if err != nil {
		writeError(w, http.StatusNotFound, "review request not found")
		return
	}

	writeJSON(w, http.StatusOK, review)
}

func (s *Server) handleUpdateReviewRequest(w http.ResponseWriter, r *http.Request) {
	reviewID := chi.URLParam(r, "review_id")
	if reviewID == "" {
		writeError(w, http.StatusBadRequest, "review_id is required")
		return
	}

	var req struct {
		Status string `json:"status"`
		Notes  string `json:"notes"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate review status
	if !models.IsValidReviewStatus(req.Status) {
		writeError(w, http.StatusBadRequest, "invalid status: must be one of: pending, in_review, approved, changes_requested, rejected")
		return
	}

	if err := s.store.UpdateReviewRequest(reviewID, models.ReviewStatus(req.Status), req.Notes); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "review request not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	review, _ := s.store.GetReviewRequest(reviewID)
	writeJSON(w, http.StatusOK, review)
}

// ---- Session Messages ----

type createMessageRequest struct {
	ToSessionID string                 `json:"to_session_id"`
	MessageType string                 `json:"message_type"`
	Subject     string                 `json:"subject"`
	Body        string                 `json:"body"`
	Context     map[string]interface{} `json:"context"`
}

func (s *Server) handleCreateSessionMessage(w http.ResponseWriter, r *http.Request) {
	fromSessionID := chi.URLParam(r, "id")
	if fromSessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	var req createMessageRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ToSessionID == "" || req.MessageType == "" || req.Subject == "" || req.Body == "" {
		writeError(w, http.StatusBadRequest, "to_session_id, message_type, subject, and body are required")
		return
	}

	// Validate message type
	if !models.IsValidMessageType(req.MessageType) {
		writeError(w, http.StatusBadRequest, "invalid message_type: must be one of: question, info, request, response")
		return
	}

	// Verify both sessions exist
	if _, err := s.store.GetSession(fromSessionID); err != nil {
		writeError(w, http.StatusNotFound, "from session not found")
		return
	}
	if _, err := s.store.GetSession(req.ToSessionID); err != nil {
		writeError(w, http.StatusBadRequest, "to session not found")
		return
	}

	msg := &models.SessionMessage{
		ID:            models.NewID(),
		FromSessionID: fromSessionID,
		ToSessionID:   req.ToSessionID,
		Type:          models.MessageType(req.MessageType),
		Subject:       req.Subject,
		Body:          req.Body,
		Context:       req.Context,
		Status:        models.MessageSent,
		CreatedAt:     time.Now(),
	}

	if err := s.store.CreateSessionMessage(msg); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

func (s *Server) handleListSessionMessages(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	messages, err := s.store.GetSessionMessages(sessionID)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if messages == nil {
		messages = []*models.SessionMessage{}
	}
	writeJSON(w, http.StatusOK, messages)
}

func (s *Server) handleMarkMessageRead(w http.ResponseWriter, r *http.Request) {
	messageID := chi.URLParam(r, "message_id")
	if messageID == "" {
		writeError(w, http.StatusBadRequest, "message_id is required")
		return
	}

	var req struct {
		SessionID string `json:"session_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	// Verify session exists
	if _, err := s.store.GetSession(req.SessionID); err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if err := s.store.MarkMessageRead(messageID, req.SessionID); err != nil {
		if err == store.ErrNotFound {
			writeError(w, http.StatusNotFound, "message not found or already read, or you are not the recipient")
			return
		}
		writeInternalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
