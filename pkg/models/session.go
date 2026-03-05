package models

import "time"

type AgentType string

const (
	AgentTypeClaude AgentType = "claude"
	AgentTypeCodex  AgentType = "codex"
)

type SessionType string

const (
	SessionTypeAgent SessionType = "agent" // Agent-based session (claude/codex in a repo)
	SessionTypeShell SessionType = "shell" // Interactive shell session
)

type SessionStatus string

const (
	SessionStatusPending  SessionStatus = "pending"
	SessionStatusStarting SessionStatus = "starting"
	SessionStatusRunning  SessionStatus = "running"
	SessionStatusIdle     SessionStatus = "idle"
	SessionStatusStopping SessionStatus = "stopping"
	SessionStatusStopped  SessionStatus = "stopped"
	SessionStatusFailed   SessionStatus = "failed"
)

type Session struct {
	ID              string            `json:"id" db:"id"`
	Name            string            `json:"name" db:"name"`
	HostID          string            `json:"host_id" db:"host_id"`
	TemplateID      string            `json:"template_id,omitempty" db:"template_id"`
	Type            SessionType       `json:"type" db:"type"`
	AgentType       AgentType         `json:"agent_type,omitempty" db:"agent_type"`
	Status          SessionStatus     `json:"status" db:"status"`
	Prompt          string            `json:"prompt,omitempty" db:"prompt"`
	BranchName      string            `json:"branch_name,omitempty" db:"branch_name"`
	WorktreePath    string            `json:"worktree_path,omitempty" db:"worktree_path"`
	TmuxSessionName string            `json:"tmux_session" db:"tmux_session"`
	AgentPID        int               `json:"agent_pid" db:"agent_pid"`
	ModelOverride   string            `json:"model_override,omitempty" db:"model_override"`
	EnvVars         map[string]string `json:"env_vars"`
	MCPServers      []MCPServerConfig `json:"mcp_servers"`
	Plugins         []string          `json:"plugins"`
	AllowedTools    []string          `json:"allowed_tools"`
	ExtraFlags      []string          `json:"extra_flags"`
	LastOutput      string            `json:"last_output,omitempty" db:"last_output"`
	StartedAt       *time.Time        `json:"started_at,omitempty" db:"started_at"`
	StoppedAt       *time.Time        `json:"stopped_at,omitempty" db:"stopped_at"`
	CreatedAt       time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at" db:"updated_at"`
}
