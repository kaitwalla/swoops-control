package mcpconfig

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaitwalla/swoops-control/pkg/models"
)

func TestGenerateClaudeCodeSettings(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Generate Claude Code config
	err := GenerateClaudeCodeConfig(tmpDir, "test-session", "localhost:9090", "test-api-key")
	if err != nil {
		t.Fatalf("GenerateClaudeCodeConfig failed: %v", err)
	}

	// Verify .mcp.json was created
	mcpPath := filepath.Join(tmpDir, ".mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		t.Errorf(".mcp.json was not created")
	}

	// Verify .claude/settings.json was created
	settingsPath := filepath.Join(tmpDir, ".claude", "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Errorf(".claude/settings.json was not created")
	}

	// Read and verify settings content
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings ClaudeCodeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Verify settings values
	if settings.Attribution.Commit != "" {
		t.Errorf("Expected empty commit attribution, got: %q", settings.Attribution.Commit)
	}
	if settings.Attribution.PR != "" {
		t.Errorf("Expected empty PR attribution, got: %q", settings.Attribution.PR)
	}
	if settings.Permissions.DefaultMode != "acceptEdits" {
		t.Errorf("Expected defaultMode 'acceptEdits', got: %q", settings.Permissions.DefaultMode)
	}
	if len(settings.Permissions.Allow) == 0 {
		t.Errorf("Expected at least one permission in allow list")
	}
}

func TestGenerateClaudeCodeSettingsDoesNotOverwrite(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Create .claude directory and settings.json with custom content
	claudeDir := filepath.Join(tmpDir, ".claude")
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		t.Fatalf("Failed to create .claude directory: %v", err)
	}

	customSettings := ClaudeCodeSettings{
		Attribution: ClaudeCodeAttribution{
			Commit: "Custom attribution",
			PR:     "Custom PR",
		},
	}
	data, _ := json.Marshal(customSettings)
	settingsPath := filepath.Join(claudeDir, "settings.json")
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		t.Fatalf("Failed to write custom settings: %v", err)
	}

	// Generate config (should not overwrite)
	err := GenerateClaudeCodeConfig(tmpDir, "test-session", "localhost:9090", "test-api-key")
	if err != nil {
		t.Fatalf("GenerateClaudeCodeConfig failed: %v", err)
	}

	// Verify custom settings were not overwritten
	data, err = os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings ClaudeCodeSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	if settings.Attribution.Commit != "Custom attribution" {
		t.Errorf("Custom settings were overwritten")
	}
}

func TestGenerateCodexSettings(t *testing.T) {
	// Create a temporary directory for testing
	tmpDir := t.TempDir()

	// Generate Codex config
	err := GenerateCodexConfig(tmpDir, "test-session", "localhost:9090", "test-api-key")
	if err != nil {
		t.Fatalf("GenerateCodexConfig failed: %v", err)
	}

	// Verify .codex/mcp.json was created
	mcpPath := filepath.Join(tmpDir, ".codex", "mcp.json")
	if _, err := os.Stat(mcpPath); os.IsNotExist(err) {
		t.Errorf(".codex/mcp.json was not created")
	}

	// Verify .codex/settings.json was created
	settingsPath := filepath.Join(tmpDir, ".codex", "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Errorf(".codex/settings.json was not created")
	}

	// Read and verify settings content
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings.json: %v", err)
	}

	var settings CodexSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("Failed to parse settings.json: %v", err)
	}

	// Verify settings values
	if settings.Git.CommandAttribution != "disable" {
		t.Errorf("Expected command_attribution 'disable', got: %q", settings.Git.CommandAttribution)
	}
	if !settings.Permissions.AllowGitPush {
		t.Errorf("Expected allow_git_push to be true")
	}
}

func TestGenerateMCPConfigForSession(t *testing.T) {
	tests := []struct {
		name      string
		agentType models.AgentType
		expectDir string
	}{
		{
			name:      "Claude Code agent",
			agentType: models.AgentTypeClaude,
			expectDir: ".claude",
		},
		{
			name:      "Codex agent",
			agentType: models.AgentTypeCodex,
			expectDir: ".codex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			err := GenerateMCPConfigForSession(tt.agentType, tmpDir, "test-session", "localhost:9090", "test-api-key")
			if err != nil {
				t.Fatalf("GenerateMCPConfigForSession failed: %v", err)
			}

			// Verify expected directory was created
			expectedDir := filepath.Join(tmpDir, tt.expectDir)
			if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
				t.Errorf("Expected directory %s was not created", tt.expectDir)
			}

			// Verify settings.json exists
			settingsPath := filepath.Join(expectedDir, "settings.json")
			if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
				t.Errorf("settings.json was not created in %s", tt.expectDir)
			}
		})
	}
}
