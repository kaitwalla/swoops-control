package models

import "time"

// AgentStatusUpdate represents a status report from an AI agent running in a session
type AgentStatusUpdate struct {
	ID        string                 `json:"id"`
	SessionID string                 `json:"session_id"`
	Type      AgentStatusType        `json:"status_type"`
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

type AgentStatusType string

const (
	StatusWorking   AgentStatusType = "working"
	StatusIdle      AgentStatusType = "idle"
	StatusBlocked   AgentStatusType = "blocked"
	StatusCompleted AgentStatusType = "completed"
	StatusError     AgentStatusType = "error"
)

// SessionTask represents a task that can be assigned to a session
type SessionTask struct {
	ID          string                 `json:"id"`
	SessionID   string                 `json:"session_id"`
	Type        TaskType               `json:"task_type"`
	Priority    int                    `json:"priority"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Context     map[string]interface{} `json:"context,omitempty"`
	Status      TaskStatus             `json:"status"`
	RetrievedAt *time.Time             `json:"retrieved_at,omitempty"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type TaskType string

const (
	TaskInstruction TaskType = "instruction"
	TaskFix         TaskType = "fix"
	TaskReview      TaskType = "review"
	TaskRefactor    TaskType = "refactor"
	TaskTest        TaskType = "test"
)

type TaskStatus string

const (
	TaskPending   TaskStatus = "pending"
	TaskRetrieved TaskStatus = "retrieved"
	TaskCompleted TaskStatus = "completed"
	TaskFailed    TaskStatus = "failed"
)

// ReviewRequest represents a code review request from an AI agent
type ReviewRequest struct {
	ID            string             `json:"id"`
	SessionID     string             `json:"session_id"`
	Type          ReviewType         `json:"request_type"`
	Title         string             `json:"title"`
	Description   string             `json:"description"`
	FilePaths     []string           `json:"file_paths,omitempty"`
	Diff          string             `json:"diff,omitempty"`
	Status        ReviewStatus       `json:"status"`
	ReviewerNotes string             `json:"reviewer_notes,omitempty"`
	ReviewedAt    *time.Time         `json:"reviewed_at,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
}

type ReviewType string

const (
	ReviewCode         ReviewType = "code"
	ReviewArchitecture ReviewType = "architecture"
	ReviewSecurity     ReviewType = "security"
	ReviewPerformance  ReviewType = "performance"
)

type ReviewStatus string

const (
	ReviewPending          ReviewStatus = "pending"
	ReviewInReview         ReviewStatus = "in_review"
	ReviewApproved         ReviewStatus = "approved"
	ReviewChangesRequested ReviewStatus = "changes_requested"
	ReviewRejected         ReviewStatus = "rejected"
)

// SessionMessage represents a message for session-to-session coordination
type SessionMessage struct {
	ID            string                 `json:"id"`
	FromSessionID string                 `json:"from_session_id"`
	ToSessionID   string                 `json:"to_session_id"`
	Type          MessageType            `json:"message_type"`
	Subject       string                 `json:"subject"`
	Body          string                 `json:"body"`
	Context       map[string]interface{} `json:"context,omitempty"`
	Status        MessageStatus          `json:"status"`
	ReadAt        *time.Time             `json:"read_at,omitempty"`
	CreatedAt     time.Time              `json:"created_at"`
}

type MessageType string

const (
	MessageQuestion MessageType = "question"
	MessageInfo     MessageType = "info"
	MessageRequest  MessageType = "request"
	MessageResponse MessageType = "response"
)

type MessageStatus string

const (
	MessageSent      MessageStatus = "sent"
	MessageRead      MessageStatus = "read"
	MessageResponded MessageStatus = "responded"
)

// Validation functions for enum types

func IsValidAgentStatusType(s string) bool {
	switch AgentStatusType(s) {
	case StatusWorking, StatusIdle, StatusBlocked, StatusCompleted, StatusError:
		return true
	}
	return false
}

func IsValidTaskType(s string) bool {
	switch TaskType(s) {
	case TaskInstruction, TaskFix, TaskReview, TaskRefactor, TaskTest:
		return true
	}
	return false
}

func IsValidTaskStatus(s string) bool {
	switch TaskStatus(s) {
	case TaskPending, TaskRetrieved, TaskCompleted, TaskFailed:
		return true
	}
	return false
}

func IsValidReviewType(s string) bool {
	switch ReviewType(s) {
	case ReviewCode, ReviewArchitecture, ReviewSecurity, ReviewPerformance:
		return true
	}
	return false
}

func IsValidReviewStatus(s string) bool {
	switch ReviewStatus(s) {
	case ReviewPending, ReviewInReview, ReviewApproved, ReviewChangesRequested, ReviewRejected:
		return true
	}
	return false
}

func IsValidMessageType(s string) bool {
	switch MessageType(s) {
	case MessageQuestion, MessageInfo, MessageRequest, MessageResponse:
		return true
	}
	return false
}

func IsValidMessageStatus(s string) bool {
	switch MessageStatus(s) {
	case MessageSent, MessageRead, MessageResponded:
		return true
	}
	return false
}
