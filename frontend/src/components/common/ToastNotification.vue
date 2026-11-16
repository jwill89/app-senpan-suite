<script setup lang="ts">
/**
 * Toast notification - fixed-position success/error/info popup, plus the sticky
 * alerts stacked above it. Both live in one top-right column so an alert and a
 * toast never sit on top of each other. Driven by the UI store.
 */
import { useRouter } from 'vue-router'
import { useUiStore, type UiAlert } from '@/stores/ui'

const ui = useUiStore()
const router = useRouter()

/** Follows an alert's action and dismisses it - acting on it is the point. */
function follow(a: UiAlert): void {
  ui.dismissAlert(a.id)
  if (a.action) {
    void router.push(a.action.to)
  }
}
</script>

<template>
  <div class="toast-notification">
    <div
      v-for="a in ui.alerts"
      :key="a.id"
      class="toast-notification__toast toast-notification__toast--alert"
      role="alert"
    >
      <div class="toast-notification__head">
        <font-awesome-icon :icon="['fas', 'lock-open']" class="toast-notification__icon" />
        <strong class="toast-notification__title">{{ a.title }}</strong>
        <button
          type="button"
          class="toast-notification__close"
          aria-label="Dismiss"
          title="Dismiss"
          @click="ui.dismissAlert(a.id)"
        >
          <font-awesome-icon :icon="['fas', 'circle-xmark']" />
        </button>
      </div>
      <p class="toast-notification__body">{{ a.message }}</p>
      <div v-if="a.action" class="toast-notification__actions">
        <button type="button" class="btn-view btn-sm" @click="follow(a)">
          {{ a.action.label }}
        </button>
      </div>
    </div>
    <!-- eslint-disable-next-line vuejs-accessibility/click-events-have-key-events, vuejs-accessibility/no-static-element-interactions -- a status live region that clears itself on a timer; click-to-dismiss is a pointer shortcut, and making it focusable would pull it out of the status role -->
    <div
      v-if="ui.toast.show"
      :class="['toast-notification__toast', 'toast-notification__toast--' + ui.toast.type]"
      role="status"
      aria-live="polite"
      title="Click to dismiss"
      @click="ui.dismissToast()"
    >
      {{ ui.toast.message }}
    </div>
  </div>
</template>
