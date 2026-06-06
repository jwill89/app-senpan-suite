/**
 * Lightweight toast notifications (module-level singleton). Replaces the
 * `notify()` method and toast state that lived on the legacy monolith.
 */
import { reactive } from 'vue'

export type ToastType = 'info' | 'success' | 'error'

export interface Toast {
  id: number
  message: string
  type: ToastType
}

const toasts = reactive<Toast[]>([])
let nextId = 1

function dismiss(id: number): void {
  const i = toasts.findIndex((t) => t.id === id)
  if (i >= 0) toasts.splice(i, 1)
}

function show(message: string, type: ToastType = 'info', ms = 3500): void {
  const id = nextId++
  toasts.push({ id, message, type })
  window.setTimeout(() => dismiss(id), ms)
}

export function useToast() {
  return { toasts, show, dismiss }
}
