import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { defineComponent, h, nextTick, ref } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { PublicGposeSignup } from '@/api/client'

const sound = vi.hoisted(() => ({
  armAudioOnFirstGesture: vi.fn(),
  playGposeCallChime: vi.fn(),
  vibrate: vi.fn(),
}))
vi.mock('@/lib/sound', () => sound)

import {
  GPOSE_CALL_MESSAGE,
  GPOSE_SLOT_MESSAGE,
  GPOSE_SLOT_STARTED_MESSAGE,
  gposeAlertsDue,
  useGposeAlerts,
  type GposeAlertSource,
} from '@/composables/useGposeAlerts'
import { useUiStore } from '@/stores/ui'

const START = Date.parse('2026-08-01T18:00:00Z')

function signup(over: Partial<PublicGposeSignup> = {}): PublicGposeSignup {
  return {
    id: 1,
    boothId: 2,
    typeName: 'Solo',
    price: 0,
    minutes: 15,
    waitlisted: false,
    position: 0,
    slotStart: '2026-08-01T18:00:00Z',
    slotEnd: '2026-08-01T18:15:00Z',
    status: 'waiting',
    paid: false,
    calledAt: null,
    createdAt: '2026-08-01T12:00:00Z',
    photoUrl: '',
    photoExpiresAt: null,
    ...over,
  }
}

const booth = (signups: PublicGposeSignup[]): GposeAlertSource[] => [
  { boothTitle: 'Lantern Gpose', signups },
]

describe('gposeAlertsDue', () => {
  it('alerts a booked slot from 10 minutes before it until it ends', () => {
    const src = booth([signup()])
    expect(gposeAlertsDue(src, START - 11 * 60_000, true, new Set())).toEqual([])
    expect(gposeAlertsDue(src, START - 10 * 60_000, true, new Set())).toEqual([
      { key: 'slot:1:2026-08-01T18:00:00Z', title: 'Lantern Gpose', message: GPOSE_SLOT_MESSAGE },
    ])
    expect(gposeAlertsDue(src, START + 60_000, true, new Set())[0]?.message).toBe(
      GPOSE_SLOT_STARTED_MESSAGE,
    )
    expect(gposeAlertsDue(src, START + 15 * 60_000, true, new Set())).toEqual([])
    // Only where slot alerts are asked for: the festival dashboard.
    expect(gposeAlertsDue(src, START - 60_000, false, new Set())).toEqual([])
  })

  it('alerts a staff call until it is dismissed, and again on a new call', () => {
    const called = signup({ slotStart: null, slotEnd: null, calledAt: '2026-08-01T17:00:00Z' })
    const due = gposeAlertsDue(booth([called]), START, false, new Set())
    expect(due).toEqual([
      { key: 'call:1:2026-08-01T17:00:00Z', title: 'Lantern Gpose', message: GPOSE_CALL_MESSAGE },
    ])
    expect(gposeAlertsDue(booth([called]), START, false, new Set([due[0]!.key]))).toEqual([])
    const again = { ...called, calledAt: '2026-08-01T17:05:00Z' }
    expect(gposeAlertsDue(booth([again]), START, false, new Set([due[0]!.key]))).toHaveLength(1)
  })

  it('leaves out anyone no longer waiting, and the waitlist', () => {
    const src = booth([
      signup({ id: 1, status: 'completed', calledAt: '2026-08-01T17:00:00Z' }),
      signup({ id: 2, waitlisted: true }),
    ])
    expect(gposeAlertsDue(src, START, true, new Set())).toEqual([])
  })
})

describe('useGposeAlerts', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.setSystemTime(START - 60_000)
    localStorage.clear()
    vi.clearAllMocks()
    document.title = 'Senpan'
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  function host(signups: ReturnType<typeof ref<PublicGposeSignup[]>>) {
    return mount(
      defineComponent({
        setup() {
          useGposeAlerts(() => booth(signups.value ?? []), { slotAlerts: true })
          return () => h('div')
        },
      }),
    )
  }

  it('rings, flashes the title and stays until dismissed', async () => {
    const signups = ref<PublicGposeSignup[]>([signup()])
    const wrapper = host(signups)
    const ui = useUiStore()
    expect(ui.alerts).toHaveLength(1)
    expect(ui.alerts[0]?.message).toBe(GPOSE_SLOT_MESSAGE)
    expect(sound.playGposeCallChime).toHaveBeenCalledTimes(1)
    expect(sound.vibrate).toHaveBeenCalled()

    vi.advanceTimersByTime(1_000)
    expect(document.title).not.toBe('Senpan')
    vi.advanceTimersByTime(15_000)
    expect(sound.playGposeCallChime.mock.calls.length).toBeGreaterThan(1)

    // The visitor dismisses it: it stays dismissed, across a remount too.
    ui.dismissAlert(ui.alerts[0]!.id)
    await nextTick()
    vi.advanceTimersByTime(2_000)
    expect(ui.alerts).toHaveLength(0)
    expect(document.title).toBe('Senpan')
    const rings = sound.playGposeCallChime.mock.calls.length
    vi.advanceTimersByTime(30_000)
    expect(sound.playGposeCallChime.mock.calls.length).toBe(rings)
    wrapper.unmount()

    const again = host(signups)
    expect(useUiStore().alerts).toHaveLength(0)
    again.unmount()
  })

  it('takes an alert down when staff finish the sign-up, without counting it dismissed', async () => {
    const signups = ref<PublicGposeSignup[]>([signup()])
    const wrapper = host(signups)
    const ui = useUiStore()
    expect(ui.alerts).toHaveLength(1)
    signups.value = [signup({ status: 'completed' })]
    await nextTick()
    expect(ui.alerts).toHaveLength(0)
    signups.value = [signup()]
    await nextTick()
    expect(ui.alerts).toHaveLength(1)
    wrapper.unmount()
    expect(ui.alerts).toHaveLength(0)
  })
})
