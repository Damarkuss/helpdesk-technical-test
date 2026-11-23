import { http } from './http'

export const authApi = {
  login: (email, password) => http.post('/api/auth/login', { email, password }, { auth: false }),
  register: (name, email, password) => http.post('/api/auth/register', { name, email, password }, { auth: false }),
  me: () => http.get('/api/me'),
}

export const metaApi = {
  get: () => http.get('/api/meta', { auth: false }),
}

export const ticketsApi = {
  list: (params, opts) => http.get('/api/tickets', { query: params, ...opts }),
  get: (id) => http.get(`/api/tickets/${id}`),
  create: (payload) => http.post('/api/tickets', payload),
  update: (id, patch) => http.patch(`/api/tickets/${id}`, patch),
  remove: (id) => http.delete(`/api/tickets/${id}`),
  addComment: (id, body, isInternal) => http.post(`/api/tickets/${id}/comments`, { body, is_internal: isInternal }),
}

export const usersApi = {
  list: (params) => http.get('/api/users', { query: params }),
  assignable: () => http.get('/api/agents'),
  changeRole: (id, role) => http.patch(`/api/users/${id}/role`, { role }),
}
