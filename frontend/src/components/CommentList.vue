<script setup>
import { formatDateTime, ROLE_LABELS } from '../utils/format'
defineProps({ comments: { type: Array, required: true } })
</script>

<template>
  <ul v-if="comments.length" class="comments">
    <li v-for="c in comments" :key="c.id" class="comment" :class="{ internal: c.is_internal }">
      <header class="comment-head">
        <strong>{{ c.author.name }}</strong>
        <span class="role-chip">{{ ROLE_LABELS[c.author.role] }}</span>
        <span v-if="c.is_internal" class="badge badge-internal">Internal note · staff only</span>
        <time :datetime="c.created_at" class="muted">{{ formatDateTime(c.created_at) }}</time>
      </header>
      <p class="comment-body">{{ c.body }}</p>
    </li>
  </ul>
  <p v-else class="muted">No comments yet.</p>
</template>
