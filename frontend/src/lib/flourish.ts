/**
 * Cleans a theme's flourish SVG before CornerFlourish inlines it with v-html.
 *
 * The server sanitizes an SVG when it is uploaded (internal/uploads/svg.go); this is
 * defense in depth against a tampered or older file. It also takes every `style`
 * attribute apart, as the server does on upload: the page's Content-Security-Policy
 * refuses style attributes in inserted markup, so each safe declaration becomes the SVG
 * presentation attribute of the same name, which the policy doesn't govern and which
 * draws the same.
 */

/** Elements dropped with everything inside them: scripting and embedding vectors. */
const BANNED_TAGS = new Set([
  'script',
  'foreignobject',
  'iframe',
  'object',
  'embed',
  'style',
  'handler',
  'set',
  'animate',
])

/**
 * The SVG presentation attributes a style declaration may become: each takes the same
 * value as the CSS property of its name.
 */
const PRESENTATION_ATTRIBUTES = new Set([
  'alignment-baseline',
  'baseline-shift',
  'clip-rule',
  'color',
  'color-interpolation',
  'color-interpolation-filters',
  'direction',
  'display',
  'dominant-baseline',
  'fill',
  'fill-opacity',
  'fill-rule',
  'flood-color',
  'flood-opacity',
  'font-family',
  'font-size',
  'font-size-adjust',
  'font-stretch',
  'font-style',
  'font-variant',
  'font-weight',
  'image-rendering',
  'letter-spacing',
  'lighting-color',
  'opacity',
  'overflow',
  'paint-order',
  'pointer-events',
  'shape-rendering',
  'stop-color',
  'stop-opacity',
  'stroke',
  'stroke-dasharray',
  'stroke-dashoffset',
  'stroke-linecap',
  'stroke-linejoin',
  'stroke-miterlimit',
  'stroke-opacity',
  'stroke-width',
  'text-anchor',
  'text-decoration',
  'text-rendering',
  'unicode-bidi',
  'vector-effect',
  'visibility',
  'word-spacing',
  'writing-mode',
])

/** A value that could load something, run something or break out of the attribute. */
const UNSAFE_VALUE = /url\(|expression|javascript:|@import|[\\"<>{}]/i

/**
 * Replaces an element's `style` attribute with the presentation attributes its safe
 * declarations name. A declaration outranks an attribute of the same name, as it did
 * in the style attribute; anything else in it is dropped.
 */
export function styleToPresentationAttributes(el: Element): void {
  const style = el.getAttribute('style')
  if (style === null) {
    return
  }
  el.removeAttribute('style')
  for (const declaration of style.split(';')) {
    const colon = declaration.indexOf(':')
    if (colon < 0) {
      continue
    }
    const name = declaration.slice(0, colon).trim().toLowerCase()
    const value = declaration
      .slice(colon + 1)
      .replace(/!\s*important\s*$/i, '')
      .trim()
    if (PRESENTATION_ATTRIBUTES.has(name) && value && !UNSAFE_VALUE.test(value)) {
      el.setAttribute(name, value)
    }
  }
}

/**
 * Parses flourish markup, requires an <svg> root, drops active-content elements, strips
 * on* event handlers and javascript:/data:/vbscript: values, and turns style attributes
 * into presentation attributes. Returns '' for anything that isn't clean SVG.
 */
export function sanitizeFlourishSvg(markup: string): string {
  if (!markup.includes('<svg')) {
    return ''
  }
  let doc: Document
  try {
    doc = new DOMParser().parseFromString(markup, 'image/svg+xml')
  } catch {
    return ''
  }
  if (doc.getElementsByTagName('parsererror').length > 0) {
    return ''
  }
  const root = doc.documentElement
  if (root.nodeName.toLowerCase() !== 'svg') {
    return ''
  }
  for (const el of [root, ...Array.from(root.querySelectorAll('*'))]) {
    if (el !== root && BANNED_TAGS.has(el.nodeName.toLowerCase())) {
      el.remove()
      continue
    }
    styleToPresentationAttributes(el)
    for (const attr of Array.from(el.attributes)) {
      const name = attr.name.toLowerCase()
      if (name.startsWith('on')) {
        el.removeAttribute(attr.name) // event handler
        continue
      }
      if (/(?:javascript|data|vbscript)\s*:/i.test(attr.value)) {
        el.removeAttribute(attr.name) // scripting / smuggled resource URL
      }
    }
  }
  return new XMLSerializer().serializeToString(root)
}
