import { describe, it, expect, beforeEach, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import type { User } from '@/api/client'

// The tab reads through the API client; only its calls are faked (unwrap is the real one).
const ok = (data: unknown) => ({ data, response: new Response(null, { status: 200 }) })
const api = vi.hoisted(() => ({ GET: vi.fn(), POST: vi.fn(), PATCH: vi.fn(), DELETE: vi.fn() }))
vi.mock('@/api/client', async (importOriginal) => ({
  ...(await importOriginal<object>()),
  api,
}))

import UsersTab from '@/components/admin/UsersTab.vue'
import { useAuthStore } from '@/stores/auth'
import { axeFindings } from '@/__tests__/axe'

function user(overrides: Partial<User>): User {
  return {
    id: 1,
    username: 'someone',
    isAdmin: false,
    isActive: true,
    permissions: [],
    createdAt: '2026-11-09T10:00:00Z',
    lastLoginAt: null,
    hasPassword: true,
    owner: false,
    discord: null,
    ...overrides,
  }
}

const owner = user({ id: 1, username: 'mochi', isAdmin: true, owner: true })
const otherAdmin = user({ id: 2, username: 'kiri', isAdmin: true })
const staff = user({ id: 3, username: 'sora', permissions: ['bingo-cards'] })

/** Mounts the tab signed in as `me`, with the three accounts listed. */
async function mountAs(me: User): Promise<VueWrapper> {
  useAuthStore().user = me
  const wrapper = mount(UsersTab)
  await flushPromises()
  return wrapper
}

/** The table row naming username. */
function row(wrapper: VueWrapper, username: string): HTMLTableRowElement {
  const cell = wrapper.findAll('td .code-highlight').find((c) => c.text() === username)
  if (!cell) {
    throw new Error(`no row for ${username}`)
  }
  return cell.element.closest('tr')!
}

/** The labels of the buttons in a row. */
function buttons(tr: HTMLTableRowElement): string[] {
  return [...tr.querySelectorAll('button')].map((b) => b.textContent.trim())
}

beforeEach(() => {
  setActivePinia(createPinia())
  Object.values(api).forEach((fn) => fn.mockReset())
  api.GET.mockImplementation(async (path: string) =>
    ok(path === '/config' ? { discordSignIn: true } : { users: [owner, otherAdmin, staff] }),
  )
})

describe('the Users tab', () => {
  it("offers no way to change or delete an owner, nor another admin a link to the owner's Discord", async () => {
    const wrapper = await mountAs(otherAdmin)
    const tr = row(wrapper, 'mochi')
    expect(tr.textContent).toContain('Owner account')
    expect(tr.textContent).toContain('Owner')
    expect(buttons(tr)).toEqual([])
    expect(buttons(row(wrapper, 'sora'))).toEqual(['Discord', 'Edit', 'Delete'])
  })

  it('lets an owner link its own Discord account', async () => {
    const wrapper = await mountAs(owner)
    expect(buttons(row(wrapper, 'mochi'))).toEqual(['Discord'])
  })

  it("doesn't offer an admin to delete, deactivate or demote its own account", async () => {
    const wrapper = await mountAs(otherAdmin)
    const tr = row(wrapper, 'kiri')
    expect(buttons(tr)).toEqual(['Discord', 'Edit'])

    const edit = [...tr.querySelectorAll('button')].find((b) => b.textContent.includes('Edit'))!
    edit.click()
    await flushPromises()
    const boxes = wrapper.findAll('.perm-option input[type="checkbox"]').slice(0, 2)
    expect(boxes.map((b) => (b.element as HTMLInputElement).disabled)).toEqual([true, true])
    expect(wrapper.text()).toContain("another admin can deactivate or demote it, but you can't")
  })

  it("leaves another admin's account open to change", async () => {
    const wrapper = await mountAs(owner)
    const tr = row(wrapper, 'kiri')
    expect(buttons(tr)).toEqual(['Discord', 'Edit', 'Delete'])
    const edit = [...tr.querySelectorAll('button')].find((b) => b.textContent.includes('Edit'))!
    edit.click()
    await flushPromises()
    const boxes = wrapper.findAll('.perm-option input[type="checkbox"]').slice(0, 2)
    expect(boxes.map((b) => (b.element as HTMLInputElement).disabled)).toEqual([false, false])
  })

  it('has nothing axe flags', async () => {
    const wrapper = await mountAs(otherAdmin)
    expect(await axeFindings(wrapper.element)).toEqual([])
  })
})
