package models

import "time"

type HostStatus string

const (
	HostStatusOnline       HostStatus = "online"
	HostStatusOffline      HostStatus = "offline"
	HostStatusDegraded     HostStatus = "degraded"
	HostStatusProvisioning HostStatus = "provisioning"
)

type Host struct {
	ID                   string            `json:"id" db:"id"`
	Name                 string            `json:"name" db:"name"`
	Hostname             string            `json:"hostname" db:"hostname"`
	SSHPort              int               `json:"ssh_port" db:"ssh_port"`
	SSHUser              string            `json:"ssh_user" db:"ssh_user"`
	SSHKeyPath           string            `json:"ssh_key_path" db:"ssh_key_path"`
	OS                   string            `json:"os" db:"os"`
	Arch                 string            `json:"arch" db:"arch"`
	Status               HostStatus        `json:"status" db:"status"`
	AgentVersion         string            `json:"agent_version" db:"agent_version"`
	AgentAuthToken       string            `json:"-" db:"agent_auth_token"`        // Exclude from JSON for security
	CertDownloaded       bool              `json:"-" db:"cert_downloaded"`         // Track if client cert has been downloaded
	UpdateAvailable      bool              `json:"update_available" db:"update_available"`
	LatestVersion        string            `json:"latest_version,omitempty" db:"latest_version"`
	UpdateURL            string            `json:"update_url,omitempty" db:"update_url"`
	Labels               map[string]string `json:"labels"`
	MaxSessions          int               `json:"max_sessions" db:"max_sessions"`
	BaseRepoPath         string            `json:"base_repo_path" db:"base_repo_path"`
	WorktreeRoot         string            `json:"worktree_root" db:"worktree_root"`
	InstalledPlugins     []PluginRef       `json:"installed_plugins"`
	InstalledTools       []InstalledTool   `json:"installed_tools"`
	LastHeartbeat        *time.Time        `json:"last_heartbeat,omitempty" db:"last_heartbeat"`
	CreatedAt            time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time         `json:"updated_at" db:"updated_at"`
}

type PluginRef struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InstalledTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}
