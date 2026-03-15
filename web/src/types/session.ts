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
  working_directory?: string;  // Custom working directory (alternative to worktree)
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

export type DirectorySourceType = 'existing' | 'new_folder' | 'clone_repo' | 'new_repo';

export interface DirectorySource {
  type: DirectorySourceType;
  existing_path?: string;       // For type='existing'
  new_folder_name?: string;      // For type='new_folder'
  repo_url?: string;             // For type='clone_repo'
  repo_name?: string;            // For type='new_repo'
  repo_description?: string;     // For type='new_repo'
  repo_private?: boolean;        // For type='new_repo'
  clone_folder_name?: string;    // Optional custom folder name for cloned repos
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
  working_directory?: string;    // Custom working directory path
  directory_source?: DirectorySource;  // How to set up the working directory
}
