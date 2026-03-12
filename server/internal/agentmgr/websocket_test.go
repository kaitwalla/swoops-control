package agentmgr

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

func TestAgentWebSocketConnection(t *testing.T) {
	// Create in-memory database
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// Create a test host
	host := &models.Host{
		ID:             models.NewID(),
		Name:           "test-host",
		Hostname:       "localhost",
		Status:         models.HostStatusOnline,
		AgentAuthToken: "test-token-123",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.CreateHost(host); err != nil {
		t.Fatalf("failed to create host: %v", err)
	}

	// Create agent manager
	am := New(s, slog.Default())
	defer am.Close()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(am.HandleAgentWebSocket))
	defer server.Close()

	// Connect to WebSocket with correct token
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=test-token-123"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer conn.Close()

	// Verify connection is registered
	am.wsConnsMu.RLock()
	_, exists := am.wsConns[host.ID]
	am.wsConnsMu.RUnlock()

	if !exists {
		t.Fatal("expected websocket to be registered")
	}

	// Queue a command
	cmd := &agentrpc.SessionCommand{
		CommandID: "cmd-123",
		SessionID: "sess-456",
		Command:   "launch_session",
		Args: map[string]string{
			"session_type": "shell",
		},
	}

	if err := am.QueueCommand(host.ID, cmd); err != nil {
		t.Fatalf("failed to queue command: %v", err)
	}

	// Read notification from WebSocket
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg map[string]interface{}
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("failed to read notification: %v", err)
	}

	// Verify notification type
	if msg["type"] != "new_command" {
		t.Fatalf("expected type 'new_command', got %v", msg["type"])
	}

	// Retrieve pending commands
	commands, err := am.GetPendingCommands(context.Background(), host.ID)
	if err != nil {
		t.Fatalf("failed to get pending commands: %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	if commands[0].CommandID != "cmd-123" {
		t.Fatalf("expected command ID 'cmd-123', got %s", commands[0].CommandID)
	}

	// Verify queue is cleared
	commands, err = am.GetPendingCommands(context.Background(), host.ID)
	if err != nil {
		t.Fatalf("failed to get pending commands: %v", err)
	}

	if len(commands) != 0 {
		t.Fatalf("expected queue to be cleared, got %d commands", len(commands))
	}
}

func TestAgentWebSocketAuthFailure(t *testing.T) {
	// Create in-memory database
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// Create agent manager
	am := New(s, slog.Default())
	defer am.Close()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(am.HandleAgentWebSocket))
	defer server.Close()

	// Try to connect with invalid token
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=invalid-token"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)

	// Connection should fail
	if err == nil {
		t.Fatal("expected connection to fail with invalid token")
	}

	// Should get 401 Unauthorized
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestAgentWebSocketMissingToken(t *testing.T) {
	// Create in-memory database
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// Create agent manager
	am := New(s, slog.Default())
	defer am.Close()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(am.HandleAgentWebSocket))
	defer server.Close()

	// Try to connect without token
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)

	// Connection should fail
	if err == nil {
		t.Fatal("expected connection to fail without token")
	}

	// Should get 401 Unauthorized
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.StatusCode)
	}
}

func TestAgentWebSocketReconnection(t *testing.T) {
	// Create in-memory database
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// Create a test host
	host := &models.Host{
		ID:             models.NewID(),
		Name:           "test-host",
		Hostname:       "localhost",
		Status:         models.HostStatusOnline,
		AgentAuthToken: "test-token-123",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.CreateHost(host); err != nil {
		t.Fatalf("failed to create host: %v", err)
	}

	// Create agent manager
	am := New(s, slog.Default())
	defer am.Close()

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(am.HandleAgentWebSocket))
	defer server.Close()

	// First connection
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "?token=test-token-123"
	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect first websocket: %v", err)
	}

	// Wait a bit for registration
	time.Sleep(100 * time.Millisecond)

	// Second connection (should close the first one)
	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect second websocket: %v", err)
	}
	defer conn2.Close()

	// Wait a bit
	time.Sleep(100 * time.Millisecond)

	// Verify only one connection is registered
	am.wsConnsMu.RLock()
	count := 0
	if _, exists := am.wsConns[host.ID]; exists {
		count++
	}
	am.wsConnsMu.RUnlock()

	if count != 1 {
		t.Fatalf("expected 1 active connection, got %d", count)
	}

	// First connection should be closed
	conn1.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	_, _, err = conn1.ReadMessage()
	if err == nil {
		t.Fatal("expected first connection to be closed")
	}
}

func TestNotifyNewCommandWithoutWebSocket(t *testing.T) {
	// Create in-memory database
	s, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer s.Close()

	// Create a test host
	host := &models.Host{
		ID:             models.NewID(),
		Name:           "test-host",
		Hostname:       "localhost",
		Status:         models.HostStatusOnline,
		AgentAuthToken: "test-token-123",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.CreateHost(host); err != nil {
		t.Fatalf("failed to create host: %v", err)
	}

	// Create agent manager
	am := New(s, slog.Default())
	defer am.Close()

	// Queue a command without WebSocket connected (should not panic)
	cmd := &agentrpc.SessionCommand{
		CommandID: "cmd-123",
		SessionID: "sess-456",
		Command:   "launch_session",
		Args: map[string]string{
			"session_type": "shell",
		},
	}

	if err := am.QueueCommand(host.ID, cmd); err != nil {
		t.Fatalf("failed to queue command: %v", err)
	}

	// Should be able to retrieve command later (polling fallback)
	commands, err := am.GetPendingCommands(context.Background(), host.ID)
	if err != nil {
		t.Fatalf("failed to get pending commands: %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}
}
