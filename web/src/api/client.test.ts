import { describe, it, expect, beforeEach, vi } from 'vitest';
import { api, setApiKey } from './client';

describe('API client', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.clear();
    // Reset fetch mock
    globalThis.fetch = vi.fn();
  });

  describe('setApiKey', () => {
    it('should store API key in localStorage', () => {
      setApiKey('test-api-key');
      expect(localStorage.getItem('swoops_api_key')).toBe('test-api-key');
    });
  });

  describe('request headers', () => {
    it('should include Authorization header when API key is set', async () => {
      setApiKey('test-api-key');

      vi.mocked(fetch).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: 'test' }),
      } as Response);

      await api.get('/test');

      expect(fetch).toHaveBeenCalledWith(
        '/api/v1/test',
        expect.objectContaining({
          headers: expect.objectContaining({
            'Authorization': 'Bearer test-api-key',
            'Content-Type': 'application/json',
          }),
        })
      );
    });

    it('should not include Authorization header when API key is not set', async () => {
      vi.mocked(fetch).mockResolvedValueOnce({
        ok: true,
        json: async () => ({ data: 'test' }),
      } as Response);

      await api.get('/test');

      expect(fetch).toHaveBeenCalledWith(
        '/api/v1/test',
        expect.objectContaining({
          headers: {
            'Content-Type': 'application/json',
          },
        })
      );
    });
  });

  describe('GET requests', () => {
    it('should make GET request and return JSON', async () => {
      const mockData = { id: '1', name: 'Test' };
      vi.mocked(fetch).mockResolvedValueOnce({
        ok: true,
        json: async () => mockData,
      } as Response);

      const result = await api.get('/test');

      expect(fetch).toHaveBeenCalledWith('/api/v1/test', expect.any(Object));
      expect(result).toEqual(mockData);
    });

    it('should handle 204 No Content response', async () => {
      vi.mocked(fetch).mockResolvedValueOnce({
        ok: true,
        status: 204,
        json: async () => ({}),
      } as Response);

      const result = await api.get('/test');

      expect(result).toBeUndefined();
    });
  });

  describe('POST requests', () => {
    it('should make POST request with body', async () => {
      const requestBody = { name: 'Test' };
      const mockResponse = { id: '1', ...requestBody };

      vi.mocked(fetch).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      } as Response);

      const result = await api.post('/test', requestBody);

      expect(fetch).toHaveBeenCalledWith(
        '/api/v1/test',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify(requestBody),
        })
      );
      expect(result).toEqual(mockResponse);
    });
  });

  describe('PUT requests', () => {
    it('should make PUT request with body', async () => {
      const requestBody = { name: 'Updated' };
      const mockResponse = { id: '1', ...requestBody };

      vi.mocked(fetch).mockResolvedValueOnce({
        ok: true,
        json: async () => mockResponse,
      } as Response);

      const result = await api.put('/test/1', requestBody);

      expect(fetch).toHaveBeenCalledWith(
        '/api/v1/test/1',
        expect.objectContaining({
          method: 'PUT',
          body: JSON.stringify(requestBody),
        })
      );
      expect(result).toEqual(mockResponse);
    });
  });

  describe('DELETE requests', () => {
    it('should make DELETE request', async () => {
      vi.mocked(fetch).mockResolvedValueOnce({
        ok: true,
        status: 204,
      } as Response);

      await api.del('/test/1');

      expect(fetch).toHaveBeenCalledWith(
        '/api/v1/test/1',
        expect.objectContaining({
          method: 'DELETE',
        })
      );
    });
  });

  describe('error handling', () => {
    it('should throw error with message from response body', async () => {
      const errorMessage = 'Resource not found';
      vi.mocked(fetch).mockResolvedValueOnce({
        ok: false,
        statusText: 'Not Found',
        json: async () => ({ error: errorMessage }),
      } as Response);

      await expect(api.get('/test')).rejects.toThrow(errorMessage);
    });

    it('should throw error with statusText when response body is invalid', async () => {
      vi.mocked(fetch).mockResolvedValueOnce({
        ok: false,
        statusText: 'Internal Server Error',
        json: async () => {
          throw new Error('Invalid JSON');
        },
      } as unknown as Response);

      await expect(api.get('/test')).rejects.toThrow('Internal Server Error');
    });

    it('should handle network errors', async () => {
      vi.mocked(fetch).mockRejectedValueOnce(new Error('Network error'));

      await expect(api.get('/test')).rejects.toThrow('Network error');
    });
  });
});
