import { describe, it, expect, vi, afterEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import EmojiPickerModal from '@/components/common/EmojiPickerModal.vue'
import { axeFindings } from '@/__tests__/axe'

afterEach(() => {
  vi.restoreAllMocks()
  document.body.innerHTML = ''
})

describe('EmojiPickerModal', () => {
  // The real vue3-emoji-picker (2.x), lazy-loaded as in the app. It warns about each 1.x
  // prop it is given, but only once per prop for the whole run, so this is the file's first
  // mount: a clean console says the modal uses the 2.x names.
  it('emits the chosen emoji character, and gives the picker no 1.x props', async () => {
    const warn = vi.spyOn(console, 'warn')
    const wrapper = mount(EmojiPickerModal, { attachTo: document.body })
    let emoji: HTMLElement | null = null
    await vi.waitFor(async () => {
      await flushPromises()
      emoji = document.querySelector<HTMLElement>('.v3-emojis [data-i]')
      expect(emoji).not.toBeNull()
    })

    const fromPicker = warn.mock.calls.filter((args) =>
      String(args[0]).includes('[vue3-emoji-picker]'),
    )
    expect(fromPicker).toEqual([])

    ;(emoji as unknown as HTMLElement).click()
    await flushPromises()
    const [[chosen]] = wrapper.emitted('select') as [[string]]
    expect(chosen).toBeTruthy()
    // The character itself, not a code such as "1f600".
    expect(chosen).not.toMatch(/^[0-9a-f-]+$/i)
    wrapper.unmount()
  })

  // The picker renders every emoji as a button, thousands of nodes for axe to walk, so
  // this one takes seconds alone and longer beside the rest of the check.
  it('has nothing axe flags once the picker has loaded', { timeout: 60_000 }, async () => {
    const wrapper = mount(EmojiPickerModal, {
      props: { allowClear: true },
      attachTo: document.body,
    })
    await vi.waitFor(
      async () => {
        await flushPromises()
        expect(document.querySelector('.v3-emojis [data-i]')).not.toBeNull()
      },
      { timeout: 10_000 },
    )
    expect(await axeFindings(wrapper.element)).toEqual([])
    wrapper.unmount()
  })
})
