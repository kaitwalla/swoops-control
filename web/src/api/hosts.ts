import { api } from './client';
import type { Host, CreateHostRequest } from '../types/host';

export const hostsApi = {
  list: () => api.get<Host[]>('/hosts'),
  get: (id: string) => api.get<Host>(`/hosts/${id}`),
  create: (data: CreateHostRequest) => api.post<Host>('/hosts', data),
  update: (id: string, data: Partial<CreateHostRequest>) => api.put<Host>(`/hosts/${id}`, data),
  del: (id: string) => api.del<void>(`/hosts/${id}`),
};
