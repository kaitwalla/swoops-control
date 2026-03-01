# MCP Bridge Usage Guide

The Swoops MCP Bridge enables AI agents (Claude Code, Codex) to communicate with the control plane and coordinate with each other.

## Overview

When you create a session, Swoops automatically generates an MCP configuration file (`.mcp.json` for Claude Code, `.codex/mcp.json` for Codex) that connects the agent to the control plane.

The agent gains access to 4 MCP tools:
- `report_status` - Report what you're working on
- `get_task` - Retrieve tasks assigned by humans
- `request_review` - Request human code review
- `coordinate_with_session` - Message other AI agent sessions

## MCP Tools

### report_status

Report your current status and progress to the control plane.

**Parameters:**
- `status_type` (required): "working" | "idle" | "blocked" | "completed" | "error"
- `message` (required): Human-readable status message
- `details` (optional): Additional context (file paths, line numbers, etc.)

**Example:**
```
report_status(
  status_type="working",
  message="Refactoring authentication module",
  details={"file": "auth/oauth.go", "line": 127}
)
```

**Use cases:**
- Let humans know what you're currently working on
- Report when you're blocked and need help
- Announce task completion
- Report errors for human intervention

### get_task

Retrieve the next pending task from the control plane.

**Parameters:** None

**Returns:** Task object with:
- `task_type`: "instruction" | "fix" | "review" | "refactor" | "test"
- `priority`: Higher numbers = higher priority
- `title`: Short task title
- `description`: Detailed task description
- `context`: Optional additional context

**Example:**
```
task = get_task()
# Returns: {"title": "Fix memory leak", "description": "...", "priority": 10}
```

**Use cases:**
- Check for work assigned by humans
- Get prioritized instructions
- Receive fix requests or refactoring tasks

### request_review

Request human review of your work before proceeding.

**Parameters:**
- `request_type` (required): "code" | "architecture" | "security" | "performance"
- `title` (required): Short review title
- `description` (required): What needs review
- `file_paths` (optional): List of file paths
- `diff` (optional): Git diff or code snippet

**Example:**
```
request_review(
  request_type="security",
  title="Review OAuth implementation",
  description="Please review the new authentication flow for security issues",
  file_paths=["auth/oauth.go", "auth/middleware.go"],
  diff="diff --git a/auth/oauth.go ..."
)
```

**Use cases:**
- Get approval before deploying code
- Request security review of sensitive changes
- Get feedback on architectural decisions
- Verify performance optimizations

### coordinate_with_session

Send a message to another AI agent session.

**Parameters:**
- `to_session_id` (required): Target session ID
- `message_type` (required): "question" | "info" | "request" | "response"
- `subject` (required): Message subject
- `body` (required): Message content
- `context` (optional): Additional context

**Example:**
```
coordinate_with_session(
  to_session_id="sess-abc123",
  message_type="question",
  subject="API endpoint design",
  body="How should we structure the /users endpoint? RESTful or GraphQL?",
  context={"component": "api_server"}
)
```

**Use cases:**
- Ask questions to agents working on related components
- Share information between agents
- Request collaboration on complex tasks
- Coordinate changes across multiple codebases

## Workflows

### Workflow 1: Task-Based Work

1. **Human** creates a task via Swoops UI
2. **Agent** calls `get_task()` to retrieve it
3. **Agent** reports progress: `report_status(status_type="working", message="Implementing feature X")`
4. **Agent** completes work and requests review: `request_review(...)`
5. **Human** approves/rejects via Swoops UI
6. **Agent** marks complete: `report_status(status_type="completed", message="Feature X ready")`

### Workflow 2: Multi-Agent Coordination

1. **Agent A** working on backend API
2. **Agent B** working on frontend
3. **Agent B** needs API endpoint info: `coordinate_with_session(to_session_id=agent_a_session, ...)`
4. **Agent A** receives message, responds with details
5. **Both agents** can continue work with shared context

### Workflow 3: Blocked on Human Input

1. **Agent** encounters architectural decision
2. **Agent** reports: `report_status(status_type="blocked", message="Need decision on database choice")`
3. **Human** sees blocked status in UI
4. **Human** creates task with decision
5. **Agent** calls `get_task()` and continues

## Viewing MCP Activity

In the Swoops UI session detail page, view MCP activity in tabs:

- **Agent Status**: Real-time status updates from the agent
- **Tasks**: Create and manage tasks for the agent
- **Review Requests**: Approve/reject agent's review requests
- **Messages**: View inter-session communication

## Configuration

MCP config is automatically generated during session launch. Example `.mcp.json`:

```json
{
  "mcpServers": {
    "swoops-orchestrator": {
      "command": "swoops-agent",
      "args": [
        "mcp-serve",
        "--session-id", "sess-abc123",
        "--server", "http://localhost:8080"
      ],
      "env": {
        "SWOOPS_API_KEY": "your-api-key"
      }
    }
  }
}
```

## Best Practices

1. **Report status frequently** - Keep humans informed of progress
2. **Use meaningful messages** - Clear, descriptive status messages
3. **Include context** - Add file paths, line numbers, error details
4. **Request reviews early** - Don't wait until everything is done
5. **Coordinate proactively** - Message other agents before conflicts arise
6. **Check for tasks regularly** - Poll `get_task()` periodically
7. **Use appropriate message types** - "question" vs "info" vs "request"

## Troubleshooting

**MCP tools not available:**
- Verify `.mcp.json` exists in worktree
- Check agent has `swoops-agent` binary in PATH
- Verify `SWOOPS_API_KEY` environment variable is set

**Tools returning errors:**
- Check control plane is running and accessible
- Verify session ID is correct
- Check API key is valid

**Messages not being delivered:**
- Verify target session exists and is running
- Check session IDs are correct
- Review session messages tab in UI
