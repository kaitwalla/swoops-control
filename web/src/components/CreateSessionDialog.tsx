import { useState, useEffect } from 'react';
import { X } from 'lucide-react';
import type { CreateSessionRequest, AgentType } from '../types/session';
import type { Host } from '../types/host';
import { hostsApi } from '../api/hosts';

interface CreateSessionDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: CreateSessionRequest) => Promise<void>;
  preselectedHostId?: string;
}

export function CreateSessionDialog({ open, onClose, onSubmit, preselectedHostId }: CreateSessionDialogProps) {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [hostId, setHostId] = useState(preselectedHostId || '');
  const [agentType, setAgentType] = useState<AgentType>('claude');
  const [prompt, setPrompt] = useState('');
  const [name, setName] = useState('');
  const [branchName, setBranchName] = useState('');
  const [modelOverride, setModelOverride] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (open) {
      hostsApi.list().then(setHosts).catch(() => {});
      if (preselectedHostId) setHostId(preselectedHostId);
    }
  }, [open, preselectedHostId]);

  if (!open) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      await onSubmit({
        host_id: hostId,
        agent_type: agentType,
        prompt,
        name: name || undefined,
        branch_name: branchName || undefined,
        model_override: modelOverride || undefined,
      });
      // Reset form
      setPrompt('');
      setName('');
      setBranchName('');
      setModelOverride('');
      onClose();
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-gray-900 border border-gray-700 rounded-lg w-full max-w-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Create Session</h2>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-300">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-gray-400 mb-1">Host *</label>
              <select
                value={hostId}
                onChange={(e) => setHostId(e.target.value)}
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                required
              >
                <option value="">Select host...</option>
                {hosts.map((h) => (
                  <option key={h.id} value={h.id}>
                    {h.name} ({h.hostname})
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Agent Type *</label>
              <select
                value={agentType}
                onChange={(e) => setAgentType(e.target.value as AgentType)}
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
              >
                <option value="claude">Claude Code</option>
                <option value="codex">Codex</option>
              </select>
            </div>
          </div>

          <div>
            <label className="block text-sm text-gray-400 mb-1">Prompt *</label>
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm min-h-[100px]"
              placeholder="Describe the task for the AI agent..."
              required
            />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-gray-400 mb-1">Session Name</label>
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                placeholder="Auto-generated if empty"
              />
            </div>
            <div>
              <label className="block text-sm text-gray-400 mb-1">Branch Name</label>
              <input
                value={branchName}
                onChange={(e) => setBranchName(e.target.value)}
                className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                placeholder="swoops/<session-name>"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm text-gray-400 mb-1">Model Override</label>
            <input
              value={modelOverride}
              onChange={(e) => setModelOverride(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
              placeholder="e.g., claude-sonnet-4-20250514"
            />
          </div>

          {error && <p className="text-red-400 text-sm">{error}</p>}

          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-400 hover:text-gray-200"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !hostId || !prompt.trim()}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? 'Creating...' : 'Create Session'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
