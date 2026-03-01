import { describe, it, expect, beforeEach, vi } from 'vitest';
import { versionApi } from './version';
import { api } from './client';

vi.mock('./client');

describe('versionApi', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch version information', async () => {
    const mockVersion = {
      version: '1.0.0',
      git_commit: 'abc123',
      build_time: '2024-01-01T00:00:00Z',
      latest_version: '1.1.0',
      update_available: true,
      update_url: 'https://github.com/example/repo/releases/tag/v1.1.0',
      release_notes: 'New features and bug fixes',
    };

    vi.mocked(api.get).mockResolvedValue(mockVersion);

    const result = await versionApi.get();

    expect(api.get).toHaveBeenCalledWith('/version');
    expect(result).toEqual(mockVersion);
  });

  it('should handle version without update info', async () => {
    const mockVersion = {
      version: '1.0.0',
      git_commit: 'abc123',
      build_time: '2024-01-01T00:00:00Z',
    };

    vi.mocked(api.get).mockResolvedValue(mockVersion);

    const result = await versionApi.get();

    expect(result).toEqual(mockVersion);
    expect(result.update_available).toBeUndefined();
  });

  it('should handle failed update check', async () => {
    const mockVersion = {
      version: '1.0.0',
      git_commit: 'abc123',
      build_time: '2024-01-01T00:00:00Z',
      update_check_failed: true,
    };

    vi.mocked(api.get).mockResolvedValue(mockVersion);

    const result = await versionApi.get();

    expect(result.update_check_failed).toBe(true);
  });
});
