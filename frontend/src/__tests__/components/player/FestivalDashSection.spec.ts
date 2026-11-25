import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FestivalDashSection from '@/components/player/FestivalDashSection.vue'
import { axeFindings } from '@/__tests__/axe'

function mountSection(collapsed = false) {
  return mount(FestivalDashSection, {
    props: {
      id: 'stamps',
      title: 'Obon Matsuri Stamp Rally',
      icon: 'stamp',
      summary: '2 of 4',
      collapsed,
      current: true,
    },
    slots: { default: '<p>The stamp card</p>' },
    attachTo: document.body,
  })
}

describe('FestivalDashSection', () => {
  it('heads its section with a button naming it and its tally', async () => {
    const wrapper = mountSection()
    const section = wrapper.get('section')
    expect(section.attributes('id')).toBe('festival-stamps')
    expect(section.attributes('aria-labelledby')).toBe('festival-stamps-title')

    const toggle = wrapper.get('#festival-stamps-title button')
    expect(toggle.text()).toContain('Obon Matsuri Stamp Rally')
    expect(toggle.text()).toContain('2 of 4')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(toggle.attributes('aria-controls')).toBe('festival-stamps-body')
    expect(wrapper.get('#festival-stamps-body').isVisible()).toBe(true)
    expect(await axeFindings(wrapper.element)).toEqual([])
    wrapper.unmount()
  })

  it('asks to be folded from its heading', async () => {
    const wrapper = mountSection()
    await wrapper.get('button').trigger('click')
    expect(wrapper.emitted('toggle')).toHaveLength(1)
    wrapper.unmount()
  })

  it('folded, hides its body but keeps it, and keeps its tally in view', async () => {
    const wrapper = mountSection(true)
    const toggle = wrapper.get('button')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(toggle.text()).toContain('2 of 4')
    expect(wrapper.get('section').classes()).toContain('is-collapsed')
    // Hidden, not unmounted: the plan keeps its zoom and the drum its spin.
    const body = wrapper.get('#festival-stamps-body')
    expect(body.isVisible()).toBe(false)
    expect(body.text()).toBe('The stamp card')
    expect(await axeFindings(wrapper.element)).toEqual([])
    wrapper.unmount()
  })
})
