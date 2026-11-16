import { describe, it, expect } from 'vitest'
import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import FormField from '@/components/common/ui/FormField.vue'
import { axeFindings } from '@/__tests__/axe'

describe('FormField', () => {
  it('renders the label and the control slot', () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title' },
      slots: { default: '<input class="probe" />' },
    })
    expect(wrapper.find('.field-label').text()).toBe('Title')
    expect(wrapper.find('input.probe').exists()).toBe(true)
  })

  it('appends a required marker when required', () => {
    const wrapper = mount(FormField, { props: { label: 'Title', required: true } })
    expect(wrapper.find('.field-label .form-field__required').exists()).toBe(true)
  })

  it('omits the required marker by default', () => {
    const wrapper = mount(FormField, { props: { label: 'Title' } })
    expect(wrapper.find('.form-field__required').exists()).toBe(false)
  })

  it('renders help text from the help prop', () => {
    const wrapper = mount(FormField, { props: { label: 'X', help: 'Be careful' } })
    expect(wrapper.find('.form-field__help').text()).toBe('Be careful')
  })

  it('renders the #help slot over the help prop when both are given', () => {
    const wrapper = mount(FormField, {
      props: { label: 'X', help: 'plain' },
      slots: { help: '<a href="#">link</a>' },
    })
    expect(wrapper.find('.form-field__help a').exists()).toBe(true)
  })

  it('renders no label element when neither label prop nor slot is given', () => {
    const wrapper = mount(FormField, { slots: { default: '<input />' } })
    expect(wrapper.find('.field-label').exists()).toBe(false)
  })

  it('wires htmlFor to the label for attribute', () => {
    const wrapper = mount(FormField, { props: { label: 'X', htmlFor: 'my-input' } })
    expect(wrapper.find('.field-label').attributes('for')).toBe('my-input')
  })

  it('ties the label to its only control when no htmlFor is given', async () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title' },
      slots: { default: '<input class="probe" />' },
    })
    await nextTick()
    const id = wrapper.find('input.probe').attributes('id')
    expect(id).toBeTruthy()
    expect(wrapper.find('.field-label').attributes('for')).toBe(id)
  })

  it('keeps an id the control already has when tying the label to it', async () => {
    const wrapper = mount(FormField, {
      props: { label: 'Kind' },
      slots: { default: '<select id="kind"><option>a</option></select>' },
    })
    await nextTick()
    expect(wrapper.find('.field-label').attributes('for')).toBe('kind')
  })

  it('leaves a field holding several controls untied', async () => {
    const wrapper = mount(FormField, {
      props: { label: 'Days' },
      slots: { default: '<input type="checkbox" /><input type="checkbox" />' },
    })
    await nextTick()
    expect(wrapper.find('.field-label').attributes('for')).toBeUndefined()
    expect(wrapper.findAll('input').every((input) => !input.attributes('id'))).toBe(true)
  })

  it('has nothing axe flags with the label tied by itself', async () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title', help: 'Shown on the card' },
      slots: { default: '<input />' },
    })
    await nextTick()
    expect(await axeFindings(wrapper.element)).toEqual([])
  })

  it('has nothing axe flags', async () => {
    const wrapper = mount(FormField, {
      props: { label: 'Title', htmlFor: 'title', required: true, help: 'Shown on the card' },
      slots: { default: '<input id="title" required />' },
    })
    expect(await axeFindings(wrapper.element)).toEqual([])
  })
})
