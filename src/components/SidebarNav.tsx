import { Link } from '@tanstack/react-router'
import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { direction, numberFormat, t } from '../i18n'
import { badgeColors } from './cells'
import { ChevronIcon, CloseIcon } from './icons'
import type { NavItem } from './Layout'

/** A sidebar group as groupNavigation returns it. */
export interface NavGroupView {
  heading: string
  icon?: ReactNode
  ungrouped: boolean
  items: NavItem[]
}

/**
 * The sidebar menu, laid out like MKSine's (WordPress style):
 * - "open" (desktop): each group is one row; hovering or focusing it shows its items in a
 *   panel beside the sidebar, and a click opens its first item. A group of one item is a
 *   plain link.
 * - "rail" (collapsed desktop): icons only; a group's icon shows the same panel.
 * - "mobile" (the drawer): groups open and close in place; the current one is open.
 * A search box filters every item, showing the matches under their group headings.
 */
export function SidebarNav({ groups, mode, onNavigate }: { groups: NavGroupView[]; mode: 'open' | 'rail' | 'mobile'; onNavigate: () => void }) {
  const [query, setQuery] = useState('')
  const q = query.trim().toLowerCase()
  const filtering = mode !== 'rail' && q !== ''
  const shown = filtering
    ? groups
        .map((g) => {
          const all = !g.ungrouped && g.heading.toLowerCase().includes(q)
          return { ...g, items: all ? g.items : g.items.filter((i) => i.label.toLowerCase().includes(q)) }
        })
        .filter((g) => g.items.length > 0)
    : groups

  return (
    <>
      {mode !== 'rail' && (
        <div className="px-3 pt-1 pb-2">
          <label className="relative block">
            <span className="sr-only">{t('nav.searchLabel')}</span>
            <span className="pointer-events-none absolute inset-y-0 start-2.5 flex items-center text-gray-400 [&_svg]:size-4" aria-hidden="true">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round">
                <circle cx="11" cy="11" r="7" />
                <path d="m20 20-3.5-3.5" />
              </svg>
            </span>
            <input
              type="search"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => e.key === 'Escape' && setQuery('')}
              placeholder={t('nav.search')}
              autoComplete="off"
              spellCheck={false}
              className="block w-full rounded-lg border-0 bg-gray-900/[0.04] py-1.5 ps-8 pe-8 text-sm placeholder:text-gray-400 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5 [&::-webkit-search-cancel-button]:hidden"
            />
            {query && (
              <button
                type="button"
                onClick={() => setQuery('')}
                aria-label={t('nav.clearSearch')}
                title={t('nav.clearSearch')}
                className="absolute inset-y-0 end-1.5 flex items-center text-gray-400 hover:text-gray-600 [&_svg]:size-4"
              >
                <CloseIcon />
              </button>
            )}
          </label>
        </div>
      )}
      <nav className={'flex-1 overflow-y-auto pb-6 ' + (mode === 'rail' ? 'space-y-1 px-2 pt-2' : 'space-y-1 px-3 pt-1')}>
        {filtering && shown.length === 0 && <p className="px-3 py-2 text-sm text-gray-500">{t('nav.noMatches')}</p>}
        {shown.map((g) =>
          g.ungrouped || filtering ? (
            <div key={g.heading} className={filtering && !g.ungrouped ? 'pt-2' : ''}>
              {filtering && !g.ungrouped && <p className="px-3 pb-1 text-[11px] font-semibold tracking-wider text-gray-400 dark:text-gray-500">{g.heading}</p>}
              <ul className="space-y-0.5">
                {g.items.map((item) => (
                  <li key={item.key}>
                    <ItemLink item={item} rail={mode === 'rail'} onClick={onNavigate} />
                  </li>
                ))}
              </ul>
            </div>
          ) : mode === 'mobile' ? (
            <AccordionGroup key={g.heading} group={g} onNavigate={onNavigate} />
          ) : (
            <FlyoutGroup key={g.heading} group={g} rail={mode === 'rail'} onNavigate={onNavigate} />
          ),
        )}
      </nav>
    </>
  )
}

const rowBase = 'group flex w-full items-center gap-3 rounded-xl px-3 py-2 text-sm font-medium '
const rowIdle = 'text-gray-600 hover:bg-gray-900/[0.04] hover:text-gray-950 dark:text-gray-400 dark:hover:bg-white/5 dark:hover:text-white'
const rowActive = 'bg-primary-50 text-primary-700 ring-1 ring-primary-600/10 dark:bg-primary-500/10 dark:text-primary-300 dark:ring-primary-400/10'

function iconClass(active: boolean) {
  return 'shrink-0 [&_svg]:size-5 ' + (active ? 'text-primary-600 dark:text-primary-400' : 'text-gray-400 group-hover:text-gray-600 dark:text-gray-500 dark:group-hover:text-gray-300')
}

function Badge({ badge }: { badge?: NavItem['badge'] }) {
  if (!badge || badge.count <= 0) return null
  return (
    <span className={'shrink-0 rounded-full px-2 py-0.5 text-xs font-semibold tabular-nums ring-1 ring-inset ' + (badgeColors[badge.color ?? 'primary'] ?? badgeColors.primary)}>
      {numberFormat().format(badge.count)}
    </span>
  )
}

/** One entry: a link with its icon, label and count (icon only in the rail). */
function ItemLink({ item, rail, onClick }: { item: NavItem; rail?: boolean; onClick: () => void }) {
  return (
    <Link
      to={item.to}
      params={item.params as never}
      onClick={onClick}
      aria-current={item.active ? 'page' : undefined}
      aria-label={rail ? item.label : undefined}
      title={rail ? item.label : undefined}
      className={rowBase + (rail ? 'justify-center px-0 ' : '') + (item.active ? rowActive : rowIdle)}
    >
      <span className={'relative ' + iconClass(item.active)}>
        {item.icon}
        {rail && item.badge && item.badge.count > 0 && <span className="absolute -top-1 -end-1 size-2 rounded-full bg-danger-500" aria-hidden="true" />}
      </span>
      {!rail && <span className="min-w-0 flex-1 truncate">{item.label}</span>}
      {!rail && <Badge badge={item.badge} />}
    </Link>
  )
}

/** The sum of a group's counts, shown on its row. */
function groupBadge(items: NavItem[]): NavItem['badge'] {
  const withCount = items.filter((i) => i.badge && i.badge.count > 0)
  if (withCount.length === 0) return undefined
  return { count: withCount.reduce((n, i) => n + (i.badge?.count ?? 0), 0), color: withCount[0].badge?.color }
}

/** A group in the drawer: a button that opens its items in place. */
function AccordionGroup({ group, onNavigate }: { group: NavGroupView; onNavigate: () => void }) {
  const active = group.items.some((i) => i.active)
  const [open, setOpen] = useState(active)
  const id = useId()
  useEffect(() => {
    if (active) setOpen(true)
  }, [active])
  return (
    <div>
      <button type="button" aria-expanded={open} aria-controls={id} onClick={() => setOpen((o) => !o)} className={rowBase + (active ? 'text-primary-700 dark:text-primary-300' : rowIdle)}>
        <span className={iconClass(active)}>{group.icon}</span>
        <span className="min-w-0 flex-1 truncate text-start">{group.heading}</span>
        <Badge badge={groupBadge(group.items)} />
        <span className={'text-gray-400 transition-transform [&_svg]:size-4 ' + (open ? 'rotate-180' : '')}>
          <ChevronIcon dir="down" />
        </span>
      </button>
      {open && (
        <ul id={id} className="mt-0.5 space-y-0.5 border-s border-gray-200 ms-5 ps-2 dark:border-white/10">
          {group.items.map((item) => (
            <li key={item.key}>
              <ItemLink item={item} onClick={onNavigate} />
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

const HIDE_DELAY = 200

/**
 * A group in the open sidebar or the rail: one row; its items show in a panel beside the
 * sidebar on hover or focus. A click on the row opens the first item.
 */
function FlyoutGroup({ group, rail, onNavigate }: { group: NavGroupView; rail: boolean; onNavigate: () => void }) {
  const active = group.items.some((i) => i.active)
  const first = group.items[0]
  const solo = group.items.length === 1 && !rail
  const row = useRef<HTMLAnchorElement>(null)
  const panel = useRef<HTMLDivElement>(null)
  const [pos, setPos] = useState<{ top: number; side: number } | null>(null)
  const timer = useRef<number | undefined>(undefined)
  const panelId = useId()
  const rtl = direction() === 'rtl'

  const show = () => {
    window.clearTimeout(timer.current)
    const r = row.current?.getBoundingClientRect()
    if (!r) return
    // Beside the sidebar: to the left of it in RTL, to the right in LTR.
    setPos({ top: Math.max(8, Math.min(r.top, window.innerHeight - 64)), side: rtl ? window.innerWidth - r.left + 6 : r.right + 6 })
  }
  const hideSoon = () => {
    window.clearTimeout(timer.current)
    timer.current = window.setTimeout(() => setPos(null), HIDE_DELAY)
  }
  const hide = () => {
    window.clearTimeout(timer.current)
    setPos(null)
  }
  useEffect(() => () => window.clearTimeout(timer.current), [])
  // Keep the panel on screen: move it up when it would run off the bottom.
  useEffect(() => {
    const el = panel.current
    if (!el || !pos) return
    const overflow = el.getBoundingClientRect().bottom - (window.innerHeight - 8)
    if (overflow > 0) el.style.top = `${Math.max(8, pos.top - overflow)}px`
  }, [pos])
  useEffect(() => {
    if (!pos) return
    const close = () => setPos(null)
    window.addEventListener('scroll', close, true)
    window.addEventListener('resize', close)
    return () => {
      window.removeEventListener('scroll', close, true)
      window.removeEventListener('resize', close)
    }
  }, [pos])

  if (!first) return null
  if (solo) return <ItemLink item={{ ...first, icon: group.icon ?? first.icon, label: group.heading }} onClick={onNavigate} />

  return (
    <div onMouseEnter={show} onMouseLeave={hideSoon}>
      <Link
        ref={row}
        to={first.to}
        params={first.params as never}
        onClick={() => {
          hide()
          onNavigate()
        }}
        onFocus={show}
        onBlur={(e) => !panel.current?.contains(e.relatedTarget as Node) && hideSoon()}
        onKeyDown={(e) => {
          if (e.key === 'Escape') hide()
          if (e.key === (rtl ? 'ArrowLeft' : 'ArrowRight')) {
            e.preventDefault()
            show()
            window.setTimeout(() => panel.current?.querySelector<HTMLElement>('a')?.focus())
          }
        }}
        aria-haspopup="true"
        aria-expanded={pos !== null}
        aria-controls={pos ? panelId : undefined}
        aria-label={rail ? group.heading : undefined}
        title={rail ? group.heading : undefined}
        className={rowBase + (rail ? 'justify-center px-0 ' : '') + (active ? rowActive : rowIdle)}
      >
        <span className={'relative ' + iconClass(active)}>
          {group.icon ?? first.icon}
          {rail && groupBadge(group.items) && <span className="absolute -top-1 -end-1 size-2 rounded-full bg-danger-500" aria-hidden="true" />}
        </span>
        {!rail && (
          <>
            <span className="min-w-0 flex-1 truncate">{group.heading}</span>
            <Badge badge={groupBadge(group.items)} />
            <span className="text-gray-400 [&_svg]:size-4" aria-hidden="true">
              <ChevronIcon dir={rtl ? 'left' : 'right'} />
            </span>
          </>
        )}
      </Link>
      {pos &&
        createPortal(
          <div
            ref={panel}
            id={panelId}
            role="group"
            aria-label={group.heading}
            onMouseEnter={show}
            onMouseLeave={hideSoon}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                hide()
                row.current?.focus()
              }
            }}
            onBlur={(e) => !panel.current?.contains(e.relatedTarget as Node) && e.relatedTarget !== row.current && hideSoon()}
            style={{ position: 'fixed', top: pos.top, [rtl ? 'right' : 'left']: pos.side }}
            className="z-50 w-60 animate-pop rounded-2xl bg-white p-1.5 shadow-lg ring-1 ring-gray-950/5 dark:bg-gray-900 dark:text-white dark:ring-white/10"
          >
            <p className="px-3 pt-1.5 pb-1 text-[11px] font-semibold tracking-wider text-gray-400 dark:text-gray-500">{group.heading}</p>
            <ul className="space-y-0.5">
              {group.items.map((item) => (
                <li key={item.key}>
                  <ItemLink
                    item={item}
                    onClick={() => {
                      hide()
                      onNavigate()
                    }}
                  />
                </li>
              ))}
            </ul>
          </div>,
          document.body,
        )}
    </div>
  )
}
