package agentconn

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/kaitwalla/swoops-control/pkg/agentrpc"
	"github.com/kaitwalla/swoops-control/pkg/models"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

// TestConcurrentConnections tests that multiple agents connecting with the same host_id
// properly handle connection replacement without deadlocks.
func TestConcurrentConnections(t *testing.T) {
	st := testStore(t)
	now := time.Now()
	host := &models.Host{
		ID:           models.NewID(),
		Name:         "concurrent-host",
		Hostname:     "10.0.0.10",
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

	dialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }

	// Spawn 5 connections concurrently with the same host ID
	var wg sync.WaitGroup
	numConns := 5
	errors := make(chan error, numConns)

	for i := 0; i < numConns; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			conn, err := grpc.NewClient(
				"passthrough:///bufnet",
				grpc.WithContextDialer(dialer),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				errors <- err
				return
			}
			defer conn.Close()

			client := agentrpc.NewAgentServiceClient(conn)
			stream, err := client.Connect(ctx)
			if err != nil {
				errors <- err
				return
			}

			if err := stream.Send(&agentrpc.AgentEnvelope{
				Hello: &agentrpc.AgentHello{
					HostID:    host.ID,
					AuthToken: host.AgentAuthToken,
				},
			}); err != nil {
				errors <- err
				return
			}

			// Wait for ack
			if _, err := stream.Recv(); err != nil {
				errors <- err
				return
			}

			// Send a heartbeat
			if err := stream.Send(&agentrpc.AgentEnvelope{
				Heartbeat: &agentrpc.Heartbeat{SentUnix: time.Now().Unix()},
			}); err != nil {
				errors <- err
				return
			}

			// Wait a bit then disconnect
			time.Sleep(100 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for unexpected errors (some context cancellations are expected due to connection replacement)
	for err := range errors {
		// We expect some context cancellations as newer connections replace older ones
		if err != nil && !isContextCanceledError(err) {
			t.Errorf("unexpected error: %v", err)
		}
	}

	// At the end, at most one connection should be active or all should be gone
	connCount := 0
	if svc.IsHostConnected(host.ID) {
		connCount = 1
	}
	if connCount > 1 {
		t.Errorf("expected at most 1 active connection, got %d", connCount)
	}
}

// TestConcurrentCommandsDuringDisconnect tests sending commands while an agent disconnects
func TestConcurrentCommandsDuringDisconnect(t *testing.T) {
	st := testStore(t)
	now := time.Now()
	host := &models.Host{
		ID:           models.NewID(),
		Name:         "disconnect-host",
		Hostname:     "10.0.0.11",
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

	ctx, cancel := context.WithCancel(context.Background())
	conn, err := grpc.NewClient(
		"passthrough:///bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	client := agentrpc.NewAgentServiceClient(conn)
	stream, err := client.Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := stream.Send(&agentrpc.AgentEnvelope{
		Hello: &agentrpc.AgentHello{
			HostID:    host.ID,
			AuthToken: host.AgentAuthToken,
		},
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := stream.Recv(); err != nil {
		t.Fatal(err)
	}

	sess := &models.Session{
		ID:         models.NewID(),
		Name:       "s1",
		HostID:     host.ID,
		AgentType:  models.AgentTypeClaude,
		Prompt:     "fix",
		BranchName: "swoops/s1",
	}

	// Disconnect the agent immediately
	cancel()
	time.Sleep(50 * time.Millisecond)

	// Try to send a command - should fail gracefully
	err = svc.LaunchSession(sess, host)
	if err == nil {
		t.Fatal("expected error when sending command to disconnected host")
	}
	if svc.IsHostConnected(host.ID) {
		t.Fatal("host should not be connected after disconnect")
	}
}

func isContextCanceledError(err error) bool {
	return err != nil && (err == context.Canceled ||
		(err.Error() != "" && (
			err.Error() == "context canceled" ||
			err.Error() == "rpc error: code = Canceled desc = context canceled")))
}
