import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useId, useRef, useState, type KeyboardEvent } from 'react'
import type { SearchResult } from '../protocol'
import type { ResourceDefinition } from '../resource'
import { BadgeColumn } from '../table'
import { SearchIcon, SquaresIcon } from './icons'
import { usePanel } from './Panel'
import { iconButton } from './styles'
import { t } from '../i18n'

const EXIT_MS = 240
const isMac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform)

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = window.setTimeout(() => setV(value), ms)
    return () => window.clearTimeout(t)
  }, [value, ms])
  return v
}

const typing = (el: EventTarget | null) =>
  el instanceof HTMLElement && (el.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName))

/** The topbar search button; Ctrl/⌘+K or "/" opens it from anywhere. */
export function GlobalSearch() {
  const { meta } = usePanel()
  const [open, setOpen] = useState(false)

  useEffect(() => {
    const onKey = (e: globalThis.KeyboardEvent) => {
      if ((e.key.toLowerCase() === 'k' && (e.metaKey || e.ctrlKey)) || (e.key === '/' && !typing(e.target))) {
        e.preventDefault()
        setOpen(true)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  if (!meta.search) return null
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-haspopup="dialog"
        aria-keyshortcuts={isMac ? 'Meta+K' : 'Control+K'}
        className="hidden w-64 items-center gap-2 rounded-lg bg-gray-900/[0.04] px-3 py-1.5 text-sm text-gray-500 ring-1 ring-gray-950/5 hover:bg-gray-900/[0.07] sm:flex dark:bg-white/5 dark:text-gray-400 dark:ring-white/10 dark:hover:bg-white/10"
      >
        <SearchIcon />
        <span className="flex-1 text-start">{t('common.searchPlaceholder')}</span>
        <kbd dir="ltr" className="rounded-md bg-white px-1.5 py-0.5 font-sans text-[11px] text-gray-500 ring-1 ring-gray-950/10 dark:bg-white/10 dark:text-gray-300 dark:ring-white/10">
          {isMac ? '⌘K' : 'Ctrl K'}
        </kbd>
      </button>
      <button type="button" onClick={() => setOpen(true)} aria-label={t('common.search')} aria-haspopup="dialog" className={iconButton + ' sm:hidden'}>
        <SearchIcon />
      </button>
      {open && <SearchDialog onClose={() => setOpen(false)} />}
    </>
  )
}

/** How a detail value reads: a badge column's label if the resource has one. */
function detailText(resource: ResourceDefinition | undefined, column: string, value: unknown): string | null {
  if (value == null || value === '') return null
  const col = resource?.columns.find((c) => c.name === column)
  if (col instanceof BadgeColumn) return col._labels[String(value)] ?? String(value)
  return String(value)
}

function SearchDialog({ onClose }: { onClose: () => void }) {
  const { api, resources } = usePanel()
  const navigate = useNavigate()
  const dialog = useRef<HTMLDialogElement>(null)
  const input = useRef<HTMLInputElement>(null)
  const closing = useRef(false)
  const listId = useId()
  const [q, setQ] = useState('')
  const [active, setActive] = useState(0)
  const term = useDebounced(q.trim(), 250)

  useEffect(() => {
    dialog.current?.showModal()
    input.current?.focus()
  }, [])

  const query = useQuery({
    queryKey: ['search', term],
    queryFn: () => api.search(term),
    enabled: term.length >= 2,
    placeholderData: keepPreviousData,
    select: (r) => r.data,
  })
  const results = term.length >= 2 ? (query.data ?? []) : []
  useEffect(() => setActive(0), [term])

  // Results grouped by resource, in the panel's order; one flat index for the keyboard.
  const groups = resources
    .map((r) => ({ resource: r, items: results.filter((x) => x.resource === r.name) }))
    .filter((g) => g.items.length > 0)
  const flat = groups.flatMap((g) => g.items)

  function close() {
    if (closing.current || !dialog.current) return
    closing.current = true
    dialog.current.close()
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    window.setTimeout(onClose, reduced ? 0 : EXIT_MS)
  }

  function open(r: SearchResult) {
    const def = resources.find((x) => x.name === r.resource)
    const id = String(r.key)
    close()
    if (def && def.infolist.length > 0) void navigate({ to: '/$resource/$id', params: { resource: r.resource, id } })
    else if (def && def.fields.length > 0) void navigate({ to: '/$resource/$id/edit', params: { resource: r.resource, id } })
    else void navigate({ to: '/$resource', params: { resource: r.resource } })
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (flat.length === 0) return
      setActive((a) => (a + (e.key === 'ArrowDown' ? 1 : -1) + flat.length) % flat.length)
    } else if (e.key === 'Enter' && flat[active]) {
      e.preventDefault()
      open(flat[active])
    }
  }

  const optionId = (i: number) => `${listId}-${i}`
  let index = -1

  return (
    <dialog
      ref={dialog}
      aria-label={t('search.dialog')}
      onCancel={(e) => {
        e.preventDefault()
        close()
      }}
      onMouseDown={(e) => e.target === e.currentTarget && close()}
      className="mks-dialog mx-auto mt-[12vh] mb-auto w-[calc(100%-2rem)] max-w-xl bg-transparent p-0 text-gray-950 dark:text-white"
    >
      <div className="overflow-hidden rounded-2xl bg-white shadow-xl ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10">
        <div className="flex items-center gap-3 border-b border-gray-100 px-4 dark:border-white/5">
          <span className="text-gray-400 [&_svg]:size-5">
            <SearchIcon />
          </span>
          <input
            ref={input}
            type="search"
            role="combobox"
            aria-expanded={flat.length > 0}
            aria-controls={listId}
            aria-autocomplete="list"
            aria-activedescendant={flat.length > 0 ? optionId(active) : undefined}
            aria-label={t('common.search')}
            placeholder={t('search.placeholder')}
            value={q}
            onChange={(e) => setQ(e.target.value)}
            onKeyDown={onKeyDown}
            className="h-14 flex-1 border-0 bg-transparent text-base outline-none placeholder:text-gray-400"
          />
          {query.isFetching && <span className="size-4 animate-spin rounded-full border-2 border-gray-300 border-t-primary-500" aria-hidden="true" />}
          <kbd className="rounded-md px-1.5 py-0.5 text-[11px] text-gray-400 ring-1 ring-gray-950/10 dark:ring-white/10">Esc</kbd>
        </div>

        <div className="max-h-[min(60vh,28rem)] overflow-y-auto">
          {term.length < 2 ? (
            <p className="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{t('search.minLength')}</p>
          ) : flat.length === 0 && !query.isFetching ? (
            <p className="px-5 py-10 text-center text-sm text-gray-500 dark:text-gray-400" role="status">
              {t('search.none', { term })}
            </p>
          ) : (
            <div id={listId} role="listbox" aria-label={t('search.results')} className="p-2">
              {groups.map((g) => (
                <div key={g.resource.name} role="group" aria-labelledby={`${listId}-${g.resource.name}`} className="mb-1">
                  <p id={`${listId}-${g.resource.name}`} className="px-3 pt-2 pb-1 text-[11px] font-semibold tracking-wider text-gray-400 dark:text-gray-500">
                    {g.resource.pluralLabel}
                  </p>
                  {g.items.map((r) => {
                    index++
                    const i = index
                    const details = Object.entries(r.details ?? {})
                      .map(([k, v]) => detailText(g.resource, k, v))
                      .filter(Boolean)
                    return (
                      <div
                        key={`${r.resource}:${r.key}`}
                        id={optionId(i)}
                        role="option"
                        aria-selected={i === active}
                        onMouseMove={() => setActive(i)}
                        onClick={() => open(r)}
                        className={'flex items-center gap-3 rounded-xl px-3 py-2.5 ' + (i === active ? 'bg-primary-50 text-primary-900 dark:bg-primary-500/10 dark:text-white' : '')}
                      >
                        <span className={'shrink-0 [&_svg]:size-5 ' + (i === active ? 'text-primary-600 dark:text-primary-400' : 'text-gray-400')}>
                          {g.resource.icon ?? <SquaresIcon />}
                        </span>
                        <span className="min-w-0 flex-1 truncate text-sm font-medium">{r.title || `#${r.key}`}</span>
                        {details.length > 0 && <span className="shrink-0 text-xs text-gray-500 dark:text-gray-400">{details.join(' · ')}</span>}
                      </div>
                    )
                  })}
                </div>
              ))}
            </div>
          )}
        </div>
        <footer className="flex items-center gap-4 border-t border-gray-100 px-4 py-2.5 text-[11px] text-gray-400 dark:border-white/5">
          <span>
            <kbd className="font-sans">↑↓</kbd> {t('search.move')}
          </span>
          <span>
            <kbd className="font-sans">Enter</kbd> {t('search.open')}
          </span>
        </footer>
      </div>
    </dialog>
  )
}
