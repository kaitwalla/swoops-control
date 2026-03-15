package agentmgr

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

// Service manages agent connections, command queuing, and output pub/sub.
type Service struct {
	store  *store.Store
	logger *slog.Logger

	// WebSocket connections for notifications (hostID -> conn)
	wsConnsMu sync.RWMutex
	wsConns   map[string]*websocket.Conn

	// Pending commands queue
	cmdQueue *CommandQueue

	// Output pub/sub (session_id -> subscribers)
	outputMu      sync.RWMutex
	outputSubs    map[string][]chan string
	outputBuffers map[string]string // Buffer last output for new subscribers

	// WebSocket upgrader
	wsUpgrader websocket.Upgrader
}

// New creates a new agent manager service.
func New(s *store.Store, logger *slog.Logger) *Service {
	return &Service{
		store:         s,
		logger:        logger,
		wsConns:       make(map[string]*websocket.Conn),
		cmdQueue:      NewCommandQueue(),
		outputSubs:    make(map[string][]chan string),
		outputBuffers: make(map[string]string),
		wsUpgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				// Allow all origins for agent connections
				// Agent authentication is done via token
				return true
			},
		},
	}
}

// UpdateHeartbeat processes a heartbeat from an agent.
func (s *Service) UpdateHeartbeat(ctx context.Context, hostID string, req HeartbeatRequest) error {
	now := time.Now()

	// Update heartbeat timestamp and host status
	if err := s.store.TouchHostHeartbeat(hostID, now); err != nil {
		return fmt.Errorf("update heartbeat: %w", err)
	}

	// Update agent version if provided
	if req.CurrentVersion != "" {
		if err := s.store.UpdateAgentVersion(hostID, req.CurrentVersion); err != nil {
			s.logger.Error("failed to update agent version", "error", err, "host_id", hostID)
		}
	}

	// Update version info if provided
	if req.UpdateAvailable {
		if err := s.store.UpdateHostUpdateInfo(hostID, req.UpdateAvailable, req.LatestVersion, req.UpdateURL); err != nil {
			s.logger.Error("failed to update host update info", "error", err, "host_id", hostID)
		}
	}

	s.logger.Debug("heartbeat received", "host_id", hostID, "running_sessions", req.RunningSessions)
	return nil
}

// HeartbeatRequest matches the spec in the refactor plan.
type HeartbeatRequest struct {
	HostID          string `json:"host_id"`
	RunningSessions int    `json:"running_sessions"`
	UpdateAvailable bool   `json:"update_available"`
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateURL       string `json:"update_url"`
}

// GetPendingCommands retrieves all pending commands for a host.
func (s *Service) GetPendingCommands(ctx context.Context, hostID string) ([]*agentrpc.SessionCommand, error) {
	commands := s.cmdQueue.DequeueAll(hostID)
	s.logger.Debug("pending commands retrieved", "host_id", hostID, "count", len(commands))
	return commands, nil
}

// QueueCommand adds a command to the pending queue and notifies the agent via WebSocket.
func (s *Service) QueueCommand(hostID string, cmd *agentrpc.SessionCommand) error {
	s.cmdQueue.Enqueue(hostID, cmd)
	s.logger.Info("command queued", "host_id", hostID, "command_id", cmd.CommandID, "command", cmd.Command)

	// Send WebSocket notification if connected
	s.notifyNewCommand(hostID)

	return nil
}

// notifyNewCommand sends a WebSocket notification to the agent about a new command.
func (s *Service) notifyNewCommand(hostID string) {
	s.wsConnsMu.RLock()
	conn, ok := s.wsConns[hostID]
	s.wsConnsMu.RUnlock()

	if !ok {
		s.logger.Debug("no websocket connection for host, agent will poll", "host_id", hostID)
		return
	}

	msg := map[string]string{"type": "new_command"}
	if err := conn.WriteJSON(msg); err != nil {
		s.logger.Warn("failed to send websocket notification", "host_id", hostID, "error", err)
		// Close stale connection
		s.UnregisterWebSocket(hostID, conn)
		conn.Close()
	}
}

// RegisterWebSocket registers a WebSocket connection for a host.
func (s *Service) RegisterWebSocket(hostID string, conn *websocket.Conn) {
	s.wsConnsMu.Lock()
	defer s.wsConnsMu.Unlock()

	// Close old connection if exists
	if oldConn, ok := s.wsConns[hostID]; ok {
		oldConn.Close()
	}

	s.wsConns[hostID] = conn
	s.logger.Info("websocket registered", "host_id", hostID)
}

// UnregisterWebSocket removes a WebSocket connection for a host.
// It only unregisters if the provided connection matches the current one (prevents race with reconnection).
func (s *Service) UnregisterWebSocket(hostID string, conn *websocket.Conn) {
	s.wsConnsMu.Lock()
	defer s.wsConnsMu.Unlock()

	// Only unregister if this is the current connection
	if currentConn, ok := s.wsConns[hostID]; ok && currentConn == conn {
		delete(s.wsConns, hostID)
		s.logger.Info("websocket unregistered", "host_id", hostID)
	}
}

// PublishSessionOutput publishes session output to all subscribers.
func (s *Service) PublishSessionOutput(sessionID, content string) {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()

	// Update buffer for new subscribers
	s.outputBuffers[sessionID] = content

	// Send to all subscribers
	subs := s.outputSubs[sessionID]
	for _, ch := range subs {
		select {
		case ch <- content:
		default:
			// Subscriber is slow, skip to avoid blocking
			s.logger.Warn("subscriber slow, dropping message", "session_id", sessionID)
		}
	}
}

// SubscribeSessionOutput subscribes to session output updates.
func (s *Service) SubscribeSessionOutput(sessionID string) chan string {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()

	ch := make(chan string, 100) // Buffered to avoid blocking publisher
	s.outputSubs[sessionID] = append(s.outputSubs[sessionID], ch)

	// Send buffered output if available
	if buf, ok := s.outputBuffers[sessionID]; ok && buf != "" {
		select {
		case ch <- buf:
		default:
		}
	}

	s.logger.Debug("output subscriber added", "session_id", sessionID)
	return ch
}

// UnsubscribeSessionOutput unsubscribes from session output updates.
func (s *Service) UnsubscribeSessionOutput(sessionID string, ch chan string) {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()

	subs := s.outputSubs[sessionID]
	for i, sub := range subs {
		if sub == ch {
			// Remove from slice
			s.outputSubs[sessionID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			s.logger.Debug("output subscriber removed", "session_id", sessionID)
			return
		}
	}
}

// Close cleans up the service resources.
func (s *Service) Close() {
	// Close all WebSocket connections
	s.wsConnsMu.Lock()
	for hostID, conn := range s.wsConns {
		conn.Close()
		s.logger.Info("websocket closed on shutdown", "host_id", hostID)
	}
	s.wsConns = make(map[string]*websocket.Conn)
	s.wsConnsMu.Unlock()

	// Close all output subscribers
	s.outputMu.Lock()
	for sessionID, subs := range s.outputSubs {
		for _, ch := range subs {
			close(ch)
		}
		s.logger.Debug("output subscribers closed", "session_id", sessionID)
	}
	s.outputSubs = make(map[string][]chan string)
	s.outputMu.Unlock()
}

// SendCommand sends a command to an agent (implements AgentController interface).
func (s *Service) SendCommand(hostID, command string, args map[string]string) error {
	cmd := &agentrpc.SessionCommand{
		CommandID: models.NewID(),
		SessionID: args["session_id"], // Extract from args
		Command:   command,
		Args:      args,
	}
	return s.QueueCommand(hostID, cmd)
}

// IsHostConnected checks if an agent is connected via WebSocket.
func (s *Service) IsHostConnected(hostID string) bool {
	s.wsConnsMu.RLock()
	defer s.wsConnsMu.RUnlock()
	_, ok := s.wsConns[hostID]
	return ok
}

// LaunchSession sends a launch session command to the agent.
func (s *Service) LaunchSession(sess *models.Session, host *models.Host) error {
	args := map[string]string{
		"session_id":   sess.ID,
		"session_type": string(sess.Type),
		"work_dir":     sess.WorktreePath,
		"prompt":       sess.Prompt,
	}
	return s.SendCommand(host.ID, agentrpc.CommandLaunch, args)
}

// StopSession sends a stop session command to the agent.
func (s *Service) StopSession(sess *models.Session, host *models.Host) error {
	args := map[string]string{
		"session_id": sess.ID,
	}
	return s.SendCommand(host.ID, agentrpc.CommandStop, args)
}

// SendInput sends input to a session via the agent.
func (s *Service) SendInput(sess *models.Session, host *models.Host, input string) error {
	args := map[string]string{
		"session_id": sess.ID,
		"input":      input,
	}
	return s.SendCommand(host.ID, agentrpc.CommandInput, args)
}

// HandleAgentWebSocket handles the WebSocket connection for agent notifications.
// Endpoint: /api/v1/ws/agent/connect?token=...
func (s *Service) HandleAgentWebSocket(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate via token query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token parameter", http.StatusUnauthorized)
		return
	}

	// 2. Look up host by auth token
	host, err := s.authenticateAgentToken(r.Context(), token)
	if err != nil {
		s.logger.Warn("agent websocket auth failed", "error", err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// 3. Upgrade to WebSocket
	conn, err := s.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", "host_id", host.ID, "error", err)
		return
	}
	defer conn.Close()

	// 4. Register connection
	s.RegisterWebSocket(host.ID, conn)
	defer s.UnregisterWebSocket(host.ID, conn)

	s.logger.Info("agent connected via websocket", "host_id", host.ID, "host_name", host.Name)

	// 5. Set up ping/pong for keepalive
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// 6. Start ping ticker
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// 7. Keep connection alive (read loop)
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			// Read messages from agent (mostly just pong responses)
			_, _, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					s.logger.Warn("websocket read error", "host_id", host.ID, "error", err)
				}
				return
			}
		}
	}()

	// 8. Send periodic pings
	for {
		select {
		case <-done:
			return
		case <-pingTicker.C:
			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second)); err != nil {
				s.logger.Warn("websocket ping failed", "host_id", host.ID, "error", err)
				return
			}
		}
	}
}

// authenticateAgentToken looks up a host by auth token with constant-time comparison.
func (s *Service) authenticateAgentToken(ctx context.Context, token string) (*models.Host, error) {
	// Use the store method to look up by token
	host, err := s.store.GetHostByAuthToken(token)
	if err != nil {
		// For security, do a constant-time comparison even on error
		// to prevent timing attacks
		_ = subtle.ConstantTimeCompare([]byte(token), []byte("dummy-token-for-timing"))
		return nil, err
	}

	// Verify the token with constant-time comparison
	if subtle.ConstantTimeCompare([]byte(token), []byte(host.AgentAuthToken)) != 1 {
		return nil, sql.ErrNoRows
	}

	return host, nil
}
