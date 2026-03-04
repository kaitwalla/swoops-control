import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { User, LoginRequest } from '../types/auth';
import { api } from '../api/client';

interface AuthStore {
  user: User | null;
  token: string | null;
  loading: boolean;
  error: string | null;
  isInitialized: boolean; // Tracks if we've attempted to fetch current user

  login: (credentials: LoginRequest) => Promise<void>;
  logout: () => Promise<void>;
  fetchCurrentUser: () => Promise<void>;
  clearError: () => void;
}

export const useAuthStore = create<AuthStore>()(
  persist(
    (set, get) => ({
      user: null,
      token: null,
      loading: false,
      error: null,
      isInitialized: false,

      login: async (credentials: LoginRequest) => {
        set({ loading: true, error: null });
        try {
          const response = await api.post<{ user: User; token: string }>(
            '/auth/login',
            credentials
          );

          // Store token in localStorage for API client
          localStorage.setItem('swoops_session_token', response.token);

          set({
            user: response.user,
            token: response.token,
            loading: false,
            error: null,
            isInitialized: true
          });
        } catch (err) {
          set({
            error: err instanceof Error ? err.message : 'Login failed',
            loading: false,
            isInitialized: true
          });
          throw err;
        }
      },

      logout: async () => {
        try {
          await api.post('/auth/logout', {});
        } catch (err) {
          // Ignore logout errors
        }

        // Clear token from localStorage
        localStorage.removeItem('swoops_session_token');
        localStorage.removeItem('swoops_api_key');

        set({ user: null, token: null, error: null, isInitialized: true });
      },

      fetchCurrentUser: async () => {
        set({ loading: true, error: null });
        try {
          const user = await api.get<User>('/auth/me');
          set({ user, loading: false, error: null, isInitialized: true });
        } catch (err) {
          // Only clear token on authentication errors (401/403)
          const errorMessage = err instanceof Error ? err.message : 'Failed to fetch user';
          const isAuthError =
            errorMessage.includes('authentication') ||
            errorMessage.includes('not authenticated') ||
            errorMessage.includes('Unauthorized') ||
            errorMessage.includes('Forbidden');

          set({
            user: isAuthError ? null : get().user,
            token: isAuthError ? null : get().token,
            error: errorMessage,
            loading: false,
            isInitialized: true
          });
        }
      },

      clearError: () => set({ error: null }),
    }),
    {
      name: 'auth-storage',
      partialize: (state) => ({
        token: state.token,
        user: state.user
      }),
    }
  )
);
