<script setup>
import { reactive } from 'vue'
import { useRouter } from 'vue-router'
import FormField from '../components/FormField.vue'
import { useAuthStore } from '../stores/auth'
import { useToastStore } from '../stores/toast'
import { useForm } from '../composables/useForm'
import { isEmail } from '../utils/validation'

const auth = useAuthStore()
const toast = useToastStore()
const router = useRouter()
const { errors, formError, busy, run, reset } = useForm()

const form = reactive({ name: '', email: '', password: '', confirm: '' })

function validate() {
  reset()
  if (form.name.trim().length < 2) errors.name = 'Enter your full name (at least 2 characters).'
  if (!isEmail(form.email)) errors.email = 'Enter a valid email address.'
  if (form.password.length < 8) errors.password = 'Use at least 8 characters.'
  if (form.confirm !== form.password) errors.confirm = 'Passwords do not match.'
  return Object.keys(errors).length === 0
}

async function onSubmit() {
  if (!validate()) return
  const ok = await run(() => auth.register(form.name.trim(), form.email, form.password))
  if (ok) {
    toast.success(`Welcome, ${auth.user.name}! Your account is ready.`)
    router.push({ name: 'tickets' })
  }
}
</script>

<template>
  <section class="auth-card card">
    <h1>Create your account</h1>
    <p class="muted">New accounts can create tickets and follow their own requests.</p>

    <p v-if="formError" class="alert alert-error" role="alert">{{ formError }}</p>

    <form novalidate @submit.prevent="onSubmit">
      <FormField id="name" label="Full name" :error="errors.name" v-slot="{ id, describedby, invalid }">
        <input :id="id" v-model="form.name" autocomplete="name" autofocus :aria-describedby="describedby" :aria-invalid="invalid" />
      </FormField>
      <FormField id="email" label="Email" :error="errors.email" v-slot="{ id, describedby, invalid }">
        <input :id="id" v-model="form.email" type="email" autocomplete="email" :aria-describedby="describedby" :aria-invalid="invalid" />
      </FormField>
      <FormField id="password" label="Password" hint="At least 8 characters." :error="errors.password" v-slot="{ id, describedby, invalid }">
        <input :id="id" v-model="form.password" type="password" autocomplete="new-password" :aria-describedby="describedby" :aria-invalid="invalid" />
      </FormField>
      <FormField id="confirm" label="Confirm password" :error="errors.confirm" v-slot="{ id, describedby, invalid }">
        <input :id="id" v-model="form.confirm" type="password" autocomplete="new-password" :aria-describedby="describedby" :aria-invalid="invalid" />
      </FormField>
      <button class="btn btn-primary btn-block" :disabled="busy">{{ busy ? 'Creating account…' : 'Create account' }}</button>
    </form>

    <p class="auth-switch">Already registered? <RouterLink :to="{ name: 'login' }">Sign in</RouterLink></p>
  </section>
</template>
