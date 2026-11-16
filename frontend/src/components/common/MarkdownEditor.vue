<script setup lang="ts">
/**
 * WYSIWYG Markdown editor exposed as a v-model component.
 *
 * The bound value is **Markdown**: what the app stores, renders with markdown-it and
 * posts to Discord. So the editor offers a Discord-safe subset: inline formatting,
 * headings, quotes, lists, dividers and links, from a toolbar that stays above the
 * text. Markdown typed straight in (`# `, `> `, `- `, `**bold**`) is formatted as it
 * is typed. The engine and how it writes Markdown are in `lib/markdownEditor.ts`.
 *
 * The engine (Tiptap) is imported when the editor mounts, so it stays out of every
 * page that has none. Until it arrives the toolbar is shown disabled.
 */
import { nextTick, onBeforeUnmount, onMounted, ref, useId, useTemplateRef, watch } from 'vue'
import type { Editor } from '@tiptap/core'
import EmojiPickerModal from '@/components/common/EmojiPickerModal.vue'
import type * as EngineModule from '@/lib/markdownEditor'

const props = withDefaults(
  defineProps<{ modelValue: string; minHeight?: string; placeholder?: string }>(),
  { minHeight: '180px', placeholder: '' },
)
const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'blur'): void
}>()

type Engine = typeof EngineModule

const surface = useTemplateRef<HTMLDivElement>('surface')
const linkInput = useTemplateRef<HTMLInputElement>('link-input')

let engine: Engine | null = null
let editor: Editor | null = null
/** The Markdown the editor holds as of its last change, so only a real change is emitted. */
let current = ''
let disposed = false
// Read `disposed` through a call so control-flow analysis doesn't treat it as
// still-false after the await below - it flips if the component unmounts while the
// engine is loading.
const isDisposed = (): boolean => disposed

const ready = ref(false)

interface Tool {
  key: string
  label: string
  /** A solid icon's name. */
  icon: string
  run: (editor: Editor) => void
  /** Whether the selection already has it; a tool without one is an action, not a toggle. */
  isActive?: (editor: Editor) => boolean
}

const toolGroups: Tool[][] = [
  [
    {
      key: 'bold',
      label: 'Bold',
      icon: 'bold',
      run: (e) => e.chain().focus().toggleBold().run(),
      isActive: (e) => e.isActive('bold'),
    },
    {
      key: 'italic',
      label: 'Italic',
      icon: 'italic',
      run: (e) => e.chain().focus().toggleItalic().run(),
      isActive: (e) => e.isActive('italic'),
    },
    {
      key: 'strike',
      label: 'Strikethrough',
      icon: 'strikethrough',
      run: (e) => e.chain().focus().toggleStrike().run(),
      isActive: (e) => e.isActive('strike'),
    },
    {
      key: 'code',
      label: 'Inline code',
      icon: 'code',
      run: (e) => e.chain().focus().toggleCode().run(),
      isActive: (e) => e.isActive('code'),
    },
  ],
  [
    {
      key: 'bulletList',
      label: 'Bulleted list',
      icon: 'list-ul',
      run: (e) => e.chain().focus().toggleBulletList().run(),
      isActive: (e) => e.isActive('bulletList'),
    },
    {
      key: 'orderedList',
      label: 'Numbered list',
      icon: 'list-ol',
      run: (e) => e.chain().focus().toggleOrderedList().run(),
      isActive: (e) => e.isActive('orderedList'),
    },
  ],
  [
    {
      key: 'link',
      label: 'Link',
      icon: 'link',
      run: (e) => void openLink(e),
      isActive: (e) => e.isActive('link'),
    },
    {
      key: 'emoji',
      label: 'Emoji',
      icon: 'face-smile',
      run: () => {
        emojiOpen.value = true
      },
    },
  ],
  [
    {
      key: 'blockquote',
      label: 'Quote',
      icon: 'block-quote',
      run: (e) => e.chain().focus().toggleBlockquote().run(),
      isActive: (e) => e.isActive('blockquote'),
    },
    {
      key: 'divider',
      label: 'Divider',
      icon: 'horizontal-rule',
      run: (e) => e.chain().focus().setHorizontalRule().run(),
    },
  ],
]
const tools = toolGroups.flat()

const HEADING_LEVELS = [1, 2, 3, 4, 5, 6] as const
const blockStyles = [
  { value: 'paragraph', label: 'Paragraph' },
  ...HEADING_LEVELS.map((level) => ({ value: `h${level}`, label: `Heading ${level}` })),
]

/** Which toggles the selection has, and its block's style; refreshed on each transaction. */
const active = ref<Record<string, boolean>>({})
const blockStyle = ref('paragraph')

function syncToolbar(): void {
  const e = editor
  if (!e) {
    return
  }
  active.value = Object.fromEntries(tools.map((tool) => [tool.key, tool.isActive?.(e) ?? false]))
  const level = HEADING_LEVELS.find((l) => e.isActive('heading', { level: l }))
  blockStyle.value = level ? `h${level}` : 'paragraph'
}

function use(tool: Tool): void {
  if (editor) {
    tool.run(editor)
  }
}

function onStyleChange(event: Event): void {
  const value = (event.target as HTMLSelectElement).value
  const level = HEADING_LEVELS.find((l) => value === `h${l}`)
  const chain = editor?.chain().focus()
  if (level) {
    chain?.setHeading({ level }).run()
  } else {
    chain?.setParagraph().run()
  }
}

// -- Links ----------------------------------------------------------------------
const linkOpen = ref(false)
const linkUrl = ref('')
const linkRefused = ref(false)
const linkNoteId = useId()

async function openLink(e: Editor): Promise<void> {
  linkUrl.value = String(e.getAttributes('link').href ?? '')
  linkRefused.value = false
  linkOpen.value = true
  await nextTick()
  linkInput.value?.focus()
}

function closeLink(): void {
  linkOpen.value = false
  editor?.commands.focus()
}

/** The address as a link takes it: a bare "example.com" gains https://. */
function linkAddress(typed: string): string {
  const address = typed.trim()
  return /^[a-z][a-z0-9+.-]*:/i.test(address) ? address : `https://${address}`
}

/**
 * Links the selection, or the link the cursor is in, to the typed address. With
 * nothing selected the address itself goes in as the link's text. Only web and email
 * addresses are taken: they are what Discord and the site's renderer link.
 */
function applyLink(): void {
  if (!editor) {
    return
  }
  if (!linkUrl.value.trim()) {
    removeLink()
    return
  }
  const href = linkAddress(linkUrl.value)
  if (!/^(?:https?:\/\/[^\s]+|mailto:[^\s]+)$/i.test(href)) {
    linkRefused.value = true
    return
  }
  const chain = editor.chain().focus()
  if (editor.state.selection.empty && !editor.isActive('link')) {
    chain
      .insertContent({ type: 'text', text: href, marks: [{ type: 'link', attrs: { href } }] })
      .unsetMark('link')
      .run()
  } else {
    chain.extendMarkRange('link').setLink({ href }).run()
  }
  linkOpen.value = false
}

function removeLink(): void {
  editor?.chain().focus().extendMarkRange('link').unsetLink().run()
  linkOpen.value = false
}

// -- Emoji ----------------------------------------------------------------------
/** Whether the emoji picker (opened from the toolbar) is showing. */
const emojiOpen = ref(false)

/**
 * Inserts the picked emoji at the cursor as a literal character - NOT a
 * `:shortcode:`, which only Discord resolves; the character itself survives every
 * hop and renders in the site's own view too.
 *
 * The insert waits for the modal to unmount. ModalOverlay hands focus back to the
 * editor with a raw `.focus()`, which collapses the caret to the start of the block;
 * inserting first would let that land after our own focus and move the caret. Going
 * second, `focus()` puts the caret just after the emoji.
 */
async function onEmojiSelect(emoji: string): Promise<void> {
  emojiOpen.value = false
  if (!emoji) {
    return
  }
  await nextTick()
  editor?.chain().focus().insertContent({ type: 'text', text: emoji }).run()
}

// -- Lifecycle ------------------------------------------------------------------
function onUpdate(): void {
  if (!editor || !engine) {
    return
  }
  const markdown = engine.toMarkdown(editor)
  if (markdown !== current) {
    current = markdown
    emit('update:modelValue', markdown)
  }
}

onMounted(async () => {
  const loaded = await import('@/lib/markdownEditor')
  if (isDisposed() || !surface.value) {
    return
  }
  engine = loaded
  editor = loaded.createMarkdownEditor({
    element: surface.value,
    content: props.modelValue || '',
    placeholder: props.placeholder,
    onUpdate,
    onTransaction: syncToolbar,
    onBlur: () => emit('blur'),
  })
  // The starting text, as the editor writes it: what a change is measured against,
  // so opening a form never reports an edit nobody made.
  current = loaded.toMarkdown(editor)
  ready.value = true
  syncToolbar()
})

onBeforeUnmount(() => {
  disposed = true
  editor?.destroy()
  editor = null
  engine = null
})

// A value set from outside (an AniList fill, a form reset) replaces the text; the
// editor's own changes come back here unchanged and are left alone.
watch(
  () => props.modelValue,
  (value) => {
    if (editor && engine && value !== current) {
      engine.setMarkdown(editor, value || '')
      current = engine.toMarkdown(editor)
    }
  },
)
</script>

<template>
  <div class="markdown-editor" :style="{ '--markdown-editor-min-height': minHeight }">
    <div class="markdown-editor__toolbar" role="toolbar" aria-label="Formatting">
      <select
        class="markdown-editor__style"
        aria-label="Text style"
        :value="blockStyle"
        :disabled="!ready"
        @change="onStyleChange"
      >
        <option v-for="style in blockStyles" :key="style.value" :value="style.value">
          {{ style.label }}
        </option>
      </select>
      <template v-for="group in toolGroups" :key="group[0]?.key">
        <span class="markdown-editor__separator" aria-hidden="true"></span>
        <!-- mousedown is prevented so a click leaves the focus, and the selection,
             in the text. -->
        <button
          v-for="tool in group"
          :key="tool.key"
          type="button"
          class="btn-sm markdown-editor__tool"
          :class="active[tool.key] ? 'btn-action' : 'btn-neutral'"
          :title="tool.label"
          :aria-label="tool.label"
          :aria-pressed="tool.isActive ? Boolean(active[tool.key]) : undefined"
          :disabled="!ready"
          @mousedown.prevent
          @click="use(tool)"
        >
          <font-awesome-icon :icon="['fas', tool.icon]" />
        </button>
      </template>
    </div>

    <form v-if="linkOpen" class="markdown-editor__link" novalidate @submit.prevent="applyLink">
      <input
        ref="link-input"
        v-model="linkUrl"
        class="markdown-editor__link-input"
        type="url"
        inputmode="url"
        placeholder="https://"
        aria-label="Link address"
        :aria-invalid="linkRefused || undefined"
        :aria-describedby="linkRefused ? linkNoteId : undefined"
        @input="linkRefused = false"
        @keydown.esc.prevent="closeLink"
      />
      <button type="submit" class="btn-confirm btn-sm">Apply</button>
      <button v-if="active.link" type="button" class="btn-danger btn-sm" @click="removeLink">
        <font-awesome-icon :icon="['fas', 'link-slash']" />
        Remove
      </button>
      <button type="button" class="btn-neutral btn-sm" @click="closeLink">Cancel</button>
      <p v-if="linkRefused" :id="linkNoteId" class="markdown-editor__link-note">
        Use a web address (https://...) or an email address (mailto:...).
      </p>
    </form>

    <div ref="surface"></div>
  </div>

  <!-- Opened from the toolbar's emoji button; inserts the character at the cursor.
       The picker itself is lazy inside the modal, so it costs nothing until used. -->
  <EmojiPickerModal v-if="emojiOpen" @select="onEmojiSelect" @close="emojiOpen = false" />
</template>
