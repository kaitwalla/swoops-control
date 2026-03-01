-- Phase 4: MCP Bridge tables

-- Agent status updates reported via MCP report_status tool
CREATE TABLE IF NOT EXISTS agent_status_updates (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    status_type TEXT NOT NULL CHECK(status_type IN ('working', 'idle', 'blocked', 'completed', 'error')),
    message TEXT NOT NULL,
    details_json TEXT DEFAULT '{}', -- Additional context (current file, line number, etc.)
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_agent_status_session_id ON agent_status_updates(session_id);
CREATE INDEX IF NOT EXISTS idx_agent_status_created_at ON agent_status_updates(created_at);

-- Tasks that can be assigned to sessions via MCP get_task tool
CREATE TABLE IF NOT EXISTS session_tasks (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    task_type TEXT NOT NULL CHECK(task_type IN ('instruction', 'fix', 'review', 'refactor', 'test')),
    priority INTEGER NOT NULL DEFAULT 0, -- Higher priority tasks returned first
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    context_json TEXT DEFAULT '{}', -- File paths, line numbers, etc.
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'retrieved', 'completed', 'failed')),
    retrieved_at DATETIME,
    completed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_session_tasks_session_id ON session_tasks(session_id);
CREATE INDEX IF NOT EXISTS idx_session_tasks_status ON session_tasks(status);
CREATE INDEX IF NOT EXISTS idx_session_tasks_priority ON session_tasks(priority DESC);

-- Code review requests from agents via MCP request_review tool
CREATE TABLE IF NOT EXISTS review_requests (
    id TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    request_type TEXT NOT NULL CHECK(request_type IN ('code', 'architecture', 'security', 'performance')),
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    file_paths_json TEXT DEFAULT '[]', -- Files to review
    diff TEXT DEFAULT '', -- Git diff or code snippet
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending', 'in_review', 'approved', 'changes_requested', 'rejected')),
    reviewer_notes TEXT DEFAULT '',
    reviewed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_review_requests_session_id ON review_requests(session_id);
CREATE INDEX IF NOT EXISTS idx_review_requests_status ON review_requests(status);

-- Messages for session-to-session coordination via MCP coordinate_with_session tool
CREATE TABLE IF NOT EXISTS session_messages (
    id TEXT PRIMARY KEY,
    from_session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    to_session_id TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    message_type TEXT NOT NULL CHECK(message_type IN ('question', 'info', 'request', 'response')),
    subject TEXT NOT NULL,
    body TEXT NOT NULL,
    context_json TEXT DEFAULT '{}',
    status TEXT NOT NULL DEFAULT 'sent' CHECK(status IN ('sent', 'read', 'responded')),
    read_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_session_messages_from ON session_messages(from_session_id);
CREATE INDEX IF NOT EXISTS idx_session_messages_to ON session_messages(to_session_id);
CREATE INDEX IF NOT EXISTS idx_session_messages_status ON session_messages(status);
