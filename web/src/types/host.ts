export type HostStatus = 'online' | 'offline' | 'degraded' | 'provisioning';

export interface InstalledTool {
  name: string;
  version: string;
  path: string;
}

export interface PluginRef {
  name: string;
  version: string;
}

export interface Host {
  id: string;
  name: string;
  hostname: string;
  ssh_port: number;
  ssh_user: string;
  ssh_key_path: string;
  os: string;
  arch: string;
  status: HostStatus;
  agent_version: string;
  agent_user?: string;
  update_available: boolean;
  latest_version?: string;
  update_url?: string;
  labels: Record<string, string>;
  max_sessions: number;
  base_repo_path: string;
  worktree_root: string;
  installed_plugins: PluginRef[];
  installed_tools: InstalledTool[];
  last_heartbeat: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateHostRequest {
  name: string;
  hostname: string;
  ssh_port: number;
  ssh_user: string;
  ssh_key_path: string;
  max_sessions: number;
  labels: Record<string, string>;
  base_repo_path: string;
  worktree_root: string;
}
