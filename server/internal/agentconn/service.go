package agentconn

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/config"
	"github.com/kaitwalla/swoops-control/server/internal/metrics"
	"github.com/kaitwalla/swoops-control/server/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service struct {
	agentrpc.UnimplementedAgentServiceServer

	store  *store.Store
	config *config.Config
	logger *slog.Logger

	checkInterval time.Duration
	degradedAfter time.Duration
	offlineAfter  time.Duration

	now func() time.Time

	// Connection management - lock ordering: always acquire connMu before pendingMu or outputMu
	connMu sync.RWMutex
	conns  map[string]*hostConn

	pendingMu sync.Mutex
	pending   map[string]chan *agentrpc.CommandResult

	outputMu   sync.RWMutex
	outputSubs map[string]map[chan string]struct{}

	done chan struct{}
	wg   sync.WaitGroup
}

type hostConn struct {
	hostID string
	sendCh chan *agentrpc.ControlEnvelope
	done   chan struct{}
	once   sync.Once
}

func NewService(s *store.Store, cfg *config.Config, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	svc := &Service{
		store:         s,
		config:        cfg,
		logger:        logger,
		checkInterval: defaultCheckInterval,
		degradedAfter: defaultDegradedAfter,
		offlineAfter:  defaultOfflineAfter,
		now:           time.Now,
		conns:         make(map[string]*hostConn),
		pending:       make(map[string]chan *agentrpc.CommandResult),
		outputSubs:    make(map[string]map[chan string]struct{}),
		done:          make(chan struct{}),
	}
	svc.wg.Add(1)
	go svc.monitorLoop()
	return svc
}

func (s *Service) Close() {
	close(s.done)
	s.wg.Wait()

	// Clean up output subscriptions
	s.outputMu.Lock()
	for _, subs := range s.outputSubs {
		for ch := range subs {
			close(ch)
		}
	}
	s.outputSubs = map[string]map[chan string]struct{}{}
	s.outputMu.Unlock()

	// Clean up connections
	s.connMu.Lock()
	connsToClose := make([]*hostConn, 0, len(s.conns))
	for _, c := range s.conns {
		c.once.Do(func() { close(c.done) })
		connsToClose = append(connsToClose, c)
	}
	s.conns = map[string]*hostConn{}
	s.connMu.Unlock()

	// Close all sendCh channels outside the lock to avoid blocking
	for _, c := range connsToClose {
		close(c.sendCh)
	}

	// Clean up pending commands
	s.pendingMu.Lock()
	for key, ch := range s.pending {
		close(ch)
		delete(s.pending, key)
	}
	s.pendingMu.Unlock()
}

func (s *Service) Connect(stream agentrpc.AgentService_ConnectServer) error {
	ctx := stream.Context()
	metrics.AgentConnectionsTotal.Inc()

	// Receive and validate hello message
	first, err := stream.Recv()
	if err != nil {
		metrics.AgentConnectionErrors.WithLabelValues("recv_error").Inc()
		if errors.Is(err, io.EOF) {
			return status.Error(codes.InvalidArgument, "missing hello message")
		}
		return err
	}
	if first.Hello == nil {
		return status.Error(codes.InvalidArgument, "first message must be hello")
	}

	hello := first.Hello
	if err := validateHello(hello); err != nil {
		metrics.AgentConnectionErrors.WithLabelValues("validation_failed").Inc()
		return status.Error(codes.InvalidArgument, err.Error())
	}

	// Load host and validate authentication
	host, err := s.store.GetHost(hello.HostID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			metrics.AgentConnectionErrors.WithLabelValues("host_not_found").Inc()
			s.logger.Warn("agent connection attempt for unregistered host",
				"host_id", hello.HostID,
				"remote_addr", peerAddr(ctx))
			return status.Error(codes.NotFound, "host not registered")
		}
		metrics.AgentConnectionErrors.WithLabelValues("db_error").Inc()
		s.logger.Error("failed to load host",
			"host_id", hello.HostID,
			"error", err)
		return status.Errorf(codes.Internal, "load host: %v", err)
	}

	// Authenticate using constant-time comparison
	if !authenticateAgent(host.AgentAuthToken, hello.AuthToken) {
		metrics.AgentConnectionErrors.WithLabelValues("auth_failed").Inc()
		s.logger.Warn("agent authentication failed",
			"host_id", hello.HostID,
			"host_name", host.Name,
			"remote_addr", peerAddr(ctx))
		return status.Error(codes.Unauthenticated, "invalid authentication token")
	}

	// Update host heartbeat
	now := s.now()
	if err := s.store.UpsertHostHeartbeat(hello.HostID, hello.AgentVersion, hello.OS, hello.Arch, hello.HostName, now); err != nil {
		s.logger.Error("failed to update hello heartbeat",
			"host_id", hello.HostID,
			"error", err)
		return status.Errorf(codes.Internal, "update hello heartbeat: %v", err)
	}

	s.logger.Info("agent connected",
		"host_id", hello.HostID,
		"host_name", hello.HostName,
		"agent_version", hello.AgentVersion,
		"os", hello.OS,
		"arch", hello.Arch)

	conn := s.registerHostConn(hello.HostID)
	metrics.AgentConnectionsActive.Inc()
	defer func() {
		s.unregisterHostConn(hello.HostID, conn)
		metrics.AgentConnectionsActive.Dec()
	}()

	// Send hello acknowledgement
	conn.sendCh <- &agentrpc.ControlEnvelope{
		Ack: &agentrpc.Ack{Message: "hello_ack"},
	}

	// Start send goroutine
	sendErr := make(chan error, 1)
	go func() {
		for {
			select {
			case <-conn.done:
				sendErr <- nil
				return
			case msg, ok := <-conn.sendCh:
				if !ok {
					sendErr <- nil
					return
				}
				if err := stream.Send(msg); err != nil {
					sendErr <- err
					return
				}
			}
		}
	}()

	// Main receive loop
	for {
		msg, err := stream.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				conn.once.Do(func() { close(conn.done) })
				s.logger.Info("agent disconnected (EOF)",
					"host_id", hello.HostID,
					"host_name", host.Name)
				return nil
			}
			conn.once.Do(func() { close(conn.done) })
			s.logger.Warn("agent disconnected with error",
				"host_id", hello.HostID,
				"host_name", host.Name,
				"error", err)
			return err
		}

		// Check if send goroutine failed
		select {
		case err := <-sendErr:
			if err != nil {
				conn.once.Do(func() { close(conn.done) })
				s.logger.Warn("agent send failed",
					"host_id", hello.HostID,
					"error", err)
				return err
			}
			conn.once.Do(func() { close(conn.done) })
			return nil
		default:
		}

		// Process message
		switch {
		case msg.Heartbeat != nil:
			heartbeatAt := s.now()
			if msg.Heartbeat.SentUnix > 0 {
				heartbeatAt = time.Unix(msg.Heartbeat.SentUnix, 0)
			}
			if err := s.store.TouchHostHeartbeat(hello.HostID, heartbeatAt); err != nil {
				s.logger.Error("failed to update heartbeat",
					"host_id", hello.HostID,
					"error", err)
				return status.Errorf(codes.Internal, "update heartbeat: %v", err)
			}
			// Update agent version and update info if provided
			if msg.Heartbeat.UpdateAvailable || msg.Heartbeat.CurrentVersion != "" {
				if err := s.store.UpdateHostUpdateInfo(hello.HostID, msg.Heartbeat.UpdateAvailable, msg.Heartbeat.LatestVersion, msg.Heartbeat.UpdateURL); err != nil {
					s.logger.Error("failed to update host update info",
						"host_id", hello.HostID,
						"error", err)
					// Non-fatal, continue processing
				}
			}
			metrics.AgentHeartbeatsReceived.Inc()

		case msg.Output != nil:
			if msg.Output.SessionID != "" {
				if err := s.store.UpdateSessionOutput(msg.Output.SessionID, msg.Output.Content); err != nil {
					if !errors.Is(err, store.ErrNotFound) {
						s.logger.Error("failed to persist agent output",
							"session_id", msg.Output.SessionID,
							"host_id", hello.HostID,
							"error", err)
					}
				}
				s.publishOutput(msg.Output.SessionID, msg.Output.Content)
			}

		case msg.CommandResult != nil:
			s.resolvePendingCommand(hello.HostID, msg.CommandResult)

		default:
			// Unknown payload shape; ignore for forward compatibility
			s.logger.Debug("received unknown message type from agent",
				"host_id", hello.HostID)
		}
	}
}

func (s *Service) registerHostConn(hostID string) *hostConn {
	c := &hostConn{
		hostID: hostID,
		sendCh: make(chan *agentrpc.ControlEnvelope, commandChannelBuffer),
		done:   make(chan struct{}),
	}

	s.connMu.Lock()
	defer s.connMu.Unlock()

	// If there's a previous connection, close it first
	if prev, ok := s.conns[hostID]; ok {
		prev.once.Do(func() { close(prev.done) })
		// Close the channel outside the critical section by deferring it
		go func() {
			// Give the old connection a moment to drain
			time.Sleep(100 * time.Millisecond)
			close(prev.sendCh)
		}()
	}
	s.conns[hostID] = c

	// Clear pending commands for this host
	s.clearPendingForHost(hostID)

	return c
}

func (s *Service) unregisterHostConn(hostID string, conn *hostConn) {
	s.connMu.Lock()
	defer s.connMu.Unlock()

	current, ok := s.conns[hostID]
	if !ok || current != conn {
		return
	}
	delete(s.conns, hostID)

	// Clear pending commands for this host
	s.clearPendingForHost(hostID)
}

func (s *Service) clearPendingForHost(hostID string) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()

	prefix := hostID + ":"
	for key, ch := range s.pending {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			close(ch)
			delete(s.pending, key)
		}
	}
}

func (s *Service) IsHostConnected(hostID string) bool {
	s.connMu.RLock()
	defer s.connMu.RUnlock()
	_, ok := s.conns[hostID]
	return ok
}

func (s *Service) LaunchSession(sess *models.Session, host *models.Host) error {
	args := map[string]string{
		"session_type":   string(sess.Type),
		"session_name":   sess.Name,
		"agent_type":     string(sess.AgentType),
		"prompt":         sess.Prompt,
		"branch_name":    sess.BranchName,
		"worktree_path":  sess.WorktreePath,
		"tmux_session":   sess.TmuxSessionName,
		"base_repo_path": host.BaseRepoPath,
		"worktree_root":  host.WorktreeRoot,
		"model_override": sess.ModelOverride,
	}

	// For shell sessions, set working directory
	if sess.Type == models.SessionTypeShell {
		// Use WorktreePath if set, otherwise default to base repo path or home directory
		workDir := sess.WorktreePath
		if workDir == "" {
			if host.BaseRepoPath != "" {
				workDir = host.BaseRepoPath
			} else {
				workDir = "~"
			}
		}
		args["work_dir"] = workDir
	}

	// Add MCP config parameters if config is available
	if s.config != nil {
		var serverAddr string
		if s.config.Server.ExternalURL != "" {
			// Use configured external URL for reliable connectivity
			serverAddr = s.config.Server.ExternalURL
		} else if s.config.Server.Host == "0.0.0.0" {
			// For gRPC-connected agents, use localhost (agent is on same machine as control plane)
			// If host is remote, the SSH path should be used instead
			serverAddr = fmt.Sprintf("http://localhost:%d", s.config.Server.Port)
		} else {
			serverAddr = fmt.Sprintf("http://%s:%d", s.config.Server.Host, s.config.Server.Port)
		}
		args["server_addr"] = serverAddr
		args["api_key"] = s.config.Auth.APIKey
	}

	cmd := &agentrpc.SessionCommand{
		SessionID: sess.ID,
		Command:   agentrpc.CommandLaunch,
		Args:      args,
	}
	return s.sendCommand(host.ID, cmd)
}

func (s *Service) StopSession(sess *models.Session, host *models.Host) error {
	cmd := &agentrpc.SessionCommand{
		SessionID: sess.ID,
		Command:   agentrpc.CommandStop,
	}
	return s.sendCommand(host.ID, cmd)
}

func (s *Service) SendInput(sess *models.Session, host *models.Host, input string) error {
	cmd := &agentrpc.SessionCommand{
		SessionID: sess.ID,
		Command:   agentrpc.CommandInput,
		Args: map[string]string{
			"input": input,
		},
	}
	return s.sendCommand(host.ID, cmd)
}

// SendCommand sends a generic command to the agent on the specified host.
func (s *Service) SendCommand(hostID, command string, args map[string]string) error {
	cmd := &agentrpc.SessionCommand{
		Command: command,
		Args:    args,
	}
	return s.sendCommand(hostID, cmd)
}

func (s *Service) sendCommand(hostID string, cmd *agentrpc.SessionCommand) error {
	cmd.CommandID = models.NewID()
	key := pendingKey(hostID, cmd.CommandID)
	ackCh := make(chan *agentrpc.CommandResult, 1)

	s.pendingMu.Lock()
	s.pending[key] = ackCh
	s.pendingMu.Unlock()

	defer func() {
		s.pendingMu.Lock()
		delete(s.pending, key)
		s.pendingMu.Unlock()
	}()

	// Get connection (avoiding holding connMu while waiting for result)
	s.connMu.RLock()
	conn, ok := s.conns[hostID]
	s.connMu.RUnlock()

	if !ok {
		return fmt.Errorf("host %s is not connected via agent", hostID)
	}

	// Send command
	select {
	case <-conn.done:
		return fmt.Errorf("host %s agent stream closed", hostID)
	case conn.sendCh <- &agentrpc.ControlEnvelope{Command: cmd}:
		// Wait for command result
		select {
		case res, ok := <-ackCh:
			if !ok {
				return fmt.Errorf("command %s closed before result", cmd.CommandID)
			}
			if !res.Ok {
				if res.Message != "" {
					s.logger.Warn("agent command failed",
						"host_id", hostID,
						"command", cmd.Command,
						"session_id", cmd.SessionID,
						"error", res.Message)
					return fmt.Errorf("agent command %s failed: %s", cmd.Command, res.Message)
				}
				return fmt.Errorf("agent command %s failed", cmd.Command)
			}
			s.logger.Debug("agent command succeeded",
				"host_id", hostID,
				"command", cmd.Command,
				"session_id", cmd.SessionID)
			return nil
		case <-conn.done:
			return fmt.Errorf("host %s disconnected before command result", hostID)
		case <-time.After(commandResultTimeout):
			s.logger.Warn("agent command timed out",
				"host_id", hostID,
				"command", cmd.Command,
				"session_id", cmd.SessionID,
				"timeout", commandResultTimeout)
			return fmt.Errorf("timed out waiting for agent command result (%s)", cmd.Command)
		}
	case <-time.After(commandQueueTimeout):
		return fmt.Errorf("timed out queueing command for host %s", hostID)
	}
}

func (s *Service) resolvePendingCommand(hostID string, res *agentrpc.CommandResult) {
	if res == nil || res.CommandID == "" {
		return
	}

	key := pendingKey(hostID, res.CommandID)
	s.pendingMu.Lock()
	ch, ok := s.pending[key]
	s.pendingMu.Unlock()

	if !ok {
		s.logger.Debug("received command result for unknown command",
			"host_id", hostID,
			"command_id", res.CommandID)
		return
	}

	select {
	case ch <- res:
	default:
		s.logger.Warn("command result channel full, dropping result",
			"host_id", hostID,
			"command_id", res.CommandID)
	}
}

func pendingKey(hostID, commandID string) string {
	return hostID + ":" + commandID
}

func (s *Service) SubscribeSessionOutput(sessionID string) chan string {
	ch := make(chan string, outputSubChannelBuffer)
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	if s.outputSubs[sessionID] == nil {
		s.outputSubs[sessionID] = make(map[chan string]struct{})
	}
	s.outputSubs[sessionID][ch] = struct{}{}
	return ch
}

func (s *Service) UnsubscribeSessionOutput(sessionID string, ch chan string) {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	subs, ok := s.outputSubs[sessionID]
	if !ok {
		return
	}
	if _, exists := subs[ch]; exists {
		delete(subs, ch)
		close(ch)
	}
	if len(subs) == 0 {
		delete(s.outputSubs, sessionID)
	}
}

func (s *Service) publishOutput(sessionID, output string) {
	s.outputMu.RLock()
	defer s.outputMu.RUnlock()
	for ch := range s.outputSubs[sessionID] {
		select {
		case ch <- output:
		default:
			// Channel full, skip this update
		}
	}
}

func (s *Service) monitorLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(s.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			s.reconcileHeartbeatStatus(s.now())
		}
	}
}

func (s *Service) reconcileHeartbeatStatus(now time.Time) {
	hosts, err := s.store.ListHosts()
	if err != nil {
		s.logger.Error("heartbeat monitor: failed to list hosts", "error", err)
		return
	}

	for _, h := range hosts {
		next := desiredHostStatus(h, now, s.degradedAfter, s.offlineAfter)
		if next == h.Status {
			continue
		}
		if err := s.store.UpdateHostStatus(h.ID, next); err != nil {
			s.logger.Error("heartbeat monitor: failed to update status",
				"host_id", h.ID,
				"host_name", h.Name,
				"old_status", h.Status,
				"new_status", next,
				"error", err)
		} else {
			s.logger.Info("host status changed",
				"host_id", h.ID,
				"host_name", h.Name,
				"old_status", h.Status,
				"new_status", next)
		}
	}
}

func desiredHostStatus(h *models.Host, now time.Time, degradedAfter, offlineAfter time.Duration) models.HostStatus {
	if h.LastHeartbeat == nil {
		return models.HostStatusOffline
	}

	age := now.Sub(*h.LastHeartbeat)
	switch {
	case age >= offlineAfter:
		return models.HostStatusOffline
	case age >= degradedAfter:
		return models.HostStatusDegraded
	default:
		return models.HostStatusOnline
	}
}

// validateHello validates the hello message fields
func validateHello(hello *agentrpc.AgentHello) error {
	if hello.HostID == "" {
		return fmt.Errorf("hello.host_id is required")
	}
	if len(hello.HostID) > maxHostIDLength {
		return fmt.Errorf("hello.host_id too long (max %d characters)", maxHostIDLength)
	}
	if hello.AuthToken == "" {
		return fmt.Errorf("hello.auth_token is required")
	}
	if len(hello.AuthToken) > maxAuthTokenLength {
		return fmt.Errorf("hello.auth_token too long (max %d characters)", maxAuthTokenLength)
	}
	if len(hello.AgentVersion) > maxAgentVersionLength {
		return fmt.Errorf("hello.agent_version too long (max %d characters)", maxAgentVersionLength)
	}
	return nil
}

// authenticateAgent performs constant-time comparison of auth tokens
func authenticateAgent(expectedToken, providedToken string) bool {
	if expectedToken == "" || providedToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expectedToken), []byte(providedToken)) == 1
}

// peerAddr extracts peer address from context for logging
func peerAddr(ctx context.Context) string {
	// This would use grpc peer.FromContext in a real implementation
	// For now, return a placeholder
	return "unknown"
}
