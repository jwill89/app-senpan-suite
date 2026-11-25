/**
 * Fits each festival-map pitch's label inside its pitch: whole words only, every
 * line showing.
 *
 * The plan draws every label at ONE size (`.map-stall` in mapeditor.css), and CSS
 * alone can only cap that size by a pitch's height. Whether a name fits ACROSS a
 * pitch depends on its longest word, which only layout knows. So after layout each
 * label is measured, and one that overflows its pitch gets the largest factor that
 * fits written to it as `--label-fit`, which `.map-stall-label` multiplies its size
 * by. A label that fits is left at the plan's size, so names still read at one size
 * across the plan wherever they can.
 *
 * The label sizes are in the plan's own units (`cqw`), so a factor holds at any
 * rendered size of the plan: a resize re-measures only because line breaks can
 * land differently, and zooming, which is a transform, needs nothing.
 *
 * Used by the public map and the admin editor alike, so the editor shows a label
 * exactly as visitors will see it.
 */
import { onBeforeUnmount, onMounted, type ShallowRef } from 'vue'

/** The smallest factor tried: past this a name is too long for its pitch to say. */
const MIN_FIT = 0.3

/** Halvings of the search: 7 lands within half a percent of the best fit. */
const STEPS = 7

const LABEL = '.map-stall-label'

function overflows(label: HTMLElement): boolean {
  // Half a pixel of slack: sub-pixel rounding can report 1px over on a label that
  // visibly fits.
  return (
    label.scrollWidth > label.clientWidth + 0.5 || label.scrollHeight > label.clientHeight + 0.5
  )
}

/**
 * Sets one label's `--label-fit` to the largest factor at which it fits, or
 * clears it when the label fits at the plan's size. A label with no box (its map
 * hidden) is left alone, to be measured when it shows.
 */
export function fitLabel(label: HTMLElement): void {
  if (!label.clientWidth || !label.clientHeight) {
    return
  }
  label.style.removeProperty('--label-fit')
  if (!overflows(label)) {
    return
  }
  let fits = MIN_FIT
  let tooBig = 1
  for (let i = 0; i < STEPS; i++) {
    const mid = (fits + tooBig) / 2
    label.style.setProperty('--label-fit', mid.toFixed(3))
    if (overflows(label)) {
      tooBig = mid
    } else {
      fits = mid
    }
  }
  label.style.setProperty('--label-fit', fits.toFixed(3))
}

/**
 * Keeps every label inside `canvasRef` fitted: on mount, when the plan resizes,
 * when a pitch changes (a rename, a resize in the editor, a day filter) and when
 * a web font arrives. Returns `refit`, for the moments an observer can miss - the
 * plan image's load, which is when the pitches first get their height.
 */
export function useStallLabelFit(canvasRef: Readonly<ShallowRef<HTMLElement | null>>): {
  refit: () => void
} {
  /** Labels waiting to be measured; `null` means all of them. */
  let dirty: Set<HTMLElement> | null = new Set()
  let scheduled = false
  let resizeObserver: ResizeObserver | null = null
  let mutationObserver: MutationObserver | null = null

  function run(): void {
    scheduled = false
    const canvas = canvasRef.value
    if (!canvas) {
      return
    }
    const labels = dirty ?? canvas.querySelectorAll<HTMLElement>(LABEL)
    dirty = new Set()
    labels.forEach((label) => {
      if (label.isConnected) {
        fitLabel(label)
      }
    })
  }

  function schedule(label?: HTMLElement): void {
    if (!label) {
      dirty = null
    } else if (dirty) {
      dirty.add(label)
    }
    if (scheduled) {
      return
    }
    scheduled = true
    // Before the next paint, so a resized pitch never shows its label overflowing.
    // A page that paints nothing runs no animation frames - a background tab, an
    // embed scrolled out of view, a window behind another - and may still call
    // itself visible, so a timer backs the frame up: whichever comes first fits.
    const once = (): void => {
      if (scheduled) {
        run()
      }
    }
    if (!document.hidden && typeof requestAnimationFrame === 'function') {
      requestAnimationFrame(once)
    }
    setTimeout(once, 100)
  }

  function refit(): void {
    schedule()
  }

  /** The label a mutation touched, or none when it was a label's own fit. */
  function labelOf(record: MutationRecord): HTMLElement | null | 'self' {
    const target = record.target instanceof Element ? record.target : record.target.parentElement
    if (!target) {
      return null
    }
    if (record.type === 'attributes' && target.matches(LABEL)) {
      return 'self' // this composable writing --label-fit
    }
    const stall = target.closest('.map-stall')
    return stall?.querySelector<HTMLElement>(LABEL) ?? null
  }

  function onFontsLoaded(): void {
    schedule()
  }

  onMounted(() => {
    const canvas = canvasRef.value
    if (!canvas) {
      return
    }
    schedule()
    if (typeof ResizeObserver !== 'undefined') {
      resizeObserver = new ResizeObserver(() => schedule())
      resizeObserver.observe(canvas)
    }
    mutationObserver = new MutationObserver((records) => {
      for (const record of records) {
        const label = labelOf(record)
        if (label === 'self') {
          continue
        }
        if (label) {
          schedule(label)
        } else {
          // A pitch added or removed, or a change outside any one pitch.
          schedule()
        }
      }
    })
    mutationObserver.observe(canvas, {
      subtree: true,
      childList: true,
      characterData: true,
      attributes: true,
      attributeFilter: ['style', 'class'],
    })
    // Absent in jsdom, which the specs run in.
    if ('fonts' in document) {
      document.fonts.addEventListener('loadingdone', onFontsLoaded)
    }
  })

  onBeforeUnmount(() => {
    resizeObserver?.disconnect()
    mutationObserver?.disconnect()
    if ('fonts' in document) {
      document.fonts.removeEventListener('loadingdone', onFontsLoaded)
    }
  })

  return { refit }
}
