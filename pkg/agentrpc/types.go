package agentrpc

type AgentEnvelope struct {
	Hello         *AgentHello    `json:"hello,omitempty"`
	Heartbeat     *Heartbeat     `json:"heartbeat,omitempty"`
	Output        *SessionOutput `json:"output,omitempty"`
	CommandResult *CommandResult `json:"command_result,omitempty"`
}

type AgentHello struct {
	HostID       string `json:"host_id,omitempty"`
	HostName     string `json:"host_name,omitempty"`
	AgentVersion string `json:"agent_version,omitempty"`
	OS           string `json:"os,omitempty"`
	Arch         string `json:"arch,omitempty"`
	AuthToken    string `json:"auth_token,omitempty"`
}

type Heartbeat struct {
	SentUnix         int64  `json:"sent_unix,omitempty"`
	RunningSessions  int32  `json:"running_sessions,omitempty"`
	UpdateAvailable  bool   `json:"update_available,omitempty"`
	CurrentVersion   string `json:"current_version,omitempty"`
	LatestVersion    string `json:"latest_version,omitempty"`
	UpdateURL        string `json:"update_url,omitempty"`
}

type SessionOutput struct {
	SessionID string `json:"session_id,omitempty"`
	Content   string `json:"content,omitempty"`
	EOF       bool   `json:"eof,omitempty"`
}

type ControlEnvelope struct {
	Ack     *Ack            `json:"ack,omitempty"`
	Command *SessionCommand `json:"command,omitempty"`
}

type Ack struct {
	Message string `json:"message,omitempty"`
}

type SessionCommand struct {
	CommandID string            `json:"command_id,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Command   string            `json:"command,omitempty"`
	Args      map[string]string `json:"args,omitempty"`
}

type CommandResult struct {
	CommandID string `json:"command_id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Ok        bool   `json:"ok"`
	Message   string `json:"message,omitempty"`
}

const (
	CommandLaunch           = "launch_session"
	CommandStop             = "stop_session"
	CommandInput            = "send_input"
	CommandUpdateAgent      = "update_agent"
	CommandCheckForUpdates  = "check_for_updates"
)
