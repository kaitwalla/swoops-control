# Changelog

All notable changes to Swoops will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-02-28

### Added - Phase 6: Production Hardening
- **HTTPS/TLS Support**: Full TLS 1.3 encryption for HTTP API server
  - Configurable via `server.tls_enabled`, `server.tls_cert`, `server.tls_key`
  - Environment variables: `SWOOPS_TLS_ENABLED`, `SWOOPS_TLS_CERT`, `SWOOPS_TLS_KEY`
  - Development (HTTP) and production (HTTPS) modes

- **Mutual TLS (mTLS)**: Client certificate authentication for agent connections
  - Server-side client certificate verification with configurable CA
  - Agent-side client certificate presentation
  - Configuration: `grpc.require_mtls`, `grpc.client_ca`
  - Environment: `SWOOPS_GRPC_REQUIRE_MTLS`, `SWOOPS_GRPC_CLIENT_CA`

- **Prometheus Metrics**: Comprehensive instrumentation at `/metrics` endpoint
  - HTTP request metrics (latency, status codes) with bounded cardinality
  - gRPC connection metrics (active connections, errors, heartbeats)
  - Session lifecycle metrics (commands, duration)
  - Host status metrics
  - WebSocket connection tracking
  - Automatic path normalization to prevent unbounded cardinality

- **Docker Support**: Production-ready containerization
  - Multi-stage Dockerfiles for control plane and agent
  - Alpine-based images (<50MB)
  - Non-root user execution
  - Health checks included
  - Docker Compose setup with Prometheus and Grafana

- **Integration Tests**: End-to-end test suite
  - Host registration and API validation
  - Agent gRPC connection with authentication
  - Heartbeat functionality
  - Metrics collection verification
  - WebSocket upgrade validation
  - Path normalization testing

- **Documentation**: Comprehensive production deployment guide
  - TLS certificate generation (self-signed and Let's Encrypt)
  - Docker and Kubernetes deployment examples
  - Security hardening checklist
  - Monitoring and alerting configuration
  - Troubleshooting guide

### Fixed - Critical Production Issues

- **[P0] WebSocket Hijacker Interface**: Metrics middleware now properly preserves `http.Hijacker`, `http.Flusher`, and `http.Pusher` interfaces, fixing WebSocket upgrade failures
  - Added interface delegation in `responseWriter` wrapper
  - Test: `TestWebSocketWithMetricsMiddleware`

- **[P1] Docker Workspace Builds**: Docker builds now include all Go workspace modules
  - Both Dockerfiles copy complete workspace (`pkg/`, `server/`, `agent/`)
  - Fixes `go mod download` failures in multi-module workspace

- **[P2] Unbounded Metrics Cardinality**: HTTP metrics now use normalized paths
  - Resource IDs replaced with `:id` placeholders
  - Prevents Prometheus memory exhaustion from infinite time series
  - Example: `/api/v1/sessions/abc123` → `/api/v1/sessions/:id`
  - Test: `TestMetricsPathNormalization`

### Security
- TLS 1.3 encryption for HTTP and gRPC
- mTLS with client certificate verification
- Non-root container execution
- Read-only certificate mounts in Docker
- Comprehensive input validation
- Constant-time authentication comparison

### Performance
- Bounded metrics cardinality via path normalization
- Minimal Docker images (Alpine-based)
- Multi-stage builds for smaller images
- Efficient metric collection with minimal overhead

### Documentation
- [`PRODUCTION.md`](PRODUCTION.md) - Complete production deployment guide
- [`DEVELOPER_GUIDE.md`](DEVELOPER_GUIDE.md) - Comprehensive developer guide for AI agents and humans
- Updated README.md with production features

## [0.2.0] - Phase 4: MCP Bridge

### Added
- MCP stdio server (`swoops-agent mcp-serve`)
- Four MCP tools for AI agent coordination:
  - `report_status` - Report agent status
  - `get_task` - Retrieve pending tasks
  - `request_review` - Request human review
  - `coordinate_with_session` - Message other agents
- Automatic MCP config generation during session launch
- Database schema for MCP entities
- Control plane API endpoints for MCP operations

## [0.1.0] - Phase 3: Swoops Agent + gRPC

### Added
- gRPC bidirectional streaming for agent connections
- Agent authentication with 256-bit secure tokens
- TLS support for agent connections
- Heartbeat tracking with host status FSM
- Agent daemon implementation
- Agent service installer (systemd/launchd)
- Comprehensive test suite (40+ tests)
- Structured logging with slog

### Security
- Agent authentication with constant-time comparison
- Optional TLS encryption for gRPC
- Comprehensive input validation
- JSON-formatted structured logs

## [0.0.1] - Phases 1-2: Foundation

### Added
- Control plane server (REST API + WebSocket)
- SQLite persistence with WAL mode
- Host management (CRUD operations)
- Session orchestration via SSH
- Git worktree creation on remote hosts
- Tmux session management
- Live output capture via tmux
- React + Vite + Tailwind frontend
- API authentication (Bearer token)
- CORS configuration
- SSH client with known_hosts TOFU

---

## Release Notes

### v0.3.0 - Production Ready

This release marks Swoops as production-ready with enterprise-grade security, observability, and operational features:

**Security Hardening:**
- Full TLS/mTLS support for all connections
- Non-root container execution
- Comprehensive certificate validation
- Security best practices throughout

**Observability:**
- Prometheus metrics with bounded cardinality
- Structured JSON logging
- Health checks and monitoring
- Comprehensive error tracking

**Deployment:**
- Production Docker images
- Kubernetes manifests
- Complete deployment documentation
- Integration test suite

**Critical Fixes:**
- WebSocket support verified and working
- Docker builds functional with workspaces
- Metrics cardinality automatically bounded

**Recommended Upgrade Path:**
1. Review [`PRODUCTION.md`](PRODUCTION.md) for deployment guidelines
2. Generate TLS certificates for production use
3. Update configuration to enable TLS/mTLS
4. Test deployment in staging environment
5. Deploy to production with monitoring enabled

---

## Support

- **Issues**: https://github.com/swoopsh/swoops/issues
- **Documentation**: https://github.com/swoopsh/swoops
- **Security**: security@swoops.sh
