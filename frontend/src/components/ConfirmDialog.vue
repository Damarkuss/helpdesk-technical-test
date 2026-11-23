<script setup>
import { ref, watch } from 'vue'

const props = defineProps({
  open: Boolean,
  title: { type: String, required: true },
  message: { type: String, default: '' },
  confirmLabel: { type: String, default: 'Confirm' },
  danger: Boolean,
  busy: Boolean,
})
const emit = defineEmits(['confirm', 'cancel'])

const dialog = ref(null)

// Uses the native <dialog> element: focus trapping, Escape to close and a backdrop for free.
watch(
  () => props.open,
  (open) => {
    if (open && !dialog.value?.open) dialog.value?.showModal()
    if (!open && dialog.value?.open) dialog.value.close()
  },
)
</script>

<template>
  <dialog ref="dialog" class="dialog" @cancel.prevent="emit('cancel')">
    <h2>{{ title }}</h2>
    <p v-if="message">{{ message }}</p>
    <div class="dialog-actions">
      <button class="btn btn-secondary" :disabled="busy" @click="emit('cancel')">Cancel</button>
      <button class="btn" :class="danger ? 'btn-danger' : 'btn-primary'" :disabled="busy" @click="emit('confirm')">
        {{ confirmLabel }}
      </button>
    </div>
  </dialog>
</template>
