package sessionmgr

import (
	"os"
	"testing"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/store"
)

type fakeAgentController struct {
	connected bool
	launches  int
	stops     int
	inputs    int
}

func (f *fakeAgentController) IsHostConnected(hostID string) bool { return f.connected }
func (f *fakeAgentController) LaunchSession(sess *models.Session, host *models.Host) error {
	f.launches++
	return nil
}
func (f *fakeAgentController) StopSession(sess *models.Session, host *models.Host) error {
	f.stops++
	return nil
}
func (f *fakeAgentController) SendInput(sess *models.Session, host *models.Host, input string) error {
	f.inputs++
	return nil
}

func testStore(t *testing.T) *store.Store {
	t.Helper()
	tmp, err := os.CreateTemp("", "swoops-sessionmgr-test-*.db")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	t.Cleanup(func() { os.Remove(tmp.Name()) })

	s, err := store.New(tmp.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestLaunchSessionUsesAgentWhenConnected(t *testing.T) {
	st := testStore(t)
	mgr := New(st)
	controller := &fakeAgentController{connected: true}
	mgr.SetAgentController(controller)

	now := time.Now()
	host := &models.Host{
		ID:           models.NewID(),
		Name:         "h1",
		Hostname:     "invalid-host",
		SSHPort:      22,
		SSHUser:      "deploy",
		SSHKeyPath:   "/tmp/k",
		Status:       models.HostStatusOnline,
		MaxSessions:  10,
		BaseRepoPath: "/repo",
		WorktreeRoot: "/worktrees",
		Labels:       map[string]string{},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := st.CreateHost(host); err != nil {
		t.Fatal(err)
	}

	sess := &models.Session{
		ID:         models.NewID(),
		Name:       "s1",
		HostID:     host.ID,
		AgentType:  models.AgentTypeClaude,
		Status:     models.SessionStatusPending,
		Prompt:     "fix",
		BranchName: "swoops/s1",
		EnvVars:    map[string]string{},
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := st.CreateSession(sess); err != nil {
		t.Fatal(err)
	}

	if err := mgr.LaunchSession(sess.ID, host.ID); err != nil {
		t.Fatalf("LaunchSession: %v", err)
	}
	if controller.launches != 1 {
		t.Fatalf("launches=%d want 1", controller.launches)
	}

	got, err := st.GetSession(sess.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != models.SessionStatusRunning {
		t.Fatalf("status=%q want %q", got.Status, models.SessionStatusRunning)
	}
	if got.WorktreePath == "" {
		t.Fatal("worktree_path should be populated for launch metadata")
	}
}
