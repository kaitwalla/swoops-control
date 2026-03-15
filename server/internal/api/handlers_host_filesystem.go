package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/pkg/sshexec"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

// DirectoryEntry represents a directory or file on a host.
type DirectoryEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// ListDirectoriesRequest is the request to list directories on a host.
type ListDirectoriesRequest struct {
	Path string `json:"path"`
}

// CreateDirectoryRequest is the request to create a directory on a host.
type CreateDirectoryRequest struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

// CloneRepositoryRequest is the request to clone a GitHub repository on a host.
type CloneRepositoryRequest struct {
	Path       string `json:"path"`        // Parent directory where to clone
	RepoURL    string `json:"repo_url"`    // Git repository URL
	FolderName string `json:"folder_name"` // Optional custom folder name
}

// handleListHostDirectories lists directories in a path on a host.
func (s *Server) handleListHostDirectories(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")
	if hostID == "" {
		http.Error(w, "host_id is required", http.StatusBadRequest)
		return
	}

	var req ListDirectoriesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" {
		http.Error(w, "path is required", http.StatusBadRequest)
		return
	}

	// Get host
	host, err := s.store.GetHost(hostID)
	if err == store.ErrNotFound {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Execute command to list directories
	// Using ls -1 -F to get directory names with trailing / for directories
	cmd := fmt.Sprintf("ls -1 -F %s 2>/dev/null | grep '/$' | sed 's/\\/$//'", shellQuote(req.Path))

	result, err := s.executeCommandOnHost(host, cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to list directories: %v", err), http.StatusInternalServerError)
		return
	}

	// Parse result
	var entries []DirectoryEntry
	lines := strings.Split(strings.TrimSpace(result), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		entries = append(entries, DirectoryEntry{
			Name:  line,
			Path:  filepath.Join(req.Path, line),
			IsDir: true,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// handleCreateHostDirectory creates a directory on a host.
func (s *Server) handleCreateHostDirectory(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")
	if hostID == "" {
		http.Error(w, "host_id is required", http.StatusBadRequest)
		return
	}

	var req CreateDirectoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" || req.Name == "" {
		http.Error(w, "path and name are required", http.StatusBadRequest)
		return
	}

	// Validate directory name (prevent path traversal)
	if strings.Contains(req.Name, "/") || strings.Contains(req.Name, "..") {
		http.Error(w, "invalid directory name", http.StatusBadRequest)
		return
	}

	// Get host
	host, err := s.store.GetHost(hostID)
	if err == store.ErrNotFound {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Create directory
	fullPath := filepath.Join(req.Path, req.Name)
	cmd := fmt.Sprintf("mkdir -p %s", shellQuote(fullPath))

	_, err = s.executeCommandOnHost(host, cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create directory: %v", err), http.StatusInternalServerError)
		return
	}

	// Return created directory info
	result := DirectoryEntry{
		Name:  req.Name,
		Path:  fullPath,
		IsDir: true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}

// handleCloneRepository clones a Git repository on a host.
func (s *Server) handleCloneRepository(w http.ResponseWriter, r *http.Request) {
	hostID := chi.URLParam(r, "id")
	if hostID == "" {
		http.Error(w, "host_id is required", http.StatusBadRequest)
		return
	}

	var req CloneRepositoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Path == "" || req.RepoURL == "" {
		http.Error(w, "path and repo_url are required", http.StatusBadRequest)
		return
	}

	// Get host
	host, err := s.store.GetHost(hostID)
	if err == store.ErrNotFound {
		http.Error(w, "host not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Build git clone command
	cloneCmd := fmt.Sprintf("cd %s && git clone %s", shellQuote(req.Path), shellQuote(req.RepoURL))
	if req.FolderName != "" {
		// Validate folder name
		if strings.Contains(req.FolderName, "/") || strings.Contains(req.FolderName, "..") {
			http.Error(w, "invalid folder name", http.StatusBadRequest)
			return
		}
		cloneCmd += " " + shellQuote(req.FolderName)
	}

	result, err := s.executeCommandOnHost(host, cloneCmd)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to clone repository: %v\n%s", err, result), http.StatusInternalServerError)
		return
	}

	// Determine the cloned directory name
	folderName := req.FolderName
	if folderName == "" {
		// Extract repo name from URL (e.g., https://github.com/user/repo.git -> repo)
		parts := strings.Split(req.RepoURL, "/")
		if len(parts) > 0 {
			folderName = strings.TrimSuffix(parts[len(parts)-1], ".git")
		}
	}

	// Return created directory info
	response := DirectoryEntry{
		Name:  folderName,
		Path:  filepath.Join(req.Path, folderName),
		IsDir: true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// executeCommandOnHost executes a command on a host via agent or SSH.
func (s *Server) executeCommandOnHost(host *models.Host, command string) (string, error) {
	// Try agent first if available
	if s.agentMgr != nil && s.agentMgr.IsHostConnected(host.ID) {
		return s.executeCommandViaAgent(host.ID, command)
	}

	// Fall back to SSH
	return s.executeCommandViaSSH(host, command)
}

// executeCommandViaAgent executes a command via the agent.
func (s *Server) executeCommandViaAgent(hostID, command string) (string, error) {
	// TODO: Implement agent-based command execution
	// For now, return an error
	return "", fmt.Errorf("agent-based command execution not yet implemented")
}

// executeCommandViaSSH executes a command via SSH.
func (s *Server) executeCommandViaSSH(host *models.Host, command string) (string, error) {
	client := sshexec.NewClient(host.Hostname, host.SSHPort, host.SSHUser, host.SSHKeyPath)
	output, err := client.Exec(command)
	if err != nil {
		return string(output), err
	}
	return string(output), nil
}

// shellQuote quotes a string for safe use in shell commands.
func shellQuote(s string) string {
	// Use single quotes and escape any single quotes in the string
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}
