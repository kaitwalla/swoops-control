import { useEffect, useState, useCallback } from 'react';
import { useParams, Link } from 'react-router-dom';
import { sessionsApi } from '../api/sessions';
import { hostsApi } from '../api/hosts';
import { StatusBadge } from '../components/StatusBadge';
import { TerminalOutput } from '../components/TerminalOutput';
import { AgentStatusPanel } from '../components/AgentStatusPanel';
import { TaskManagementPanel } from '../components/TaskManagementPanel';
import { ReviewRequestsPanel } from '../components/ReviewRequestsPanel';
import { SessionMessagesPanel } from '../components/SessionMessagesPanel';
import type { Session } from '../types/session';
import type { Host } from '../types/host';
import { ArrowLeft, Square, Send, RefreshCw } from 'lucide-react';

export function SessionDetail() {
  const { id } = useParams<{ id: string }>();
  const [session, setSession] = useState<Session | null>(null);
  const [host, setHost] = useState<Host | null>(null);
  const [error, setError] = useState('');
  const [inputText, setInputText] = useState('');
  const [activeTab, setActiveTab] = useState<'status' | 'tasks' | 'reviews' | 'messages'>('status');

  const fetchSession = useCallback(() => {
    if (!id) return;
    sessionsApi.get(id).then((s) => {
      setSession(s);
      hostsApi.get(s.host_id).then(setHost).catch(() => {});
    }).catch((e) => setError(e.message));
  }, [id]);

  useEffect(() => {
    fetchSession();
    // Poll session status every 3s to catch state transitions
    const interval = setInterval(fetchSession, 3000);
    return () => clearInterval(interval);
  }, [fetchSession]);

  const handleStop = async () => {
    if (!id) return;
    await sessionsApi.stop(id);
    fetchSession();
  };

  const handleSendInput = async () => {
    if (!id || !inputText.trim()) return;
    await sessionsApi.sendInput(id, inputText);
    setInputText('');
  };

  if (error) return <div className="p-6 text-red-400">{error}</div>;
  if (!session) return <div className="p-6 text-gray-500">Loading...</div>;

  const isActive = ['running', 'starting', 'pending', 'idle'].includes(session.status);

  return (
    <div className="p-6 space-y-6">
      <div className="flex items-center gap-3">
        <Link to="/sessions" className="text-gray-500 hover:text-gray-300">
          <ArrowLeft size={18} />
        </Link>
        <h1 className="text-2xl font-bold">{session.name}</h1>
        <StatusBadge status={session.status} />
        <span className="text-sm text-gray-500 bg-gray-800 px-2 py-0.5 rounded">{session.agent_type}</span>
        <button onClick={fetchSession} className="text-gray-500 hover:text-gray-300 ml-auto" title="Refresh">
          <RefreshCw size={16} />
        </button>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 space-y-2">
          <h2 className="text-sm font-semibold text-gray-400 uppercase">Details</h2>
          <div className="text-sm"><span className="text-gray-500">Host:</span>{' '}
            {host ? (
              <Link to={`/hosts/${host.id}`} className="text-blue-400 hover:underline">{host.name}</Link>
            ) : session.host_id.slice(0, 8)}
          </div>
          <div className="text-sm"><span className="text-gray-500">Branch:</span> <span className="font-mono text-xs">{session.branch_name}</span></div>
          {session.worktree_path && (
            <div className="text-sm"><span className="text-gray-500">Worktree:</span> <span className="font-mono text-xs">{session.worktree_path}</span></div>
          )}
          {session.tmux_session && (
            <div className="text-sm"><span className="text-gray-500">Tmux:</span> <span className="font-mono text-xs">{session.tmux_session}</span></div>
          )}
          {session.model_override && (
            <div className="text-sm"><span className="text-gray-500">Model:</span> {session.model_override}</div>
          )}
          {session.agent_pid > 0 && (
            <div className="text-sm"><span className="text-gray-500">PID:</span> {session.agent_pid}</div>
          )}
          {session.started_at && (
            <div className="text-sm"><span className="text-gray-500">Started:</span> {new Date(session.started_at).toLocaleString()}</div>
          )}
          {session.stopped_at && (
            <div className="text-sm"><span className="text-gray-500">Stopped:</span> {new Date(session.stopped_at).toLocaleString()}</div>
          )}
        </div>

        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4 space-y-2 col-span-2">
          <h2 className="text-sm font-semibold text-gray-400 uppercase">Prompt</h2>
          <p className="text-sm whitespace-pre-wrap">{session.prompt}</p>
        </div>
      </div>

      {/* Terminal output with xterm.js */}
      <div className="bg-gray-950 border border-gray-800 rounded-lg overflow-hidden">
        <div className="flex items-center justify-between px-4 py-2 border-b border-gray-800">
          <span className="text-sm text-gray-400">
            Output
            {isActive && <span className="ml-2 inline-block w-2 h-2 rounded-full bg-green-500 animate-pulse" />}
          </span>
          {isActive && (
            <button
              onClick={handleStop}
              className="flex items-center gap-1 text-sm text-orange-400 hover:text-orange-300"
            >
              <Square size={12} /> Stop
            </button>
          )}
        </div>
        <div className="p-1">
          <TerminalOutput
            initialOutput={session.last_output}
            sessionId={id}
            isActive={isActive}
          />
        </div>
        {isActive && (
          <div className="flex items-center border-t border-gray-800 px-4 py-2 gap-2">
            <span className="text-gray-600 text-sm">$</span>
            <input
              className="flex-1 bg-transparent text-sm text-gray-200 outline-none font-mono"
              placeholder="Send input to session..."
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleSendInput()}
            />
            <button onClick={handleSendInput} className="text-gray-500 hover:text-blue-400">
              <Send size={14} />
            </button>
          </div>
        )}
      </div>

      {/* MCP Features */}
      <div className="bg-gray-900 border border-gray-800 rounded-lg overflow-hidden">
        <div className="flex border-b border-gray-800">
          <button
            onClick={() => setActiveTab('status')}
            className={`px-4 py-2 text-sm font-medium ${
              activeTab === 'status' ? 'bg-gray-800 text-white' : 'text-gray-400 hover:text-gray-300'
            }`}
          >
            Agent Status
          </button>
          <button
            onClick={() => setActiveTab('tasks')}
            className={`px-4 py-2 text-sm font-medium ${
              activeTab === 'tasks' ? 'bg-gray-800 text-white' : 'text-gray-400 hover:text-gray-300'
            }`}
          >
            Tasks
          </button>
          <button
            onClick={() => setActiveTab('reviews')}
            className={`px-4 py-2 text-sm font-medium ${
              activeTab === 'reviews' ? 'bg-gray-800 text-white' : 'text-gray-400 hover:text-gray-300'
            }`}
          >
            Review Requests
          </button>
          <button
            onClick={() => setActiveTab('messages')}
            className={`px-4 py-2 text-sm font-medium ${
              activeTab === 'messages' ? 'bg-gray-800 text-white' : 'text-gray-400 hover:text-gray-300'
            }`}
          >
            Messages
          </button>
        </div>
        <div className="p-4">
          {activeTab === 'status' && <AgentStatusPanel sessionId={id!} />}
          {activeTab === 'tasks' && <TaskManagementPanel sessionId={id!} />}
          {activeTab === 'reviews' && <ReviewRequestsPanel sessionId={id!} />}
          {activeTab === 'messages' && <SessionMessagesPanel sessionId={id!} />}
        </div>
      </div>

      {(session.allowed_tools?.length ?? 0) > 0 && (
        <div className="bg-gray-900 border border-gray-800 rounded-lg p-4">
          <h2 className="text-sm font-semibold text-gray-400 uppercase mb-2">Allowed Tools</h2>
          <div className="flex flex-wrap gap-2">
            {session.allowed_tools.map((t) => (
              <span key={t} className="text-xs bg-gray-800 text-gray-300 px-2 py-0.5 rounded">{t}</span>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
