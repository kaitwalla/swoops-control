import { useState, useEffect } from 'react';
import { X, FolderPlus, GitBranch, FolderOpen, Github } from 'lucide-react';
import type { CreateSessionRequest, AgentType, SessionType, Session, DirectorySourceType, DirectorySource } from '../types/session';
import type { Host } from '../types/host';
import { hostsApi } from '../api/hosts';
import { sessionsApi } from '../api/sessions';
import { githubApi, type GitHubRepo } from '../api/github';
import { filesystemApi, type DirectoryEntry } from '../api/filesystem';
import { useNavigate } from 'react-router-dom';

interface CreateSessionDialogProps {
  open: boolean;
  onClose: () => void;
  onSubmit: (data: CreateSessionRequest) => Promise<void>;
  preselectedHostId?: string;
}

type DialogMode = 'create' | 'join';

export function CreateSessionDialog({ open, onClose, onSubmit, preselectedHostId }: CreateSessionDialogProps) {
  const navigate = useNavigate();
  const [mode, setMode] = useState<DialogMode>('create');
  const [hosts, setHosts] = useState<Host[]>([]);
  const [existingSessions, setExistingSessions] = useState<Session[]>([]);
  const [hostId, setHostId] = useState(preselectedHostId || '');
  const [selectedSessionId, setSelectedSessionId] = useState('');
  const [sessionType, setSessionType] = useState<SessionType>('agent');
  const [agentType, setAgentType] = useState<AgentType>('claude');
  const [prompt, setPrompt] = useState('');
  const [name, setName] = useState('');
  const [branchName, setBranchName] = useState('');
  const [modelOverride, setModelOverride] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  // Directory selection state
  const [directorySourceType, setDirectorySourceType] = useState<DirectorySourceType>('existing');
  const [directories, setDirectories] = useState<DirectoryEntry[]>([]);
  const [selectedDirectory, setSelectedDirectory] = useState('');
  const [newFolderName, setNewFolderName] = useState('');
  const [githubRepos, setGithubRepos] = useState<GitHubRepo[]>([]);
  const [selectedRepo, setSelectedRepo] = useState('');
  const [cloneFolderName, setCloneFolderName] = useState('');
  const [newRepoName, setNewRepoName] = useState('');
  const [newRepoDescription, setNewRepoDescription] = useState('');
  const [newRepoPrivate, setNewRepoPrivate] = useState(true);
  const [loadingDirectories, setLoadingDirectories] = useState(false);
  const [loadingRepos, setLoadingRepos] = useState(false);

  const selectedHost = hosts.find(h => h.id === hostId);
  const hasDefaultRootDir = selectedHost?.default_root_directory && selectedHost.default_root_directory.trim() !== '';

  useEffect(() => {
    if (open) {
      hostsApi.list().then(setHosts).catch(() => {});
      if (preselectedHostId) setHostId(preselectedHostId);
    }
  }, [open, preselectedHostId]);

  // Load existing sessions when host changes in join mode
  useEffect(() => {
    if (open && mode === 'join' && hostId) {
      setSelectedSessionId('');
      sessionsApi.list({ host_id: hostId }).then((sessions) => {
        const joinable = sessions.filter(s =>
          ['running', 'idle', 'pending', 'starting'].includes(s.status)
        );
        setExistingSessions(joinable);
      }).catch(() => setExistingSessions([]));
    }
  }, [open, mode, hostId]);

  // Load directories when host changes and has default root directory
  useEffect(() => {
    if (open && mode === 'create' && sessionType === 'agent' && hostId && hasDefaultRootDir && directorySourceType === 'existing') {
      setLoadingDirectories(true);
      filesystemApi.listDirectories(hostId, selectedHost!.default_root_directory!)
        .then(dirs => {
          setDirectories(dirs);
          setLoadingDirectories(false);
        })
        .catch(err => {
          console.error('Failed to load directories:', err);
          setDirectories([]);
          setLoadingDirectories(false);
        });
    }
  }, [open, mode, sessionType, hostId, hasDefaultRootDir, directorySourceType]);

  // Load GitHub repos when clone_repo option is selected
  useEffect(() => {
    if (open && mode === 'create' && sessionType === 'agent' && directorySourceType === 'clone_repo' && !githubRepos.length) {
      setLoadingRepos(true);
      githubApi.listRepos()
        .then(repos => {
          setGithubRepos(repos);
          setLoadingRepos(false);
        })
        .catch(err => {
          console.error('Failed to load GitHub repos:', err);
          setGithubRepos([]);
          setLoadingRepos(false);
        });
    }
  }, [open, mode, sessionType, directorySourceType]);

  if (!open) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError('');
    try {
      if (mode === 'join') {
        if (!selectedSessionId || !prompt.trim()) {
          setError('Please select a session and enter a prompt');
          setLoading(false);
          return;
        }
        await sessionsApi.sendInput(selectedSessionId, prompt);
        navigate(`/sessions/${selectedSessionId}`);
        onClose();
      } else {
        // Create new session
        const request: CreateSessionRequest = {
          host_id: hostId,
          type: sessionType,
          agent_type: sessionType === 'agent' ? agentType : undefined,
          prompt: prompt || undefined,
          name: name || undefined,
          branch_name: branchName || undefined,
          model_override: modelOverride || undefined,
        };

        // Add directory source information if using custom root directory
        if (sessionType === 'agent' && hasDefaultRootDir) {
          const directorySource: DirectorySource = {
            type: directorySourceType,
          };

          switch (directorySourceType) {
            case 'existing':
              if (!selectedDirectory) {
                setError('Please select an existing directory');
                setLoading(false);
                return;
              }
              directorySource.existing_path = selectedDirectory;
              request.working_directory = selectedDirectory;
              break;

            case 'new_folder':
              if (!newFolderName.trim()) {
                setError('Please enter a folder name');
                setLoading(false);
                return;
              }
              directorySource.new_folder_name = newFolderName;
              request.working_directory = `${selectedHost!.default_root_directory}/${newFolderName}`;
              break;

            case 'clone_repo':
              if (!selectedRepo) {
                setError('Please select a repository to clone');
                setLoading(false);
                return;
              }
              const repo = githubRepos.find(r => r.clone_url === selectedRepo);
              directorySource.repo_url = selectedRepo;
              directorySource.clone_folder_name = cloneFolderName || undefined;
              const folderName = cloneFolderName || repo?.name || 'repo';
              request.working_directory = `${selectedHost!.default_root_directory}/${folderName}`;
              break;

            case 'new_repo':
              if (!newRepoName.trim()) {
                setError('Please enter a repository name');
                setLoading(false);
                return;
              }
              directorySource.repo_name = newRepoName;
              directorySource.repo_description = newRepoDescription || undefined;
              directorySource.repo_private = newRepoPrivate;
              request.working_directory = `${selectedHost!.default_root_directory}/${newRepoName}`;
              break;
          }

          request.directory_source = directorySource;
        }

        await onSubmit(request);
        onClose();
      }
      // Reset form
      setPrompt('');
      setName('');
      setBranchName('');
      setModelOverride('');
      setSelectedSessionId('');
      setSelectedDirectory('');
      setNewFolderName('');
      setSelectedRepo('');
      setCloneFolderName('');
      setNewRepoName('');
      setNewRepoDescription('');
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 bg-black/60 flex items-center justify-center z-50">
      <div className="bg-gray-900 border border-gray-700 rounded-lg w-full max-w-2xl p-6 max-h-[90vh] overflow-y-auto">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold">{mode === 'create' ? 'Create Session' : 'Join Existing Session'}</h2>
          <button onClick={onClose} className="text-gray-500 hover:text-gray-300">
            <X size={18} />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          {/* Mode Toggle */}
          <div className="flex gap-2 p-1 bg-gray-800 rounded">
            <button
              type="button"
              onClick={() => setMode('create')}
              className={`flex-1 px-3 py-2 text-sm rounded transition-colors ${
                mode === 'create'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-400 hover:text-gray-200'
              }`}
            >
              Create New
            </button>
            <button
              type="button"
              onClick={() => setMode('join')}
              className={`flex-1 px-3 py-2 text-sm rounded transition-colors ${
                mode === 'join'
                  ? 'bg-blue-600 text-white'
                  : 'text-gray-400 hover:text-gray-200'
              }`}
            >
              Join Existing
            </button>
          </div>

          {/* Host Selection (always shown) */}
          <div>
            <label className="block text-sm text-gray-400 mb-1">Host *</label>
            <select
              value={hostId}
              onChange={(e) => setHostId(e.target.value)}
              className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
              required
            >
              <option value="">Select host...</option>
              {hosts.map((h) => (
                <option key={h.id} value={h.id}>
                  {h.name} ({h.hostname})
                </option>
              ))}
            </select>
          </div>

          {mode === 'join' ? (
            /* Join Mode: Session Selection */
            <>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Session *</label>
                <select
                  value={selectedSessionId}
                  onChange={(e) => setSelectedSessionId(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                  required
                  disabled={!hostId}
                >
                  <option value="">Select session...</option>
                  {existingSessions.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.name} - {s.agent_type} ({s.status})
                    </option>
                  ))}
                </select>
                {hostId && existingSessions.length === 0 && (
                  <p className="text-xs text-gray-500 mt-1">No active sessions found on this host</p>
                )}
              </div>

              <div>
                <label className="block text-sm text-gray-400 mb-1">Continue with Prompt *</label>
                <textarea
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm min-h-[100px]"
                  placeholder="Send a new prompt to continue this session..."
                  required
                />
              </div>
            </>
          ) : (
            /* Create Mode: Full Form */
            <>
              <div>
                <label className="block text-sm text-gray-400 mb-1">Session Type *</label>
                <select
                  value={sessionType}
                  onChange={(e) => setSessionType(e.target.value as SessionType)}
                  className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                >
                  <option value="agent">Agent Session (Claude Code / Codex in a repo)</option>
                  <option value="shell">Interactive Shell</option>
                </select>
              </div>

              {sessionType === 'agent' && (
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Agent Type *</label>
                  <select
                    value={agentType}
                    onChange={(e) => setAgentType(e.target.value as AgentType)}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                  >
                    <option value="claude">Claude Code</option>
                    <option value="codex">Codex</option>
                  </select>
                </div>
              )}

              {/* Directory Selection - Only shown for agent sessions with default root directory */}
              {sessionType === 'agent' && hasDefaultRootDir && (
                <div className="border border-gray-700 rounded-lg p-4 space-y-4">
                  <div>
                    <label className="block text-sm font-medium text-gray-300 mb-2">Working Directory</label>
                    <p className="text-xs text-gray-500 mb-3">Root: {selectedHost?.default_root_directory}</p>
                  </div>

                  {/* Directory Source Type Selection */}
                  <div className="grid grid-cols-2 gap-2">
                    <button
                      type="button"
                      onClick={() => setDirectorySourceType('existing')}
                      className={`flex items-center gap-2 px-3 py-2 text-sm rounded border transition-colors ${
                        directorySourceType === 'existing'
                          ? 'bg-blue-600/20 border-blue-600 text-blue-400'
                          : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'
                      }`}
                    >
                      <FolderOpen size={16} />
                      <span>Existing Folder</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => setDirectorySourceType('new_folder')}
                      className={`flex items-center gap-2 px-3 py-2 text-sm rounded border transition-colors ${
                        directorySourceType === 'new_folder'
                          ? 'bg-blue-600/20 border-blue-600 text-blue-400'
                          : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'
                      }`}
                    >
                      <FolderPlus size={16} />
                      <span>New Folder</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => setDirectorySourceType('clone_repo')}
                      className={`flex items-center gap-2 px-3 py-2 text-sm rounded border transition-colors ${
                        directorySourceType === 'clone_repo'
                          ? 'bg-blue-600/20 border-blue-600 text-blue-400'
                          : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'
                      }`}
                    >
                      <GitBranch size={16} />
                      <span>Clone Repo</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => setDirectorySourceType('new_repo')}
                      className={`flex items-center gap-2 px-3 py-2 text-sm rounded border transition-colors ${
                        directorySourceType === 'new_repo'
                          ? 'bg-blue-600/20 border-blue-600 text-blue-400'
                          : 'bg-gray-800 border-gray-700 text-gray-400 hover:border-gray-600'
                      }`}
                    >
                      <Github size={16} />
                      <span>New Repo</span>
                    </button>
                  </div>

                  {/* Directory Source Type Options */}
                  {directorySourceType === 'existing' && (
                    <div>
                      <label className="block text-sm text-gray-400 mb-1">Select Directory *</label>
                      <select
                        value={selectedDirectory}
                        onChange={(e) => setSelectedDirectory(e.target.value)}
                        className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                        disabled={loadingDirectories}
                        required
                      >
                        <option value="">
                          {loadingDirectories ? 'Loading directories...' : 'Select a directory...'}
                        </option>
                        {directories.map((dir) => (
                          <option key={dir.path} value={dir.path}>
                            {dir.name}
                          </option>
                        ))}
                      </select>
                      {!loadingDirectories && directories.length === 0 && (
                        <p className="text-xs text-gray-500 mt-1">No subdirectories found</p>
                      )}
                    </div>
                  )}

                  {directorySourceType === 'new_folder' && (
                    <div>
                      <label className="block text-sm text-gray-400 mb-1">Folder Name *</label>
                      <input
                        type="text"
                        value={newFolderName}
                        onChange={(e) => setNewFolderName(e.target.value)}
                        className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                        placeholder="my-project"
                        required
                      />
                      <p className="text-xs text-gray-500 mt-1">
                        Will create: {selectedHost?.default_root_directory}/{newFolderName || '...'}
                      </p>
                    </div>
                  )}

                  {directorySourceType === 'clone_repo' && (
                    <div className="space-y-3">
                      <div>
                        <label className="block text-sm text-gray-400 mb-1">GitHub Repository *</label>
                        <select
                          value={selectedRepo}
                          onChange={(e) => setSelectedRepo(e.target.value)}
                          className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                          disabled={loadingRepos}
                          required
                        >
                          <option value="">
                            {loadingRepos ? 'Loading repositories...' : 'Select a repository...'}
                          </option>
                          {githubRepos.map((repo) => (
                            <option key={repo.id} value={repo.clone_url}>
                              {repo.full_name} {repo.private ? '🔒' : ''}
                            </option>
                          ))}
                        </select>
                        {!loadingRepos && githubRepos.length === 0 && (
                          <p className="text-xs text-yellow-500 mt-1">
                            No repositories found. Configure your GitHub token in settings.
                          </p>
                        )}
                      </div>
                      <div>
                        <label className="block text-sm text-gray-400 mb-1">Custom Folder Name (optional)</label>
                        <input
                          type="text"
                          value={cloneFolderName}
                          onChange={(e) => setCloneFolderName(e.target.value)}
                          className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                          placeholder="Leave empty to use repository name"
                        />
                      </div>
                    </div>
                  )}

                  {directorySourceType === 'new_repo' && (
                    <div className="space-y-3">
                      <div>
                        <label className="block text-sm text-gray-400 mb-1">Repository Name *</label>
                        <input
                          type="text"
                          value={newRepoName}
                          onChange={(e) => setNewRepoName(e.target.value)}
                          className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                          placeholder="my-new-project"
                          required
                        />
                      </div>
                      <div>
                        <label className="block text-sm text-gray-400 mb-1">Description (optional)</label>
                        <input
                          type="text"
                          value={newRepoDescription}
                          onChange={(e) => setNewRepoDescription(e.target.value)}
                          className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                          placeholder="A brief description of your project"
                        />
                      </div>
                      <div className="flex items-center gap-2">
                        <input
                          type="checkbox"
                          id="repo-private"
                          checked={newRepoPrivate}
                          onChange={(e) => setNewRepoPrivate(e.target.checked)}
                          className="w-4 h-4 bg-gray-800 border-gray-700 rounded"
                        />
                        <label htmlFor="repo-private" className="text-sm text-gray-400">
                          Private repository
                        </label>
                      </div>
                    </div>
                  )}
                </div>
              )}

              <div>
                <label className="block text-sm text-gray-400 mb-1">Initial Prompt</label>
                <textarea
                  value={prompt}
                  onChange={(e) => setPrompt(e.target.value)}
                  className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm min-h-[100px]"
                  placeholder="Optional: Describe an initial task, or leave empty to start an interactive session"
                />
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Session Name</label>
                  <input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                    placeholder="Auto-generated if empty"
                  />
                </div>
                {sessionType === 'agent' && !hasDefaultRootDir && (
                  <div>
                    <label className="block text-sm text-gray-400 mb-1">Branch Name</label>
                    <input
                      value={branchName}
                      onChange={(e) => setBranchName(e.target.value)}
                      className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                      placeholder="Optional: e.g., feature/my-work"
                    />
                  </div>
                )}
              </div>

              {sessionType === 'agent' && (
                <div>
                  <label className="block text-sm text-gray-400 mb-1">Model Override</label>
                  <input
                    value={modelOverride}
                    onChange={(e) => setModelOverride(e.target.value)}
                    className="w-full bg-gray-800 border border-gray-700 rounded px-3 py-2 text-sm"
                    placeholder="e.g., claude-sonnet-4-20250514"
                  />
                </div>
              )}
            </>
          )}

          {error && <p className="text-red-400 text-sm">{error}</p>}

          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-400 hover:text-gray-200"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !hostId || (mode === 'join' && (!selectedSessionId || !prompt.trim()))}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 rounded text-sm disabled:opacity-50 disabled:cursor-not-allowed"
            >
              {loading ? (mode === 'join' ? 'Joining...' : 'Creating...') : (mode === 'join' ? 'Join & Send' : 'Create Session')}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
