import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth'

const routes = [
  { path: '/', redirect: { name: 'tickets' } },
  { path: '/login', name: 'login', component: () => import('../views/LoginView.vue'), meta: { guestOnly: true, title: 'Sign in' } },
  { path: '/register', name: 'register', component: () => import('../views/RegisterView.vue'), meta: { guestOnly: true, title: 'Create account' } },
  { path: '/tickets', name: 'tickets', component: () => import('../views/TicketListView.vue'), meta: { requiresAuth: true, title: 'Tickets' } },
  { path: '/tickets/new', name: 'ticket-new', component: () => import('../views/TicketCreateView.vue'), meta: { requiresAuth: true, title: 'New ticket' } },
  { path: '/tickets/:id(\\d+)', name: 'ticket', component: () => import('../views/TicketDetailView.vue'), props: true, meta: { requiresAuth: true, title: 'Ticket' } },
  { path: '/users', name: 'users', component: () => import('../views/UsersView.vue'), meta: { requiresAuth: true, roles: ['admin'], title: 'Users' } },
  { path: '/forbidden', name: 'forbidden', component: () => import('../views/ForbiddenView.vue'), meta: { title: 'No access' } },
  { path: '/:pathMatch(.*)*', name: 'not-found', component: () => import('../views/NotFoundView.vue'), meta: { title: 'Not found' } },
]

export function createAppRouter(history = createWebHistory()) {
  const router = createRouter({
    history,
    routes,
    scrollBehavior: () => ({ top: 0 }),
  })

  // Navigation guard: login-only pages, admin-only pages, and guest-only pages
  // (login/register are pointless once signed in).
  router.beforeEach(async (to) => {
    const auth = useAuthStore()

    if (auth.token && !auth.user) {
      try {
        await auth.restore()
      } catch {
        // Server unreachable: keep the token but we cannot show protected pages yet.
        if (to.meta.requiresAuth) return { name: 'login', query: { redirect: to.fullPath, reason: 'offline' } }
      }
    }

    if (to.meta.guestOnly && auth.isAuthenticated) return { name: 'tickets' }

    if (to.meta.requiresAuth && !auth.isAuthenticated) {
      return { name: 'login', query: { redirect: to.fullPath } }
    }

    if (to.meta.roles && !to.meta.roles.includes(auth.role)) {
      return { name: 'forbidden' }
    }
  })

  router.afterEach((to) => {
    document.title = to.meta.title ? `${to.meta.title} · Helpdesk` : 'Helpdesk'
  })

  return router
}
