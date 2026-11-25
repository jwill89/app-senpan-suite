import { describe, it, expect, afterEach } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { defineComponent, h, nextTick, ref, useTemplateRef } from 'vue'
import { fitLabel, useStallLabelFit } from '@/composables/useStallLabelFit'

/**
 * jsdom lays nothing out, so each label here is given a box and a content size
 * that shrinks with its --label-fit, the way a real label's does: `natural` is how
 * wide (or tall) its content is at the plan's size.
 */
function sized(
  label: HTMLElement,
  box: { width: number; height: number },
  natural: { width: number; height: number },
): HTMLElement {
  const fit = () => Number(label.style.getPropertyValue('--label-fit') || 1)
  Object.defineProperties(label, {
    clientWidth: { configurable: true, get: () => box.width },
    clientHeight: { configurable: true, get: () => box.height },
    scrollWidth: { configurable: true, get: () => Math.max(box.width, natural.width * fit()) },
    scrollHeight: { configurable: true, get: () => Math.max(box.height, natural.height * fit()) },
  })
  return label
}

function label(): HTMLElement {
  const el = document.createElement('span')
  el.className = 'map-stall-label'
  return el
}

const fitOf = (el: HTMLElement) => el.style.getPropertyValue('--label-fit')

describe('fitLabel', () => {
  it('leaves a label that fits at the plan size', () => {
    const el = sized(label(), { width: 100, height: 40 }, { width: 80, height: 30 })
    fitLabel(el)
    expect(fitOf(el)).toBe('')
  })

  it('shrinks a label whose longest word is wider than its pitch, just enough to fit', () => {
    const el = sized(label(), { width: 100, height: 40 }, { width: 160, height: 30 })
    fitLabel(el)
    // The largest factor that fits is 100 / 160; the search lands within a step of
    // it, and within the half pixel of rounding slack a real label is allowed.
    expect(Number(fitOf(el))).toBeCloseTo(100 / 160, 2)
    expect(el.scrollWidth).toBeLessThanOrEqual(el.clientWidth + 0.5)
  })

  it('shrinks a label with more lines than its pitch is tall', () => {
    const el = sized(label(), { width: 100, height: 40 }, { width: 90, height: 64 })
    fitLabel(el)
    expect(Number(fitOf(el))).toBeCloseTo(40 / 64, 2)
    expect(el.scrollHeight).toBeLessThanOrEqual(el.clientHeight + 0.5)
  })

  it('clears an old fit once the label fits without it', () => {
    const el = sized(label(), { width: 100, height: 40 }, { width: 80, height: 30 })
    el.style.setProperty('--label-fit', '0.5')
    fitLabel(el)
    expect(fitOf(el)).toBe('')
  })

  it('keeps the fit of a label with no box, whose map is hidden', () => {
    const el = sized(label(), { width: 0, height: 0 }, { width: 80, height: 30 })
    el.style.setProperty('--label-fit', '0.5')
    fitLabel(el)
    expect(fitOf(el)).toBe('0.5')
  })
})

describe('useStallLabelFit', () => {
  let wrapper: VueWrapper | null = null
  afterEach(() => {
    wrapper?.unmount()
    wrapper = null
  })

  /** A canvas holding one pitch, whose name can be changed. */
  const Canvas = defineComponent({
    props: { name: { type: String, required: true } },
    setup(props) {
      const canvasRef = useTemplateRef<HTMLElement>('canvasRef')
      useStallLabelFit(canvasRef)
      return () =>
        h('div', { ref: 'canvasRef', class: 'map-canvas' }, [
          h('div', { class: 'map-stall' }, [h('span', { class: 'map-stall-label' }, props.name)]),
        ])
    },
  })

  /** Long enough for the fit to have run: its animation frame, or the timer behind it. */
  const settle = () => new Promise((resolve) => setTimeout(resolve, 150))

  it('fits the labels once mounted, and again when a name changes', async () => {
    const natural = ref({ width: 160, height: 30 })
    wrapper = mount(Canvas, { props: { name: 'The Green Gaelicat' }, attachTo: document.body })
    const el = sized(
      wrapper.get('.map-stall-label').element as HTMLElement,
      { width: 100, height: 40 },
      natural.value,
    )
    // Mounting already scheduled the first fit; the box above is in place before it runs.
    await settle()
    expect(Number(fitOf(el))).toBeCloseTo(100 / 160, 2)

    // A shorter name fits whole: the rename is seen and the fit lifted.
    natural.value.width = 70
    await wrapper.setProps({ name: 'Front Desk' })
    await nextTick()
    await settle()
    expect(fitOf(el)).toBe('')
  })
})
