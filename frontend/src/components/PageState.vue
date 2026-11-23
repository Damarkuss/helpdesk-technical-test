<script setup>
// One place for the three states every data view needs: loading, error, empty.
defineProps({
  loading: Boolean,
  error: { type: String, default: '' },
  empty: Boolean,
  emptyTitle: { type: String, default: 'Nothing here yet' },
  emptyText: { type: String, default: '' },
})
defineEmits(['retry'])
</script>

<template>
  <div v-if="loading" class="state" role="status" aria-live="polite">
    <span class="spinner" aria-hidden="true"></span>
    <span>Loading…</span>
  </div>

  <div v-else-if="error" class="state state-error" role="alert">
    <strong>Something went wrong</strong>
    <p>{{ error }}</p>
    <button class="btn btn-secondary" @click="$emit('retry')">Try again</button>
  </div>

  <div v-else-if="empty" class="state">
    <strong>{{ emptyTitle }}</strong>
    <p v-if="emptyText">{{ emptyText }}</p>
    <slot name="empty-action" />
  </div>

  <slot v-else />
</template>
