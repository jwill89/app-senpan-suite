/**
 * Typed API client. Replaces the untyped `api()` helper from the legacy app.js.
 *
 * All requests are same-origin (`/api/...`): in dev, Vite proxies `/api` to the
 * Go backend on :8080; in production, Apache serves the SPA and proxies `/api`.
 * Cookies (the admin session) are always included.
 */

const BASE = '/api'

/** Error carrying the server's `{"error": "..."}` message and HTTP status. */
export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, init: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}/${path}`, {
    credentials: 'include',
    ...init,
  })
  // Endpoints always return JSON (including errors).
  const data = (await res.json().catch(() => ({}))) as { error?: string } & T
  if (!res.ok) {
    throw new ApiError(data.error || 'Request failed', res.status)
  }
  return data
}

/** GET a JSON resource. */
export function apiGet<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'GET' })
}

/** POST a JSON body and parse the JSON response. */
export function apiPost<T>(path: string, body: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
}

/** POST multipart form data (e.g. image upload); does not set Content-Type. */
export function apiPostForm<T>(path: string, form: FormData): Promise<T> {
  return request<T>(path, { method: 'POST', body: form })
}
