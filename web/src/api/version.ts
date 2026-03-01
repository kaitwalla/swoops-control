import { api } from './client';

export interface VersionInfo {
  version: string;
  git_commit: string;
  build_time: string;
  latest_version?: string;
  update_available?: boolean;
  update_url?: string;
  release_notes?: string;
  update_check_failed?: boolean;
}

export const versionApi = {
  get: () => api.get<VersionInfo>('/version'),
};
