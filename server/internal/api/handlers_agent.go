package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/agentmgr"
)

// handleAgentHeartbeat processes heartbeat updates from agents.
// POST /api/v1/agent/heartbeat
func (s *Server) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	// Get host_id from auth middleware context
	hostID, ok := HostIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "host_id not found in context")
		return
	}

	var req agentmgr.HeartbeatRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Update heartbeat timestamp
	if err := s.agentMgr.UpdateHeartbeat(r.Context(), hostID, req); err != nil {
		writeInternalError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// handleGetPendingCommands retrieves pending commands for an agent.
// GET /api/v1/agent/commands/pending
func (s *Server) handleGetPendingCommands(w http.ResponseWriter, r *http.Request) {
	// Get host_id from auth middleware context
	hostID, ok := HostIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "host_id not found in context")
		return
	}

	commands, err := s.agentMgr.GetPendingCommands(r.Context(), hostID)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	// Return commands array (empty array if no commands)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"commands": commands,
	})
}

// handleAgentCommandResult processes command execution results from agents.
// POST /api/v1/agent/command-results
func (s *Server) handleAgentCommandResult(w http.ResponseWriter, r *http.Request) {
	// Get host_id from auth middleware context
	hostID, ok := HostIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "host_id not found in context")
		return
	}

	var req agentrpc.CommandResult
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Log the command result (using standard log for now, will be improved later)
	// s.logger().Info("command result received", ...)
	_ = hostID // Suppress unused variable warning for now

	// Update session status based on command result
	if req.SessionID != "" {
		sess, err := s.store.GetSession(req.SessionID)
		if err == nil {
			// Update session status based on command type and result
			if req.Ok {
				// Session launched successfully
				if sess.Status == models.SessionStatusPending {
					sess.Status = models.SessionStatusRunning
					if err := s.store.UpdateSession(sess); err != nil {
						// Log error (will be improved with proper logger later)
						_ = err
					}
				}
			} else {
				// Command failed
				sess.Status = models.SessionStatusFailed
				if err := s.store.UpdateSession(sess); err != nil {
					// Log error (will be improved with proper logger later)
					_ = err
				}
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleAgentSessionOutput processes session output from agents.
// POST /api/v1/agent/sessions/:session_id/output
func (s *Server) handleAgentSessionOutput(w http.ResponseWriter, r *http.Request) {
	// Get host_id from auth middleware context
	hostID, ok := HostIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "host_id not found in context")
		return
	}

	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}

	var req agentrpc.SessionOutput
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Verify session exists and belongs to this host
	sess, err := s.store.GetSession(sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if sess.HostID != hostID {
		writeError(w, http.StatusForbidden, "session does not belong to this host")
		return
	}

	// Append to session output in DB
	if err := s.store.UpdateSessionOutput(sessionID, req.Content); err != nil {
		writeInternalError(w, err)
		return
	}

	// Publish to WebSocket subscribers (web UI)
	s.agentMgr.PublishSessionOutput(sessionID, req.Content)

	// If EOF, mark session as completed
	if req.EOF {
		sess.Status = models.SessionStatusStopped
		if err := s.store.UpdateSession(sess); err != nil {
			// Log error (will be improved with proper logger later)
			_ = err
		}
	}

	w.WriteHeader(http.StatusOK)
}
