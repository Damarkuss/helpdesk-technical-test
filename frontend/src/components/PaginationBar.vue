<script setup>
import { computed } from 'vue'

const props = defineProps({
  page: { type: Number, required: true },
  totalPages: { type: Number, required: true },
  total: { type: Number, required: true },
  pageSize: { type: Number, required: true },
})
const emit = defineEmits(['change'])

const from = computed(() => (props.total === 0 ? 0 : (props.page - 1) * props.pageSize + 1))
const to = computed(() => Math.min(props.page * props.pageSize, props.total))
</script>

<template>
  <nav class="pagination" aria-label="Pagination">
    <span class="pagination-info">Showing {{ from }}–{{ to }} of {{ total }}</span>
    <span class="pagination-buttons">
      <button class="btn btn-secondary btn-sm" :disabled="page <= 1" @click="emit('change', page - 1)">Previous</button>
      <span class="pagination-page">Page {{ page }} of {{ Math.max(totalPages, 1) }}</span>
      <button class="btn btn-secondary btn-sm" :disabled="page >= totalPages" @click="emit('change', page + 1)">Next</button>
    </span>
  </nav>
</template>
