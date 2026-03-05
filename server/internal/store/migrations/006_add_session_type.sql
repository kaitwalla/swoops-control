-- Add session type column to support different session types (agent vs shell)
ALTER TABLE sessions ADD COLUMN type TEXT NOT NULL DEFAULT 'agent' CHECK(type IN ('agent', 'shell'));

-- Recreate sessions table to make agent_type, prompt, and branch_name optional
-- SQLite doesn't support modifying constraints, so we need to recreate the table

-- Step 1: Create new table with updated schema
CREATE TABLE sessions_new (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    host_id TEXT NOT NULL REFERENCES hosts(id),
    template_id TEXT DEFAULT '',
    type TEXT NOT NULL DEFAULT 'agent' CHECK(type IN ('agent', 'shell')),
    agent_type TEXT DEFAULT '' CHECK(agent_type IN ('', 'claude', 'codex')),
    status TEXT NOT NULL DEFAULT 'pending',
    prompt TEXT DEFAULT '',
    branch_name TEXT DEFAULT '',
    worktree_path TEXT DEFAULT '',
    tmux_session TEXT DEFAULT '',
    agent_pid INTEGER DEFAULT 0,
    model_override TEXT DEFAULT '',
    env_vars_json TEXT DEFAULT '{}',
    mcp_servers_json TEXT DEFAULT '[]',
    plugins_json TEXT DEFAULT '[]',
    allowed_tools_json TEXT DEFAULT '[]',
    extra_flags_json TEXT DEFAULT '[]',
    last_output TEXT DEFAULT '',
    started_at DATETIME,
    stopped_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Step 2: Copy data from old table to new table
INSERT INTO sessions_new
SELECT
    id, name, host_id, template_id, 'agent', agent_type, status, prompt, branch_name,
    worktree_path, tmux_session, agent_pid, model_override, env_vars_json,
    mcp_servers_json, plugins_json, allowed_tools_json, extra_flags_json,
    last_output, started_at, stopped_at, created_at, updated_at
FROM sessions;

-- Step 3: Drop old table
DROP TABLE sessions;

-- Step 4: Rename new table to sessions
ALTER TABLE sessions_new RENAME TO sessions;

-- Step 5: Recreate indexes
CREATE INDEX IF NOT EXISTS idx_sessions_host_id ON sessions(host_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_type ON sessions(type);
