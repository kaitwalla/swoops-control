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

auth:
  api_key: your-persistent-api-key
```

## API

All endpoints (except `/api/v1/health`) require authentication via Bearer token:

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
| POST | `/api/v1/sessions/{id}/stop` | Stop a session |
| POST | `/api/v1/sessions/{id}/input` | Send input to session |
| GET | `/api/v1/sessions/{id}/output` | Get session output |

## Project Structure

```
swoops/
├── pkg/                  # Shared Go library
│   ├── models/           #   Domain types (Host, Session, Plugin, etc.)
│   ├── tmux/             #   Tmux CLI wrapper
│   ├── worktree/         #   Git worktree CLI wrapper
│   └── sshexec/          #   SSH client with known_hosts TOFU
├── server/               # Control plane
│   ├── cmd/swoopsd/      #   Server entrypoint
│   └── internal/
│       ├── config/       #   YAML + env config
│       ├── store/        #   SQLite persistence + migrations
│       ├── api/          #   REST API (Chi router, auth middleware)
│       └── frontend/     #   go:embed compiled React assets
├── agent/                # Swoops agent (runs on each host)
│   └── cmd/swoops-agent/ #   Agent entrypoint (stub)
├── web/                  # React + Vite + Tailwind frontend
│   └── src/
│       ├── api/          #   Typed API client with auth
│       ├── stores/       #   Zustand state management
│       ├── pages/        #   Dashboard, Hosts, Sessions, etc.
│       └── components/   #   Reusable UI components
├── proto/                # Protobuf definitions (Phase 3)
├── Makefile
└── go.work
```

## Current Status — Phase 1 Complete

### What works now
- **Control plane server** — single Go binary (16MB) with embedded React frontend
- **API authentication** — Bearer token auth on all mutating endpoints, auto-generated key on first run
- **Host management** — register, update, delete hosts via REST API and Web UI
- **Session management** — create, list, stop, delete sessions via REST API and Web UI
- **SQLite persistence** — WAL mode, foreign key enforcement, automatic migrations
- **Web UI** — Dashboard, Hosts list, Host detail, Sessions list, Session detail pages
- **SSH client** — known_hosts TOFU (trust on first use), key mismatch rejection
- **Test suite** — store tests (CRUD, foreign keys, not-found) + API tests (auth, validation, error sanitization)

### Security
- All API routes (except health) require Bearer token authentication
- CORS restricted to configured origins (defaults to localhost only)
- Internal errors are logged server-side, clients receive generic "internal server error"
- SSH host key verification via known_hosts (TOFU, rejects mismatches)
- SQLite foreign key constraints enforced
- Delete/update operations return 404 for nonexistent resources

## Roadmap

### Phase 2: Sessions via SSH (next)
- Execute tmux/worktree operations on remote hosts over SSH
- Launch Claude Code and Codex sessions from the Web UI
- WebSocket-based live output streaming (tmux capture-pane)
- Send input to running sessions

### Phase 3: Swoops Agent + gRPC
- Agent daemon on each host (systemd on Linux, launchd on macOS)
- gRPC bidirectional streaming (agent-initiated, NAT-friendly)
- Heartbeat tracking, host status FSM
- Output streaming via gRPC instead of SSH polling

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
- HTTPS/TLS for control plane
- mTLS for agent connections
- Prometheus metrics
- Docker images
- Integration tests

## Building

```bash
make build          # Build frontend + server + agent
make build-agent-all  # Cross-compile agent for all platforms
make dev            # Dev mode with hot reload
make clean          # Clean build artifacts
```

### Prerequisites
- Go 1.23+
- Node.js 18+
- npm

## License

TBD
