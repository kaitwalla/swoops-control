import { useState, useEffect } from 'react';
import { X, Copy, Check } from 'lucide-react';
import type { CreateHostRequest } from '../types/host';
import { api } from '../api/client';

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: CreateHostRequest) => Promise<void>;
}

interface ServerInfo {
  grpc_address: string;
  grpc_secure: boolean;
  http_url: string;
  setup_command: string;
}

export function CreateHostDialog({ open, onClose, onSubmit }: Props) {
  const [form, setForm] = useState<CreateHostRequest>({
    name: '',
    hostname: '',
    ssh_port: 22,
    ssh_user: '',
    ssh_key_path: '',
    max_sessions: 10,
    labels: {},
    base_repo_path: '/opt/swoops/repo',
    worktree_root: '/opt/swoops/worktrees',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [serverInfo, setServerInfo] = useState<ServerInfo | null>(null);
  const [copied, setCopied] = useState(false);
  const [agentSetupCommand, setAgentSetupCommand] = useState<string | null>(null);
  const [generatingAgent, setGeneratingAgent] = useState(false);

  useEffect(() => {
    if (open) {
      api.get<ServerInfo>('/server-info')
        .then(setServerInfo)
        .catch(err => console.error('Failed to fetch server info:', err));
      // Reset agent setup command when dialog opens
      setAgentSetupCommand(null);
    }
  }, [open]);

  const generateAgentSetup = async () => {
    setGeneratingAgent(true);
    setError('');
    try {
      // Create a new agent host
      const hostName = `agent-${Date.now()}`;
      const response = await api.post<{
        id: string;
        name: string;
        auth_token: string;
        client_cert?: string;
        client_key?: string;
      }>('/hosts/agent', {
        name: hostName
      });

      // Build setup command with credentials (add timestamp to bypass caching)
      const timestamp = Date.now();
      let setupCmd = `curl -fsSL "https://raw.githubusercontent.com/kaitwalla/swoops-control/main/setup.sh?${timestamp}" | bash -s --`;

      // Fetch server info to get server address
      const serverInfo = await api.get<ServerInfo>('/server-info');
      setupCmd += ` --server ${serverInfo.grpc_address}`;
      setupCmd += ` --host-id ${response.id}`;
      setupCmd += ` --auth-token ${response.auth_token}`;

      if (serverInfo.grpc_secure) {
        setupCmd += ` --download-ca --http-url ${serverInfo.http_url}`;

        // If we have client cert/key, we need to save them first
        if (response.client_cert && response.client_key) {
          setupCmd += ` # Note: Client certificates will be downloaded during setup`;
        }
      }

      setAgentSetupCommand(setupCmd);
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setGeneratingAgent(false);
    }
  };

  if (!open) return null;

  const copyToClipboard = async (text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      await onSubmit(form);
      onClose();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setLoading(false);
    }
  };

  const update = (field: keyof CreateHostRequest, value: unknown) =>
    setForm((prev) => ({ ...prev, [field]: value }));

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-gray-900 border border-gray-800 rounded-lg w-full max-w-2xl p-6 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Add Host</h2>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-300">
            <X size={18} />
          </button>
        </div>

        {serverInfo && (
          <div className="mb-6 p-4 bg-blue-900/20 border border-blue-800 rounded-lg">
            <h3 className="text-sm font-semibold text-blue-400 mb-2">Quick Setup (Agent-based)</h3>
            {!agentSetupCommand ? (
              <>
                <p className="text-xs text-gray-400 mb-3">
                  Click the button below to generate a setup command for a new agent-based host:
                </p>
                <button
                  type="button"
                  onClick={generateAgentSetup}
                  disabled={generatingAgent}
                  className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 rounded disabled:opacity-50"
                >
                  {generatingAgent ? 'Generating...' : 'Generate Agent Setup Command'}
                </button>
              </>
            ) : (
              <>
                <p className="text-xs text-gray-400 mb-3">
                  Run this command on your remote machine to automatically install and configure the agent:
                </p>
                <div className="flex items-start gap-2 bg-gray-950 rounded p-3 font-mono text-xs">
                  <code className="flex-1 break-all text-gray-300">{agentSetupCommand}</code>
                  <button
                    type="button"
                    onClick={() => copyToClipboard(agentSetupCommand)}
                    className="flex-shrink-0 p-1.5 hover:bg-gray-800 rounded transition-colors"
                    title="Copy to clipboard"
                  >
                    {copied ? (
                      <Check size={14} className="text-green-400" />
                    ) : (
                      <Copy size={14} className="text-gray-400" />
                    )}
                  </button>
                </div>
                <p className="text-xs text-gray-500 mt-2">
                  After setup, the agent will connect to: <span className="text-gray-400 font-mono">{serverInfo.grpc_address}</span>
                </p>
              </>
            )}
          </div>
        )}

        <div className="mb-4 pb-4 border-b border-gray-800">
          <h3 className="text-sm font-semibold text-gray-400">Or configure manually:</h3>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <label className="block">
              <span className="text-sm text-gray-400">Name</span>
              <input
                className="mt-1 w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                value={form.name}
                onChange={(e) => update('name', e.target.value)}
                placeholder="gpu-box-1"
                required
              />
            </label>
            <label className="block">
              <span className="text-sm text-gray-400">Hostname / IP</span>
              <input
                className="mt-1 w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                value={form.hostname}
                onChange={(e) => update('hostname', e.target.value)}
                placeholder="10.0.1.50"
                required
              />
            </label>
          </div>

          <div className="grid grid-cols-3 gap-4">
            <label className="block">
              <span className="text-sm text-gray-400">SSH User</span>
              <input
                className="mt-1 w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                value={form.ssh_user}
                onChange={(e) => update('ssh_user', e.target.value)}
                placeholder="deploy"
                required
              />
            </label>
            <label className="block">
              <span className="text-sm text-gray-400">SSH Port</span>
              <input
                type="number"
                className="mt-1 w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                value={form.ssh_port}
                onChange={(e) => update('ssh_port', parseInt(e.target.value))}
              />
            </label>
            <label className="block">
              <span className="text-sm text-gray-400">Max Sessions</span>
              <input
                type="number"
                className="mt-1 w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                value={form.max_sessions}
                onChange={(e) => update('max_sessions', parseInt(e.target.value))}
              />
            </label>
          </div>

          <label className="block">
            <span className="text-sm text-gray-400">SSH Key Path (on control plane)</span>
            <input
              className="mt-1 w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
              value={form.ssh_key_path}
              onChange={(e) => update('ssh_key_path', e.target.value)}
              placeholder="/etc/swoops/keys/host.pem"
              required
            />
          </label>

          <div className="grid grid-cols-2 gap-4">
            <label className="block">
              <span className="text-sm text-gray-400">Base Repo Path</span>
              <input
                className="mt-1 w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                value={form.base_repo_path}
                onChange={(e) => update('base_repo_path', e.target.value)}
              />
            </label>
            <label className="block">
              <span className="text-sm text-gray-400">Worktree Root</span>
              <input
                className="mt-1 w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                value={form.worktree_root}
                onChange={(e) => update('worktree_root', e.target.value)}
              />
            </label>
          </div>

          {error && <p className="text-sm text-red-400">{error}</p>}

          <div className="flex justify-end gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-400 hover:text-gray-200"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading}
              className="px-4 py-2 text-sm bg-blue-600 hover:bg-blue-500 rounded disabled:opacity-50"
            >
              {loading ? 'Adding...' : 'Add Host'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
