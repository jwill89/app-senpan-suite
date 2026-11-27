import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { GposeBooth, GposeSignup } from '@/api/client'

// The store calls the API client; only its calls are faked (unwrap is the real one).
const ok = (data: unknown, status = 200) => ({ data, response: new Response(null, { status }) })
const problem = (detail: string, status = 409) => ({
  error: { title: 'Conflict', detail },
  response: new Response(null, { status }),
})
const api = vi.hoisted(() => ({
  GET: vi.fn(),
  POST: vi.fn(),
  PUT: vi.fn(),
  PATCH: vi.fn(),
  DELETE: vi.fn(),
}))
vi.mock('@/api/client', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  api,
}))

import {
  emptyGposeBoothForm,
  formatGposeSlot,
  gposeBoothForm,
  gposeBoothInput,
  gposePriceLabel,
  gposeStanding,
  useGposeStore,
} from '@/stores/gpose'
import { useUiStore } from '@/stores/ui'
import { savedGposeSignup } from '@/lib/signups'

function booth(over: Partial<GposeBooth> = {}): GposeBooth {
  return {
    id: 3,
    title: 'Lantern Gpose',
    description: '',
    mode: 'timeslot',
    maxSignups: 0,
    maxWaitlist: 2,
    perPerson: 1,
    slotStep: 15,
    status: 'open',
    festivalMapId: 7,
    festivalMapName: 'Obon',
    occupantId: 11,
    stallName: 'Tea Bar',
    createdAt: '2026-07-01T00:00:00Z',
    types: [{ typeId: 1, name: 'Solo', price: 5000, minutes: 30 }],
    windows: [{ startAt: '2026-08-01T18:00:00Z', endAt: '2026-08-01T20:00:00Z' }],
    bookedCount: 0,
    waitlistCount: 0,
    waitingCount: 0,
    ...over,
  }
}

function signup(over: Partial<GposeSignup> = {}): GposeSignup {
  return {
    id: 8,
    boothId: 3,
    typeId: 1,
    typeName: 'Solo',
    price: 5000,
    minutes: 30,
    characterName: 'Aya',
    world: 'Gilgamesh',
    waitlisted: false,
    position: 1,
    slotStart: null,
    slotEnd: null,
    status: 'waiting',
    paid: false,
    calledAt: null,
    createdAt: '2026-08-01T12:00:00Z',
    assignableStarts: [],
    ...over,
  }
}

describe('gpose helpers', () => {
  it('turns a booth into its form and back, keeping only what its mode uses', () => {
    const form = gposeBoothForm(booth())
    expect(form.windows).toHaveLength(1)
    expect(form.types[0]).toEqual({ typeId: 1, price: 5000, minutes: 30 })
    const body = gposeBoothInput(form)
    expect(body.maxSignups).toBe(0)
    expect(body.windows[0]?.startAt).toBe('2026-08-01T18:00:00.000Z')
    expect(body.occupantId).toBe(11)

    const basic = { ...form, mode: 'basic' as const, maxSignups: 0, festivalMapId: null }
    const sent = gposeBoothInput(basic)
    expect(sent.maxSignups).toBe(1)
    expect(sent.windows).toEqual([])
    expect(sent.types[0]?.minutes).toBe(0)
    expect(sent.occupantId).toBeNull()
    expect(emptyGposeBoothForm().mode).toBe('basic')
  })

  it('words prices, slots and where a sign-up stands', () => {
    expect(gposePriceLabel(0)).toBe('')
    expect(gposePriceLabel(5000)).toContain('gil')
    expect(formatGposeSlot('2026-08-01T18:00:00Z', '2026-08-01T18:30:00Z')).toContain(' - ')
    expect(formatGposeSlot(null, null)).toBe('')
    expect(gposeStanding(signup())).toBe('#1 in line')
    expect(gposeStanding(signup({ waitlisted: true, position: 2 }))).toBe('Waitlist #2')
    expect(gposeStanding(signup({ status: 'no_show' }))).toBe('Missed')
    expect(gposeStanding(signup({ status: 'completed' }))).toBe('Done')
    expect(gposeStanding(signup({ status: 'canceled' }))).toBe('Canceled')
    expect(gposeStanding(signup({ slotStart: '2026-08-01T18:00:00Z' }))).not.toBe('#1 in line')
  })
})

describe('gpose store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    localStorage.clear()
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true)
  })

  it('loads booths and splits them by status', async () => {
    api.GET.mockResolvedValueOnce(ok({ booths: [booth(), booth({ id: 4, status: 'closed' })] }))
    const store = useGposeStore()
    await store.loadBooths()
    expect(store.openBooths.map((b) => b.id)).toEqual([3])
    expect(store.closedBooths.map((b) => b.id)).toEqual([4])
  })

  it('saves a new booth from the form and reads the list again', async () => {
    const store = useGposeStore()
    store.newBoothForm()
    store.boothForm!.title = 'Lantern Gpose'
    store.boothForm!.types = [{ typeId: 1, price: 0, minutes: 15 }]
    api.POST.mockResolvedValueOnce(ok(booth({ mode: 'basic' }), 201))
    api.GET.mockResolvedValueOnce(ok({ booths: [booth()] }))
    expect(await store.saveBooth()).toBe(true)
    expect(api.POST.mock.calls[0]?.[0]).toBe('/gpose-booths')
    expect(store.boothForm).toBeNull()

    store.editBoothForm(booth())
    api.PUT.mockResolvedValueOnce(problem('This booth has sign-ups'))
    expect(await store.saveBooth()).toBe(false)
    expect(store.boothForm).not.toBeNull()
  })

  it('changes, calls and deletes sign-ups, reading the list after a change', async () => {
    const store = useGposeStore()
    store.signups = [signup()]
    api.PATCH.mockResolvedValueOnce(ok(signup({ status: 'completed' })))
    api.GET.mockResolvedValueOnce(ok({ signups: [signup({ status: 'completed' })] }))
    expect(await store.changeSignup(signup(), { status: 'completed' })).toBe(true)
    expect(store.signups[0]?.status).toBe('completed')

    api.POST.mockResolvedValueOnce(ok(signup({ calledAt: '2026-08-01T18:00:00Z' })))
    await store.callSignup(signup())
    expect(api.POST.mock.calls[0]?.[0]).toBe('/gpose-booths/{id}/signups/{signupId}/call')
    expect(store.signups[0]?.calledAt).toBe('2026-08-01T18:00:00Z')

    api.DELETE.mockResolvedValueOnce(ok(null, 204))
    api.GET.mockResolvedValueOnce(ok({ signups: [] }))
    await store.deleteSignup(signup())
    expect(store.signups).toEqual([])
  })

  it('signs a visitor up and remembers who it was', async () => {
    const store = useGposeStore()
    api.POST.mockResolvedValueOnce(
      ok({ id: 8, boothId: 3, typeName: 'Solo', waitlisted: true, status: 'waiting' }, 201),
    )
    const made = await store.signUp(
      { id: 3, title: 'Lantern Gpose' },
      { characterName: ' Aya ', world: 'Gilgamesh', typeId: 1, waitlist: true, slotStart: 'x' },
    )
    expect(made?.waitlisted).toBe(true)
    const body = (api.POST.mock.calls[0]?.[1] as { body: Record<string, unknown> }).body
    expect(body.slotStart).toBeUndefined()
    expect(body.characterName).toBe('Aya')
    expect(savedGposeSignup(3)?.name).toBe('Aya')

    api.POST.mockResolvedValueOnce(problem('This booth is full.'))
    expect(
      await store.signUp({ id: 3, title: 'x' }, { characterName: 'B', world: 'W', typeId: 1 }),
    ).toBeNull()
  })

  it('looks a visitor up, and answers null when the read fails', async () => {
    const store = useGposeStore()
    expect(await store.lookUp(3, '', 'Gilgamesh')).toEqual([])
    api.POST.mockResolvedValueOnce(ok({ signups: [{ id: 8 }] }))
    expect(await store.lookUp(3, 'Aya', 'Gilgamesh')).toHaveLength(1)
    api.POST.mockResolvedValueOnce(problem('Too many', 429))
    expect(await store.lookUp(3, 'Aya', 'Gilgamesh')).toBeNull()
  })

  it('reads a public booth, and says when it is gone', async () => {
    const store = useGposeStore()
    api.GET.mockResolvedValueOnce(problem('There is no such gpose booth.', 404))
    await store.loadPublicBooth(3)
    expect(store.publicNotFound).toBe(true)
  })
})
