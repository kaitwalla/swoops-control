package mcpconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kaitwalla/swoops-control/pkg/models"
)

// ClaudeCodeConfig represents the .mcp.json format for Claude Code
type ClaudeCodeConfig struct {
	MCPServers map[string]ClaudeCodeServer `json:"mcpServers"`
}

type ClaudeCodeServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// ClaudeCodeSettings represents the .claude/settings.json format
type ClaudeCodeSettings struct {
	Schema      string                        `json:"$schema,omitempty"`
	Attribution ClaudeCodeAttribution         `json:"attribution"`
	Permissions ClaudeCodePermissions         `json:"permissions"`
	Env         map[string]string             `json:"env,omitempty"`
}

type ClaudeCodeAttribution struct {
	Commit string `json:"commit"`
	PR     string `json:"pr"`
}

type ClaudeCodePermissions struct {
	Allow   []string `json:"allow,omitempty"`
	Deny    []string `json:"deny,omitempty"`
	Ask     []string `json:"ask,omitempty"`
	DefaultMode string `json:"defaultMode,omitempty"`
}

// CodexConfig represents the MCP server configuration for Codex
// Note: Codex uses TOML format for configuration
type CodexConfig struct {
	MCPServers []CodexServer `json:"mcp_servers"` // Codex may use JSON or TOML
}

type CodexServer struct {
	Name    string            `json:"name"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// CodexSettings represents the .codex/config.toml-like settings
// Note: Exact format may vary based on Codex version
type CodexSettings struct {
	Git         CodexGitSettings  `json:"git,omitempty"`
	Permissions CodexPermissions  `json:"permissions,omitempty"`
}

type CodexGitSettings struct {
	// Attribution settings - exact format TBD based on Codex implementation
	CommandAttribution string `json:"command_attribution,omitempty"` // "default", "custom", or "disable"
	CoAuthoredBy      string `json:"co_authored_by,omitempty"`      // Custom co-author string if needed
}

type CodexPermissions struct {
	AllowGitPush bool `json:"allow_git_push,omitempty"`
}

// GenerateClaudeCodeConfig creates a .mcp.json file for Claude Code
func GenerateClaudeCodeConfig(worktreePath, sessionID, serverAddr, apiKey string) error {
	config := ClaudeCodeConfig{
		MCPServers: map[string]ClaudeCodeServer{
			"swoops-orchestrator": {
				Command: "swoops-agent",
				Args: []string{
					"mcp-serve",
					"--session-id", sessionID,
					"--server", serverAddr,
				},
				Env: map[string]string{
					"SWOOPS_API_KEY": apiKey,
				},
			},
		},
	}

	configPath := filepath.Join(worktreePath, ".mcp.json")
	if err := writeJSONFile(configPath, config); err != nil {
		return err
	}

	// Generate settings.json if it doesn't exist
	return generateClaudeCodeSettingsIfNotExists(worktreePath)
}

// generateClaudeCodeSettingsIfNotExists creates .claude/settings.json with sensible defaults
// Only creates the file if it doesn't already exist
func generateClaudeCodeSettingsIfNotExists(worktreePath string) error {
	claudeDir := filepath.Join(worktreePath, ".claude")
	settingsPath := filepath.Join(claudeDir, "settings.json")

	// Check if settings.json already exists
	if _, err := os.Stat(settingsPath); err == nil {
		// File exists, don't overwrite
		return nil
	}

	// Create .claude directory if it doesn't exist
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude directory: %w", err)
	}

	// Create default settings
	settings := ClaudeCodeSettings{
		Schema: "https://json.schemastore.org/claude-code-settings.json",
		Attribution: ClaudeCodeAttribution{
			Commit: "", // No attribution per user preference
			PR:     "", // No attribution per user preference
		},
		Permissions: ClaudeCodePermissions{
			Allow: []string{
				"Bash(git push *)", // Allow git push without asking
			},
			DefaultMode: "acceptEdits",
		},
	}

	return writeJSONFile(settingsPath, settings)
}

// GenerateCodexConfig creates MCP configuration for Codex
// Note: The exact format depends on Codex's MCP implementation
func GenerateCodexConfig(worktreePath, sessionID, serverAddr, apiKey string) error {
	// Check if Codex uses .codex/config.toml or another format
	codexDir := filepath.Join(worktreePath, ".codex")
	if err := os.MkdirAll(codexDir, 0o755); err != nil {
		return fmt.Errorf("create .codex directory: %w", err)
	}

	// For now, generate a JSON-based config similar to Claude Code
	// This may need to be updated based on Codex's actual MCP support
	config := ClaudeCodeConfig{
		MCPServers: map[string]ClaudeCodeServer{
			"swoops-orchestrator": {
				Command: "swoops-agent",
				Args: []string{
					"mcp-serve",
					"--session-id", sessionID,
					"--server", serverAddr,
				},
				Env: map[string]string{
					"SWOOPS_API_KEY": apiKey,
				},
			},
		},
	}

	configPath := filepath.Join(codexDir, "mcp.json")
	if err := writeJSONFile(configPath, config); err != nil {
		return err
	}

	// Generate settings if they don't exist
	return generateCodexSettingsIfNotExists(worktreePath)
}

// generateCodexSettingsIfNotExists creates .codex/settings.json with sensible defaults
// Note: This is a best-effort configuration - exact format may need updates for Codex
func generateCodexSettingsIfNotExists(worktreePath string) error {
	codexDir := filepath.Join(worktreePath, ".codex")
	settingsPath := filepath.Join(codexDir, "settings.json")

	// Check if settings already exist
	if _, err := os.Stat(settingsPath); err == nil {
		// File exists, don't overwrite
		return nil
	}

	// Create default settings based on user preferences
	settings := CodexSettings{
		Git: CodexGitSettings{
			CommandAttribution: "disable", // No attribution per user preference
		},
		Permissions: CodexPermissions{
			AllowGitPush: true, // Allow git push without asking per user preference
		},
	}

	return writeJSONFile(settingsPath, settings)
}

// GenerateMCPConfigForSession generates the appropriate MCP config based on agent type
func GenerateMCPConfigForSession(agentType models.AgentType, worktreePath, sessionID, serverAddr, apiKey string) error {
	switch agentType {
	case models.AgentTypeClaude:
		return GenerateClaudeCodeConfig(worktreePath, sessionID, serverAddr, apiKey)
	case models.AgentTypeCodex:
		return GenerateCodexConfig(worktreePath, sessionID, serverAddr, apiKey)
	default:
		return fmt.Errorf("unsupported agent type: %s", agentType)
	}
}

func writeJSONFile(path string, data interface{}) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}

	return nil
}
