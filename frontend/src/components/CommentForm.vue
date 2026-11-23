<script setup>
import { ref } from 'vue'
import FormField from './FormField.vue'
import { ApiError } from '../api/http'

const props = defineProps({
  canWriteInternal: Boolean,
  submit: { type: Function, required: true }, // (body, isInternal) => Promise
})

const body = ref('')
const internal = ref(false)
const busy = ref(false)
const error = ref('')

async function onSubmit() {
  error.value = ''
  if (!body.value.trim()) {
    error.value = 'Write a comment first.'
    return
  }
  busy.value = true
  try {
    await props.submit(body.value.trim(), props.canWriteInternal && internal.value)
    body.value = ''
    internal.value = false
  } catch (err) {
    error.value = err instanceof ApiError ? (err.fields.body ?? err.message) : 'Could not send the comment.'
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <form class="comment-form" novalidate @submit.prevent="onSubmit">
    <FormField id="comment-body" label="Add a comment" :error="error" v-slot="{ id, describedby, invalid }">
      <textarea :id="id" v-model="body" rows="4" maxlength="5000" :aria-describedby="describedby" :aria-invalid="invalid" placeholder="Write your reply…"></textarea>
    </FormField>
    <div class="form-row">
      <label v-if="canWriteInternal" class="checkbox">
        <input v-model="internal" type="checkbox" />
        Internal note (only agents and admins can see it)
      </label>
      <button class="btn btn-primary" :disabled="busy">{{ busy ? 'Sending…' : internal ? 'Add internal note' : 'Send comment' }}</button>
    </div>
  </form>
</template>
