# Swoops

Distributed AI Agent Orchestrator Control Plane.

Manage multiple AI agent sessions (Claude Code, Codex) across a fleet of remote hosts from a centralized Web UI, using Git Worktrees for session isolation and Tmux for process persistence.

### Architecture

```
Internet                     Production Deployment
   │
   ├─ HTTPS:443 ─────────> Reverse Proxy (Caddy/nginx)
   │                       • Automatic Let's Encrypt
   │                       • Auto-renewal
   │                       │
   │                       ├─> HTTP:8080 ────> Control Plane
   │                       │                   (REST + WebSocket)
   │                       │                        │
   │                       └─> gRPC:9090 ──┐       │
   │                                        │       │
   └─ Agents ──────────────────────────────┘       │
      (mTLS authenticated)                         │
                                                    │
                                              ┌─────┴─────┐
                                              │           │
                                         SQLite DB   MCP Bridge
                                                          │
                                                    ┌─────┴─────┐
                                                    │ AI Agents │
                                                    │   (tmux)  │
                                                    └───────────┘
```

**Key Features:**
- **Web UI**: React-based dashboard for managing hosts and sessions
- **Control Plane**: Go server with REST API, WebSocket streaming, and gRPC
- **Agent**: Daemon running on each host for session management
  - Auto-updates with one-click from UI
  - No SSH keys required for shell sessions
- **Reverse Proxy**: Caddy or nginx with automatic HTTPS (recommended for production)
- **MCP Bridge**: AI agents can coordinate via MCP tools

## Installation

### Quick Install (Recommended)

Install the latest release binaries:

```bash
# Install both server and agent
curl -fsSL https://raw.githubusercontent.com/kaitwalla/swoops-control/main/install.sh | bash

# Install only the server
curl -fsSL https://raw.githubusercontent.com/kaitwalla/swoops-control/main/install.sh | bash -s -- --server

# Install only the agent
curl -fsSL https://raw.githubusercontent.com/kaitwalla/swoops-control/main/install.sh | bash -s -- --agent

# Install specific version to custom directory
curl -fsSL https://raw.githubusercontent.com/kaitwalla/swoops-control/main/install.sh | bash -s -- --version v1.0.0 --install-dir /usr/local/bin
```

Or download the install script and run it locally:

```bash
curl -fsSL -o install.sh https://raw.githubusercontent.com/kaitwalla/swoops-control/main/install.sh
chmod +x install.sh
./install.sh --help
```

### Manual Download

Download pre-built binaries from the [releases page](https://github.com/kaitwalla/swoops-control/releases):

- `swoopsd-{linux,darwin}-{amd64,arm64}` - Control plane server
- `swoops-agent-{linux,darwin}-{amd64,arm64}` - Agent daemon

### Build from Source

## Quick Start

### Interactive Setup (Recommended)

For production deployments, use the interactive setup script:

```bash
# Download and run the setup script
curl -fsSL https://raw.githubusercontent.com/kaitwalla/swoops-control/main/setup.sh | bash

# Or clone and run locally
./setup.sh
```

The setup script will:
- ✅ Guide you through all configuration options
- ✅ Generate configuration files for server/agent
- ✅ Generate certificates via step-ca (automated CA with renewal) or self-signed
- ✅ Configure reverse proxy (Caddy or nginx)
- ✅ Install as systemd/launchd service
- ✅ Provide next steps for starting services

**Certificate Distribution for Remote Agents:**

When using TLS/mTLS with remote agents, you have two options:

**Option 1 (Recommended): Automatic Download**

Agents can automatically download the CA certificate from the server on first run:

```bash
# Agent will download CA cert from the control plane HTTP API
swoops-agent run \
  --server server.example.com:9090 \
  --host-id my-host \
  --download-ca \
  --insecure=false \
  --http-url http://server.example.com:8080

# CA cert is saved to ~/.config/swoops/certs/server-ca.pem by default
```

**Option 2: Manual Distribution**

Copy certificates from the server to each agent machine:

```bash
# On each agent machine, copy the CA certificate from the server
scp user@server:/etc/swoops/certs/server-ca.pem /etc/swoops/certs/

# If using mTLS, also copy client certificates
scp user@server:/etc/swoops/certs/agent-cert.pem /etc/swoops/certs/
scp user@server:/etc/swoops/certs/agent-key.pem /etc/swoops/certs/
chmod 600 /etc/swoops/certs/agent-key.pem  # Keep private key secure
```

The setup script provides detailed instructions for your specific configuration.

### Development

```bash
# Build everything
make build

# Run the server (auto-generates API key on first run)
./bin/swoopsd

# Development mode (server + frontend hot reload)
make dev

# Start an agent (after host registration)
./bin/swoops-agent run --server 127.0.0.1:9090 --host-id <host-id>

# Install as a persistent service
# Linux user service:
./bin/swoops-agent service-install --host-id <host-id> --server 127.0.0.1:9090 --scope user
# macOS launchd agent:
./bin/swoops-agent service-install --host-id <host-id> --server 127.0.0.1:9090
```

The server starts on `http://localhost:8080`. On first run, it generates an ephemeral API key printed to stdout. Set `SWOOPS_API_KEY` or configure `auth.api_key` in a config file to persist it.

```bash
# With a config file
./bin/swoopsd -config swoopsd.yaml

# With environment variables
SWOOPS_API_KEY=your-key SWOOPS_DB_PATH=./data.db ./bin/swoopsd
```

### Production Deployment

For production deployments, see the complete guide with **automatic HTTPS via reverse proxy**:

**📖 [Production Deployment Guide](PRODUCTION.md)**

**Recommended:** Use Caddy or nginx as a reverse proxy for:
- ✅ **Automatic Let's Encrypt certificates** - zero-config HTTPS
- ✅ **Auto-renewal** - no manual intervention needed
- ✅ **WebSocket support** - built-in upgrade handling
- ✅ **Standard deployment pattern** - separation of concerns

The guide also covers:
- Caddy and nginx configurations with working examples
- Docker Compose setup with automatic HTTPS
- mTLS setup for agent connections
- Prometheus metrics and Grafana dashboards
- Kubernetes deployment manifests
- Security hardening checklist

**📋 [Changelog](CHANGELOG.md)** - Full release history and version notes

**👨‍💻 [Developer Guide](DEVELOPER_GUIDE.md)** - Complete guide for developers and AI agents

## Configuration

```yaml
# swoopsd.yaml
server:
  host: 0.0.0.0
  port: 8080
  allowed_origins:
    - http://localhost:5173  # Vite dev server
  tls_enabled: false          # Enable in production
  tls_cert: /path/to/server-cert.pem
  tls_key: /path/to/server-key.pem

database:
  path: swoops.db

grpc:
  host: 0.0.0.0
  port: 9090
  tls_cert: /path/to/grpc-cert.pem    # Required in production (insecure=false)
  tls_key: /path/to/grpc-key.pem      # Required in production
  client_ca: /path/to/client-ca.pem   # Required for mTLS
  insecure: true                      # Set to false in production
  require_mtls: false                 # Set to true for mTLS

auth:
  api_key: your-persistent-api-key
```

## API

All endpoints (except `/api/v1/health`, `/api/v1/version`, and `/api/v1/ca-cert`) require authentication via Bearer token or `?token=` query parameter:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" http://localhost:8080/api/v1/hosts
```

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check (unauthenticated) |
| GET | `/api/v1/version` | Version information (unauthenticated) |
| GET | `/api/v1/ca-cert` | Download CA certificate (unauthenticated) |
| GET | `/api/v1/stats` | Aggregate statistics |
| GET/POST | `/api/v1/hosts` | List / register hosts |
| GET/PUT/DELETE | `/api/v1/hosts/{id}` | Host CRUD |
| GET | `/api/v1/hosts/{id}/sessions` | Sessions on a host |
| GET/POST | `/api/v1/sessions` | List / create sessions |
| GET/DELETE | `/api/v1/sessions/{id}` | Session detail / cleanup |
| POST | `/api/v1/sessions/{id}/stop` | Stop a session (kills tmux, removes worktree) |
| POST | `/api/v1/sessions/{id}/input` | Send input to tmux session |
| GET | `/api/v1/sessions/{id}/output` | Get session output (live capture) |
| WS | `/api/v1/ws/sessions/{id}/output` | WebSocket live output stream |
| POST | `/api/v1/sessions/{id}/status` | Report agent status (MCP tool) |
| GET | `/api/v1/sessions/{id}/status` | List status updates |
| POST | `/api/v1/sessions/{id}/tasks` | Create task for session |
| GET | `/api/v1/sessions/{id}/tasks` | List all tasks |
| GET | `/api/v1/sessions/{id}/tasks/next` | Get next pending task (MCP tool) |
| POST | `/api/v1/sessions/{id}/reviews` | Create review request (MCP tool) |
| POST | `/api/v1/sessions/{id}/messages` | Send message to another session (MCP tool) |
| GET | `/api/v1/sessions/{id}/messages` | List session messages |
| GET | `/api/v1/reviews` | List all review requests |
| GET/PUT | `/api/v1/reviews/{review_id}` | Get/update review request |

## Session Lifecycle

When you create a session from the UI, the control plane:

1. **Creates a git worktree** on the target host via SSH (`git worktree add -b <branch> <path> HEAD`)
2. **Starts a tmux session** in the worktree directory (`tmux new-session -d -s swoop-<id> -c <path>`)
3. **Launches the AI agent** inside tmux (e.g., `claude --print '<prompt>'` or `codex '<prompt>'`)
4. **Polls output** via `tmux capture-pane` every 1s, broadcasting to WebSocket subscribers
5. **On stop**: kills the tmux session, removes the worktree, updates status

## Project Structure

```
swoops/
├── pkg/                  # Shared Go library
│   ├── models/           #   Domain types (Host, Session, Plugin, etc.)
│   ├── tmux/             #   Tmux CLI wrapper (local + SSH)
│   ├── worktree/         #   Git worktree CLI wrapper (local + SSH)
│   └── sshexec/          #   SSH client with known_hosts TOFU
├── server/               # Control plane
│   ├── cmd/swoopsd/      #   Server entrypoint
│   └── internal/
│       ├── config/       #   YAML + env config
│       ├── store/        #   SQLite persistence + migrations
│       ├── sessionmgr/   #   Session lifecycle orchestration via SSH
│       ├── api/          #   REST + WebSocket API (Chi router, auth)
│       └── frontend/     #   go:embed compiled React assets
├── agent/                # Swoops agent (runs on each host)
│   └── cmd/swoops-agent/ #   Agent entrypoint + service installer (systemd/launchd)
├── web/                  # React + Vite + Tailwind frontend
│   └── src/
│       ├── api/          #   Typed API client with auth + WebSocket
│       ├── stores/       #   Zustand state management
│       ├── pages/        #   Dashboard, Hosts, Sessions, etc.
│       └── components/   #   TerminalOutput, CreateSessionDialog, etc.
├── proto/                # Protobuf definitions (Phase 3)
├── Makefile
└── go.work
```

## Current Status — Production Ready!

### What works now
- **Control plane server** — single Go binary (17MB) with embedded React frontend
- **API authentication** — Bearer token auth + `?token=` query param (for WebSocket), auto-generated key on first run
- **Host management** — register, update, delete hosts via REST API and Web UI
- **Session orchestration via SSH** — create sessions that launch AI agents on remote hosts:
  - Git worktree creation on remote host
  - Tmux session creation and agent launch
  - Live output capture via `tmux capture-pane` polling (1s interval)
  - Send input to running sessions
  - Stop sessions (kill tmux + remove worktree)
- **WebSocket output streaming** — live terminal output via `/api/v1/ws/sessions/{id}/output`
- **xterm.js terminal viewer** — real terminal rendering in the browser with WebSocket + polling fallback
- **CreateSessionDialog** — create sessions from the Web UI (select host, agent type, prompt, branch)
- **SQLite persistence** — WAL mode, foreign key enforcement, automatic migrations
- **Web UI** — Dashboard, Hosts list/detail, Sessions list/detail with live terminal, Plugins/Templates stubs
- **SSH client** — known_hosts TOFU (trust on first use), key mismatch rejection, connection pooling
- **Test suite** — 40+ tests: store CRUD (10), API endpoints (8), session manager (7), agent connection (11+), heartbeat monitoring (4)
- **gRPC Agent Service** — bidirectional streaming for agent connections with authentication and optional TLS
- **Agent-based orchestration** — sessions can be launched via connected agents (gRPC) with automatic SSH fallback
- **Host heartbeat monitoring** — automatic host status tracking (online/degraded/offline) based on agent heartbeats
- **Live output streaming** — dual-path output (tmux polling for SSH, gRPC streaming from agents)
- **Structured logging** — JSON logs with slog for production observability
- **HTTPS/TLS** — Optional TLS encryption for HTTP API server
- **Agent mTLS** — Mutual TLS authentication for agent gRPC connections with client certificates
- **Prometheus metrics** — `/metrics` endpoint with comprehensive instrumentation
- **Docker images** — Production-ready containerized deployment with multi-stage builds
- **Integration tests** — End-to-end test suite validating API, gRPC, and metrics

### Security
- All API routes (except health and metrics) require Bearer token authentication
- WebSocket auth via `?token=` query parameter (browser WebSocket can't set headers)
- **HTTPS/TLS** — Full TLS 1.3 support for HTTP API server with certificate validation
- **Agent mTLS** — Mutual TLS with client certificate verification (optional but recommended)
- **Agent authentication** — cryptographically secure tokens (256-bit) with constant-time comparison
- **gRPC TLS support** — TLS 1.3 encryption for agent connections
- **Input validation** — comprehensive validation of all gRPC inputs (host IDs, tokens, versions)
- CORS restricted to configured origins (defaults to localhost only)
- Internal errors are logged server-side, clients receive generic "internal server error"
- SSH host key verification via known_hosts (TOFU, rejects mismatches)
- SQLite foreign key constraints enforced
- Delete/update operations return 404 for nonexistent resources
- Constant-time API key comparison
- Agent auth tokens excluded from JSON API responses
- Non-root container execution with minimal base images
- Read-only certificate and configuration mounts

## Future Roadmap

### Plugin System
- Plugins as git repos with `swoops-plugin.yaml` manifest
- Platform-aware binary resolution (linux/darwin x amd64/arm64)
- Install/update/remove plugins across hosts
- Agent CLI installation (Claude Code, Codex) via plugin system

### Future Enhancements
- PostgreSQL support for multi-instance deployments
- Redis caching for session state
- Rate limiting on API endpoints
- SAML/OAuth authentication
- Audit logging with retention policies
- Multi-tenancy support

## Building

```bash
make build          # Build frontend + server + agent
make build-agent-all  # Cross-compile agent for all platforms
make dev            # Dev mode with hot reload
make clean          # Clean build artifacts
```

### Prerequisites
- Go 1.25+
- Node.js 18+
- npm

## License

TBD
