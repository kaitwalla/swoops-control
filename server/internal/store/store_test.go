package store

import (
	"os"
	"testing"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/models"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	tmp, err := os.CreateTemp("", "swoops-test-*.db")
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

func TestHostCRUD(t *testing.T) {
	s := testStore(t)

	// Create
	now := time.Now()
	h := &models.Host{
		ID:           models.NewID(),
		Name:         "test-host",
		Hostname:     "10.0.0.1",
		SSHPort:      22,
		SSHUser:      "deploy",
		SSHKeyPath:   "/tmp/key.pem",
		Status:       models.HostStatusOffline,
		Labels:       map[string]string{"env": "test"},
		MaxSessions:  5,
		BaseRepoPath: "/opt/swoops/repo",
		WorktreeRoot: "/opt/swoops/worktrees",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.CreateHost(h); err != nil {
		t.Fatalf("CreateHost: %v", err)
	}

	// Get
	got, err := s.GetHost(h.ID)
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}
	if got.Name != "test-host" {
		t.Errorf("got name %q, want %q", got.Name, "test-host")
	}
	if got.Labels["env"] != "test" {
		t.Errorf("got label env=%q, want %q", got.Labels["env"], "test")
	}

	// List
	hosts, err := s.ListHosts()
	if err != nil {
		t.Fatalf("ListHosts: %v", err)
	}
	if len(hosts) != 1 {
		t.Errorf("got %d hosts, want 1", len(hosts))
	}

	// Update
	h.Name = "updated-host"
	if err := s.UpdateHost(h); err != nil {
		t.Fatalf("UpdateHost: %v", err)
	}
	got, _ = s.GetHost(h.ID)
	if got.Name != "updated-host" {
		t.Errorf("got name %q after update, want %q", got.Name, "updated-host")
	}

	// Delete
	if err := s.DeleteHost(h.ID); err != nil {
		t.Fatalf("DeleteHost: %v", err)
	}
	hosts, _ = s.ListHosts()
	if len(hosts) != 0 {
		t.Errorf("got %d hosts after delete, want 0", len(hosts))
	}
}

func TestDeleteNonexistentHost(t *testing.T) {
	s := testStore(t)
	err := s.DeleteHost("nonexistent")
	if err != ErrNotFound {
		t.Errorf("got err %v, want ErrNotFound", err)
	}
}

func TestUpdateNonexistentHost(t *testing.T) {
	s := testStore(t)
	h := &models.Host{ID: "nonexistent", Name: "x", Hostname: "x", SSHUser: "x", SSHKeyPath: "x"}
	err := s.UpdateHost(h)
	if err != ErrNotFound {
		t.Errorf("got err %v, want ErrNotFound", err)
	}
}

func TestHostHeartbeatAndStatusUpdates(t *testing.T) {
	s := testStore(t)

	now := time.Now()
	h := &models.Host{
		ID:           models.NewID(),
		Name:         "hb-host",
		Hostname:     "10.0.0.5",
		SSHPort:      22,
		SSHUser:      "deploy",
		SSHKeyPath:   "/tmp/key.pem",
		Status:       models.HostStatusOffline,
		MaxSessions:  5,
		BaseRepoPath: "/opt/swoops/repo",
		WorktreeRoot: "/opt/swoops/worktrees",
		Labels:       map[string]string{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.CreateHost(h); err != nil {
		t.Fatalf("CreateHost: %v", err)
	}

	heartbeatAt := now.Add(3 * time.Second)
	if err := s.UpsertHostHeartbeat(h.ID, "v0.3.0", "linux", "arm64", "test-hostname", "swoops", heartbeatAt); err != nil {
		t.Fatalf("UpsertHostHeartbeat: %v", err)
	}
	got, err := s.GetHost(h.ID)
	if err != nil {
		t.Fatalf("GetHost: %v", err)
	}
	if got.Status != models.HostStatusOnline {
		t.Fatalf("status=%q want %q", got.Status, models.HostStatusOnline)
	}
	if got.AgentVersion != "v0.3.0" {
		t.Fatalf("agent_version=%q want v0.3.0", got.AgentVersion)
	}
	if got.LastHeartbeat == nil {
		t.Fatal("last_heartbeat should be set")
	}

	if err := s.UpdateHostStatus(h.ID, models.HostStatusDegraded); err != nil {
		t.Fatalf("UpdateHostStatus: %v", err)
	}
	got, _ = s.GetHost(h.ID)
	if got.Status != models.HostStatusDegraded {
		t.Fatalf("status=%q want %q", got.Status, models.HostStatusDegraded)
	}

	nextBeat := heartbeatAt.Add(5 * time.Second)
	if err := s.TouchHostHeartbeat(h.ID, nextBeat); err != nil {
		t.Fatalf("TouchHostHeartbeat: %v", err)
	}
	got, _ = s.GetHost(h.ID)
	if got.Status != models.HostStatusOnline {
		t.Fatalf("status=%q want %q", got.Status, models.HostStatusOnline)
	}
}

func TestSessionCRUD(t *testing.T) {
	s := testStore(t)

	// Create host first (foreign key)
	now := time.Now()
	h := &models.Host{
		ID: models.NewID(), Name: "h1", Hostname: "10.0.0.1",
		SSHPort: 22, SSHUser: "u", SSHKeyPath: "/k",
		Status: models.HostStatusOffline, MaxSessions: 10,
		BaseRepoPath: "/r", WorktreeRoot: "/w",
		Labels: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateHost(h); err != nil {
		t.Fatal(err)
	}

	// Create session
	sess := &models.Session{
		ID: models.NewID(), Name: "s1", HostID: h.ID,
		AgentType: models.AgentTypeClaude, Status: models.SessionStatusPending,
		Prompt: "fix bug", BranchName: "swoops/s1",
		EnvVars:   map[string]string{"KEY": "val"},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Get
	got, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.Prompt != "fix bug" {
		t.Errorf("got prompt %q, want %q", got.Prompt, "fix bug")
	}
	if got.EnvVars["KEY"] != "val" {
		t.Errorf("got env KEY=%q, want %q", got.EnvVars["KEY"], "val")
	}

	// List
	sessions, _ := s.ListSessions("", "")
	if len(sessions) != 1 {
		t.Errorf("got %d sessions, want 1", len(sessions))
	}

	// List with filter
	sessions, _ = s.ListSessions(h.ID, "pending")
	if len(sessions) != 1 {
		t.Errorf("got %d sessions with filter, want 1", len(sessions))
	}
	sessions, _ = s.ListSessions("", "running")
	if len(sessions) != 0 {
		t.Errorf("got %d sessions with status=running, want 0", len(sessions))
	}

	// Update status
	if err := s.UpdateSessionStatus(sess.ID, models.SessionStatusRunning); err != nil {
		t.Fatalf("UpdateSessionStatus: %v", err)
	}
	got, _ = s.GetSession(sess.ID)
	if got.Status != models.SessionStatusRunning {
		t.Errorf("got status %q, want %q", got.Status, models.SessionStatusRunning)
	}

	// Delete
	if err := s.DeleteSession(sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}
}

func TestDeleteNonexistentSession(t *testing.T) {
	s := testStore(t)
	err := s.DeleteSession("nonexistent")
	if err != ErrNotFound {
		t.Errorf("got err %v, want ErrNotFound", err)
	}
}

func TestUpdateStatusNonexistentSession(t *testing.T) {
	s := testStore(t)
	err := s.UpdateSessionStatus("nonexistent", models.SessionStatusStopped)
	if err != ErrNotFound {
		t.Errorf("got err %v, want ErrNotFound", err)
	}
}

func TestUpdateSession(t *testing.T) {
	s := testStore(t)

	// Create host first
	now := time.Now()
	h := &models.Host{
		ID: models.NewID(), Name: "h1", Hostname: "10.0.0.1",
		SSHPort: 22, SSHUser: "u", SSHKeyPath: "/k",
		Status: models.HostStatusOffline, MaxSessions: 10,
		BaseRepoPath: "/r", WorktreeRoot: "/w",
		Labels: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateHost(h); err != nil {
		t.Fatal(err)
	}

	// Create session
	sess := &models.Session{
		ID: models.NewID(), Name: "s1", HostID: h.ID,
		AgentType: models.AgentTypeClaude, Status: models.SessionStatusPending,
		Prompt: "fix bug", BranchName: "swoops/s1",
		EnvVars: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateSession(sess); err != nil {
		t.Fatal(err)
	}

	// Update session
	sess.Status = models.SessionStatusRunning
	sess.WorktreePath = "/w/s1"
	sess.TmuxSessionName = "swoop-abc123"
	sess.LastOutput = "hello world"
	startedAt := time.Now()
	sess.StartedAt = &startedAt
	if err := s.UpdateSession(sess); err != nil {
		t.Fatalf("UpdateSession: %v", err)
	}

	// Verify
	got, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.SessionStatusRunning {
		t.Errorf("status = %q, want running", got.Status)
	}
	if got.WorktreePath != "/w/s1" {
		t.Errorf("worktree_path = %q, want /w/s1", got.WorktreePath)
	}
	if got.TmuxSessionName != "swoop-abc123" {
		t.Errorf("tmux_session = %q, want swoop-abc123", got.TmuxSessionName)
	}
	if got.LastOutput != "hello world" {
		t.Errorf("last_output = %q, want 'hello world'", got.LastOutput)
	}
	if got.StartedAt == nil {
		t.Error("started_at should be set")
	}
}

func TestUpdateSessionOutput(t *testing.T) {
	s := testStore(t)

	// Create host + session
	now := time.Now()
	h := &models.Host{
		ID: models.NewID(), Name: "h2", Hostname: "10.0.0.2",
		SSHPort: 22, SSHUser: "u", SSHKeyPath: "/k",
		Status: models.HostStatusOffline, MaxSessions: 10,
		BaseRepoPath: "/r", WorktreeRoot: "/w",
		Labels: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateHost(h); err != nil {
		t.Fatal(err)
	}
	sess := &models.Session{
		ID: models.NewID(), Name: "s2", HostID: h.ID,
		AgentType: models.AgentTypeClaude, Status: models.SessionStatusRunning,
		Prompt: "fix bug", BranchName: "swoops/s2",
		EnvVars: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.CreateSession(sess); err != nil {
		t.Fatal(err)
	}

	// Update output
	if err := s.UpdateSessionOutput(sess.ID, "new output"); err != nil {
		t.Fatalf("UpdateSessionOutput: %v", err)
	}

	got, _ := s.GetSession(sess.ID)
	if got.LastOutput != "new output" {
		t.Errorf("last_output = %q, want 'new output'", got.LastOutput)
	}

	// Update nonexistent
	err := s.UpdateSessionOutput("nonexistent", "x")
	if err != ErrNotFound {
		t.Errorf("UpdateSessionOutput nonexistent: got %v, want ErrNotFound", err)
	}
}

func TestForeignKeyEnforced(t *testing.T) {
	s := testStore(t)
	now := time.Now()

	sess := &models.Session{
		ID: models.NewID(), Name: "orphan", HostID: "nonexistent-host",
		AgentType: models.AgentTypeClaude, Status: models.SessionStatusPending,
		Prompt: "x", BranchName: "b",
		EnvVars: map[string]string{}, CreatedAt: now, UpdatedAt: now,
	}
	err := s.CreateSession(sess)
	if err == nil {
		t.Error("expected foreign key error when creating session with nonexistent host_id")
	}
}
