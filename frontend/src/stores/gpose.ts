/**
 * Gpose booths store: the Gpose Booth page's booths, their sign-ups and the site's
 * gpose types, and the public side - one booth as a visitor sees it, signing up, and
 * a visitor's own sign-ups.
 *
 * A basic booth takes sign-ups in order up to its limit, then a waitlist; a timeslot
 * booth books start times inside its open times. The server decides where every
 * sign-up lands and which times are free: the page shows what it answers.
 */
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import {
  api,
  unwrap,
  type FestivalMap,
  type FestivalStallOccupant,
  type GposeBooth,
  type GposeBoothInput,
  type GposeSignup,
  type GposeSignupChange,
  type GposeType,
  type PublicGposeBooth,
  type PublicGposeSignup,
} from '@/api/client'
import { datetimeLocalToUtc, utcToDatetimeLocal } from '@/lib/datetime'
import { saveGposeSignup } from '@/lib/signups'
import { useUiStore } from './ui'

export type GposeMode = 'basic' | 'timeslot'

/** One type a booth offers, as the form edits it. */
export interface GposeBoothTypeForm {
  typeId: number
  price: number
  minutes: number
}

/** One open time, as `datetime-local` values in this admin's wall clock. */
export interface GposeWindowForm {
  startAt: string
  endAt: string
}

/** The admin form model for a booth (id 0 = new). */
export interface GposeBoothForm {
  id: number
  title: string
  description: string
  mode: GposeMode
  maxSignups: number
  maxWaitlist: number
  perPerson: number
  slotStep: number
  types: GposeBoothTypeForm[]
  windows: GposeWindowForm[]
  festivalMapId: number | null
  occupantId: number | null
}

/** What a visitor's sign-up sends. */
export interface GposeSignUpRequest {
  characterName: string
  world: string
  typeId: number
  slotStart?: string
  waitlist?: boolean
  turnstileToken?: string
}

/** A blank booth: basic, one place per person. */
export function emptyGposeBoothForm(): GposeBoothForm {
  return {
    id: 0,
    title: '',
    description: '',
    mode: 'basic',
    maxSignups: 10,
    maxWaitlist: 0,
    perPerson: 1,
    slotStep: 15,
    types: [],
    windows: [],
    festivalMapId: null,
    occupantId: null,
  }
}

/** The form for an existing booth. */
export function gposeBoothForm(b: GposeBooth): GposeBoothForm {
  return {
    id: b.id,
    title: b.title,
    description: b.description,
    mode: b.mode,
    maxSignups: b.mode === 'basic' ? b.maxSignups : 10,
    maxWaitlist: b.maxWaitlist,
    perPerson: b.perPerson,
    slotStep: b.slotStep,
    types: b.types.map((t) => ({ typeId: t.typeId, price: t.price, minutes: t.minutes || 15 })),
    windows: b.windows.map((w) => ({
      startAt: utcToDatetimeLocal(w.startAt),
      endAt: utcToDatetimeLocal(w.endAt),
    })),
    festivalMapId: b.festivalMapId,
    occupantId: b.occupantId,
  }
}

/** The form as the save sends it. */
export function gposeBoothInput(f: GposeBoothForm): GposeBoothInput {
  const timeslot = f.mode === 'timeslot'
  return {
    title: f.title.trim(),
    description: f.description,
    mode: f.mode,
    maxSignups: timeslot ? 0 : Math.max(1, Math.round(f.maxSignups || 0)),
    maxWaitlist: Math.max(0, Math.round(f.maxWaitlist || 0)),
    perPerson: Math.max(1, Math.round(f.perPerson || 0)),
    slotStep: Math.round(f.slotStep || 0),
    types: f.types.map((t) => ({
      typeId: t.typeId,
      price: Math.max(0, Math.round(t.price || 0)),
      minutes: timeslot ? Math.round(t.minutes || 0) : 0,
    })),
    windows: timeslot
      ? f.windows.map((w) => ({
          startAt: datetimeLocalToUtc(w.startAt),
          endAt: datetimeLocalToUtc(w.endAt),
        }))
      : [],
    festivalMapId: f.festivalMapId,
    occupantId: f.festivalMapId ? f.occupantId : null,
  }
}

/** A gil price as a label, or '' for none. */
export function gposePriceLabel(price: number): string {
  return price > 0 ? `${price.toLocaleString()} gil` : ''
}

/** A shoot's time as weekday, day and clock in the reader's zone. */
export function formatGposeTime(at: string | null | undefined): string {
  if (!at) {
    return ''
  }
  const d = new Date(at)
  if (Number.isNaN(d.getTime())) {
    return ''
  }
  return d.toLocaleString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

/** Just the clock of a time, for the end of a slot on the same day. */
export function formatGposeClock(at: string | null | undefined): string {
  if (!at) {
    return ''
  }
  const d = new Date(at)
  return Number.isNaN(d.getTime())
    ? ''
    : d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
}

/** A slot as "Sat, Aug 1, 6:00 PM - 6:15 PM". */
export function formatGposeSlot(
  start: string | null | undefined,
  end: string | null | undefined,
): string {
  const from = formatGposeTime(start)
  return from && end ? `${from} - ${formatGposeClock(end)}` : from
}

/** Where a sign-up stands, in words. */
export function gposeStanding(s: PublicGposeSignup | GposeSignup): string {
  switch (s.status) {
    case 'completed':
      return 'Done'
    case 'no_show':
      return 'Missed'
    case 'canceled':
      return 'Canceled'
  }
  if (s.waitlisted) {
    return `Waitlist #${s.position}`
  }
  if (s.slotStart) {
    return formatGposeSlot(s.slotStart, s.slotEnd)
  }
  return `#${s.position} in line`
}

export const useGposeStore = defineStore('gpose', () => {
  const ui = useUiStore()

  // -- Types ----------------------------------------------------------------------

  const types = ref<GposeType[]>([])
  const typesLoading = ref(false)

  async function loadTypes(): Promise<void> {
    typesLoading.value = true
    try {
      types.value = (await unwrap(api.GET('/gpose-types'))).types
    } catch (e) {
      ui.notify((e as Error).message, 'error')
    } finally {
      typesLoading.value = false
    }
  }

  async function createType(name: string): Promise<boolean> {
    if (!name.trim()) {
      return false
    }
    try {
      await unwrap(api.POST('/gpose-types', { body: { name: name.trim() } }))
      await loadTypes()
      return true
    } catch (e) {
      ui.notify((e as Error).message, 'error')
      return false
    }
  }

  async function renameType(id: number, name: string): Promise<boolean> {
    try {
      await unwrap(
        api.PUT('/gpose-types/{id}', { params: { path: { id } }, body: { name: name.trim() } }),
      )
      await loadTypes()
      return true
    } catch (e) {
      ui.notify((e as Error).message, 'error')
      return false
    }
  }

  async function deleteType(t: GposeType): Promise<void> {
    if (
      !(await ui.confirm(`Delete the ${t.name} type?`, {
        title: 'Delete gpose type',
        confirmText: 'Delete',
      }))
    ) {
      return
    }
    try {
      await unwrap(api.DELETE('/gpose-types/{id}', { params: { path: { id: t.id } } }))
      types.value = types.value.filter((x) => x.id !== t.id)
    } catch (e) {
      ui.notify((e as Error).message, 'error')
    }
  }

  // -- Booths ---------------------------------------------------------------------

  const booths = ref<GposeBooth[]>([])
  const boothsLoading = ref(false)
  /** The booth open on its detail screen, with its types and open times. */
  const selectedBooth = ref<GposeBooth | null>(null)
  const signups = ref<GposeSignup[]>([])
  const signupsLoading = ref(false)
  /** The sign-up a change is being sent for, so its buttons wait. */
  const changing = ref<number | null>(null)
  const boothForm = ref<GposeBoothForm | null>(null)
  const saving = ref(false)
  const festivalMaps = ref<FestivalMap[]>([])
  const mapStalls = ref<FestivalStallOccupant[]>([])

  const openBooths = computed(() => booths.value.filter((b) => b.status === 'open'))
  const closedBooths = computed(() => booths.value.filter((b) => b.status !== 'open'))

  async function loadBooths(): Promise<void> {
    boothsLoading.value = true
    try {
      booths.value = (await unwrap(api.GET('/gpose-booths'))).booths
    } catch (e) {
      ui.notify((e as Error).message, 'error')
    } finally {
      boothsLoading.value = false
    }
  }

  async function loadBoothDetail(id: number): Promise<void> {
    try {
      selectedBooth.value = await unwrap(
        api.GET('/gpose-booths/{id}', { params: { path: { id } } }),
      )
    } catch (e) {
      ui.notify((e as Error).message, 'error')
    }
  }

  async function loadSignups(id: number): Promise<void> {
    signupsLoading.value = signups.value.length === 0
    try {
      signups.value = (
        await unwrap(api.GET('/gpose-booths/{id}/signups', { params: { path: { id } } }))
      ).signups
    } catch (e) {
      ui.notify((e as Error).message, 'error')
    } finally {
      signupsLoading.value = false
    }
  }

  /** Opens a booth's detail screen. */
  async function viewBooth(b: GposeBooth): Promise<void> {
    selectedBooth.value = b
    signups.value = []
    await Promise.all([loadBoothDetail(b.id), loadSignups(b.id)])
  }

  function newBoothForm(): void {
    boothForm.value = emptyGposeBoothForm()
  }

  function editBoothForm(b: GposeBooth): void {
    boothForm.value = gposeBoothForm(b)
  }

  function cancelBoothForm(): void {
    boothForm.value = null
  }

  /** Saves the form; true once saved. */
  async function saveBooth(): Promise<boolean> {
    const f = boothForm.value
    if (!f || saving.value) {
      return false
    }
    saving.value = true
    try {
      const body = gposeBoothInput(f)
      const saved = f.id
        ? await unwrap(api.PUT('/gpose-booths/{id}', { params: { path: { id: f.id } }, body }))
        : await unwrap(api.POST('/gpose-booths', { body }))
      boothForm.value = null
      if (selectedBooth.value?.id === saved.id) {
        selectedBooth.value = saved
      }
      ui.notify(f.id ? 'Booth saved' : 'Booth created', 'success')
      await loadBooths()
      return true
    } catch (e) {
      ui.notify((e as Error).message, 'error')
      return false
    } finally {
      saving.value = false
    }
  }

  async function setBoothClosed(closed: boolean): Promise<void> {
    const b = selectedBooth.value
    if (!b) {
      return
    }
    if (
      closed &&
      !(await ui.confirm(
        'Close this booth? It stops taking sign-ups and leaves the public pages.',
        {
          title: 'Close booth',
          confirmText: 'Close',
        },
      ))
    ) {
      return
    }
    try {
      const params = { path: { id: b.id } }
      selectedBooth.value = await unwrap(
        closed
          ? api.POST('/gpose-booths/{id}/close', { params })
          : api.POST('/gpose-booths/{id}/reopen', { params }),
      )
      ui.notify(closed ? 'Booth closed' : 'Booth reopened', 'success')
      await loadBooths()
    } catch (e) {
      ui.notify((e as Error).message, 'error')
    }
  }

  /** Deletes a booth after asking; true once it is gone. */
  async function deleteBooth(id: number): Promise<boolean> {
    if (
      !(await ui.confirm('Delete this booth and all its sign-ups?', {
        title: 'Delete booth',
        confirmText: 'Delete',
      }))
    ) {
      return false
    }
    try {
      await unwrap(api.DELETE('/gpose-booths/{id}', { params: { path: { id } } }))
      booths.value = booths.value.filter((b) => b.id !== id)
      if (selectedBooth.value?.id === id) {
        selectedBooth.value = null
      }
      ui.notify('Booth deleted', 'info')
      return true
    } catch (e) {
      ui.notify((e as Error).message, 'error')
      return false
    }
  }

  /** Loads the festivals (and the linked one's stalls) for the form's pickers. */
  async function loadFormSources(): Promise<void> {
    if (!types.value.length) {
      await loadTypes()
    }
    try {
      festivalMaps.value = (await unwrap(api.GET('/festival-maps'))).maps
    } catch {
      festivalMaps.value = []
    }
    await loadMapStalls(boothForm.value?.festivalMapId ?? null)
  }

  async function loadMapStalls(mapId: number | null): Promise<void> {
    if (!mapId) {
      mapStalls.value = []
      return
    }
    try {
      const stalls = (
        await unwrap(api.GET('/festival-maps/{id}', { params: { path: { id: mapId } } }))
      ).stalls
      mapStalls.value = stalls.flatMap((stall) => stall.occupants)
    } catch {
      mapStalls.value = []
    }
  }

  /** Files the form's booth under a festival (or none); its stall belongs to the old one. */
  async function setFestivalMap(mapId: number | null): Promise<void> {
    const f = boothForm.value
    if (!f || f.festivalMapId === mapId) {
      return
    }
    f.festivalMapId = mapId
    f.occupantId = null
    await loadMapStalls(mapId)
  }

  // -- Sign-ups, as staff work them ------------------------------------------------

  function replaceSignup(s: GposeSignup): void {
    signups.value = signups.value.map((x) => (x.id === s.id ? s : x))
  }

  /** Sends a change to a sign-up and reads the list again: one change can move others. */
  async function changeSignup(s: GposeSignup, change: GposeSignupChange): Promise<boolean> {
    if (changing.value !== null) {
      return false
    }
    changing.value = s.id
    try {
      const saved = await unwrap(
        api.PATCH('/gpose-booths/{id}/signups/{signupId}', {
          params: { path: { id: s.boothId, signupId: s.id } },
          body: change,
        }),
      )
      replaceSignup(saved)
      await loadSignups(s.boothId)
      return true
    } catch (e) {
      ui.notify((e as Error).message, 'error')
      await loadSignups(s.boothId)
      return false
    } finally {
      changing.value = null
    }
  }

  /** Calls a person up: their dashboard and booth page alert them until they dismiss it. */
  async function callSignup(s: GposeSignup): Promise<void> {
    if (changing.value !== null) {
      return
    }
    changing.value = s.id
    try {
      replaceSignup(
        await unwrap(
          api.POST('/gpose-booths/{id}/signups/{signupId}/call', {
            params: { path: { id: s.boothId, signupId: s.id } },
          }),
        ),
      )
      ui.notify(`Called ${s.characterName}`, 'success')
    } catch (e) {
      ui.notify((e as Error).message, 'error')
    } finally {
      changing.value = null
    }
  }

  async function deleteSignup(s: GposeSignup): Promise<void> {
    if (
      !(await ui.confirm(`Delete ${s.characterName}'s sign-up? This can't be undone.`, {
        title: 'Delete sign-up',
        confirmText: 'Delete',
      }))
    ) {
      return
    }
    try {
      await unwrap(
        api.DELETE('/gpose-booths/{id}/signups/{signupId}', {
          params: { path: { id: s.boothId, signupId: s.id } },
        }),
      )
      await loadSignups(s.boothId)
    } catch (e) {
      ui.notify((e as Error).message, 'error')
    }
  }

  // -- The public side -------------------------------------------------------------

  const publicBooths = ref<PublicGposeBooth[]>([])
  const publicBooth = ref<PublicGposeBooth | null>(null)
  const publicLoading = ref(false)
  const publicNotFound = ref(false)
  /** The visitor's own sign-ups at the booth open publicly. */
  const mySignups = ref<PublicGposeSignup[]>([])
  const signingUp = ref(false)

  async function loadPublicBooths(): Promise<void> {
    publicLoading.value = true
    try {
      publicBooths.value = (await unwrap(api.GET('/gpose-booths/public'))).booths
    } catch {
      publicBooths.value = []
    } finally {
      publicLoading.value = false
    }
  }

  /** Reads one open booth for its page; quiet when `silent` (a background refresh). */
  async function loadPublicBooth(id: number, silent = false): Promise<void> {
    if (!silent) {
      publicLoading.value = true
      publicNotFound.value = false
    }
    try {
      publicBooth.value = await unwrap(
        api.GET('/gpose-booths/{id}/public', { params: { path: { id } } }),
      )
    } catch {
      if (!silent) {
        publicBooth.value = null
        publicNotFound.value = true
      }
    } finally {
      publicLoading.value = false
    }
  }

  /** A visitor's own sign-ups at a booth, by their whole name and world. */
  async function lookUp(
    boothId: number,
    name: string,
    world: string,
  ): Promise<PublicGposeSignup[] | null> {
    if (!name.trim() || !world.trim()) {
      return []
    }
    try {
      return (
        await unwrap(
          api.POST('/gpose-booths/{id}/lookup', {
            params: { path: { id: boothId } },
            body: { characterName: name.trim(), world: world.trim() },
          }),
        )
      ).signups
    } catch {
      return null
    }
  }

  /**
   * Signs the visitor up, remembering on this device the name it was made under so
   * the booth page finds it again. Answers the sign-up, or null when refused (the
   * reason is shown).
   */
  async function signUp(
    booth: { id: number; title: string },
    req: GposeSignUpRequest,
  ): Promise<PublicGposeSignup | null> {
    if (signingUp.value) {
      return null
    }
    signingUp.value = true
    try {
      const made = await unwrap(
        api.POST('/gpose-booths/{id}/sign-up', {
          params: { path: { id: booth.id } },
          body: {
            characterName: req.characterName.trim(),
            world: req.world.trim(),
            typeId: req.typeId,
            slotStart: req.waitlist ? undefined : req.slotStart,
            waitlist: req.waitlist || undefined,
            turnstileToken: req.turnstileToken || undefined,
          },
        }),
      )
      saveGposeSignup({
        boothId: booth.id,
        boothTitle: booth.title,
        name: req.characterName.trim(),
        world: req.world.trim(),
      })
      ui.notify(made.waitlisted ? "You're on the waitlist." : "You're signed up!", 'success')
      return made
    } catch (e) {
      ui.notify((e as Error).message, 'error')
      return null
    } finally {
      signingUp.value = false
    }
  }

  return {
    types,
    typesLoading,
    loadTypes,
    createType,
    renameType,
    deleteType,
    booths,
    boothsLoading,
    openBooths,
    closedBooths,
    selectedBooth,
    signups,
    signupsLoading,
    changing,
    boothForm,
    saving,
    festivalMaps,
    mapStalls,
    loadBooths,
    loadBoothDetail,
    loadSignups,
    viewBooth,
    newBoothForm,
    editBoothForm,
    cancelBoothForm,
    saveBooth,
    setBoothClosed,
    deleteBooth,
    loadFormSources,
    setFestivalMap,
    changeSignup,
    callSignup,
    deleteSignup,
    publicBooths,
    publicBooth,
    publicLoading,
    publicNotFound,
    mySignups,
    signingUp,
    loadPublicBooths,
    loadPublicBooth,
    lookUp,
    signUp,
  }
})
