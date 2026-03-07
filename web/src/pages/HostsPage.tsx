import { useEffect, useState } from 'react';
import { useHostStore } from '../stores/hostStore';
import { useSessionStore } from '../stores/sessionStore';
import { HostCard } from '../components/HostCard';
import { CreateHostDialog } from '../components/CreateHostDialog';
import { Plus } from 'lucide-react';

export function HostsPage() {
  const { hosts, loading, error, fetchHosts, createHost, deleteHost, triggerUpdate, checkForUpdates } = useHostStore();
  const { sessions, fetchSessions } = useSessionStore();
  const [showCreate, setShowCreate] = useState(false);

  useEffect(() => {
    fetchHosts();
    fetchSessions();
  }, [fetchHosts, fetchSessions]);

  const sessionCountByHost = sessions.reduce<Record<string, number>>((acc, s) => {
    acc[s.host_id] = (acc[s.host_id] || 0) + 1;
    return acc;
  }, {});

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Hosts</h1>
        <button
          onClick={() => setShowCreate(true)}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded text-sm"
        >
          <Plus size={16} />
          Add Host
        </button>
      </div>

      {error && <p className="text-red-400 text-sm">{error}</p>}

      {loading ? (
        <p className="text-gray-500">Loading...</p>
      ) : hosts.length === 0 ? (
        <div className="text-center py-12">
          <p className="text-gray-500 mb-4">No hosts registered yet</p>
          <button
            onClick={() => setShowCreate(true)}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded text-sm"
          >
            Add your first host
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-3 gap-4">
          {hosts.map((host) => (
            <HostCard
              key={host.id}
              host={host}
              sessionCount={sessionCountByHost[host.id] || 0}
              onDelete={deleteHost}
              onUpdate={triggerUpdate}
              onCheckForUpdates={checkForUpdates}
            />
          ))}
        </div>
      )}

      <CreateHostDialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        onSubmit={async (data) => {
          await createHost(data);
        }}
      />
    </div>
  );
}
