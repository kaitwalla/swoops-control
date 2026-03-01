package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/swoopsh/swoops/pkg/models"
	"github.com/swoopsh/swoops/server/internal/config"
	"github.com/swoopsh/swoops/server/internal/store"
)

const testAPIKey = "test-api-key-12345"

func testServer(t *testing.T) *Server {
	t.Helper()
	tmp, err := os.CreateTemp("", "swoops-api-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	s, err := store.New(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })

	cfg := config.DefaultConfig()
	cfg.Auth.APIKey = testAPIKey

	srv := NewServer(s, cfg)

	// Stub out the async launcher so tests never attempt SSH
	srv.launchFunc = func(sessionID, hostID string) error { return nil }

	return srv
}

func doRequest(srv *Server, method, path string, body interface{}, apiKey string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestHealthNoAuth(t *testing.T) {
	srv := testServer(t)
	w := doRequest(srv, "GET", "/api/v1/health", nil, "")
	if w.Code != http.StatusOK {
		t.Errorf("health got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestStatsRequiresAuth(t *testing.T) {
	srv := testServer(t)

	// No auth
	w := doRequest(srv, "GET", "/api/v1/stats", nil, "")
	if w.Code != http.StatusUnauthorized {
		t.Errorf("stats without auth got %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// Wrong key
	w = doRequest(srv, "GET", "/api/v1/stats", nil, "wrong-key")
	if w.Code != http.StatusForbidden {
		t.Errorf("stats with wrong key got %d, want %d", w.Code, http.StatusForbidden)
	}

	// Correct key
	w = doRequest(srv, "GET", "/api/v1/stats", nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Errorf("stats with correct key got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestHostCRUDEndpoints(t *testing.T) {
	srv := testServer(t)

	// List (empty)
	w := doRequest(srv, "GET", "/api/v1/hosts", nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Fatalf("list hosts got %d", w.Code)
	}
	var hosts []models.Host
	json.NewDecoder(w.Body).Decode(&hosts)
	if len(hosts) != 0 {
		t.Errorf("got %d hosts, want 0", len(hosts))
	}

	// Create
	body := map[string]interface{}{
		"name": "test-host", "hostname": "10.0.0.1",
		"ssh_user": "deploy", "ssh_key_path": "/tmp/key",
	}
	w = doRequest(srv, "POST", "/api/v1/hosts", body, testAPIKey)
	if w.Code != http.StatusCreated {
		t.Fatalf("create host got %d: %s", w.Code, w.Body.String())
	}
	var created models.Host
	json.NewDecoder(w.Body).Decode(&created)
	if created.Name != "test-host" {
		t.Errorf("created host name %q", created.Name)
	}

	// Get
	w = doRequest(srv, "GET", "/api/v1/hosts/"+created.ID, nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Errorf("get host got %d", w.Code)
	}

	// Get nonexistent
	w = doRequest(srv, "GET", "/api/v1/hosts/nonexistent", nil, testAPIKey)
	if w.Code != http.StatusNotFound {
		t.Errorf("get nonexistent host got %d, want 404", w.Code)
	}

	// Delete
	w = doRequest(srv, "DELETE", "/api/v1/hosts/"+created.ID, nil, testAPIKey)
	if w.Code != http.StatusNoContent {
		t.Errorf("delete host got %d", w.Code)
	}

	// Delete nonexistent
	w = doRequest(srv, "DELETE", "/api/v1/hosts/nonexistent", nil, testAPIKey)
	if w.Code != http.StatusNotFound {
		t.Errorf("delete nonexistent host got %d, want 404", w.Code)
	}
}

func TestCreateHostValidation(t *testing.T) {
	srv := testServer(t)

	// Missing required fields
	body := map[string]interface{}{"name": "x"}
	w := doRequest(srv, "POST", "/api/v1/hosts", body, testAPIKey)
	if w.Code != http.StatusBadRequest {
		t.Errorf("create host missing fields got %d, want 400", w.Code)
	}
}

func TestSessionCRUDEndpoints(t *testing.T) {
	srv := testServer(t)

	// Create host first
	hostBody := map[string]interface{}{
		"name": "sess-host", "hostname": "10.0.0.1",
		"ssh_user": "deploy", "ssh_key_path": "/tmp/key",
	}
	w := doRequest(srv, "POST", "/api/v1/hosts", hostBody, testAPIKey)
	if w.Code != http.StatusCreated {
		t.Fatalf("create host got %d", w.Code)
	}
	var host models.Host
	json.NewDecoder(w.Body).Decode(&host)

	// Create session — response must always be "pending" (race-free snapshot)
	sessBody := map[string]interface{}{
		"host_id":    host.ID,
		"agent_type": "claude",
		"prompt":     "fix the bug in auth.go",
	}
	w = doRequest(srv, "POST", "/api/v1/sessions", sessBody, testAPIKey)
	if w.Code != http.StatusCreated {
		t.Fatalf("create session got %d: %s", w.Code, w.Body.String())
	}
	var sess models.Session
	json.NewDecoder(w.Body).Decode(&sess)
	if sess.AgentType != "claude" {
		t.Errorf("session agent_type %q, want claude", sess.AgentType)
	}
	if sess.Status != "pending" {
		t.Errorf("session status %q, want pending", sess.Status)
	}
	if sess.BranchName == "" {
		t.Error("session branch_name should be auto-generated")
	}

	// Get session
	w = doRequest(srv, "GET", "/api/v1/sessions/"+sess.ID, nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Errorf("get session got %d", w.Code)
	}

	// List sessions
	w = doRequest(srv, "GET", "/api/v1/sessions", nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Fatalf("list sessions got %d", w.Code)
	}
	var sessions []models.Session
	json.NewDecoder(w.Body).Decode(&sessions)
	if len(sessions) != 1 {
		t.Errorf("got %d sessions, want 1", len(sessions))
	}

	// List sessions by host
	w = doRequest(srv, "GET", "/api/v1/hosts/"+host.ID+"/sessions", nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Fatalf("list host sessions got %d", w.Code)
	}

	// Get output — session is pending with no tmux, returns stored (empty) output
	w = doRequest(srv, "GET", "/api/v1/sessions/"+sess.ID+"/output", nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Errorf("get output got %d", w.Code)
	}

	// Send input — session is pending, has no tmux_session, expect 500 (no tmux)
	inputBody := map[string]interface{}{"input": "test input"}
	w = doRequest(srv, "POST", "/api/v1/sessions/"+sess.ID+"/input", inputBody, testAPIKey)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("send input to pending session got %d, want 500", w.Code)
	}

	// Stop session — updates status to stopped via store (no SSH needed)
	w = doRequest(srv, "POST", "/api/v1/sessions/"+sess.ID+"/stop", nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Errorf("stop session got %d, want 200", w.Code)
	}

	// Verify stopped
	w = doRequest(srv, "GET", "/api/v1/sessions/"+sess.ID, nil, testAPIKey)
	var stopped models.Session
	json.NewDecoder(w.Body).Decode(&stopped)
	if stopped.Status != "stopped" {
		t.Errorf("session status after stop %q, want stopped", stopped.Status)
	}

	// Stop again — already stopped, should return current status
	w = doRequest(srv, "POST", "/api/v1/sessions/"+sess.ID+"/stop", nil, testAPIKey)
	if w.Code != http.StatusOK {
		t.Errorf("stop already-stopped session got %d, want 200", w.Code)
	}

	// Get nonexistent session
	w = doRequest(srv, "GET", "/api/v1/sessions/nonexistent", nil, testAPIKey)
	if w.Code != http.StatusNotFound {
		t.Errorf("get nonexistent session got %d, want 404", w.Code)
	}

	// Delete session
	w = doRequest(srv, "DELETE", "/api/v1/sessions/"+sess.ID, nil, testAPIKey)
	if w.Code != http.StatusNoContent {
		t.Errorf("delete session got %d, want 204", w.Code)
	}
}

func TestCreateSessionReturnsStableSnapshot(t *testing.T) {
	srv := testServer(t)

	// Create host
	hostBody := map[string]interface{}{
		"name": "snap-host", "hostname": "10.0.0.2",
		"ssh_user": "deploy", "ssh_key_path": "/tmp/key",
	}
	w := doRequest(srv, "POST", "/api/v1/hosts", hostBody, testAPIKey)
	if w.Code != http.StatusCreated {
		t.Fatalf("create host got %d", w.Code)
	}
	var host models.Host
	json.NewDecoder(w.Body).Decode(&host)

	// Create session
	sessBody := map[string]interface{}{
		"host_id":    host.ID,
		"agent_type": "codex",
		"prompt":     "refactor utils",
	}
	w = doRequest(srv, "POST", "/api/v1/sessions", sessBody, testAPIKey)
	if w.Code != http.StatusCreated {
		t.Fatalf("create session got %d", w.Code)
	}
	var sess models.Session
	json.NewDecoder(w.Body).Decode(&sess)

	// The response must be a clean "pending" snapshot, never reflecting
	// any mutations from the async launcher goroutine.
	if sess.Status != "pending" {
		t.Errorf("create response status %q, want pending (race-free snapshot)", sess.Status)
	}
	if sess.WorktreePath != "" {
		t.Errorf("create response worktree_path %q, want empty (not yet launched)", sess.WorktreePath)
	}
	if sess.TmuxSessionName != "" {
		t.Errorf("create response tmux_session %q, want empty (not yet launched)", sess.TmuxSessionName)
	}
}

func TestCreateSessionValidation(t *testing.T) {
	srv := testServer(t)

	// Missing required fields
	w := doRequest(srv, "POST", "/api/v1/sessions", map[string]interface{}{}, testAPIKey)
	if w.Code != http.StatusBadRequest {
		t.Errorf("create session with no fields got %d, want 400", w.Code)
	}

	// Invalid agent type
	w = doRequest(srv, "POST", "/api/v1/sessions", map[string]interface{}{
		"host_id": "x", "agent_type": "invalid", "prompt": "test",
	}, testAPIKey)
	if w.Code != http.StatusBadRequest {
		t.Errorf("create session with invalid agent_type got %d, want 400", w.Code)
	}

	// Non-existent host
	w = doRequest(srv, "POST", "/api/v1/sessions", map[string]interface{}{
		"host_id": "nonexistent", "agent_type": "claude", "prompt": "test",
	}, testAPIKey)
	if w.Code != http.StatusBadRequest {
		t.Errorf("create session with nonexistent host got %d, want 400", w.Code)
	}
}

func TestTokenQueryParamAuth(t *testing.T) {
	srv := testServer(t)

	// Auth via query param
	req := httptest.NewRequest("GET", "/api/v1/stats?token="+testAPIKey, nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("auth via query param got %d, want 200", w.Code)
	}

	// Wrong token via query param
	req = httptest.NewRequest("GET", "/api/v1/stats?token=wrong-key", nil)
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("auth via wrong query param got %d, want 403", w.Code)
	}
}

func TestInternalErrorsNotLeaked(t *testing.T) {
	srv := testServer(t)

	// Create a host, then create it again (duplicate name)
	body := map[string]interface{}{
		"name": "dup", "hostname": "10.0.0.1",
		"ssh_user": "u", "ssh_key_path": "/k",
	}
	w := doRequest(srv, "POST", "/api/v1/hosts", body, testAPIKey)
	if w.Code != http.StatusCreated {
		t.Fatalf("first create got %d", w.Code)
	}

	w = doRequest(srv, "POST", "/api/v1/hosts", body, testAPIKey)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("duplicate create got %d, want 500", w.Code)
	}

	var errResp map[string]string
	json.NewDecoder(w.Body).Decode(&errResp)
	if errResp["error"] != "internal server error" {
		t.Errorf("error response leaked internals: %q", errResp["error"])
	}
}
