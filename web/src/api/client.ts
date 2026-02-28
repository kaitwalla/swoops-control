const BASE = '/api/v1';

// API key is injected into the page by the server or set via localStorage
function getApiKey(): string {
  // Check localStorage first (set via settings page or login)
  const stored = localStorage.getItem('swoops_api_key');
  if (stored) return stored;

  // Check meta tag (server-injected)
  const meta = document.querySelector('meta[name="swoops-api-key"]');
  if (meta) return meta.getAttribute('content') || '';

  return '';
}

export function setApiKey(key: string) {
  localStorage.setItem('swoops_api_key', key);
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const apiKey = getApiKey();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (apiKey) {
    headers['Authorization'] = `Bearer ${apiKey}`;
  }

  const res = await fetch(`${BASE}${path}`, {
    headers,
    ...options,
  });
  if (!res.ok) {
    const body = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(body.error || res.statusText);
  }
  if (res.status === 204) return undefined as T;
  return res.json();
}

export const api = {
  get: <T>(path: string) => request<T>(path),
  post: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'POST', body: JSON.stringify(body) }),
  put: <T>(path: string, body: unknown) =>
    request<T>(path, { method: 'PUT', body: JSON.stringify(body) }),
  del: <T>(path: string) => request<T>(path, { method: 'DELETE' }),
};
