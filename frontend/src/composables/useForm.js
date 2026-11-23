import { reactive, ref } from 'vue'
import { ApiError } from '../api/http'

/**
 * Small helper for forms: tracks per-field errors, a form-level error and a busy flag,
 * and maps ApiError responses (including server-side field errors) onto them.
 */
export function useForm() {
  const errors = reactive({})
  const formError = ref('')
  const busy = ref(false)

  function reset() {
    for (const key of Object.keys(errors)) delete errors[key]
    formError.value = ''
  }

  function setServerError(err) {
    if (err instanceof ApiError) {
      Object.assign(errors, err.fields)
      if (!Object.keys(err.fields).length || err.fields._) formError.value = err.fields?._ ?? err.message
    } else {
      formError.value = 'Unexpected error. Please try again.'
    }
  }

  /** Runs fn with busy handling; returns true on success. Errors are mapped onto the form. */
  async function run(fn) {
    reset()
    busy.value = true
    try {
      await fn()
      return true
    } catch (err) {
      setServerError(err)
      return false
    } finally {
      busy.value = false
    }
  }

  return { errors, formError, busy, reset, run }
}
