import { describe, it, expect, beforeEach, vi } from 'vitest';
import { sessionsApi, createOutputWebSocket } from './sessions';
import { api } from './client';

vi.mock('./client');

describe('sessionsApi', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('list', () => {
    it('should fetch sessions without params', async () => {
      const mockSessions = [{ id: 'session-1' }];
      vi.mocked(api.get).mockResolvedValue(mockSessions);

      const result = await sessionsApi.list();

      expect(api.get).toHaveBeenCalledWith('/sessions');
      expect(result).toEqual(mockSessions);
    });

    it('should fetch sessions with host_id param', async () => {
      const mockSessions = [{ id: 'session-1', host_id: 'host-1' }];
      vi.mocked(api.get).mockResolvedValue(mockSessions);

      const result = await sessionsApi.list({ host_id: 'host-1' });

      expect(api.get).toHaveBeenCalledWith('/sessions?host_id=host-1');
      expect(result).toEqual(mockSessions);
    });

    it('should fetch sessions with status param', async () => {
      const mockSessions = [{ id: 'session-1', status: 'running' }];
      vi.mocked(api.get).mockResolvedValue(mockSessions);

      const result = await sessionsApi.list({ status: 'running' });

      expect(api.get).toHaveBeenCalledWith('/sessions?status=running');
      expect(result).toEqual(mockSessions);
    });

    it('should fetch sessions with multiple params', async () => {
      const mockSessions = [{ id: 'session-1' }];
      vi.mocked(api.get).mockResolvedValue(mockSessions);

      const result = await sessionsApi.list({ host_id: 'host-1', status: 'running' });

      expect(api.get).toHaveBeenCalledWith('/sessions?host_id=host-1&status=running');
      expect(result).toEqual(mockSessions);
    });
  });

  describe('get', () => {
    it('should fetch a single session by id', async () => {
      const mockSession = { id: 'session-1' };
      vi.mocked(api.get).mockResolvedValue(mockSession);

      const result = await sessionsApi.get('session-1');

      expect(api.get).toHaveBeenCalledWith('/sessions/session-1');
      expect(result).toEqual(mockSession);
    });
  });

  describe('create', () => {
    it('should create a new session', async () => {
      const createData = {
        host_id: 'host-1',
        agent_type: 'claude' as const,
        prompt: 'Test prompt',
      };
      const mockSession = { id: 'session-1', ...createData };
      vi.mocked(api.post).mockResolvedValue(mockSession);

      const result = await sessionsApi.create(createData);

      expect(api.post).toHaveBeenCalledWith('/sessions', createData);
      expect(result).toEqual(mockSession);
    });
  });

  describe('stop', () => {
    it('should stop a session', async () => {
      const mockResponse = { status: 'stopped' };
      vi.mocked(api.post).mockResolvedValue(mockResponse);

      const result = await sessionsApi.stop('session-1');

      expect(api.post).toHaveBeenCalledWith('/sessions/session-1/stop', {});
      expect(result).toEqual(mockResponse);
    });
  });

  describe('del', () => {
    it('should delete a session', async () => {
      vi.mocked(api.del).mockResolvedValue(undefined);

      await sessionsApi.del('session-1');

      expect(api.del).toHaveBeenCalledWith('/sessions/session-1');
    });
  });

  describe('sendInput', () => {
    it('should send input to a session', async () => {
      const input = 'test input';
      const mockResponse = { status: 'ok' };
      vi.mocked(api.post).mockResolvedValue(mockResponse);

      const result = await sessionsApi.sendInput('session-1', input);

      expect(api.post).toHaveBeenCalledWith('/sessions/session-1/input', { input });
      expect(result).toEqual(mockResponse);
    });
  });

  describe('getOutput', () => {
    it('should get session output', async () => {
      const mockOutput = { output: 'test output' };
      vi.mocked(api.get).mockResolvedValue(mockOutput);

      const result = await sessionsApi.getOutput('session-1');

      expect(api.get).toHaveBeenCalledWith('/sessions/session-1/output');
      expect(result).toEqual(mockOutput);
    });
  });
});

describe('createOutputWebSocket', () => {
  beforeEach(() => {
    // Mock location
    Object.defineProperty(window, 'location', {
      value: {
        protocol: 'http:',
        host: 'localhost:3000',
      },
      writable: true,
    });

    // Mock WebSocket
    global.WebSocket = vi.fn() as any;
  });

  it('should create WebSocket with correct URL and API key from localStorage', () => {
    localStorage.setItem('swoops_api_key', 'test-key');

    createOutputWebSocket('session-123');

    expect(WebSocket).toHaveBeenCalledWith(
      'ws://localhost:3000/api/v1/ws/sessions/session-123/output?token=test-key'
    );
  });

  it('should use wss protocol when location protocol is https', () => {
    Object.defineProperty(window, 'location', {
      value: {
        protocol: 'https:',
        host: 'example.com',
      },
      writable: true,
    });
    localStorage.setItem('swoops_api_key', 'test-key');

    createOutputWebSocket('session-123');

    expect(WebSocket).toHaveBeenCalledWith(
      'wss://example.com/api/v1/ws/sessions/session-123/output?token=test-key'
    );
  });

  it('should handle missing API key', () => {
    localStorage.removeItem('swoops_api_key');

    createOutputWebSocket('session-123');

    expect(WebSocket).toHaveBeenCalledWith(
      'ws://localhost:3000/api/v1/ws/sessions/session-123/output?token='
    );
  });
});
