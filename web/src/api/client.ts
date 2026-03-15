const BASE = '/api/v1';

// Get authentication token (session token or API key)
function getAuthToken(): string {
  // Try session token first (from user login)
  const sessionToken = localStorage.getItem('swoops_session_token');
  if (sessionToken) return sessionToken;

  // Fall back to API key (for API-based access)
  const apiKey = localStorage.getItem('swoops_api_key');
  if (apiKey) return apiKey;

  // Check meta tag (server-injected API key)
  const meta = document.querySelector('meta[name="swoops-api-key"]');
  if (meta) return meta.getAttribute('content') || '';

  return '';
}

export function setApiKey(key: string) {
  localStorage.setItem('swoops_api_key', key);
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const token = getAuthToken();
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  const res = await fetch(`${BASE}${path}`, {
    ...options,
    credentials: 'include', // Include cookies for session auth
    headers: {
      ...headers,
      ...(options?.headers as Record<string, string>),
    },
  });

  // Handle authentication failures
  if (res.status === 401 || res.status === 403) {
    // Clear stale auth tokens
    localStorage.removeItem('swoops_session_token');
    localStorage.removeItem('swoops_api_key');

    // Redirect to login page (unless already on login)
    if (!window.location.pathname.includes('/login')) {
      window.location.href = '/login';
    }

    throw new Error('Authentication expired. Please log in again.');
  }

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
