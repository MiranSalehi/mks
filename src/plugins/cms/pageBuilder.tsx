import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { ApiError, createPortal, defineField, Modal, moveBlock, PanelForm, tr, usePanel, type CustomFieldProps, type FieldErrors, type FormBuilder, type SchemaNode } from '../../index'
import { allBlockDefs, field, presentation, type BlockDef, type FieldDef } from './builder'
import { BlockIcon, blockCategories, blockMeta } from './blockMeta'

// The page builder, laid out like MKSine's: a toolbar (templates, undo/redo, full screen,
// add component), device widths, and a canvas that is the real page drawn by the server
// with each block framed (its name, ✎ ↑ ↓ ⧉ ×) and "+" rows between blocks and inside
// every column. A click edits a block in a side panel; dragging its name moves it,
// between columns too. The frame runs no scripts: this component wires it.

const L = (fa: string, en: string) => tr({ fa, en })

interface PBBlock {
  type: string
  data: Record<string, any>
}

const HISTORY = 50

// --- paths: "2" is block 2 of the page, "2.0.1" block 1 of slot 0 of block 2 ------------

export function listOf(blocks: PBBlock[], list: string): PBBlock[] | null {
  if (list === '') return blocks
  const segs = list.split('.').map(Number)
  let cur: PBBlock[] = blocks
  for (let i = 0; i < segs.length; i += 2) {
    const b = cur[segs[i]]
    const slot = b?.data?.children?.[segs[i + 1]]
    if (!slot) return null
    cur = Array.isArray(slot.blocks) ? slot.blocks : []
  }
  return cur
}

export function withList(blocks: PBBlock[], list: string, next: PBBlock[]): PBBlock[] {
  if (list === '') return next
  const segs = list.split('.').map(Number)
  const [i, slot, ...rest] = segs
  return blocks.map((b, j) => {
    if (j !== i) return b
    const children = [...(b.data.children ?? [])]
    const inner = Array.isArray(children[slot]?.blocks) ? children[slot].blocks : []
    children[slot] = { ...children[slot], blocks: withList(inner, rest.join('.'), next) }
    return { ...b, data: { ...b.data, children } }
  })
}

function splitPath(path: string): { list: string; index: number } {
  const segs = path.split('.')
  return { list: segs.slice(0, -1).join('.'), index: Number(segs[segs.length - 1]) }
}

export function blockAt(blocks: PBBlock[], path: string): PBBlock | undefined {
  const { list, index } = splitPath(path)
  return listOf(blocks, list)?.[index]
}

export function replaceAt(blocks: PBBlock[], path: string, block: PBBlock | null, extra: PBBlock[] = []): PBBlock[] {
  const { list, index } = splitPath(path)
  const items = [...(listOf(blocks, list) ?? [])]
  items.splice(index, 1, ...(block ? [block] : []), ...extra)
  return withList(blocks, list, items)
}

export function insertAt(blocks: PBBlock[], list: string, index: number, block: PBBlock): PBBlock[] {
  const items = [...(listOf(blocks, list) ?? [])]
  items.splice(Math.max(0, Math.min(index, items.length)), 0, block)
  return withList(blocks, list, items)
}

/** The server's error key prefix of a block: "builder_payload.1.data.children.0.blocks.2". */
function errorPrefix(name: string, path: string): string {
  const segs = path.split('.')
  let out = `${name}.${segs[0]}`
  for (let i = 1; i < segs.length; i += 2) out += `.data.children.${segs[i]}.blocks.${segs[i + 1]}`
  return out
}

/** The canvas path of an error key, and the rest of the key. */
function errorPath(name: string, key: string): { path: string; field: string } | null {
  if (!key.startsWith(name + '.')) return null
  const m = key.slice(name.length + 1).match(/^(\d+)((?:\.data\.children\.\d+\.blocks\.\d+)*)(?:\.data\.(.+))?$/)
  if (!m) return null
  const nested = [...m[2].matchAll(/\.data\.children\.(\d+)\.blocks\.(\d+)/g)].map((x) => `${x[1]}.${x[2]}`)
  return { path: [m[1], ...nested].join('.'), field: m[3] ?? '' }
}

/** A new block of a type: its fields' defaults, and empty slots for containers. */
function newBlock(def: BlockDef): PBBlock {
  const data: Record<string, any> = {}
  for (const f of [...def.fields, ...presentation]) if (f.default !== undefined) data[f.name] = f.default
  if (def.slots) data.children = Array.from({ length: Math.max(1, def.minSlots ?? 1) }, () => ({ blocks: [] }))
  return { type: def.type, data }
}

/** A deep copy (duplicate). */
const clone = (b: PBBlock): PBBlock => JSON.parse(JSON.stringify(b))

// --- icons -------------------------------------------------------------------------------

const I = ({ d, className = 'size-4' }: { d: string; className?: string }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
    <path d={d} />
  </svg>
)
const icons = {
  squares: 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
  sparkles: 'M12 3l2 5 5 2-5 2-2 5-2-5-5-2 5-2zM19 15l1 2 2 1-2 1-1 2-1-2-2-1 2-1z',
  undo: 'M9 14L4 9l5-5M4 9h10a6 6 0 0 1 0 12h-3',
  redo: 'M15 14l5-5-5-5M20 9H10a6 6 0 0 0 0 12h3',
  expand: 'M4 9V4h5M20 9V4h-5M4 15v5h5M20 15v5h-5',
  shrink: 'M9 4v5H4M15 4v5h5M9 20v-5H4M15 20v-5h5',
  plus: 'M12 5v14M5 12h14',
  close: 'M6 6l12 12M18 6 6 18',
  warn: 'M12 9v4M12 16.5v.01M10.3 4.2 2.6 18a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0z',
}

const outlineBtn =
  'inline-flex h-8 items-center gap-1.5 rounded-lg border border-gray-200 bg-white px-3 text-xs font-semibold text-gray-700 hover:bg-gray-50 disabled:opacity-50 dark:border-white/10 dark:bg-white/5 dark:text-gray-200 dark:hover:bg-white/10'
const primaryBtn =
  'inline-flex h-8 items-center gap-1.5 rounded-lg bg-primary-600 px-3 text-xs font-semibold text-white shadow-[0_2px_8px_color-mix(in_oklab,var(--color-primary-600)_30%,transparent)] hover:bg-primary-700 disabled:opacity-50'

// --- the canvas's look (injected into the frame, in the panel's colors) ----------------

function canvasCSS(): string {
  const root = getComputedStyle(document.documentElement)
  const p = (n: number) => root.getPropertyValue(`--color-primary-${n}`).trim() || '#f97316'
  return `
main{width:auto!important;max-width:none!important;margin:8px!important}
.topline{display:none}
.mks-list{display:flow-root;min-height:4px}
.mks-block{margin:8px;outline:1px solid #e5e7eb;background:#fff;display:flow-root;cursor:pointer;position:relative}
.mks-block:hover,.mks-block:focus-within{outline-color:${p(400)}}
.mks-block:has(.mks-block:hover),.mks-block:has(.mks-insert:hover){outline-color:#e5e7eb}
.mks-block.is-selected{outline:2px solid ${p(700)}}
.mks-tools{display:flex;align-items:center;gap:3px;height:26px;padding:0 8px;background:#f9fafb;color:#4b5563;border-bottom:1px solid #e5e7eb;font:600 12px/26px system-ui,sans-serif}
.mks-block.is-selected>.mks-tools{background:${p(50)};color:${p(800)};border-color:${p(200)}}
.mks-name{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;cursor:grab;user-select:none}
.mks-tools button{display:none;width:18px;height:18px;padding:0;border:0;border-radius:6px;background:${p(100)};color:${p(800)};font:700 13px/18px system-ui,sans-serif;cursor:pointer}
.mks-block:hover>.mks-tools button,.mks-block:focus-within>.mks-tools button,.mks-block.is-selected>.mks-tools button{display:inline-block}
.mks-block:has(.mks-block:hover)>.mks-tools button{display:none}
.mks-tools button:hover{background:${p(200)}}
.mks-tools button[data-pb-act=delete]:hover{color:#b91c1c}
.mks-insert{position:relative;display:flex;align-items:center;justify-content:center;height:16px;margin:0 16px;cursor:pointer;outline:0}
.mks-insert::before{content:"";position:absolute;inset-inline:0;top:50%;height:1px;background:color-mix(in srgb,${p(400)} 55%,transparent)}
.mks-insert-pill{position:relative;display:inline-flex;align-items:center;justify-content:center;gap:6px;min-width:18px;height:18px;border-radius:999px;background:${p(50)};color:${p(500)};box-shadow:inset 0 0 0 1px ${p(200)};font:600 12px/1 system-ui,sans-serif}
.mks-plus{display:inline-grid;place-items:center;width:16px;height:16px;border-radius:999px}
.mks-insert-label{display:none;white-space:nowrap}
.mks-insert:hover,.mks-insert:focus-visible,.mks-insert.is-visible{height:32px}
.mks-insert:hover::before,.mks-insert:focus-visible::before{background:color-mix(in srgb,${p(600)} 70%,transparent)}
.mks-insert:hover .mks-insert-pill,.mks-insert:focus-visible .mks-insert-pill{height:auto;padding:6px 10px;background:${p(600)};color:#fff;box-shadow:none}
.mks-insert:hover .mks-plus,.mks-insert:focus-visible .mks-plus{background:#fff;color:${p(600)}}
.mks-insert.is-visible .mks-insert-pill{height:auto;padding:6px 10px}
.mks-insert:hover .mks-insert-label,.mks-insert:focus-visible .mks-insert-label,.mks-insert.is-visible .mks-insert-label{display:inline}
.mks-dragging{opacity:.35;outline-style:dashed!important;pointer-events:none}
.mks-drop-zone{outline:2px solid ${p(600)};outline-offset:2px}
.mks-drop-line{height:4px;margin:2px 8px;border-radius:2px;background:${p(700)};box-shadow:0 0 0 4px ${p(100)}}
.mks-ghost{position:fixed;z-index:2147483647;pointer-events:none;max-width:240px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;padding:4px 10px;border-radius:8px;background:${p(700)};color:#fff;font:600 12px system-ui,sans-serif}
body.mks-grabbing,body.mks-grabbing *{cursor:grabbing!important}
`
}

const devices = {
  desktop: { label: L('دسکتاپ', 'Desktop'), width: '100%' },
  tablet: { label: L('تبلت', 'Tablet'), width: '768px' },
  mobile: { label: L('موبایل', 'Mobile'), width: '390px' },
} as const

type Device = keyof typeof devices

// --- templates (the same keys as cms.PageTemplates) --------------------------------------

const templates: { key: string; name: string; description: string; category: string }[] = [
  { key: 'landing', name: L('صفحهٔ فرود', 'Landing page'), description: L('هیرو، ویژگی‌ها، نظر مشتریان، پرسش‌ها و فراخوان اقدام.', 'Hero, features, testimonials, FAQ and a call to action.'), category: 'marketing' },
  { key: 'about', name: L('دربارهٔ ما', 'About us'), description: L('عنوان، معرفی و دو ستون مأموریت و چشم‌انداز.', 'Title, story and two columns: mission and vision.'), category: 'general' },
  { key: 'contact', name: L('تماس با ما', 'Contact'), description: L('نشانی و راه‌های تماس در دو کارت.', 'Address and contact options in two cards.'), category: 'general' },
  { key: 'services', name: L('خدمات', 'Services'), description: L('هیرو، سه خدمت، تب‌های روش کار و فراخوان اقدام.', 'Hero, three services, tabs and a call to action.'), category: 'business' },
  { key: 'blank', name: L('صفحهٔ خالی', 'Blank page'), description: L('بدون هیچ بلوکی شروع کنید.', 'Start with no blocks.'), category: 'general' },
  { key: 'mksine_home', name: L('خانهٔ پیش‌فرض MKSine', 'MKSine default home'), description: L('صفحهٔ اصلی پوستهٔ MKSine با بخش‌های آن.', "The MKSine theme's home with its sections."), category: 'marketing' },
]
const templateCategories: Record<string, string> = { general: L('عمومی', 'General'), marketing: L('بازاریابی', 'Marketing'), business: L('کسب‌وکار', 'Business') }

// --- the add-component dialog ------------------------------------------------------------

function AddDialog({ onPick, onClose }: { onPick: (def: BlockDef) => void; onClose: () => void }) {
  const defs = allBlockDefs()
  const [query, setQuery] = useState('')
  const cats = useMemo(() => {
    const known = blockCategories.filter((c) => defs.some((d) => d.category === c.key))
    const extra = [...new Set(defs.map((d) => d.category))].filter((c) => !blockCategories.some((k) => k.key === c)).map((c) => ({ key: c, label: c, icon: 'puzzle' }))
    return [...known, ...extra]
  }, [defs])
  const [tab, setTab] = useState(cats[0]?.key ?? 'content')
  const q = query.trim().toLowerCase()
  const shown = q
    ? defs.filter((d) => [d.label, blockMeta[d.type]?.description ?? '', d.type].some((s) => s.toLowerCase().includes(q)))
    : defs.filter((d) => d.category === tab)
  return (
    <Modal
      title={L('افزودن کامپوننت', 'Add component')}
      size="xl"
      onClose={onClose}
      footer={
        <div className="flex justify-end">
          <button type="button" onClick={onClose} className={outlineBtn}>
            {L('انصراف', 'Cancel')}
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        <input
          autoFocus
          type="search"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          placeholder={L('جستجوی کامپوننت', 'Search components')}
          className="block w-full rounded-xl border-0 bg-white px-4 py-3 text-sm ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5 dark:ring-white/10"
        />
        {!q && (
          <div role="tablist" className="flex flex-wrap gap-1 rounded-lg bg-gray-100 p-1 dark:bg-white/5">
            {cats.map((c) => (
              <button
                key={c.key}
                role="tab"
                type="button"
                aria-selected={tab === c.key}
                onClick={() => setTab(c.key)}
                className={
                  'inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-semibold ' +
                  (tab === c.key ? 'bg-white text-gray-900 shadow-sm dark:bg-white/10 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400')
                }
              >
                <BlockIcon name={c.icon} className="size-3.5" />
                {c.label}
              </button>
            ))}
          </div>
        )}
        <div className="max-h-[min(28rem,calc(100vh-14rem))] min-h-48 overflow-y-auto rounded-xl border border-gray-200 bg-gray-50/50 p-3.5 dark:border-white/10 dark:bg-white/[0.02]">
          {shown.length === 0 ? (
            <p className="py-10 text-center text-sm text-gray-500">
              {q ? L('کامپوننتی با این نام پیدا نشد.', 'No component matches.') : L('در این دسته کامپوننتی وجود ندارد.', 'This category is empty.')}
            </p>
          ) : (
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {shown.map((d) => (
                <button
                  key={d.type}
                  type="button"
                  onClick={() => onPick(d)}
                  className="group flex items-start gap-3 rounded-xl border border-gray-200 bg-white p-3.5 text-start hover:border-primary-200 hover:shadow-[0_4px_12px_color-mix(in_oklab,var(--color-primary-600)_10%,transparent)] dark:border-white/10 dark:bg-gray-900"
                >
                  <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 group-hover:bg-primary-50 group-hover:text-primary-600 dark:bg-white/5">
                    <BlockIcon name={blockMeta[d.type]?.icon ?? 'puzzle'} />
                  </span>
                  <span className="min-w-0">
                    <span className="block text-[13px] font-semibold text-gray-900 dark:text-white">{d.label}</span>
                    <span className="mt-0.5 line-clamp-2 block text-[11px] text-gray-400">{blockMeta[d.type]?.description ?? ''}</span>
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    </Modal>
  )
}

function TemplatesDialog({ onPick, onClose }: { onPick: (key: string) => void; onClose: () => void }) {
  const groups = Object.entries(templateCategories).map(([key, label]) => ({ key, label, items: templates.filter((t) => t.category === key) }))
  return (
    <Modal title={L('انتخاب قالب', 'Choose a template')} size="xl" onClose={onClose}>
      <div className="space-y-5">
        <p className="text-sm text-gray-600 dark:text-gray-300">{L('شروع با چیدمان ازپیش‌ساخته', 'Start from a ready-made layout')}</p>
        <div className="flex items-start gap-2 rounded-lg bg-warning-50 p-3 text-sm text-warning-800 ring-1 ring-warning-600/20 dark:bg-warning-500/10 dark:text-warning-300">
          <I d={icons.warn} className="mt-0.5 size-4 shrink-0" />
          {L('بارگذاری قالب محتوای فعلی را جایگزین می‌کند. ابتدا تغییرات را ذخیره کنید.', 'A template replaces the current content. Save your changes first.')}
        </div>
        {groups.map((g) => (
          <section key={g.key}>
            <h3 className="mb-2 text-[10px] font-semibold tracking-wider text-gray-400 uppercase">{g.label}</h3>
            <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              {g.items.map((t) => (
                <button key={t.key} type="button" onClick={() => onPick(t.key)} className="group flex items-start gap-3 rounded-xl border border-gray-200 bg-white p-3.5 text-start hover:border-primary-200 dark:border-white/10 dark:bg-gray-900">
                  <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 group-hover:bg-primary-50 group-hover:text-primary-600 dark:bg-white/5">
                    <I d={icons.squares} className="size-5" />
                  </span>
                  <span className="min-w-0">
                    <span className="block text-[13px] font-semibold">{t.name}</span>
                    <span className="mt-0.5 line-clamp-2 block text-[11px] text-gray-400">{t.description}</span>
                    <span className="mt-1 hidden text-[11px] font-semibold text-primary-600 group-hover:block">{L('استفاده از قالب ←', 'Use template →')}</span>
                  </span>
                </button>
              ))}
            </div>
          </section>
        ))}
      </div>
    </Modal>
  )
}

// --- the block editor (side panel) ---------------------------------------------------------

function slotOptions(def: BlockDef): Record<string, string> | null {
  if (!def.slots || def.slots <= 1) return null
  const out: Record<string, string> = {}
  for (let n = def.minSlots ?? 1; n <= def.slots; n++) out[String(n)] = String(n)
  return out
}

function EditorPanel({
  block,
  def,
  errors,
  onSave,
  onClose,
}: {
  block: PBBlock
  def: BlockDef
  errors: FieldErrors
  onSave: (data: Record<string, any>) => void
  onClose: () => void
}) {
  const slots = slotOptions(def)
  const count = Array.isArray(block.data.children) ? block.data.children.length : 0
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [onClose])
  const content = (f: FormBuilder<any>): SchemaNode<any>[] => [
    ...(slots
      ? [
          f.select('__slots')
            .label(def.type === 'card' ? L('بخش‌ها', 'Parts') : L('تعداد ستون‌ها', 'Number of columns'))
            .options(slots)
            .default(String(count || def.minSlots || 1))
            .columnSpan('full'),
        ]
      : []),
    ...def.fields.map((fd: FieldDef) => field(f, fd)),
  ]
  // On the body, not inside the page's form: a form may not sit in another.
  return createPortal(
    <div className="fixed inset-0 z-50" role="dialog" aria-modal="true" aria-label={def.label}>
      <div className="absolute inset-0 bg-gray-950/40 backdrop-blur-[2px]" onClick={onClose} aria-hidden="true" />
      <div className="absolute inset-y-0 end-0 flex w-full max-w-lg flex-col bg-white shadow-2xl animate-in dark:bg-gray-900">
        <div className="flex items-center justify-between gap-3 border-b border-gray-100 px-5 py-4 dark:border-white/10">
          <h2 className="text-base font-semibold">{def.label || L('ویرایش کامپوننت', 'Edit component')}</h2>
          <button type="button" onClick={onClose} aria-label={L('بستن', 'Close')} className="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-white/10">
            <I d={icons.close} />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-5 py-4">
          <PanelForm
            initial={{ ...block.data, __slots: String(count || def.minSlots || 1) }}
            initialErrors={errors}
            schema={(f) => [
              f.tabs([
                f.tab(L('محتوا', 'Content')).schema(content(f)),
                f.tab(L('ظاهر بلوک', 'Block style')).schema(presentation.map((fd) => field(f, fd))),
              ]),
            ]}
            onSubmit={async (values) => {
              const { __slots, ...data } = values as Record<string, any>
              const next: Record<string, any> = { ...block.data, ...data }
              if (slots) {
                const want = Number(__slots) || count
                const children = [...(block.data.children ?? [])]
                const dropped = children.slice(want).some((s: any) => Array.isArray(s?.blocks) && s.blocks.length > 0)
                if (dropped && !window.confirm(L('بخش‌های حذف‌شده کامپوننت دارند و پاک می‌شوند. ادامه می‌دهید؟', 'Removed parts hold components that will be deleted. Continue?'))) return
                while (children.length < want) children.push({ blocks: [] })
                next.children = children.slice(0, want)
              }
              onSave(next)
            }}
            actions={({ saving }) => (
              <div className="sticky bottom-0 -mx-5 flex justify-end gap-2 border-t border-gray-100 bg-white px-5 py-3 dark:border-white/10 dark:bg-gray-900">
                <button type="button" onClick={onClose} className={outlineBtn}>
                  {L('انصراف', 'Cancel')}
                </button>
                <button type="submit" disabled={saving} className={primaryBtn}>
                  {L('ذخیره تغییرات', 'Save changes')}
                </button>
              </div>
            )}
          />
        </div>
      </div>
    </div>,
    document.body,
  )
}

// --- the builder -------------------------------------------------------------------------

interface Options {
  /** The other field holding the content width ("contained", "wide", "full"), if any. */
  widthField?: string
}

function PageBuilderControl({ value, onChange, errors, get, options }: CustomFieldProps<PBBlock[] | null, Options>) {
  const { api, notify } = usePanel()
  const blocks = useMemo(() => (Array.isArray(value) ? value : []), [value])
  const name = 'builder_payload'
  const blocksRef = useRef(blocks)
  blocksRef.current = blocks
  const past = useRef<PBBlock[][]>([])
  const future = useRef<PBBlock[][]>([])
  const [, bump] = useState(0)
  const [device, setDevice] = useState<Device>('desktop')
  const [full, setFull] = useState(false)
  const [picker, setPicker] = useState<{ list: string; index: number } | null>(null)
  const [choosingTemplate, setChoosingTemplate] = useState(false)
  const [editing, setEditing] = useState<string | null>(null)
  const [html, setHtml] = useState('')
  const [busy, setBusy] = useState(false)
  const [previewErrors, setPreviewErrors] = useState<FieldErrors>({})
  const frame = useRef<HTMLIFrameElement>(null)
  const scroll = useRef(0)
  const defs = allBlockDefs()
  const defOf = useCallback((type: string) => allBlockDefs().find((d) => d.type === type), [])
  const labels = useMemo(() => Object.fromEntries(defs.map((d) => [d.type, d.label])), [defs])

  /** Every change goes through here: it is one undo step. */
  const commit = useCallback(
    (next: PBBlock[]) => {
      past.current = [...past.current.slice(-(HISTORY - 1)), blocksRef.current]
      future.current = []
      onChange(next)
      bump((n) => n + 1)
    },
    [onChange],
  )
  const undo = useCallback(() => {
    const prev = past.current.pop()
    if (!prev) return
    future.current.push(blocksRef.current)
    onChange(prev)
    bump((n) => n + 1)
  }, [onChange])
  const redo = useCallback(() => {
    const next = future.current.pop()
    if (!next) return
    past.current.push(blocksRef.current)
    onChange(next)
    bump((n) => n + 1)
  }, [onChange])

  // Undo and redo from the keyboard, unless typing in a field.
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const typing = (e.target as HTMLElement | null)?.closest?.('input,textarea,select,[contenteditable=true]')
      if (typing || editing || picker || !(e.metaKey || e.ctrlKey)) return
      const k = e.key.toLowerCase()
      if (k === 'z' && !e.shiftKey) {
        e.preventDefault()
        undo()
      } else if ((k === 'z' && e.shiftKey) || k === 'y') {
        e.preventDefault()
        redo()
      } else if (k === 'escape' && full) setFull(false)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [undo, redo, editing, picker, full])
  useEffect(() => {
    if (!full) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && !editing && !picker && setFull(false)
    window.addEventListener('keydown', onKey)
    document.body.style.overflow = 'hidden'
    return () => {
      window.removeEventListener('keydown', onKey)
      document.body.style.overflow = ''
    }
  }, [full, editing, picker])

  // The canvas: the server draws the unsaved blocks (300ms after the last change).
  const width = options.widthField ? get?.(options.widthField) : undefined
  const json = JSON.stringify(blocks)
  useEffect(() => {
    if (blocks.length === 0) {
      setHtml('')
      return
    }
    const timer = window.setTimeout(() => {
      setBusy(true)
      api
        .plugin<{ html: string; errors?: FieldErrors }>('cms', '/preview', {
          method: 'POST',
          body: JSON.stringify({ payload: JSON.parse(json), width, canvas: true, labels }),
        })
        .then((r) => {
          scroll.current = frame.current?.contentWindow?.scrollY ?? scroll.current
          setHtml(r.html)
          // The server sends one message per field; the form wants a list.
          setPreviewErrors(Object.fromEntries(Object.entries(r.errors ?? {}).map(([k, v]) => [k, Array.isArray(v) ? v : [String(v)]])))
        })
        .catch((e: unknown) => notify(e instanceof ApiError ? e.message : String(e), 'danger'))
        .finally(() => setBusy(false))
    }, 300)
    return () => window.clearTimeout(timer)
    // labels change only when plugins do
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [json, width])

  // Every error, by block path (the save's and the canvas's).
  const allErrors = useMemo(() => ({ ...previewErrors, ...(errors ?? {}) }), [previewErrors, errors])
  const errorsByBlock = useMemo(() => {
    const out: Record<string, FieldErrors> = {}
    for (const [key, msgs] of Object.entries(allErrors)) {
      const at = errorPath(name, key)
      if (!at) continue
      ;(out[at.path] ??= {})[at.field || '_'] = msgs
    }
    return out
  }, [allErrors])

  // Actions from the canvas.
  const act = useCallback(
    (action: string, path: string) => {
      const cur = blocksRef.current
      const { list, index } = splitPath(path)
      const items = listOf(cur, list) ?? []
      const b = items[index]
      if (!b) return
      if (action === 'edit') setEditing(path)
      if (action === 'delete') {
        commit(replaceAt(cur, path, null))
        if (editing === path) setEditing(null)
      }
      if (action === 'copy') commit(replaceAt(cur, path, b, [clone(b)]))
      if (action === 'up' && index > 0) {
        const next = [...items]
        ;[next[index - 1], next[index]] = [next[index], next[index - 1]]
        commit(withList(cur, list, next))
      }
      if (action === 'down' && index < items.length - 1) {
        const next = [...items]
        ;[next[index + 1], next[index]] = [next[index], next[index + 1]]
        commit(withList(cur, list, next))
      }
    },
    [commit, editing],
  )
  const actRef = useRef(act)
  actRef.current = act

  /** Wires the frame after each load: its look, clicks, keys and dragging. */
  function wire() {
    const doc = frame.current?.contentDocument
    const win = frame.current?.contentWindow
    if (!doc || !win) return
    const style = doc.createElement('style')
    style.textContent = canvasCSS()
    doc.head?.appendChild(style)
    if (scroll.current) win.scrollTo(0, scroll.current)
    if (editing) doc.querySelector(`[data-pb-path="${editing}"]`)?.classList.add('is-selected')
    let dragged = false
    doc.addEventListener('click', (e) => {
      e.preventDefault()
      if (dragged) {
        dragged = false
        return
      }
      const t = e.target as Element
      const button = t.closest('[data-pb-act]')
      const block = t.closest('[data-pb-path]')
      const insert = t.closest('[data-pb-insert]')
      if (insert) {
        const [list, index] = (insert.getAttribute('data-pb-insert') ?? '|0').split('|')
        setPicker({ list, index: Number(index) })
      } else if (button && block) {
        actRef.current(button.getAttribute('data-pb-act') ?? '', block.getAttribute('data-pb-path') ?? '')
      } else if (block) {
        actRef.current('edit', block.getAttribute('data-pb-path') ?? '')
      }
    })
    doc.addEventListener('keydown', (e) => {
      const insert = (e.target as Element).closest?.('[data-pb-insert]')
      if (insert && (e.key === 'Enter' || e.key === ' ')) {
        e.preventDefault()
        ;(insert as HTMLElement).click()
      }
    })
    // Dragging a block by its name, anywhere: the page or any column.
    doc.addEventListener('pointerdown', (e) => {
      const handle = (e.target as Element).closest('[data-pb-drag]')
      const block = handle?.closest('.mks-block') as HTMLElement | null
      if (!handle || !block) return
      const from = block.getAttribute('data-pb-path') ?? ''
      const sx = e.clientX
      const sy = e.clientY
      let started = false
      let ghost: HTMLElement | null = null
      let line: HTMLElement | null = null
      let zone: Element | null = null
      let target: { list: string; index: number } | null = null
      const move = (ev: PointerEvent) => {
        if (!started) {
          if (Math.hypot(ev.clientX - sx, ev.clientY - sy) < 6) return
          started = true
          block.classList.add('mks-dragging')
          doc.body.classList.add('mks-grabbing')
          ghost = doc.createElement('div')
          ghost.className = 'mks-ghost'
          ghost.textContent = handle.textContent
          doc.body.appendChild(ghost)
          line = doc.createElement('div')
          line.className = 'mks-drop-line'
        }
        ghost!.style.left = `${ev.clientX + 12}px`
        ghost!.style.top = `${ev.clientY + 12}px`
        let z = doc.elementFromPoint(ev.clientX, ev.clientY)?.closest('[data-pb-list]') ?? null
        while (z && block.contains(z)) z = z.parentElement?.closest('[data-pb-list]') ?? null
        if (zone !== z) {
          zone?.classList.remove('mks-drop-zone')
          z?.classList.add('mks-drop-zone')
          zone = z
        }
        if (!z) {
          line!.remove()
          target = null
          return
        }
        const kids = Array.from(z.querySelectorAll(':scope > .mks-block'))
        let index = kids.length
        for (let i = 0; i < kids.length; i++) {
          const r = kids[i].getBoundingClientRect()
          if (ev.clientY < r.top + r.height / 2) {
            index = i
            break
          }
        }
        target = { list: z.getAttribute('data-pb-list') ?? '', index }
        if (index < kids.length) z.insertBefore(line!, kids[index])
        else z.appendChild(line!)
      }
      const up = () => {
        doc.removeEventListener('pointermove', move)
        doc.removeEventListener('pointerup', up)
        if (!started) return
        dragged = true
        block.classList.remove('mks-dragging')
        doc.body.classList.remove('mks-grabbing')
        ghost?.remove()
        line?.remove()
        zone?.classList.remove('mks-drop-zone')
        if (!target) return
        const cur = blocksRef.current
        const item = blockAt(cur, from)
        const parentPath = target.list ? target.list.split('.').slice(0, -1).join('.') : ''
        const owner = target.list ? (blockAt(cur, parentPath) ?? null) : null
        const slot = target.list ? Number(target.list.split('.').pop()) : -1
        if (!item || (target.list && !owner)) return
        const next = moveBlock(cur as any[], item as any, { owner: owner as any, slot }, target.index)
        if (next && next !== cur) commit(next as PBBlock[])
      }
      doc.addEventListener('pointermove', move)
      doc.addEventListener('pointerup', up)
    })
  }

  const editingBlock = editing ? blockAt(blocks, editing) : undefined
  const editingDef = editingBlock ? defOf(editingBlock.type) : undefined
  const errorCount = Object.keys(errorsByBlock).length

  const deviceSwitch = (
    <div className="flex gap-1.5" role="group" aria-label={L('اندازهٔ صفحه', 'Screen size')}>
      {(Object.keys(devices) as Device[]).map((d) => (
        <button
          key={d}
          type="button"
          aria-pressed={device === d}
          onClick={() => setDevice(d)}
          className={'rounded-md px-2.5 py-1 text-xs font-semibold ' + (device === d ? 'bg-primary-600 text-white' : 'bg-white text-gray-600 ring-1 ring-gray-200 hover:bg-gray-50 dark:bg-white/5 dark:text-gray-300 dark:ring-white/10')}
        >
          {devices[d].label}
        </button>
      ))}
    </div>
  )

  return (
    <div className={full ? 'fixed inset-0 z-30 flex flex-col bg-white dark:bg-gray-950' : 'overflow-hidden rounded-xl border border-gray-200/80 bg-white shadow-[0_1px_3px_rgb(0_0_0/0.05)] dark:border-white/10 dark:bg-gray-900'}>
      {/* Toolbar */}
      <div className={'sticky top-0 z-20 border-b border-gray-100 bg-white/95 px-5 py-3 backdrop-blur-md dark:border-white/10 dark:bg-gray-900/95 ' + (full ? 'grid grid-cols-3 items-center gap-3' : 'flex flex-wrap items-center justify-between gap-3')}>
        <div className="flex items-center gap-3">
          {full ? (
            <button type="button" onClick={() => setFull(false)} className={outlineBtn} title={L('خروج از تمام‌صفحه', 'Exit full screen')} aria-label={L('خروج از تمام‌صفحه', 'Exit full screen')}>
              <I d={icons.shrink} />
            </button>
          ) : (
            <span className="flex size-9 items-center justify-center rounded-lg bg-primary-600 text-white shadow-[0_4px_12px_color-mix(in_oklab,var(--color-primary-600)_35%,transparent)]">
              <I d={icons.squares} className="size-5" />
            </span>
          )}
          <div>
            <p className="text-[13px] font-semibold">{full ? String(get?.('title') ?? L('صفحه‌ساز', 'Page builder')) : L('صفحه‌ساز', 'Page builder')}</p>
            <p className="text-[11px] text-gray-400 tabular-nums">{L(`${blocks.length.toLocaleString('fa')} کامپوننت`, `${blocks.length} components`)}</p>
          </div>
        </div>
        {full && <div className="flex justify-center">{deviceSwitch}</div>}
        <div className="flex flex-wrap items-center justify-end gap-2">
          <button type="button" className={outlineBtn + ' max-sm:hidden'} title={L('شروع با یک قالب ازپیش‌طراحی‌شده', 'Start from a ready-made template')} onClick={() => setChoosingTemplate(true)}>
            <span className="text-primary-500">
              <I d={icons.sparkles} />
            </span>
            {L('استفاده از قالب', 'Use template')}
          </button>
          <div className="flex h-8 items-center rounded-lg border border-gray-200 dark:border-white/10">
            <button type="button" onClick={undo} disabled={past.current.length === 0} title={L('بازگردانی (Cmd+Z)', 'Undo (Cmd+Z)')} aria-label={L('بازگردانی', 'Undo')} className="flex size-8 items-center justify-center text-gray-600 disabled:cursor-not-allowed disabled:text-gray-300 dark:text-gray-300 dark:disabled:text-gray-600">
              <I d={icons.undo} />
            </button>
            <span className="h-4 w-px bg-gray-200 dark:bg-white/10" />
            <button type="button" onClick={redo} disabled={future.current.length === 0} title={L('جلوبری (Cmd+Shift+Z)', 'Redo (Cmd+Shift+Z)')} aria-label={L('جلوبری', 'Redo')} className="flex size-8 items-center justify-center text-gray-600 disabled:cursor-not-allowed disabled:text-gray-300 dark:text-gray-300 dark:disabled:text-gray-600">
              <I d={icons.redo} />
            </button>
          </div>
          {!full && (
            <button type="button" className={outlineBtn} onClick={() => setFull(true)}>
              <I d={icons.expand} />
              <span className="max-sm:hidden">{L('تمام‌صفحه', 'Full screen')}</span>
            </button>
          )}
          <button type="button" className={primaryBtn} onClick={() => setPicker({ list: '', index: blocks.length })}>
            <I d={icons.plus} />
            {L('افزودن کامپوننت', 'Add component')}
          </button>
        </div>
      </div>

      {/* Stage */}
      <div className={(full ? 'flex flex-1 flex-col bg-gray-100 p-4 dark:bg-black/30' : 'bg-gray-50 px-4 py-5 dark:bg-white/[0.02]') + ' space-y-3'}>
        {!full && <div className="flex justify-start">{deviceSwitch}</div>}
        {errorCount > 0 && (
          <div role="alert" className="rounded-lg bg-danger-50 p-3 text-sm text-danger-700 ring-1 ring-danger-600/15 dark:bg-danger-500/10 dark:text-danger-300">
            <p className="font-semibold">{L(`${errorCount.toLocaleString('fa')} کامپوننت خطا دارد:`, `${errorCount} components have errors:`)}</p>
            <ul className="mt-1 space-y-0.5">
              {Object.entries(errorsByBlock).map(([path, errs]) => (
                <li key={path}>
                  <button type="button" className="underline underline-offset-2" onClick={() => setEditing(path)}>
                    {labels[blockAt(blocks, path)?.type ?? ''] ?? path}
                  </button>
                  {': '}
                  {Object.values(errs)[0]?.[0]}
                </li>
              ))}
            </ul>
          </div>
        )}
        {blocks.length === 0 ? (
          <button
            type="button"
            onClick={() => setPicker({ list: '', index: 0 })}
            className="flex w-full items-center gap-4 rounded-xl border border-primary-200 bg-gradient-to-l from-primary-50 to-white p-5 text-start rtl:bg-gradient-to-r dark:from-primary-500/10 dark:to-transparent dark:border-primary-500/30"
          >
            <span className="flex size-10 items-center justify-center rounded-xl bg-white text-primary-600 shadow-sm dark:bg-white/10">
              <I d={icons.squares} className="size-5" />
            </span>
            <span className="flex-1">
              <span className="block text-sm font-semibold">{L('صفحهٔ شما خالی است', 'Your page is empty')}</span>
              <span className="block text-xs text-gray-500">{L('با افزودن اولین کامپوننت شروع کنید', 'Start by adding the first component')}</span>
            </span>
            <span className={primaryBtn + ' rounded-full'}>
              <I d={icons.plus} />
              {L('افزودن کامپوننت', 'Add component')}
            </span>
          </button>
        ) : (
          <div className="relative" style={{ width: devices[device].width, maxWidth: '100%', marginInline: 'auto', transition: 'width 150ms' }}>
            <iframe
              ref={frame}
              title={L('صفحه‌ساز', 'Page builder')}
              sandbox="allow-same-origin"
              srcDoc={html}
              onLoad={wire}
              className={'block w-full rounded-xl border border-gray-200 bg-white dark:border-white/10 ' + (full ? 'shadow-[0_12px_40px_rgb(0_0_0/0.08)]' : '')}
              style={{ height: full ? 'calc(100vh - 9rem)' : 'min(860px, calc(100vh - 180px))', minHeight: full ? undefined : 640 }}
            />
            {busy && (
              <div className="absolute inset-0 flex items-center justify-center rounded-xl bg-white/55 dark:bg-gray-900/55" aria-hidden="true">
                <span className="flex size-10 items-center justify-center rounded-full bg-white shadow-md dark:bg-gray-800">
                  <span className="size-5 animate-spin rounded-full border-2 border-primary-500/25 border-t-primary-600" />
                </span>
              </div>
            )}
          </div>
        )}
      </div>

      {picker && (
        <AddDialog
          onClose={() => setPicker(null)}
          onPick={(def) => {
            commit(insertAt(blocksRef.current, picker.list, picker.index, newBlock(def)))
            setPicker(null)
          }}
        />
      )}
      {choosingTemplate && (
        <TemplatesDialog
          onClose={() => setChoosingTemplate(false)}
          onPick={async (key) => {
            try {
              const r = await api.plugin<{ templates: Record<string, PBBlock[]> }>('cms', '/templates')
              commit(r.templates[key] ?? [])
              setChoosingTemplate(false)
            } catch (e) {
              notify(e instanceof Error ? e.message : String(e), 'danger')
            }
          }}
        />
      )}
      {editing && editingBlock && editingDef && (
        <EditorPanel
          key={editing}
          block={editingBlock}
          def={editingDef}
          errors={Object.fromEntries(Object.entries(errorsByBlock[editing] ?? {}).filter(([k]) => k !== '_'))}
          onClose={() => setEditing(null)}
          onSave={(data) => {
            commit(replaceAt(blocksRef.current, editing, { ...editingBlock, data }))
            setEditing(null)
          }}
        />
      )}
    </div>
  )
}

/** The page builder field (MKSine's): blocks of `{type, data}` with nested slots. */
export const pageBuilder = defineField<PBBlock[] | null, Options>({ component: PageBuilderControl, defaults: {}, empty: [] })

export { errorPath as pageBuilderErrorPath, errorPrefix as pageBuilderErrorPrefix }
