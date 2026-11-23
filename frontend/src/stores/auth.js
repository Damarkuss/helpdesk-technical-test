import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { authApi } from '../api'
import { ApiError } from '../api/http'

const STORAGE_KEY = 'helpdesk.token'

// localStorage can throw (private mode, blocked storage), so guard every access.
function readToken() {
  try {
    return localStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}
function writeToken(value) {
  try {
    if (value) localStorage.setItem(STORAGE_KEY, value)
    else localStorage.removeItem(STORAGE_KEY)
  } catch {
    /* ignore: the session just will not survive a reload */
  }
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(readToken())
  const user = ref(null)

  const isAuthenticated = computed(() => !!token.value && !!user.value)
  const role = computed(() => user.value?.role ?? null)
  const isStaff = computed(() => role.value === 'agent' || role.value === 'admin')
  const isAdmin = computed(() => role.value === 'admin')

  function setSession(result) {
    token.value = result.token
    user.value = result.user
    writeToken(result.token)
  }

  async function login(email, password) {
    setSession(await authApi.login(email, password))
  }

  async function register(name, email, password) {
    setSession(await authApi.register(name, email, password))
  }

  /** Restores the user for a token found in storage (page reload). */
  async function restore() {
    if (!token.value || user.value) return
    try {
      user.value = await authApi.me()
    } catch (err) {
      // Only a rejected token ends the session; a network blip should not log people out.
      if (err instanceof ApiError && err.status === 401) logout()
      else throw err
    }
  }

  function logout() {
    token.value = null
    user.value = null
    writeToken(null)
  }

  return { token, user, isAuthenticated, role, isStaff, isAdmin, login, register, restore, logout }
})
