const statusColors: Record<string, string> = {
  online: 'bg-green-500/20 text-green-400',
  offline: 'bg-gray-500/20 text-gray-400',
  degraded: 'bg-yellow-500/20 text-yellow-400',
  provisioning: 'bg-blue-500/20 text-blue-400',
  pending: 'bg-gray-500/20 text-gray-400',
  starting: 'bg-blue-500/20 text-blue-400',
  running: 'bg-green-500/20 text-green-400',
  idle: 'bg-yellow-500/20 text-yellow-400',
  stopping: 'bg-orange-500/20 text-orange-400',
  stopped: 'bg-gray-500/20 text-gray-400',
  failed: 'bg-red-500/20 text-red-400',
};

export function StatusBadge({ status }: { status: string }) {
  const color = statusColors[status] ?? 'bg-gray-500/20 text-gray-400';
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium ${color}`}>
      {status}
    </span>
  );
}
