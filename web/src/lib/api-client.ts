export class ApiError extends Error {
  code: string;
  status: number;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
  }
}

type ApiFetchOptions = RequestInit & {
  redirectOnUnauthorized?: boolean;
};

export async function apiFetch<T = unknown>(
  path: string,
  options: ApiFetchOptions = {},
): Promise<T> {
  const { redirectOnUnauthorized = true, ...fetchOptions } = options;
  const headers = new Headers(fetchOptions.headers);
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json');
  }
  const method = (fetchOptions.method ?? 'GET').toUpperCase();
  if (method !== 'GET' && method !== 'HEAD') {
    headers.set('X-Requested-With', 'xhr');
  }

  const res = await fetch(path, {
    ...fetchOptions,
    headers,
    credentials: 'include',
  });

  if (res.status === 401) {
    if (redirectOnUnauthorized && typeof globalThis.location !== 'undefined') {
      globalThis.location.href = '/login';
    }
    throw new ApiError(401, 'UNAUTHORIZED', 'Not authenticated');
  }

  if (!res.ok) {
    let code = 'UNKNOWN';
    let message = res.statusText;
    try {
      const body = await res.json();
      code = body.code ?? code;
      message = body.message ?? body.error ?? message;
    } catch {}
    throw new ApiError(res.status, code, message);
  }

  if (res.status === 204) return undefined as T;
  return res.json();
}
