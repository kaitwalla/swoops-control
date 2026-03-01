package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/swoopsh/swoops/pkg/agentrpc"
	"github.com/swoopsh/swoops/pkg/models"
	"github.com/swoopsh/swoops/server/internal/api"
	"github.com/swoopsh/swoops/server/internal/agentconn"
	"github.com/swoopsh/swoops/server/internal/config"
	"github.com/swoopsh/swoops/server/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"log/slog"
	"net"
	"net/http/httptest"
)

// TestEndToEndHostRegistrationAndSession tests the complete workflow:
// 1. Create a host via REST API
// 2. Connect agent via gRPC
// 3. Verify metrics collection
func TestEndToEndHostRegistrationAndSession(t *testing.T) {
	// Setup test database
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	// Setup test config
	cfg := config.DefaultConfig()
	cfg.Auth.APIKey = "test-api-key"
	cfg.GRPC.Insecure = true
	cfg.GRPC.Port = 0 // Use random port

	// Create API server
	apiServer := api.NewServer(st, cfg)
	defer apiServer.Close()

	// Create agent connection service
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))
	agentSvc := agentconn.NewService(st, cfg, logger)
	defer agentSvc.Close()

	apiServer.SetAgentOutputSource(agentSvc)
	apiServer.SetAgentController(agentSvc)

	// Start gRPC server
	grpcListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	grpcServer := grpc.NewServer()
	agentrpc.RegisterAgentServiceServer(grpcServer, agentSvc)

	go grpcServer.Serve(grpcListener)
	defer grpcServer.GracefulStop()

	grpcAddr := grpcListener.Addr().String()

	// Start HTTP test server
	httpServer := httptest.NewServer(apiServer)
	defer httpServer.Close()

	baseURL := httpServer.URL
	apiKey := cfg.Auth.APIKey

	// Test 1: Create a host
	t.Run("CreateHost", func(t *testing.T) {
		createHostReq := map[string]interface{}{
			"name":           "test-host",
			"hostname":       "test.example.com",
			"ssh_user":       "testuser",
			"ssh_key_path":   "/home/testuser/.ssh/id_rsa",
			"base_repo_path": "/repos/myproject",
		}
		body, _ := json.Marshal(createHostReq)

		req, _ := http.NewRequest("POST", baseURL+"/api/v1/hosts", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to create host: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("Expected status 201, got %d", resp.StatusCode)
		}

		var host models.Host
		if err := json.NewDecoder(resp.Body).Decode(&host); err != nil {
			t.Fatalf("Failed to decode host response: %v", err)
		}

		if host.ID == "" {
			t.Fatal("Expected non-empty host ID")
		}
		if host.Name != "test-host" {
			t.Fatalf("Expected host name 'test-host', got '%s'", host.Name)
		}

		t.Logf("Created host: %s (ID: %s)", host.Name, host.ID)
	})

	// Test 2: List hosts
	t.Run("ListHosts", func(t *testing.T) {
		req, _ := http.NewRequest("GET", baseURL+"/api/v1/hosts", nil)
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to list hosts: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		var hosts []models.Host
		if err := json.NewDecoder(resp.Body).Decode(&hosts); err != nil {
			t.Fatalf("Failed to decode hosts response: %v", err)
		}

		if len(hosts) != 1 {
			t.Fatalf("Expected 1 host, got %d", len(hosts))
		}

		t.Logf("Listed %d host(s)", len(hosts))
	})

	// Test 3: Connect agent via gRPC
	t.Run("ConnectAgent", func(t *testing.T) {
		// Get the host to retrieve auth token
		hosts, _ := st.ListHosts()
		if len(hosts) == 0 {
			t.Fatal("No hosts found")
		}
		host := hosts[0]

		// Connect agent
		conn, err := grpc.NewClient(grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			t.Fatalf("Failed to connect to gRPC: %v", err)
		}
		defer conn.Close()

		client := agentrpc.NewAgentServiceClient(conn)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		stream, err := client.Connect(ctx)
		if err != nil {
			t.Fatalf("Failed to open stream: %v", err)
		}

		// Send hello
		err = stream.Send(&agentrpc.AgentEnvelope{
			Hello: &agentrpc.AgentHello{
				HostID:       host.ID,
				AuthToken:    host.AgentAuthToken,
				HostName:     "test-host",
				AgentVersion: "test-v1",
				OS:           "linux",
				Arch:         "amd64",
			},
		})
		if err != nil {
			t.Fatalf("Failed to send hello: %v", err)
		}

		// Receive acknowledgement
		msg, err := stream.Recv()
		if err != nil {
			t.Fatalf("Failed to receive ack: %v", err)
		}

		if msg.Ack == nil {
			t.Fatal("Expected ack message")
		}

		t.Logf("Agent connected successfully")

		// Send heartbeat
		err = stream.Send(&agentrpc.AgentEnvelope{
			Heartbeat: &agentrpc.Heartbeat{
				SentUnix:        time.Now().Unix(),
				RunningSessions: 0,
			},
		})
		if err != nil {
			t.Fatalf("Failed to send heartbeat: %v", err)
		}

		t.Logf("Heartbeat sent successfully")

		// Close stream
		stream.CloseSend()
	})

	// Test 4: Health check
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/api/v1/health")
		if err != nil {
			t.Fatalf("Failed to check health: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Logf("Health check passed")
	})

	// Test 5: Metrics endpoint
	t.Run("MetricsEndpoint", func(t *testing.T) {
		resp, err := http.Get(baseURL + "/metrics")
		if err != nil {
			t.Fatalf("Failed to fetch metrics: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}

		t.Logf("Metrics endpoint accessible")
	})
}

// TestMetricsInstrumentation verifies that metrics are being collected
func TestMetricsInstrumentation(t *testing.T) {
	// Setup test database
	dbPath := filepath.Join(t.TempDir(), "test.db")
	st, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer st.Close()

	// Setup test config
	cfg := config.DefaultConfig()
	cfg.Auth.APIKey = "test-api-key"

	// Create API server
	apiServer := api.NewServer(st, cfg)
	defer apiServer.Close()

	// Start HTTP test server
	httpServer := httptest.NewServer(apiServer)
	defer httpServer.Close()

	baseURL := httpServer.URL

	// Make several API calls to generate metrics
	for i := 0; i < 5; i++ {
		resp, _ := http.Get(baseURL + "/api/v1/health")
		resp.Body.Close()
	}

	// Fetch metrics
	resp, err := http.Get(baseURL + "/metrics")
	if err != nil {
		t.Fatalf("Failed to fetch metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode)
	}

	// Read metrics body
	var buf bytes.Buffer
	buf.ReadFrom(resp.Body)
	metricsOutput := buf.String()

	// Verify that key metrics are present
	expectedMetrics := []string{
		"swoops_http_requests_total",
		"swoops_http_request_duration_seconds",
		"swoops_agent_connections_total",
		"swoops_agent_connections_active",
	}

	for _, metric := range expectedMetrics {
		if !bytes.Contains([]byte(metricsOutput), []byte(metric)) {
			t.Errorf("Expected metric %s not found in output", metric)
		}
	}

	t.Logf("All expected metrics found")
}

func init() {
	// Ensure test output is visible
	fmt.Println("Running integration tests...")
}
