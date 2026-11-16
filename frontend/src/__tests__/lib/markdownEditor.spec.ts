import { afterEach, describe, expect, it, vi } from 'vitest'
import { Editor, type JSONContent } from '@tiptap/core'
import {
  createMarkdownEditor,
  markdownExtensions,
  setMarkdown,
  toMarkdown,
} from '@/lib/markdownEditor'

const editors: Editor[] = []

afterEach(() => {
  editors.splice(0).forEach((editor) => editor.destroy())
})

function load(content: string | JSONContent): Editor {
  const editor = new Editor({
    extensions: markdownExtensions(),
    content,
    ...(typeof content === 'string' ? { contentType: 'markdown' as const } : {}),
  })
  editors.push(editor)
  return editor
}

/** The stored text after the editor has loaded it and written it back. */
function roundTrip(markdown: string): string {
  return toMarkdown(load(markdown))
}

/** A document of one paragraph holding `text` as typed. */
function typed(text: string): JSONContent {
  return { type: 'doc', content: [{ type: 'paragraph', content: [{ type: 'text', text }] }] }
}

/** Text the editor writes back exactly as stored. */
const unchanged: [string, string][] = [
  ['line breaks inside a paragraph', 'Line one\nLine two\nLine three'],
  ['a paragraph break', 'First paragraph\n\nSecond paragraph'],
  [
    'Discord timestamps, mentions, channels and custom emoji',
    'Starts <t:1700000000:F> (<t:1700000000:R>) with <@123456> in <#789> <:pepe_hands:42> <a:dance:43>',
  ],
  ['spoilers and subtext', '||the prize|| is a surprise\n-# small print'],
  ['emoji shortcodes and snake_case', ':video_game: :white_check_mark: snake_case_name'],
  ['emoji characters', 'Party 🎉 tonight ✨ at 8'],
  ['ampersands and comparisons', 'Tea & cakes, 1 < 2 > 0'],
  ['every heading level', '# One\n\n## Two\n\n### Three\n\n#### Four\n\n##### Five\n\n###### Six'],
  ['bold, italic, strikethrough and code', '**bold** *italic* ~~struck~~ `code` ***both***'],
  ['intraword emphasis', '*a*b and a*b*'],
  ['a line break inside bold', '**first**\n**second**'],
  ['a bulleted list', '- one\n- two\n- three'],
  ['a numbered list', '1. one\n2. two'],
  ['a numbered list that starts later', '3. three\n4. four'],
  ['a nested list', '- outer\n  - inner\n- next'],
  ['a quote over two lines', '> first line\n> second line'],
  ['a quote of two paragraphs', '> one\n>\n> two'],
  ['a masked link', 'Read [the rules](https://example.com/rules) first'],
  ['an address as its own link', 'Visit <https://example.com/~shop_1> today'],
  ['a link whose text is another address', '[https://example.com](https://example.org)'],
  ['a divider', 'Above\n\n---\n\nBelow'],
  ['a fenced code block', '```js\nconst a = 1 < 2 && b_c\n```'],
  ['a table, kept as text', '| Prize | Odds |\n| --- | --- |\n| Mount | 1% |'],
  ['an inline image', 'See ![the map](https://example.com/map.png) here'],
  ['an image on its own line', 'Above\n\n![banner](https://example.com/banner.png)'],
  [
    'escaped block starts',
    '\\# not a heading\n\n1\\. not a list\n\n\\- not a bullet\n\n\\> not a quote',
  ],
  ['a backslash', 'C:\\\\Games and a \\* star'],
  ['an empty text', ''],
]

/** Text the editor rewrites in another form that renders the same. */
const normalized: [string, string, string][] = [
  ['a hard break written with two spaces', 'a  \nb', 'a\nb'],
  ['a hard break written with a backslash', 'a\\\nb', 'a\nb'],
  ['a hard break written as <br>', 'a<br>b\nc<br />d', 'a\nb\nc\nd'],
  ['an empty paragraph written as <br />', 'a\n\n<br />\n\nb', 'a\n\nb'],
  ['extra blank lines', 'a\n\n\n\nb', 'a\n\nb'],
  ['a line break at the end of a paragraph', 'Text<br />\n\nMore', 'Text\n\nMore'],
  ['star bullets', '* one\n* two', '- one\n- two'],
  ['a loose list', '- one\n\n- two', '- one\n- two'],
  ['a list straight after a paragraph', 'Prizes:\n- gil\n- a mount', 'Prizes:\n\n- gil\n- a mount'],
  ['parenthesis numbering', '1) one\n2) two', '1. one\n2. two'],
  ['underscore emphasis', '_italic_ and __bold__', '*italic* and **bold**'],
  ['a bare address', 'Go to https://example.com now', 'Go to <https://example.com> now'],
  ['an escaped timestamp', 'At \\<t:1700000000:F\\>', 'At <t:1700000000:F>'],
  ['an escaped shortcode', 'Win :video\\_game: prizes', 'Win :video_game: prizes'],
  ['an HTML entity', 'Tea &amp; cakes', 'Tea & cakes'],
  ['a task list', '- [ ] to do\n- [x] done', '- \\[ \\] to do\n- \\[x\\] done'],
  ['bold written as HTML', '<b>bold</b> text', '**bold** text'],
  ['bold around a link', '**[shop](https://example.com)**', '[**shop**](https://example.com)'],
]

describe('markdownEditor round trip', () => {
  it.each(unchanged)('keeps %s as stored', (_label, markdown) => {
    expect(roundTrip(markdown)).toBe(markdown)
  })

  it.each(normalized)('rewrites %s', (_label, markdown, written) => {
    expect(roundTrip(markdown)).toBe(written)
  })

  const stored: [string, string][] = [
    ...unchanged,
    ...normalized.map(([label, markdown]): [string, string] => [label, markdown]),
  ]

  it.each(stored)('writes %s the same way twice', (_label, markdown) => {
    const once = roundTrip(markdown)
    expect(roundTrip(once)).toBe(once)
  })

  it('reads a single newline as a line break, not a space', () => {
    const paragraph = load('Line one\nLine two').getJSON().content[0]
    expect(paragraph?.content?.map((node) => node.type)).toEqual(['text', 'hardBreak', 'text'])
  })

  it('shows an image it cannot draw as its Markdown', () => {
    const paragraph = load('![map](https://example.com/map.png)').getJSON().content[0]
    expect(paragraph?.type).toBe('paragraph')
    expect(paragraph?.content?.[0]).toMatchObject({
      type: 'markdownImage',
      attrs: { source: '![map](https://example.com/map.png)' },
    })
  })
})

describe('markdownEditor escaping of typed text', () => {
  it.each([
    ['a heading marker', '# not a heading', '\\# not a heading'],
    ['a list number', '1. not a list', '1\\. not a list'],
    ['a bullet', '- not a bullet', '\\- not a bullet'],
    ['a quote marker', '> not a quote', '\\> not a quote'],
    ['a divider', '---', '\\---'],
    ['emphasis characters', 'a * b _c_ ~d~ `e`', 'a \\* b \\_c\\_ \\~d\\~ \\`e\\`'],
    ['an HTML tag', 'type <b> for bold', 'type \\<b> for bold'],
    [
      'Discord syntax',
      '<t:1700000000:F> <@1> <:x_y:2> & ||z||',
      '<t:1700000000:F> <@1> <:x_y:2> & ||z||',
    ],
    ['a marker mid-line', 'Prize # 1 - a > b', 'Prize # 1 - a > b'],
  ])('escapes %s only where it would change the meaning', (_label, text, written) => {
    const markdown = toMarkdown(load(typed(text)))
    expect(markdown).toBe(written)
    expect(load(markdown).getJSON()).toEqual(typed(text))
  })

  it('escapes block markers at the start of a line after a line break', () => {
    const doc: JSONContent = {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          content: [
            { type: 'text', text: 'Rules' },
            { type: 'hardBreak' },
            { type: 'text', text: '# one' },
            { type: 'hardBreak' },
            { type: 'text', text: '2. two' },
          ],
        },
      ],
    }
    const markdown = toMarkdown(load(doc))
    expect(markdown).toBe('Rules\n\\# one\n2\\. two')
    expect(load(markdown).getJSON()).toEqual(doc)
  })

  it('writes a link whose text is its address as <address>', () => {
    const doc: JSONContent = {
      type: 'doc',
      content: [
        {
          type: 'paragraph',
          content: [
            {
              type: 'text',
              text: 'https://example.com/a_b~c',
              marks: [{ type: 'link', attrs: { href: 'https://example.com/a_b~c' } }],
            },
          ],
        },
      ],
    }
    expect(toMarkdown(load(doc))).toBe('<https://example.com/a_b~c>')
  })
})

describe('createMarkdownEditor', () => {
  it('reports an edit but not a value set from outside', () => {
    const onUpdate = vi.fn()
    const editor = createMarkdownEditor({
      element: document.createElement('div'),
      content: 'Hello',
      placeholder: 'Say something',
      onUpdate,
      onTransaction: () => undefined,
      onBlur: () => undefined,
    })
    editors.push(editor)

    setMarkdown(editor, '# Replaced')
    expect(onUpdate).not.toHaveBeenCalled()
    expect(toMarkdown(editor)).toBe('# Replaced')

    // The end of the heading's text (the editor keeps an empty paragraph after it).
    const end = (editor.state.doc.firstChild?.nodeSize ?? 2) - 1
    editor.commands.insertContentAt(end, { type: 'text', text: ' text' })
    expect(onUpdate).toHaveBeenCalledTimes(1)
    expect(toMarkdown(editor)).toBe('# Replaced text')
  })

  it('names the editable text for assistive technology', () => {
    const element = document.createElement('div')
    const editor = createMarkdownEditor({
      element,
      content: '',
      placeholder: 'Describe the raffle',
      onUpdate: () => undefined,
      onTransaction: () => undefined,
      onBlur: () => undefined,
    })
    editors.push(editor)

    const surface = element.querySelector('.markdown-editor__surface')
    expect(surface?.getAttribute('role')).toBe('textbox')
    expect(surface?.getAttribute('aria-multiline')).toBe('true')
    expect(surface?.getAttribute('aria-label')).toBe('Describe the raffle')
    expect(toMarkdown(editor)).toBe('')
  })
})
