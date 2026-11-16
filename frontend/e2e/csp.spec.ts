/// <reference lib="dom" />
import type { Page } from '@playwright/test'
import { STAFF } from './env.ts'
import { expect, test, title } from './fixtures.ts'

/**
 * The Content-Security-Policy, on the built app as the server sends it: each answer
 * for a page names a fresh nonce, allows nothing inline without it and names hosts,
 * never a whole scheme; and loading the pages, public and admin, trips none of it -
 * the styles and scripts the app adds at run time carry the page's nonce.
 */

/** The violations a page has met so far: its own reports, then the console's. */
async function violations(page: Page, logged: string[]): Promise<string[]> {
  const reported = await page.evaluate(
    () => (window as unknown as { cspViolations?: string[] }).cspViolations ?? [],
  )
  return [...reported, ...logged]
}

/**
 * Has the page record every policy violation from its first script on, and returns
 * the list the console's reports of one go into.
 */
async function watch(page: Page): Promise<string[]> {
  await page.addInitScript(() => {
    const seen: string[] = []
    ;(window as unknown as { cspViolations: string[] }).cspViolations = seen
    document.addEventListener('securitypolicyviolation', (e) => {
      seen.push(
        `${e.effectiveDirective} refused ${e.blockedURI || 'inline'} (${e.sourceFile}:${String(e.lineNumber)})`,
      )
    })
  })
  const logged: string[] = []
  page.on('console', (message) => {
    if (/Content[ -]Security[ -]Policy/i.test(message.text())) {
      logged.push(message.text())
    }
  })
  return logged
}

/**
 * Waits for what the app adds to the page at run time once its settings arrive - the
 * uploaded fonts' <style> and the heading font's stylesheet, which the site serves -
 * and checks the <style> carries the nonce the page was served with.
 */
async function settled(page: Page): Promise<void> {
  const style = page.locator('style#uploaded-fonts')
  await style.waitFor({ state: 'attached' })
  const [meta, own] = await Promise.all([
    page.locator('meta[property="csp-nonce"]').evaluate((el) => (el as HTMLMetaElement).nonce),
    style.evaluate((el) => (el as HTMLStyleElement).nonce),
  ])
  expect(meta).not.toBe('')
  expect(own).toBe(meta)
  await page.evaluate(() => document.fonts.ready)
}

test('every page answer names a fresh nonce and allows nothing inline', async ({ request }) => {
  const nonces = new Set<string>()
  for (const path of ['/', '/index.html', '/raffles', '/admin/login']) {
    const answer = await request.get(path)
    expect(answer.status()).toBe(200)
    const policy = answer.headers()['content-security-policy'] ?? ''
    expect(policy, path).toContain("default-src 'self'")
    expect(policy, path).toContain("frame-ancestors 'none'")
    expect(policy, path).not.toContain("'unsafe-inline'")
    expect(policy, path).not.toContain("'unsafe-eval'")
    expect(policy.split(/[;\s]+/), path).not.toContain('https:')
    expect(policy, path).not.toContain('*')
    expect(policy, path).toContain("font-src 'self';")
    expect(policy, path).not.toMatch(/fonts\.(googleapis|gstatic)\.com/)

    const nonce = /script-src 'self' 'nonce-([^']+)'/.exec(policy)?.[1] ?? ''
    expect(nonce, path).not.toBe('')
    expect(policy, path).toContain(`style-src 'self' 'nonce-${nonce}'`)
    expect(nonces.has(nonce), `${path} reused a nonce`).toBe(false)
    nonces.add(nonce)
    const page = await answer.text()
    expect(page, path).toContain(`<meta property="csp-nonce" nonce="${nonce}"`)
    expect(page, path).not.toContain('__CSP_NONCE__')
    expect(answer.headers().etag, path).toBeUndefined()
    expect(answer.headers()['referrer-policy'], path).toBe('same-origin')
    expect(answer.headers()['x-content-type-options'], path).toBe('nosniff')
  }
})

test("a festival map's embed may be framed by the allowed sites alone", async ({ request }) => {
  const embed = await request.get('/embed/festival-maps/1')
  expect(embed.status()).toBe(200)
  const policy = embed.headers()['content-security-policy'] ?? ''
  expect(policy).toContain("frame-ancestors 'self' https://senpan.cafe https://*.senpan.cafe;")
  expect(embed.headers()['x-frame-options']).toBeUndefined()

  const map = await request.get('/festival-maps/1')
  expect(map.headers()['content-security-policy']).toContain("frame-ancestors 'none'")
  expect(map.headers()['x-frame-options']).toBe('DENY')
})

const publicPages: { path: string; heading: string }[] = [
  { path: '/', heading: 'Senpan App Suite' },
  { path: '/card-requests', heading: 'Personal Card Request' },
  { path: '/festival-maps', heading: 'Festival Maps' },
  { path: '/stamp-rallies', heading: 'Stamp Rallies' },
  { path: '/admin/login', heading: 'Sign In' },
]

for (const { path, heading } of publicPages) {
  test(`${path} loads without a policy violation`, async ({ page }) => {
    const logged = await watch(page)
    await page.goto(path)
    await expect(page.getByRole('heading', { name: title(heading) })).toBeVisible()
    await settled(page)
    expect(await violations(page, logged)).toEqual([])
  })
}

test('a raffle loads without a policy violation', async ({ page, raffle }) => {
  const logged = await watch(page)
  await page.goto(`/raffles/${String(raffle.id)}`)
  await expect(page.getByRole('heading', { name: title(raffle.title) })).toBeVisible()
  await expect(page.getByText(raffle.description)).toBeVisible()
  await settled(page)
  expect(await violations(page, logged)).toEqual([])
})

test('a bingo board loads without a policy violation', async ({ page, bingo }) => {
  const logged = await watch(page)
  await page.goto(`/play/${bingo.cardId}`)
  await expect(page.getByRole('heading', { name: title(`Board ${bingo.cardId}`) })).toBeVisible()
  await expect(page.getByRole('status', { name: 'Connection status: Live' })).toBeVisible()
  await settled(page)
  expect(await violations(page, logged)).toEqual([])
})

test('the admin pages load without a policy violation', async ({ page }) => {
  const logged = await watch(page)
  // Signed in through the page's own context, so the pages below open signed in.
  expect((await page.request.post('/api/v1/auth/login', { data: STAFF })).ok()).toBe(true)
  for (const path of ['/admin/bingo/game', '/admin/system/settings', '/admin/atelier/fonts']) {
    await page.goto(path)
    await expect(page).toHaveURL(new RegExp(`${path}$`))
    await expect(page.getByRole('heading', { level: 2 }).first()).toBeVisible()
    await settled(page)
    expect(await violations(page, logged), path).toEqual([])
  }
})
