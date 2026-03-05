import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useSessionStore } from './sessionStore';
import { sessionsApi } from '../api/sessions';
import type { Session, CreateSessionRequest } from '../types/session';

vi.mock('../api/sessions');

describe('sessionStore', () => {
  const mockSession: Session = {
    id: 'session-1',
    name: 'Test Session',
    host_id: 'host-1',
    template_id: 'template-1',
    type: 'agent',
    agent_type: 'claude',
    status: 'running',
    prompt: 'Test prompt',
    branch_name: 'test-branch',
    worktree_path: '/worktrees/test',
    tmux_session: 'test-tmux',
    agent_pid: 1234,
    model_override: 'claude-3-opus',
    env_vars: { TEST: 'value' },
    mcp_servers: [],
    plugins: [],
    allowed_tools: [],
    extra_flags: [],
    last_output: 'Test output',
    started_at: '2024-01-01T00:00:00Z',
    stopped_at: null,
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  };

  beforeEach(() => {
    // Reset store state
    useSessionStore.setState({
      sessions: [],
      loading: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  describe('fetchSessions', () => {
    it('should fetch sessions without params', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue([mockSession]);

      const { fetchSessions } = useSessionStore.getState();
      await fetchSessions();

      const { sessions, loading, error } = useSessionStore.getState();
      expect(sessions).toEqual([mockSession]);
      expect(loading).toBe(false);
      expect(error).toBeNull();
      expect(sessionsApi.list).toHaveBeenCalledWith(undefined);
    });

    it('should fetch sessions with filter params', async () => {
      vi.mocked(sessionsApi.list).mockResolvedValue([mockSession]);

      const params = { host_id: 'host-1', status: 'running' };
      const { fetchSessions } = useSessionStore.getState();
      await fetchSessions(params);

      expect(sessionsApi.list).toHaveBeenCalledWith(params);
      expect(useSessionStore.getState().sessions).toEqual([mockSession]);
    });

    it('should set loading state while fetching', async () => {
      let resolvePromise: (value: Session[]) => void;
      const promise = new Promise<Session[]>((resolve) => {
        resolvePromise = resolve;
      });
      vi.mocked(sessionsApi.list).mockReturnValue(promise);

      const { fetchSessions } = useSessionStore.getState();
      const fetchPromise = fetchSessions();

      expect(useSessionStore.getState().loading).toBe(true);

      resolvePromise!([mockSession]);
      await fetchPromise;

      expect(useSessionStore.getState().loading).toBe(false);
    });

    it('should handle fetch errors', async () => {
      const errorMessage = 'Failed to fetch sessions';
      vi.mocked(sessionsApi.list).mockRejectedValue(new Error(errorMessage));

      const { fetchSessions } = useSessionStore.getState();
      await fetchSessions();

      const { sessions, loading, error } = useSessionStore.getState();
      expect(sessions).toEqual([]);
      expect(loading).toBe(false);
      expect(error).toBe(errorMessage);
    });
  });

  describe('createSession', () => {
    it('should create a session and add it to the beginning of the list', async () => {
      const createRequest: CreateSessionRequest = {
        host_id: 'host-1',
        agent_type: 'claude',
        prompt: 'New session prompt',
      };

      const newSession: Session = {
        ...mockSession,
        id: 'session-2',
        name: 'New Session',
        prompt: createRequest.prompt,
      };

      vi.mocked(sessionsApi.create).mockResolvedValue(newSession);

      // Set initial state with one session
      useSessionStore.setState({ sessions: [mockSession] });

      const { createSession } = useSessionStore.getState();
      const result = await createSession(createRequest);

      expect(result).toEqual(newSession);
      // New session should be at the beginning
      expect(useSessionStore.getState().sessions).toEqual([newSession, mockSession]);
    });
  });

  describe('stopSession', () => {
    it('should stop a session and update its status', async () => {
      vi.mocked(sessionsApi.stop).mockResolvedValue({ status: 'stopped' });

      // Set initial state with a running session
      useSessionStore.setState({ sessions: [mockSession] });

      const { stopSession } = useSessionStore.getState();
      await stopSession('session-1');

      expect(sessionsApi.stop).toHaveBeenCalledWith('session-1');
      const updatedSession = useSessionStore.getState().sessions[0];
      expect(updatedSession.status).toBe('stopped');
    });

    it('should only update the specified session', async () => {
      const session2: Session = { ...mockSession, id: 'session-2', status: 'running' };
      vi.mocked(sessionsApi.stop).mockResolvedValue({ status: 'stopped' });

      useSessionStore.setState({ sessions: [mockSession, session2] });

      const { stopSession } = useSessionStore.getState();
      await stopSession('session-1');

      const sessions = useSessionStore.getState().sessions;
      expect(sessions[0].status).toBe('stopped');
      expect(sessions[1].status).toBe('running');
    });
  });

  describe('deleteSession', () => {
    it('should delete a session from the store', async () => {
      vi.mocked(sessionsApi.del).mockResolvedValue(undefined);

      useSessionStore.setState({ sessions: [mockSession] });

      const { deleteSession } = useSessionStore.getState();
      await deleteSession('session-1');

      expect(sessionsApi.del).toHaveBeenCalledWith('session-1');
      expect(useSessionStore.getState().sessions).toEqual([]);
    });

    it('should only remove the specified session', async () => {
      const session2: Session = { ...mockSession, id: 'session-2' };
      vi.mocked(sessionsApi.del).mockResolvedValue(undefined);

      useSessionStore.setState({ sessions: [mockSession, session2] });

      const { deleteSession } = useSessionStore.getState();
      await deleteSession('session-1');

      expect(useSessionStore.getState().sessions).toEqual([session2]);
    });
  });
});
