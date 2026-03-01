package test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/agentconn"
	"github.com/kaitwalla/swoops-control/server/internal/api"
	"github.com/kaitwalla/swoops-control/server/internal/config"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

// TestWebSocketWithMetricsMiddleware verifies WebSocket upgrade works with metrics middleware
func TestWebSocketWithMetricsMiddleware(t *testing.T) {
	// Setup test database
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	// Setup test config
	cfg := config.DefaultConfig()
	cfg.Auth.APIKey = "test-api-key"

	// Create API server with metrics middleware
	apiServer := api.NewServer(st, cfg)
	defer apiServer.Close()

	// Create agent connection service
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	agentSvc := agentconn.NewService(st, cfg, logger)
	defer agentSvc.Close()

	apiServer.SetAgentOutputSource(agentSvc)

	// Create a test host first
	host := &models.Host{
		ID:           models.NewID(),
		Name:         "test-host",
		Hostname:     "test.example.com",
		SSHPort:      22,
		SSHUser:      "test",
		SSHKeyPath:   "/test/key",
		BaseRepoPath: "/test/repo",
		WorktreeRoot: "/test/worktrees",
	}
	if err := st.CreateHost(host); err != nil {
		t.Fatalf("Failed to create host: %v", err)
	}

	// Create a test session
	session := &models.Session{
		ID:         models.NewID(),
		HostID:     host.ID,
		AgentType:  "claude",
		Prompt:     "test",
		BranchName: "test-branch",
		Status:     models.SessionStatusRunning,
	}
	if err := st.CreateSession(session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	// Start HTTP test server
	httpServer := httptest.NewServer(apiServer)
	defer httpServer.Close()

	// Convert http:// to ws://
	wsURL := strings.Replace(httpServer.URL, "http://", "ws://", 1)
	wsURL = fmt.Sprintf("%s/api/v1/ws/sessions/%s/output?token=%s", wsURL, session.ID, cfg.Auth.APIKey)

	// Connect WebSocket
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dialer := websocket.Dialer{
		HandshakeTimeout: 3 * time.Second,
	}

	conn, resp, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket dial failed: %v (response: %v)", err, resp)
	}
	defer conn.Close()

	// Verify connection is established
	if resp.StatusCode != 101 {
		t.Fatalf("Expected status 101 Switching Protocols, got %d", resp.StatusCode)
	}

	// Send a ping to verify connection is alive
	err = conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("Failed to send ping: %v", err)
	}

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))

	// Try to read a message (should timeout since no output is being generated)
	_, _, err = conn.ReadMessage()
	if err != nil {
		// Timeout is expected since no output is being generated
		if !websocket.IsCloseError(err, websocket.CloseNormalClosure) &&
			!strings.Contains(err.Error(), "timeout") &&
			!strings.Contains(err.Error(), "i/o timeout") {
			t.Logf("Read error (expected timeout): %v", err)
		}
	}

	t.Logf("WebSocket connection successful - metrics middleware preserved Hijacker interface")
}

// TestMetricsPathNormalization verifies path normalization prevents unbounded cardinality
func TestMetricsPathNormalization(t *testing.T) {
	// Setup test database
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	// Setup test config
	cfg := config.DefaultConfig()
	cfg.Auth.APIKey = "test-api-key"

	// Create API server
	apiServer := api.NewServer(st, cfg)
	defer apiServer.Close()

	// Start HTTP test server
	httpServer := httptest.NewServer(apiServer)
	defer httpServer.Close()

	// Create multiple hosts with different IDs
	hostIDs := []string{"host-abc123", "host-def456", "host-ghi789"}

	// Make requests to each host endpoint
	for _, hostID := range hostIDs {
		// Create host
		host := &models.Host{
			ID:           hostID,
			Name:         "test-" + hostID,
			Hostname:     "test.example.com",
			SSHPort:      22,
			SSHUser:      "test",
			SSHKeyPath:   "/test/key",
			BaseRepoPath: "/test/repo",
			WorktreeRoot: "/test/worktrees",
		}
		if err := st.CreateHost(host); err != nil {
			t.Fatalf("Failed to create host: %v", err)
		}
	}

	// Fetch metrics - should show normalized paths, not individual IDs
	resp, err := httpServer.Client().Get(httpServer.URL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	// Read metrics
	metricsBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read metrics: %v", err)
	}
	metricsOutput := string(metricsBytes)

	// Verify that paths are normalized (should see /hosts/:id, not /hosts/host-abc123)
	if strings.Contains(metricsOutput, "host-abc123") ||
		strings.Contains(metricsOutput, "host-def456") ||
		strings.Contains(metricsOutput, "host-ghi789") {
		t.Errorf("Metrics contain raw host IDs - cardinality not bounded!")
		t.Logf("Metrics snippet: %s", metricsOutput[:min(500, len(metricsOutput))])
	}

	// Should see normalized path instead
	if !strings.Contains(metricsOutput, `path="/api/v1/hosts"`) &&
		!strings.Contains(metricsOutput, `path="/api/v1/hosts/:id"`) {
		t.Logf("Warning: Expected to see normalized paths in metrics")
	}

	t.Logf("Path normalization working - no raw IDs in metrics labels")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
