<script setup>
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ticketsApi } from '../api'
import PageState from '../components/PageState.vue'
import PaginationBar from '../components/PaginationBar.vue'
import PriorityBadge from '../components/PriorityBadge.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useDebouncedRef } from '../composables/useDebouncedRef'
import { useAuthStore } from '../stores/auth'
import { PRIORITIES, PRIORITY_LABELS, STATUSES, STATUS_LABELS, formatDateTime, timeAgo } from '../utils/format'

const PAGE_SIZE = 10

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

// The URL is the single source of truth for filters, sorting and page, so
// refresh, back/forward and shared links all behave.
const query = computed(() => {
  const q = route.query
  return {
    q: String(q.q ?? ''),
    status: String(q.status ?? ''),
    priority: String(q.priority ?? ''),
    assignee: String(q.assignee ?? ''),
    sort: String(q.sort ?? 'created_at'),
    order: q.order === 'asc' ? 'asc' : 'desc',
    page: Math.max(1, parseInt(q.page, 10) || 1),
  }
})

const hasFilters = computed(() => !!(query.value.q || query.value.status || query.value.priority || query.value.assignee))

const state = reactive({ loading: true, error: '', tickets: [], total: 0, totalPages: 0 })

let seq = 0
let controller = null

async function load() {
  const mine = ++seq
  controller?.abort() // a newer request supersedes the previous one
  controller = new AbortController()
  state.loading = true
  state.error = ''
  try {
    const res = await ticketsApi.list({ ...query.value, page_size: PAGE_SIZE }, { signal: controller.signal })
    if (mine !== seq) return
    state.tickets = res.data
    state.total = res.total
    state.totalPages = res.total_pages
    // A stale ?page= beyond the last page: jump back to the last one.
    if (res.data.length === 0 && res.total > 0 && query.value.page > res.total_pages) setQuery({ page: res.total_pages })
  } catch (err) {
    if (err.name === 'AbortError' || mine !== seq) return
    state.error = err.message
  } finally {
    if (mine === seq) state.loading = false
  }
}

watch(query, load, { immediate: true })

function setQuery(patch) {
  const next = { ...route.query, ...patch }
  if (!('page' in patch)) delete next.page // any filter change goes back to page 1
  for (const key of Object.keys(next)) if (next[key] === '' || next[key] == null || next[key] === 1) delete next[key]
  router.replace({ query: next })
}

// Search box: debounced so we do not hit the API on every keystroke.
const search = ref(query.value.q)
const debouncedSearch = useDebouncedRef(search, 300)
watch(debouncedSearch, (v) => {
  if (v.trim() !== query.value.q) setQuery({ q: v.trim() })
})
watch(
  () => query.value.q,
  (v) => {
    if (v !== search.value.trim()) search.value = v
  },
)

function clearFilters() {
  search.value = ''
  router.replace({ query: {} })
}

function toggleSort(column) {
  if (query.value.sort === column) setQuery({ order: query.value.order === 'asc' ? 'desc' : 'asc' })
  else setQuery({ sort: column, order: column === 'title' ? 'asc' : 'desc' })
}

function ariaSort(column) {
  if (query.value.sort !== column) return 'none'
  return query.value.order === 'asc' ? 'ascending' : 'descending'
}
</script>

<template>
  <section>
    <div class="page-head">
      <div>
        <h1>{{ auth.isStaff ? 'All tickets' : 'My tickets' }}</h1>
        <p class="muted">
          {{ auth.isStaff ? 'Everything the helpdesk is working on.' : 'Requests you have submitted to the helpdesk.' }}
        </p>
      </div>
      <RouterLink :to="{ name: 'ticket-new' }" class="btn btn-primary">New ticket</RouterLink>
    </div>

    <form class="filters card" role="search" @submit.prevent>
      <div class="filter">
        <label for="f-search">Search title</label>
        <input id="f-search" v-model="search" type="search" placeholder="e.g. printer" autocomplete="off" />
      </div>
      <div class="filter">
        <label for="f-status">Status</label>
        <select id="f-status" :value="query.status" @change="setQuery({ status: $event.target.value })">
          <option value="">All statuses</option>
          <option v-for="s in STATUSES" :key="s" :value="s">{{ STATUS_LABELS[s] }}</option>
        </select>
      </div>
      <div class="filter">
        <label for="f-priority">Priority</label>
        <select id="f-priority" :value="query.priority" @change="setQuery({ priority: $event.target.value })">
          <option value="">All priorities</option>
          <option v-for="p in PRIORITIES" :key="p" :value="p">{{ PRIORITY_LABELS[p] }}</option>
        </select>
      </div>
      <div v-if="auth.isStaff" class="filter">
        <label for="f-assignee">Assignee</label>
        <select id="f-assignee" :value="query.assignee" @change="setQuery({ assignee: $event.target.value })">
          <option value="">Anyone</option>
          <option value="me">Assigned to me</option>
          <option value="unassigned">Unassigned</option>
        </select>
      </div>
      <button v-if="hasFilters" type="button" class="btn btn-ghost" @click="clearFilters">Clear filters</button>
    </form>

    <PageState
      :loading="state.loading && !state.tickets.length"
      :error="state.error"
      :empty="!state.loading && !state.tickets.length"
      :empty-title="hasFilters ? 'No tickets match your filters' : 'No tickets yet'"
      :empty-text="hasFilters ? 'Try a different search or clear the filters.' : 'Create your first ticket and the helpdesk team will pick it up.'"
      @retry="load"
    >
      <template #empty-action>
        <button v-if="hasFilters" class="btn btn-secondary" @click="clearFilters">Clear filters</button>
        <RouterLink v-else :to="{ name: 'ticket-new' }" class="btn btn-primary">Create a ticket</RouterLink>
      </template>

      <div class="card table-card" :class="{ 'is-refreshing': state.loading }" :aria-busy="state.loading">
        <table class="table responsive">
          <thead>
            <tr>
              <th scope="col">#</th>
              <th scope="col" :aria-sort="ariaSort('title')">
                <button class="sort" @click="toggleSort('title')">Title <span aria-hidden="true">{{ query.sort === 'title' ? (query.order === 'asc' ? '▲' : '▼') : '' }}</span></button>
              </th>
              <th scope="col" :aria-sort="ariaSort('status')">
                <button class="sort" @click="toggleSort('status')">Status <span aria-hidden="true">{{ query.sort === 'status' ? (query.order === 'asc' ? '▲' : '▼') : '' }}</span></button>
              </th>
              <th scope="col" :aria-sort="ariaSort('priority')">
                <button class="sort" @click="toggleSort('priority')">Priority <span aria-hidden="true">{{ query.sort === 'priority' ? (query.order === 'asc' ? '▲' : '▼') : '' }}</span></button>
              </th>
              <th v-if="auth.isStaff" scope="col">Requester</th>
              <th scope="col">Assignee</th>
              <th scope="col" :aria-sort="ariaSort('updated_at')">
                <button class="sort" @click="toggleSort('updated_at')">Updated <span aria-hidden="true">{{ query.sort === 'updated_at' ? (query.order === 'asc' ? '▲' : '▼') : '' }}</span></button>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="t in state.tickets" :key="t.id">
              <td data-label="ID" class="muted">#{{ t.id }}</td>
              <td data-label="Title" class="cell-title">
                <RouterLink :to="{ name: 'ticket', params: { id: t.id } }">{{ t.title }}</RouterLink>
              </td>
              <td data-label="Status"><StatusBadge :status="t.status" /></td>
              <td data-label="Priority">
                <PriorityBadge :priority="t.priority" />
                <span v-if="t.is_overdue" class="badge badge-overdue" title="Urgent ticket still waiting for an agent">Overdue</span>
              </td>
              <td v-if="auth.isStaff" data-label="Requester">{{ t.requester.name }}</td>
              <td data-label="Assignee">
                <span v-if="t.assignee">{{ t.assignee.name }}</span>
                <span v-else class="muted">Unassigned</span>
              </td>
              <td data-label="Updated" :title="formatDateTime(t.updated_at)">{{ timeAgo(t.updated_at) }}</td>
            </tr>
          </tbody>
        </table>
        <PaginationBar :page="query.page" :total-pages="state.totalPages" :total="state.total" :page-size="PAGE_SIZE" @change="(p) => setQuery({ page: p })" />
      </div>
    </PageState>
  </section>
</template>
