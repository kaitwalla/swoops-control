import { create } from 'zustand';
import type { Session, CreateSessionRequest } from '../types/session';
import { sessionsApi } from '../api/sessions';

interface SessionStore {
  sessions: Session[];
  loading: boolean;
  error: string | null;
  fetchSessions: (params?: { host_id?: string; status?: string }) => Promise<void>;
  createSession: (data: CreateSessionRequest) => Promise<Session>;
  stopSession: (id: string) => Promise<void>;
  deleteSession: (id: string) => Promise<void>;
}

export const useSessionStore = create<SessionStore>((set, get) => ({
  sessions: [],
  loading: false,
  error: null,

  fetchSessions: async (params) => {
    set({ loading: true, error: null });
    try {
      const sessions = await sessionsApi.list(params);
      // Only update if data has changed to avoid unnecessary re-renders
      const current = get().sessions;
      const hasChanged = JSON.stringify(current) !== JSON.stringify(sessions);
      if (hasChanged) {
        set({ sessions, loading: false });
      } else {
        set({ loading: false });
      }
    } catch (e) {
      set({ error: (e as Error).message, loading: false });
    }
  },

  createSession: async (data: CreateSessionRequest) => {
    const session = await sessionsApi.create(data);
    set({ sessions: [session, ...get().sessions] });
    return session;
  },

  stopSession: async (id: string) => {
    await sessionsApi.stop(id);
    set({
      sessions: get().sessions.map((s) =>
        s.id === id ? { ...s, status: 'stopped' as const } : s
      ),
    });
  },

  deleteSession: async (id: string) => {
    await sessionsApi.del(id);
    set({ sessions: get().sessions.filter((s) => s.id !== id) });
  },
}));
