import { defineStore } from 'pinia'
import { ref } from 'vue'

let nextId = 1

export const useToastStore = defineStore('toast', () => {
  const items = ref([])

  function push(type, message, timeout = 4500) {
    const id = nextId++
    items.value.push({ id, type, message })
    if (timeout > 0) setTimeout(() => dismiss(id), timeout)
    return id
  }
  function dismiss(id) {
    items.value = items.value.filter((t) => t.id !== id)
  }

  return {
    items,
    dismiss,
    success: (msg) => push('success', msg),
    error: (msg) => push('error', msg, 7000),
    info: (msg) => push('info', msg),
  }
})
