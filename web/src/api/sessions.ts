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
};
