import { describe, it, expect, beforeEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type {
  FestivalDashboard,
  FestivalGposeBooth,
  PublicFestivalMap,
  PublicGposeBooth,
  PublicGposeSignup,
  PublicStampCard,
} from '@/api/client'

// The store calls the API client; only its calls are faked (unwrap is the real one).
// The festival's map and dashboard answers are faked as plain values (or a refusal),
// and each call is answered the way the client does: `{ data, error, response }`.
const ep = vi.hoisted(() => {
  const publicDetail = vi.fn()
  const dashboard = vi.fn()
  const enter = vi.fn()
  const lookup = vi.fn()
  const reply = async (fn: () => unknown) => {
    try {
      return { data: await fn(), response: new Response(null, { status: 200 }) }
    } catch (e) {
      return {
        error: { title: 'Not Found', detail: (e as Error).message },
        response: new Response(null, { status: 404 }),
      }
    }
  }
  interface Init {
    params: { path: { idOrSlug: string; id?: number } }
    body: unknown
  }
  return {
    publicDetail,
    dashboard,
    enter,
    lookup,
    GET: vi.fn((_path: string, init: Init) => reply(() => publicDetail(init.params.path.idOrSlug))),
    POST: vi.fn((path: string, init: Init) => {
      if (path === '/raffles/{id}/enter') {
        return enter(path, init)
      }
      if (path === '/gpose-booths/{id}/lookup') {
        return reply(() => lookup(init.params.path.id, init.body))
      }
      return reply(() => dashboard(init.params.path.idOrSlug, init.body))
    }),
  }
})
vi.mock('@/api/client', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  api: { GET: ep.GET, POST: ep.POST },
}))

import { festivalConnectState, useFestivalDashboardStore } from '@/stores/festivalDashboard'
import { useGaraponsStore } from '@/stores/garapons'
import { useStampRalliesStore } from '@/stores/stampRallies'
import {
  saveFestivalVisit,
  saveRallySignup,
  savedFestivalVisit,
  savedRaffleSignup,
  savedRallySignup,
} from '@/lib/signups'

function map(over: Partial<PublicFestivalMap> = {}): PublicFestivalMap {
  return {
    id: 7,
    title: 'Obon',
    slug: 'obon',
    publicPath: 'obon',
    stalls: [],
    times: [],
    ...over,
  } as PublicFestivalMap
}

function card(): PublicStampCard {
  return {
    participantName: 'Aria @ Gilgamesh',
    completed: false,
    rally: { id: 3, title: 'Obon Rally', completionMode: 'all' },
    stamps: [
      { id: 1, occupantId: 11, collected: true, stampType: 'food' },
      { id: 2, occupantId: 12, collected: false, stampType: 'game' },
      { id: 3, collected: true, stampType: 'food' },
    ],
    prizes: [],
  } as unknown as PublicStampCard
}

function dashboard(over: Partial<FestivalDashboard> = {}): FestivalDashboard {
  return {
    participantName: '',
    world: '',
    rally: null,
    card: null,
    cardToken: null,
    garapon: null,
    raffles: [],
    ema: null,
    gposeBooths: [],
    ...over,
  }
}

/** What the last dashboard read sent. */
function lastSent(): {
  cardToken: string
  garaponToken: string
  characterName: string
  world: string
} {
  return ep.dashboard.mock.calls.at(-1)![1]
}

beforeEach(() => {
  setActivePinia(createPinia())
  localStorage.clear()
  window.history.replaceState(null, '')
  ep.publicDetail.mockReset()
  ep.dashboard.mockReset()
  ep.enter.mockReset()
  ep.GET.mockClear()
  ep.POST.mockClear()
  ep.publicDetail.mockResolvedValue(map({ stampRallyId: 3 }))
})

describe('opening a festival', () => {
  it('sends the links this device holds for the festival rally', async () => {
    saveRallySignup({
      rallyId: 3,
      rallyTitle: 'Obon Rally',
      name: 'Aria',
      world: 'Gilgamesh',
      cardToken: 'card-a',
      garaponToken: 'draw-a',
    })
    ep.dashboard.mockResolvedValue(dashboard())
    await useFestivalDashboardStore().open('obon')

    expect(ep.dashboard).toHaveBeenCalledWith('obon', {
      cardToken: 'card-a',
      garaponToken: 'draw-a',
      characterName: 'Aria',
      world: 'Gilgamesh',
    })
  })

  it("hands the card to the stamp-rally store, so the card page's panel renders it", async () => {
    ep.dashboard.mockResolvedValue(
      dashboard({
        participantName: 'Aria',
        world: 'Gilgamesh',
        card: card(),
        cardToken: 'card-a',
        rally: { id: 3, title: 'Obon Rally', cardImage: '', details: '', signupOpen: true },
      }),
    )
    const store = useFestivalDashboardStore()
    await store.open('obon')

    expect(useStampRalliesStore().publicCard?.rally.title).toBe('Obon Rally')
    // Collected stamps that sit on a map occupant - the third is on none.
    expect([...store.collectedOccupantIds]).toEqual([11])
    // What the read confirmed is kept for next time.
    expect(savedRallySignup(3)?.cardToken).toBe('card-a')
    expect(savedFestivalVisit(7)).toMatchObject({ name: 'Aria', world: 'Gilgamesh', rallyId: 3 })
  })

  it('shows the drum without a drawing link, and only draws with one', async () => {
    const garapon = { id: 5, title: 'Drum', status: 'open', prizes: [] }
    ep.dashboard.mockResolvedValue(
      dashboard({
        garapon: {
          garapon,
          defaultDraws: 2,
          player: null,
          draws: [],
          hasPairedLink: true,
          pairedDrawsLeft: 2,
        } as unknown as FestivalDashboard['garapon'],
      }),
    )
    const store = useFestivalDashboardStore()
    await store.open('obon')

    expect(useGaraponsStore().publicGarapon?.title).toBe('Drum')
    expect(useGaraponsStore().publicPlayer).toBeNull()
    expect(store.garaponToken).toBe('')
  })

  it('reports a festival that is not published', async () => {
    ep.publicDetail.mockRejectedValue(new Error('404'))
    const store = useFestivalDashboardStore()
    await store.open('draft')
    expect(store.notFound).toBe(true)
    expect(ep.dashboard).not.toHaveBeenCalled()
  })
})

describe('links handed over from the card and drawing pages', () => {
  it("takes a drawing link in place of the device's own, and lets the server find its card", async () => {
    saveRallySignup({
      rallyId: 3,
      rallyTitle: 'Obon Rally',
      name: 'Aria',
      world: 'Gilgamesh',
      cardToken: 'card-a',
      garaponToken: 'draw-a',
    })
    window.history.replaceState(festivalConnectState({ garaponToken: 'draw-b' }), '')
    ep.dashboard.mockResolvedValue(dashboard())
    await useFestivalDashboardStore().open('obon')

    expect(lastSent().garaponToken).toBe('draw-b')
    // Card A belongs with draw A; the server returns draw B's own card.
    expect(lastSent().cardToken).toBe('')
    // Taken off the history entry, so a reload can't replay it after a "forget".
    expect((window.history.state as Record<string, unknown>).festivalConnect).toBeNull()
  })

  it('drops the drawing link kept for a different card', async () => {
    saveRallySignup({
      rallyId: 3,
      rallyTitle: 'Obon Rally',
      name: 'Aria',
      world: 'Gilgamesh',
      cardToken: 'card-a',
      garaponToken: 'draw-a',
    })
    window.history.replaceState(festivalConnectState({ cardToken: 'card-b' }), '')
    ep.dashboard.mockResolvedValue(dashboard())
    await useFestivalDashboardStore().open('obon')

    expect(lastSent()).toMatchObject({ cardToken: 'card-b', garaponToken: '' })
  })
})

describe('keeping the device honest', () => {
  it('forgets a saved card the festival no longer knows', async () => {
    saveRallySignup({
      rallyId: 3,
      rallyTitle: 'Obon Rally',
      name: 'Aria',
      world: 'Gilgamesh',
      cardToken: 'gone',
    })
    ep.dashboard.mockResolvedValue(dashboard())
    await useFestivalDashboardStore().open('obon')
    expect(savedRallySignup(3)).toBeUndefined()
  })

  it('reads again with the card when the map did not name the rally', async () => {
    // Outside its window the public map stops naming the rally, so the first read
    // goes out without the card the device holds for it.
    ep.publicDetail.mockResolvedValue(map({ stampRallyId: null }))
    saveRallySignup({
      rallyId: 3,
      rallyTitle: 'Obon Rally',
      name: 'Aria',
      world: 'Gilgamesh',
      cardToken: 'card-a',
    })
    const rally = { id: 3, title: 'Obon Rally', cardImage: '', details: '', signupOpen: false }
    ep.dashboard
      .mockResolvedValueOnce(dashboard({ rally }))
      .mockResolvedValueOnce(dashboard({ rally, card: card(), cardToken: 'card-a' }))
    const store = useFestivalDashboardStore()
    await store.open('obon')

    expect(ep.dashboard).toHaveBeenCalledTimes(2)
    expect(lastSent().cardToken).toBe('card-a')
    expect(store.dashboard?.card).toBeTruthy()
  })
})

describe('entering a raffle', () => {
  it('enters under the visitor the dashboard knows and keeps the spelling', async () => {
    saveFestivalVisit({ mapId: 7, name: 'Aria', world: 'Gilgamesh' })
    const raffle = { id: 9, title: 'Gaelicat Raffle', maxEntries: 3 }
    ep.dashboard.mockResolvedValue(
      dashboard({
        participantName: 'Aria',
        world: 'Gilgamesh',
        raffles: [
          { raffle, running: true, totalEntries: 0 } as unknown as FestivalDashboard['raffles'][0],
        ],
      }),
    )
    ep.enter.mockResolvedValue({
      data: { message: 'Entered', amountDue: 50000 },
      response: new Response(null, { status: 201 }),
    })
    const store = useFestivalDashboardStore()
    await store.open('obon')

    expect(await store.enterRaffle(9, 2, 'ts')).toBe(true)
    expect(ep.enter).toHaveBeenCalledWith('/raffles/{id}/enter', {
      params: { path: { id: 9 } },
      body: { characterName: 'Aria', world: 'Gilgamesh', numEntries: 2, turnstileToken: 'ts' },
    })
    expect(store.entered[9]?.amountDue).toBe(50000)
    expect(savedRaffleSignup(9)).toMatchObject({ name: 'Aria', world: 'Gilgamesh' })
  })
})

describe('the gpose booths', () => {
  const signup = (over: Partial<PublicGposeSignup> = {}): PublicGposeSignup => ({
    id: 1,
    boothId: 4,
    typeName: 'Solo',
    price: 0,
    minutes: 0,
    waitlisted: false,
    position: 0,
    slotStart: null,
    slotEnd: null,
    status: 'completed',
    paid: false,
    calledAt: null,
    createdAt: '2026-08-01T12:00:00Z',
    photoUrl: '',
    photoExpiresAt: null,
    ...over,
  })
  const booth = (id: number, open: boolean, signups: PublicGposeSignup[]): FestivalGposeBooth => ({
    booth: { id, title: `Booth ${id}` } as PublicGposeBooth,
    open,
    signups,
  })

  it('reads the open booths again while a gpose may come, and keeps a closed one as it was', async () => {
    const shot = '/api/v1/gpose-photos/0123456789abcdef0123456789abcdef'
    ep.dashboard.mockResolvedValue(
      dashboard({
        participantName: 'Aria',
        world: 'Gilgamesh',
        gposeBooths: [
          booth(4, true, [signup()]),
          booth(5, false, [signup({ id: 2, boothId: 5, photoUrl: shot })]),
        ],
      }),
    )
    const store = useFestivalDashboardStore()
    await store.open('obon')
    expect(store.gposeWaiting).toBe(false)
    expect(store.gposeAwaitingPhoto).toBe(true)

    ep.lookup.mockResolvedValueOnce({ signups: [signup({ photoUrl: shot })] })
    await store.refreshGpose()
    expect(ep.lookup).toHaveBeenCalledTimes(1)
    expect(ep.lookup.mock.calls[0]?.[0]).toBe(4)
    expect(store.gposeBooths.map((b) => b.signups[0]?.photoUrl)).toEqual([shot, shot])
    expect(store.gposeAwaitingPhoto).toBe(false)
  })
})
