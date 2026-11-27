import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { PublicGposeBooth, PublicGposeSignup } from '@/api/client'

const ok = (data: unknown, status = 200) => ({ data, response: new Response(null, { status }) })
const api = vi.hoisted(() => ({ GET: vi.fn(), POST: vi.fn() }))
vi.mock('@/api/client', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  api,
}))

import GposeBoothPanel from '@/components/player/GposeBoothPanel.vue'
import { axeFindings } from '@/__tests__/axe'

const T1 = '2026-08-01T18:00:00Z'
const T2 = '2026-08-01T18:15:00Z'

function booth(over: Partial<PublicGposeBooth> = {}): PublicGposeBooth {
  return {
    id: 3,
    title: 'Lantern Gpose',
    description: 'Pay at the booth.',
    mode: 'timeslot',
    perPerson: 1,
    maxSignups: 0,
    maxWaitlist: 2,
    placesLeft: 0,
    waitlistLeft: 2,
    types: [
      { typeId: 1, name: 'Solo', price: 0, minutes: 15, starts: [T1, T2] },
      { typeId: 2, name: 'Group', price: 10000, minutes: 30, starts: [] },
    ],
    windows: [],
    festivalName: '',
    festivalPath: '',
    stallName: 'Tea Bar',
    ...over,
  }
}

function signup(over: Partial<PublicGposeSignup> = {}): PublicGposeSignup {
  return {
    id: 8,
    boothId: 3,
    typeName: 'Solo',
    price: 0,
    minutes: 15,
    waitlisted: false,
    position: 1,
    slotStart: null,
    slotEnd: null,
    status: 'waiting',
    paid: false,
    calledAt: null,
    createdAt: '2026-08-01T12:00:00Z',
    ...over,
  }
}

function mountPanel(props: Record<string, unknown> = {}) {
  return mount(GposeBoothPanel, {
    props: { booth: booth(), signups: [], turnstileSiteKey: '', ...props },
    global: {
      stubs: { MarkdownText: { props: ['source'], template: '<div>{{ source }}</div>' } },
    },
    attachTo: document.body,
  })
}

describe('GposeBoothPanel', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
    localStorage.clear()
  })

  it('books one of the offered times as the visitor the page knows', async () => {
    const wrapper = mountPanel({
      visitorName: 'Aya',
      visitorWorld: 'Gilgamesh',
      fixedVisitor: true,
    })
    expect(wrapper.text()).toContain('Signing up as')
    expect(wrapper.findAll('select')[1]?.findAll('option')).toHaveLength(2)
    expect(await axeFindings(wrapper.element)).toEqual([])

    api.POST.mockResolvedValueOnce(ok(signup({ slotStart: T1, slotEnd: T2, position: 0 }), 201))
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const body = (api.POST.mock.calls[0]?.[1] as { body: Record<string, unknown> }).body
    expect(body).toMatchObject({
      characterName: 'Aya',
      world: 'Gilgamesh',
      typeId: 1,
      slotStart: T1,
    })
    expect(wrapper.emitted('signed-up')?.[0]?.slice(1)).toEqual(['Aya', 'Gilgamesh'])
    wrapper.unmount()
  })

  it('offers the waitlist in place of a time', async () => {
    const wrapper = mountPanel({
      visitorName: 'Aya',
      visitorWorld: 'Gilgamesh',
      fixedVisitor: true,
    })
    await wrapper.get('input[type="checkbox"]').setValue(true)
    expect(wrapper.findAll('select')).toHaveLength(1)
    api.POST.mockResolvedValueOnce(ok(signup({ waitlisted: true }), 201))
    await wrapper.get('form').trigger('submit')
    await flushPromises()
    const body = (api.POST.mock.calls[0]?.[1] as { body: Record<string, unknown> }).body
    expect(body.waitlist).toBe(true)
    expect(body.slotStart).toBeUndefined()
    wrapper.unmount()
  })

  it('shows where the visitor stands, and stops at the per-person limit', async () => {
    const wrapper = mountPanel({
      booth: booth({ mode: 'basic', maxSignups: 4, placesLeft: 2 }),
      signups: [signup({ calledAt: '2026-08-01T18:00:00Z' })],
    })
    expect(wrapper.text()).toContain('#1 in line')
    expect(wrapper.text()).toContain('Called up')
    expect(wrapper.text()).toContain('2 of 4 places left')
    expect(wrapper.find('form').exists()).toBe(false)
    expect(await axeFindings(wrapper.element)).toEqual([])
    wrapper.unmount()
  })

  it('asks who is signing up when the page does not know, and says when it is full', async () => {
    const wrapper = mountPanel({
      booth: booth({ mode: 'basic', maxSignups: 2, placesLeft: 0, maxWaitlist: 1 }),
    })
    expect(wrapper.find('input[aria-label="Character name"]').exists()).toBe(true)
    expect(wrapper.text()).toContain("you'll join the waitlist")
    expect(await axeFindings(wrapper.element)).toEqual([])
    await wrapper.setProps({
      booth: booth({
        mode: 'basic',
        maxSignups: 2,
        placesLeft: 0,
        maxWaitlist: 1,
        waitlistLeft: 0,
      }),
    })
    expect(wrapper.text()).toContain('This booth is full right now.')
    wrapper.unmount()
  })
})
