CREATE TABLE IF NOT EXISTS hosts (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    hostname TEXT NOT NULL,
    ssh_port INTEGER NOT NULL DEFAULT 22,
    ssh_user TEXT NOT NULL,
    ssh_key_path TEXT NOT NULL,
    os TEXT NOT NULL DEFAULT '',
    arch TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL DEFAULT 'offline',
    agent_version TEXT DEFAULT '',
    labels_json TEXT DEFAULT '{}',
    max_sessions INTEGER NOT NULL DEFAULT 10,
    base_repo_path TEXT NOT NULL DEFAULT '/opt/swoops/repo',
    worktree_root TEXT NOT NULL DEFAULT '/opt/swoops/worktrees',
    installed_plugins_json TEXT DEFAULT '[]',
    installed_tools_json TEXT DEFAULT '[]',
    last_heartbeat DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    host_id TEXT NOT NULL REFERENCES hosts(id),
    template_id TEXT DEFAULT '',
    agent_type TEXT NOT NULL CHECK(agent_type IN ('claude', 'codex')),
    status TEXT NOT NULL DEFAULT 'pending',
    prompt TEXT NOT NULL DEFAULT '',
    branch_name TEXT NOT NULL,
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

CREATE INDEX IF NOT EXISTS idx_sessions_host_id ON sessions(host_id);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);

CREATE TABLE IF NOT EXISTS plugins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    git_url TEXT NOT NULL,
    description TEXT DEFAULT '',
    version TEXT DEFAULT '',
    type TEXT NOT NULL CHECK(type IN ('mcp-server', 'cli-tool', 'script-bundle')),
    install_spec_json TEXT DEFAULT '{}',
    agent_types_json TEXT DEFAULT '["claude","codex"]',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS session_templates (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    description TEXT DEFAULT '',
    agent_type TEXT NOT NULL CHECK(agent_type IN ('claude', 'codex')),
    model_override TEXT DEFAULT '',
    plugins_json TEXT DEFAULT '[]',
    mcp_servers_json TEXT DEFAULT '[]',
    allowed_tools_json TEXT DEFAULT '[]',
    extra_flags_json TEXT DEFAULT '[]',
    env_vars_json TEXT DEFAULT '{}',
    default_prompt TEXT DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
