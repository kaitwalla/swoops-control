import type { Host } from '../types/host';
import { StatusBadge } from './StatusBadge';
import { Server, Trash2, Download, RefreshCw } from 'lucide-react';
import { Link } from 'react-router-dom';
import { useState } from 'react';

interface HostCardProps {
  host: Host;
  sessionCount: number;
  onDelete: (id: string) => void;
  onUpdate: (id: string) => Promise<void>;
  onCheckForUpdates: (id: string) => Promise<void>;
}

export function HostCard({ host, sessionCount, onDelete, onUpdate, onCheckForUpdates }: HostCardProps) {
  const [updating, setUpdating] = useState(false);
  const [checking, setChecking] = useState(false);
  const [updateError, setUpdateError] = useState<string | null>(null);

  const handleUpdate = async () => {
    setUpdating(true);
    setUpdateError(null);
    try {
      await onUpdate(host.id);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to trigger update';
      setUpdateError(errorMessage);
      console.error('Failed to trigger update:', error);
    } finally {
      setUpdating(false);
    }
  };

  const handleCheckForUpdates = async () => {
    setChecking(true);
    setUpdateError(null);
    try {
      await onCheckForUpdates(host.id);
    } catch (error) {
      const errorMessage = error instanceof Error ? error.message : 'Failed to check for updates';
      setUpdateError(errorMessage);
      console.error('Failed to check for updates:', error);
    } finally {
      setChecking(false);
    }
  };

  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 hover:border-gray-700 transition-colors">
      <div className="flex items-start justify-between">
        <Link to={`/hosts/${host.id}`} className="flex items-center gap-2 hover:text-white">
          <Server size={16} className="text-gray-500" />
          <span className="font-medium">{host.name}</span>
        </Link>
        <StatusBadge status={host.status} />
      </div>
      <div className="mt-3 space-y-1 text-sm text-gray-400">
        <div>{host.hostname}:{host.ssh_port}</div>
        <div>{host.ssh_user}@{host.os || 'unknown'}/{host.arch || 'unknown'}</div>
        <div>{sessionCount} session{sessionCount !== 1 ? 's' : ''} / {host.max_sessions} max</div>
        {host.update_available && host.latest_version && (
          <div className="text-yellow-500 text-xs">
            Update available: v{host.latest_version}
          </div>
        )}
        {updateError && (
          <div className="text-red-400 text-xs">
            {updateError}
          </div>
        )}
      </div>
      <div className="mt-3 flex items-center gap-2">
        {Object.entries(host.labels || {}).map(([k, v]) => (
          <span key={k} className="text-xs bg-gray-800 text-gray-400 px-2 py-0.5 rounded">
            {k}={v}
          </span>
        ))}
      </div>
      <div className="mt-3 flex justify-end gap-2">
        <button
          onClick={handleCheckForUpdates}
          disabled={checking || host.status !== 'online'}
          className="text-gray-500 hover:text-blue-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
          title={checking ? 'Checking for updates...' : 'Check for updates'}
          aria-label={checking ? 'Checking for updates' : 'Check for updates'}
        >
          <RefreshCw size={14} className={checking ? 'animate-spin' : ''} />
        </button>
        {host.update_available && (
          <button
            onClick={handleUpdate}
            disabled={updating || host.status !== 'online'}
            className="text-gray-500 hover:text-yellow-400 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
            title={updating ? 'Updating...' : 'Update agent'}
            aria-label={updating ? 'Updating agent' : 'Update agent'}
          >
            <Download size={14} className={updating ? 'animate-pulse' : ''} />
          </button>
        )}
        <button
          onClick={() => onDelete(host.id)}
          className="text-gray-500 hover:text-red-400 transition-colors"
          aria-label="Delete host"
        >
          <Trash2 size={14} />
        </button>
      </div>
    </div>
  );
}
