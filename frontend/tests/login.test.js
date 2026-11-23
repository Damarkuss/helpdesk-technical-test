import { mount, flushPromises } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory } from 'vue-router'
import { createAppRouter } from '../src/router'
import { ApiError } from '../src/api/http'
import LoginView from '../src/views/LoginView.vue'
import { useAuthStore } from '../src/stores/auth'

vi.mock('../src/api', () => ({
  authApi: { login: vi.fn(), register: vi.fn(), me: vi.fn() },
  metaApi: {},
  ticketsApi: {},
  usersApi: {},
}))
import { authApi } from '../src/api'

async function mountLogin() {
  const pinia = createPinia()
  setActivePinia(pinia)
  const router = createAppRouter(createMemoryHistory())
  await router.push('/login')
  await router.isReady()
  const wrapper = mount(LoginView, { global: { plugins: [pinia, router] } })
  return { wrapper, router, auth: useAuthStore() }
}

describe('LoginView', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('validates on the client and does not call the API when the form is invalid', async () => {
    const { wrapper } = await mountLogin()
    await wrapper.find('form').trigger('submit')

    expect(authApi.login).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Enter your email address.')
    expect(wrapper.text()).toContain('Enter your password.')
  })

  it('rejects a malformed email address', async () => {
    const { wrapper } = await mountLogin()
    await wrapper.find('#email').setValue('not-an-email')
    await wrapper.find('#password').setValue('secret123')
    await wrapper.find('form').trigger('submit')

    expect(authApi.login).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('Enter a valid email address.')
  })

  it('shows the server error when credentials are wrong', async () => {
    authApi.login.mockRejectedValue(new ApiError({ status: 401, code: 'unauthenticated', message: 'invalid email or password' }))
    const { wrapper, auth } = await mountLogin()
    await wrapper.find('#email').setValue('rina@helpdesk.local')
    await wrapper.find('#password').setValue('wrong-password')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(wrapper.find('.alert-error').text()).toBe('invalid email or password')
    expect(auth.isAuthenticated).toBe(false)
  })

  it('signs in and goes to the ticket list on success', async () => {
    authApi.login.mockResolvedValue({ token: 'abc', user: { id: 1, name: 'Rina', role: 'requester' } })
    const { wrapper, router, auth } = await mountLogin()
    await wrapper.find('#email').setValue('rina@helpdesk.local')
    await wrapper.find('#password').setValue('Password123!')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(authApi.login).toHaveBeenCalledWith('rina@helpdesk.local', 'Password123!')
    expect(auth.isAuthenticated).toBe(true)
    // The ticket list is a lazy-loaded route, so wait for the navigation to finish.
    await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('tickets'))
  })

  it('ignores off-site redirect targets after login', async () => {
    authApi.login.mockResolvedValue({ token: 'abc', user: { id: 1, name: 'Rina', role: 'requester' } })
    const { wrapper, router } = await mountLogin()
    await router.replace({ name: 'login', query: { redirect: '//evil.example.com' } })
    await wrapper.find('#email').setValue('rina@helpdesk.local')
    await wrapper.find('#password').setValue('Password123!')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    await vi.waitFor(() => expect(router.currentRoute.value.name).toBe('tickets'))
    expect(router.currentRoute.value.fullPath).not.toContain('evil.example.com')
  })
})
