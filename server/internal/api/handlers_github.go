package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/go-github/v60/github"
	"github.com/kaitwalla/swoops-control/server/internal/store"
	"golang.org/x/oauth2"
)

// UpdateGitHubTokenRequest is the request to update a user's GitHub token.
type UpdateGitHubTokenRequest struct {
	Token string `json:"token"`
}

// GitHubRepo represents a simplified GitHub repository response.
type GitHubRepo struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	FullName    string `json:"full_name"`
	Description string `json:"description,omitempty"`
	Private     bool   `json:"private"`
	HTMLURL     string `json:"html_url"`
	CloneURL    string `json:"clone_url"`
	SSHURL      string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
}

// CreateGitHubRepoRequest is the request to create a new GitHub repository.
type CreateGitHubRepoRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Private     bool   `json:"private"`
}

// handleUpdateGitHubToken updates the current user's GitHub personal access token.
func (s *Server) handleUpdateGitHubToken(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var req UpdateGitHubTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Token == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	// Validate token by attempting to get user info from GitHub
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: req.Token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	_, _, err := client.Users.Get(ctx, "")
	if err != nil {
		http.Error(w, "invalid GitHub token", http.StatusBadRequest)
		return
	}

	// Update token in database
	if err := s.store.UpdateUserGitHubToken(userID, req.Token); err != nil {
		http.Error(w, "failed to update GitHub token", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListGitHubRepos lists the current user's GitHub repositories.
func (s *Server) handleListGitHubRepos(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	// Get user's GitHub token
	user, err := s.store.GetUserByID(userID)
	if err == store.ErrNotFound {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if user.GitHubToken == "" {
		http.Error(w, "GitHub token not configured", http.StatusBadRequest)
		return
	}

	// Create GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: user.GitHubToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// List all repos (owned by user and accessible)
	opt := &github.RepositoryListOptions{
		Visibility:  "all",
		Affiliation: "owner,collaborator",
		Sort:        "updated",
		ListOptions: github.ListOptions{PerPage: 100},
	}

	var allRepos []*github.Repository
	for {
		repos, resp, err := client.Repositories.List(ctx, "", opt)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to list repositories: %v", err), http.StatusInternalServerError)
			return
		}
		allRepos = append(allRepos, repos...)
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}

	// Convert to simplified format
	result := make([]GitHubRepo, 0, len(allRepos))
	for _, repo := range allRepos {
		result = append(result, GitHubRepo{
			ID:          repo.GetID(),
			Name:        repo.GetName(),
			FullName:    repo.GetFullName(),
			Description: repo.GetDescription(),
			Private:     repo.GetPrivate(),
			HTMLURL:     repo.GetHTMLURL(),
			CloneURL:    repo.GetCloneURL(),
			SSHURL:      repo.GetSSHURL(),
			DefaultBranch: repo.GetDefaultBranch(),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleCreateGitHubRepo creates a new GitHub repository.
func (s *Server) handleCreateGitHubRepo(w http.ResponseWriter, r *http.Request) {
	userID, ok := UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}

	var req CreateGitHubRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "repository name is required", http.StatusBadRequest)
		return
	}

	// Get user's GitHub token
	user, err := s.store.GetUserByID(userID)
	if err == store.ErrNotFound {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if user.GitHubToken == "" {
		http.Error(w, "GitHub token not configured", http.StatusBadRequest)
		return
	}

	// Create GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: user.GitHubToken})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// Create repository
	repo := &github.Repository{
		Name:        github.String(req.Name),
		Description: github.String(req.Description),
		Private:     github.Bool(req.Private),
		AutoInit:    github.Bool(true), // Initialize with README
	}

	createdRepo, _, err := client.Repositories.Create(ctx, "", repo)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create repository: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to simplified format
	result := GitHubRepo{
		ID:          createdRepo.GetID(),
		Name:        createdRepo.GetName(),
		FullName:    createdRepo.GetFullName(),
		Description: createdRepo.GetDescription(),
		Private:     createdRepo.GetPrivate(),
		HTMLURL:     createdRepo.GetHTMLURL(),
		CloneURL:    createdRepo.GetCloneURL(),
		SSHURL:      createdRepo.GetSSHURL(),
		DefaultBranch: createdRepo.GetDefaultBranch(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(result)
}
