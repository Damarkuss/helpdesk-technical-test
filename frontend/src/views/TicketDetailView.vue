<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ticketsApi, usersApi } from '../api'
import CommentForm from '../components/CommentForm.vue'
import CommentList from '../components/CommentList.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import PageState from '../components/PageState.vue'
import PriorityBadge from '../components/PriorityBadge.vue'
import StatusBadge from '../components/StatusBadge.vue'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import { PRIORITIES, PRIORITY_LABELS, STATUS_LABELS, categoryLabel, formatDateTime, transitionLabel, transitionTone } from '../utils/format'

const props = defineProps({ id: { type: String, required: true } })

const router = useRouter()
const auth = useAuthStore()
const toast = useToastStore()

const loading = ref(true)
const error = ref('')
const ticket = ref(null)
const busy = ref(false)
const staff = ref([]) // people a ticket can be assigned to (admin only)

const confirm = ref(null) // { title, message, label, danger, action }

const isClosed = computed(() => ticket.value?.status === 'closed')
const isMine = computed(() => ticket.value?.assignee?.id === auth.user?.id)

async function load() {
  loading.value = true
  error.value = ''
  try {
    ticket.value = await ticketsApi.get(props.id)
  } catch (err) {
    ticket.value = null
    error.value = err.message
  } finally {
    loading.value = false
  }
}

watch(() => props.id, load, { immediate: true })

onMounted(async () => {
  if (!auth.isAdmin) return
  try {
    staff.value = (await usersApi.assignable()).data
  } catch {
    /* the assignee menu simply stays short; the rest of the page works */
  }
})

/** Applies a partial update and merges the result, keeping the loaded comments. */
async function patch(changes, successMessage) {
  busy.value = true
  try {
    const updated = await ticketsApi.update(props.id, changes)
    ticket.value = { ...updated, comments: ticket.value.comments }
    toast.success(successMessage)
    return true
  } catch (err) {
    toast.error(err.message)
    // Someone else changed the ticket in the meantime: show the fresh state.
    if (err.status === 409) await load()
    return false
  } finally {
    busy.value = false
  }
}

function changeStatus(to) {
  const from = ticket.value.status
  if (from === 'open' && to === 'closed') {
    confirm.value = {
      title: 'Cancel this ticket?',
      message: 'The ticket will be closed and can no longer be changed.',
      label: 'Cancel ticket',
      danger: true,
      action: () => patch({ status: to }, 'Ticket cancelled.'),
    }
    return
  }
  patch({ status: to }, `Ticket is now ${STATUS_LABELS[to].toLowerCase()}.`)
}

async function changePriority(event) {
  const el = event.target
  const ok = await patch({ priority: el.value }, 'Priority updated.')
  if (!ok) el.value = ticket.value.priority // undo the visible change when the server refused it
}

async function changeAssignee(event) {
  const el = event.target
  const id = el.value ? Number(el.value) : null
  const ok = await patch({ assignee_id: id }, id ? 'Assignee updated.' : 'Ticket unassigned.')
  if (!ok) el.value = ticket.value.assignee?.id ?? ''
}

function takeTicket() {
  return patch({ assignee_id: auth.user.id }, 'Ticket assigned to you.')
}
function releaseTicket() {
  return patch({ assignee_id: null }, 'Ticket unassigned.')
}

function askDelete() {
  confirm.value = {
    title: `Delete ticket #${ticket.value.id}?`,
    message: 'This permanently removes the ticket and all of its comments.',
    label: 'Delete ticket',
    danger: true,
    action: async () => {
      busy.value = true
      try {
        await ticketsApi.remove(props.id)
        toast.success('Ticket deleted.')
        router.push({ name: 'tickets' })
      } catch (err) {
        toast.error(err.message)
      } finally {
        busy.value = false
      }
    },
  }
}

async function runConfirmed() {
  const { action } = confirm.value
  confirm.value = null
  await action()
}

async function addComment(body, internal) {
  const created = await ticketsApi.addComment(props.id, body, internal)
  ticket.value = { ...ticket.value, comments: [...ticket.value.comments, created] }
  toast.success(internal ? 'Internal note added.' : 'Comment added.')
}
</script>

<template>
  <section>
    <RouterLink :to="{ name: 'tickets' }" class="back-link">← All tickets</RouterLink>

    <PageState :loading="loading" :error="error" @retry="load">
      <template v-if="ticket">
        <div class="page-head">
          <div>
            <h1>{{ ticket.title }} <span class="muted ticket-id">#{{ ticket.id }}</span></h1>
            <p class="badge-row">
              <StatusBadge :status="ticket.status" />
              <PriorityBadge :priority="ticket.priority" />
              <span v-if="ticket.is_overdue" class="badge badge-overdue">Overdue</span>
            </p>
          </div>
        </div>

        <div class="detail-layout">
          <div class="detail-main">
            <article class="card">
              <h2>Description</h2>
              <p class="prewrap">{{ ticket.description }}</p>
            </article>

            <article class="card">
              <h2>Conversation</h2>
              <CommentList :comments="ticket.comments" />
              <CommentForm v-if="!isClosed" :can-write-internal="auth.isStaff" :submit="addComment" />
              <p v-else class="alert alert-info">This ticket is closed, so it no longer accepts comments.</p>
            </article>
          </div>

          <aside class="detail-side">
            <section v-if="ticket.allowed_transitions.length" class="card">
              <h2>Actions</h2>
              <div class="action-stack">
                <button
                  v-for="to in ticket.allowed_transitions"
                  :key="to"
                  class="btn btn-block"
                  :class="`btn-${transitionTone(ticket.status, to)}`"
                  :disabled="busy"
                  @click="changeStatus(to)"
                >
                  {{ transitionLabel(ticket.status, to) }}
                </button>
              </div>
            </section>

            <section class="card">
              <h2>Details</h2>
              <dl class="details">
                <dt>Requester</dt>
                <dd>{{ ticket.requester.name }}</dd>

                <dt>Assignee</dt>
                <dd>
                  <template v-if="auth.isAdmin && !isClosed">
                    <label class="sr-only" for="assignee">Assignee</label>
                    <select id="assignee" :value="ticket.assignee?.id ?? ''" :disabled="busy" @change="changeAssignee">
                      <option value="">Unassigned</option>
                      <option v-for="s in staff" :key="s.id" :value="s.id">{{ s.name }}</option>
                    </select>
                  </template>
                  <template v-else>
                    <span v-if="ticket.assignee">{{ ticket.assignee.name }}</span>
                    <span v-else class="muted">Unassigned</span>
                    <template v-if="auth.isStaff && !isClosed">
                      <button v-if="!isMine" class="btn btn-secondary btn-sm inline-btn" :disabled="busy" @click="takeTicket">Assign to me</button>
                      <button v-else class="btn btn-ghost btn-sm inline-btn" :disabled="busy" @click="releaseTicket">Unassign me</button>
                    </template>
                  </template>
                </dd>

                <dt>Priority</dt>
                <dd>
                  <template v-if="auth.isStaff && !isClosed">
                    <label class="sr-only" for="priority">Priority</label>
                    <select id="priority" :value="ticket.priority" :disabled="busy" @change="changePriority">
                      <option v-for="p in PRIORITIES" :key="p" :value="p">{{ PRIORITY_LABELS[p] }}</option>
                    </select>
                  </template>
                  <PriorityBadge v-else :priority="ticket.priority" />
                </dd>

                <dt>Category</dt>
                <dd>{{ categoryLabel(ticket.category) }}</dd>
                <dt>Created</dt>
                <dd>{{ formatDateTime(ticket.created_at) }}</dd>
                <dt>Last update</dt>
                <dd>{{ formatDateTime(ticket.updated_at) }}</dd>
              </dl>
            </section>

            <section v-if="auth.isAdmin" class="card">
              <h2>Admin</h2>
              <button class="btn btn-danger btn-block" :disabled="busy" @click="askDelete">Delete ticket</button>
            </section>
          </aside>
        </div>
      </template>
    </PageState>

    <ConfirmDialog
      :open="!!confirm"
      :title="confirm?.title ?? ''"
      :message="confirm?.message ?? ''"
      :confirm-label="confirm?.label ?? 'Confirm'"
      :danger="confirm?.danger"
      :busy="busy"
      @confirm="runConfirmed"
      @cancel="confirm = null"
    />
  </section>
</template>
