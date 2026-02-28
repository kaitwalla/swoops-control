package tmux

import (
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes tmux commands. For local use, leave ExecFunc nil.
// For remote (SSH) use, set ExecFunc to execute commands via SSH.
type Runner struct {
	// ExecFunc overrides how commands are executed.
	// If nil, commands run locally via os/exec.
	ExecFunc func(name string, args ...string) ([]byte, error)
}

func (r *Runner) exec(name string, args ...string) ([]byte, error) {
	if r.ExecFunc != nil {
		return r.ExecFunc(name, args...)
	}
	return exec.Command(name, args...).CombinedOutput()
}

// CreateSession creates a new detached tmux session.
func (r *Runner) CreateSession(name, workDir string) error {
	args := []string{"new-session", "-d", "-s", name}
	if workDir != "" {
		args = append(args, "-c", workDir)
	}
	out, err := r.exec("tmux", args...)
	if err != nil {
		return fmt.Errorf("tmux new-session: %w: %s", err, out)
	}
	return nil
}

// SendKeys sends text to a tmux session's active pane.
func (r *Runner) SendKeys(sessionName, text string) error {
	out, err := r.exec("tmux", "send-keys", "-t", sessionName, text, "Enter")
	if err != nil {
		return fmt.Errorf("tmux send-keys: %w: %s", err, out)
	}
	return nil
}

// CapturePane captures the visible content of a tmux pane.
// Returns the last `lines` lines of the pane content.
func (r *Runner) CapturePane(sessionName string, lines int) (string, error) {
	startLine := fmt.Sprintf("-%d", lines)
	out, err := r.exec("tmux", "capture-pane", "-t", sessionName, "-p", "-S", startLine)
	if err != nil {
		return "", fmt.Errorf("tmux capture-pane: %w: %s", err, out)
	}
	return string(out), nil
}

// KillSession destroys a tmux session.
func (r *Runner) KillSession(sessionName string) error {
	out, err := r.exec("tmux", "kill-session", "-t", sessionName)
	if err != nil {
		return fmt.Errorf("tmux kill-session: %w: %s", err, out)
	}
	return nil
}

// HasSession checks if a tmux session exists.
func (r *Runner) HasSession(sessionName string) bool {
	_, err := r.exec("tmux", "has-session", "-t", sessionName)
	return err == nil
}

// ListSessions returns a list of active tmux session names.
func (r *Runner) ListSessions() ([]string, error) {
	out, err := r.exec("tmux", "list-sessions", "-F", "#{session_name}")
	if err != nil {
		// No server running = no sessions
		if strings.Contains(string(out), "no server running") {
			return nil, nil
		}
		return nil, fmt.Errorf("tmux list-sessions: %w: %s", err, out)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var sessions []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			sessions = append(sessions, l)
		}
	}
	return sessions, nil
}
