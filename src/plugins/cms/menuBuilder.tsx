import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { ApiError, definePage, direction, formatNumber, Modal, tr, useLocation, useNavigate, usePanel } from '../../index'
import type { Menu, MenuItem } from './types'

// The menu builder (MKSine's MenuBuilder page): sources on one side (custom link,
// categories, tags, pages, posts), the menu as a flat list on the other. Rows are dragged
// to reorder; dragging sideways, or the ←/→ buttons, nests them. Nothing is saved until
// "Save menu".

const L = (fa: string, en: string) => tr({ fa, en })

/** Deepest level a row may sit at (0 is the top): six levels, as the server allows. */
const MAX_DEPTH = 5
const INDENT = 24

type Entry = Omit<MenuItem, 'children'>

interface Row {
  key: number
  item: Entry
  depth: number
}

let nextKey = 1

export function toRows(tree: MenuItem[], depth = 0, out: Row[] = []): Row[] {
  for (const node of tree) {
    const { children, ...item } = node
    out.push({ key: nextKey++, item, depth })
    toRows(children ?? [], depth + 1, out)
  }
  return out
}

export function toTree(rows: Row[]): MenuItem[] {
  const root: MenuItem[] = []
  const stack: { depth: number; list: MenuItem[] }[] = [{ depth: -1, list: root }]
  for (const r of rows) {
    while (stack.length > 1 && stack[stack.length - 1].depth >= r.depth) stack.pop()
    const node: MenuItem = { ...r.item, children: [] }
    stack[stack.length - 1].list.push(node)
    stack.push({ depth: r.depth, list: node.children })
  }
  return root
}

/** The rows of an entry and everything under it: [start, end). */
function blockOf(rows: Row[], i: number): [number, number] {
  let end = i + 1
  while (end < rows.length && rows[end].depth > rows[i].depth) end++
  return [i, end]
}

/** Moves under the entry before it (as its last child). */
export function indent(rows: Row[], i: number): Row[] | null {
  if (i === 0 || rows[i].depth > rows[i - 1].depth) return null
  const [s, e] = blockOf(rows, i)
  const deepest = Math.max(...rows.slice(s, e).map((r) => r.depth))
  if (deepest + 1 > MAX_DEPTH) return null
  return rows.map((r, j) => (j >= s && j < e ? { ...r, depth: r.depth + 1 } : r))
}

/** Moves out of its parent, to just after the parent and its other children. */
export function outdent(rows: Row[], i: number): Row[] | null {
  if (rows[i].depth === 0) return null
  const [s, e] = blockOf(rows, i)
  let parent = i - 1
  while (parent >= 0 && rows[parent].depth >= rows[i].depth) parent--
  const [, parentEnd] = blockOf(rows, parent)
  const block = rows.slice(s, e).map((r) => ({ ...r, depth: r.depth - 1 }))
  const rest = [...rows.slice(0, s), ...rows.slice(e)]
  const at = parentEnd - (e - s)
  return [...rest.slice(0, at), ...block, ...rest.slice(at)]
}

const typeBadges: Record<string, string> = {
  link: 'Custom link',
  page: 'Page',
  post: 'Post',
  category: 'Category',
  tag: 'Tag',
  group: 'Group',
}

/** The record kinds rows point at: their resource and field. */
const refs = {
  page: { resource: 'pages', field: 'page_id' },
  post: { resource: 'cms_posts', field: 'post_id' },
  category: { resource: 'cms_categories', field: 'category_id' },
  tag: { resource: 'cms_tags', field: 'tag_id' },
} as const
type RefKind = keyof typeof refs

function refId(item: Entry): number | null {
  const ref = refs[item.type as RefKind]
  if (!ref) return null
  const v = (item as unknown as Record<string, unknown>)[ref.field]
  return typeof v === 'number' ? v : null
}

const Icon = ({ d, className = 'size-4' }: { d: string; className?: string }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
    <path d={d} />
  </svg>
)
const icons = {
  plus: 'M12 8v8M8 12h8M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18z',
  bars: 'M4 7h16M4 12h16M4 17h16',
  barsLeft: 'M4 6h16M4 12h10M4 18h16',
  link: 'M10 14a4 4 0 0 0 5.7 0l3-3a4 4 0 0 0-5.7-5.7l-1 1M14 10a4 4 0 0 0-5.7 0l-3 3a4 4 0 0 0 5.7 5.7l1-1',
  tag: 'M4 4h7l9 9-7 7-9-9zM8 8h.01',
  hash: 'M5 9h14M4 15h14M10 3 8 21M16 3l-2 18',
  page: 'M7 3h7l5 5v13H7zM14 3v5h5',
  post: 'M7 3h7l5 5v13H7zM14 3v5h5M10 13h6M10 17h6',
  chevron: 'M6 9l6 6 6-6',
  handle: 'M5 10h14M5 14h14',
  pencil: 'M4 20h4L19 9l-4-4L4 16zM13.5 6.5l4 4',
  trash: 'M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3',
  check: 'M5 13l4 4L19 7',
  search: 'M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM20 20l-4-4',
}

const sources: { key: 'link' | RefKind; label: string; icon: string }[] = [
  { key: 'link', label: L('لینک سفارشی', 'Custom link'), icon: icons.link },
  { key: 'category', label: L('دسته‌ها', 'Categories'), icon: icons.tag },
  { key: 'tag', label: L('برچسب‌ها', 'Tags'), icon: icons.hash },
  { key: 'page', label: L('صفحات', 'Pages'), icon: icons.page },
  { key: 'post', label: L('نوشته‌ها', 'Posts'), icon: icons.post },
]

const panelCls = 'overflow-hidden rounded-xl border border-gray-200/80 bg-white shadow-[0_1px_3px_0_rgb(0_0_0/0.06)] dark:border-white/[0.07] dark:bg-gray-900'
const inputCls =
  'w-full rounded-lg border border-gray-200 bg-white px-3 py-2 text-sm text-gray-900 outline-none placeholder:text-gray-400 focus:border-primary-400 focus:ring-2 focus:ring-primary-400/20 dark:border-white/[0.08] dark:bg-white/[0.04] dark:text-white'
const primaryBtn = 'inline-flex items-center gap-2 rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm hover:bg-primary-500 disabled:opacity-50'

function PanelHead({ icon, title, hint, end }: { icon: string; title: string; hint?: string; end?: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-gray-100 px-4 py-3.5 dark:border-white/[0.06]">
      <div className="flex items-center gap-3">
        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-500/10 dark:text-primary-400 dark:ring-primary-500/20">
          <Icon d={icon} />
        </div>
        <div>
          <h3 className="text-[13px] font-semibold text-gray-900 dark:text-white">{title}</h3>
          {hint && <p className="text-[11px] text-gray-400 dark:text-gray-500">{hint}</p>}
        </div>
      </div>
      {end}
    </div>
  )
}

/** One source: a custom link form, or a searchable, paged list of records to tick. */
function Source({ source, onAdd }: { source: (typeof sources)[number]; onAdd: (entries: Entry[]) => void }) {
  const { api } = usePanel()
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [term, setTerm] = useState('')
  const [page, setPage] = useState(1)
  const [picked, setPicked] = useState<Map<number, string>>(new Map())
  const [url, setUrl] = useState('')
  const [label, setLabel] = useState('')
  const isLink = source.key === 'link'
  useEffect(() => {
    const t = window.setTimeout(() => {
      setTerm(search)
      setPage(1)
    }, 300)
    return () => window.clearTimeout(t)
  }, [search])
  const list = useQuery({
    queryKey: ['menu-source', source.key, term, page],
    enabled: open && !isLink,
    queryFn: () => api.resourceOptions(refs[source.key as RefKind].resource, { search: term, page }),
  })

  function add() {
    if (isLink) {
      if (!url.trim() || !label.trim()) return
      onAdd([{ type: 'link', label: label.trim(), url: url.trim(), new_tab: false, page_id: null, post_id: null, category_id: null, tag_id: null }])
      setUrl('')
      setLabel('')
      return
    }
    const ref = refs[source.key as RefKind]
    onAdd(
      [...picked].map(([id, name]) => ({
        type: source.key,
        label: name,
        url: '',
        new_tab: false,
        page_id: null,
        post_id: null,
        category_id: null,
        tag_id: null,
        [ref.field]: id,
      })),
    )
    setPicked(new Map())
  }

  return (
    <div className={'overflow-hidden rounded-xl border transition-colors ' + (open ? 'border-primary-200 dark:border-primary-500/30' : 'border-gray-200/80 dark:border-white/[0.07]')}>
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
        className={'flex w-full items-center justify-between px-3.5 py-3 text-start hover:bg-gray-50 dark:hover:bg-white/[0.04] ' + (open ? 'bg-gray-50 dark:bg-white/[0.03]' : '')}
      >
        <span className="flex items-center gap-2.5">
          <span className={'flex size-7 shrink-0 items-center justify-center rounded-lg ' + (open ? 'bg-primary-50 text-primary-600 dark:bg-primary-500/10 dark:text-primary-400' : 'bg-gray-100 text-gray-500 dark:bg-white/[0.07] dark:text-gray-400')}>
            <Icon d={source.icon} className="size-3.5" />
          </span>
          <span className="text-[13px] font-semibold text-gray-800 dark:text-gray-200">{source.label}</span>
        </span>
        <span className={'text-gray-400 transition-transform ' + (open ? 'rotate-180' : '')}>
          <Icon d={icons.chevron} />
        </span>
      </button>
      {open && (
        <div className="border-t border-gray-100 bg-gray-50/50 p-3.5 dark:border-white/[0.06] dark:bg-white/[0.02]">
          {isLink ? (
            <div className="space-y-2.5">
              <label className="block">
                <span className="mb-1.5 block text-[11px] font-semibold tracking-wider text-gray-500">{L('نشانی', 'URL')}</span>
                <input dir="ltr" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://" className={inputCls} />
              </label>
              <label className="block">
                <span className="mb-1.5 block text-[11px] font-semibold tracking-wider text-gray-500">{L('متن لینک', 'Link text')}</span>
                <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder={L('متن لینک را وارد کنید', 'Enter the link text')} className={inputCls} />
              </label>
            </div>
          ) : (
            <div className="space-y-2.5">
              <div className="relative">
                <span className="pointer-events-none absolute start-3 top-1/2 -translate-y-1/2 text-gray-400">
                  <Icon d={icons.search} className="size-3.5" />
                </span>
                <input type="search" value={search} onChange={(e) => setSearch(e.target.value)} placeholder={L('جستجو…', 'Search…')} className={inputCls + ' ps-9'} />
              </div>
              {list.isPending ? (
                <p className="py-7 text-center text-xs text-gray-400">{L('در حال بارگذاری…', 'Loading…')}</p>
              ) : (list.data?.data.length ?? 0) === 0 ? (
                <p className="py-5 text-center text-xs text-gray-400">{L('آیتمی موجود نیست', 'Nothing here')}</p>
              ) : (
                <div className="max-h-48 space-y-0.5 overflow-y-auto rounded-lg border border-gray-200/60 bg-white p-1 dark:border-white/[0.06] dark:bg-gray-950/40">
                  {list.data?.data.map((o) => {
                    const id = Number(o.value)
                    return (
                      <label key={String(o.value)} className="flex cursor-pointer items-center gap-2.5 rounded-md px-2 py-1.5 hover:bg-gray-50 dark:hover:bg-white/[0.05]">
                        <input
                          type="checkbox"
                          className="mks-check"
                          checked={picked.has(id)}
                          onChange={(e) =>
                            setPicked((m) => {
                              const next = new Map(m)
                              if (e.target.checked) next.set(id, o.label)
                              else next.delete(id)
                              return next
                            })
                          }
                        />
                        <span className="truncate text-xs text-gray-700 dark:text-gray-300">{o.label}</span>
                      </label>
                    )
                  })}
                </div>
              )}
              {(page > 1 || list.data?.meta.has_more) && (
                <div className="flex items-center justify-between gap-3 border-t border-gray-100 pt-2.5 dark:border-white/[0.06]">
                  <span className="text-[11px] text-gray-400">{L(`صفحهٔ ${formatNumber(page)}`, `Page ${page}`)}</span>
                  <div className="flex gap-1">
                    <button type="button" disabled={page <= 1} onClick={() => setPage((p) => p - 1)} aria-label={L('قبلی', 'Previous')} className="flex size-7 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 disabled:opacity-40 dark:border-white/10 dark:bg-white/5">
                      <Icon d={direction() === 'rtl' ? 'M9 6l6 6-6 6' : 'M15 6l-6 6 6 6'} className="size-3.5" />
                    </button>
                    <button type="button" disabled={!list.data?.meta.has_more} onClick={() => setPage((p) => p + 1)} aria-label={L('بعدی', 'Next')} className="flex size-7 items-center justify-center rounded-lg border border-gray-200 bg-white text-gray-500 disabled:opacity-40 dark:border-white/10 dark:bg-white/5">
                      <Icon d={direction() === 'rtl' ? 'M15 6l-6 6 6 6' : 'M9 6l6 6-6 6'} className="size-3.5" />
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}
          <button
            type="button"
            onClick={add}
            disabled={isLink ? !url.trim() || !label.trim() : picked.size === 0}
            className="mt-3 flex w-full items-center justify-center gap-2 rounded-lg bg-primary-600 px-4 py-2.5 text-xs font-semibold text-white hover:bg-primary-500 disabled:opacity-50"
          >
            <Icon d="M12 5v14M5 12h14" className="size-3.5" />
            {L('افزودن به منو', 'Add to menu')}
          </button>
        </div>
      )}
    </div>
  )
}

/** Edit one row: label, address (custom links) and target. */
function EditDialog({ row, title, onSave, onClose }: { row: Row; title: string; onSave: (item: Entry) => void; onClose: () => void }) {
  const [label, setLabel] = useState(row.item.label ?? '')
  const [url, setUrl] = useState(row.item.url ?? '')
  const [newTab, setNewTab] = useState(!!row.item.new_tab)
  const isLink = row.item.type === 'link'
  return (
    <Modal
      title={L('ویرایش آیتم', 'Edit item')}
      onClose={onClose}
      closeOnBackdrop={false}
      footer={
        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm font-semibold ring-1 ring-gray-950/10 dark:ring-white/10">
            {L('انصراف', 'Cancel')}
          </button>
          <button
            type="button"
            className={primaryBtn}
            disabled={(isLink || row.item.type === 'group') && !label.trim()}
            onClick={() => onSave({ ...row.item, label: label.trim(), url: isLink ? url.trim() : row.item.url, new_tab: newTab })}
          >
            {L('ذخیره', 'Save')}
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        <label className="block text-sm">
          <span className="mb-1.5 block font-medium">{L('برچسب', 'Label')}</span>
          <input value={label} onChange={(e) => setLabel(e.target.value)} placeholder={title} maxLength={100} className={inputCls} />
        </label>
        {isLink && (
          <label className="block text-sm">
            <span className="mb-1.5 block font-medium">{L('نشانی', 'URL')}</span>
            <input dir="ltr" value={url} onChange={(e) => setUrl(e.target.value)} placeholder="https://" maxLength={2048} className={inputCls} />
          </label>
        )}
        {row.item.type !== 'group' && (
          <label className="block text-sm">
            <span className="mb-1.5 block font-medium">{L('هدف', 'Target')}</span>
            <select value={newTab ? '_blank' : '_self'} onChange={(e) => setNewTab(e.target.value === '_blank')} className={inputCls}>
              <option value="_self">{L('همان تب', 'Same tab')}</option>
              <option value="_blank">{L('تب جدید', 'New tab')}</option>
            </select>
          </label>
        )}
      </div>
    </Modal>
  )
}

/** The titles of the records rows point at, for rows without a label of their own. */
function useRefTitles(rows: Row[]): Record<string, string> {
  const { api } = usePanel()
  const wanted = useMemo(() => {
    const out: Partial<Record<RefKind, number[]>> = {}
    for (const r of rows) {
      const id = refId(r.item)
      if (id !== null && !r.item.label) (out[r.item.type as RefKind] ??= []).push(id)
    }
    return out
  }, [rows])
  const key = JSON.stringify(wanted)
  const q = useQuery({
    queryKey: ['menu-ref-titles', key],
    enabled: key !== '{}',
    queryFn: async () => {
      const titles: Record<string, string> = {}
      for (const [kind, ids] of Object.entries(wanted) as [RefKind, number[]][]) {
        const res = await api.resourceOptions(refs[kind].resource, { values: ids })
        for (const o of res.data) titles[`${kind}:${o.value}`] = o.label
      }
      return titles
    },
  })
  return q.data ?? {}
}

function MenuStructure({ rows, setRows }: { rows: Row[]; setRows: (rows: Row[]) => void }) {
  const titles = useRefTitles(rows)
  const [editing, setEditing] = useState<Row | null>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const [drag, setDrag] = useState<{ from: number; size: number; to: number; depth: number } | null>(null)
  const rtl = direction() === 'rtl'
  const titleOf = (r: Row) => r.item.label || titles[`${r.item.type}:${refId(r.item)}`] || typeBadges[r.item.type] || ''

  /** Pointer drag: up and down picks the place, sideways the depth (MKSine's flat list). */
  function startDrag(e: React.PointerEvent, i: number) {
    e.preventDefault()
    const [s, end] = blockOf(rows, i)
    const block = rows.slice(s, end)
    const rel = Math.max(...block.map((r) => r.depth)) - rows[i].depth
    const startX = e.clientX
    const startDepth = rows[i].depth
    const others = [...rows.slice(0, s), ...rows.slice(end)]
    const els = Array.from(listRef.current?.querySelectorAll<HTMLElement>('[data-menu-row]') ?? [])
    const mids = els.map((el) => {
      const b = el.getBoundingClientRect()
      return b.top + b.height / 2
    })
    // The middles of the rows that stay, for finding the drop place.
    const stay = mids.filter((_, j) => j < s || j >= end)
    let state = { from: s, size: end - s, to: s, depth: startDepth }
    const place = (x: number, y: number) => {
      let to = stay.findIndex((m) => y < m)
      if (to < 0) to = others.length
      const prev = others[to - 1]
      const next = others[to]
      const wanted = startDepth + Math.round(((rtl ? startX - x : x - startX)) / INDENT)
      const max = Math.min(prev ? prev.depth + 1 : 0, MAX_DEPTH - rel)
      const min = next ? Math.max(0, next.depth - rel - 1) : 0
      state = { from: s, size: end - s, to, depth: Math.max(min, Math.min(max, wanted)) }
      setDrag(state)
    }
    place(e.clientX, e.clientY)
    const move = (ev: PointerEvent) => place(ev.clientX, ev.clientY)
    const up = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', up)
      const shift = state.depth - startDepth
      const moved = block.map((r) => ({ ...r, depth: r.depth + shift }))
      setRows([...others.slice(0, state.to), ...moved, ...others.slice(state.to)])
      setDrag(null)
    }
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', up)
  }

  const action = 'flex h-7 items-center gap-1 rounded-md px-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-white/[0.07] dark:hover:text-gray-300'
  // While dragging, the moving rows are dimmed and a line shows where they will land.
  const others = drag ? [...rows.slice(0, drag.from), ...rows.slice(drag.from + drag.size)] : rows
  const lineBefore = drag ? (others[drag.to] ? others[drag.to].key : -1) : null

  const marker = (depth: number) => (
    <div aria-hidden="true" className="h-1 rounded-full bg-primary-500" style={{ marginInlineStart: depth * INDENT }} />
  )

  return (
    <div ref={listRef} className="space-y-1.5">
      {rows.map((r, i) => {
        const moving = drag && i >= drag.from && i < drag.from + drag.size
        const title = titleOf(r)
        return (
          <div key={r.key}>
            {drag && lineBefore === r.key && marker(drag.depth)}
            <div data-menu-row className={'relative ' + (moving ? 'opacity-40' : '')} style={{ paddingInlineStart: r.depth * INDENT }}>
              {Array.from({ length: r.depth }, (_, g) => (
                <span key={g} aria-hidden="true" className="absolute inset-y-[-6px] w-px bg-gray-200 dark:bg-white/10" style={{ insetInlineStart: g * INDENT + 11 }} />
              ))}
              <div className="group flex items-center gap-2.5 rounded-xl border border-gray-200/80 bg-white px-3 py-2.5 shadow-[0_1px_3px_0_rgb(0_0_0/0.05)] hover:border-primary-200 dark:border-white/[0.07] dark:bg-gray-900">
                <button
                  type="button"
                  onPointerDown={(e) => startDrag(e, i)}
                  aria-label={L(`جابه‌جا کردن «${title}»`, `Move "${title}"`)}
                  className="shrink-0 cursor-grab touch-none rounded-md p-1 text-gray-300 hover:bg-gray-100 hover:text-gray-500 active:cursor-grabbing dark:text-gray-600 dark:hover:bg-white/[0.06]"
                >
                  <Icon d={icons.handle} />
                </button>
                <span className="shrink-0 rounded-md border border-gray-100 bg-gray-50 px-2 py-0.5 text-[10px] font-semibold tracking-wider text-gray-400 uppercase dark:border-white/[0.06] dark:bg-white/[0.04]" dir="ltr">
                  {typeBadges[r.item.type] ?? r.item.type}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-[13px] leading-tight font-semibold text-gray-900 dark:text-gray-100">{title}</p>
                  <p className="mt-0.5 flex items-center gap-1.5 truncate text-[11px] leading-tight text-gray-400">
                    {r.item.type === 'link' && r.item.url && (
                      <>
                        <span className="truncate" dir="ltr">
                          {r.item.url}
                        </span>
                        <span className="shrink-0">•</span>
                      </>
                    )}
                    {r.item.type !== 'group' && <span className="shrink-0">{r.item.new_tab ? L('تب جدید', 'New tab') : L('همان تب', 'Same tab')}</span>}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-0.5 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100">
                  {r.depth > 0 && (
                    <button type="button" className={action + ' hover:bg-primary-50 hover:text-primary-600'} title={L('یک سطح بالاتر آوردن', 'Move up a level')} onClick={() => setRows(outdent(rows, i) ?? rows)}>
                      <Icon d={rtl ? 'M9 14l5-5-5-5M14 9H8a4 4 0 0 0-4 4v7' : 'M15 14l-5-5 5-5M10 9h6a4 4 0 0 1 4 4v7'} className="size-3.5" />
                      <span className="text-[10px] leading-none font-semibold">{L('خروج', 'Out')}</span>
                    </button>
                  )}
                  {indent(rows, i) && (
                    <button type="button" className={action} title={L('زیرمنو کردن (تو در تو زیر آیتم بالا)', 'Nest under the item above')} onClick={() => setRows(indent(rows, i) ?? rows)}>
                      <Icon d={rtl ? 'M17 7 7 17M7 9v8h8' : 'M7 7l10 10M17 9v8H9'} className="size-3.5" />
                      <span className="text-[10px] leading-none font-semibold">{L('زیرمنو', 'Nest')}</span>
                    </button>
                  )}
                  <div className="mx-0.5 h-4 w-px bg-gray-200 dark:bg-white/[0.08]" />
                  <button type="button" className={action + ' w-7 justify-center px-0'} title={L('ویرایش', 'Edit')} aria-label={L(`ویرایش «${title}»`, `Edit "${title}"`)} onClick={() => setEditing(r)}>
                    <Icon d={icons.pencil} className="size-3.5" />
                  </button>
                  <button
                    type="button"
                    className={action + ' w-7 justify-center px-0 hover:bg-danger-50 hover:text-danger-500'}
                    title={L('حذف', 'Remove')}
                    aria-label={L(`حذف «${title}»`, `Remove "${title}"`)}
                    onClick={() => {
                      if (!window.confirm(L('آیا از حذف این آیتم اطمینان دارید؟', 'Remove this item?'))) return
                      const [s, e] = blockOf(rows, i)
                      // Its children move up to its place.
                      setRows([...rows.slice(0, s), ...rows.slice(s + 1, e).map((c) => ({ ...c, depth: c.depth - 1 })), ...rows.slice(e)])
                    }}
                  >
                    <Icon d={icons.trash} className="size-3.5" />
                  </button>
                </div>
              </div>
            </div>
          </div>
        )
      })}
      {drag && lineBefore === -1 && marker(drag.depth)}
      {editing && (
        <EditDialog
          row={editing}
          title={titleOf(editing)}
          onClose={() => setEditing(null)}
          onSave={(item) => {
            setRows(rows.map((r) => (r.key === editing.key ? { ...r, item } : r)))
            setEditing(null)
          }}
        />
      )}
    </div>
  )
}

function MenuPicker({ menus, current, onPick, onClose }: { menus: Menu[]; current?: number; onPick: (id: number) => void; onClose: () => void }) {
  const [id, setId] = useState(current ?? menus[0]?.id)
  return (
    <Modal
      title={L('انتخاب منو', 'Select menu')}
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <button type="button" onClick={onClose} className="rounded-lg px-4 py-2 text-sm font-semibold ring-1 ring-gray-950/10 dark:ring-white/10">
            {L('انصراف', 'Cancel')}
          </button>
          <button type="button" className={primaryBtn} disabled={!id} onClick={() => id && onPick(id)}>
            {L('انتخاب', 'Select')}
          </button>
        </div>
      }
    >
      <label className="block text-sm">
        <span className="mb-1.5 block font-medium">{L('منو', 'Menu')}</span>
        <select className={inputCls} value={id ?? ''} onChange={(e) => setId(Number(e.target.value))}>
          {menus.map((m) => (
            <option key={m.id} value={m.id}>
              {m.name}
            </option>
          ))}
        </select>
      </label>
    </Modal>
  )
}

function MenuBuilderPage() {
  const { api, notify } = usePanel()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const { hash } = useLocation()
  const menuId = Number(String(hash ?? '').replace(/^#/, '')) || undefined
  const [picking, setPicking] = useState(false)
  const [rows, setRows] = useState<Row[] | null>(null)
  const [dirty, setDirty] = useState(false)
  const [saving, setSaving] = useState(false)
  const menus = useQuery({ queryKey: ['menu-builder-menus'], queryFn: () => api.list<Menu>('menus', { perPage: 100, sort: ['name'] }) })
  const menu = useQuery({ queryKey: ['menu-builder', menuId], enabled: !!menuId, queryFn: () => api.get<Menu>('menus', menuId!) })
  useEffect(() => {
    if (menu.data) {
      setRows(toRows(menu.data.data.items ?? []))
      setDirty(false)
    }
  }, [menu.data])
  const change = (next: Row[]) => {
    setRows(next)
    setDirty(true)
  }

  async function save() {
    if (!menuId || !rows) return
    setSaving(true)
    try {
      await api.update('menus', menuId, { items: toTree(rows) })
      notify(L('منو با موفقیت ذخیره شد', 'Menu saved'), 'success')
      setDirty(false)
      await queryClient.invalidateQueries({ queryKey: ['menu-builder', menuId] })
    } catch (e) {
      notify(e instanceof ApiError ? (Object.values(e.errors)[0]?.[0] ?? e.message) : String(e), 'danger')
    } finally {
      setSaving(false)
    }
  }

  const name = menu.data?.data.name
  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{L('ساخت منو', 'Menu builder')}</h1>
          {name && <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{L(`در حال ویرایش: ${name}`, `Editing: ${name}`)}</p>}
        </div>
        <div className="flex flex-wrap gap-2">
          <button type="button" className={primaryBtn} onClick={() => setPicking(true)}>
            <Icon d={icons.barsLeft} />
            {L('انتخاب منو', 'Select menu')}
          </button>
          {menuId && (
            <button type="button" className={primaryBtn} disabled={saving || !dirty} onClick={() => void save()}>
              <Icon d={icons.check} />
              {L('ذخیره منو', 'Save menu')}
            </button>
          )}
        </div>
      </div>

      {!menuId ? (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div className="mb-5 flex size-14 items-center justify-center rounded-2xl bg-primary-50 text-primary-600 ring-1 ring-primary-100 dark:bg-primary-500/10 dark:text-primary-400 dark:ring-primary-500/20">
            <Icon d={icons.barsLeft} className="size-7" />
          </div>
          <h3 className="mb-1.5 text-[15px] font-semibold">{L('منویی انتخاب نشده', 'No menu selected')}</h3>
          <p className="max-w-xs text-sm text-gray-500">{L('از دکمهٔ انتخاب منو در بالای صفحه برای شروع استفاده کنید.', 'Use "Select menu" above to start.')}</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 gap-5 lg:grid-cols-3">
          <div className="space-y-3 lg:col-span-1">
            <div className={panelCls}>
              <PanelHead icon={icons.plus} title={L('افزودن آیتم به منو', 'Add menu items')} />
              <div className="space-y-2 p-3">
                {sources.map((s) => (
                  <Source
                    key={s.key}
                    source={s}
                    onAdd={(entries) => {
                      change([...(rows ?? []), ...entries.map((item) => ({ key: nextKey++, item, depth: 0 }))])
                      notify(L('آیتم(ها) اضافه شد', 'Items added'), 'success')
                    }}
                  />
                ))}
              </div>
            </div>
          </div>
          <div className="lg:col-span-2">
            <div className={panelCls}>
              <PanelHead
                icon={icons.bars}
                title={L('ساختار منو', 'Menu structure')}
                hint={L('برای مرتب‌سازی بکشید. از دکمه‌های → / ← برای تو در تو کردن استفاده کنید.', 'Drag to reorder. Use the → / ← buttons to nest.')}
                end={
                  <span className="inline-flex items-center rounded-full border border-gray-200/80 bg-gray-50 px-2.5 py-1 text-[11px] font-semibold text-gray-500 dark:border-white/[0.07] dark:bg-white/[0.04]">
                    {L(`${formatNumber(rows?.length ?? 0)} آیتم`, `${rows?.length ?? 0} items`)}
                  </span>
                }
              />
              <div className="min-h-80 p-4">
                {!rows ? (
                  <div className="h-48 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
                ) : rows.length === 0 ? (
                  <div className="flex flex-col items-center justify-center rounded-xl border border-dashed border-primary-200 bg-primary-50/20 py-16 text-center dark:border-primary-500/25">
                    <p className="text-[13px] font-semibold text-gray-700 dark:text-gray-300">{L('از پنل کناری آیتم اضافه کنید', 'Add items from the side panel')}</p>
                    <p className="mt-1 text-xs text-gray-400">{L('برای مرتب‌سازی بکشید. از دکمه‌های → / ← برای تو در تو کردن استفاده کنید.', 'Drag to reorder. Use the → / ← buttons to nest.')}</p>
                  </div>
                ) : (
                  <MenuStructure rows={rows} setRows={change} />
                )}
              </div>
            </div>
          </div>
        </div>
      )}
      {picking && menus.data && (
        <MenuPicker
          menus={menus.data.data}
          current={menuId}
          onClose={() => setPicking(false)}
          onPick={(id) => {
            if (dirty && !window.confirm(L('تغییرهای ذخیره‌نشده از بین می‌رود. ادامه می‌دهید؟', 'Unsaved changes will be lost. Continue?'))) return
            setPicking(false)
            setRows(null)
            void navigate({ to: '/$resource', params: { resource: 'menu_builder' }, hash: String(id) } as never)
          }}
        />
      )}
    </div>
  )
}

/** MKSine's "Menu builder": not in the sidebar; opened from the menus list or a menu's page. */
export const menuBuilderPage = definePage({
  name: 'menu_builder',
  label: L('ساخت منو', 'Menu builder'),
  navigation: false,
  component: MenuBuilderPage,
})
