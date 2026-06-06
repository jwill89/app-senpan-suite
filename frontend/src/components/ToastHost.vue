<script setup lang="ts">
/** Renders the active toasts. Mounted once at the app root. */
import { useToast } from '@/composables/useToast'

const { toasts, dismiss } = useToast()
</script>

<template>
  <div class="toast-host" aria-live="polite">
    <button
      v-for="t in toasts"
      :key="t.id"
      type="button"
      class="toast"
      :class="t.type"
      @click="dismiss(t.id)"
    >
      {{ t.message }}
    </button>
  </div>
</template>

<style scoped>
.toast-host {
  position: fixed;
  right: 16px;
  bottom: 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  z-index: 1000;
}
.toast {
  border: 1px solid #3a3a4a;
  border-left-width: 4px;
  border-radius: 8px;
  background: #20202e;
  color: #f4f4f8;
  padding: 10px 14px;
  font: inherit;
  text-align: left;
  cursor: pointer;
  max-width: 320px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.35);
}
.toast.success {
  border-left-color: #2cb67d;
}
.toast.error {
  border-left-color: #ff6b6b;
}
.toast.info {
  border-left-color: var(--accent, #e53170);
}
</style>
