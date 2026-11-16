import { describe, it, expect, vi, afterEach } from 'vitest'
// Exercises the real CHANGELOG.md files (the root's, plugins/ and backend/) through the
// `virtual:changelog` build plugin (config/changelog-plugin.ts), registered in vite.config.ts.
import {
  changelog,
  PLUGIN_REPO_URL,
  PLUGIN_INSTALL_STEPS,
  CHANGELOG_LABELS,
  PLUGIN_MASTER_PATH,
  fetchLivePluginVersion,
} from '@/lib/changelog'
import { APP_VERSION } from '@/lib/version'

describe('changelog pipeline', () => {
  it('parses each CHANGELOG.md into structured entries', () => {
    expect(Object.keys(changelog).sort()).toEqual(['app', 'backend', 'plugin'])
    for (const key of ['app', 'backend', 'plugin'] as const) {
      const section = changelog[key]
      expect(section.entries.length, `${key} entries`).toBeGreaterThan(0)
      // `latest` mirrors the newest entry's version.
      expect(section.latest).toBe(section.entries[0]!.version)
      // Every entry has a version and at least a date or some groups.
      for (const e of section.entries) {
        expect(e.version, `${key} entry version`).toBeTruthy()
      }
    }
  })

  it('parses labelled change-groups with bullet bodies', () => {
    expect(changelog.app.entries[0]!.groups.length).toBeGreaterThan(0)
    // Find any app entry with an "Added" group rather than assuming the newest one has
    // it (newer entries may be Changed/Fixed only) - the point is that the parser
    // produces labelled groups whose bodies are markdown bullet lists.
    const added = changelog.app.entries
      .flatMap((e) => e.groups)
      .find((g) => g.label.toLowerCase() === 'added')
    expect(added, 'an app entry should have an Added group').toBeTruthy()
    expect(added?.body).toContain('- ') // its body is a markdown bullet list
  })

  it("parses each part's version + date formats", () => {
    expect(changelog.app.latest).toMatch(/^\d+\.\d+\.\d+$/)
    expect(changelog.backend.latest).toMatch(/^\d+\.\d+\.\d+$/)
    // The Dalamud plugin uses a four-part AssemblyVersion.
    expect(changelog.plugin.latest).toMatch(/^\d+\.\d+\.\d+\.\d+$/)
    // Dates are parsed off the heading (`## [x] - YYYY-MM-DD`).
    for (const key of ['app', 'backend', 'plugin'] as const) {
      expect(changelog[key].entries[0]!.date, `${key} date`).toMatch(/^\d{4}-\d{2}-\d{2}$/)
    }
  })

  it('leaves out Unreleased and the link references at the foot', () => {
    for (const key of ['app', 'backend', 'plugin'] as const) {
      const { entries } = changelog[key]
      expect(entries.map((e) => e.version.toLowerCase())).not.toContain('unreleased')
      // The oldest entry is last in the file, just above the links.
      const last = entries.at(-1)!
      const text = [last.intro, ...last.groups.map((g) => g.body)].join('\n')
      expect(text, `${key} oldest entry`).not.toMatch(/^\[[^\]]+\]:/m)
    }
  })

  it('keeps the files separate (no cross-bleed)', () => {
    expect(changelog.app.latest).not.toBe(changelog.plugin.latest)
    expect(changelog.backend.entries.map((e) => e.version)).not.toContain(changelog.app.latest)
  })

  it("has an entry for this build's version, newest in the app's changelog", () => {
    // A release changes the version and its CHANGELOG.md entry together.
    expect(changelog.app.latest).toBe(APP_VERSION)
  })

  it("continues the app's history with the SPA's own from before 4.0.0", () => {
    const versions = changelog.app.entries.map((e) => e.version)
    const at = versions.indexOf('4.0.0')
    expect(at).toBeGreaterThan(-1)
    expect(versions[at + 1]).toBe('3.32.0')
    expect(versions.at(-1)).toBe('1.0.0')
  })

  it('provides Dalamud install details for the plugin', () => {
    expect(PLUGIN_REPO_URL).toBe('https://apps.senpan.cafe/plugin/pluginmaster.json')
    expect(PLUGIN_INSTALL_STEPS.length).toBeGreaterThan(0)
    expect(PLUGIN_INSTALL_STEPS.some((s) => s.detail.includes('/senpan'))).toBe(true)
    expect(CHANGELOG_LABELS.plugin).toBe('Plugin')
    expect(Object.keys(CHANGELOG_LABELS).sort()).toEqual(['app', 'backend', 'plugin'])
  })
})

describe('fetchLivePluginVersion', () => {
  afterEach(() => vi.unstubAllGlobals())

  function stubFetch(impl: () => Promise<unknown> | never) {
    vi.stubGlobal('fetch', vi.fn(impl))
  }

  it('returns the AssemblyVersion of the first pluginmaster entry', async () => {
    stubFetch(async () => ({ ok: true, json: async () => [{ AssemblyVersion: '2.3.4.0' }] }))
    expect(await fetchLivePluginVersion()).toBe('2.3.4.0')
  })

  it('fetches the same-origin repo-index path with no-store', async () => {
    const spy = vi.fn(async () => ({
      ok: true,
      json: async () => [{ AssemblyVersion: '1.0.0.0' }],
    }))
    vi.stubGlobal('fetch', spy)
    await fetchLivePluginVersion()
    expect(spy).toHaveBeenCalledWith(PLUGIN_MASTER_PATH, { cache: 'no-store' })
  })

  it('returns null on a non-ok response', async () => {
    stubFetch(async () => ({ ok: false, json: async () => [] }))
    expect(await fetchLivePluginVersion()).toBeNull()
  })

  it('returns null when the payload is not the expected array shape', async () => {
    stubFetch(async () => ({ ok: true, json: async () => ({}) }))
    expect(await fetchLivePluginVersion()).toBeNull()
  })

  it('returns null when the first entry has no AssemblyVersion', async () => {
    stubFetch(async () => ({ ok: true, json: async () => [{}] }))
    expect(await fetchLivePluginVersion()).toBeNull()
  })

  it('returns null (never throws) when fetch rejects', async () => {
    stubFetch(() => {
      throw new Error('network down')
    })
    expect(await fetchLivePluginVersion()).toBeNull()
  })
})
