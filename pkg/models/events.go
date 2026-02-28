package models

import "time"

type EventType string

const (
	EventHostStatusChanged    EventType = "host.status_changed"
	EventHostHeartbeat        EventType = "host.heartbeat"
	EventSessionCreated       EventType = "session.created"
	EventSessionStatusChanged EventType = "session.status_changed"
	EventSessionOutput        EventType = "session.output"
	EventSessionStopped       EventType = "session.stopped"
	EventPluginSyncCompleted  EventType = "plugin_sync.completed"
	EventToolInstalled        EventType = "tool.installed"
	EventError                EventType = "error"
)

type WSEvent struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Payload   interface{} `json:"payload"`
}
