# Swoops Developer Guide

Complete developer documentation for AI agents and human developers working on Swoops.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Codebase Structure](#codebase-structure)
- [Key Components](#key-components)
- [Development Workflow](#development-workflow)
- [Testing Strategy](#testing-strategy)
- [Common Tasks](#common-tasks)
- [Code Patterns & Conventions](#code-patterns--conventions)
- [Security Considerations](#security-considerations)
- [Performance & Optimization](#performance--optimization)
- [Troubleshooting](#troubleshooting)

## Architecture Overview

Swoops is a distributed AI agent orchestration platform with three main components:

```
┌───────────┐      HTTPS/WSS       ┌────────────────┐     gRPC (mTLS)      ┌───────────────┐
│  Web UI   │ <──────────────────>  │ Control Plane  │ <──────────────────>  │ Swoops Agent  │
│ (React)   │  REST + WebSocket     │ (Go: swoopsd)  │  Bidirectional       │ (per host)    │
└───────────┘                       └────────────────┘  stream               └───────┬───────┘
                                           │                                         │
                                      SQLite DB                                MCP (stdio)
                                                                                     │
                                                                              ┌──────┴──────┐
                                                                              │  AI Agent   │
                                                                              │ claude/codex│
                                                                              └─────────────┘
```

### Data Flow

1. **Session Creation**: User creates session via Web UI → Control plane creates worktree + tmux via SSH or gRPC
2. **Output Streaming**: Agent captures tmux output → gRPC stream → Control plane → WebSocket → Web UI
3. **MCP Coordination**: AI agent calls MCP tools → swoops-agent → HTTP API → Control plane database
4. **Heartbeats**: Agent sends heartbeats → Control plane tracks host status (online/degraded/offline)

## Codebase Structure

```
swoops/
├── pkg/                       # Shared Go library (domain models, utilities)
│   ├── models/                # Domain types: Host, Session, Task, Review, etc.
│   ├── tmux/                  # Tmux CLI wrapper (local + SSH execution)
│   ├── worktree/              # Git worktree CLI wrapper (local + SSH)
│   └── sshexec/               # SSH client with TOFU host key verification
│
├── server/                    # Control plane server
│   ├── cmd/swoopsd/           # Server entrypoint (main.go)
│   ├── internal/
│   │   ├── config/            # Configuration (YAML + env, validation)
│   │   ├── store/             # SQLite persistence (migrations, CRUD)
│   │   ├── sessionmgr/        # Session lifecycle orchestration
│   │   ├── agentconn/         # gRPC agent service (bidirectional streaming)
│   │   ├── metrics/           # Prometheus metrics with bounded cardinality
│   │   ├── api/               # REST + WebSocket API (Chi router, auth middleware)
│   │   └── frontend/          # go:embed compiled React assets
│   ├── test/                  # Integration tests
│   └── Dockerfile             # Multi-stage Docker build
│
├── agent/                     # Swoops agent (runs on each host)
│   ├── cmd/swoops-agent/
│   │   ├── main.go            # Agent entrypoint
│   │   ├── service.go         # Service installer (systemd/launchd)
│   │   └── mcp.go             # MCP stdio server implementation
│   └── Dockerfile             # Agent Docker build
│
├── web/                       # React frontend
│   ├── src/
│   │   ├── api/               # Typed API client with auth + WebSocket
│   │   ├── stores/            # Zustand state management
│   │   ├── pages/             # Dashboard, Hosts, Sessions, etc.
│   │   └── components/        # UI components (TerminalOutput, etc.)
│   ├── vite.config.ts         # Vite build configuration
│   └── package.json
│
├── proto/                     # Protobuf definitions (gRPC API)
├── Makefile                   # Build automation
├── go.work                    # Go workspace configuration
├── docker-compose.yml         # Local development setup
└── prometheus.yml             # Prometheus scrape config
```

## Key Components

### 1. Control Plane (server/)

**Entry Point**: `server/cmd/swoopsd/main.go`

The control plane is a single Go binary that:
- Serves the embedded React frontend
- Exposes REST + WebSocket API for Web UI
- Runs gRPC server for agent connections
- Orchestrates sessions via SSH or connected agents
- Persists data in SQLite with WAL mode

**Key Files**:
- `internal/api/router.go` - HTTP routes and middleware setup
- `internal/api/handlers_session.go` - Session CRUD and WebSocket streaming
- `internal/api/handlers_mcp.go` - MCP coordination endpoints
- `internal/agentconn/service.go` - gRPC bidirectional streaming service
- `internal/sessionmgr/manager.go` - Session lifecycle management
- `internal/store/store.go` - Database layer with migrations
- `internal/metrics/metrics.go` - Prometheus instrumentation

### 2. Agent (agent/)

**Entry Point**: `agent/cmd/swoops-agent/main.go`

The agent runs on each host and:
- Connects to control plane via gRPC with mTLS
- Sends heartbeats to maintain host status
- Executes commands (create/stop sessions, send input)
- Streams tmux output to control plane
- Provides MCP stdio server for AI agents

**Commands**:
```bash
swoops-agent run          # Run agent daemon
swoops-agent mcp-serve    # Start MCP stdio server
swoops-agent service-install  # Install as systemd/launchd service
```

### 3. Shared Libraries (pkg/)

**Domain Models**: `pkg/models/`
- `Host` - Remote host registration
- `Session` - AI agent session (worktree + tmux + agent)
- `Task` - Instructions for AI agents
- `Review` - Code review requests from agents
- `Message` - Inter-session communication

**Utilities**:
- `pkg/tmux/` - Tmux operations (create, send input, capture output)
- `pkg/worktree/` - Git worktree management
- `pkg/sshexec/` - SSH execution with known_hosts TOFU

### 4. Database Schema

**Location**: `server/internal/store/migrations/`

**Core Tables**:
- `hosts` - Registered hosts with auth tokens
- `sessions` - AI agent sessions with status
- `session_output` - Captured terminal output
- `tasks` - Task queue for AI agents
- `reviews` - Review requests from AI agents
- `messages` - Inter-session messages
- `status_updates` - Agent status history

**Key Features**:
- SQLite with WAL mode for concurrent reads
- Foreign key constraints enforced
- Automatic migrations on startup
- Auth tokens excluded from JSON responses

## Development Workflow

### 1. Initial Setup

```bash
# Clone repository
git clone https://github.com/swoopsh/swoops.git
cd swoops

# Build everything (frontend + server + agent)
make build

# Run in development mode (hot reload)
make dev
```

### 2. Making Changes

**Backend Changes**:
```bash
# Edit Go code in server/ or agent/
vim server/internal/api/handlers_session.go

# Rebuild
make build-server

# Run tests
cd server/internal/api && go test -v
```

**Frontend Changes**:
```bash
# Edit React components in web/
vim web/src/components/TerminalOutput.tsx

# Frontend hot-reloads automatically in dev mode
# Or rebuild manually:
make build-web
```

**Database Changes**:
```bash
# Create new migration
cat > server/internal/store/migrations/003_add_new_table.sql <<EOF
CREATE TABLE new_table (
  id TEXT PRIMARY KEY,
  created_at INTEGER NOT NULL
);
EOF

# Migration runs automatically on next server start
```

### 3. Testing Locally

```bash
# Start server
./bin/swoopsd

# In another terminal, register a host
curl -H "Authorization: Bearer <api-key>" \
  -X POST http://localhost:8080/api/v1/hosts \
  -d '{"hostname": "local", "ssh_host": "localhost"}'

# Start agent (if testing agent features)
./bin/swoops-agent run \
  --server 127.0.0.1:9090 \
  --host-id <host-id> \
  --insecure
```

### 4. Running Tests

```bash
# All tests
make test

# Specific package
cd server/internal/store && go test -v

# Integration tests
cd server/test && go test -v

# Frontend tests
cd web && npm test
```

## Testing Strategy

### Unit Tests

**Location**: Co-located with source code (`*_test.go`)

**Patterns**:
```go
func TestSessionCreate(t *testing.T) {
    // Setup
    store := setupTestStore(t)
    defer store.Close()

    // Execute
    sess, err := store.CreateSession(&models.Session{
        HostID: "test-host",
        AgentType: "claude",
    })

    // Assert
    assert.NoError(t, err)
    assert.NotEmpty(t, sess.ID)
}
```

**Key Test Files**:
- `server/internal/store/store_test.go` - Database CRUD tests
- `server/internal/api/handlers_test.go` - API endpoint tests
- `server/internal/agentconn/service_test.go` - gRPC tests
- `server/internal/sessionmgr/manager_test.go` - Session lifecycle tests

### Integration Tests

**Location**: `server/test/`

**What They Test**:
- End-to-end API flows (create host → connect agent → create session)
- gRPC authentication and streaming
- WebSocket upgrades and messaging
- Metrics collection and cardinality
- Database migrations

**Run With**:
```bash
cd server/test && go test -v
```

### Concurrency Tests

**Location**: `server/internal/agentconn/concurrent_test.go`

Tests for race conditions:
- Multiple agents connecting with same host_id
- Commands during agent disconnect/reconnect
- Lock ordering (connMu → pendingMu → outputMu)

## Common Tasks

### Adding a New API Endpoint

1. **Define handler**:
```go
// server/internal/api/handlers_foo.go
func (s *Server) handleCreateFoo(w http.ResponseWriter, r *http.Request) {
    var req CreateFooRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeError(w, "invalid request", http.StatusBadRequest)
        return
    }

    foo, err := s.store.CreateFoo(&req)
    if err != nil {
        writeError(w, "failed to create", http.StatusInternalServerError)
        return
    }

    writeJSON(w, foo, http.StatusCreated)
}
```

2. **Register route**:
```go
// server/internal/api/router.go
r.Route("/api/v1", func(r chi.Router) {
    r.Use(s.authMiddleware)
    r.Post("/foos", s.handleCreateFoo)
})
```

3. **Add tests**:
```go
// server/internal/api/handlers_test.go
func TestCreateFoo(t *testing.T) {
    // Test implementation
}
```

### Adding a New Database Table

1. **Create migration**:
```sql
-- server/internal/store/migrations/00X_add_foos.sql
CREATE TABLE foos (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  created_at INTEGER NOT NULL,
  CONSTRAINT fk_host FOREIGN KEY (host_id) REFERENCES hosts(id) ON DELETE CASCADE
);

CREATE INDEX idx_foos_created ON foos(created_at DESC);
```

2. **Add model**:
```go
// pkg/models/foo.go
type Foo struct {
    ID        string `json:"id"`
    Name      string `json:"name"`
    CreatedAt int64  `json:"created_at"`
}
```

3. **Add store methods**:
```go
// server/internal/store/store.go
func (s *Store) CreateFoo(foo *models.Foo) (*models.Foo, error) {
    foo.ID = uuid.New().String()
    foo.CreatedAt = time.Now().Unix()

    _, err := s.db.Exec(`
        INSERT INTO foos (id, name, created_at)
        VALUES (?, ?, ?)
    `, foo.ID, foo.Name, foo.CreatedAt)

    return foo, err
}
```

### Adding a New MCP Tool

1. **Add tool to agent**:
```go
// agent/cmd/swoops-agent/mcp.go
func (s *mcpServer) handleNewTool(params json.RawMessage) (interface{}, error) {
    var req NewToolRequest
    if err := json.Unmarshal(params, &req); err != nil {
        return nil, err
    }

    // Call control plane API
    resp, err := s.callAPI("/api/v1/sessions/" + s.sessionID + "/newtool", req)
    return resp, err
}
```

2. **Add API endpoint**:
```go
// server/internal/api/handlers_mcp.go
func (s *Server) handleNewTool(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

3. **Update MCP config generation**:
```go
// server/internal/sessionmgr/manager.go
// Add tool to .mcp.json during session creation
```

## Code Patterns & Conventions

### Error Handling

**Internal Errors**:
```go
// Log detailed error server-side, return generic message to client
if err != nil {
    s.logger.Error("failed to create session",
        "error", err,
        "host_id", hostID,
    )
    writeError(w, "internal server error", http.StatusInternalServerError)
    return
}
```

**User Errors**:
```go
// Return specific error message for user mistakes
if req.AgentType == "" {
    writeError(w, "agent_type is required", http.StatusBadRequest)
    return
}
```

### Structured Logging

**Use slog with context**:
```go
s.logger.Info("session created",
    "session_id", session.ID,
    "host_id", session.HostID,
    "agent_type", session.AgentType,
)

s.logger.Error("command failed",
    "error", err,
    "session_id", sessionID,
    "command_type", cmdType,
)
```

### Concurrency

**Lock Ordering** (always acquire in this order):
1. `connMu` (agent connections)
2. `pendingMu` (pending commands)
3. `outputMu` (output buffers)

**Example**:
```go
s.connMu.Lock()
conn, ok := s.conns[hostID]
if ok {
    conn.pendingMu.Lock()
    // Do work
    conn.pendingMu.Unlock()
}
s.connMu.Unlock()
```

### Channel Cleanup

**Always close in goroutine**:
```go
// Bad - can deadlock
close(ch)

// Good - non-blocking cleanup
go func() {
    time.Sleep(100 * time.Millisecond)  // Grace period
    close(ch)
}()
```

### Metrics

**Use normalizePath for bounded cardinality**:
```go
// Bad - unbounded cardinality
path := r.URL.Path  // /api/v1/sessions/abc123

// Good - bounded cardinality
path := normalizePath(r.URL.Path)  // /api/v1/sessions/:id
metrics.HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(duration)
```

## Security Considerations

### Authentication

1. **API Key**: All endpoints (except `/health` and `/metrics`) require Bearer token
2. **Agent Auth**: Agents authenticate with 256-bit secure tokens (constant-time comparison)
3. **WebSocket Auth**: Token passed via query param (`?token=...`) since WebSocket can't set headers

### TLS/mTLS

**Production Requirements**:
- HTTPS for HTTP API (server.tls_enabled=true)
- TLS for gRPC (grpc.insecure=false)
- mTLS for agents (grpc.require_mtls=true)

**Certificate Management**:
- Server certificates in `/etc/swoops/certs/`
- Client certificates for each agent
- CA certificate for verification

### Input Validation

**Always validate**:
```go
func validateHello(hello *agentrpc.AgentHello) error {
    if hello.HostID == "" {
        return errors.New("host_id required")
    }
    if len(hello.HostID) > 255 {
        return errors.New("host_id too long")
    }
    // More validation...
}
```

### SQL Injection

**Use parameterized queries**:
```go
// Good
db.Exec("SELECT * FROM hosts WHERE id = ?", hostID)

// Never concatenate
db.Exec("SELECT * FROM hosts WHERE id = '" + hostID + "'")  // NEVER!
```

## Performance & Optimization

### Database

**WAL Mode**: Enables concurrent reads
```go
db.Exec("PRAGMA journal_mode=WAL")
```

**Indexes**: On frequently queried columns
```sql
CREATE INDEX idx_sessions_host ON sessions(host_id);
CREATE INDEX idx_sessions_created ON sessions(created_at DESC);
```

### Metrics Cardinality

**Path Normalization**: Prevents unbounded time series
```go
func normalizePath(path string) string {
    // /api/v1/sessions/abc123 → /api/v1/sessions/:id
    patterns := []struct {
        pattern string
        replace string
    }{
        {`/hosts/[a-f0-9-]+`, `/hosts/:id`},
        {`/sessions/[a-f0-9-]+`, `/sessions/:id`},
        // Add more patterns as needed
    }

    for _, p := range patterns {
        re := regexp.MustCompile(p.pattern)
        path = re.ReplaceAllString(path, p.replace)
    }
    return path
}
```

### Channel Buffering

**Sized Buffers**: Prevent blocking on bursts
```go
const (
    OutputChannelBuffer = 64   // Output messages
    CommandChannelBuffer = 16  // Commands to agent
)

sendCh: make(chan *agentrpc.ControlMessage, CommandChannelBuffer)
```

## Troubleshooting

### Common Issues

**"invalid authentication token"**
- Check agent token matches host's `AgentAuthToken`
- Verify `--host-id` is correct
- Token is generated on host creation

**"WebSocket upgrade failed"**
- Ensure metrics middleware preserves `http.Hijacker` interface
- Check token in query param: `?token=YOUR_KEY`
- Verify connection not blocked by CORS

**"Docker build fails with module not found"**
- Ensure all workspace modules copied in Dockerfile
- Both server and agent need `pkg/`, `server/go.mod`, `agent/go.mod`

**"High memory usage in Prometheus"**
- Check metrics cardinality is bounded (path normalization)
- Verify no custom metrics with unbounded labels
- Monitor with: `curl /metrics | grep swoops_http_requests_total | wc -l`

### Debug Commands

```bash
# Check agent logs
journalctl -u swoops-agent -f

# Test gRPC connectivity
grpcurl -insecure localhost:9090 list

# Check metrics cardinality
curl http://localhost:8080/metrics | grep swoops_http_requests_total

# Database queries
sqlite3 swoops.db "SELECT * FROM hosts;"

# WebSocket test
wscat -c "ws://localhost:8080/api/v1/ws/sessions/{id}/output?token=YOUR_KEY"
```

### Performance Profiling

```bash
# CPU profile
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profile
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Race detection
go test -race ./...
```

## Setup Script Reference

The interactive setup script (`setup.sh`) automates production deployment configuration:

### What It Does

1. **Component Selection**: Choose server, agent, or both
2. **Deployment Type**: Production with reverse proxy, direct TLS, or development
3. **Server Config**: Domain, ports, database path, API key generation
4. **Agent Config**: Control plane connection, host ID, authentication
5. **TLS/mTLS**: Certificate paths or automatic generation for testing
6. **Reverse Proxy**: Generate Caddy or nginx configuration
7. **Service Installation**: Create systemd/launchd service files
8. **Summary**: Complete next steps and verification commands

### Generated Files

**Server:**
- `swoopsd.yaml` - Server configuration
- `swoopsd.env` - Environment variables for systemd
- `Caddyfile` or `nginx-swoops.conf` - Reverse proxy config
- `/etc/systemd/system/swoopsd.service` - Systemd service (Linux)
- `~/Library/LaunchAgents/com.swoops.server.plist` - Launchd service (macOS)

**Agent:**
- `agent.env` - Agent configuration and environment variables
- `/etc/systemd/system/swoops-agent.service` - Systemd service (Linux)
- `~/Library/LaunchAgents/com.swoops.agent.plist` - Launchd service (macOS)

**Certificates (if generated):**
- `certs/ca-cert.pem` - Certificate Authority
- `certs/grpc-server-cert.pem` - gRPC server certificate
- `certs/grpc-server-key.pem` - gRPC server private key
- `certs/agent-cert.pem` - Agent client certificate (mTLS)
- `certs/agent-key.pem` - Agent client private key
- `certs/client-ca.pem` - CA for client verification
- `certs/server-ca.pem` - CA for server verification

### Usage Examples

```bash
# Interactive setup for production server with Caddy
./setup.sh
# Choose: 1 (server), 1 (reverse proxy), configure domain, generate API key

# Setup agent on remote host
./setup.sh
# Choose: 2 (agent), provide control plane hostname and auth token

# Setup both on same machine for testing
./setup.sh
# Choose: 3 (both), 3 (development mode)
```

### Modifying Generated Configs

After setup, you can manually edit configurations:

```bash
# Edit server config
vim /etc/swoops/swoopsd.yaml

# Reload service
sudo systemctl restart swoopsd

# Edit agent config
vim /etc/swoops/agent.env

# Reload service
sudo systemctl restart swoops-agent
```

## Additional Resources

- **[README.md](README.md)** - Quick start and feature overview
- **[PRODUCTION.md](PRODUCTION.md)** - Production deployment guide
- **[MCP_USAGE.md](MCP_USAGE.md)** - MCP tools for AI agent coordination
- **[CHANGELOG.md](CHANGELOG.md)** - Version history and release notes

## Contributing

When making changes:

1. **Read existing code** - Follow established patterns
2. **Write tests** - Unit tests for logic, integration tests for flows
3. **Update docs** - Keep README and this guide current
4. **Check security** - Review for OWASP top 10 vulnerabilities
5. **Test concurrency** - Run with `-race` flag
6. **Verify metrics** - Ensure labels are bounded
7. **Update changelog** - Document user-facing changes
