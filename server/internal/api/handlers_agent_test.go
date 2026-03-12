package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/agentmgr"
	"github.com/kaitwalla/swoops-control/server/internal/config"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

func TestAgentHeartbeat(t *testing.T) {
	// Setup test database
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer db.Close()

	// Create test host
	host := &models.Host{
		ID:             models.NewID(),
		Name:           "test-host",
		Hostname:       "test.example.com",
		SSHPort:        22,
		SSHUser:        "test",
		SSHKeyPath:     "/tmp/key",
		Status:         models.HostStatusOffline,
		Labels:         map[string]string{},
		MaxSessions:    10,
		BaseRepoPath:   "/tmp/repo",
		WorktreeRoot:   "/tmp/worktrees",
		AgentAuthToken: "test-token-123",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateHost(host); err != nil {
		t.Fatalf("failed to create host: %v", err)
	}

	// Create server with agent manager
	cfg := &config.Config{}
	srv := NewServer(db, cfg)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	agentMgr := agentmgr.New(db, logger)
	srv.SetAgentManager(agentMgr)

	// Create heartbeat request
	req := agentmgr.HeartbeatRequest{
		HostID:          host.ID,
		RunningSessions: 3,
		UpdateAvailable: false,
		CurrentVersion:  "1.0.0",
		LatestVersion:   "1.0.0",
		UpdateURL:       "",
	}
	body, _ := json.Marshal(req)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/api/v1/agent/heartbeat", bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer test-token-123")
	w := httptest.NewRecorder()

	// Call handler with auth middleware
	handler := srv.AgentAuth()(http.HandlerFunc(srv.handleAgentHeartbeat))
	handler.ServeHTTP(w, httpReq)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify host was updated
	updatedHost, err := db.GetHost(host.ID)
	if err != nil {
		t.Fatalf("failed to get updated host: %v", err)
	}

	if updatedHost.Status != models.HostStatusOnline {
		t.Errorf("expected status online, got %s", updatedHost.Status)
	}

	if updatedHost.LastHeartbeat == nil {
		t.Error("expected last_heartbeat to be set")
	}
}

func TestGetPendingCommands(t *testing.T) {
	// Setup test database
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer db.Close()

	// Create test host
	host := &models.Host{
		ID:             models.NewID(),
		Name:           "test-host",
		Hostname:       "test.example.com",
		SSHPort:        22,
		SSHUser:        "test",
		SSHKeyPath:     "/tmp/key",
		Status:         models.HostStatusOnline,
		Labels:         map[string]string{},
		MaxSessions:    10,
		BaseRepoPath:   "/tmp/repo",
		WorktreeRoot:   "/tmp/worktrees",
		AgentAuthToken: "test-token-456",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateHost(host); err != nil {
		t.Fatalf("failed to create host: %v", err)
	}

	// Create server with agent manager
	cfg := &config.Config{}
	srv := NewServer(db, cfg)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	agentMgr := agentmgr.New(db, logger)
	srv.SetAgentManager(agentMgr)

	// Queue a command
	cmd := &agentrpc.SessionCommand{
		CommandID: "cmd-123",
		SessionID: "sess-456",
		Command:   "launch_session",
		Args: map[string]string{
			"session_type": "shell",
			"work_dir":     "/tmp",
		},
	}
	if err := agentMgr.QueueCommand(host.ID, cmd); err != nil {
		t.Fatalf("failed to queue command: %v", err)
	}

	// Create HTTP request
	httpReq := httptest.NewRequest("GET", "/api/v1/agent/commands/pending", nil)
	httpReq.Header.Set("Authorization", "Bearer test-token-456")
	w := httptest.NewRecorder()

	// Call handler with auth middleware
	handler := srv.AgentAuth()(http.HandlerFunc(srv.handleGetPendingCommands))
	handler.ServeHTTP(w, httpReq)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Parse response
	var resp struct {
		Commands []*agentrpc.SessionCommand `json:"commands"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify command was returned
	if len(resp.Commands) != 1 {
		t.Errorf("expected 1 command, got %d", len(resp.Commands))
	}

	if len(resp.Commands) > 0 {
		if resp.Commands[0].CommandID != cmd.CommandID {
			t.Errorf("expected command_id %s, got %s", cmd.CommandID, resp.Commands[0].CommandID)
		}
	}

	// Second request should return empty (commands already dequeued)
	httpReq2 := httptest.NewRequest("GET", "/api/v1/agent/commands/pending", nil)
	httpReq2.Header.Set("Authorization", "Bearer test-token-456")
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, httpReq2)

	var resp2 struct {
		Commands []*agentrpc.SessionCommand `json:"commands"`
	}
	if err := json.NewDecoder(w2.Body).Decode(&resp2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp2.Commands) != 0 {
		t.Errorf("expected 0 commands on second call, got %d", len(resp2.Commands))
	}
}

func TestAgentCommandResult(t *testing.T) {
	// Setup test database
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer db.Close()

	// Create test host
	host := &models.Host{
		ID:             models.NewID(),
		Name:           "test-host",
		Hostname:       "test.example.com",
		SSHPort:        22,
		SSHUser:        "test",
		SSHKeyPath:     "/tmp/key",
		Status:         models.HostStatusOnline,
		Labels:         map[string]string{},
		MaxSessions:    10,
		BaseRepoPath:   "/tmp/repo",
		WorktreeRoot:   "/tmp/worktrees",
		AgentAuthToken: "test-token-789",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateHost(host); err != nil {
		t.Fatalf("failed to create host: %v", err)
	}

	// Create test session
	sess := &models.Session{
		ID:              models.NewID(),
		Name:            "test-session",
		HostID:          host.ID,
		Type:            models.SessionTypeShell,
		Status:          models.SessionStatusPending,
		Prompt:          "echo ready",
		BranchName:      "main",
		WorktreePath:    "/tmp/worktree",
		TmuxSessionName: "tmux-1",
		EnvVars:         map[string]string{},
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	if err := db.CreateSession(sess); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Create server with agent manager
	cfg := &config.Config{}
	srv := NewServer(db, cfg)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	agentMgr := agentmgr.New(db, logger)
	srv.SetAgentManager(agentMgr)

	// Create command result
	result := agentrpc.CommandResult{
		CommandID: "cmd-123",
		SessionID: sess.ID,
		Ok:        true,
		Message:   "session launched successfully",
	}
	body, _ := json.Marshal(result)

	// Create HTTP request
	httpReq := httptest.NewRequest("POST", "/api/v1/agent/command-results", bytes.NewReader(body))
	httpReq.Header.Set("Authorization", "Bearer test-token-789")
	w := httptest.NewRecorder()

	// Call handler with auth middleware
	handler := srv.AgentAuth()(http.HandlerFunc(srv.handleAgentCommandResult))
	handler.ServeHTTP(w, httpReq)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify session status was updated
	updatedSess, err := db.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("failed to get updated session: %v", err)
	}

	if updatedSess.Status != models.SessionStatusRunning {
		t.Errorf("expected status running, got %s", updatedSess.Status)
	}
}

func TestAgentAuthMiddleware(t *testing.T) {
	// Setup test database
	db, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer db.Close()

	// Create test host
	host := &models.Host{
		ID:             models.NewID(),
		Name:           "test-host",
		Hostname:       "test.example.com",
		SSHPort:        22,
		SSHUser:        "test",
		SSHKeyPath:     "/tmp/key",
		Status:         models.HostStatusOnline,
		Labels:         map[string]string{},
		MaxSessions:    10,
		BaseRepoPath:   "/tmp/repo",
		WorktreeRoot:   "/tmp/worktrees",
		AgentAuthToken: "valid-token",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := db.CreateHost(host); err != nil {
		t.Fatalf("failed to create host: %v", err)
	}

	// Create server
	cfg := &config.Config{}
	srv := NewServer(db, cfg)

	// Test valid token
	t.Run("valid token", func(t *testing.T) {
		httpReq := httptest.NewRequest("GET", "/test", nil)
		httpReq.Header.Set("Authorization", "Bearer valid-token")
		w := httptest.NewRecorder()

		handler := srv.AgentAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hostID, ok := HostIDFromContext(r.Context())
			if !ok {
				t.Error("expected host_id in context")
			}
			if hostID != host.ID {
				t.Errorf("expected host_id %s, got %s", host.ID, hostID)
			}
			w.WriteHeader(http.StatusOK)
		}))
		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
	})

	// Test invalid token
	t.Run("invalid token", func(t *testing.T) {
		httpReq := httptest.NewRequest("GET", "/test", nil)
		httpReq.Header.Set("Authorization", "Bearer invalid-token")
		w := httptest.NewRecorder()

		handler := srv.AgentAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called with invalid token")
		}))
		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})

	// Test missing token
	t.Run("missing token", func(t *testing.T) {
		httpReq := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		handler := srv.AgentAuth()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("handler should not be called without token")
		}))
		handler.ServeHTTP(w, httpReq)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", w.Code)
		}
	})
}
