/**
 * Alerts a visitor that it is their turn at a gpose booth, until they dismiss it:
 * a sticky alert on the page, the tab's title flashing (what shows in the taskbar
 * while the page is in the background), a chime every 15 seconds and, on a phone, a
 * vibration with each.
 *
 * Two things set one off:
 *  - staff calling the person up (`calledAt`, from the admin page or the plugin),
 *    which a basic booth's gposer does when they're next; calling again alerts again;
 *  - at a timeslot booth, their slot coming up: from 10 minutes before it starts
 *    until it ends. Only where `slotAlerts` is set - the festival dashboard.
 *
 * Nothing here asks the server: the page keeps the sign-ups fresh (it reads them
 * every half minute while one waits), and this works from what it is handed. A
 * dismissal is kept on the device, by sign-up and by call or slot, so a reload doesn't
 * bring a dismissed alert back while a new call still does.
 */
import { onBeforeUnmount, onMounted, watch } from 'vue'
import type { PublicGposeSignup } from '@/api/client'
import { armAudioOnFirstGesture, playGposeCallChime, vibrate } from '@/lib/sound'
import { useUiStore } from '@/stores/ui'

/** How long before a booked slot its alert starts. */
export const GPOSE_ALERT_LEAD_MS = 10 * 60_000

/** What the alerts say (owner's wording). */
export const GPOSE_CALL_MESSAGE =
  "You're next in line for the gpose booth! Please be ready in 10-15 minutes!"
export const GPOSE_SLOT_MESSAGE = 'Your gpose booth slot starts in 10 minutes! Please be ready!'
/** For an alert first shown once the slot has begun: a waitlist placement, or a late visit. */
export const GPOSE_SLOT_STARTED_MESSAGE =
  'Your gpose booth slot has started! Please head to the booth now.'

const SOUND_EVERY_MS = 15_000
const FLASH_EVERY_MS = 1_000
const DISMISSED_KEY = 'senpan_gpose_dismissed'
const MAX_DISMISSED = 200
const FLASH_TITLE = "\u{1F4F8} It's your turn!"

/** One booth's sign-ups, as the page knows them. */
export interface GposeAlertSource {
  boothTitle: string
  signups: readonly PublicGposeSignup[]
}

/** An alert that is due now. */
export interface GposeAlertDue {
  key: string
  title: string
  message: string
}

/** The alerts due at `now`, leaving out those dismissed. Pure, for the tests. */
export function gposeAlertsDue(
  sources: readonly GposeAlertSource[],
  now: number,
  slotAlerts: boolean,
  dismissed: ReadonlySet<string>,
): GposeAlertDue[] {
  const due: GposeAlertDue[] = []
  for (const src of sources) {
    for (const s of src.signups) {
      if (s.status !== 'waiting') {
        continue
      }
      if (s.calledAt) {
        const key = `call:${s.id}:${s.calledAt}`
        if (!dismissed.has(key)) {
          due.push({ key, title: src.boothTitle, message: GPOSE_CALL_MESSAGE })
        }
      }
      if (slotAlerts && s.slotStart && s.slotEnd && !s.waitlisted) {
        const start = Date.parse(s.slotStart)
        const end = Date.parse(s.slotEnd)
        const key = `slot:${s.id}:${s.slotStart}`
        if (now >= start - GPOSE_ALERT_LEAD_MS && now < end && !dismissed.has(key)) {
          due.push({
            key,
            title: src.boothTitle,
            message: now >= start ? GPOSE_SLOT_STARTED_MESSAGE : GPOSE_SLOT_MESSAGE,
          })
        }
      }
    }
  }
  return due
}

function readDismissed(): Set<string> {
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(DISMISSED_KEY) ?? '[]')
    return new Set(Array.isArray(raw) ? raw.filter((k): k is string => typeof k === 'string') : [])
  } catch {
    return new Set()
  }
}

function writeDismissed(keys: Set<string>): void {
  try {
    localStorage.setItem(DISMISSED_KEY, JSON.stringify([...keys].slice(-MAX_DISMISSED)))
  } catch {
    // Blocked or full: the alert stays dismissed for this visit only.
  }
}

export function useGposeAlerts(
  sources: () => readonly GposeAlertSource[],
  options: { slotAlerts: boolean },
): void {
  const ui = useUiStore()
  const dismissed = readDismissed()
  /** The alerts on screen: key -> the ui store's alert id. */
  const shown = new Map<string, number>()
  let tick: ReturnType<typeof setInterval> | null = null
  let sound: ReturnType<typeof setInterval> | null = null
  let flash: ReturnType<typeof setInterval> | null = null
  let baseTitle = ''

  function ring(): void {
    playGposeCallChime()
    vibrate([300, 150, 300])
  }

  function startEffects(): void {
    sound ??= setInterval(ring, SOUND_EVERY_MS)
    if (flash === null) {
      baseTitle = document.title
      flash = setInterval(() => {
        document.title = document.title === FLASH_TITLE ? baseTitle : FLASH_TITLE
      }, FLASH_EVERY_MS)
    }
  }

  function stopEffects(): void {
    if (sound !== null) {
      clearInterval(sound)
      sound = null
    }
    if (flash !== null) {
      clearInterval(flash)
      flash = null
      document.title = baseTitle
    }
  }

  function update(): void {
    const due = gposeAlertsDue(sources(), Date.now(), options.slotAlerts, dismissed)
    const keys = new Set(due.map((d) => d.key))
    // One no longer due (the slot ended, staff marked it done): taken down unmarked,
    // forgotten from `shown` first so the dismissal watch doesn't count it.
    for (const [key, id] of [...shown]) {
      if (!keys.has(key)) {
        shown.delete(key)
        ui.dismissAlert(id)
      }
    }
    let fresh = false
    for (const d of due) {
      if (!shown.has(d.key)) {
        shown.set(d.key, ui.alert({ title: d.title, message: d.message, icon: 'camera' }))
        fresh = true
      }
    }
    if (fresh) {
      ring()
    }
    if (shown.size > 0) {
      startEffects()
    } else {
      stopEffects()
    }
  }

  // An alert gone from the stack without this taking it down: the visitor dismissed it.
  watch(
    () => ui.alerts.map((a) => a.id),
    (ids) => {
      let changed = false
      for (const [key, id] of [...shown]) {
        if (!ids.includes(id)) {
          shown.delete(key)
          dismissed.add(key)
          changed = true
        }
      }
      if (changed) {
        writeDismissed(dismissed)
        if (shown.size === 0) {
          stopEffects()
        }
      }
    },
  )
  watch(sources, update, { deep: true })

  onMounted(() => {
    armAudioOnFirstGesture()
    update()
    // The clock moves a slot into its alert, and out of it.
    tick = setInterval(update, 1_000)
  })
  onBeforeUnmount(() => {
    if (tick !== null) {
      clearInterval(tick)
    }
    for (const [key, id] of [...shown]) {
      shown.delete(key)
      ui.dismissAlert(id)
    }
    stopEffects()
  })
}
