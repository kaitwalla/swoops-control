package sessionmgr

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/swoopsh/swoops/pkg/models"
	"github.com/swoopsh/swoops/pkg/sshexec"
	"github.com/swoopsh/swoops/pkg/tmux"
	"github.com/swoopsh/swoops/pkg/worktree"
	"github.com/swoopsh/swoops/server/internal/store"
)

// Manager orchestrates session lifecycle operations on remote hosts via SSH.
type Manager struct {
	store *store.Store

	mu      sync.Mutex
	clients map[string]*sshexec.Client // host ID -> SSH client
	outputs map[string]*OutputStreamer  // session ID -> output streamer
}

// New creates a new session manager.
func New(s *store.Store) *Manager {
	return &Manager{
		store:   s,
		clients: make(map[string]*sshexec.Client),
		outputs: make(map[string]*OutputStreamer),
	}
}

// getSSHClient returns a cached or new SSH client for the given host.
func (m *Manager) getSSHClient(host *models.Host) *sshexec.Client {
	m.mu.Lock()
	defer m.mu.Unlock()

	if client, ok := m.clients[host.ID]; ok {
		return client
	}

	client := sshexec.NewClient(host.Hostname, host.SSHPort, host.SSHUser, host.SSHKeyPath)
	m.clients[host.ID] = client
	return client
}

// tmuxName returns the tmux session name for a swoops session.
func tmuxName(sessionID string) string {
	return "swoop-" + sessionID[:12]
}

// LaunchSession creates a worktree, starts a tmux session, and launches the AI agent on the remote host.
// It re-reads the session from the store to avoid aliasing with the caller's copy.
func (m *Manager) LaunchSession(sessionID, hostID string) error {
	sess, err := m.store.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}
	host, err := m.store.GetHost(hostID)
	if err != nil {
		return fmt.Errorf("get host: %w", err)
	}

	client := m.getSSHClient(host)
	execFn := client.ExecFunc()

	tmuxRunner := &tmux.Runner{ExecFunc: execFn}
	wtManager := &worktree.Manager{ExecFunc: execFn}

	// Compute paths
	worktreePath := filepath.Join(host.WorktreeRoot, sess.Name)
	tmuxSession := tmuxName(sess.ID)

	// Update session to starting
	sess.Status = models.SessionStatusStarting
	sess.WorktreePath = worktreePath
	sess.TmuxSessionName = tmuxSession
	now := time.Now()
	sess.StartedAt = &now
	if err := m.store.UpdateSession(sess); err != nil {
		return fmt.Errorf("update session to starting: %w", err)
	}

	// 1. Ensure worktree root exists
	if _, err := client.Exec(fmt.Sprintf("mkdir -p %s", shellQuote(host.WorktreeRoot))); err != nil {
		m.failSession(sess, fmt.Errorf("create worktree root: %w", err))
		return err
	}

	// 2. Create git worktree
	if err := wtManager.Create(host.BaseRepoPath, worktreePath, sess.BranchName); err != nil {
		m.failSession(sess, fmt.Errorf("create worktree: %w", err))
		return err
	}

	// 3. Create tmux session
	if err := tmuxRunner.CreateSession(tmuxSession, worktreePath); err != nil {
		// Clean up worktree on failure
		wtManager.Remove(host.BaseRepoPath, worktreePath)
		m.failSession(sess, fmt.Errorf("create tmux session: %w", err))
		return err
	}

	// 4. Build the agent command
	agentCmd := buildAgentCommand(sess)

	// 5. Launch agent in tmux
	if err := tmuxRunner.SendKeys(tmuxSession, agentCmd); err != nil {
		tmuxRunner.KillSession(tmuxSession)
		wtManager.Remove(host.BaseRepoPath, worktreePath)
		m.failSession(sess, fmt.Errorf("launch agent: %w", err))
		return err
	}

	// 6. Update session to running
	sess.Status = models.SessionStatusRunning
	if err := m.store.UpdateSession(sess); err != nil {
		return fmt.Errorf("update session to running: %w", err)
	}

	// 7. Start output streamer
	streamer := NewOutputStreamer(sess.ID, tmuxSession, tmuxRunner, m.store)
	m.mu.Lock()
	m.outputs[sess.ID] = streamer
	m.mu.Unlock()
	streamer.Start()

	log.Printf("session %s launched on host %s (tmux: %s, worktree: %s)", sess.ID, host.Name, tmuxSession, worktreePath)
	return nil
}

// StopSession stops a running session: kills tmux, removes worktree.
func (m *Manager) StopSession(sess *models.Session, host *models.Host) error {
	client := m.getSSHClient(host)
	execFn := client.ExecFunc()

	tmuxRunner := &tmux.Runner{ExecFunc: execFn}
	wtManager := &worktree.Manager{ExecFunc: execFn}

	// Stop output streamer
	m.mu.Lock()
	if streamer, ok := m.outputs[sess.ID]; ok {
		streamer.Stop()
		delete(m.outputs, sess.ID)
	}
	m.mu.Unlock()

	// Update status to stopping
	sess.Status = models.SessionStatusStopping
	m.store.UpdateSession(sess)

	// Kill tmux session
	if sess.TmuxSessionName != "" {
		if err := tmuxRunner.KillSession(sess.TmuxSessionName); err != nil {
			log.Printf("warn: kill tmux session %s: %v", sess.TmuxSessionName, err)
		}
	}

	// Remove worktree
	if sess.WorktreePath != "" {
		if err := wtManager.Remove(host.BaseRepoPath, sess.WorktreePath); err != nil {
			log.Printf("warn: remove worktree %s: %v", sess.WorktreePath, err)
		}
	}

	// Update session to stopped
	now := time.Now()
	sess.Status = models.SessionStatusStopped
	sess.StoppedAt = &now
	if err := m.store.UpdateSession(sess); err != nil {
		return fmt.Errorf("update session to stopped: %w", err)
	}

	log.Printf("session %s stopped", sess.ID)
	return nil
}

// SendInput sends text to a running session's tmux pane.
func (m *Manager) SendInput(sess *models.Session, host *models.Host, input string) error {
	client := m.getSSHClient(host)
	execFn := client.ExecFunc()
	tmuxRunner := &tmux.Runner{ExecFunc: execFn}

	if sess.TmuxSessionName == "" {
		return fmt.Errorf("session has no tmux session")
	}

	return tmuxRunner.SendKeys(sess.TmuxSessionName, input)
}

// GetOutput captures the current output from a session's tmux pane.
func (m *Manager) GetOutput(sess *models.Session, host *models.Host) (string, error) {
	client := m.getSSHClient(host)
	execFn := client.ExecFunc()
	tmuxRunner := &tmux.Runner{ExecFunc: execFn}

	if sess.TmuxSessionName == "" {
		return sess.LastOutput, nil
	}

	output, err := tmuxRunner.CapturePane(sess.TmuxSessionName, 500)
	if err != nil {
		// If tmux session is gone, return last stored output
		return sess.LastOutput, nil
	}

	return output, nil
}

// GetOutputStreamer returns the output streamer for a session, if active.
func (m *Manager) GetOutputStreamer(sessionID string) *OutputStreamer {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.outputs[sessionID]
}

// Close cleans up all SSH connections and stops all streamers.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, streamer := range m.outputs {
		streamer.Stop()
	}

	for _, client := range m.clients {
		client.Close()
	}
}

func (m *Manager) failSession(sess *models.Session, err error) {
	log.Printf("session %s failed: %v", sess.ID, err)
	now := time.Now()
	sess.Status = models.SessionStatusFailed
	sess.StoppedAt = &now
	sess.LastOutput = fmt.Sprintf("Error: %v", err)
	m.store.UpdateSession(sess)
}

// buildAgentCommand constructs the CLI command to launch the AI agent.
func buildAgentCommand(sess *models.Session) string {
	var parts []string

	// Set environment variables
	for k, v := range sess.EnvVars {
		parts = append(parts, fmt.Sprintf("export %s=%s;", k, shellQuote(v)))
	}

	switch sess.AgentType {
	case models.AgentTypeClaude:
		cmd := "claude"
		if sess.ModelOverride != "" {
			cmd += " --model " + shellQuote(sess.ModelOverride)
		}
		for _, tool := range sess.AllowedTools {
			cmd += " --allowedTools " + shellQuote(tool)
		}
		for _, flag := range sess.ExtraFlags {
			cmd += " " + flag
		}
		// Pass prompt via --print for non-interactive, or -p for dangling prompt
		cmd += " --print " + shellQuote(sess.Prompt)
		parts = append(parts, cmd)

	case models.AgentTypeCodex:
		cmd := "codex"
		if sess.ModelOverride != "" {
			cmd += " --model " + shellQuote(sess.ModelOverride)
		}
		for _, flag := range sess.ExtraFlags {
			cmd += " " + flag
		}
		cmd += " " + shellQuote(sess.Prompt)
		parts = append(parts, cmd)
	}

	return strings.Join(parts, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
