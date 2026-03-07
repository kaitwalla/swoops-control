import { create } from 'zustand';
import type { Host, CreateHostRequest } from '../types/host';
import { hostsApi } from '../api/hosts';

interface HostStore {
  hosts: Host[];
  loading: boolean;
  error: string | null;
  fetchHosts: () => Promise<void>;
  createHost: (data: CreateHostRequest) => Promise<Host>;
  deleteHost: (id: string) => Promise<void>;
  triggerUpdate: (id: string) => Promise<void>;
  checkForUpdates: (id: string) => Promise<void>;
}

export const useHostStore = create<HostStore>((set, get) => ({
  hosts: [],
  loading: false,
  error: null,

  fetchHosts: async () => {
    set({ loading: true, error: null });
    try {
      const hosts = await hostsApi.list();
      set({ hosts, loading: false });
    } catch (e) {
      set({ error: (e as Error).message, loading: false });
    }
  },

  createHost: async (data: CreateHostRequest) => {
    const host = await hostsApi.create(data);
    set({ hosts: [...get().hosts, host] });
    return host;
  },

  deleteHost: async (id: string) => {
    await hostsApi.del(id);
    set({ hosts: get().hosts.filter((h) => h.id !== id) });
  },

  triggerUpdate: async (id: string) => {
    await hostsApi.triggerUpdate(id);
    // Refresh hosts to get updated state
    await get().fetchHosts();
  },

  checkForUpdates: async (id: string) => {
    await hostsApi.checkForUpdates(id);
    // Wait a bit for the agent to respond, then refresh
    setTimeout(async () => {
      await get().fetchHosts();
    }, 2000);
  },
}));
