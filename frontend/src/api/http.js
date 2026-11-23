// Central HTTP client. Every API call goes through request(), which:
//  - attaches the bearer token,
//  - turns error responses into ApiError (status, code, message, per-field errors),
//  - calls the registered handler on 401 so the app can log the user out in one place.

const BASE = import.meta.env?.VITE_API_URL ?? ''

export class ApiError extends Error {
  constructor({ status, code, message, fields }) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.fields = fields ?? {}
  }
}

let tokenProvider = () => null
let unauthorizedHandler = () => {}

export function setTokenProvider(fn) {
  tokenProvider = fn
}

export function setUnauthorizedHandler(fn) {
  unauthorizedHandler = fn
}

function buildUrl(path, query) {
  const url = new URL(BASE + path, window.location.origin)
  for (const [key, value] of Object.entries(query ?? {})) {
    if (value !== undefined && value !== null && value !== '') url.searchParams.set(key, value)
  }
  return url.pathname + url.search
}

export async function request(method, path, { query, body, auth = true, signal } = {}) {
  const headers = { Accept: 'application/json' }
  const token = auth ? tokenProvider() : null
  if (token) headers.Authorization = `Bearer ${token}`
  if (body !== undefined) headers['Content-Type'] = 'application/json'

  let res
  try {
    res = await fetch(buildUrl(path, query), {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal,
    })
  } catch (err) {
    if (err?.name === 'AbortError') throw err
    throw new ApiError({ status: 0, code: 'network_error', message: 'Cannot reach the server. Check your connection and try again.' })
  }

  const isJson = res.headers.get('content-type')?.includes('application/json')
  const data = res.status === 204 || !isJson ? null : await res.json().catch(() => null)

  if (!res.ok) {
    // A 401 on a request that carried a token means the session is no longer valid.
    if (res.status === 401 && token) unauthorizedHandler()
    const e = data?.error
    throw new ApiError({
      status: res.status,
      code: e?.code ?? 'unknown_error',
      message: e?.message ?? `Request failed (${res.status})`,
      fields: e?.fields,
    })
  }
  return data
}

export const http = {
  get: (path, opts) => request('GET', path, opts),
  post: (path, body, opts) => request('POST', path, { ...opts, body }),
  patch: (path, body, opts) => request('PATCH', path, { ...opts, body }),
  delete: (path, opts) => request('DELETE', path, opts),
}
