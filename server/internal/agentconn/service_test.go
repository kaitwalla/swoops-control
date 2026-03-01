package agentconn

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"github.com/kaitwalla/swoops-control/server/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func testStore(t *testing.T) *store.Store {
	t.Helper()
	tmp, err := os.CreateTemp("", "swoops-agentconn-test-*.db")
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

func TestConnectUpdatesHostHeartbeat(t *testing.T) {
	st := testStore(t)
	now := time.Now()
	host := &models.Host{
		ID:           models.NewID(),
		Name:         "agent-host",
		Hostname:     "10.0.0.8",
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

	svc := NewService(st, nil, nil) // nil logger uses default
	defer svc.Close()

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	agentrpc.RegisterAgentServiceServer(gs, svc)
	defer gs.Stop()
	go func() { _ = gs.Serve(lis) }()

	ctx := context.Background()
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	defer conn.Close()

	client := agentrpc.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}

	if err := stream.Send(&agentrpc.AgentEnvelope{
		Hello: &agentrpc.AgentHello{
			HostID:       host.ID,
			AgentVersion: "v0.3.0",
			OS:           "linux",
			Arch:         "amd64",
			AuthToken:    host.AgentAuthToken,
		},
	}); err != nil {
		t.Fatalf("send hello: %v", err)
	}

	if _, err := stream.Recv(); err != nil {
		t.Fatalf("recv ack: %v", err)
	}

	if err := stream.Send(&agentrpc.AgentEnvelope{
		Heartbeat: &agentrpc.Heartbeat{SentUnix: time.Now().Unix()},
	}); err != nil {
		t.Fatalf("send heartbeat: %v", err)
	}

	got, err := st.GetHost(host.ID)
	if err != nil {
		t.Fatalf("get host: %v", err)
	}
	if got.Status != models.HostStatusOnline {
		t.Fatalf("status=%q want %q", got.Status, models.HostStatusOnline)
	}
	if got.AgentVersion != "v0.3.0" {
		t.Fatalf("agent_version=%q want %q", got.AgentVersion, "v0.3.0")
	}
	if got.LastHeartbeat == nil {
		t.Fatal("last_heartbeat should be set")
	}
}

func TestLaunchSessionSendsCommandToConnectedHost(t *testing.T) {
	st := testStore(t)
	now := time.Now()
	host := &models.Host{
		ID:           models.NewID(),
		Name:         "agent-host",
		Hostname:     "10.0.0.9",
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

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	agentrpc.RegisterAgentServiceServer(gs, svc)
	defer gs.Stop()
	go func() { _ = gs.Serve(lis) }()

	ctx := context.Background()
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	defer conn.Close()

	client := agentrpc.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}

	if err := stream.Send(&agentrpc.AgentEnvelope{
		Hello: &agentrpc.AgentHello{
			HostID:    host.ID,
			AuthToken: host.AgentAuthToken,
		},
	}); err != nil {
		t.Fatalf("send hello: %v", err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("recv ack: %v", err)
	}

	sess := &models.Session{
		ID:           models.NewID(),
		Name:         "s1",
		HostID:       host.ID,
		AgentType:    models.AgentTypeClaude,
		Prompt:       "fix",
		BranchName:   "swoops/s1",
		WorktreePath: "/worktrees/s1",
	}
	done := make(chan error, 1)
	go func() {
		msg, err := stream.Recv()
		if err != nil {
			done <- err
			return
		}
		if msg.Command == nil {
			done <- errors.New("expected command payload")
			return
		}
		if msg.Command.Command != agentrpc.CommandLaunch {
			done <- errors.New("unexpected command type")
			return
		}
		if msg.Command.SessionID != sess.ID {
			done <- errors.New("unexpected session id")
			return
		}
		err = stream.Send(&agentrpc.AgentEnvelope{
			CommandResult: &agentrpc.CommandResult{
				CommandID: msg.Command.CommandID,
				SessionID: msg.Command.SessionID,
				Ok:        true,
			},
		})
		done <- err
	}()

	if err := svc.LaunchSession(sess, host); err != nil {
		t.Fatalf("LaunchSession: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("agent handler: %v", err)
	}
}

func TestDesiredHostStatus(t *testing.T) {
	now := time.Now()
	degradedAfter := 10 * time.Second
	offlineAfter := 20 * time.Second

	cases := []struct {
		name    string
		host    *models.Host
		want    models.HostStatus
		now     time.Time
		dgrd    time.Duration
		offline time.Duration
	}{
		{
			name: "missing heartbeat is offline",
			host: &models.Host{},
			want: models.HostStatusOffline,
		},
		{
			name: "fresh heartbeat is online",
			host: &models.Host{LastHeartbeat: ptrTime(now.Add(-5 * time.Second))},
			want: models.HostStatusOnline,
		},
		{
			name: "stale heartbeat is degraded",
			host: &models.Host{LastHeartbeat: ptrTime(now.Add(-15 * time.Second))},
			want: models.HostStatusDegraded,
		},
		{
			name: "very stale heartbeat is offline",
			host: &models.Host{LastHeartbeat: ptrTime(now.Add(-30 * time.Second))},
			want: models.HostStatusOffline,
		},
		{
			name: "custom thresholds",
			host: &models.Host{LastHeartbeat: ptrTime(now.Add(-8 * time.Second))},
			want: models.HostStatusDegraded,
			dgrd: 5 * time.Second,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			dgrd := degradedAfter
			off := offlineAfter
			if tc.dgrd > 0 {
				dgrd = tc.dgrd
			}
			if tc.offline > 0 {
				off = tc.offline
			}
			got := desiredHostStatus(tc.host, now, dgrd, off)
			if got != tc.want {
				t.Fatalf("status=%q want %q", got, tc.want)
			}
		})
	}
}

func TestLaunchSessionCommandFailure(t *testing.T) {
	st := testStore(t)
	now := time.Now()
	host := &models.Host{
		ID:           models.NewID(),
		Name:         "agent-host",
		Hostname:     "10.0.0.9",
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

	lis := bufconn.Listen(1024 * 1024)
	gs := grpc.NewServer()
	agentrpc.RegisterAgentServiceServer(gs, svc)
	defer gs.Stop()
	go func() { _ = gs.Serve(lis) }()

	ctx := context.Background()
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial bufconn: %v", err)
	}
	defer conn.Close()

	client := agentrpc.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		t.Fatalf("connect stream: %v", err)
	}
	if err := stream.Send(&agentrpc.AgentEnvelope{
		Hello: &agentrpc.AgentHello{
			HostID:    host.ID,
			AuthToken: host.AgentAuthToken,
		},
	}); err != nil {
		t.Fatalf("send hello: %v", err)
	}
	if _, err := stream.Recv(); err != nil {
		t.Fatalf("recv ack: %v", err)
	}

	sess := &models.Session{
		ID:           models.NewID(),
		Name:         "s1",
		HostID:       host.ID,
		AgentType:    models.AgentTypeClaude,
		Prompt:       "fix",
		BranchName:   "swoops/s1",
		WorktreePath: "/worktrees/s1",
	}

	done := make(chan error, 1)
	go func() {
		msg, err := stream.Recv()
		if err != nil {
			done <- err
			return
		}
		if msg.Command == nil {
			done <- errors.New("missing command")
			return
		}
		err = stream.Send(&agentrpc.AgentEnvelope{
			CommandResult: &agentrpc.CommandResult{
				CommandID: msg.Command.CommandID,
				SessionID: msg.Command.SessionID,
				Ok:        false,
				Message:   "simulated failure",
			},
		})
		done <- err
	}()

	err = svc.LaunchSession(sess, host)
	if err == nil {
		t.Fatal("expected launch error from failed command result")
	}
	if got := err.Error(); !strings.Contains(got, "simulated failure") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("agent handler: %v", err)
	}
}

func ptrTime(v time.Time) *time.Time {
	return &v
}

func TestOutputSubscriptionPublish(t *testing.T) {
	st := testStore(t)
	svc := NewService(st, nil, nil)
	defer svc.Close()

	ch := svc.SubscribeSessionOutput("s1")
	defer svc.UnsubscribeSessionOutput("s1", ch)

	svc.publishOutput("s1", "hello")

	select {
	case out := <-ch:
		if out != "hello" {
			t.Fatalf("output=%q want %q", out, "hello")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for output")
	}
}
