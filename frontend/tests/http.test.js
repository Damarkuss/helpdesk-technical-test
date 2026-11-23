import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, http, setTokenProvider, setUnauthorizedHandler } from '../src/api/http'

function jsonResponse(status, body) {
  return new Response(body === null ? null : JSON.stringify(body), {
    status,
    headers: body === null ? {} : { 'Content-Type': 'application/json' },
  })
}

describe('http client', () => {
  const unauthorized = vi.fn()

  beforeEach(() => {
    unauthorized.mockReset()
    setTokenProvider(() => 'tok-123')
    setUnauthorizedHandler(unauthorized)
  })
  afterEach(() => vi.unstubAllGlobals())

  it('sends the bearer token, JSON body and query string (skipping empty values)', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { ok: true }))
    vi.stubGlobal('fetch', fetchMock)

    await http.get('/api/tickets', { query: { q: 'printer', status: '', page: 2 } })
    await http.post('/api/tickets', { title: 'x' })

    const [url, init] = fetchMock.mock.calls[0]
    expect(url).toBe('/api/tickets?q=printer&page=2')
    expect(init.headers.Authorization).toBe('Bearer tok-123')

    const [, postInit] = fetchMock.mock.calls[1]
    expect(postInit.body).toBe('{"title":"x"}')
    expect(postInit.headers['Content-Type']).toBe('application/json')
  })

  it('turns an error response into an ApiError with code and field messages', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(422, { error: { code: 'validation_error', message: 'bad input', fields: { title: 'is required' } } }),
      ),
    )
    const err = await http.post('/api/tickets', {}).catch((e) => e)

    expect(err).toBeInstanceOf(ApiError)
    expect(err).toMatchObject({ status: 422, code: 'validation_error', message: 'bad input', fields: { title: 'is required' } })
    expect(unauthorized).not.toHaveBeenCalled()
  })

  it('calls the unauthorized handler on 401 when a token was sent', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(401, { error: { code: 'unauthenticated', message: 'expired' } })))
    await expect(http.get('/api/me')).rejects.toBeInstanceOf(ApiError)
    expect(unauthorized).toHaveBeenCalledTimes(1)
  })

  it('does not treat a failed login (401, no token sent) as an expired session', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(401, { error: { code: 'unauthenticated', message: 'invalid email or password' } })))
    const err = await http.post('/api/auth/login', {}, { auth: false }).catch((e) => e)
    expect(err.message).toBe('invalid email or password')
    expect(unauthorized).not.toHaveBeenCalled()
  })

  it('reports network failures with a friendly message', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    const err = await http.get('/api/me').catch((e) => e)
    expect(err).toMatchObject({ status: 0, code: 'network_error' })
  })

  it('returns null for 204 responses', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(jsonResponse(204, null)))
    await expect(http.delete('/api/tickets/1')).resolves.toBeNull()
  })
})
