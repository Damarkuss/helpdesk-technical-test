import { onScopeDispose, ref, watch } from 'vue'

/** Returns a ref whose value follows `source` after `delay` ms of quiet. */
export function useDebouncedRef(source, delay = 300) {
  const debounced = ref(source.value)
  let timer
  watch(source, (v) => {
    clearTimeout(timer)
    timer = setTimeout(() => (debounced.value = v), delay)
  })
  onScopeDispose(() => clearTimeout(timer))
  return debounced
}
