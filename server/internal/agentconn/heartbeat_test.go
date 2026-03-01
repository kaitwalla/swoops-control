package agentconn

import (
	"testing"
	"time"

	"github.com/swoopsh/swoops/pkg/models"
)

// TestHeartbeatMonitor tests that the background heartbeat monitor
// correctly transitions host status based on heartbeat age.
func TestHeartbeatMonitor(t *testing.T) {
	st := testStore(t)
	now := time.Now()

	host := &models.Host{
		ID:           models.NewID(),
		Name:         "monitor-host",
		Hostname:     "10.0.0.12",
		SSHPort:      22,
		SSHUser:      "deploy",
		SSHKeyPath:   "/tmp/key",
		Status:       models.HostStatusOffline,
		MaxSessions:  5,
		BaseRepoPath: "/repo",
		WorktreeRoot: "/worktrees",
		Labels:       map[string]string{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := st.CreateHost(host); err != nil {
		t.Fatal(err)
	}

	// Create service with short intervals for testing
	svc := NewService(st, nil, nil)
	svc.checkInterval = 100 * time.Millisecond
	svc.degradedAfter = 200 * time.Millisecond
	svc.offlineAfter = 400 * time.Millisecond

	// Override the time function for deterministic testing
	fixedTime := time.Now()
	svc.now = func() time.Time {
		return fixedTime
	}
	defer svc.Close()

	// Set a recent heartbeat
	recentTime := fixedTime.Add(-50 * time.Millisecond)
	if err := st.UpsertHostHeartbeat(host.ID, "v1.0.0", "linux", "amd64", recentTime); err != nil {
		t.Fatal(err)
	}

	// Run one check - should be online
	svc.reconcileHeartbeatStatus(fixedTime)
	h, _ := st.GetHost(host.ID)
	if h.Status != models.HostStatusOnline {
		t.Fatalf("status=%q want %q (fresh heartbeat)", h.Status, models.HostStatusOnline)
	}

	// Advance time to degraded threshold
	fixedTime = fixedTime.Add(250 * time.Millisecond)
	svc.now = func() time.Time {
		return fixedTime
	}
	svc.reconcileHeartbeatStatus(fixedTime)
	h, _ = st.GetHost(host.ID)
	if h.Status != models.HostStatusDegraded {
		t.Fatalf("status=%q want %q (degraded heartbeat)", h.Status, models.HostStatusDegraded)
	}

	// Advance time to offline threshold
	fixedTime = fixedTime.Add(250 * time.Millisecond)
	svc.now = func() time.Time {
		return fixedTime
	}
	svc.reconcileHeartbeatStatus(fixedTime)
	h, _ = st.GetHost(host.ID)
	if h.Status != models.HostStatusOffline {
		t.Fatalf("status=%q want %q (offline heartbeat)", h.Status, models.HostStatusOffline)
	}

	// Send a new heartbeat to bring it back online
	freshTime := fixedTime.Add(-10 * time.Millisecond)
	if err := st.TouchHostHeartbeat(host.ID, freshTime); err != nil {
		t.Fatal(err)
	}
	svc.reconcileHeartbeatStatus(fixedTime)
	h, _ = st.GetHost(host.ID)
	if h.Status != models.HostStatusOnline {
		t.Fatalf("status=%q want %q (recovered with fresh heartbeat)", h.Status, models.HostStatusOnline)
	}
}

// TestHeartbeatMonitorLoop tests that the monitor runs in the background
func TestHeartbeatMonitorLoop(t *testing.T) {
	st := testStore(t)
	now := time.Now()

	host := &models.Host{
		ID:           models.NewID(),
		Name:         "loop-host",
		Hostname:     "10.0.0.13",
		SSHPort:      22,
		SSHUser:      "deploy",
		SSHKeyPath:   "/tmp/key",
		Status:       models.HostStatusOnline,
		MaxSessions:  5,
		BaseRepoPath: "/repo",
		WorktreeRoot: "/worktrees",
		Labels:       map[string]string{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := st.CreateHost(host); err != nil {
		t.Fatal(err)
	}

	// Create service with very short intervals
	svc := NewService(st, nil, nil)
	svc.checkInterval = 50 * time.Millisecond
	svc.degradedAfter = 300 * time.Millisecond
	svc.offlineAfter = 1 * time.Second
	defer svc.Close()

	// Set a heartbeat that will become degraded (but not offline) soon
	staleTime := now.Add(-100 * time.Millisecond)
	if err := st.UpsertHostHeartbeat(host.ID, "v1.0.0", "linux", "amd64", staleTime); err != nil {
		t.Fatal(err)
	}

	// Wait for monitor to detect degraded status
	// The heartbeat will be ~400ms old after waiting 300ms
	time.Sleep(350 * time.Millisecond)

	// Host should have been marked degraded by the background monitor
	h, err := st.GetHost(host.ID)
	if err != nil {
		t.Fatal(err)
	}
	if h.Status != models.HostStatusDegraded {
		t.Fatalf("background monitor should have marked host as degraded, got status=%q", h.Status)
	}
}

// TestNoHeartbeatIsOffline tests that hosts without any heartbeat are considered offline
func TestNoHeartbeatIsOffline(t *testing.T) {
	st := testStore(t)
	now := time.Now()

	host := &models.Host{
		ID:           models.NewID(),
		Name:         "no-heartbeat-host",
		Hostname:     "10.0.0.14",
		SSHPort:      22,
		SSHUser:      "deploy",
		SSHKeyPath:   "/tmp/key",
		Status:       models.HostStatusOffline,
		MaxSessions:  5,
		BaseRepoPath: "/repo",
		WorktreeRoot: "/worktrees",
		Labels:       map[string]string{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := st.CreateHost(host); err != nil {
		t.Fatal(err)
	}

	svc := NewService(st, nil, nil)
	defer svc.Close()

	svc.reconcileHeartbeatStatus(time.Now())
	h, _ := st.GetHost(host.ID)
	if h.Status != models.HostStatusOffline {
		t.Fatalf("host without heartbeat should be offline, got status=%q", h.Status)
	}
}
