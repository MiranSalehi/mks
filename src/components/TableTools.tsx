import { useCallback, useEffect, useRef, useState } from 'react'
import type { ResourceDefinition } from '../resource'
import { humanize } from '../resource'
import { TextColumn, type Column, type SummaryFunction } from '../table'
import { ColumnsIcon } from './icons'
import { numberFormat, t } from '../i18n'

// --- hidden columns -----------------------------------------------------------------------

const storageKey = (resource: string) => `mks:${resource}:columns`

/**
 * Which toggleable columns are hidden, remembered in this browser. Only the user's own
 * choices are stored, so a column's default can still change later.
 */
export function useHiddenColumns(resource: ResourceDefinition): [Set<string>, (name: string, hidden: boolean) => void] {
  const read = useCallback((): Record<string, boolean> => {
    try {
      const v: unknown = JSON.parse(localStorage.getItem(storageKey(resource.name)) ?? '{}')
      return v && typeof v === 'object' ? (v as Record<string, boolean>) : {}
    } catch {
      return {}
    }
  }, [resource.name])
  const [choices, setChoices] = useState(read)
  useEffect(() => setChoices(read()), [read])

  const hidden = new Set(
    resource.columns
      .filter((c) => c._toggleable !== 'no' && (choices[c.name] ?? c._toggleable === 'hidden'))
      .map((c) => c.name),
  )
  const set = (name: string, isHidden: boolean) => {
    const next = { ...choices, [name]: isHidden }
    setChoices(next)
    try {
      localStorage.setItem(storageKey(resource.name), JSON.stringify(next))
    } catch {
      // storage full or disabled: the choice lasts for this page only
    }
  }
  return [hidden, set]
}

// --- popover menus ------------------------------------------------------------------------

export function useDismiss(open: boolean, close: () => void) {
  const root = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => root.current && !root.current.contains(e.target as Node) && close()
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open, close])
  return root
}

export const toolButton =
  'inline-flex items-center gap-1.5 rounded-lg bg-white px-3 py-1.5 text-sm font-medium text-gray-700 shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 hover:ring-gray-950/20 active:scale-[0.98] dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10'

export function ColumnsMenu({ resource, hidden, onChange }: { resource: ResourceDefinition; hidden: Set<string>; onChange: (name: string, hidden: boolean) => void }) {
  const [open, setOpen] = useState(false)
  const close = useCallback(() => setOpen(false), [])
  const root = useDismiss(open, close)
  const button = useRef<HTMLButtonElement>(null)
  const toggleable = resource.columns.filter((c) => c._toggleable !== 'no')
  if (toggleable.length === 0) return null
  return (
    <div ref={root} className="relative">
      <button ref={button} type="button" aria-expanded={open} aria-controls="table-columns" onClick={() => setOpen(!open)} className={toolButton}>
        <ColumnsIcon />
        {t('list.columns')}
      </button>
      {open && (
        <fieldset
          id="table-columns"
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              setOpen(false)
              button.current?.focus()
            }
          }}
          className="absolute end-0 top-full z-20 mt-2 w-56 origin-top animate-pop space-y-2.5 rounded-2xl bg-white p-4 shadow-lg ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10"
        >
          <legend className="sr-only">{t('list.shownColumns')}</legend>
          {toggleable.map((c) => (
            <label key={c.name} className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input type="checkbox" className="mks-check" checked={!hidden.has(c.name)} onChange={(e) => onChange(c.name, !e.target.checked)} />
              {c._label || humanize(c.name)}
            </label>
          ))}
        </fieldset>
      )}
    </div>
  )
}

// --- summaries ----------------------------------------------------------------------------

export const summaryLabels: Record<SummaryFunction, string> = { sum: t('summary.sum'), avg: t('summary.avg'), min: t('summary.min'), max: t('summary.max') }

/** A summary value in the column's number format; averages get one decimal. */
export function formatSummary(column: Column<any>, fn: SummaryFunction, value: number | null | undefined): string {
  if (value == null) return '—'
  const base = column instanceof TextColumn && column._numeric ? column._numeric : {}
  const options: Intl.NumberFormatOptions = fn === 'avg' ? { ...base, maximumFractionDigits: 1 } : base
  return numberFormat(options).format(value)
}
