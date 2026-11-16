/**
 * The Markdown editor's engine: Tiptap with its Markdown package, set up for the
 * Markdown this app stores. The same text is rendered on the site by markdown-it
 * (`lib/markdown.ts`, `breaks: true`) and posted to Discord, so it is a Discord-safe
 * subset: inline formatting (bold, italic, strikethrough, inline code), headings,
 * quotes, lists, dividers and links. Fenced code blocks still load and save, so text
 * that holds one keeps it.
 *
 * Where Tiptap's own Markdown output would change what a reader sees, the dialect
 * below writes it the way markdown-it and Discord both read it:
 *
 *   - A line break inside a paragraph is a bare newline, the form the stored text
 *     already uses, rather than two trailing spaces.
 *   - Text is never written as HTML entities. `&`, `<` and `>` stay as typed, so a
 *     Discord timestamp (`<t:1700000000:F>`), mention (`<@123>`) or custom emoji
 *     (`<:name:123>`) reaches Discord intact; a `<` is escaped only where it would
 *     open an HTML tag.
 *   - An underscore between two letters or digits is left alone (`:video_game:`,
 *     `snake_case`), since neither CommonMark nor Discord reads it as emphasis.
 *   - Text that would start a block (`# `, `> `, `- `, `1. `) at the start of a line is
 *     escaped, so it stays text.
 *   - A link whose text is its own address is written `<https://...>`.
 *   - An empty paragraph is left out: neither renderer shows one.
 *   - A table is kept as the text it was written as, and an image as its Markdown
 *     source, since the editor has neither.
 *
 * This module is loaded on demand by MarkdownEditor.vue, so Tiptap stays out of every
 * page that has no editor.
 */
import {
  Editor,
  Extension,
  mergeAttributes,
  Node,
  type Extensions,
  type JSONContent,
  type MarkdownParseResult,
} from '@tiptap/core'
import { HardBreak } from '@tiptap/extension-hard-break'
import { Link } from '@tiptap/extension-link'
import { Paragraph } from '@tiptap/extension-paragraph'
import { Placeholder } from '@tiptap/extensions'
import { Markdown } from '@tiptap/markdown'
import StarterKit from '@tiptap/starter-kit'
import { Marked, type marked } from 'marked'
import { cspNonce } from '@/lib/csp'

/** The flag the serializer puts on a link mark it writes as `<address>`. */
const AUTOLINK = 'markdownAutolink'

/** An address CommonMark accepts between angle brackets. */
const AUTOLINK_ADDRESS = /^[A-Za-z][A-Za-z0-9+.-]{1,31}:[^\s<>\p{Cc}]*$/u

/** A `<` that would open an HTML tag, comment or declaration. */
const TAG_OPENER = /^<(?:[A-Za-z][A-Za-z0-9-]*(?=[\s/>]|$)|\/[A-Za-z]|[!?])/

const WORD_BEFORE = /[\p{L}\p{N}]$/u
const WORD_AFTER = /^[\p{L}\p{N}]/u

/** Backslash-escapes what would otherwise be read as inline Markdown. */
function escapeInline(text: string): string {
  return text.replace(/[\\`*~[\]_<]/g, (ch: string, at: number) => {
    if (ch === '_') {
      const intraword =
        WORD_BEFORE.test(text.slice(Math.max(0, at - 2), at)) &&
        WORD_AFTER.test(text.slice(at + 1, at + 3))
      return intraword ? ch : '\\_'
    }
    if (ch === '<') {
      return TAG_OPENER.test(text.slice(at)) ? '\\<' : ch
    }
    return `\\${ch}`
  })
}

/**
 * Escapes a line's opening characters when they would start a block: a heading, a
 * quote, a list item, or a setext underline or thematic break. A `*` or `_` there is
 * already escaped by escapeInline.
 */
function escapeLineStart(line: string): string {
  return line
    .replace(/^( {0,3})(?=#{1,6}(?:[ \t]|$)|>|[-+](?:[ \t]|$)|[-=]+[ \t]*$)/, '$1\\')
    .replace(/^( {0,3}\d{1,9})([.)])(?=[ \t]|$)/, '$1\\$2')
}

/** Whether a text node begins a line: first in its block, or just after a line break. */
function startsLine(node: JSONContent, parent?: JSONContent): boolean {
  const siblings = parent?.content ?? []
  const at = siblings.indexOf(node)
  return at <= 0 || siblings[at - 1]?.type === 'hardBreak'
}

/** How a text node is written: replaces Tiptap's, which writes `&`, `<` and `>` as entities. */
function encodeText(text: string, node: JSONContent, parent?: JSONContent): string {
  const marks = node.marks ?? []
  const literal =
    parent?.type === 'codeBlock' ||
    marks.some((mark) => mark.type === 'code' || (mark.type === 'link' && mark.attrs?.[AUTOLINK]))
  if (literal) {
    return text
  }
  // Marked text opens with its delimiter, so only plain text can start a block.
  const first = marks.length === 0 && startsLine(node, parent)
  return escapeInline(text)
    .split('\n')
    .map((line, i) => (i > 0 || first ? escapeLineStart(line) : line))
    .join('\n')
}

/**
 * Tiptap's Markdown package with the dialect's text writer. The package writes every
 * text node through its manager's `encodeTextForMarkdown`, which it keeps private, so
 * the writer is put in its place on the manager each editor creates.
 */
const DialectMarkdown = Markdown.extend({
  onBeforeCreate(event) {
    this.parent?.(event)
    const manager = this.editor.markdown as unknown as
      { encodeTextForMarkdown: typeof encodeText } | undefined
    if (manager) {
      manager.encodeTextForMarkdown = encodeText
    }
  },
})

/** A line break inside a paragraph is written as a bare newline. */
const DialectHardBreak = HardBreak.extend({
  renderMarkdown: () => '\n',
})

/** A link whose text is its own address is written `<address>`. */
const DialectLink = Link.extend({
  renderMarkdown(node, helpers, context) {
    if (node.attrs?.[AUTOLINK]) {
      return `<${helpers.renderChildren(node)}>`
    }
    return this.parent?.(node, helpers, context) ?? ''
  },
})

/** A paragraph that holds only an image keeps it inline, where MarkdownImage lives. */
const DialectParagraph = Paragraph.extend({
  parseMarkdown(token, helpers): MarkdownParseResult {
    const tokens = token.tokens ?? []
    if (tokens.length === 1 && tokens[0]?.type === 'image') {
      return helpers.createNode('paragraph', undefined, helpers.parseInline(tokens))
    }
    return this.parent?.(token, helpers) ?? []
  },
})

/**
 * An image in the stored text. The editor has no images, so it shows the Markdown
 * that draws one and writes it back unchanged.
 */
const MarkdownImage = Node.create({
  name: 'markdownImage',
  group: 'inline',
  inline: true,
  atom: true,
  selectable: true,

  addAttributes() {
    return {
      source: {
        default: '',
        parseHTML: (element) => element.getAttribute('data-markdown') ?? '',
        renderHTML: (attributes) => ({ 'data-markdown': String(attributes.source) }),
      },
    }
  },

  parseHTML() {
    return [{ tag: 'span[data-markdown]' }]
  },

  renderHTML({ node, HTMLAttributes }) {
    return [
      'span',
      mergeAttributes(HTMLAttributes, { class: 'markdown-editor__source' }),
      String(node.attrs.source),
    ]
  },

  markdownTokenName: 'image',
  parseMarkdown: (token, helpers) =>
    helpers.createNode('markdownImage', { source: (token.raw ?? '').trim() }),
  renderMarkdown: (node) => String(node.attrs?.source ?? ''),
})

/**
 * A task-list item (`- [ ] task`) read as a plain list item that starts with its box
 * as text, since the editor has no task lists. It is written back escaped
 * (`- \[ \] task`), which every renderer shows the same way.
 */
const TaskMarkerAsText = Extension.create({
  name: 'taskMarkerAsText',
  // Read before ListItem, which takes every other item.
  priority: 1000,
  markdownTokenName: 'list_item',
  parseMarkdown: (token, helpers) => {
    if (!token.task) {
      return []
    }
    const [first, ...rest] = (token.tokens ?? []).filter((t) => t.type !== 'checkbox')
    const inline = first?.type === 'text' || first?.type === 'paragraph'
    const marker: JSONContent = { type: 'text', text: token.checked ? '[x] ' : '[ ] ' }
    return helpers.createNode('listItem', undefined, [
      helpers.createNode('paragraph', undefined, [
        marker,
        ...(inline ? helpers.parseInline(first.tokens ?? []) : []),
      ]),
      ...helpers.parseChildren(inline || !first ? rest : [first, ...rest]),
    ])
  },
})

/**
 * The parser: GitHub-flavored, with a single newline read as a line break. Tables are
 * not read as tables, so one keeps its lines as text. Each editor gets its own,
 * because Tiptap registers its extensions' tokenizers on the instance it is given.
 */
function createParser(): typeof marked {
  const parser = new Marked({
    gfm: true,
    breaks: true,
    tokenizer: { table: () => undefined },
  })
  // Tiptap reads only what an instance and the default export share: the lexer, the
  // options and `use`.
  return parser as unknown as typeof marked
}

/** The editor's extensions; `placeholder` is shown while it is empty. */
export function markdownExtensions(placeholder = ''): Extensions {
  return [
    StarterKit.configure({
      hardBreak: false,
      link: false,
      paragraph: false,
      underline: false,
    }),
    DialectParagraph,
    DialectHardBreak,
    DialectLink.configure({ openOnClick: false, defaultProtocol: 'https' }),
    MarkdownImage,
    TaskMarkerAsText,
    Placeholder.configure({ placeholder }),
    DialectMarkdown.configure({ marked: createParser() }),
  ]
}

/** A paragraph with nothing in it but line breaks. */
function isBlankParagraph(node: JSONContent): boolean {
  return node.type === 'paragraph' && (node.content ?? []).every((n) => n.type === 'hardBreak')
}

type MarkJSON = NonNullable<JSONContent['marks']>[number]

/** The link mark on a text node that carries no other mark. */
function soleLink(node: JSONContent | undefined): MarkJSON | undefined {
  const marks = node?.type === 'text' ? (node.marks ?? []) : []
  return marks.length === 1 && marks[0]?.type === 'link' ? marks[0] : undefined
}

function linksTo(node: JSONContent | undefined, href: unknown): boolean {
  return (node?.marks ?? []).some((mark) => mark.type === 'link' && mark.attrs?.href === href)
}

/** Flags each link that can be written `<address>`: its whole text is its address. */
function flagAutolinks(nodes: JSONContent[]): JSONContent[] {
  return nodes.map((node, i) => {
    const link = soleLink(node)
    const href: unknown = link?.attrs?.href
    const text = node.text ?? ''
    const own =
      typeof href === 'string' &&
      !link?.attrs?.title &&
      (text === href || `mailto:${text}` === href) &&
      AUTOLINK_ADDRESS.test(href) &&
      !linksTo(nodes[i - 1], href) &&
      !linksTo(nodes[i + 1], href)
    if (!link || !own) {
      return node
    }
    return { ...node, marks: [{ ...link, attrs: { ...link.attrs, [AUTOLINK]: true } }] }
  })
}

/** A block's text without line breaks at either end, which no renderer shows. */
function trimBreaks(content: JSONContent[]): JSONContent[] {
  let start = 0
  let end = content.length
  while (start < end && content[start]?.type === 'hardBreak') {
    start++
  }
  while (end > start && content[end - 1]?.type === 'hardBreak') {
    end--
  }
  return content.slice(start, end)
}

/**
 * The document as the serializer should see it: no empty paragraphs, no line breaks
 * at the ends of a block, and autolinks flagged.
 */
function prepare(node: JSONContent): JSONContent {
  if (!node.content) {
    return node
  }
  if (node.type === 'paragraph' || node.type === 'heading') {
    return { ...node, content: flagAutolinks(trimBreaks(node.content)) }
  }
  // Empty paragraphs are left out of the document and of quotes, though a quote keeps
  // one, empty or not. A list item keeps its own: it must start with a paragraph.
  const kept =
    node.type === 'doc' || node.type === 'blockquote'
      ? node.content.filter((child) => !isBlankParagraph(child))
      : node.content
  const content = kept.length || node.type === 'doc' ? kept : node.content.slice(0, 1)
  return { ...node, content: content.map(prepare) }
}

/** The editor's document as the Markdown the app stores. */
export function toMarkdown(editor: Editor): string {
  return editor.markdown?.serialize(prepare(editor.getJSON())) ?? ''
}

/** Replaces the editor's document with `markdown`, without reporting it as an edit. */
export function setMarkdown(editor: Editor, markdown: string): void {
  editor.commands.setContent(markdown, { contentType: 'markdown', emitUpdate: false })
}

export interface MarkdownEditorOptions {
  /** The element the editable surface is placed in. */
  element: HTMLElement
  /** The Markdown the editor starts with. */
  content: string
  /** Shown while the editor is empty, and its accessible name. */
  placeholder: string
  /** Called after each change to the document. */
  onUpdate: () => void
  /** Called after every transaction, so a toolbar can follow the selection. */
  onTransaction: () => void
  onBlur: () => void
}

export function createMarkdownEditor(options: MarkdownEditorOptions): Editor {
  return new Editor({
    element: options.element,
    extensions: markdownExtensions(options.placeholder),
    content: options.content,
    contentType: 'markdown',
    // Tiptap adds a <style> of its base rules; the page's policy admits it by nonce.
    injectNonce: cspNonce || undefined,
    editorProps: {
      attributes: {
        class: 'markdown-editor__surface markdown-text',
        role: 'textbox',
        'aria-multiline': 'true',
        'aria-label': options.placeholder || 'Formatted text',
      },
    },
    onUpdate: () => options.onUpdate(),
    onTransaction: () => options.onTransaction(),
    onBlur: () => options.onBlur(),
  })
}
