import type { Host } from '../types/host';
import { StatusBadge } from './StatusBadge';
import { Server, Trash2 } from 'lucide-react';
import { Link } from 'react-router-dom';

interface HostCardProps {
  host: Host;
  sessionCount: number;
  onDelete: (id: string) => void;
}

export function HostCard({ host, sessionCount, onDelete }: HostCardProps) {
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
      </div>
      <div className="mt-3 flex items-center gap-2">
        {Object.entries(host.labels || {}).map(([k, v]) => (
          <span key={k} className="text-xs bg-gray-800 text-gray-400 px-2 py-0.5 rounded">
            {k}={v}
          </span>
        ))}
      </div>
      <div className="mt-3 flex justify-end">
        <button
          onClick={() => onDelete(host.id)}
          className="text-gray-500 hover:text-red-400 transition-colors"
        >
          <Trash2 size={14} />
        </button>
      </div>
    </div>
  );
}
