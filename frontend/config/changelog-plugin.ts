import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import type { Plugin } from 'vite'

// Release notes live in one CHANGELOG.md per versioned part, outside the frontend
// project: the app's at the repo root (its whole line, the SPA's history before 4.0.0
// included), the plugin's in plugins/ and the backend's closed pre-4.0.0 history in
// backend/. This plugin reads and parses them at build/dev time and exposes them as the
// virtual module `virtual:changelog` (declared in env.d.ts), so the admin dashboard can
// render each part's changelog without a backend round-trip or a committed generated
// file. Shared between the build and the specs, both on vite.config.ts.
const CHANGELOG_PATHS = {
  app: fileURLToPath(new URL('../../CHANGELOG.md', import.meta.url)),
  plugin: fileURLToPath(new URL('../../plugins/CHANGELOG.md', import.meta.url)),
  backend: fileURLToPath(new URL('../../backend/CHANGELOG.md', import.meta.url)),
}

// A structured view of a changelog so the UI can render it with proper typography +
// navigation instead of dumping raw markdown. Each file becomes an ordered list of
// version entries; each entry has an intro paragraph and labelled change-groups
// (Added / Fixed / ...) whose `body` is the markdown of that group's bullet list
// (rendered with markdown-it in the modal).
interface ChangelogGroup {
  label: string
  body: string
}
interface ChangelogEntry {
  version: string
  date: string
  intro: string
  groups: ChangelogGroup[]
}
interface ChangelogSection {
  latest: string
  entries: ChangelogEntry[]
}
type Changelog = Record<keyof typeof CHANGELOG_PATHS, ChangelogSection>

/**
 * Parses one Keep a Changelog file into its released versions, newest first. A
 * `## [x.y.z] - date` heading starts a version and a `### Added` / `### Fixed` / ...
 * heading a change-group within it. The header before the first version, the
 * `## [Unreleased]` section and the link references at the foot of the file
 * (`[x.y.z]: https://...`) are left out.
 */
export function parseChangelogFile(md: string): ChangelogSection {
  const entries: ChangelogEntry[] = []
  let cur: { version: string; date: string; intro: string[]; groups: ChangelogGroup[] } | null =
    null
  let group: ChangelogGroup | null = null

  const flush = () => {
    if (!cur) {
      return
    }
    entries.push({
      version: cur.version,
      date: cur.date,
      intro: cur.intro.join('\n').trim(),
      groups: cur.groups
        .map((g) => ({ label: g.label, body: g.body.trim() }))
        .filter((g) => g.body),
    })
    cur = null
  }

  for (const line of md.split(/\r?\n/)) {
    // `## [version] - date` starts a version entry (date optional); any other level-2
    // heading, `## [Unreleased]` among them, ends the current one and starts nothing.
    if (/^##\s/.test(line)) {
      flush()
      group = null
      const v = /^##\s+\[([^\]]+)\]\s*(?:[-–—]\s*(.+?))?\s*$/.exec(line)
      const version = v?.[1]?.trim() ?? ''
      if (v && version.toLowerCase() !== 'unreleased') {
        // .at() is typed `string | undefined` (the date group is optional), unlike
        // v[2] which the RegExpExecArray type widens to `string`.
        cur = { version, date: (v.at(2) ?? '').trim(), intro: [], groups: [] }
      }
      continue
    }
    if (!cur) {
      continue
    }
    // A link reference (`[5.4.0]: https://...`) belongs to the foot, not to an entry.
    if (/^\[[^\]]+\]:\s*\S/.test(line)) {
      continue
    }
    // `### Added` / `### Fixed` / ... starts a change-group within the entry.
    const g = /^###\s+(.+?)\s*$/.exec(line)
    if (g) {
      group = { label: g[1].trim(), body: '' }
      cur.groups.push(group)
      continue
    }
    if (group) {
      group.body += line + '\n'
    } else {
      cur.intro.push(line)
    }
  }
  flush()
  return { latest: entries[0]?.version ?? '', entries }
}

/** Vite plugin exposing the parsed changelogs as `import { changelog } from 'virtual:changelog'`. */
export function changelogPlugin(): Plugin {
  const virtualId = 'virtual:changelog'
  const resolvedId = '\0' + virtualId
  return {
    name: 'app-changelog',
    resolveId(id) {
      return id === virtualId ? resolvedId : undefined
    },
    load(id) {
      if (id !== resolvedId) {
        return undefined
      }
      const changelog = {} as Changelog
      for (const [part, path] of Object.entries(CHANGELOG_PATHS) as [keyof Changelog, string][]) {
        // Re-read on every load and register each file so `vite dev` picks up edits.
        this.addWatchFile(path)
        let md = ''
        try {
          md = readFileSync(path, 'utf-8')
        } catch {
          md = ''
        }
        changelog[part] = parseChangelogFile(md)
      }
      return `export const changelog = ${JSON.stringify(changelog)}\n`
    },
  }
}
