import { useEffect, useState } from 'react';
import { api } from '../api/client';
import { useHostStore } from '../stores/hostStore';
import { useSessionStore } from '../stores/sessionStore';
import { HostCard } from '../components/HostCard';
import { StatusBadge } from '../components/StatusBadge';
import { Link } from 'react-router-dom';
import { Server, Terminal, Activity } from 'lucide-react';

interface Stats {
  total_hosts: number;
  online_hosts: number;
  total_sessions: number;
  sessions_by_status: Record<string, number>;
}

export function Dashboard() {
  const { hosts, fetchHosts, deleteHost, triggerUpdate } = useHostStore();
  const { sessions, fetchSessions } = useSessionStore();
  const [stats, setStats] = useState<Stats | null>(null);

  useEffect(() => {
    fetchHosts();
    fetchSessions();
    api.get<Stats>('/stats').then(setStats);
  }, [fetchHosts, fetchSessions]);

  const sessionCountByHost = sessions.reduce<Record<string, number>>((acc, s) => {
    acc[s.host_id] = (acc[s.host_id] || 0) + 1;
    return acc;
  }, {});

  return (
    <div className="p-6 space-y-6">
      <h1 className="text-2xl font-bold">Dashboard</h1>

      <div className="grid grid-cols-4 gap-4">
        <StatCard icon={Server} label="Total Hosts" value={stats?.total_hosts ?? 0} />
        <StatCard icon={Server} label="Online" value={stats?.online_hosts ?? 0} color="text-green-400" />
        <StatCard icon={Terminal} label="Total Sessions" value={stats?.total_sessions ?? 0} />
        <StatCard
          icon={Activity}
          label="Running"
          value={stats?.sessions_by_status?.running ?? 0}
          color="text-green-400"
        />
      </div>

      <div>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold">Hosts</h2>
          <Link to="/hosts" className="text-sm text-blue-400 hover:text-blue-300">
            View all
          </Link>
        </div>
        {hosts.length === 0 ? (
          <p className="text-gray-500 text-sm">
            No hosts registered.{' '}
            <Link to="/hosts" className="text-blue-400 hover:underline">
              Add one
            </Link>
          </p>
        ) : (
          <div className="grid grid-cols-3 gap-4">
            {hosts.slice(0, 6).map((host) => (
              <HostCard
                key={host.id}
                host={host}
                sessionCount={sessionCountByHost[host.id] || 0}
                onDelete={deleteHost}
                onUpdate={triggerUpdate}
              />
            ))}
          </div>
        )}
      </div>

      <div>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold">Recent Sessions</h2>
          <Link to="/sessions" className="text-sm text-blue-400 hover:text-blue-300">
            View all
          </Link>
        </div>
        {sessions.length === 0 ? (
          <p className="text-gray-500 text-sm">No sessions yet.</p>
        ) : (
          <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-gray-800 text-gray-400 text-left">
                  <th className="px-4 py-2">Name</th>
                  <th className="px-4 py-2">Agent</th>
                  <th className="px-4 py-2">Status</th>
                  <th className="px-4 py-2">Branch</th>
                  <th className="px-4 py-2">Created</th>
                </tr>
              </thead>
              <tbody>
                {sessions.slice(0, 10).map((sess) => (
                  <tr key={sess.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                    <td className="px-4 py-2">
                      <Link to={`/sessions/${sess.id}`} className="text-blue-400 hover:underline">
                        {sess.name}
                      </Link>
                    </td>
                    <td className="px-4 py-2 text-gray-400">{sess.agent_type}</td>
                    <td className="px-4 py-2">
                      <StatusBadge status={sess.status} />
                    </td>
                    <td className="px-4 py-2 text-gray-400 font-mono text-xs">{sess.branch_name}</td>
                    <td className="px-4 py-2 text-gray-500 text-xs">
                      {new Date(sess.created_at).toLocaleString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}

function StatCard({
  icon: Icon,
  label,
  value,
  color = 'text-white',
}: {
  icon: React.ElementType;
  label: string;
  value: number;
  color?: string;
}) {
  return (
    <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
      <div className="flex items-center gap-2 text-gray-400 text-sm mb-1">
        <Icon size={14} />
        {label}
      </div>
      <div className={`text-2xl font-bold ${color}`}>{value}</div>
    </div>
  );
}
