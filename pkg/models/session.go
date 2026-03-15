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

type DirectorySourceType string

const (
	DirectorySourceExisting  DirectorySourceType = "existing"
	DirectorySourceNewFolder DirectorySourceType = "new_folder"
	DirectorySourceCloneRepo DirectorySourceType = "clone_repo"
	DirectorySourceNewRepo   DirectorySourceType = "new_repo"
)

// DirectorySource describes how to set up the working directory for a session
type DirectorySource struct {
	Type             DirectorySourceType `json:"type"`
	ExistingPath     string              `json:"existing_path,omitempty"`
	NewFolderName    string              `json:"new_folder_name,omitempty"`
	RepoURL          string              `json:"repo_url,omitempty"`
	RepoName         string              `json:"repo_name,omitempty"`
	RepoDescription  string              `json:"repo_description,omitempty"`
	RepoPrivate      bool                `json:"repo_private,omitempty"`
	CloneFolderName  string              `json:"clone_folder_name,omitempty"`
}

type Session struct {
	ID               string            `json:"id" db:"id"`
	Name             string            `json:"name" db:"name"`
	HostID           string            `json:"host_id" db:"host_id"`
	TemplateID       string            `json:"template_id,omitempty" db:"template_id"`
	Type             SessionType       `json:"type" db:"type"`
	AgentType        AgentType         `json:"agent_type,omitempty" db:"agent_type"`
	Status           SessionStatus     `json:"status" db:"status"`
	Prompt           string            `json:"prompt,omitempty" db:"prompt"`
	BranchName       string            `json:"branch_name,omitempty" db:"branch_name"`
	WorktreePath     string            `json:"worktree_path,omitempty" db:"worktree_path"`
	WorkingDirectory string            `json:"working_directory,omitempty" db:"working_directory"` // Custom working directory (alternative to worktree)
	TmuxSessionName  string            `json:"tmux_session" db:"tmux_session"`
	AgentPID         int               `json:"agent_pid" db:"agent_pid"`
	ModelOverride    string            `json:"model_override,omitempty" db:"model_override"`
	EnvVars          map[string]string `json:"env_vars"`
	MCPServers       []MCPServerConfig `json:"mcp_servers"`
	Plugins          []string          `json:"plugins"`
	AllowedTools     []string          `json:"allowed_tools"`
	ExtraFlags       []string          `json:"extra_flags"`
	LastOutput       string            `json:"last_output,omitempty" db:"last_output"`
	StartedAt        *time.Time        `json:"started_at,omitempty" db:"started_at"`
	StoppedAt        *time.Time        `json:"stopped_at,omitempty" db:"stopped_at"`
	CreatedAt        time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at" db:"updated_at"`
}
