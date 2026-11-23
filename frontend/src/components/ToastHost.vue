<script setup>
import { useToastStore } from '../stores/toast'
const toast = useToastStore()
</script>

<template>
  <div class="toasts" aria-live="polite">
    <div v-for="t in toast.items" :key="t.id" class="toast" :class="`toast-${t.type}`" role="status">
      <span>{{ t.message }}</span>
      <button class="toast-close" aria-label="Dismiss" @click="toast.dismiss(t.id)">×</button>
    </div>
  </div>
</template>
