import { useState } from 'react';
import { X } from 'lucide-react';
import type { CreateHostRequest } from '../types/host';

interface Props {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: CreateHostRequest) => Promise<void>;
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

  if (!open) return null;

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
      <div className="bg-gray-900 border border-gray-800 rounded-lg w-full max-w-lg p-6">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">Add Host</h2>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-300">
            <X size={18} />
          </button>
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
