import { describe, it, expect } from 'vitest'
import { sanitizeFlourishSvg } from '@/lib/flourish'

const SVG = 'xmlns="http://www.w3.org/2000/svg"'

describe('sanitizeFlourishSvg', () => {
  it('turns safe style declarations into presentation attributes', () => {
    const out = sanitizeFlourishSvg(
      `<svg ${SVG}><path d="M0 0" fill="red" style="fill: #fff; stroke-width: 2px !important; opacity:.5"/></svg>`,
    )
    expect(out).not.toContain('style=')
    expect(out).toContain('fill="#fff"')
    expect(out).toContain('stroke-width="2px"')
    expect(out).toContain('opacity=".5"')
  })

  it('drops declarations that are no presentation attribute or could load something', () => {
    const out = sanitizeFlourishSvg(
      `<svg ${SVG}><path d="M0 0" style="fill:url(https://evil.test/a.svg#x);position:fixed;stroke:blue"/></svg>`,
    )
    expect(out).not.toContain('style=')
    expect(out).not.toContain('evil.test')
    expect(out).not.toContain('position')
    expect(out).toContain('stroke="blue"')
  })

  it('still drops scripting elements and event handlers', () => {
    const out = sanitizeFlourishSvg(
      `<svg ${SVG} onload="alert(1)"><script>alert(1)</script><style>.a{fill:red}</style><path d="M0 0"/></svg>`,
    )
    expect(out).not.toContain('onload')
    expect(out).not.toContain('<script')
    expect(out).not.toContain('<style')
    expect(out).toContain('<path')
  })

  it('refuses markup without an svg root', () => {
    expect(sanitizeFlourishSvg('<div>hi</div>')).toBe('')
    expect(sanitizeFlourishSvg('<svg')).toBe('')
  })
})
