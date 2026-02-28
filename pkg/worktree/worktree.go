package worktree

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles git worktree operations.
type Manager struct {
	// ExecFunc overrides command execution for remote (SSH) use.
	ExecFunc func(name string, args ...string) ([]byte, error)
}

func (m *Manager) exec(name string, args ...string) ([]byte, error) {
	if m.ExecFunc != nil {
		return m.ExecFunc(name, args...)
	}
	return exec.Command(name, args...).CombinedOutput()
}

// Create creates a new git worktree at the given path, checking out (or creating) the branch.
// baseRepoPath is the path to the base git repository.
func (m *Manager) Create(baseRepoPath, worktreePath, branch string) error {
	// First, fetch to ensure we have latest refs
	out, err := m.exec("git", "-C", baseRepoPath, "fetch", "origin")
	if err != nil {
		return fmt.Errorf("git fetch: %w: %s", err, out)
	}

	// Create the worktree with a new branch based on origin/main (or origin/master)
	out, err = m.exec("git", "-C", baseRepoPath, "worktree", "add", "-b", branch, worktreePath, "HEAD")
	if err != nil {
		// Branch may already exist, try without -b
		out2, err2 := m.exec("git", "-C", baseRepoPath, "worktree", "add", worktreePath, branch)
		if err2 != nil {
			return fmt.Errorf("git worktree add: %w: %s / %s", err2, out, out2)
		}
	}
	return nil
}

// Remove removes a git worktree.
func (m *Manager) Remove(baseRepoPath, worktreePath string) error {
	out, err := m.exec("git", "-C", baseRepoPath, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		return fmt.Errorf("git worktree remove: %w: %s", err, out)
	}
	return nil
}

// List returns a list of worktree paths for the given base repo.
func (m *Manager) List(baseRepoPath string) ([]string, error) {
	out, err := m.exec("git", "-C", baseRepoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w: %s", err, out)
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "worktree ") {
			p := strings.TrimPrefix(line, "worktree ")
			// Skip the main worktree (the base repo itself)
			if filepath.Clean(p) != filepath.Clean(baseRepoPath) {
				paths = append(paths, p)
			}
		}
	}
	return paths, nil
}

// Prune cleans up stale worktree references.
func (m *Manager) Prune(baseRepoPath string) error {
	out, err := m.exec("git", "-C", baseRepoPath, "worktree", "prune")
	if err != nil {
		return fmt.Errorf("git worktree prune: %w: %s", err, out)
	}
	return nil
}
