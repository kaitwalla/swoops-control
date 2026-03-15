import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useSessionStore } from '../stores/sessionStore';
import { useHostStore } from '../stores/hostStore';
import { StatusBadge } from '../components/StatusBadge';
import { CreateSessionDialog } from '../components/CreateSessionDialog';
import { Plus, Square, Trash2 } from 'lucide-react';

export function SessionsPage() {
  const { sessions, loading, error, fetchSessions, createSession, stopSession, deleteSession } = useSessionStore();
  const { hosts, fetchHosts } = useHostStore();
  const [showCreate, setShowCreate] = useState(false);

  useEffect(() => {
    fetchSessions();
    fetchHosts();
    // Poll for updates
    const interval = setInterval(fetchSessions, 5000);
    return () => clearInterval(interval);
  }, [fetchSessions, fetchHosts]);

  const hostMap = Object.fromEntries(hosts.map((h) => [h.id, h]));

  const handleDelete = async (id: string, isActive: boolean) => {
    const message = isActive
      ? 'This session is running. Are you sure you want to stop and delete it?'
      : 'Are you sure you want to delete this session?';
    if (window.confirm(message)) {
      await deleteSession(id);
    }
  };

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Sessions</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded text-sm"
        >
          <Plus size={16} />
          Create Session
        </button>
      </div>

      {error && <p className="text-red-400 text-sm">{error}</p>}

      {loading ? (
        <p className="text-gray-500">Loading...</p>
      ) : sessions.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-gray-500 mb-4">No sessions yet</p>
          <button
            onClick={() => setShowCreate(true)}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded text-sm"
          >
            Create your first session
          </button>
        </div>
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
                    {hostMap[sess.host_id] ? (
                      <Link to={`/hosts/${sess.host_id}`} className="hover:text-blue-400">
                        {hostMap[sess.host_id].name}
                      </Link>
                    ) : (
                      sess.host_id.slice(0, 8)
                    )}
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
                      <button
                        onClick={() => handleDelete(sess.id, ['running', 'starting', 'pending'].includes(sess.status))}
                        className="text-gray-500 hover:text-red-400"
                        title={['running', 'starting', 'pending'].includes(sess.status) ? "Stop and delete session" : "Delete session"}
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <CreateSessionDialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onSubmit={async (data) => {
          await createSession(data);
        }}
      />
    </div>
  );
}
