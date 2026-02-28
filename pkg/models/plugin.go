package models

import "time"

type PluginType string

const (
	PluginTypeMCPServer    PluginType = "mcp-server"
	PluginTypeCLITool      PluginType = "cli-tool"
	PluginTypeScriptBundle PluginType = "script-bundle"
)

type Plugin struct {
	ID          string            `json:"id" db:"id"`
	Name        string            `json:"name" db:"name"`
	GitURL      string            `json:"git_url" db:"git_url"`
	Description string            `json:"description" db:"description"`
	Version     string            `json:"version" db:"version"`
	Type        PluginType        `json:"type" db:"type"`
	InstallSpec PluginInstallSpec `json:"install_spec"`
	AgentTypes  []string          `json:"agent_types"`
	CreatedAt   time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at" db:"updated_at"`
}

type PluginInstallSpec struct {
	BuildCommand string            `json:"build_command,omitempty" yaml:"build_command"`
	Binaries     []PluginBinary    `json:"binaries" yaml:"binaries"`
	MCPConfig    *MCPServerConfig  `json:"mcp_config,omitempty" yaml:"mcp_config"`
	EnvVars      []PluginEnvVar    `json:"env_vars,omitempty" yaml:"env_vars"`
	Dependencies []string          `json:"dependencies,omitempty" yaml:"dependencies"`
}

type PluginBinary struct {
	Name     string `json:"name" yaml:"name"`
	Path     string `json:"path" yaml:"path"`
	Platform string `json:"platform" yaml:"platform"`
}

type PluginEnvVar struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Required    bool   `json:"required" yaml:"required"`
}
