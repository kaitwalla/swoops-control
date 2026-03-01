# Swoops

Distributed AI Agent Orchestrator Control Plane.

Manage multiple AI agent sessions (Claude Code, Codex) across a fleet of remote hosts from a centralized Web UI, using Git Worktrees for session isolation and Tmux for process persistence.

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

## Quick Start

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

## Configuration

```yaml
# swoopsd.yaml
server:
  host: 0.0.0.0
  port: 8080
  allowed_origins:
    - http://localhost:5173  # Vite dev server

database:
  path: swoops.db

grpc:
  host: 0.0.0.0
  port: 9090
  tls_cert: /path/to/cert.pem  # Required in production (insecure=false)
  tls_key: /path/to/key.pem    # Required in production
  insecure: true               # Set to false in production

auth:
  api_key: your-persistent-api-key
```

## API

All endpoints (except `/api/v1/health`) require authentication via Bearer token or `?token=` query parameter:

```bash
curl -H "Authorization: Bearer YOUR_API_KEY" http://localhost:8080/api/v1/hosts
```

| Method | Path | Description |
|--------|------|-------------|
| GET | `/api/v1/health` | Health check (unauthenticated) |
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

## Current Status — Phase 3 Complete

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

### Security
- All API routes (except health) require Bearer token authentication
- WebSocket auth via `?token=` query parameter (browser WebSocket can't set headers)
- **Agent authentication** — cryptographically secure tokens (256-bit) with constant-time comparison
- **gRPC TLS support** — configurable TLS encryption for agent connections (production recommended)
- **Input validation** — comprehensive validation of all gRPC inputs (host IDs, tokens, versions)
- CORS restricted to configured origins (defaults to localhost only)
- Internal errors are logged server-side, clients receive generic "internal server error"
- SSH host key verification via known_hosts (TOFU, rejects mismatches)
- SQLite foreign key constraints enforced
- Delete/update operations return 404 for nonexistent resources
- Constant-time API key comparison
- Agent auth tokens excluded from JSON API responses

## Roadmap

### Phase 3: Swoops Agent + gRPC ✅ COMPLETE
**Implemented:**
- ✅ gRPC bidirectional streaming endpoint (`AgentService/Connect`)
- ✅ Agent authentication with 256-bit secure tokens
- ✅ TLS support for agent connections (configurable)
- ✅ Heartbeat tracking with host status FSM (`online`/`degraded`/`offline`)
- ✅ Background heartbeat monitor with configurable thresholds
- ✅ Session output ingestion via gRPC stream (dual-path: tmux + gRPC)
- ✅ WebSocket terminal endpoint streams from both SSH tmux and gRPC agent output
- ✅ Session lifecycle routing prefers connected agent, with automatic SSH fallback
- ✅ Agent command execution with explicit acknowledgements and timeout handling (10s)
- ✅ Comprehensive input validation and error handling
- ✅ Structured logging with slog (JSON format)
- ✅ Concurrent connection handling with proper lock ordering
- ✅ Test suite: concurrent connections, heartbeat monitoring, command failures
- ✅ JSON codec for gRPC (human-readable, easier debugging)

**What's left for agents:**
- Agent daemon implementation (`swoops-agent run`)
- Agent service installer (systemd/launchd)
- Agent-side session management (worktree + tmux + output streaming)

### Phase 4: MCP Bridge
- Agent acts as MCP stdio server for AI agents
- Tools: `report_status`, `get_task`, `request_review`, `coordinate_with_session`
- MCP config generation for Claude Code (`.mcp.json`) and Codex (`.codex/config.toml`)

### Phase 5: Plugin System
- Plugins as git repos with `swoops-plugin.yaml` manifest
- Platform-aware binary resolution (linux/darwin x amd64/arm64)
- Install/update/remove plugins across hosts
- Agent CLI installation (Claude Code, Codex) via plugin system

### Phase 6: Production Hardening
- HTTPS/TLS for control plane (HTTP server)
- ✅ TLS for agent gRPC connections (implemented in Phase 3)
- mTLS for agent connections (mutual TLS authentication)
- Prometheus metrics
- Docker images
- Integration tests
- ✅ Structured logging (implemented in Phase 3)

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
