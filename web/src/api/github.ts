import { api } from './client';

export interface GitHubRepo {
  id: number;
  name: string;
  full_name: string;
  description?: string;
  private: boolean;
  html_url: string;
  clone_url: string;
  ssh_url: string;
  default_branch: string;
}

export interface CreateGitHubRepoRequest {
  name: string;
  description?: string;
  private: boolean;
}

export interface UpdateGitHubTokenRequest {
  token: string;
}

export const githubApi = {
  async updateToken(token: string): Promise<void> {
    await api.put('/github/token', { token });
  },

  async listRepos(): Promise<GitHubRepo[]> {
    return api.get<GitHubRepo[]>('/github/repos');
  },

  async createRepo(data: CreateGitHubRepoRequest): Promise<GitHubRepo> {
    return api.post<GitHubRepo>('/github/repos', data);
  },
};
