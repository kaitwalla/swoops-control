package api

import (
	"database/sql"
	"log"
	"net/http"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kaitwalla/swoops-control/pkg/models"
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

// validSessionNamePattern matches alphanumeric, hyphens, underscores, and periods only.
// This prevents path traversal attacks via session names.
var validSessionNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// validEnvVarKeyPattern matches valid environment variable names (POSIX standard).
// Must start with letter or underscore, followed by letters, digits, or underscores.
var validEnvVarKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// validateSessionName checks if a session name is safe to use in file paths.
// Returns an error if the name contains path traversal sequences or invalid characters.
func validateSessionName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	// Only allow safe characters
	if !validSessionNamePattern.MatchString(name) {
		return false
	}
	// Ensure no hidden path traversal after cleaning
	cleaned := filepath.Clean(name)
	if cleaned != name || filepath.IsAbs(cleaned) {
		return false
	}
	return true
}

// validateEnvVars checks that all environment variable keys are valid POSIX names.
// This prevents shell injection via malicious env var names like "FOO; malicious".
func validateEnvVars(envVars map[string]string) (string, bool) {
	for key := range envVars {
		if !validEnvVarKeyPattern.MatchString(key) {
			return key, false
		}
	}
	return "", true
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
	// Validate session name to prevent path traversal attacks
	if !validateSessionName(req.Name) {
		writeError(w, http.StatusBadRequest, "session name contains invalid characters or path traversal sequences")
		return
	}
	if req.BranchName == "" {
		req.BranchName = "swoops/" + req.Name
	}
	if req.EnvVars == nil {
		req.EnvVars = make(map[string]string)
	}
	// Validate environment variable keys to prevent shell injection
	if invalidKey, ok := validateEnvVars(req.EnvVars); !ok {
		writeError(w, http.StatusBadRequest, "invalid environment variable key: "+invalidKey)
		return
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

	// Launch session on host asynchronously.
	// The session manager chooses agent routing when available and falls back to SSH.
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

	var ch chan string
	var cleanupOnce sync.Once
	cleanup := func() {}

	// Prefer tmux streamer for SSH-backed sessions; fall back to gRPC agent output.
	if streamer := s.sessionMgr.GetOutputStreamer(id); streamer != nil {
		ch = streamer.Subscribe()
		cleanup = func() {
			cleanupOnce.Do(func() { streamer.Unsubscribe(ch) })
		}
	} else if s.agentOut != nil {
		ch = s.agentOut.SubscribeSessionOutput(id)
		cleanup = func() {
			cleanupOnce.Do(func() { s.agentOut.UnsubscribeSessionOutput(id, ch) })
		}
	} else {
		return
	}
	defer cleanup()

	clientDone := make(chan struct{})
	// Read pump: consume pings/close from client
	go func() {
		defer close(clientDone)
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-clientDone:
			return
		case output, ok := <-ch:
			if !ok {
				return
			}
			if err := conn.WriteJSON(map[string]string{"type": "output", "data": output}); err != nil {
				return
			}
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
