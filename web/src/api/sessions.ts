import { api } from './client';
import type { Session, CreateSessionRequest } from '../types/session';

export const sessionsApi = {
  list: (params?: { host_id?: string; status?: string }) => {
    const query = new URLSearchParams();
    if (params?.host_id) query.set('host_id', params.host_id);
    if (params?.status) query.set('status', params.status);
    const qs = query.toString();
    return api.get<Session[]>(`/sessions${qs ? '?' + qs : ''}`);
  },
  get: (id: string) => api.get<Session>(`/sessions/${id}`),
  create: (data: CreateSessionRequest) => api.post<Session>('/sessions', data),
  stop: (id: string) => api.post<{ status: string }>(`/sessions/${id}/stop`, {}),
  del: (id: string) => api.del<void>(`/sessions/${id}`),
  sendInput: (id: string, input: string) =>
    api.post<{ status: string }>(`/sessions/${id}/input`, { input }),
  getOutput: (id: string) => api.get<{ output: string }>(`/sessions/${id}/output`),
};

// WebSocket output stream
export function createOutputWebSocket(sessionId: string): WebSocket {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const host = window.location.host;
  const apiKey = localStorage.getItem('swoops_api_key') || '';
  // Pass API key as query param since WebSocket doesn't support custom headers in browser
  const ws = new WebSocket(
    `${protocol}//${host}/api/v1/ws/sessions/${sessionId}/output?token=${encodeURIComponent(apiKey)}`
  );
  return ws;
}
