<script setup>
import { reactive, ref, watch } from 'vue'
import { usersApi } from '../api'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import PageState from '../components/PageState.vue'
import PaginationBar from '../components/PaginationBar.vue'
import { useDebouncedRef } from '../composables/useDebouncedRef'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import { ROLES, ROLE_LABELS, formatDateTime } from '../utils/format'

const PAGE_SIZE = 10

const auth = useAuthStore()
const toast = useToastStore()

const state = reactive({ loading: true, error: '', users: [], total: 0, totalPages: 0 })
const page = ref(1)
const search = ref('')
const roleFilter = ref('')
const debouncedSearch = useDebouncedRef(search, 300)

const pending = ref(null) // { user, role } waiting for confirmation
const saving = ref(false)

let seq = 0
async function load() {
  const mine = ++seq
  state.loading = true
  state.error = ''
  try {
    const res = await usersApi.list({ q: debouncedSearch.value.trim(), role: roleFilter.value, page: page.value, page_size: PAGE_SIZE })
    if (mine !== seq) return
    Object.assign(state, { users: res.data, total: res.total, totalPages: res.total_pages })
  } catch (err) {
    if (mine === seq) state.error = err.message
  } finally {
    if (mine === seq) state.loading = false
  }
}

watch([debouncedSearch, roleFilter], () => {
  page.value = 1
  load()
})
watch(page, load, { immediate: true })

function askChange(user, event) {
  const el = event.target
  const role = el.value
  el.value = user.role // keep showing the real role until the change is confirmed
  if (role !== user.role) pending.value = { user, role }
}

async function confirmChange() {
  const { user, role } = pending.value
  saving.value = true
  try {
    const updated = await usersApi.changeRole(user.id, role)
    user.role = updated.role
    toast.success(`${user.name} is now ${ROLE_LABELS[updated.role].toLowerCase()}.`)
  } catch (err) {
    toast.error(err.message)
  } finally {
    saving.value = false
    pending.value = null
  }
}
</script>

<template>
  <section>
    <div class="page-head">
      <div>
        <h1>Users</h1>
        <p class="muted">Manage who can create tickets, handle them, or administer the helpdesk.</p>
      </div>
    </div>

    <form class="filters card" role="search" @submit.prevent>
      <div class="filter">
        <label for="u-search">Search name or email</label>
        <input id="u-search" v-model="search" type="search" autocomplete="off" />
      </div>
      <div class="filter">
        <label for="u-role">Role</label>
        <select id="u-role" v-model="roleFilter">
          <option value="">All roles</option>
          <option v-for="r in ROLES" :key="r" :value="r">{{ ROLE_LABELS[r] }}</option>
        </select>
      </div>
    </form>

    <PageState
      :loading="state.loading && !state.users.length"
      :error="state.error"
      :empty="!state.loading && !state.users.length"
      empty-title="No users found"
      empty-text="Try a different search."
      @retry="load"
    >
      <div class="card table-card" :class="{ 'is-refreshing': state.loading }" :aria-busy="state.loading">
        <table class="table responsive">
          <thead>
            <tr>
              <th scope="col">Name</th>
              <th scope="col">Email</th>
              <th scope="col">Role</th>
              <th scope="col">Joined</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="u in state.users" :key="u.id">
              <td data-label="Name">
                {{ u.name }} <span v-if="u.id === auth.user.id" class="role-chip">You</span>
              </td>
              <td data-label="Email">{{ u.email }}</td>
              <td data-label="Role">
                <label class="sr-only" :for="`role-${u.id}`">Role of {{ u.name }}</label>
                <select
                  :id="`role-${u.id}`"
                  :value="u.role"
                  :disabled="u.id === auth.user.id"
                  :title="u.id === auth.user.id ? 'You cannot change your own role' : ''"
                  @change="askChange(u, $event)"
                >
                  <option v-for="r in ROLES" :key="r" :value="r">{{ ROLE_LABELS[r] }}</option>
                </select>
              </td>
              <td data-label="Joined">{{ formatDateTime(u.created_at) }}</td>
            </tr>
          </tbody>
        </table>
        <PaginationBar :page="page" :total-pages="state.totalPages" :total="state.total" :page-size="PAGE_SIZE" @change="(p) => (page = p)" />
      </div>
    </PageState>

    <ConfirmDialog
      :open="!!pending"
      title="Change role?"
      :message="pending ? `${pending.user.name} will change from ${ROLE_LABELS[pending.user.role]} to ${ROLE_LABELS[pending.role]}. This takes effect immediately.` : ''"
      confirm-label="Change role"
      :busy="saving"
      @confirm="confirmChange"
      @cancel="pending = null"
    />
  </section>
</template>
