/**
 * Admin authentication state, shared app-wide (module-level singleton).
 *
 * Auth is cookie-based (the SCS session set by the Go backend), so there is no
 * token to store — `isAdmin` simply mirrors the server's view, confirmed via
 * GET /api/auth. The router guard calls `ensure()` before admin routes.
 */
import { ref } from 'vue'
import { apiGet, apiPost } from '@/api/client'
import type { AuthCheckResponse } from '@/types/models'

const isAdmin = ref(false)
const checked = ref(false)

/** Ask the server whether the current session is an authenticated admin. */
async function check(): Promise<boolean> {
  try {
    const r = await apiGet<AuthCheckResponse>('auth')
    isAdmin.value = r.authenticated
  } catch {
    isAdmin.value = false
  }
  checked.value = true
  return isAdmin.value
}

/** Return the current admin status, querying the server once if not yet known. */
async function ensure(): Promise<boolean> {
  if (checked.value) return isAdmin.value
  return check()
}

async function login(password: string): Promise<void> {
  await apiPost<{ success?: boolean }>('auth', { action: 'login', password })
  isAdmin.value = true
  checked.value = true
}

async function logout(): Promise<void> {
  try {
    await apiPost<{ success?: boolean }>('auth', { action: 'logout' })
  } finally {
    isAdmin.value = false
    checked.value = true
  }
}

export function useAuth() {
  return { isAdmin, checked, check, ensure, login, logout }
}
