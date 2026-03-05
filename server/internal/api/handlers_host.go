package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/certgen"
)

type createHostRequest struct {
	Name         string            `json:"name"`
	Hostname     string            `json:"hostname"`
	SSHPort      int               `json:"ssh_port"`
	SSHUser      string            `json:"ssh_user"`
	SSHKeyPath   string            `json:"ssh_key_path"`
	MaxSessions  int               `json:"max_sessions"`
	Labels       map[string]string `json:"labels"`
	BaseRepoPath string            `json:"base_repo_path"`
	WorktreeRoot string            `json:"worktree_root"`
}

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.store.ListHosts()
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if hosts == nil {
		hosts = []*models.Host{}
	}
	writeJSON(w, http.StatusOK, hosts)
}

func (s *Server) handleCreateHost(w http.ResponseWriter, r *http.Request) {
	var req createHostRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Hostname == "" || req.SSHUser == "" || req.SSHKeyPath == "" {
		writeError(w, http.StatusBadRequest, "name, hostname, ssh_user, and ssh_key_path are required")
		return
	}

	if req.SSHPort == 0 {
		req.SSHPort = 22
	}
	if req.MaxSessions == 0 {
		req.MaxSessions = 10
	}
	if req.BaseRepoPath == "" {
		req.BaseRepoPath = "/opt/swoops/repo"
	}
	if req.WorktreeRoot == "" {
		req.WorktreeRoot = "/opt/swoops/worktrees"
	}
	if req.Labels == nil {
		req.Labels = make(map[string]string)
	}

	now := time.Now()
	host := &models.Host{
		ID:           models.NewID(),
		Name:         req.Name,
		Hostname:     req.Hostname,
		SSHPort:      req.SSHPort,
		SSHUser:      req.SSHUser,
		SSHKeyPath:   req.SSHKeyPath,
		Status:       models.HostStatusOffline,
		Labels:       req.Labels,
		MaxSessions:  req.MaxSessions,
		BaseRepoPath: req.BaseRepoPath,
		WorktreeRoot: req.WorktreeRoot,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.store.CreateHost(host); err != nil {
		writeInternalError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, host)
}

func (s *Server) handleGetHost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host, err := s.store.GetHost(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "host not found")
			return
		}
		writeInternalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, host)
}

func (s *Server) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host, err := s.store.GetHost(id)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "host not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	var req createHostRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != "" {
		host.Name = req.Name
	}
	if req.Hostname != "" {
		host.Hostname = req.Hostname
	}
	if req.SSHPort != 0 {
		host.SSHPort = req.SSHPort
	}
	if req.SSHUser != "" {
		host.SSHUser = req.SSHUser
	}
	if req.SSHKeyPath != "" {
		host.SSHKeyPath = req.SSHKeyPath
	}
	if req.MaxSessions != 0 {
		host.MaxSessions = req.MaxSessions
	}
	if req.Labels != nil {
		host.Labels = req.Labels
	}
	if req.BaseRepoPath != "" {
		host.BaseRepoPath = req.BaseRepoPath
	}
	if req.WorktreeRoot != "" {
		host.WorktreeRoot = req.WorktreeRoot
	}

	if err := s.store.UpdateHost(host); err != nil {
		if writeStoreError(w, err, "host not found") {
			return
		}
		return
	}

	writeJSON(w, http.StatusOK, host)
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := s.store.DeleteHost(id); err != nil {
		if writeStoreError(w, err, "host not found") {
			return
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListHostSessions(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")
	sessions, err := s.store.ListSessions(hostID, "")
	if err != nil {
		writeInternalError(w, err)
		return
	}
	if sessions == nil {
		sessions = []*models.Session{}
	}
	writeJSON(w, http.StatusOK, sessions)
}

type createAgentHostRequest struct {
	Name string `json:"name"`
}

type createAgentHostResponse struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AuthToken  string `json:"auth_token"`
	ClientCert string `json:"client_cert,omitempty"` // PEM-encoded client certificate for mTLS
	ClientKey  string `json:"client_key,omitempty"`  // PEM-encoded client private key for mTLS
}

// handleCreateAgentHost creates a minimal host record for agent-based hosts
// This is used when setting up a new agent via the UI
func (s *Server) handleCreateAgentHost(w http.ResponseWriter, r *http.Request) {
	var req createAgentHostRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	now := time.Now()
	host := &models.Host{
		ID:           models.NewID(),
		Name:         req.Name,
		Hostname:     "agent-managed", // Placeholder, agent will update this
		SSHPort:      22,
		SSHUser:      "",      // Not needed for agent-based hosts
		SSHKeyPath:   "",      // Not needed for agent-based hosts
		Status:       models.HostStatusOffline,
		Labels:       map[string]string{"type": "agent"},
		MaxSessions:  10,
		BaseRepoPath: "/opt/swoops/repo",
		WorktreeRoot: "/opt/swoops/worktrees",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.store.CreateHost(host); err != nil {
		writeInternalError(w, err)
		return
	}

	response := createAgentHostResponse{
		ID:        host.ID,
		Name:      host.Name,
		AuthToken: host.AgentAuthToken,
	}

	// Generate client certificates if mTLS is enabled and CA key is available
	if s.config.GRPC.RequireMTLS && s.config.GRPC.ClientCAKey != "" {
		certPEM, keyPEM, err := s.generateClientCert(host.ID)
		if err != nil {
			// Don't fail the request, but log the error
			// Note: we don't fail here because the cert can still be downloaded later
		} else {
			response.ClientCert = string(certPEM)
			response.ClientKey = string(keyPEM)
		}
	}

	// Return the host with auth token (only time we expose it)
	writeJSON(w, http.StatusCreated, response)
}

// generateClientCert generates a client certificate for the given host ID
func (s *Server) generateClientCert(hostID string) (certPEM, keyPEM []byte, err error) {
	commonName := fmt.Sprintf("swoops-agent-%s", hostID)
	return certgen.GenerateClientCertificateFromFiles(
		s.config.GRPC.ClientCA,
		s.config.GRPC.ClientCAKey,
		commonName,
	)
}

// handleGetClientCert returns client certificates for a host (one-time use, requires auth token)
func (s *Server) handleGetClientCert(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")
	authToken := r.URL.Query().Get("auth_token")

	if authToken == "" {
		writeError(w, http.StatusUnauthorized, "auth_token query parameter is required")
		return
	}

	// Get host and verify auth token
	host, err := s.store.GetHost(hostID)
	if err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "host not found")
			return
		}
		writeInternalError(w, err)
		return
	}

	if host.AgentAuthToken != authToken {
		writeError(w, http.StatusUnauthorized, "invalid auth token")
		return
	}

	// Generate client certificate
	certPEM, keyPEM, err := s.generateClientCert(hostID)
	if err != nil {
		writeInternalError(w, err)
		return
	}

	// Return as JSON
	writeJSON(w, http.StatusOK, map[string]string{
		"client_cert": string(certPEM),
		"client_key":  string(keyPEM),
	})
}
