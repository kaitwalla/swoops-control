# WebSocket Pusher + REST Refactor Plan

## Executive Summary

**Goal**: Replace gRPC bidirectional streaming with **WebSocket pusher (notifications only) + REST APIs**

**Architecture Pattern**:
- 🔔 **WebSocket**: Server → Agent notifications only (lightweight push)
- 📤 **REST**: Agent → Server for all data (heartbeat, output, results)
- 📥 **REST**: Agent ← Server for polling (commands, status)

**Benefits**:
- ✅ Eliminate ~500 lines of complex goroutine synchronization
- ✅ Remove lock ordering constraints
- ✅ No WebSocket message protocol design needed
- ✅ Standard REST APIs - easy to test with curl
- ✅ Agent can fallback to polling if WebSocket disconnects
- ✅ No bidirectional message routing complexity
- ✅ Native browser debugging support
- ✅ No protobuf code generation

**Estimated Effort**: 1-2 days with 4 parallel workstreams

---

## New Architecture

### Protocol Stack (After Refactor)

```
┌─────────────────────────────────────────────────────────────┐
│                     WEB BROWSER                              │
│  - WebSocket: /api/v1/ws/sessions/:id/output               │
│  - REST: /api/v1/sessions/* (commands)                      │
└─────────────┬───────────────────────────────────────────────┘
              │
              │ WebSocket + REST
              ↓
┌─────────────────────────────────────────────────────────────┐
│           SWOOPS CONTROL PLANE (swoopsd)                    │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ HTTP Server (port 8080)                                │ │
│  │                                                         │ │
│  │  REST API:                                             │ │
│  │   POST /api/v1/agent/heartbeat                         │ │
│  │   GET  /api/v1/agent/commands/pending                  │ │
│  │   POST /api/v1/agent/command-results                   │ │
│  │   POST /api/v1/agent/sessions/:id/output               │ │
│  │                                                         │ │
│  │  WebSocket (notifications only):                       │ │
│  │   /api/v1/ws/agent/connect                            │ │
│  │   /api/v1/ws/sessions/:id/output                      │ │
│  └────────────────────────────────────────────────────────┘ │
│                                                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ Agent Connection Manager (simplified)                  │ │
│  │  - WebSocket conn map (for notifications only)         │ │
│  │  - Pending commands queue (per host)                   │ │
│  │  - Output pub/sub (unchanged)                          │ │
│  │  - Heartbeat tracking (via REST POSTs)                 │ │
│  └────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
              ↑
              │ WS notifications (lightweight)
              │ REST data transfer
              ↓
┌─────────────────────────────────────────────────────────────┐
│            SWOOPS AGENT (swoops-agent)                      │
│                                                              │
│  WebSocket listener:                                         │
│   - wss://server:8080/api/v1/ws/agent/connect              │
│   - Receives: {"type": "new_command"}                      │
│   - Fallback: polls every 5s if WS disconnected            │
│                                                              │
│  REST client:                                                │
│   - POST /heartbeat (every 10s)                             │
│   - GET /commands/pending (when notified or polling)        │
│   - POST /sessions/:id/output (stream output)               │
│   - POST /command-results (after executing)                 │
└─────────────────────────────────────────────────────────────┘
```

---

## Communication Flows

### Flow 1: Agent Heartbeat (Every 10s)

```
Agent                           Server
  |                               |
  | POST /api/v1/agent/heartbeat  |
  |------------------------------>|
  | {                             |
  |   "host_id": "host-123",      |
  |   "running_sessions": 3,      |
  |   "update_available": false   |
  | }                             |
  |                               |
  |        200 OK                 |
  |<------------------------------|
```

**Server updates**: `last_heartbeat` timestamp in DB, checks if host should be marked degraded/offline

---

### Flow 2: Server Sends Command to Agent

```
Server                                    Agent
  |                                        |
  | (User clicks "Launch Session")         |
  |                                        |
  | 1. Store command in pending queue      |
  |                                        |
  | 2. Send WebSocket notification         |
  |     {"type": "new_command"}            |
  |--------------------------------------->|
  |                                        |
  |                                        | 3. Agent receives notification
  |                                        |
  |   GET /api/v1/agent/commands/pending   |
  |<---------------------------------------|
  |                                        |
  | 200 OK                                 |
  | {                                      |
  |   "commands": [                        |
  |     {                                  |
  |       "command_id": "cmd-789",         |
  |       "session_id": "sess-456",        |
  |       "command": "launch_session",     |
  |       "args": {                        |
  |         "session_type": "shell",       |
  |         "work_dir": "~"                |
  |       }                                |
  |     }                                  |
  |   ]                                    |
  | }                                      |
  |--------------------------------------->|
  |                                        |
  |                                        | 4. Agent executes command
  |                                        |
  |  POST /api/v1/agent/command-results    |
  |<---------------------------------------|
  | {                                      |
  |   "command_id": "cmd-789",             |
  |   "session_id": "sess-456",            |
  |   "ok": true,                          |
  |   "message": "session launched"        |
  | }                                      |
  |                                        |
  | 200 OK                                 |
  |--------------------------------------->|
```

**Fallback**: If WebSocket disconnected, agent polls `/commands/pending` every 5s

---

### Flow 3: Agent Streams Session Output

```
Agent                                  Server
  |                                      |
  | (tmux session produces output)       |
  |                                      |
  | POST /api/v1/agent/sessions/sess-456/output
  |------------------------------------->|
  | {                                    |
  |   "content": "$ ls -la\ntotal 24...",|
  |   "eof": false                       |
  | }                                    |
  |                                      |
  |              200 OK                  |
  |<-------------------------------------|
  |                                      |
  |                                      | Server publishes to WebSocket
  |                                      | subscribers (web UI)
```

---

### Flow 4: WebSocket Fallback (Agent Polling)

```
Agent                                  Server
  |                                      |
  | WebSocket connection lost            |
  | ❌                                   |
  |                                      |
  | Switch to polling mode (every 5s)    |
  |                                      |
  | GET /api/v1/agent/commands/pending   |
  |------------------------------------->|
  |                                      |
  | 200 OK {"commands": []}              |
  |<-------------------------------------|
  |                                      |
  | ... 5 seconds later ...              |
  |                                      |
  | GET /api/v1/agent/commands/pending   |
  |------------------------------------->|
  |                                      |
  | Attempt to reconnect WebSocket       |
  | (in background, exponential backoff) |
  |                                      |
  | WebSocket reconnected! ✅            |
  |------------------------------------->|
  |                                      |
  | Stop polling, resume notification mode
```

---

## REST API Specification

### Agent Endpoints (Server-side)

#### `POST /api/v1/agent/heartbeat`

**Purpose**: Agent reports it's alive and sends status update

**Authentication**: Bearer token (host auth_token)

**Request Body**:
```json
{
  "host_id": "host-123",
  "running_sessions": 3,
  "update_available": false,
  "current_version": "1.7.4",
  "latest_version": "1.7.4",
  "update_url": ""
}
```

**Response**: `200 OK` (empty body)

**Server Action**:
- Update `last_heartbeat` timestamp in DB
- Update host status (online/degraded/offline)
- Store update info if provided

---

#### `GET /api/v1/agent/commands/pending`

**Purpose**: Agent polls for pending commands

**Authentication**: Bearer token (host auth_token)

**Query Params**: None (host_id derived from auth token)

**Response**: `200 OK`
```json
{
  "commands": [
    {
      "command_id": "cmd-789",
      "session_id": "sess-456",
      "command": "launch_session",
      "args": {
        "session_type": "shell",
        "work_dir": "~",
        "prompt": "echo 'ready'"
      }
    }
  ]
}
```

**Server Action**:
- Look up host_id from auth token
- Return all pending commands for this host (FIFO queue)
- Mark commands as "delivered" (move to in-flight state)

**Note**: Returns empty array if no pending commands

---

#### `POST /api/v1/agent/command-results`

**Purpose**: Agent sends command execution result

**Authentication**: Bearer token (host auth_token)

**Request Body**:
```json
{
  "command_id": "cmd-789",
  "session_id": "sess-456",
  "ok": true,
  "message": "session launched successfully"
}
```

**Response**: `200 OK` (empty body)

**Server Action**:
- Resolve pending command promise (if waiting)
- Update command status in DB
- Update session status if applicable

---

#### `POST /api/v1/agent/sessions/:session_id/output`

**Purpose**: Agent streams session output to server

**Authentication**: Bearer token (host auth_token)

**Request Body**:
```json
{
  "content": "$ ls -la\ntotal 24\ndrwxr-xr-x...",
  "eof": false
}
```

**Response**: `200 OK` (empty body)

**Server Action**:
- Append to session output in DB
- Publish to WebSocket subscribers (web UI)
- If `eof: true`, mark session as completed

---

### WebSocket Notification Protocol

#### Agent Connection: `wss://server/api/v1/ws/agent/connect?token={auth_token}`

**Authentication**: Token in query parameter

**Messages (Server → Agent only)**:

```json
{"type": "new_command"}
```
Agent should call `GET /api/v1/agent/commands/pending`

```json
{"type": "ping"}
```
WebSocket keepalive (agent can ignore or respond with pong)

**No messages from Agent → Server** (WebSocket is one-way notification channel)

---

## Parallel Workstreams

### Workstream 1: REST API Endpoints (Server)
**Owner**: 1 developer
**Duration**: 4-6 hours
**Blockers**: None
**Blocks**: Workstream 2 (Agent client)

**Tasks**:
1. Create REST handlers for agent endpoints:
   - `POST /api/v1/agent/heartbeat`
   - `GET /api/v1/agent/commands/pending`
   - `POST /api/v1/agent/command-results`
   - `POST /api/v1/agent/sessions/:id/output`
2. Add authentication middleware (extract host_id from bearer token)
3. Implement pending command queue (in-memory or DB)
4. Wire up to existing session management
5. Add logging and error handling
6. Write unit tests for each endpoint

**Files to Create**:
- `server/internal/api/handlers_agent.go` (new REST handlers)
- `server/internal/api/agent_auth_middleware.go` (auth middleware)
- `server/internal/agentmgr/command_queue.go` (pending command queue)
- `server/internal/agentmgr/service.go` (simplified agent manager)

**Files to Modify**:
- `server/internal/api/router.go` (add new routes)
- `server/cmd/swoopsd/main.go` (swap agentconn for agentmgr)

**Key Code Snippet**:
```go
// handlers_agent.go
func (s *Server) handleAgentHeartbeat(w http.ResponseWriter, r *http.Request) {
    var req struct {
        HostID          string `json:"host_id"`
        RunningSessions int    `json:"running_sessions"`
        UpdateAvailable bool   `json:"update_available"`
        CurrentVersion  string `json:"current_version"`
        LatestVersion   string `json:"latest_version"`
        UpdateURL       string `json:"update_url"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request", http.StatusBadRequest)
        return
    }

    // Get host from auth middleware context
    hostID := r.Context().Value("host_id").(string)

    // Update heartbeat timestamp
    if err := s.agentMgr.UpdateHeartbeat(r.Context(), hostID, req); err != nil {
        http.Error(w, "failed to update heartbeat", http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}

func (s *Server) handleGetPendingCommands(w http.ResponseWriter, r *http.Request) {
    hostID := r.Context().Value("host_id").(string)

    commands, err := s.agentMgr.GetPendingCommands(r.Context(), hostID)
    if err != nil {
        http.Error(w, "failed to get commands", http.StatusInternalServerError)
        return
    }

    json.NewEncoder(w).Encode(map[string]interface{}{
        "commands": commands,
    })
}
```

---

### Workstream 2: Agent REST Client
**Owner**: 1 developer
**Duration**: 4-6 hours
**Blockers**: Workstream 1 (REST API must be defined)
**Blocks**: Workstream 5 (integration testing)

**Tasks**:
1. Replace gRPC client with HTTP client
2. Implement heartbeat loop (POST every 10s)
3. Implement command polling (on notification or fallback timer)
4. Implement output streaming (POST as output arrives)
5. Implement command result sending
6. Add error handling and retries
7. Keep existing command handlers (launch/stop/input)
8. Add logging

**Files to Modify**:
- `agent/cmd/swoops-agent/main.go` (replace connectAndRun)
- Create helper functions for REST calls

**Key Code Snippet**:
```go
// agent/cmd/swoops-agent/main.go

type agentHTTPClient struct {
    baseURL    string
    httpClient *http.Client
    token      string
    hostID     string
}

func (c *agentHTTPClient) sendHeartbeat(ctx context.Context, status HeartbeatStatus) error {
    body := map[string]interface{}{
        "host_id":          c.hostID,
        "running_sessions": status.RunningSessions,
        "update_available": status.UpdateAvailable,
        "current_version":  status.CurrentVersion,
        "latest_version":   status.LatestVersion,
        "update_url":       status.UpdateURL,
    }

    return c.postJSON(ctx, "/api/v1/agent/heartbeat", body, nil)
}

func (c *agentHTTPClient) pollCommands(ctx context.Context) ([]Command, error) {
    var resp struct {
        Commands []Command `json:"commands"`
    }

    if err := c.getJSON(ctx, "/api/v1/agent/commands/pending", &resp); err != nil {
        return nil, err
    }

    return resp.Commands, nil
}

func (c *agentHTTPClient) sendOutput(ctx context.Context, sessionID, content string, eof bool) error {
    body := map[string]interface{}{
        "content": content,
        "eof":     eof,
    }

    url := fmt.Sprintf("/api/v1/agent/sessions/%s/output", sessionID)
    return c.postJSON(ctx, url, body, nil)
}

func (c *agentHTTPClient) postJSON(ctx context.Context, path string, body, resp interface{}) error {
    data, err := json.Marshal(body)
    if err != nil {
        return err
    }

    req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+path, bytes.NewReader(data))
    if err != nil {
        return err
    }

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+c.token)

    httpResp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer httpResp.Body.Close()

    if httpResp.StatusCode != http.StatusOK {
        return fmt.Errorf("unexpected status: %d", httpResp.StatusCode)
    }

    if resp != nil {
        return json.NewDecoder(httpResp.Body).Decode(resp)
    }

    return nil
}

// Main agent loop
func runAgent(ctx context.Context, serverAddr, hostID, token string) error {
    client := &agentHTTPClient{
        baseURL:    fmt.Sprintf("https://%s", serverAddr),
        httpClient: &http.Client{Timeout: 30 * time.Second},
        token:      token,
        hostID:     hostID,
    }

    rt := &agentRuntime{
        client:   client,
        sessions: make(map[string]*sessionRuntime),
    }

    // Start WebSocket notification listener (with reconnection)
    go rt.listenForNotifications(ctx, serverAddr, token)

    // Heartbeat ticker
    heartbeatTicker := time.NewTicker(10 * time.Second)
    defer heartbeatTicker.Stop()

    // Fallback polling ticker (if WS disconnected)
    pollTicker := time.NewTicker(5 * time.Second)
    defer pollTicker.Stop()

    for {
        select {
        case <-ctx.Done():
            return nil

        case <-heartbeatTicker.C:
            status := rt.getHeartbeatStatus()
            if err := client.sendHeartbeat(ctx, status); err != nil {
                log.Printf("heartbeat failed: %v", err)
            }

        case <-rt.commandNotification:
            // WebSocket notification received
            rt.pollAndExecuteCommands(ctx)

        case <-pollTicker.C:
            // Fallback polling if WebSocket disconnected
            if !rt.wsConnected {
                rt.pollAndExecuteCommands(ctx)
            }
        }
    }
}

func (rt *agentRuntime) pollAndExecuteCommands(ctx context.Context) {
    commands, err := rt.client.pollCommands(ctx)
    if err != nil {
        log.Printf("poll commands failed: %v", err)
        return
    }

    for _, cmd := range commands {
        go rt.executeCommand(ctx, cmd)
    }
}
```

---

### Workstream 3: WebSocket Notification Handler (Server)
**Owner**: 1 developer
**Duration**: 3-4 hours
**Blockers**: None
**Blocks**: Workstream 4 (Agent WS listener)

**Tasks**:
1. Create WebSocket endpoint `/api/v1/ws/agent/connect`
2. Implement authentication (token from query param)
3. Store WebSocket connection in agent manager
4. Implement notification sending (when command queued)
5. Implement connection monitoring (close stale connections)
6. Handle disconnections gracefully
7. Add reconnection tracking

**Files to Create**:
- `server/internal/agentmgr/websocket.go` (WS notification handler)

**Files to Modify**:
- `server/internal/api/router.go` (add WS route)
- `server/internal/agentmgr/service.go` (store WS connections)

**Key Code Snippet**:
```go
// agentmgr/websocket.go

type AgentManager struct {
    wsConns   map[string]*websocket.Conn
    wsConnsMu sync.RWMutex

    pendingCommands map[string][]*Command
    pendingMu       sync.Mutex
}

func (s *Server) handleAgentWebSocket(w http.ResponseWriter, r *http.Request) {
    // 1. Authenticate
    token := r.URL.Query().Get("token")
    host, err := s.authenticateAgentToken(r.Context(), token)
    if err != nil {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. Upgrade to WebSocket
    conn, err := s.wsUpgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("websocket upgrade failed: %v", err)
        return
    }
    defer conn.Close()

    // 3. Register connection
    s.agentMgr.RegisterWebSocket(host.ID, conn)
    defer s.agentMgr.UnregisterWebSocket(host.ID)

    log.Printf("agent %s connected via websocket", host.ID)

    // 4. Keep connection alive (read loop, ignore messages)
    for {
        _, _, err := conn.ReadMessage()
        if err != nil {
            log.Printf("websocket read error for %s: %v", host.ID, err)
            return
        }
    }
}

func (am *AgentManager) NotifyNewCommand(hostID string) {
    am.wsConnsMu.RLock()
    conn, ok := am.wsConns[hostID]
    am.wsConnsMu.RUnlock()

    if !ok {
        // No WebSocket, agent will poll via fallback
        return
    }

    msg := map[string]string{"type": "new_command"}
    if err := conn.WriteJSON(msg); err != nil {
        log.Printf("failed to send notification to %s: %v", hostID, err)
        // Close stale connection
        am.UnregisterWebSocket(hostID)
    }
}

func (am *AgentManager) QueueCommand(hostID string, cmd *Command) error {
    am.pendingMu.Lock()
    am.pendingCommands[hostID] = append(am.pendingCommands[hostID], cmd)
    am.pendingMu.Unlock()

    // Notify via WebSocket
    am.NotifyNewCommand(hostID)

    return nil
}

func (am *AgentManager) GetPendingCommands(ctx context.Context, hostID string) ([]*Command, error) {
    am.pendingMu.Lock()
    defer am.pendingMu.Unlock()

    commands := am.pendingCommands[hostID]
    am.pendingCommands[hostID] = nil // Clear after retrieving

    return commands, nil
}
```

---

### Workstream 4: Agent WebSocket Notification Listener
**Owner**: 1 developer
**Duration**: 2-3 hours
**Blockers**: Workstream 3 (server WS handler)
**Blocks**: Workstream 5 (integration testing)

**Tasks**:
1. Connect WebSocket to server on agent startup
2. Listen for notification messages
3. Trigger command polling when notified
4. Implement reconnection with exponential backoff
5. Track connection state (for fallback polling)
6. Handle graceful shutdown

**Files to Modify**:
- `agent/cmd/swoops-agent/main.go` (add WS listener goroutine)

**Key Code Snippet**:
```go
// agent/cmd/swoops-agent/main.go

func (rt *agentRuntime) listenForNotifications(ctx context.Context, serverAddr, token string) {
    backoff := time.Second

    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        err := rt.connectWebSocket(ctx, serverAddr, token)
        if err == nil {
            return // Clean shutdown
        }

        log.Printf("websocket connection lost: %v (retry in %s)", err, backoff)

        rt.wsConnected = false
        time.Sleep(backoff)

        if backoff < 30*time.Second {
            backoff *= 2
            if backoff > 30*time.Second {
                backoff = 30 * time.Second
            }
        }
    }
}

func (rt *agentRuntime) connectWebSocket(ctx context.Context, serverAddr, token string) error {
    wsURL := fmt.Sprintf("wss://%s/api/v1/ws/agent/connect?token=%s", serverAddr, url.QueryEscape(token))

    conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
    if err != nil {
        return fmt.Errorf("dial: %w", err)
    }
    defer conn.Close()

    rt.wsConnected = true
    log.Printf("websocket connected")

    // Read loop - just listen for notifications
    for {
        var msg map[string]interface{}
        if err := conn.ReadJSON(&msg); err != nil {
            return fmt.Errorf("read: %w", err)
        }

        msgType, _ := msg["type"].(string)
        if msgType == "new_command" {
            // Notify main loop to poll for commands
            select {
            case rt.commandNotification <- struct{}{}:
            default:
                // Already has notification pending, skip
            }
        }
    }
}
```

---

### Workstream 5: Web UI Improvements (PARALLEL)
**Owner**: 1 developer
**Duration**: 3-4 hours
**Blockers**: None (can work independently)
**Blocks**: None

**Tasks**:
1. Add WebSocket reconnection logic to `TerminalOutput.tsx`
2. Implement exponential backoff (1s → 30s max)
3. Add connection state indicator (connected/reconnecting/offline)
4. Remove polling fallback (WebSocket only)
5. Add error notifications to user
6. Add retry counter

**Files to Modify**:
- `web/src/components/TerminalOutput.tsx`
- `web/src/hooks/useReconnectingWebSocket.ts` (create reusable hook)

**Same as original plan** - Web UI changes are independent

---

### Workstream 6: Integration Testing & Cleanup (LAST)
**Owner**: 1-2 developers
**Duration**: 3-4 hours
**Blockers**: All previous workstreams
**Blocks**: Production deployment

**Tasks**:
1. Test agent connection via REST + WebSocket
2. Test session launch end-to-end
3. Test output streaming
4. Test command execution
5. Test WebSocket fallback (disconnect and verify polling works)
6. Test agent reconnection
7. Test heartbeat monitoring
8. Remove gRPC infrastructure
9. Update documentation

**Files to Delete**:
- `proto/swoops/agent.proto`
- `server/internal/agentconn/` (entire directory)
- `pkg/agentrpc/` (if not reusing types)

**Files to Modify**:
- `server/cmd/swoopsd/main.go` (remove gRPC server, lines 136-190)
- `go.mod` (remove gRPC dependencies)
- `Makefile` (remove protoc)

---

## Code Complexity Comparison

### Before (gRPC Bidirectional Streaming)

```
server/internal/agentconn/service.go:  671 lines
  - 3 mutexes with lock ordering constraint
  - 2 goroutines per connection (send + recv)
  - Complex channel management
  - Command/response matching via channels
  - Protobuf marshaling

agent/cmd/swoops-agent/main.go:  ~300 lines
  - gRPC stream setup
  - Send/recv goroutine coordination
  - Protobuf marshaling
  - Complex error propagation
```

### After (REST + WebSocket Notifications)

```
server/internal/agentmgr/service.go:  ~200 lines
  - 2 mutexes (simple, no ordering constraint)
  - 1 goroutine per connection (WS read loop)
  - Simple command queue (slice)
  - No channel management needed
  - JSON marshaling

server/internal/api/handlers_agent.go:  ~150 lines
  - 4 REST handlers
  - Standard HTTP patterns
  - JSON marshaling

agent/cmd/swoops-agent/main.go:  ~200 lines
  - HTTP client (standard library)
  - 1 goroutine for WS listener
  - Simple polling logic
  - JSON marshaling
  - No complex error handling
```

**Total Reduction**: ~671 + 300 = 971 lines → ~200 + 150 + 200 = 550 lines (**43% reduction**)

---

## Migration Strategy

### Phase 1: Development (Parallel)
- **Day 1 AM**: Workstreams 1, 3 (REST API + WS notification handler)
- **Day 1 PM**: Workstreams 2, 4 (Agent REST client + WS listener)
- **Day 2 AM**: Workstream 5 (Web UI improvements)
- **Day 2 PM**: Workstream 6 (Integration testing + cleanup)

### Phase 2: Deployment
1. Tag new version (v2.0.0)
2. Deploy server (REST endpoints + WS notification handler)
3. Agents auto-update (existing mechanism)
4. Monitor for 24 hours
5. Remove gRPC code in v2.1.0 (cleanup)

### Rollback Plan
- Keep gRPC code in `legacy/grpc-backup` branch
- Can rollback server to v1.x if issues
- Agents can rollback via manual download

---

## Benefits Summary

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| **Total code** | 971 lines | 550 lines | **43% reduction** |
| **Mutexes** | 3 with ordering | 2 simple | **Simpler** |
| **Goroutines** | 2 per connection | 1 per connection | **Simpler** |
| **Protocols** | 3 (gRPC, WS, REST) | 2 (WS, REST) | **33% reduction** |
| **Message routing** | Complex envelopes | Simple REST | **Much simpler** |
| **Testing** | Need gRPC mocks | Standard HTTP mocks | **Easier** |
| **Debugging** | grpcurl/protoc | curl, browser tools | **Native** |
| **Fallback** | Manual reconnect | Auto-polling | **More resilient** |

---

## Risk Mitigation

### Risk: REST Overhead (More HTTP Requests)
**Mitigation**:
- Heartbeat is only 10s interval
- Output streaming is batched (200ms intervals)
- Command polling is event-driven (WebSocket notification)
- Fallback polling is 5s (only when WS disconnected)

### Risk: WebSocket Notification Loss
**Mitigation**:
- Agent polls every 5s as fallback if WS disconnected
- Agent polls on reconnect to catch missed commands
- Commands are queued server-side until retrieved

### Risk: Higher Latency for Commands
**Mitigation**:
- WebSocket notification is instant
- Agent polls immediately on notification
- Total latency: <100ms (vs gRPC stream: ~50ms) - acceptable

---

## Open Questions

1. **Command queue storage**: In-memory (simple) vs DB (persistent)?
   - **Recommendation**: In-memory (simpler), with DB persistence for audit trail

2. **Output batching**: Should agent batch output before POSTing?
   - **Recommendation**: Yes, batch 200ms intervals (reduce HTTP overhead)

3. **Auth token**: Reuse existing host.auth_token for Bearer auth?
   - **Recommendation**: Yes (simpler, already have it)

4. **WebSocket keepalive**: How often should server send ping?
   - **Recommendation**: Every 30s (standard practice)

5. **Command expiration**: Should pending commands expire after N minutes?
   - **Recommendation**: Yes, 5 minutes (prevent stale commands)

---

## Next Steps

1. ✅ **Review this plan** with team
2. **Create feature branch**: `feat/rest-pusher-refactor`
3. **Assign workstream owners**
4. **Kick off Workstreams 1, 3, 5 in parallel** (server-side)
5. **Then Workstreams 2, 4** (agent-side, after server APIs are defined)
6. **Finally Workstream 6** (integration + cleanup)
7. **Deploy to staging**
8. **Production rollout**

---

**Author**: Claude (AI Assistant)
**Date**: 2026-03-11
**Status**: READY FOR EXECUTION
**Pattern**: WebSocket Pusher + REST (Simplified)
