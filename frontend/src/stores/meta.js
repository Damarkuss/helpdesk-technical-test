import { defineStore } from 'pinia'
import { ref } from 'vue'
import { metaApi } from '../api'

// Allowed enum values come from the server so the UI never drifts from the API.
export const useMetaStore = defineStore('meta', () => {
  const categories = ref([])
  let pending = null

  function ensureLoaded() {
    if (categories.value.length) return Promise.resolve()
    pending ??= metaApi
      .get()
      .then((m) => {
        categories.value = m.categories
      })
      .finally(() => {
        pending = null
      })
    return pending
  }

  return { categories, ensureLoaded }
})
