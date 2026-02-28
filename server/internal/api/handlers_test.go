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

	return NewServer(s, cfg)
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
