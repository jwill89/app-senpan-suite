import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { mount } from '@vue/test-utils'

const push = vi.fn()
vi.mock('vue-router', () => ({ useRouter: () => ({ push }) }))

import ToastNotification from '@/components/common/ToastNotification.vue'
import { useUiStore } from '@/stores/ui'
import { axeFindings } from '@/__tests__/axe'

beforeEach(() => {
  setActivePinia(createPinia())
  push.mockReset()
})

describe('ToastNotification alerts', () => {
  it('renders each alert with its message, above the toast', async () => {
    const ui = useUiStore()
    ui.alert({
      title: 'Tea room unlocked',
      message: 'Jasmine Room (Room #3) has been marked as unlocked.',
    })
    ui.notify('Saved', 'success')
    const w = mount(ToastNotification)
    await w.vm.$nextTick()
    const toasts = w.findAll('.toast-notification__toast')
    expect(toasts).toHaveLength(2)
    expect(toasts[0]!.classes()).toContain('toast-notification__toast--alert')
    expect(toasts[0]!.text()).toContain('Jasmine Room (Room #3)')
  })

  it('clicking the card does not dismiss it; its close button does', async () => {
    const ui = useUiStore()
    ui.alert({ title: 'Tea room unlocked', message: 'check it' })
    const w = mount(ToastNotification)
    await w.find('.toast-notification__toast--alert').trigger('click')
    expect(ui.alerts).toHaveLength(1)
    await w.find('.toast-notification__close').trigger('click')
    expect(ui.alerts).toHaveLength(0)
  })

  it('the action button navigates and dismisses', async () => {
    const ui = useUiStore()
    ui.alert({
      title: 'Tea room unlocked',
      message: 'check it',
      action: { label: 'Open Tea Rooms', to: '/admin/teahouse/tea-rooms' },
    })
    const w = mount(ToastNotification)
    await w.find('.toast-notification__actions button').trigger('click')
    expect(push).toHaveBeenCalledWith('/admin/teahouse/tea-rooms')
    expect(ui.alerts).toHaveLength(0)
  })

  it('has nothing axe flags with an alert, its action and a toast', async () => {
    const ui = useUiStore()
    ui.alert({
      title: 'Tea room unlocked',
      message: 'check it',
      action: { label: 'Open Tea Rooms', to: '/admin/teahouse/tea-rooms' },
    })
    ui.notify('Saved', 'success')
    const w = mount(ToastNotification)
    await w.vm.$nextTick()
    expect(await axeFindings(w.element)).toEqual([])
  })
})
