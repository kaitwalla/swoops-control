package api

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/swoopsh/swoops/pkg/models"
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
