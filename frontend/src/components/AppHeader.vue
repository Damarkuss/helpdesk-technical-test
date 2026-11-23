<script setup>
import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import { ROLE_LABELS } from '../utils/format'

const auth = useAuthStore()
const toast = useToastStore()
const route = useRoute()
const router = useRouter()

const menuOpen = ref(false)
watch(() => route.fullPath, () => (menuOpen.value = false))

const links = computed(() => {
  const items = [{ to: { name: 'tickets' }, label: 'Tickets' }]
  items.push({ to: { name: 'ticket-new' }, label: 'New ticket' })
  if (auth.isAdmin) items.push({ to: { name: 'users' }, label: 'Users' })
  return items
})

async function signOut() {
  auth.logout()
  toast.info('You have been signed out.')
  await router.push({ name: 'login' })
}
</script>

<template>
  <header class="app-header">
    <div class="container header-inner">
      <RouterLink :to="{ name: 'tickets' }" class="brand">
        <span class="brand-mark" aria-hidden="true">H</span>
        Helpdesk
      </RouterLink>

      <template v-if="auth.isAuthenticated">
        <button class="menu-toggle" :aria-expanded="menuOpen" aria-controls="main-nav" @click="menuOpen = !menuOpen">
          <span class="sr-only">Menu</span>
          <span aria-hidden="true">☰</span>
        </button>
        <nav id="main-nav" class="nav" :class="{ open: menuOpen }" aria-label="Main">
          <RouterLink v-for="l in links" :key="l.label" :to="l.to" class="nav-link">{{ l.label }}</RouterLink>
          <div class="nav-user">
            <span class="user-name">{{ auth.user.name }}</span>
            <span class="role-chip">{{ ROLE_LABELS[auth.user.role] }}</span>
            <button class="btn btn-ghost btn-sm" @click="signOut">Sign out</button>
          </div>
        </nav>
      </template>
    </div>
  </header>
</template>
