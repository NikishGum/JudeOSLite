export class ApiError extends Error {
  constructor(public readonly status: number, public readonly code: string, message: string) {
    super(message);
    this.name = 'ApiError';
  }
}

// Same-origin API requests; tokens are never kept in localStorage.
// Mutating requests will gain the session/CSRF contract in V1-02.
export async function apiGet<T>(path: `/api/${string}`, signal?: AbortSignal): Promise<T> {
  const response = await fetch(path, {
    method: 'GET', credentials: 'same-origin', cache: 'no-store',
    headers: { Accept: 'application/json' }, signal,
  });
  const body: unknown = await response.json();
  if (!response.ok) {
    if (typeof body === 'object' && body !== null && 'error' in body) {
      const error = body.error;
      if (typeof error === 'object' && error !== null && 'code' in error && 'message' in error &&
          typeof error.code === 'string' && typeof error.message === 'string') {
        throw new ApiError(response.status, error.code, error.message);
      }
    }
    throw new ApiError(response.status, 'unexpected_error', 'Не удалось выполнить запрос.');
  }
  return body as T;
}
