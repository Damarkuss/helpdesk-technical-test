<script setup>
import { PRIORITY_LABELS } from '../utils/format'
defineProps({ priority: { type: String, required: true } })
</script>

<template>
  <span class="badge" :class="`priority-${priority}`">{{ PRIORITY_LABELS[priority] ?? priority }}</span>
</template>
