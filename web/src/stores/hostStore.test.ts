import { describe, it, expect, beforeEach, vi } from 'vitest';
import { useHostStore } from './hostStore';
import { hostsApi } from '../api/hosts';
import type { Host, CreateHostRequest } from '../types/host';

vi.mock('../api/hosts');

describe('hostStore', () => {
  const mockHost: Host = {
    id: 'host-1',
    name: 'Test Host',
    hostname: 'test.example.com',
    ssh_port: 22,
    ssh_user: 'testuser',
    ssh_key_path: '/path/to/key',
    os: 'linux',
    arch: 'x86_64',
    status: 'online',
    agent_version: '1.0.0',
    labels: { env: 'test' },
    max_sessions: 5,
    base_repo_path: '/repos',
    worktree_root: '/worktrees',
    installed_plugins: [],
    installed_tools: [],
    last_heartbeat: '2024-01-01T00:00:00Z',
    created_at: '2024-01-01T00:00:00Z',
    updated_at: '2024-01-01T00:00:00Z',
  };

  beforeEach(() => {
    // Reset store state
    useHostStore.setState({
      hosts: [],
      loading: false,
      error: null,
    });
    vi.clearAllMocks();
  });

  describe('fetchHosts', () => {
    it('should fetch hosts successfully', async () => {
      vi.mocked(hostsApi.list).mockResolvedValue([mockHost]);

      const { fetchHosts } = useHostStore.getState();
      await fetchHosts();

      const { hosts, loading, error } = useHostStore.getState();
      expect(hosts).toEqual([mockHost]);
      expect(loading).toBe(false);
      expect(error).toBeNull();
    });

    it('should set loading state while fetching', async () => {
      let resolvePromise: (value: Host[]) => void;
      const promise = new Promise<Host[]>((resolve) => {
        resolvePromise = resolve;
      });
      vi.mocked(hostsApi.list).mockReturnValue(promise);

      const { fetchHosts } = useHostStore.getState();
      const fetchPromise = fetchHosts();

      // Should be loading
      expect(useHostStore.getState().loading).toBe(true);

      // Resolve the promise
      resolvePromise!([mockHost]);
      await fetchPromise;

      // Should no longer be loading
      expect(useHostStore.getState().loading).toBe(false);
    });

    it('should handle fetch errors', async () => {
      const errorMessage = 'Failed to fetch hosts';
      vi.mocked(hostsApi.list).mockRejectedValue(new Error(errorMessage));

      const { fetchHosts } = useHostStore.getState();
      await fetchHosts();

      const { hosts, loading, error } = useHostStore.getState();
      expect(hosts).toEqual([]);
      expect(loading).toBe(false);
      expect(error).toBe(errorMessage);
    });
  });

  describe('createHost', () => {
    it('should create a host and add it to the store', async () => {
      const createRequest: CreateHostRequest = {
        name: 'New Host',
        hostname: 'new.example.com',
        ssh_port: 22,
        ssh_user: 'newuser',
        ssh_key_path: '/path/to/new/key',
        max_sessions: 3,
        labels: {},
        base_repo_path: '/repos',
        worktree_root: '/worktrees',
      };

      const newHost: Host = {
        ...mockHost,
        id: 'host-2',
        ...createRequest,
      };

      vi.mocked(hostsApi.create).mockResolvedValue(newHost);

      // Set initial state with one host
      useHostStore.setState({ hosts: [mockHost] });

      const { createHost } = useHostStore.getState();
      const result = await createHost(createRequest);

      expect(result).toEqual(newHost);
      expect(useHostStore.getState().hosts).toEqual([mockHost, newHost]);
    });
  });

  describe('deleteHost', () => {
    it('should delete a host from the store', async () => {
      vi.mocked(hostsApi.del).mockResolvedValue(undefined);

      // Set initial state with hosts
      useHostStore.setState({ hosts: [mockHost] });

      const { deleteHost } = useHostStore.getState();
      await deleteHost('host-1');

      expect(hostsApi.del).toHaveBeenCalledWith('host-1');
      expect(useHostStore.getState().hosts).toEqual([]);
    });

    it('should only remove the specified host', async () => {
      const host2: Host = { ...mockHost, id: 'host-2', name: 'Host 2' };
      vi.mocked(hostsApi.del).mockResolvedValue(undefined);

      // Set initial state with multiple hosts
      useHostStore.setState({ hosts: [mockHost, host2] });

      const { deleteHost } = useHostStore.getState();
      await deleteHost('host-1');

      expect(useHostStore.getState().hosts).toEqual([host2]);
    });
  });
});
