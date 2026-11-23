import { createMemoryHistory } from 'vue-router'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createAppRouter } from '../src/router'
import { useAuthStore } from '../src/stores/auth'

vi.mock('../src/api', () => ({
  authApi: { login: vi.fn(), register: vi.fn(), me: vi.fn() },
  metaApi: {},
  ticketsApi: {},
  usersApi: {},
}))

function setup({ user = null } = {}) {
  setActivePinia(createPinia())
  const auth = useAuthStore()
  if (user) {
    auth.token = 'tok'
    auth.user = user
  }
  return { auth, router: createAppRouter(createMemoryHistory()) }
}

const requester = { id: 1, name: 'Rina', role: 'requester' }
const admin = { id: 2, name: 'Adi', role: 'admin' }

describe('navigation guard', () => {
  beforeEach(() => localStorage.clear())

  it('sends anonymous visitors to the login page and remembers where they were going', async () => {
    const { router } = setup()
    await router.push('/tickets/42')
    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/tickets/42')
  })

  it('lets signed-in users open protected pages', async () => {
    const { router } = setup({ user: requester })
    await router.push('/tickets')
    expect(router.currentRoute.value.name).toBe('tickets')
  })

  it('blocks non-admins from the users page', async () => {
    const { router } = setup({ user: requester })
    await router.push('/users')
    expect(router.currentRoute.value.name).toBe('forbidden')
  })

  it('lets admins open the users page', async () => {
    const { router } = setup({ user: admin })
    await router.push('/users')
    expect(router.currentRoute.value.name).toBe('users')
  })

  it('keeps signed-in users away from login and register', async () => {
    const { router } = setup({ user: requester })
    await router.push('/login')
    expect(router.currentRoute.value.name).toBe('tickets')
  })

  it('restores the session from a stored token on reload', async () => {
    const { auth, router } = setup()
    const { authApi } = await import('../src/api')
    authApi.me.mockResolvedValue(requester)
    auth.token = 'stored-token'

    await router.push('/tickets')

    expect(authApi.me).toHaveBeenCalled()
    expect(router.currentRoute.value.name).toBe('tickets')
    expect(auth.user.name).toBe('Rina')
  })

  it('drops a rejected stored token and shows the login page', async () => {
    const { auth, router } = setup()
    const { authApi } = await import('../src/api')
    const { ApiError } = await import('../src/api/http')
    authApi.me.mockRejectedValue(new ApiError({ status: 401, code: 'unauthenticated', message: 'expired' }))
    auth.token = 'stale-token'

    await router.push('/tickets')

    expect(router.currentRoute.value.name).toBe('login')
    expect(auth.token).toBeNull()
  })
})
