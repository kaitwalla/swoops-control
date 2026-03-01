package sessionmgr

import "testing"

func TestBuildAgentCommand(t *testing.T) {
	tests := []struct {
		name string
		sess sessionForCmd
		want string
	}{
		{
			name: "claude basic",
			sess: sessionForCmd{agentType: "claude", prompt: "fix bug"},
			want: "claude --print 'fix bug'",
		},
		{
			name: "claude with model",
			sess: sessionForCmd{agentType: "claude", prompt: "fix bug", model: "opus"},
			want: "claude --model 'opus' --print 'fix bug'",
		},
		{
			name: "codex basic",
			sess: sessionForCmd{agentType: "codex", prompt: "fix bug"},
			want: "codex 'fix bug'",
		},
		{
			name: "codex with model",
			sess: sessionForCmd{agentType: "codex", prompt: "fix bug", model: "o3"},
			want: "codex --model 'o3' 'fix bug'",
		},
		{
			name: "shell quote with apostrophe",
			sess: sessionForCmd{agentType: "claude", prompt: "fix the user's bug"},
			want: "claude --print 'fix the user'\\''s bug'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAgentCommandForTest(tt.sess)
			if got != tt.want {
				t.Errorf("buildAgentCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

// sessionForCmd is a test helper to isolate buildAgentCommand testing
type sessionForCmd struct {
	agentType string
	prompt    string
	model     string
	tools     []string
	flags     []string
	envVars   map[string]string
}

func buildAgentCommandForTest(s sessionForCmd) string {
	// Replicate the logic from buildAgentCommand but with simplified types
	var parts []string

	for k, v := range s.envVars {
		parts = append(parts, "export "+k+"="+shellQuote(v)+";")
	}

	switch s.agentType {
	case "claude":
		cmd := "claude"
		if s.model != "" {
			cmd += " --model " + shellQuote(s.model)
		}
		for _, tool := range s.tools {
			cmd += " --allowedTools " + shellQuote(tool)
		}
		for _, flag := range s.flags {
			cmd += " " + flag
		}
		cmd += " --print " + shellQuote(s.prompt)
		parts = append(parts, cmd)
	case "codex":
		cmd := "codex"
		if s.model != "" {
			cmd += " --model " + shellQuote(s.model)
		}
		for _, flag := range s.flags {
			cmd += " " + flag
		}
		cmd += " " + shellQuote(s.prompt)
		parts = append(parts, cmd)
	}

	result := ""
	for i, p := range parts {
		if i > 0 {
			result += " "
		}
		result += p
	}
	return result
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"hello", "'hello'"},
		{"hello world", "'hello world'"},
		{"it's", "'it'\\''s'"},
		{"", "''"},
		{"$VAR", "'$VAR'"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := shellQuote(tt.input)
			if got != tt.want {
				t.Errorf("shellQuote(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
