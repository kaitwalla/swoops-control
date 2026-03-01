import { useEffect, useState } from 'react';
import { useParams, Link, useNavigate } from 'react-router-dom';
import { hostsApi } from '../api/hosts';
import { sessionsApi } from '../api/sessions';
import { StatusBadge } from '../components/StatusBadge';
import { CreateSessionDialog } from '../components/CreateSessionDialog';
import type { Host } from '../types/host';
import type { Session } from '../types/session';
import { ArrowLeft, Plus } from 'lucide-react';

export function HostDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [host, setHost] = useState<Host | null>(null);
  const [sessions, setSessions] = useState<Session[]>([]);
  const [error, setError] = useState('');
  const [showCreate, setShowCreate] = useState(false);

  useEffect(() => {
    if (!id) return;
    hostsApi.get(id).then(setHost).catch((e) => setError(e.message));
    sessionsApi.list({ host_id: id }).then(setSessions).catch(() => {});
  }, [id]);

  if (error) return <div className="p-6 text-red-400">{error}</div>;
  if (!host) return <div className="p-6 text-gray-500">Loading...</div>;

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/hosts" className="text-gray-500 hover:text-gray-300">
          <ArrowLeft size={18} />
        </Link>
        <h1 className="text-2xl font-bold">{host.name}</h1>
        <StatusBadge status={host.status} />
      </div>

      <div className="grid grid-cols-2 gap-6">
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 space-y-2">
          <h2 className="text-sm font-semibold text-gray-400 uppercase">Connection</h2>
          <div className="text-sm"><span className="text-gray-500">Hostname:</span> {host.hostname}</div>
          <div className="text-sm"><span className="text-gray-500">SSH:</span> {host.ssh_user}@{host.hostname}:{host.ssh_port}</div>
          <div className="text-sm"><span className="text-gray-500">Key:</span> <span className="font-mono text-xs">{host.ssh_key_path}</span></div>
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 space-y-2">
          <h2 className="text-sm font-semibold text-gray-400 uppercase">System</h2>
          <div className="text-sm"><span className="text-gray-500">OS/Arch:</span> {host.os || 'unknown'}/{host.arch || 'unknown'}</div>
          <div className="text-sm"><span className="text-gray-500">Agent:</span> {host.agent_version || 'not installed'}</div>
          <div className="text-sm"><span className="text-gray-500">Max Sessions:</span> {host.max_sessions}</div>
          <div className="text-sm"><span className="text-gray-500">Base Repo:</span> <span className="font-mono text-xs">{host.base_repo_path}</span></div>
          <div className="text-sm"><span className="text-gray-500">Worktree Root:</span> <span className="font-mono text-xs">{host.worktree_root}</span></div>
        </div>
      </div>

      {Object.keys(host.labels || {}).length > 0 && (
        <div className="flex gap-2">
          {Object.entries(host.labels).map(([k, v]) => (
            <span key={k} className="text-xs bg-gray-800 text-gray-400 px-2 py-1 rounded">
              {k}={v}
            </span>
          ))}
        </div>
      )}

      {(host.installed_tools?.length ?? 0) > 0 && (
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
          <h2 className="text-sm font-semibold text-gray-400 uppercase mb-2">Installed Tools</h2>
          <div className="flex flex-wrap gap-2">
            {host.installed_tools.map((t) => (
              <span key={t.name} className="text-xs bg-gray-800 text-gray-300 px-2 py-1 rounded">
                {t.name} {t.version && <span className="text-gray-500">v{t.version}</span>}
              </span>
            ))}
          </div>
        </div>
      )}

      <div>
        <div className="flex items-center justify-between mb-3">
          <h2 className="text-lg font-semibold">Sessions on this host</h2>
          <button
            onClick={() => setShowCreate(true)}
            className="flex items-center gap-2 px-3 py-1.5 bg-blue-600 hover:bg-blue-500 rounded text-sm"
          >
            <Plus size={14} />
            New Session
          </button>
        </div>
        {sessions.length === 0 ? (
          <p className="text-gray-500 text-sm">No sessions on this host.</p>
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
                {sessions.map((sess) => (
                  <tr key={sess.id} className="border-b border-gray-800/50 hover:bg-gray-800/30">
                    <td className="px-4 py-2">
                      <Link to={`/sessions/${sess.id}`} className="text-blue-400 hover:underline">
                        {sess.name}
                      </Link>
                    </td>
                    <td className="px-4 py-2 text-gray-400">{sess.agent_type}</td>
                    <td className="px-4 py-2"><StatusBadge status={sess.status} /></td>
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

      <CreateSessionDialog
        open={showCreate}
        onClose={() => setShowCreate(false)}
        preselectedHostId={id}
        onSubmit={async (data) => {
          const session = await sessionsApi.create(data);
          navigate(`/sessions/${session.id}`);
        }}
      />
    </div>
  );
}
