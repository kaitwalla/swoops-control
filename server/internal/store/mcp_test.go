package store

import (
	"os"
	"testing"
	"time"

	"github.com/swoopsh/swoops/pkg/models"
)

func testMCPStore(t *testing.T) *Store {
	t.Helper()
	tmp, err := os.CreateTemp("", "swoops-mcp-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	s, err := New(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func createTestHost(t *testing.T, store *Store, hostID string) {
	t.Helper()
	host := &models.Host{
		ID:          hostID,
		Name:        "test-host-" + hostID,
		Hostname:    "localhost",
		SSHPort:     22,
		SSHUser:     "test",
		SSHKeyPath:  "/tmp/test.key",
		OS:          "linux",
		Arch:        "amd64",
		Status:      models.HostStatusOnline,
		MaxSessions: 10,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := store.CreateHost(host); err != nil {
		t.Fatalf("CreateHost failed: %v", err)
	}
}

func createTestSession(t *testing.T, store *Store, sessionID string) {
	t.Helper()
	session := &models.Session{
		ID:         sessionID,
		Name:       "test-session-" + sessionID,
		HostID:     "host-1",
		AgentType:  models.AgentTypeClaude,
		Status:     models.SessionStatusRunning,
		Prompt:     "Test prompt",
		BranchName: "main",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if err := store.CreateSession(session); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
}

func TestAgentStatusUpdates(t *testing.T) {
	store := testMCPStore(t)
	createTestHost(t, store, "host-1")
	createTestSession(t, store, "sess-1")

	update := &models.AgentStatusUpdate{
		ID:        models.NewID(),
		SessionID: "sess-1",
		Type:      models.StatusWorking,
		Message:   "Implementing feature X",
		Details: map[string]interface{}{
			"file": "main.go",
			"line": 42,
		},
		CreatedAt: time.Now(),
	}

	if err := store.CreateAgentStatusUpdate(update); err != nil {
		t.Fatalf("CreateAgentStatusUpdate failed: %v", err)
	}

	updates, err := store.ListAgentStatusUpdates("sess-1", 50)
	if err != nil {
		t.Fatalf("ListAgentStatusUpdates failed: %v", err)
	}

	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}

	if updates[0].Message != "Implementing feature X" {
		t.Errorf("message mismatch: got %q", updates[0].Message)
	}
}

func TestSessionTasks(t *testing.T) {
	store := testMCPStore(t)
	createTestHost(t, store, "host-1")
	createTestSession(t, store, "sess-1")

	task := &models.SessionTask{
		ID:          models.NewID(),
		SessionID:   "sess-1",
		Type:        models.TaskFix,
		Priority:    10,
		Title:       "Fix memory leak",
		Description: "Investigate and fix memory leak in handler",
		Context: map[string]interface{}{
			"component": "http_server",
		},
		Status:    models.TaskPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := store.CreateSessionTask(task); err != nil {
		t.Fatalf("CreateSessionTask failed: %v", err)
	}

	// Get next task
	nextTask, err := store.GetNextTask("sess-1")
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}

	if nextTask == nil {
		t.Fatal("expected task, got nil")
	}

	if nextTask.Title != "Fix memory leak" {
		t.Errorf("title mismatch: got %q", nextTask.Title)
	}

	// Update task status
	if err := store.UpdateTaskStatus(task.ID, models.TaskRetrieved); err != nil {
		t.Fatalf("UpdateTaskStatus failed: %v", err)
	}

	// List all tasks
	tasks, err := store.ListSessionTasks("sess-1")
	if err != nil {
		t.Fatalf("ListSessionTasks failed: %v", err)
	}

	if len(tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(tasks))
	}

	if tasks[0].Status != models.TaskRetrieved {
		t.Errorf("status mismatch: got %q", tasks[0].Status)
	}
}

func TestReviewRequests(t *testing.T) {
	store := testMCPStore(t)
	createTestHost(t, store, "host-1")
	createTestSession(t, store, "sess-1")

	review := &models.ReviewRequest{
		ID:          models.NewID(),
		SessionID:   "sess-1",
		Type:        models.ReviewSecurity,
		Title:       "Review authentication flow",
		Description: "Please review the new OAuth implementation",
		FilePaths:   []string{"auth/oauth.go", "auth/middleware.go"},
		Diff:        "diff --git a/auth/oauth.go ...",
		Status:      models.ReviewPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := store.CreateReviewRequest(review); err != nil {
		t.Fatalf("CreateReviewRequest failed: %v", err)
	}

	// Get review
	retrieved, err := store.GetReviewRequest(review.ID)
	if err != nil {
		t.Fatalf("GetReviewRequest failed: %v", err)
	}

	if retrieved.Title != "Review authentication flow" {
		t.Errorf("title mismatch: got %q", retrieved.Title)
	}

	// Update review
	if err := store.UpdateReviewRequest(review.ID, models.ReviewApproved, "LGTM!"); err != nil {
		t.Fatalf("UpdateReviewRequest failed: %v", err)
	}

	// List reviews
	reviews, err := store.ListReviewRequests("sess-1")
	if err != nil {
		t.Fatalf("ListReviewRequests failed: %v", err)
	}

	if len(reviews) != 1 {
		t.Fatalf("expected 1 review, got %d", len(reviews))
	}

	if reviews[0].Status != models.ReviewApproved {
		t.Errorf("status mismatch: got %q", reviews[0].Status)
	}

	if reviews[0].ReviewerNotes != "LGTM!" {
		t.Errorf("notes mismatch: got %q", reviews[0].ReviewerNotes)
	}
}

func TestSessionMessages(t *testing.T) {
	store := testMCPStore(t)
	createTestHost(t, store, "host-1")
	createTestSession(t, store, "sess-1")
	createTestSession(t, store, "sess-2")

	msg := &models.SessionMessage{
		ID:            models.NewID(),
		FromSessionID: "sess-1",
		ToSessionID:   "sess-2",
		Type:          models.MessageQuestion,
		Subject:       "Need help with API design",
		Body:          "How should we structure the REST endpoints?",
		Context: map[string]interface{}{
			"component": "api_server",
		},
		Status:    models.MessageSent,
		CreatedAt: time.Now(),
	}

	if err := store.CreateSessionMessage(msg); err != nil {
		t.Fatalf("CreateSessionMessage failed: %v", err)
	}

	// Get messages for session-2 (recipient)
	messages, err := store.GetSessionMessages("sess-2")
	if err != nil {
		t.Fatalf("GetSessionMessages failed: %v", err)
	}

	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Subject != "Need help with API design" {
		t.Errorf("subject mismatch: got %q", messages[0].Subject)
	}

	// Try to mark as read with wrong session (should fail)
	if err := store.MarkMessageRead(msg.ID, "sess-1"); err == nil {
		t.Error("expected error when marking message with wrong session, got nil")
	}

	// Mark as read (by the recipient session sess-2)
	if err := store.MarkMessageRead(msg.ID, "sess-2"); err != nil {
		t.Fatalf("MarkMessageRead failed: %v", err)
	}

	// Verify status updated
	messages, _ = store.GetSessionMessages("sess-2")
	if messages[0].Status != models.MessageRead {
		t.Errorf("status mismatch: got %q", messages[0].Status)
	}

	// Try to mark as read again (should fail - already read)
	if err := store.MarkMessageRead(msg.ID, "sess-2"); err == nil {
		t.Error("expected error when marking already read message, got nil")
	}
}

func TestTaskPriority(t *testing.T) {
	store := testMCPStore(t)
	createTestHost(t, store, "host-1")
	createTestSession(t, store, "sess-1")

	// Create tasks with different priorities
	lowPriority := &models.SessionTask{
		ID:          models.NewID(),
		SessionID:   "sess-1",
		Type:        models.TaskInstruction,
		Priority:    1,
		Title:       "Low priority task",
		Description: "Do this later",
		Status:      models.TaskPending,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	highPriority := &models.SessionTask{
		ID:          models.NewID(),
		SessionID:   "sess-1",
		Type:        models.TaskFix,
		Priority:    100,
		Title:       "High priority task",
		Description: "Do this first",
		Status:      models.TaskPending,
		CreatedAt:   time.Now().Add(time.Second), // Created later
		UpdatedAt:   time.Now().Add(time.Second),
	}

	store.CreateSessionTask(lowPriority)
	store.CreateSessionTask(highPriority)

	// Should get high priority task first
	next, err := store.GetNextTask("sess-1")
	if err != nil {
		t.Fatalf("GetNextTask failed: %v", err)
	}

	if next.Title != "High priority task" {
		t.Errorf("expected high priority task, got %q", next.Title)
	}
}
