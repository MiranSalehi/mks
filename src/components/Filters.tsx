import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { DatePicker } from '../form'
import {
  activeFilters,
  DateRangeFilter,
  SelectFilter,
  TernaryFilter,
  ToggleFilter,
  type Filter,
  type FilterState,
} from '../filters'
import { humanize, type ResourceDefinition } from '../resource'
import { Combobox } from './Combobox'
import { DatePickerControl, parseValue } from './DatePicker'
import { inputClass } from './fields'
import { CloseIcon, FunnelIcon } from './icons'
import { toolButton } from './TableTools'
import { usePanel } from './Panel'
import { dateFormat, numberFormat, t } from '../i18n'

type States = Record<string, FilterState | undefined>

interface FiltersProps {
  resource: ResourceDefinition
  states: States
  onChange: (name: string, state: FilterState | undefined) => void
  onReset: () => void
}

const nf = numberFormat()
const day = dateFormat({ dateStyle: 'medium' })
const labelOf = (f: Filter<any>) => f._label ?? humanize(f.name)

/** The "Filters" button and its panel. Changes apply at once. */
export function FiltersMenu({ resource, states, onChange, onReset }: FiltersProps) {
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const button = useRef<HTMLButtonElement>(null)
  const count = activeFilters(resource.filters, states).length

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => root.current && !root.current.contains(e.target as Node) && setOpen(false)
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  return (
    <div ref={root} className="relative">
      <button
        ref={button}
        type="button"
        aria-expanded={open}
        aria-controls="table-filters"
        onClick={() => setOpen(!open)}
        className={toolButton + (count > 0 ? ' ring-primary-600/30 text-primary-700 dark:text-primary-300' : '')}
      >
        <FunnelIcon />
        {t('filters.title')}
        {count > 0 && (
          <span className="min-w-5 rounded-full bg-primary-600 px-1.5 text-center text-xs leading-5 font-semibold text-white" aria-label={t('filters.active', { count })}>
            {nf.format(count)}
          </span>
        )}
      </button>
      {open && (
        <div
          id="table-filters"
          role="group"
          aria-label={t('filters.title')}
          onKeyDown={(e) => {
            if (e.key === 'Escape' && !e.defaultPrevented) {
              setOpen(false)
              button.current?.focus()
            }
          }}
          className="absolute end-0 top-full z-20 mt-2 w-80 max-w-[calc(100vw-2rem)] origin-top animate-pop space-y-4 rounded-2xl bg-white p-4 shadow-lg ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10"
        >
          <div className="flex items-center justify-between">
            <p className="text-sm font-semibold text-gray-950 dark:text-white">{t('filters.title')}</p>
            {count > 0 && (
              <button type="button" onClick={onReset} className="text-sm font-medium text-danger-600 hover:text-danger-500 dark:text-danger-400">
                {t('filters.reset')}
              </button>
            )}
          </div>
          {resource.filters.map((f) => (
            <FilterControl key={f.name} resource={resource.name} filter={f} state={states[f.name]} onChange={(s) => onChange(f.name, s)} />
          ))}
        </div>
      )}
    </div>
  )
}

function FilterControl({
  resource,
  filter,
  state,
  onChange,
}: {
  resource: string
  filter: Filter<any>
  state: FilterState | undefined
  onChange: (state: FilterState | undefined) => void
}) {
  const id = `filter-${filter.name}`
  const label = labelOf(filter)
  const labelClass = 'block text-sm font-medium text-gray-950 dark:text-white'

  if (filter instanceof ToggleFilter) {
    return (
      <label className="flex items-center gap-2 text-sm text-gray-950 dark:text-white">
        <input type="checkbox" className="mks-check" checked={state === '1'} onChange={(e) => onChange(e.target.checked ? '1' : undefined)} />
        {label}
      </label>
    )
  }

  if (filter instanceof TernaryFilter) {
    return (
      <div className="space-y-1.5">
        <label htmlFor={id} className={labelClass}>
          {label}
        </label>
        <select id={id} className={inputClass} value={state ?? ''} onChange={(e) => onChange(e.target.value || undefined)}>
          <option value="">{filter._placeholder}</option>
          <option value="true">{filter._trueLabel}</option>
          <option value="false">{filter._falseLabel}</option>
        </select>
      </div>
    )
  }

  if (filter instanceof DateRangeFilter) {
    const { from, until } = filter.range(state)
    const picker = (name: string) => new DatePicker(name)
    return (
      <fieldset className="space-y-1.5">
        <legend id={`${id}-legend`} className={labelClass}>
          {label}
        </legend>
        {(['from', 'until'] as const).map((side) => (
          <div key={side} className="flex items-center gap-2">
            <span className="w-6 shrink-0 text-sm text-gray-500 dark:text-gray-400" id={`${id}-${side}-label`}>
              {side === 'from' ? filter._fromLabel : filter._untilLabel}
            </span>
            <div className="flex-1">
              <DatePickerControl
                id={`${id}-${side}`}
                field={picker(side)}
                value={side === 'from' ? from : until}
                onChange={(v) => {
                  const value = typeof v === 'string' ? v : ''
                  onChange(DateRangeFilter.state(side === 'from' ? value : from, side === 'until' ? value : until))
                }}
                // "Created at, from, <the chosen date>": the button's own text is part of its name.
                a11y={{ 'aria-labelledby': `${id}-legend ${id}-${side}-label ${id}-${side}` }}
              />
            </div>
          </div>
        ))}
      </fieldset>
    )
  }

  if (filter instanceof SelectFilter) {
    const values = filter.values(state)
    const set = (vals: string[]) => onChange(vals.length ? vals.join(',') : undefined)

    if (filter._relationship) {
      return (
        <div className="space-y-1.5">
          <label htmlFor={id} className={labelClass}>
            {label}
          </label>
          <Combobox
            id={id}
            resource={resource}
            relation={filter._relationship}
            multiple={filter._multiple}
            value={filter._multiple ? values : (values[0] ?? null)}
            onChange={(v) => set(v == null ? [] : Array.isArray(v) ? v.map(String) : [String(v)])}
            a11y={{}}
          />
        </div>
      )
    }

    if (filter._multiple) {
      return (
        <fieldset className="space-y-1.5">
          <legend className={labelClass}>{label}</legend>
          {Object.entries(filter._options).map(([value, text]) => (
            <label key={value} className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input
                type="checkbox"
                className="mks-check"
                checked={values.includes(value)}
                onChange={(e) => set(e.target.checked ? [...values, value] : values.filter((v) => v !== value))}
              />
              {text}
            </label>
          ))}
        </fieldset>
      )
    }

    return (
      <div className="space-y-1.5">
        <label htmlFor={id} className={labelClass}>
          {label}
        </label>
        <select id={id} className={inputClass} value={values[0] ?? ''} onChange={(e) => set(e.target.value ? [e.target.value] : [])}>
          <option value="">{t('common.all')}</option>
          {Object.entries(filter._options).map(([value, text]) => (
            <option key={value} value={value}>
              {text}
            </option>
          ))}
        </select>
      </div>
    )
  }
  return null
}

/** Chips for the active filters, each with a remove button. */
export function ActiveFilters({ resource, states, onChange, onReset }: FiltersProps) {
  const active = activeFilters(resource.filters, states)
  if (active.length === 0) return null
  return (
    <div className="flex animate-pop flex-wrap items-center gap-2 border-b border-gray-100 px-4 py-2.5 text-sm dark:border-white/5">
      <span className="text-gray-500 dark:text-gray-400">{t('filters.activeLabel')}</span>
      {active.map((f) => (
        <span key={f.name} className="inline-flex items-center gap-1 rounded-full bg-primary-50 py-1 ps-2.5 pe-1.5 text-xs font-medium text-primary-700 ring-1 ring-primary-600/15 dark:bg-primary-500/10 dark:text-primary-300 dark:ring-primary-400/20">
          <ChipText resource={resource.name} filter={f} state={states[f.name]!} />
          <button type="button" onClick={() => onChange(f.name, undefined)} aria-label={t('filters.remove', { name: labelOf(f) })} className="rounded-full p-0.5 hover:bg-primary-600/10 [&_svg]:size-3.5">
            <CloseIcon />
          </button>
        </span>
      ))}
      {active.length > 1 && (
        <button type="button" onClick={onReset} className="ms-auto text-xs font-medium text-gray-600 hover:text-gray-800 dark:text-gray-300">
          {t('filters.removeAll')}
        </button>
      )}
    </div>
  )
}

function ChipText({ resource, filter, state }: { resource: string; filter: Filter<any>; state: FilterState }) {
  const label = labelOf(filter)
  if (filter instanceof ToggleFilter) return <>{label}</>
  if (filter instanceof TernaryFilter) return <>{`${label}: ${state === 'true' ? filter._trueLabel : filter._falseLabel}`}</>
  if (filter instanceof DateRangeFilter) {
    const { from, until } = filter.range(state)
    const fmt = (d: string) => day.format(parseValue(d)!)
    const text = from && until ? t('filters.range', { from: fmt(from), until: fmt(until) }) : from ? t('filters.fromDate', { date: fmt(from) }) : t('filters.untilDate', { date: fmt(until) })
    return <>{`${label}: ${text}`}</>
  }
  if (filter instanceof SelectFilter) {
    const values = filter.values(state)
    if (filter._relationship) return <RelationChipText resource={resource} relation={filter._relationship} label={label} values={values} />
    return <>{`${label}: ${values.map((v) => filter._options[v] ?? v).join(', ')}`}</>
  }
  return <>{label}</>
}

function RelationChipText({ resource, relation, label, values }: { resource: string; relation: string; label: string; values: string[] }) {
  const { api } = usePanel()
  // The same query the combobox uses for its labels, so it is usually cached.
  const labels = useQuery({
    queryKey: ['options', resource, relation, { values: [...values].sort() }],
    queryFn: () => api.relationOptions(resource, relation, { values }),
    staleTime: 60_000,
  })
  const names = values.map((v) => labels.data?.data.find((o) => String(o.value) === v)?.label ?? (labels.isPending ? '…' : v))
  return <>{`${label}: ${names.join(', ')}`}</>
}
