/**
 * Markdown rendering with HTML sanitization.
 *
 * The legacy app rendered admin-authored markdown (raffle descriptions/rules,
 * game details) straight into `v-html` via an unpinned CDN `marked` — meaning
 * any HTML in that content reached every viewer's DOM unsanitized. Here `marked`
 * is a pinned dependency and its output is run through DOMPurify before display,
 * closing that stored-XSS hardening gap from the audit.
 */
import { marked } from 'marked'
import DOMPurify from 'dompurify'

marked.setOptions({ breaks: true })

/** Render markdown to sanitized HTML safe for `v-html`. */
export function renderMarkdown(text: string | null | undefined): string {
  if (!text) return ''
  const rawHtml = marked.parse(text, { async: false }) as string
  return DOMPurify.sanitize(rawHtml)
}
