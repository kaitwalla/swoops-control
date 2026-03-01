package mcpconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/swoopsh/swoops/pkg/models"
)

// ClaudeCodeConfig represents the .mcp.json format for Claude Code
type ClaudeCodeConfig struct {
	MCPServers map[string]ClaudeCodeServer `json:"mcpServers"`
}

type ClaudeCodeServer struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// CodexConfig represents the MCP server configuration for Codex
// Note: Codex MCP config format may vary - this is a placeholder
type CodexConfig struct {
	MCPServers []CodexServer `toml:"mcp_servers"`
}

type CodexServer struct {
	Name    string            `toml:"name"`
	Command string            `toml:"command"`
	Args    []string          `toml:"args"`
	Env     map[string]string `toml:"env,omitempty"`
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
	return writeJSONFile(configPath, config)
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
	return writeJSONFile(configPath, config)
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
