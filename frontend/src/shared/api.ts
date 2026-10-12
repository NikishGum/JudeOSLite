export class ApiError extends Error {
  constructor(public readonly status: number, public readonly code: string, message: string) {
    super(message); this.name = 'ApiError';
  }
}
let csrfToken: string | null = null;
let authGeneration = 0;
export function setCsrfToken(token: string | null) { csrfToken = token; authGeneration += 1; }
// Cookies are HttpOnly. CSRF lives only in memory, never in localStorage.
async function request<T>(path: `/api/${string}`, method: string, body?: unknown, signal?: AbortSignal): Promise<T> {
  const generation = authGeneration;
  const headers: Record<string, string> = { Accept: 'application/json' };
  if (body !== undefined) headers['Content-Type'] = 'application/json';
  if (method !== 'GET' && csrfToken) headers['X-CSRF-Token'] = csrfToken;
  const response = await fetch(path, { method, credentials: 'same-origin', cache: 'no-store', headers, signal,
    body: body === undefined ? undefined : JSON.stringify(body) });
  if (response.status === 401 && path !== '/api/auth/login' && generation === authGeneration && !signal?.aborted) {
    setCsrfToken(null); window.dispatchEvent(new Event('session-ended'));
  }
  const result: unknown = await response.json();
  if (!response.ok) {
    if (typeof result === 'object' && result !== null && 'error' in result) {
      const error = result.error;
      if (typeof error === 'object' && error !== null && 'code' in error && 'message' in error &&
          typeof error.code === 'string' && typeof error.message === 'string') {
        throw new ApiError(response.status, error.code, error.message);
      }
    }
    throw new ApiError(response.status, 'unexpected_error', 'Не удалось выполнить запрос.');
  }
  return result as T;
}
export function apiGet<T>(path: `/api/${string}`, signal?: AbortSignal): Promise<T> { return request<T>(path, 'GET', undefined, signal); }
export function apiPost<T>(path: `/api/${string}`, body?: unknown): Promise<T> { return request<T>(path, 'POST', body); }
export function apiPut<T>(path: `/api/${string}`, body: unknown): Promise<T> { return request<T>(path, 'PUT', body); }
