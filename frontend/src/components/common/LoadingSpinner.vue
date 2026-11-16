<script setup lang="ts">
/**
 * Small reusable loading spinner. Styled entirely with theme tokens (see the
 * `.loading-spinner` rules in components.css) so custom themes restyle it
 * automatically.
 *
 *   <LoadingSpinner />                      inline spinner
 *   <LoadingSpinner label="Loading..." />     spinner + muted label
 *   <LoadingSpinner block label="Loading..."/> centered block (for list/data loads)
 */
defineProps<{
  /** Optional muted text shown next to the spinner. */
  label?: string
  /** Center the spinner (+label) as a standalone block with vertical padding. */
  block?: boolean
}>()
</script>

<template>
  <div
    :class="['loading-spinner', { 'loading-spinner--block': block }]"
    role="status"
    aria-live="polite"
  >
    <span class="loading-spinner__spinner" aria-hidden="true"></span>
    <span v-if="label" class="loading-spinner__label">{{ label }}</span>
  </div>
</template>
