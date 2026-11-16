import axe from 'axe-core'

/**
 * What axe finds wrong in an element at WCAG 2.2 AA with its best practices, one line per
 * offending node. jsdom lays nothing out, so contrast is left to the end-to-end suite, and
 * `region` to the page, since a component alone sits in no landmark. `rules` turns off a further
 * rule for one call, where the spec says why.
 *
 * A component mounted without `attachTo` sits outside the document, where axe can't follow an
 * `id` reference, so its tree is put in the body for the run and taken out again after.
 */
export async function axeFindings(
  element: Element,
  rules: Record<string, { enabled: boolean }> = {},
): Promise<string[]> {
  // A component with several roots hands over its first node, which may be text or a comment.
  if (element.nodeType !== Node.ELEMENT_NODE) {
    throw new Error(`axeFindings needs an element, not ${element.nodeName}`)
  }
  let detached: Node | null = null
  if (!element.isConnected) {
    detached = element
    while (detached.parentNode) {
      detached = detached.parentNode
    }
    document.body.appendChild(detached)
  }
  try {
    const results = await axe.run(
      { include: [element] },
      {
        runOnly: ['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa', 'wcag22aa', 'best-practice'],
        rules: { 'color-contrast': { enabled: false }, region: { enabled: false }, ...rules },
      },
    )
    return results.violations.flatMap((violation) =>
      violation.nodes.map(
        (node) => `${violation.id}: ${node.target.join(' ')} - ${violation.help}`,
      ),
    )
  } finally {
    if (detached) {
      document.body.removeChild(detached)
    }
  }
}
