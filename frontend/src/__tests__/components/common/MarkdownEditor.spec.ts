import { afterAll, afterEach, beforeAll, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import MarkdownEditor from '@/components/common/MarkdownEditor.vue'
import FormField from '@/components/common/ui/FormField.vue'
import { axeFindings } from '@/__tests__/axe'

// jsdom lays nothing out, and ProseMirror measures the selection to scroll it into
// view whenever the editor takes focus.
const measured = {
  getClientRects: Object.getOwnPropertyDescriptor(Range.prototype, 'getClientRects'),
  getBoundingClientRect: Object.getOwnPropertyDescriptor(Range.prototype, 'getBoundingClientRect'),
  elementFromPoint: Object.getOwnPropertyDescriptor(document, 'elementFromPoint'),
}

beforeAll(() => {
  Range.prototype.getClientRects = () => [] as unknown as DOMRectList
  Range.prototype.getBoundingClientRect = () => new DOMRect(0, 0, 0, 0)
  document.elementFromPoint = () => null
})

afterAll(() => {
  for (const [target, name] of [
    [Range.prototype, 'getClientRects'],
    [Range.prototype, 'getBoundingClientRect'],
    [document, 'elementFromPoint'],
  ] as const) {
    const original = measured[name]
    if (original) {
      Object.defineProperty(target, name, original)
    } else {
      Reflect.deleteProperty(target, name)
    }
  }
})

/** Stands in for the emoji picker modal, which loads the real picker. */
const EmojiPickerStub = defineComponent({
  name: 'EmojiPickerModal',
  emits: ['select', 'close'],
  setup: () => () => h('div', { class: 'emoji-picker-stub' }),
})

const wrappers: VueWrapper[] = []

afterEach(() => {
  wrappers.splice(0).forEach((wrapper) => wrapper.unmount())
  document.body.innerHTML = ''
})

/** Mounts the editor and waits for its engine to load. */
async function mountEditor(modelValue: string, placeholder = 'Describe the raffle') {
  const wrapper = mount(MarkdownEditor, {
    props: { modelValue, placeholder, minHeight: '120px' },
    attachTo: document.body,
    global: { stubs: { EmojiPickerModal: EmojiPickerStub } },
  })
  wrappers.push(wrapper)
  // The engine is a lazy import, which a loaded machine (the whole check at once) can
  // take seconds to resolve.
  await vi.waitFor(
    async () => {
      await flushPromises()
      expect(wrapper.find('button[aria-label="Bold"]').attributes('disabled')).toBeUndefined()
    },
    { timeout: 10_000 },
  )
  return wrapper
}

function surface(wrapper: VueWrapper): HTMLElement {
  const element = wrapper.find('.markdown-editor__surface').element
  return element as HTMLElement
}

/** The values the editor has emitted, newest last. */
function emitted(wrapper: VueWrapper): string[] {
  return (wrapper.emitted('update:modelValue') ?? []).map(([value]) => value as string)
}

describe('MarkdownEditor', () => {
  it('loads the stored Markdown without reporting an edit', async () => {
    const wrapper = await mountEditor('**Prize:** a mount\nDraw at 8')

    expect(surface(wrapper).querySelector('strong')?.textContent).toBe('Prize:')
    expect(surface(wrapper).querySelector('br')).not.toBeNull()
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('reports Markdown when a toolbar button changes the text', async () => {
    const wrapper = await mountEditor('First line')

    await wrapper.find('button[aria-label="Bulleted list"]').trigger('click')

    expect(emitted(wrapper).at(-1)).toBe('- First line')
    expect(wrapper.find('button[aria-label="Bulleted list"]').attributes('aria-pressed')).toBe(
      'true',
    )
  })

  it('sets a heading from the text-style picker', async () => {
    const wrapper = await mountEditor('Rules')

    await wrapper.find('select[aria-label="Text style"]').setValue('h2')

    expect(emitted(wrapper).at(-1)).toBe('## Rules')
  })

  it('replaces the text when the value changes from outside, without echoing it', async () => {
    const wrapper = await mountEditor('Old text')

    await wrapper.setProps({ modelValue: '# New title\n\nNew body' })
    await flushPromises()

    expect(surface(wrapper).querySelector('h1')?.textContent).toBe('New title')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('inserts a picked emoji as the character itself', async () => {
    const wrapper = await mountEditor('Party')

    await wrapper.find('button[aria-label="Emoji"]').trigger('click')
    const picker = wrapper.findComponent(EmojiPickerStub)
    expect(picker.exists()).toBe(true)
    picker.vm.$emit('select', '🎉')
    await flushPromises()

    expect(wrapper.findComponent(EmojiPickerStub).exists()).toBe(false)
    expect(emitted(wrapper).at(-1)).toContain('🎉')
    expect(emitted(wrapper).at(-1)).not.toContain(':')
  })

  it('adds a typed address as a link', async () => {
    const wrapper = await mountEditor('')

    await wrapper.find('button[aria-label="Link"]').trigger('click')
    await wrapper.find('input[aria-label="Link address"]').setValue('example.com/shop')
    await wrapper.find('form.markdown-editor__link').trigger('submit')

    expect(emitted(wrapper).at(-1)).toBe('<https://example.com/shop>')
    expect(wrapper.find('form.markdown-editor__link').exists()).toBe(false)
  })

  it('refuses an address that is not a web or email address', async () => {
    const wrapper = await mountEditor('Click')

    await wrapper.find('button[aria-label="Link"]').trigger('click')
    const input = wrapper.find('input[aria-label="Link address"]')
    await input.setValue('javascript:alert(1)')
    await wrapper.find('form.markdown-editor__link').trigger('submit')

    expect(input.attributes('aria-invalid')).toBe('true')
    expect(wrapper.find('.markdown-editor__link-note').exists()).toBe(true)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('reports leaving the text', async () => {
    const wrapper = await mountEditor('Details')

    surface(wrapper).dispatchEvent(new FocusEvent('focus'))
    surface(wrapper).dispatchEvent(new FocusEvent('blur'))
    await flushPromises()

    expect(wrapper.emitted('blur')).toHaveLength(1)
  })

  it("leaves a form field's label off its text-style picker", async () => {
    const wrapper = mount(FormField, {
      props: { label: 'Rules' },
      slots: { default: () => h(MarkdownEditor, { modelValue: '', placeholder: 'Rules' }) },
      attachTo: document.body,
      global: { stubs: { EmojiPickerModal: EmojiPickerStub } },
    })
    wrappers.push(wrapper)
    await flushPromises()

    expect(wrapper.find('label').attributes('for')).toBeUndefined()
    expect(wrapper.find('select[aria-label="Text style"]').attributes('id')).toBeUndefined()
  })

  it('has nothing axe flags, with the link row open', async () => {
    const wrapper = await mountEditor('Some **text**')
    await wrapper.find('button[aria-label="Link"]').trigger('click')

    expect(await axeFindings(wrapper.find('.markdown-editor').element)).toEqual([])
  })
})
