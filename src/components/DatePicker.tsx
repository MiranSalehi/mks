import { useEffect, useId, useLayoutEffect, useRef, useState } from 'react'
import { DatePicker as DatePickerField } from '../form'
import { calendar, type CalendarDate } from '../calendar'
import { ChevronIcon, CloseIcon } from './icons'
import { dateFormat, numberFormat, t } from '../i18n'

const fa = numberFormat({ useGrouping: false })
const fa2 = numberFormat({ minimumIntegerDigits: 2, useGrouping: false })
const displayDate = dateFormat({ dateStyle: 'long' })
const displayDateTime = dateFormat({ dateStyle: 'long', timeStyle: 'short' })
const pad = (n: number) => String(n).padStart(2, '0')

/** The stored value as a local Date; date-only values are local midnight. */
export function parseValue(value: unknown): Date | null {
  if (typeof value !== 'string' || value === '') return null
  const dateOnly = /^\d{4}-\d{2}-\d{2}$/.exec(value)
  const d = dateOnly ? new Date(`${value}T00:00:00`) : new Date(value)
  return Number.isNaN(d.getTime()) ? null : d
}

/** Builds the stored value from a calendar day and a local time. */
export function buildValue(j: CalendarDate, withTime: boolean, hour = 0, minute = 0): string {
  const g = calendar().toGregorian(j)
  if (!withTime) return `${g.gy}-${pad(g.gm)}-${pad(g.gd)}`
  return new Date(g.gy, g.gm - 1, g.gd, hour, minute).toISOString()
}

interface Props {
  id: string
  field: DatePickerField<any>
  value: unknown
  onChange: (value: unknown) => void
  disabled?: boolean
  invalid?: boolean
  a11y: Record<string, unknown>
}

export function DatePickerControl({ id, field, value, onChange, disabled, invalid, a11y }: Props) {
  const date = parseValue(value)
  const cal = calendar()
  const selected = date ? cal.fromDate(date) : null
  const today = cal.fromDate(new Date())
  const [open, setOpen] = useState(false)
  const [view, setView] = useState(() => ({ y: (selected ?? today).y, m: (selected ?? today).m }))
  const root = useRef<HTMLDivElement>(null)
  const trigger = useRef<HTMLButtonElement>(null)
  const panel = useRef<HTMLDivElement>(null)
  const dialogId = useId()
  // The calendar opens below the field, or above it when there is no room below (a field
  // at the end of a page cannot scroll the popover into view).
  const [above, setAbove] = useState(false)
  const [maxHeight, setMaxHeight] = useState<number | undefined>(undefined)

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => root.current && !root.current.contains(e.target as Node) && setOpen(false)
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  useLayoutEffect(() => {
    if (!open) return
    const place = () => {
      const t = trigger.current
      const d = panel.current
      if (!t || !d) return
      const rect = t.getBoundingClientRect()
      const height = d.scrollHeight
      const below = window.innerHeight - rect.bottom - 8
      const aboveRoom = rect.top - 8
      const flip = height > below && aboveRoom > below
      setAbove(flip)
      const room = flip ? aboveRoom : below
      setMaxHeight(height > room ? Math.max(160, room) : undefined)
    }
    place()
    window.addEventListener('scroll', place, true)
    window.addEventListener('resize', place)
    return () => {
      window.removeEventListener('scroll', place, true)
      window.removeEventListener('resize', place)
    }
  }, [open])

  function openPicker() {
    const base = selected ?? today
    setView({ y: base.y, m: base.m })
    setOpen(true)
  }

  function pick(j: CalendarDate) {
    onChange(buildValue(j, field._withTime, date?.getHours() ?? 9, date?.getMinutes() ?? 0))
    if (!field._withTime) {
      setOpen(false)
      trigger.current?.focus()
    }
  }

  function setTime(hour: number, minute: number) {
    onChange(buildValue(selected ?? today, true, hour, minute))
  }

  const days = cal.monthLength(view.y, view.m)
  const offset = cal.firstWeekday(view.y, view.m)
  const cells: (number | null)[] = [...Array(offset).fill(null), ...Array.from({ length: days }, (_, i) => i + 1)]
  const same = (a: CalendarDate | null, d: number) => !!a && a.y === view.y && a.m === view.m && a.d === d
  const label = date ? (field._withTime ? displayDateTime : displayDate).format(date) : ''

  return (
    <div ref={root} className="relative">
      <div
        className={
          'flex w-full items-center rounded-lg bg-white shadow-sm ring-1 focus-within:ring-2 focus-within:ring-primary-600 dark:bg-white/5 ' +
          (invalid ? 'ring-danger-600 dark:ring-danger-400' : 'ring-gray-950/10 dark:ring-white/20') +
          (disabled ? ' opacity-60' : '')
        }
      >
        <button
          ref={trigger}
          id={id}
          type="button"
          disabled={disabled}
          aria-haspopup="dialog"
          aria-expanded={open}
          aria-controls={dialogId}
          onClick={() => (open ? setOpen(false) : openPicker())}
          {...a11y}
          className="flex-1 px-3 py-1.5 text-start text-sm text-gray-950 outline-none disabled:cursor-not-allowed dark:text-white"
        >
          {label || <span className="text-gray-400">{t('date.pick')}</span>}
        </button>
        {date && !disabled && (
          <button type="button" onClick={() => onChange(null)} aria-label={t('date.clear')} className="px-2 text-gray-400 hover:text-gray-600 [&_svg]:size-4">
            <CloseIcon />
          </button>
        )}
      </div>

      {open && (
        <div
          ref={panel}
          id={dialogId}
          role="dialog"
          aria-label={t('date.pick')}
          style={maxHeight !== undefined ? { maxHeight } : undefined}
          onKeyDown={(e) => {
            if (e.key === 'Escape') {
              e.preventDefault()
              e.stopPropagation()
              setOpen(false)
              trigger.current?.focus()
            }
          }}
          className={
            'absolute z-30 w-72 rounded-2xl bg-white p-3 shadow-lg ring-1 ring-gray-950/5 animate-pop dark:bg-gray-900 dark:ring-white/10 ' +
            (above ? 'bottom-full mb-2 origin-bottom ' : 'top-full mt-2 origin-top ') +
            (maxHeight !== undefined ? 'overflow-y-auto' : '')
          }
        >
          <div className="mb-2 flex items-center justify-between">
            <button type="button" onClick={() => setView(cal.addMonths(view.y, view.m, -1))} aria-label={t('date.prevMonth')} className="rounded-lg p-1 hover:bg-gray-100 dark:hover:bg-white/10 [&_svg]:rotate-90 rtl:[&_svg]:-rotate-90">
              <ChevronIcon dir="up" />
            </button>
            <span className="text-sm font-semibold" aria-live="polite">
              {cal.monthName(view.m)} {fa.format(view.y)}
            </span>
            <button type="button" onClick={() => setView(cal.addMonths(view.y, view.m, 1))} aria-label={t('date.nextMonth')} className="rounded-lg p-1 hover:bg-gray-100 dark:hover:bg-white/10 [&_svg]:-rotate-90 rtl:[&_svg]:rotate-90">
              <ChevronIcon dir="up" />
            </button>
          </div>

          <div role="grid" className="grid grid-cols-7 gap-1 text-center text-sm">
            {cal.weekdays.map((w) => (
              <span key={w} role="columnheader" className="py-1 text-xs text-gray-500 dark:text-gray-400">
                {w}
              </span>
            ))}
            {cells.map((d, i) =>
              d === null ? (
                <span key={`e${i}`} />
              ) : (
                <button
                  key={d}
                  type="button"
                  onClick={() => pick({ y: view.y, m: view.m, d })}
                  aria-pressed={same(selected, d)}
                  aria-label={`${fa.format(d)} ${cal.monthName(view.m)} ${fa.format(view.y)}`}
                  className={
                    'rounded-lg py-1.5 ' +
                    (same(selected, d)
                      ? 'bg-primary-600 font-semibold text-white'
                      : same(today, d)
                        ? 'font-semibold text-primary-600 ring-1 ring-primary-600 dark:text-primary-400'
                        : 'hover:bg-gray-100 dark:hover:bg-white/10')
                  }
                >
                  {fa.format(d)}
                </button>
              ),
            )}
          </div>

          {field._withTime && (
            <div className="mt-3 flex items-center justify-center gap-2 text-sm" dir="ltr">
              <TimeInput label={t('date.hour')} max={23} value={date?.getHours() ?? 9} onChange={(h) => setTime(h, date?.getMinutes() ?? 0)} />
              <span>:</span>
              <TimeInput label={t('date.minute')} max={59} value={date?.getMinutes() ?? 0} onChange={(m) => setTime(date?.getHours() ?? 9, m)} />
            </div>
          )}

          <div className="mt-3 flex justify-between border-t border-gray-100 pt-2 text-sm dark:border-white/10">
            <button type="button" onClick={() => pick(today)} className="font-medium text-primary-600 hover:text-primary-500 dark:text-primary-400">
              {t('date.today')}
            </button>
            <button
              type="button"
              onClick={() => {
                setOpen(false)
                trigger.current?.focus()
              }}
              className="text-gray-600 hover:text-gray-800 dark:text-gray-300"
            >
              {t('common.close')}
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

function TimeInput({ label, max, value, onChange }: { label: string; max: number; value: number; onChange: (v: number) => void }) {
  return (
    <input
      type="number"
      inputMode="numeric"
      min={0}
      max={max}
      aria-label={label}
      value={value}
      title={fa2.format(value)}
      onChange={(e) => {
        const n = Number(e.target.value)
        if (Number.isInteger(n) && n >= 0 && n <= max) onChange(n)
      }}
      className="w-16 rounded-lg border-0 bg-transparent px-2 py-1 text-center ring-1 ring-gray-950/10 dark:ring-white/20"
    />
  )
}
