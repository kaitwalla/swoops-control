package models

import "time"

type SessionTemplate struct {
	ID            string            `json:"id" db:"id"`
	Name          string            `json:"name" db:"name"`
	Description   string            `json:"description" db:"description"`
	AgentType     AgentType         `json:"agent_type" db:"agent_type"`
	ModelOverride string            `json:"model_override,omitempty" db:"model_override"`
	Plugins       []string          `json:"plugins"`
	MCPServers    []MCPServerConfig `json:"mcp_servers"`
	AllowedTools  []string          `json:"allowed_tools"`
	ExtraFlags    []string          `json:"extra_flags"`
	EnvVars       map[string]string `json:"env_vars"`
	DefaultPrompt string            `json:"default_prompt,omitempty" db:"default_prompt"`
	CreatedAt     time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at" db:"updated_at"`
}
