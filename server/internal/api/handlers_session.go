package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/swoopsh/swoops/pkg/models"
)

type createSessionRequest struct {
	Name          string            `json:"name"`
	HostID        string            `json:"host_id"`
	AgentType     models.AgentType  `json:"agent_type"`
	Prompt        string            `json:"prompt"`
	BranchName    string            `json:"branch_name"`
	TemplateID    string            `json:"template_id"`
	ModelOverride string            `json:"model_override"`
	EnvVars       map[string]string `json:"env_vars"`
	Plugins       []string          `json:"plugins"`
	AllowedTools  []string          `json:"allowed_tools"`
	ExtraFlags    []string          `json:"extra_flags"`
}

type sendInputRequest struct {
	Input string `json:"input"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	hostID := r.URL.Query().Get("host_id")
	status := r.URL.Query().Get("status")

	sessions, err := s.store.ListSessions(hostID, status)
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if sessions == nil {
		sessions = []*models.Session{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req createSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.HostID == "" || req.AgentType == "" || req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "host_id, agent_type, and prompt are required")
		return
	}

	if req.AgentType != models.AgentTypeClaude && req.AgentType != models.AgentTypeCodex {
		writeError(w, http.StatusBadRequest, "agent_type must be 'claude' or 'codex'")
		return
	}

	// Verify host exists
	_, err := s.store.GetHost(req.HostID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusBadRequest, "host not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	if req.Name == "" {
		req.Name = string(req.AgentType) + "-" + models.NewID()[:8]
	}
	if req.BranchName == "" {
		req.BranchName = "swoops/" + req.Name
	}
	if req.EnvVars == nil {
		req.EnvVars = make(map[string]string)
	}

	now := time.Now()
	sess := &models.Session{
		ID:            models.NewID(),
		Name:          req.Name,
		HostID:        req.HostID,
		TemplateID:    req.TemplateID,
		AgentType:     req.AgentType,
		Status:        models.SessionStatusPending,
		Prompt:        req.Prompt,
		BranchName:    req.BranchName,
		ModelOverride: req.ModelOverride,
		EnvVars:       req.EnvVars,
		Plugins:       req.Plugins,
		AllowedTools:  req.AllowedTools,
		ExtraFlags:    req.ExtraFlags,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	if err := s.store.CreateSession(sess); err != nil {
		writeInternalError(w, err)
		return
	}

	// TODO: Phase 2 - dispatch session creation to host via SSH
	// TODO: Phase 3 - dispatch session creation to host via gRPC agent

	writeJSON(w, http.StatusCreated, sess)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// TODO: stop session on host first
	if err := s.store.DeleteSession(id); err != nil {
		if writeStoreError(w, err, "session not found") {
			return
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStopSession(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	// TODO: Phase 2 - send stop command to host
	if err := s.store.UpdateSessionStatus(id, models.SessionStatusStopped); err != nil {
		if writeStoreError(w, err, "session not found") {
			return
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) handleSendInput(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req sendInputRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Verify session exists
	if _, err := s.store.GetSession(id); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	// TODO: Phase 2 - send input to tmux session on host
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (s *Server) handleGetOutput(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	sess, err := s.store.GetSession(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"output": sess.LastOutput})
}
