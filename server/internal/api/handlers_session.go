package api

import (
	"database/sql"
	"log"
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

	// Serialize response BEFORE launching the async goroutine.
	// The goroutine re-reads from the store, so there is no shared pointer.
	response := *sess // value copy for serialization
	writeJSON(w, http.StatusCreated, &response)

	// Launch session on host asynchronously via SSH.
	// Pass only IDs — the launcher re-reads from the store to avoid races.
	sessionID := sess.ID
	hostID := req.HostID
	go func() {
		if err := s.launchFunc(sessionID, hostID); err != nil {
			log.Printf("failed to launch session %s: %v", sessionID, err)
		}
	}()
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

	// Get session to check if it's running
	sess, err := s.store.GetSession(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	// Stop session on host if it's active
	if isActiveStatus(sess.Status) {
		host, err := s.store.GetHost(sess.HostID)
		if err == nil {
			if stopErr := s.sessionMgr.StopSession(sess, host); stopErr != nil {
				log.Printf("warn: stop session %s during delete: %v", id, stopErr)
			}
		}
	}

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

	sess, err := s.store.GetSession(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	if !isActiveStatus(sess.Status) {
		writeJSON(w, http.StatusOK, map[string]string{"status": string(sess.Status)})
		return
	}

	host, err := s.store.GetHost(sess.HostID)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	if err := s.sessionMgr.StopSession(sess, host); err != nil {
		writeInternalError(w, err)
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

	sess, err := s.store.GetSession(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "session not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	if !isActiveStatus(sess.Status) {
		writeError(w, http.StatusBadRequest, "session is not active")
		return
	}

	host, err := s.store.GetHost(sess.HostID)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	if err := s.sessionMgr.SendInput(sess, host, req.Input); err != nil {
		writeInternalError(w, err)
		return
	}

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

	// Try to get live output from the session manager
	if isActiveStatus(sess.Status) {
		host, err := s.store.GetHost(sess.HostID)
		if err == nil {
			if output, err := s.sessionMgr.GetOutput(sess, host); err == nil {
				writeJSON(w, http.StatusOK, map[string]string{"output": output})
				return
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"output": sess.LastOutput})
}

func (s *Server) handleSessionOutputWS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Verify session exists
	sess, err := s.store.GetSession(id)
	if err != nil {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	// Send initial output
	conn.WriteJSON(map[string]string{"type": "output", "data": sess.LastOutput})

	// Subscribe to live output if session is active
	streamer := s.sessionMgr.GetOutputStreamer(id)
	if streamer == nil {
		// No active streamer — just send current output and close
		return
	}

	ch := streamer.Subscribe()
	defer streamer.Unsubscribe(ch)

	// Read pump: consume pings/close from client
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for output := range ch {
		if err := conn.WriteJSON(map[string]string{"type": "output", "data": output}); err != nil {
			return
		}
	}
}

func isActiveStatus(status models.SessionStatus) bool {
	switch status {
	case models.SessionStatusPending, models.SessionStatusStarting,
		models.SessionStatusRunning, models.SessionStatusIdle:
		return true
	}
	return false
}
