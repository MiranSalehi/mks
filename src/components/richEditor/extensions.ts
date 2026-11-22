import { Extension, Mark, Node, mergeAttributes, type Editor } from '@tiptap/core'

// What the editor needs beyond Tiptap's own extensions to write what MKSine's CKEditor
// writes: named styles (classes), markers and pens, block indents, list styles, video and
// audio. The server's sanitizer (packages/mks/html.go) keeps exactly these.

/** MKSine's "Styles" dropdown (CKEditor style definitions). */
export const styleDefinitions: { name: string; element: string; classes: string }[] = [
  { name: 'Article category', element: 'h3', classes: 'category' },
  { name: 'Title', element: 'h2', classes: 'document-title' },
  { name: 'Subtitle', element: 'h3', classes: 'document-subtitle' },
  { name: 'Info box', element: 'p', classes: 'info-box' },
  { name: 'Side quote', element: 'blockquote', classes: 'side-quote' },
  { name: 'Marker', element: 'span', classes: 'marker' },
  { name: 'Spoiler', element: 'span', classes: 'spoiler' },
  { name: 'Code (dark)', element: 'pre', classes: 'fancy-code fancy-code-dark' },
  { name: 'Code (bright)', element: 'pre', classes: 'fancy-code fancy-code-bright' },
]

const INDENT = 40

/** Classes (named styles) on blocks, block indents and list styles. */
export const BlockAttributes = Extension.create({
  name: 'blockAttributes',
  addGlobalAttributes() {
    return [
      {
        types: ['paragraph', 'heading', 'blockquote', 'codeBlock'],
        attributes: {
          class: {
            default: null,
            parseHTML: (el) => el.getAttribute('class') || null,
            renderHTML: (a) => (a.class ? { class: a.class } : {}),
          },
        },
      },
      {
        types: ['paragraph', 'heading'],
        attributes: {
          indent: {
            default: 0,
            parseHTML: (el) => {
              const m = /margin-(?:left|right|inline-start)\s*:\s*(\d+)px/.exec(el.getAttribute('style') ?? '')
              return m ? Math.min(10, Math.round(Number(m[1]) / INDENT)) : 0
            },
            renderHTML: (a) => (a.indent ? { style: `margin-inline-start: ${Number(a.indent) * INDENT}px` } : {}),
          },
        },
      },
      {
        types: ['bulletList', 'orderedList'],
        attributes: {
          listStyle: {
            default: null,
            parseHTML: (el) => (el as HTMLElement).style.listStyleType || null,
            renderHTML: (a) => (a.listStyle ? { style: `list-style-type: ${a.listStyle}` } : {}),
          },
        },
      },
      {
        types: ['orderedList'],
        attributes: {
          reversed: {
            default: false,
            parseHTML: (el) => el.hasAttribute('reversed'),
            renderHTML: (a) => (a.reversed ? { reversed: 'reversed' } : {}),
          },
        },
      },
      {
        types: ['link'],
        attributes: {
          download: {
            default: null,
            parseHTML: (el) => el.getAttribute('download'),
            renderHTML: (a) => (a.download ? { download: a.download } : {}),
          },
        },
      },
    ]
  },
})

/** Highlight: CKEditor's markers (background) and pens (text color), as classes. */
export const Marker = Mark.create({
  name: 'marker',
  addAttributes() {
    return {
      kind: {
        default: 'marker-yellow',
        parseHTML: (el) => el.getAttribute('class') || 'marker-yellow',
        renderHTML: (a) => ({ class: a.kind }),
      },
    }
  },
  parseHTML() {
    return [{ tag: 'mark' }]
  },
  renderHTML({ HTMLAttributes }) {
    return ['mark', HTMLAttributes, 0]
  },
})

/** The inline named styles ("Marker", "Spoiler"): a span with the style's class. */
export const StyleSpan = Mark.create({
  name: 'styleSpan',
  priority: 1100,
  addAttributes() {
    return { class: { default: null, parseHTML: (el) => el.getAttribute('class') } }
  },
  parseHTML() {
    return [{ tag: 'span.marker' }, { tag: 'span.spoiler' }]
  },
  renderHTML({ HTMLAttributes }) {
    return ['span', HTMLAttributes, 0]
  },
})

function mediaNode(name: 'video' | 'audio') {
  return Node.create({
    name,
    group: 'block',
    atom: true,
    draggable: true,
    addAttributes() {
      return { src: { default: null } }
    },
    parseHTML() {
      return [{ tag: `${name}[src]` }]
    },
    renderHTML({ HTMLAttributes }) {
      return [name, mergeAttributes(HTMLAttributes, { controls: '', preload: 'metadata' })]
    },
  })
}

/** Library video and audio inserted by "Insert media". */
export const Video = mediaNode('video')
export const Audio = mediaNode('audio')

/** The block that indent and outdent change: a heading or a paragraph. */
function indentType(editor: Editor): 'heading' | 'paragraph' | null {
  if (editor.isActive('heading')) return 'heading'
  if (editor.isActive('paragraph')) return 'paragraph'
  return null
}

function listItemType(editor: Editor): 'listItem' | 'taskItem' | null {
  if (editor.isActive('taskItem')) return 'taskItem'
  if (editor.isActive('listItem')) return 'listItem'
  return null
}

/** Indent: a list item goes one level deeper; another block moves in by 40px. */
export function canIndent(editor: Editor): boolean {
  const item = listItemType(editor)
  if (item) return editor.can().sinkListItem(item)
  const type = indentType(editor)
  return !!type && Number(editor.getAttributes(type).indent ?? 0) < 10
}

export function indent(editor: Editor) {
  const item = listItemType(editor)
  if (item) return editor.chain().focus().sinkListItem(item).run()
  const type = indentType(editor)
  if (!type) return false
  return editor.chain().focus().updateAttributes(type, { indent: Number(editor.getAttributes(type).indent ?? 0) + 1 }).run()
}

export function canOutdent(editor: Editor): boolean {
  const item = listItemType(editor)
  if (item) return editor.can().liftListItem(item)
  const type = indentType(editor)
  return !!type && Number(editor.getAttributes(type).indent ?? 0) > 0
}

export function outdent(editor: Editor) {
  const item = listItemType(editor)
  if (item) return editor.chain().focus().liftListItem(item).run()
  const type = indentType(editor)
  if (!type) return false
  return editor.chain().focus().updateAttributes(type, { indent: Math.max(0, Number(editor.getAttributes(type).indent ?? 0) - 1) }).run()
}

/** HTML with a line break after each block, for the source view. */
export function formatHTML(html: string): string {
  return html
    .replace(/(<\/(p|h[1-6]|blockquote|pre|ul|ol|li|table|thead|tbody|tr|figure|div|video|audio)>)(?!\n)/g, '$1\n')
    .replace(/(<(ul|ol|table|thead|tbody|tr|blockquote)[^>]*>)(?!\n)/g, '$1\n')
    .replace(/(<(br|hr|img)[^>]*>)(?!\n)/g, '$1\n')
    .trim()
}
