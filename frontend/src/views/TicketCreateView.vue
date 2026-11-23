<script setup>
import { onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ticketsApi } from '../api'
import FormField from '../components/FormField.vue'
import { useForm } from '../composables/useForm'
import { useMetaStore } from '../stores/meta'
import { useToastStore } from '../stores/toast'
import { PRIORITIES, PRIORITY_LABELS, categoryLabel } from '../utils/format'

const router = useRouter()
const toast = useToastStore()
const meta = useMetaStore()
const { errors, formError, busy, run, reset } = useForm()

const form = reactive({ title: '', category: 'general', priority: 'medium', description: '' })
const metaError = ref('')

onMounted(async () => {
  try {
    await meta.ensureLoaded()
  } catch (err) {
    metaError.value = err.message
  }
})

function validate() {
  reset()
  const title = form.title.trim()
  if (title.length < 3) errors.title = 'Give the ticket a short title (at least 3 characters).'
  else if (title.length > 200) errors.title = 'Keep the title under 200 characters.'
  if (!form.description.trim()) errors.description = 'Describe the problem so we can help.'
  else if (form.description.length > 5000) errors.description = 'Keep the description under 5000 characters.'
  return Object.keys(errors).length === 0
}

async function onSubmit() {
  if (!validate()) return
  let created
  const ok = await run(async () => {
    created = await ticketsApi.create({
      title: form.title.trim(),
      description: form.description.trim(),
      category: form.category,
      priority: form.priority,
    })
  })
  if (ok) {
    toast.success(`Ticket #${created.id} created.`)
    router.push({ name: 'ticket', params: { id: created.id } })
  } else {
    toast.error('The ticket was not created. Please check the form.')
  }
}
</script>

<template>
  <section class="narrow">
    <div class="page-head">
      <div>
        <h1>New ticket</h1>
        <p class="muted">Tell us what is wrong. The more detail you give, the faster we can fix it.</p>
      </div>
    </div>

    <p v-if="metaError" class="alert alert-warning" role="alert">Could not load categories ({{ metaError }}). Defaults are used.</p>
    <p v-if="formError" class="alert alert-error" role="alert">{{ formError }}</p>

    <form class="card form-card" novalidate @submit.prevent="onSubmit">
      <FormField id="title" label="Title" :error="errors.title" v-slot="{ id, describedby, invalid }">
        <input :id="id" v-model="form.title" maxlength="200" placeholder="Short summary of the problem" :aria-describedby="describedby" :aria-invalid="invalid" />
      </FormField>

      <div class="form-grid">
        <FormField id="category" label="Category" :error="errors.category" v-slot="{ id, describedby, invalid }">
          <select :id="id" v-model="form.category" :aria-describedby="describedby" :aria-invalid="invalid">
            <option v-for="c in meta.categories.length ? meta.categories : ['general']" :key="c" :value="c">{{ categoryLabel(c) }}</option>
          </select>
        </FormField>
        <FormField id="priority" label="Priority" hint="Urgent is for outages that block your work." :error="errors.priority" v-slot="{ id, describedby, invalid }">
          <select :id="id" v-model="form.priority" :aria-describedby="describedby" :aria-invalid="invalid">
            <option v-for="p in PRIORITIES" :key="p" :value="p">{{ PRIORITY_LABELS[p] }}</option>
          </select>
        </FormField>
      </div>

      <FormField id="description" label="Description" :error="errors.description" v-slot="{ id, describedby, invalid }">
        <textarea :id="id" v-model="form.description" rows="7" maxlength="5000" placeholder="What happened? What did you expect? Any error messages?" :aria-describedby="describedby" :aria-invalid="invalid"></textarea>
        <span class="counter">{{ form.description.length }} / 5000</span>
      </FormField>

      <div class="form-actions">
        <RouterLink :to="{ name: 'tickets' }" class="btn btn-secondary">Cancel</RouterLink>
        <button class="btn btn-primary" :disabled="busy">{{ busy ? 'Creating…' : 'Create ticket' }}</button>
      </div>
    </form>
  </section>
</template>
