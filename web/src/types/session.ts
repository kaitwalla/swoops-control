export type AgentType = 'claude' | 'codex';
export type SessionType = 'agent' | 'shell';
export type SessionStatus = 'pending' | 'starting' | 'running' | 'idle' | 'stopping' | 'stopped' | 'failed';

export interface Session {
  id: string;
  name: string;
  host_id: string;
  template_id: string;
  type: SessionType;
  agent_type?: AgentType;
  status: SessionStatus;
  prompt?: string;
  branch_name?: string;
  worktree_path?: string;
  tmux_session: string;
  agent_pid: number;
  model_override?: string;
  env_vars: Record<string, string>;
  mcp_servers: unknown[];
  plugins: string[];
  allowed_tools: string[];
  extra_flags: string[];
  last_output: string;
  started_at: string | null;
  stopped_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateSessionRequest {
  name?: string;
  host_id: string;
  type?: SessionType;
  agent_type?: AgentType;
  prompt?: string;
  branch_name?: string;
  template_id?: string;
  model_override?: string;
  env_vars?: Record<string, string>;
  plugins?: string[];
  allowed_tools?: string[];
  extra_flags?: string[];
}
