package agentconn

import "time"

const (
	// Channel buffer sizes
	commandChannelBuffer  = 64 // Allows burst of commands before blocking
	outputSubChannelBuffer = 16 // Buffer for output subscription channels

	// Timeouts
	commandResultTimeout = 10 * time.Second // Max wait for agent command acknowledgement
	commandQueueTimeout  = 2 * time.Second  // Max wait to queue command to agent

	// Heartbeat monitoring
	defaultCheckInterval = 5 * time.Second  // How often to check heartbeat status
	defaultDegradedAfter = 20 * time.Second // Mark host degraded after this long without heartbeat
	defaultOfflineAfter  = 40 * time.Second // Mark host offline after this long without heartbeat

	// Validation
	maxHostIDLength       = 255
	maxAuthTokenLength    = 255
	maxAgentVersionLength = 100
)