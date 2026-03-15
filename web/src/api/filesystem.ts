import { api } from './client';

export interface DirectoryEntry {
  name: string;
  path: string;
  is_dir: boolean;
}

export interface ListDirectoriesRequest {
  path: string;
}

export interface CreateDirectoryRequest {
  path: string;
  name: string;
}

export interface CloneRepositoryRequest {
  path: string;
  repo_url: string;
  folder_name?: string;
}

export const filesystemApi = {
  async listDirectories(hostId: string, path: string): Promise<DirectoryEntry[]> {
    return api.post<DirectoryEntry[]>(`/hosts/${hostId}/directories/list`, { path });
  },

  async createDirectory(hostId: string, path: string, name: string): Promise<DirectoryEntry> {
    return api.post<DirectoryEntry>(`/hosts/${hostId}/directories/create`, { path, name });
  },

  async cloneRepository(hostId: string, data: CloneRepositoryRequest): Promise<DirectoryEntry> {
    return api.post<DirectoryEntry>(`/hosts/${hostId}/repositories/clone`, data);
  },
};
