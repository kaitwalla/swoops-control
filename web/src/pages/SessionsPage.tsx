import { useEffect } from 'react';
import { Link } from 'react-router-dom';
import { useSessionStore } from '../stores/sessionStore';
import { useHostStore } from '../stores/hostStore';
import { StatusBadge } from '../components/StatusBadge';
import { Square } from 'lucide-react';

export function SessionsPage() {
  const { sessions, loading, error, fetchSessions, stopSession } = useSessionStore();
  const { hosts, fetchHosts } = useHostStore();

  useEffect(() => {
    fetchSessions();
    fetchHosts();
  }, [fetchSessions, fetchHosts]);

  const hostMap = Object.fromEntries(hosts.map((h) => [h.id, h]));

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Sessions</h1>
      </div>

      {error && <p className="text-red-400 text-sm">{error}</p>}

      {loading ? (
        <p className="text-gray-500">Loading...</p>
      ) : sessions.length === 0 ? (
        <p className="text-gray-500">No sessions yet.</p>
      ) : (
        <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-800 text-gray-400 text-left">
                <th className="px-4 py-3">Name</th>
                <th className="px-4 py-3">Agent</th>
                <th className="px-4 py-3">Host</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">Branch</th>
                <th className="px-4 py-3">Created</th>
                <th className="px-4 py-3">Actions</th>
              </tr>
            </thead>
            <tbody>
              {sessions.map((sess) => (
                <tr key={sess.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                  <td className="px-4 py-3">
                    <Link to={`/sessions/${sess.id}`} className="text-blue-400 hover:underline">
                      {sess.name}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-gray-400">{sess.agent_type}</td>
                  <td className="px-4 py-3 text-gray-400">
                    {hostMap[sess.host_id]?.name ?? sess.host_id.slice(0, 8)}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={sess.status} />
                  </td>
                  <td className="px-4 py-3 text-gray-400 font-mono text-xs">{sess.branch_name}</td>
                  <td className="px-4 py-3 text-gray-500 text-xs">
                    {new Date(sess.created_at).toLocaleString()}
                  </td>
                  <td className="px-4 py-3">
                    <div className="flex items-center gap-2">
                      {['running', 'starting', 'pending'].includes(sess.status) && (
                        <button
                          onClick={() => stopSession(sess.id)}
                          className="text-gray-500 hover:text-orange-400"
                          title="Stop session"
                        >
                          <Square size={14} />
                        </button>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
