<script setup>
import { reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import FormField from '../components/FormField.vue'
import { useAuthStore } from '../stores/auth'
import { useForm } from '../composables/useForm'
import { isEmail, safeRedirect } from '../utils/validation'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const { errors, formError, busy, run, reset } = useForm()

const form = reactive({ email: '', password: '' })

function validate() {
  reset()
  if (!form.email.trim()) errors.email = 'Enter your email address.'
  else if (!isEmail(form.email)) errors.email = 'Enter a valid email address.'
  if (!form.password) errors.password = 'Enter your password.'
  return Object.keys(errors).length === 0
}

async function onSubmit() {
  if (!validate()) return
  const ok = await run(() => auth.login(form.email, form.password))
  if (ok) router.push(safeRedirect(route.query.redirect) ?? { name: 'tickets' })
}
</script>

<template>
  <section class="auth-card card">
    <h1>Sign in</h1>
    <p class="muted">Use your company account to see and manage support tickets.</p>

    <p v-if="route.query.reason === 'offline'" class="alert alert-warning" role="alert">
      The server could not be reached. Check your connection and try again.
    </p>
    <p v-if="formError" class="alert alert-error" role="alert">{{ formError }}</p>

    <form novalidate @submit.prevent="onSubmit">
      <FormField id="email" label="Email" :error="errors.email" v-slot="{ id, describedby, invalid }">
        <input :id="id" v-model="form.email" type="email" autocomplete="username" autofocus :aria-describedby="describedby" :aria-invalid="invalid" />
      </FormField>
      <FormField id="password" label="Password" :error="errors.password" v-slot="{ id, describedby, invalid }">
        <input :id="id" v-model="form.password" type="password" autocomplete="current-password" :aria-describedby="describedby" :aria-invalid="invalid" />
      </FormField>
      <button class="btn btn-primary btn-block" :disabled="busy">{{ busy ? 'Signing in…' : 'Sign in' }}</button>
    </form>

    <p class="auth-switch">No account yet? <RouterLink :to="{ name: 'register' }">Create one</RouterLink></p>
  </section>
</template>
