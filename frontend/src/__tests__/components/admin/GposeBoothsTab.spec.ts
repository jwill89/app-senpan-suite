import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { GposeBooth, GposeSignup } from '@/api/client'

// The tab reads through the API client and uploads through apiUpload; only those
// calls are faked (unwrap is the real one).
const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) })
const api = vi.hoisted(() => ({ GET: vi.fn(), POST: vi.fn(), PATCH: vi.fn(), DELETE: vi.fn() }))
const apiUpload = vi.hoisted(() => vi.fn())
vi.mock('@/api/client', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  api,
}))
vi.mock('@/lib/api', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  apiUpload,
}))

import GposeBoothsTab from '@/components/admin/GposeBoothsTab.vue'
import { useGposeStore } from '@/stores/gpose'
import { useUiStore } from '@/stores/ui'
import { axeFindings } from '@/__tests__/axe'

const SHOT = 'https://apps.example/api/v1/gpose-photos/0123456789abcdef0123456789abcdef'

const booth: GposeBooth = {
  id: 3,
  title: 'Lantern Gpose',
  description: '',
  mode: 'basic',
  maxSignups: 5,
  maxWaitlist: 0,
  perPerson: 1,
  slotStep: 15,
  status: 'open',
  festivalMapId: null,
  festivalMapName: '',
  occupantId: null,
  stallName: '',
  createdAt: '2026-07-01T00:00:00Z',
  types: [{ typeId: 1, name: 'Solo', price: 0, minutes: 0 }],
  windows: [],
  bookedCount: 2,
  waitlistCount: 0,
  waitingCount: 0,
}

function signup(over: Partial<GposeSignup>): GposeSignup {
  return {
    id: 8,
    boothId: 3,
    typeId: 1,
    typeName: 'Solo',
    price: 0,
    minutes: 0,
    characterName: 'Aya',
    world: 'Gilgamesh',
    waitlisted: false,
    position: 0,
    slotStart: null,
    slotEnd: null,
    status: 'completed',
    paid: false,
    calledAt: null,
    createdAt: '2026-08-01T12:00:00Z',
    assignableStarts: [],
    photoUrl: '',
    photoExpiresAt: null,
    ...over,
  }
}

let signups: GposeSignup[] = []

/** Opens the booth's sign-ups from the list, the way staff do. */
async function openBooth(): Promise<VueWrapper> {
  // The admin shell loads the booths when the tab opens; the tab only shows them.
  await useGposeStore().loadBooths()
  const wrapper = mount(GposeBoothsTab, { attachTo: document.body })
  await flushPromises()
  await wrapper.find('.list-row button.btn-view').trigger('click')
  await flushPromises()
  return wrapper
}

/** The table row naming a character. */
function rowOf(wrapper: VueWrapper, name: string) {
  const row = wrapper.findAll('tr').find((tr) => tr.text().includes(name))
  if (!row) {
    throw new Error(`no row for ${name}`)
  }
  return row
}

beforeEach(() => {
  setActivePinia(createPinia())
  Object.values(api).forEach((fn) => fn.mockReset())
  apiUpload.mockReset()
  signups = [
    signup({ id: 8, characterName: 'Aya' }),
    signup({ id: 9, characterName: 'Bo', photoUrl: SHOT, photoExpiresAt: '2026-08-02T18:00:00Z' }),
    signup({ id: 10, characterName: 'Cy', status: 'no_show' }),
  ]
  api.GET.mockImplementation(async (path: string) =>
    ok(
      path === '/gpose-booths'
        ? { booths: [booth] }
        : path === '/gpose-booths/{id}'
          ? booth
          : { signups },
    ),
  )
})

describe("a finished sign-up's gpose", () => {
  it('is uploaded for a completed one, and shown with view, replace and delete once sent', async () => {
    const wrapper = await openBooth()
    expect(rowOf(wrapper, 'Aya').find('button[aria-label="Upload Aya\'s gpose"]').exists()).toBe(
      true,
    )
    const bo = rowOf(wrapper, 'Bo')
    expect(bo.get('a[aria-label="View Bo\'s gpose"]').attributes('href')).toBe(SHOT)
    expect(bo.find('button[aria-label="Replace Bo\'s gpose"]').exists()).toBe(true)
    expect(bo.text()).toContain('Until')
    // A no-show has no gpose to take.
    expect(rowOf(wrapper, 'Cy').find('button[aria-label*="gpose"]').exists()).toBe(false)
    expect(await axeFindings(wrapper.element)).toEqual([])
    wrapper.unmount()
  })

  it('sends the file picked for that sign-up', async () => {
    const wrapper = await openBooth()
    const input = wrapper.get<HTMLInputElement>('input[type="file"]')
    // jsdom opens no file picker, so the click is only watched.
    const click = vi.spyOn(input.element, 'click').mockImplementation(() => undefined)
    await rowOf(wrapper, 'Aya').get('button[aria-label="Upload Aya\'s gpose"]').trigger('click')
    expect(click).toHaveBeenCalled()

    const file = new File(['png'], 'shot.png', { type: 'image/png' })
    Object.defineProperty(input.element, 'files', { value: [file], configurable: true })
    apiUpload.mockResolvedValueOnce(signups[0] && { ...signups[0], photoUrl: SHOT })
    await input.trigger('change')
    await flushPromises()
    expect(apiUpload.mock.calls[0]?.[0]).toBe('gpose-booths/3/signups/8/photo')
    expect((apiUpload.mock.calls[0]?.[1] as FormData).get('image')).toBe(file)
    expect(rowOf(wrapper, 'Aya').find('a[aria-label="View Aya\'s gpose"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('is deleted after asking', async () => {
    vi.spyOn(useUiStore(), 'confirm').mockResolvedValue(true)
    const wrapper = await openBooth()
    api.DELETE.mockResolvedValueOnce({ data: null, response: new Response(null, { status: 204 }) })
    await rowOf(wrapper, 'Bo').get('button[aria-label="Delete Bo\'s gpose"]').trigger('click')
    await flushPromises()
    expect(api.DELETE.mock.calls[0]?.[0]).toBe('/gpose-booths/{id}/signups/{signupId}/photo')
    expect(rowOf(wrapper, 'Bo').find('button[aria-label="Upload Bo\'s gpose"]').exists()).toBe(true)
    wrapper.unmount()
  })
})
