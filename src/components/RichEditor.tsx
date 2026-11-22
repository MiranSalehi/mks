import { EditorContent, useEditor, type Editor } from '@tiptap/react'
import { BubbleMenu } from '@tiptap/react/menus'
import StarterKit from '@tiptap/starter-kit'
import { Table, TableCell, TableHeader, TableRow } from '@tiptap/extension-table'
import { BackgroundColor, Color, FontFamily, FontSize, TextStyle } from '@tiptap/extension-text-style'
import TextAlign from '@tiptap/extension-text-align'
import { TaskItem, TaskList } from '@tiptap/extension-list'
import Image from '@tiptap/extension-image'
import { Placeholder } from '@tiptap/extensions'
import { Fragment, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from 'react'
import { direction } from '../i18n'
import { richEditorButtons, type RichEditorHandle } from '../richEditorButtons'
import { Arrow, icons } from './richEditor/icons'
import { Audio, BlockAttributes, canIndent, canOutdent, formatHTML, indent, Marker, outdent, StyleSpan, styleDefinitions, Video } from './richEditor/extensions'

// Loaded lazily (see fields.tsx): Tiptap is large, and most pages have no rich editor.
//
// The editor of MKSine's CKEditor 5 field: the same toolbar items in the same order, with
// CKEditor's labels, dropdowns and split buttons, items that do not fit moved under "⋮",
// a source view, "show blocks" and a toolbar for tables. Plugins add buttons at the start
// (addRichEditorButton), as MKSine adds "Insert media" and "Insert shortcode".

interface Props {
  labelId: string
  value: unknown
  onChange: (value: unknown) => void
  disabled?: boolean
  invalid?: boolean
  describedBy?: string
  placeholder?: string
  height?: string
}

export function RichEditorControl({ labelId, value, onChange, disabled, invalid, describedBy, placeholder, height = '300px' }: Props) {
  const root = useRef<HTMLDivElement>(null)
  const [source, setSource] = useState<string | null>(null)
  const [showBlocks, setShowBlocks] = useState(false)
  const [dialog, setDialog] = useState<string | null>(null)
  const editor = useEditor({
    extensions: [
      StarterKit.configure({ link: { openOnClick: false, autolink: true, defaultProtocol: 'https' } }),
      TextStyle,
      Color,
      BackgroundColor,
      FontFamily,
      FontSize,
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      TaskList,
      TaskItem.configure({ nested: true }),
      Table.configure({ resizable: false }),
      TableRow,
      TableHeader,
      TableCell,
      Image,
      Video,
      Audio,
      Marker,
      StyleSpan,
      BlockAttributes,
      Placeholder.configure({ placeholder: placeholder ?? '' }),
    ],
    content: typeof value === 'string' ? value : '',
    editable: !disabled,
    shouldRerenderOnTransaction: true, // keeps the toolbar's pressed states current
    editorProps: {
      attributes: {
        role: 'textbox',
        'aria-multiline': 'true',
        'aria-labelledby': labelId,
        ...(describedBy ? { 'aria-describedby': describedBy } : {}),
        ...(invalid ? { 'aria-invalid': 'true' } : {}),
        dir: direction(),
        style: `min-height: ${height}`,
        class: 'mks-prose mks-rte-content px-4 py-3 text-sm text-gray-950 outline-none dark:text-white',
      },
    },
    onUpdate: ({ editor }) => onChange(editor.isEmpty ? '' : editor.getHTML()),
  })

  if (!editor) return null
  const handle: RichEditorHandle = {
    insertHTML: (html) => editor.chain().focus().insertContent(html).run(),
    insertText: (text) => editor.chain().focus().insertContent({ type: 'text', text }).run(),
  }
  function toggleSource() {
    if (source === null) {
      setSource(formatHTML(editor!.getHTML()))
      return
    }
    editor!.commands.setContent(source, { emitUpdate: true })
    setSource(null)
  }
  const open = dialog ? richEditorButtons().find((b) => b.key === dialog) : undefined

  return (
    <div
      ref={root}
      className={
        'mks-rte relative min-w-0 rounded-lg bg-white shadow-sm ring-1 focus-within:ring-2 focus-within:ring-primary-600 dark:bg-white/5 ' +
        (invalid ? 'ring-danger-600 dark:ring-danger-400' : 'ring-gray-950/10 dark:ring-white/20')
      }
    >
      <Toolbar
        editor={editor}
        off={!!disabled}
        source={source !== null}
        showBlocks={showBlocks}
        onSource={toggleSource}
        onShowBlocks={() => setShowBlocks((v) => !v)}
        onPlugin={setDialog}
      />
      <div className={'resize-y overflow-auto rounded-b-lg ' + (showBlocks ? 'mks-show-blocks' : '')} style={{ minHeight: height }}>
        {source !== null && (
          <textarea
            value={source}
            aria-labelledby={labelId}
            dir="ltr"
            spellCheck={false}
            onChange={(e) => {
              setSource(e.target.value)
              onChange(e.target.value) // the server cleans it; the editor takes it on leaving
            }}
            style={{ minHeight: height }}
            className="block w-full resize-none border-0 bg-transparent px-4 py-3 font-mono text-[13px] leading-6 text-gray-950 outline-none dark:text-gray-100"
          />
        )}
        <div hidden={source !== null}>
          <EditorContent editor={editor} />
        </div>
      </div>
      {!disabled && source === null && <TableMenu editor={editor} root={root} />}
      {open?.dialog({ editor: handle, close: () => setDialog(null) })}
    </div>
  )
}

// --- buttons and dropdowns (CKEditor's look) -----------------------------------------------

const btnClass = (on: boolean | undefined) =>
  'inline-flex h-[30px] min-w-[30px] shrink-0 items-center justify-center gap-1.5 rounded-[3px] px-[5px] text-[13px] leading-none whitespace-nowrap text-gray-950 transition-colors disabled:cursor-default disabled:opacity-40 dark:text-gray-100 ' +
  (on
    ? 'bg-primary-50 text-primary-600 hover:bg-primary-100 dark:bg-primary-400/20 dark:text-primary-300'
    : 'hover:bg-gray-100 disabled:hover:bg-transparent dark:hover:bg-white/10')

const keep = (e: React.MouseEvent) => e.preventDefault() // keeps the editor's selection

function Btn({ label, on, disabled, onClick, children, text }: { label: string; on?: boolean; disabled?: boolean; onClick: () => void; children: ReactNode; text?: boolean }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={text ? undefined : label}
      aria-pressed={on === undefined ? undefined : on}
      disabled={disabled}
      onMouseDown={keep}
      onClick={onClick}
      className={btnClass(on)}
    >
      {children}
    </button>
  )
}

/** Closes on a click outside or Escape. */
function useDismiss(open: boolean, close: () => void) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const down = (e: MouseEvent) => {
      if (!ref.current?.contains(e.target as Node)) close()
    }
    const key = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.stopPropagation()
        close()
      }
    }
    document.addEventListener('mousedown', down)
    document.addEventListener('keydown', key, true)
    return () => {
      document.removeEventListener('mousedown', down)
      document.removeEventListener('keydown', key, true)
    }
  }, [open, close])
  return ref
}

const panelClass = 'absolute top-full start-0 z-40 mt-1 rounded-[3px] border border-gray-200 bg-white text-gray-950 shadow-lg dark:border-white/10 dark:bg-gray-900 dark:text-gray-100'

function Dropdown({
  label,
  button,
  on,
  disabled,
  split,
  noArrow,
  children,
}: {
  label: string
  button: ReactNode
  on?: boolean
  disabled?: boolean
  /** A split button: the main part runs this, the arrow opens the panel. */
  split?: () => void
  noArrow?: boolean
  children: (close: () => void) => ReactNode
}) {
  const [open, setOpen] = useState(false)
  const [flip, setFlip] = useState(false)
  const panel = useRef<HTMLDivElement>(null)
  const close = useRef(() => setOpen(false)).current
  const ref = useDismiss(open, close)
  useEffect(() => {
    if (disabled) setOpen(false)
  }, [disabled])
  // A panel that would leave the window opens toward the other side.
  useLayoutEffect(() => {
    if (!open) {
      setFlip(false)
      return
    }
    const r = panel.current?.getBoundingClientRect()
    if (r && !flip && (r.left < 4 || r.right > window.innerWidth - 4)) setFlip(true)
  }, [open, flip])
  return (
    <div ref={ref} className="relative flex shrink-0">
      {split ? (
        <div className="flex">
          <button type="button" title={label} aria-label={label} aria-pressed={!!on} disabled={disabled} onMouseDown={keep} onClick={split} className={btnClass(on) + ' rounded-e-none'}>
            {button}
          </button>
          <button
            type="button"
            title={label}
            aria-label={label}
            aria-haspopup="true"
            aria-expanded={open}
            disabled={disabled}
            onMouseDown={keep}
            onClick={() => setOpen((v) => !v)}
            className={btnClass(open || on) + ' min-w-0 rounded-s-none px-[3px]'}
          >
            <Arrow />
          </button>
        </div>
      ) : (
        <button
          type="button"
          title={label}
          aria-label={label}
          aria-haspopup="true"
          aria-expanded={open}
          disabled={disabled}
          onMouseDown={keep}
          onClick={() => setOpen((v) => !v)}
          className={btnClass(open || on)}
        >
          {button}
          {!noArrow && <Arrow />}
        </button>
      )}
      {open && (
        <div ref={panel} className={panelClass + (flip ? ' start-auto end-0' : '')}>
          {children(close)}
        </div>
      )}
    </div>
  )
}

function MenuItem({ on, disabled, onClick, children, style }: { on?: boolean; disabled?: boolean; onClick: () => void; children: ReactNode; style?: React.CSSProperties }) {
  return (
    <button
      type="button"
      role="menuitemradio"
      aria-checked={!!on}
      disabled={disabled}
      onMouseDown={keep}
      onClick={onClick}
      style={style}
      className={
        'flex w-full items-center gap-2 px-3 py-1.5 text-start text-[13px] whitespace-nowrap disabled:opacity-40 ' +
        (on ? 'bg-primary-50 text-primary-700 dark:bg-primary-400/20 dark:text-primary-300' : 'hover:bg-gray-100 dark:hover:bg-white/10')
      }
    >
      {children}
    </button>
  )
}

const Sep = () => <span aria-hidden="true" className="mx-[5px] h-6 w-px shrink-0 self-center bg-gray-200 dark:bg-white/15" />

// --- the toolbar ---------------------------------------------------------------------------

const headingItems = [
  { level: 0, label: 'Paragraph', style: { fontSize: '13px' } },
  { level: 1, label: 'Heading 1', style: { fontSize: '20px', fontWeight: 700 } },
  { level: 2, label: 'Heading 2', style: { fontSize: '17px', fontWeight: 700 } },
  { level: 3, label: 'Heading 3', style: { fontSize: '15px', fontWeight: 700 } },
  { level: 4, label: 'Heading 4', style: { fontSize: '14px', fontWeight: 700 } },
  { level: 5, label: 'Heading 5', style: { fontSize: '13px', fontWeight: 700 } },
  { level: 6, label: 'Heading 6', style: { fontSize: '12px', fontWeight: 700 } },
] as const

const fontSizes = ['10px', '12px', '14px', '', '18px', '20px', '22px']
const fontFamilies = [
  ['Default', ''],
  ['Arial', 'Arial, Helvetica, sans-serif'],
  ['Courier New', 'Courier New, Courier, monospace'],
  ['Georgia', 'Georgia, serif'],
  ['Lucida Sans Unicode', 'Lucida Sans Unicode, Lucida Grande, sans-serif'],
  ['Tahoma', 'Tahoma, Geneva, sans-serif'],
  ['Times New Roman', 'Times New Roman, Times, serif'],
  ['Trebuchet MS', 'Trebuchet MS, Helvetica, sans-serif'],
  ['Verdana', 'Verdana, Geneva, sans-serif'],
] as const
const colors = [
  ['Black', 'hsl(0, 0%, 0%)'],
  ['Dim grey', 'hsl(0, 0%, 30%)'],
  ['Grey', 'hsl(0, 0%, 60%)'],
  ['Light grey', 'hsl(0, 0%, 90%)'],
  ['White', 'hsl(0, 0%, 100%)'],
  ['Red', 'hsl(0, 75%, 60%)'],
  ['Orange', 'hsl(30, 75%, 60%)'],
  ['Yellow', 'hsl(60, 75%, 60%)'],
  ['Light green', 'hsl(90, 75%, 60%)'],
  ['Green', 'hsl(120, 75%, 60%)'],
  ['Aquamarine', 'hsl(150, 75%, 60%)'],
  ['Turquoise', 'hsl(180, 75%, 60%)'],
  ['Light blue', 'hsl(210, 75%, 60%)'],
  ['Blue', 'hsl(240, 75%, 60%)'],
  ['Purple', 'hsl(270, 75%, 60%)'],
] as const
const highlights = [
  ['Yellow marker', 'marker-yellow', '#fdfd77'],
  ['Green marker', 'marker-green', '#62f962'],
  ['Pink marker', 'marker-pink', '#fc7899'],
  ['Blue marker', 'marker-blue', '#72ccfd'],
  ['Red pen', 'pen-red', '#e71313'],
  ['Green pen', 'pen-green', '#128a00'],
] as const
const codeLanguages = [
  ['Plain text', 'plaintext'],
  ['C', 'c'],
  ['C#', 'cs'],
  ['C++', 'cpp'],
  ['CSS', 'css'],
  ['Diff', 'diff'],
  ['HTML', 'html'],
  ['Java', 'java'],
  ['JavaScript', 'javascript'],
  ['PHP', 'php'],
  ['Python', 'python'],
  ['Ruby', 'ruby'],
  ['TypeScript', 'typescript'],
  ['XML', 'xml'],
] as const
const bulletStyles = [
  ['Disc', 'disc', '●'],
  ['Circle', 'circle', '○'],
  ['Square', 'square', '■'],
] as const
const numberStyles = [
  ['Decimal', 'decimal', '1.'],
  ['Decimal with leading zero', 'decimal-leading-zero', '01.'],
  ['Lower–roman', 'lower-roman', 'i.'],
  ['Upper-roman', 'upper-roman', 'I.'],
  ['Lower-latin', 'lower-latin', 'a.'],
  ['Upper-latin', 'upper-latin', 'A.'],
] as const

type Item = { key: string; node: ReactNode } | 'sep'

function Toolbar({
  editor,
  off,
  source,
  showBlocks,
  onSource,
  onShowBlocks,
  onPlugin,
}: {
  editor: Editor
  off: boolean
  source: boolean
  showBlocks: boolean
  onSource: () => void
  onShowBlocks: () => void
  onPlugin: (key: string) => void
}) {
  const dis = off || source
  const chain = () => editor.chain().focus()
  const items: Item[] = []
  const add = (key: string, node: ReactNode) => items.push({ key, node })

  const plugins = richEditorButtons()
  for (const b of plugins) {
    add(b.key, (
      <Btn label={b.label} disabled={dis} onClick={() => onPlugin(b.key)}>
        {b.icon}
      </Btn>
    ))
  }
  if (plugins.length) items.push('sep')

  add('undo', (
    <Btn label="Undo (⌘Z)" disabled={dis || !editor.can().undo()} onClick={() => chain().undo().run()}>
      {icons.undo}
    </Btn>
  ))
  add('redo', (
    <Btn label="Redo (⌘Y)" disabled={dis || !editor.can().redo()} onClick={() => chain().redo().run()}>
      {icons.redo}
    </Btn>
  ))
  items.push('sep')
  add('source', (
    <Btn label="Source" text on={source} disabled={off} onClick={onSource}>
      {icons.source}
      <span>Source</span>
    </Btn>
  ))
  add('showBlocks', (
    <Btn label="Show blocks" on={showBlocks} disabled={dis} onClick={onShowBlocks}>
      {icons.showBlocks}
    </Btn>
  ))
  items.push('sep')

  // Heading
  const level = headingItems.find((h) => h.level > 0 && editor.isActive('heading', { level: h.level }))
  const headingLabel = level ? level.label : editor.isActive('paragraph') ? 'Paragraph' : 'Heading'
  add('heading', (
    <Dropdown label="Heading" disabled={dis} button={<span className="w-[5.75rem] truncate text-start">{headingLabel}</span>}>
      {(close) => (
        <div className="min-w-44 py-1" role="menu">
          {headingItems.map((h) => (
            <MenuItem
              key={h.level}
              style={h.style}
              on={h.level === 0 ? editor.isActive('paragraph') : editor.isActive('heading', { level: h.level })}
              onClick={() => {
                if (h.level === 0) chain().setParagraph().run()
                else chain().setHeading({ level: h.level }).run()
                close()
              }}
            >
              {h.label}
            </MenuItem>
          ))}
        </div>
      )}
    </Dropdown>
  ))

  // Styles
  const styleState = styleDefinitions.map((d) => {
    const type = d.element === 'h2' || d.element === 'h3' ? 'heading' : d.element === 'p' ? 'paragraph' : d.element === 'blockquote' ? 'blockquote' : d.element === 'pre' ? 'codeBlock' : null
    const applies =
      d.element === 'span'
        ? true
        : type === 'heading'
          ? editor.isActive('heading', { level: d.element === 'h2' ? 2 : 3 })
          : !!type && editor.isActive(type)
    const on =
      d.element === 'span'
        ? editor.isActive('styleSpan', { class: d.classes })
        : applies && !!type && String(editor.getAttributes(type).class ?? '') === d.classes
    return { d, type, applies, on }
  })
  const onStyles = styleState.filter((s) => s.on)
  const styleLabel = onStyles.length === 0 ? 'Styles' : onStyles.length === 1 ? onStyles[0].d.name : 'Multiple styles'
  const styleTile = (s: (typeof styleState)[number], close: () => void) => (
    <button
      key={s.d.name}
      type="button"
      disabled={!s.applies}
      aria-pressed={s.on}
      onMouseDown={keep}
      onClick={() => {
        if (s.d.element === 'span') {
          if (s.on) chain().unsetMark('styleSpan').run()
          else chain().setMark('styleSpan', { class: s.d.classes }).run()
        } else if (s.type) {
          chain().updateAttributes(s.type, { class: s.on ? null : s.d.classes }).run()
        }
        close()
      }}
      className={
        'flex h-[4.5rem] w-[8.5rem] flex-col items-start justify-between overflow-hidden rounded-[3px] border p-2 text-start disabled:opacity-40 ' +
        (s.on ? 'border-primary-500 bg-primary-50 dark:bg-primary-400/20' : 'border-gray-200 hover:bg-gray-50 dark:border-white/10 dark:hover:bg-white/5')
      }
    >
      <span dir="ltr" className={'mks-prose pointer-events-none w-full truncate text-start text-[12px] ' + s.d.classes}>
        AaBbCcDdEeFfGgHhIiJj
      </span>
      <span className="text-[12px] text-gray-600 dark:text-gray-300">{s.d.name}</span>
    </button>
  )
  add('style', (
    <Dropdown label="Styles" disabled={dis} on={onStyles.length > 0} button={<span className="w-[6.5rem] truncate text-start">{styleLabel}</span>}>
      {(close) => (
        <div className="w-[28.5rem] space-y-2 p-3">
          <p className="text-[11px] font-semibold tracking-wide text-gray-500 uppercase">Block styles</p>
          <div className="flex flex-wrap gap-2">{styleState.filter((s) => s.d.element !== 'span').map((s) => styleTile(s, close))}</div>
          <p className="pt-1 text-[11px] font-semibold tracking-wide text-gray-500 uppercase">Text styles</p>
          <div className="flex flex-wrap gap-2">{styleState.filter((s) => s.d.element === 'span').map((s) => styleTile(s, close))}</div>
        </div>
      )}
    </Dropdown>
  ))
  items.push('sep')

  // Font size, family, colors
  const textStyle = editor.getAttributes('textStyle')
  add('fontSize', (
    <Dropdown label="Font Size" disabled={dis} button={icons.fontSize}>
      {(close) => (
        <div className="min-w-28 py-1" role="menu">
          {fontSizes.map((s) => (
            <MenuItem
              key={s || 'default'}
              style={s ? { fontSize: s } : undefined}
              on={(textStyle.fontSize ?? '') === s}
              onClick={() => {
                if (s) chain().setFontSize(s).run()
                else chain().unsetFontSize().run()
                close()
              }}
            >
              {s ? s.replace('px', '') : 'Default'}
            </MenuItem>
          ))}
        </div>
      )}
    </Dropdown>
  ))
  add('fontFamily', (
    <Dropdown label="Font Family" disabled={dis} button={icons.fontFamily}>
      {(close) => (
        <div className="min-w-48 py-1" role="menu">
          {fontFamilies.map(([name, value]) => (
            <MenuItem
              key={name}
              style={value ? { fontFamily: value } : undefined}
              on={(textStyle.fontFamily ?? '') === value}
              onClick={() => {
                if (value) chain().setFontFamily(value).run()
                else chain().unsetFontFamily().run()
                close()
              }}
            >
              {name}
            </MenuItem>
          ))}
        </div>
      )}
    </Dropdown>
  ))
  const colorPanel = (current: string | undefined, set: (c: string) => void, unset: () => void, removeLabel: string) => (close: () => void) => (
    <div className="w-[12.5rem] p-2">
      <button
        type="button"
        onMouseDown={keep}
        onClick={() => {
          unset()
          close()
        }}
        className="mb-2 flex w-full items-center gap-2 rounded-[3px] px-1.5 py-1 text-[13px] hover:bg-gray-100 dark:hover:bg-white/10"
      >
        {icons.eraser}
        {removeLabel}
      </button>
      <div className="grid grid-cols-5 gap-1.5">
        {colors.map(([name, c]) => (
          <button
            key={name}
            type="button"
            title={name}
            aria-label={name}
            aria-pressed={current === c}
            onMouseDown={keep}
            onClick={() => {
              set(c)
              close()
            }}
            style={{ background: c }}
            className={'size-[1.85rem] rounded-[3px] ring-1 ring-gray-950/15 hover:scale-110 ' + (current === c ? 'outline-2 outline-offset-1 outline-primary-500' : '')}
          />
        ))}
      </div>
      <label className="mt-2 flex cursor-pointer items-center gap-2 rounded-[3px] border-t border-gray-100 px-1.5 pt-2 text-[13px] dark:border-white/10">
        <input type="color" className="size-6 cursor-pointer border-0 bg-transparent p-0" onChange={(e) => set(e.target.value)} />
        Color picker
      </label>
    </div>
  )
  add('fontColor', (
    <Dropdown label="Font Color" disabled={dis} button={icons.fontColor}>
      {colorPanel(textStyle.color, (c) => chain().setColor(c).run(), () => chain().unsetColor().run(), 'Remove color')}
    </Dropdown>
  ))
  add('fontBackgroundColor', (
    <Dropdown label="Font Background Color" disabled={dis} button={icons.fontBackgroundColor}>
      {colorPanel(textStyle.backgroundColor, (c) => chain().setBackgroundColor(c).run(), () => chain().unsetBackgroundColor().run(), 'Remove color')}
    </Dropdown>
  ))
  items.push('sep')

  add('bold', (
    <Btn label="Bold (⌘B)" on={editor.isActive('bold')} disabled={dis} onClick={() => chain().toggleBold().run()}>
      {icons.bold}
    </Btn>
  ))
  add('italic', (
    <Btn label="Italic (⌘I)" on={editor.isActive('italic')} disabled={dis} onClick={() => chain().toggleItalic().run()}>
      {icons.italic}
    </Btn>
  ))
  add('underline', (
    <Btn label="Underline (⌘U)" on={editor.isActive('underline')} disabled={dis} onClick={() => chain().toggleUnderline().run()}>
      {icons.underline}
    </Btn>
  ))
  items.push('sep')

  add('link', (
    <Dropdown label="Link (⌘K)" noArrow on={editor.isActive('link')} disabled={dis} button={icons.link}>
      {(close) => <LinkForm editor={editor} close={close} />}
    </Dropdown>
  ))
  add('insertTable', (
    <Dropdown label="Insert table" disabled={dis} button={icons.table}>
      {(close) => (
        <TableGrid
          onPick={(rows, cols) => {
            chain().insertTable({ rows, cols, withHeaderRow: false }).run()
            close()
          }}
        />
      )}
    </Dropdown>
  ))
  const [lastMarker, setLastMarker] = useLastMarker()
  const markerOn = editor.isActive('marker')
  add('highlight', (
    <Dropdown
      label="Highlight"
      disabled={dis}
      on={markerOn}
      split={() => (markerOn ? chain().unsetMark('marker').run() : chain().setMark('marker', { kind: lastMarker }).run())}
      button={<span style={{ '--rte-marker': highlights.find((h) => h[1] === lastMarker)?.[2] } as React.CSSProperties}>{icons.highlight}</span>}
    >
      {(close) => (
        <div className="flex items-center gap-[2px] p-1">
          {highlights.map(([name, kind, c]) => (
            <Btn
              key={kind}
              label={name}
              on={editor.isActive('marker', { kind })}
              onClick={() => {
                setLastMarker(kind)
                chain().setMark('marker', { kind }).run()
                close()
              }}
            >
              <span style={{ '--rte-marker': c } as React.CSSProperties}>{icons.highlight}</span>
            </Btn>
          ))}
          <Sep />
          <Btn
            label="Remove highlight"
            onClick={() => {
              chain().unsetMark('marker').run()
              close()
            }}
          >
            {icons.eraser}
          </Btn>
        </div>
      )}
    </Dropdown>
  ))
  add('blockQuote', (
    <Btn label="Block quote" on={editor.isActive('blockquote')} disabled={dis} onClick={() => chain().toggleBlockquote().run()}>
      {icons.blockQuote}
    </Btn>
  ))
  const inCode = editor.isActive('codeBlock')
  add('codeBlock', (
    <Dropdown label="Insert code block" disabled={dis} on={inCode} split={() => chain().toggleCodeBlock({ language: 'plaintext' }).run()} button={icons.codeBlock}>
      {(close) => (
        <div className="min-w-36 py-1" role="menu">
          {codeLanguages.map(([name, language]) => (
            <MenuItem
              key={language}
              on={inCode && editor.getAttributes('codeBlock').language === language}
              onClick={() => {
                if (inCode) chain().updateAttributes('codeBlock', { language }).run()
                else chain().setCodeBlock({ language }).run()
                close()
              }}
            >
              {name}
            </MenuItem>
          ))}
        </div>
      )}
    </Dropdown>
  ))
  items.push('sep')

  // Alignment: unset means the content's own side.
  const natural = direction() === 'rtl' ? 'right' : 'left'
  const aligns = [
    ['Align left', 'left', icons.alignLeft],
    ['Align right', 'right', icons.alignRight],
    ['Align center', 'center', icons.alignCenter],
    ['Justify', 'justify', icons.alignJustify],
  ] as const
  const blockType = editor.isActive('heading') ? 'heading' : 'paragraph'
  const align = (editor.getAttributes(blockType).textAlign as string | null) || natural
  add('alignment', (
    <Dropdown label="Text alignment" disabled={dis} button={aligns.find((a) => a[1] === align)?.[2] ?? icons.alignLeft}>
      {(close) => (
        <div className="flex items-center gap-[2px] p-1">
          {aligns.map(([name, value, icon]) => (
            <Btn
              key={value}
              label={name}
              on={align === value}
              onClick={() => {
                if (value === natural) chain().unsetTextAlign().run()
                else chain().setTextAlign(value).run()
                close()
              }}
            >
              {icon}
            </Btn>
          ))}
        </div>
      )}
    </Dropdown>
  ))
  items.push('sep')

  // Lists
  const inBullet = editor.isActive('bulletList')
  const inOrdered = editor.isActive('orderedList')
  const listTile = (name: string, mark: string, on: boolean, run: () => void) => (
    <button
      key={name}
      type="button"
      title={name}
      aria-label={name}
      aria-pressed={on}
      onMouseDown={keep}
      onClick={run}
      className={
        'flex h-[3.25rem] w-[3.25rem] flex-col justify-center gap-1 rounded-[3px] border px-1.5 text-[9px] leading-none ' +
        (on ? 'border-primary-500 bg-primary-50 text-primary-700 dark:bg-primary-400/20' : 'border-gray-200 hover:bg-gray-50 dark:border-white/10 dark:hover:bg-white/5')
      }
    >
      {[0, 1, 2].map((i) => (
        <span key={i} className="flex items-center gap-1">
          <span className="w-3 text-end">{mark}</span>
          <span className="h-[3px] flex-1 rounded bg-current opacity-30" />
        </span>
      ))}
    </button>
  )
  add('bulletedList', (
    <Dropdown label="Bulleted List" disabled={dis} on={inBullet} split={() => chain().toggleBulletList().run()} button={icons.bulletedList}>
      {(close) => (
        <div className="flex gap-1.5 p-2">
          {bulletStyles.map(([name, style, mark]) =>
            listTile(name, mark, inBullet && (editor.getAttributes('bulletList').listStyle ?? 'disc') === style, () => {
              if (!inBullet) chain().toggleBulletList().run()
              chain().updateAttributes('bulletList', { listStyle: style === 'disc' ? null : style }).run()
              close()
            }),
          )}
        </div>
      )}
    </Dropdown>
  ))
  const ordered = editor.getAttributes('orderedList')
  add('numberedList', (
    <Dropdown label="Numbered List" disabled={dis} on={inOrdered} split={() => chain().toggleOrderedList().run()} button={icons.numberedList}>
      {(close) => (
        <div className="w-[11.75rem] p-2">
          <div className="grid grid-cols-3 gap-1.5">
            {numberStyles.map(([name, style, mark]) =>
              listTile(name, mark, inOrdered && (ordered.listStyle ?? 'decimal') === style, () => {
                if (!inOrdered) chain().toggleOrderedList().run()
                chain().updateAttributes('orderedList', { listStyle: style === 'decimal' ? null : style }).run()
                close()
              }),
            )}
          </div>
          <label className="mt-2 flex items-center justify-between gap-2 border-t border-gray-100 pt-2 text-[12px] dark:border-white/10">
            Start at
            <input
              type="number"
              min={0}
              disabled={!inOrdered}
              value={inOrdered ? Number(ordered.start ?? 1) : 1}
              onChange={(e) => chain().updateAttributes('orderedList', { start: Math.max(0, Number(e.target.value) || 0) }).run()}
              className="w-16 rounded-[3px] border border-gray-300 bg-white px-1.5 py-0.5 text-[12px] disabled:opacity-40 dark:border-white/20 dark:bg-white/5"
            />
          </label>
          <label className="mt-2 flex items-center justify-between gap-2 text-[12px]">
            Reversed order
            <input
              type="checkbox"
              disabled={!inOrdered}
              checked={inOrdered && !!ordered.reversed}
              onChange={(e) => chain().updateAttributes('orderedList', { reversed: e.target.checked }).run()}
              className="mks-check"
            />
          </label>
        </div>
      )}
    </Dropdown>
  ))
  add('todoList', (
    <Btn label="To-do List" on={editor.isActive('taskList')} disabled={dis} onClick={() => chain().toggleTaskList().run()}>
      {icons.todoList}
    </Btn>
  ))
  add('outdent', (
    <Btn label="Decrease indent" disabled={dis || !canOutdent(editor)} onClick={() => outdent(editor)}>
      {icons.outdent}
    </Btn>
  ))
  add('indent', (
    <Btn label="Increase indent" disabled={dis || !canIndent(editor)} onClick={() => indent(editor)}>
      {icons.indent}
    </Btn>
  ))

  return <FittingBar items={items} />
}

/** Which marker the split button applies (the last one picked), shared by the editors. */
let lastMarkerKind = 'marker-yellow'
function useLastMarker(): [string, (k: string) => void] {
  const [kind, setKind] = useState(lastMarkerKind)
  return [
    kind,
    (k) => {
      lastMarkerKind = k
      setKind(k)
    },
  ]
}

/**
 * The toolbar row: items that do not fit go into the "⋮" panel (CKEditor's grouping). A
 * hidden copy of the row measures the items.
 */
function FittingBar({ items }: { items: Item[] }) {
  const bar = useRef<HTMLDivElement>(null)
  const measure = useRef<HTMLDivElement>(null)
  const [fit, setFit] = useState(items.length)
  const fitRef = useRef(fit)
  const [more, setMore] = useState(false)
  const closeMore = useRef(() => setMore(false)).current
  const moreRef = useDismiss(more, closeMore)

  // How many items fit; state changes only when the count does (it is measured after
  // every render, as labels change width, and on resize).
  const compute = useRef(() => {})
  compute.current = () => {
    const row = measure.current
    const el = bar.current
    if (!row || !el) return
    const gap = 2
    const widths = Array.from(row.children).map((c) => {
      const cs = getComputedStyle(c)
      return c.getBoundingClientRect().width + parseFloat(cs.marginLeft) + parseFloat(cs.marginRight) + gap
    })
    const style = getComputedStyle(el)
    const avail = el.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight)
    let n = items.length
    if (widths.reduce((a, b) => a + b, 0) > avail) {
      const reserve = 30 + gap + 11 + gap // "⋮" and its separator
      let used = 0
      n = 0
      while (n < widths.length && used + widths[n] <= avail - reserve) used += widths[n++]
      while (n > 0 && items[n - 1] === 'sep') n--
    }
    if (n !== fitRef.current) {
      fitRef.current = n
      setFit(n)
    }
  }
  useLayoutEffect(() => compute.current())
  useEffect(() => {
    const ro = new ResizeObserver(() => compute.current())
    if (bar.current) ro.observe(bar.current)
    return () => ro.disconnect()
  }, [])

  const render = (list: Item[]) => list.map((it, i) => (it === 'sep' ? <Sep key={'sep' + i} /> : <Fragment key={it.key}>{it.node}</Fragment>))
  const rest = items.slice(fit)
  while (rest[0] === 'sep') rest.shift()

  return (
    <div
      ref={bar}
      role="toolbar"
      aria-label="Editor toolbar"
      className="relative flex min-h-10 min-w-0 items-center gap-[2px] rounded-t-lg border-b [contain:inline-size] border-gray-300 bg-gray-50 px-1.5 py-[3px] dark:border-white/20 dark:bg-white/[0.04]"
    >
      <div aria-hidden="true" inert className="pointer-events-none invisible absolute inset-0 overflow-hidden">
        <div ref={measure} className="absolute start-0 top-0 flex items-center gap-[2px] whitespace-nowrap">
          {render(items)}
        </div>
      </div>
      {render(items.slice(0, fit))}
      {rest.length > 0 && (
        <>
          <Sep />
          <div ref={moreRef} className="relative">
            <Btn label="Show more items" on={more} onClick={() => setMore((v) => !v)}>
              {icons.more}
            </Btn>
            {/* Kept mounted (hidden) so the items' open dropdowns survive a resize. */}
            <div hidden={!more} className={panelClass + ' start-auto end-0 flex w-max max-w-[min(48rem,90vw)] flex-wrap items-center gap-[2px] p-1'}>
              {render(rest)}
            </div>
          </div>
        </>
      )}
    </div>
  )
}

function LinkForm({ editor, close }: { editor: Editor; close: () => void }) {
  const current = editor.getAttributes('link') as { href?: string; download?: string | null }
  const [url, setUrl] = useState(current.href ?? '')
  const [download, setDownload] = useState(!!current.download)
  const input = useRef<HTMLInputElement>(null)
  useEffect(() => input.current?.focus(), [])
  function save() {
    const href = url.trim()
    const chain = editor.chain().focus().extendMarkRange('link')
    if (href === '') chain.unsetLink().run()
    else chain.setLink({ href, download: download ? 'file' : null } as { href: string }).run()
    close()
  }
  return (
    <div className="w-80 text-[13px]">
      <div className="flex items-end gap-1 p-2">
        <label className="flex-1">
          <span className="mb-1 block text-[12px] text-gray-600 dark:text-gray-300">Link URL</span>
          <input
            ref={input}
            dir="ltr"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                save()
              }
            }}
            placeholder="https://"
            className="h-[30px] w-full rounded-[3px] border border-gray-300 bg-white px-2 text-[13px] outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-white/20 dark:bg-white/5"
          />
        </label>
        <Btn label="Save" onClick={save}>
          <span className="text-success-600">{icons.check}</span>
        </Btn>
        <Btn label="Cancel" onClick={close}>
          <span className="text-danger-600">{icons.cancel}</span>
        </Btn>
      </div>
      <label className="flex cursor-pointer items-center justify-between gap-3 border-t border-gray-100 px-3 py-2 dark:border-white/10">
        Downloadable
        <input type="checkbox" checked={download} onChange={(e) => setDownload(e.target.checked)} className="mks-check" />
      </label>
      {editor.isActive('link') && (
        <button
          type="button"
          onMouseDown={keep}
          onClick={() => {
            editor.chain().focus().extendMarkRange('link').unsetLink().run()
            close()
          }}
          className="flex w-full items-center gap-2 border-t border-gray-100 px-3 py-2 hover:bg-gray-50 dark:border-white/10 dark:hover:bg-white/5"
        >
          {icons.unlink}
          Unlink
        </button>
      )}
    </div>
  )
}

function TableGrid({ onPick }: { onPick: (rows: number, cols: number) => void }) {
  const [at, setAt] = useState<[number, number]>([1, 1])
  return (
    <div className="p-2">
      <div className="grid grid-cols-10 gap-[3px]" onMouseLeave={() => setAt([1, 1])}>
        {Array.from({ length: 100 }, (_, i) => {
          const r = Math.floor(i / 10) + 1
          const c = (i % 10) + 1
          const on = r <= at[0] && c <= at[1]
          return (
            <button
              key={i}
              type="button"
              aria-label={`${r} × ${c}`}
              onMouseDown={keep}
              onMouseEnter={() => setAt([r, c])}
              onFocus={() => setAt([r, c])}
              onClick={() => onPick(r, c)}
              className={'size-[14px] rounded-[2px] border ' + (on ? 'border-primary-500 bg-primary-100 dark:bg-primary-400/30' : 'border-gray-300 dark:border-white/20')}
            />
          )
        })}
      </div>
      <p className="mt-1.5 text-center text-[12px]">
        {at[0]} × {at[1]}
      </p>
    </div>
  )
}

// Stable props: the bubble menu registers its plugin again (a transaction, so a render)
// whenever one of them changes.
const tableMenuOptions = { placement: 'top', strategy: 'absolute' } as const
const showTableMenu = ({ editor }: { editor: Editor }) => editor.isEditable && editor.isActive('table')

/** CKEditor's table toolbar: column, row and merge menus above the table being edited. */
function TableMenu({ editor, root }: { editor: Editor; root: React.RefObject<HTMLDivElement | null> }) {
  const chain = () => editor.chain().focus()
  const rtl = direction() === 'rtl'
  const appendTo = useRef(() => root.current ?? document.body).current
  return (
    <BubbleMenu editor={editor} pluginKey="tableMenu" shouldShow={showTableMenu} appendTo={appendTo} options={tableMenuOptions} className="z-30">
      <div className="flex items-center gap-[2px] rounded-[3px] border border-gray-200 bg-white p-1 shadow-lg dark:border-white/10 dark:bg-gray-900">
        <Dropdown label="Column" button={<span className="text-[13px]">Column</span>}>
          {(close) => (
            <div className="min-w-48 py-1" role="menu">
              <MenuItem on={editor.isActive('tableHeader')} onClick={() => (chain().toggleHeaderColumn().run(), close())}>
                Header column
              </MenuItem>
              <MenuItem onClick={() => ((rtl ? chain().addColumnAfter() : chain().addColumnBefore()).run(), close())}>Insert column left</MenuItem>
              <MenuItem onClick={() => ((rtl ? chain().addColumnBefore() : chain().addColumnAfter()).run(), close())}>Insert column right</MenuItem>
              <MenuItem onClick={() => (chain().deleteColumn().run(), close())}>Delete column</MenuItem>
            </div>
          )}
        </Dropdown>
        <Dropdown label="Row" button={<span className="text-[13px]">Row</span>}>
          {(close) => (
            <div className="min-w-48 py-1" role="menu">
              <MenuItem onClick={() => (chain().toggleHeaderRow().run(), close())}>Header row</MenuItem>
              <MenuItem onClick={() => (chain().addRowBefore().run(), close())}>Insert row above</MenuItem>
              <MenuItem onClick={() => (chain().addRowAfter().run(), close())}>Insert row below</MenuItem>
              <MenuItem onClick={() => (chain().deleteRow().run(), close())}>Delete row</MenuItem>
            </div>
          )}
        </Dropdown>
        <Dropdown label="Merge cells" button={<span className="text-[13px]">Merge cells</span>}>
          {(close) => (
            <div className="min-w-48 py-1" role="menu">
              <MenuItem disabled={!editor.can().mergeCells()} onClick={() => (chain().mergeCells().run(), close())}>
                Merge cells
              </MenuItem>
              <MenuItem disabled={!editor.can().splitCell()} onClick={() => (chain().splitCell().run(), close())}>
                Split cell
              </MenuItem>
            </div>
          )}
        </Dropdown>
        <Sep />
        <Btn label="Delete table" onClick={() => chain().deleteTable().run()}>
          {icons.cancel}
        </Btn>
      </div>
    </BubbleMenu>
  )
}
