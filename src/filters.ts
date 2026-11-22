import type { ListFilter, ResourceRecord } from './protocol'
import { t } from './i18n'

/**
 * A filter's state as one string, kept in the URL as filter[name]=state, so reloads and
 * shared links show the same rows. undefined means the filter is off.
 */
export type FilterState = string

/** A column of T, or the name of a custom filter defined on the server (panel.Filter). */
export type FilterName<T> = Extract<keyof T, string> | (string & {})

export abstract class Filter<T = ResourceRecord> {
  abstract readonly kind: string
  readonly name: string
  _label?: string

  constructor(name: FilterName<T>) {
    this.name = name
  }

  label(label: string): this {
    this._label = label
    return this
  }

  /** What the server receives for a state; nothing when the state means "off" or is invalid. */
  abstract params(state: FilterState): ListFilter[]

  /** The operators the server must allow for this filter (for dev warnings). */
  abstract get operators(): ListFilter['op'][]
}

const splitValues = (state: FilterState) => state.split(',').filter(Boolean)

export class SelectFilter<T = ResourceRecord> extends Filter<T> {
  readonly kind = 'select'
  _options: Record<string, string> = {}
  _relationship?: string
  _multiple = false

  options(options: Record<string, string>): this {
    this._options = options
    return this
  }

  /** Options are records of a relation of this resource, searched on the server. */
  relationship(relation: string): this {
    this._relationship = relation
    return this
  }

  /** Allows choosing several values; the server gets the "in" operator. */
  multiple(multiple = true): this {
    this._multiple = multiple
    return this
  }

  values(state: FilterState | undefined): string[] {
    return state ? splitValues(state) : []
  }

  params(state: FilterState): ListFilter[] {
    const values = splitValues(state)
    if (values.length === 0) return []
    if (this._multiple) return [{ column: this.name, op: 'in', value: values }]
    return [{ column: this.name, op: 'eq', value: values[0] }]
  }

  get operators(): ListFilter['op'][] {
    return [this._multiple ? 'in' : 'eq']
  }
}

/** Three states: all, yes, no. */
export class TernaryFilter<T = ResourceRecord> extends Filter<T> {
  readonly kind = 'ternary'
  _trueLabel = t('common.yes')
  _falseLabel = t('common.no')
  _placeholder = t('common.all')
  _nullable = false

  labels(whenTrue: string, whenFalse: string, all?: string): this {
    this._trueLabel = whenTrue
    this._falseLabel = whenFalse
    if (all) this._placeholder = all
    return this
  }

  /**
   * "Yes" means the column has a value, "no" that it is empty (the server's "null"
   * operator), e.g. published_at for "published".
   */
  nullable(nullable = true): this {
    this._nullable = nullable
    return this
  }

  params(state: FilterState): ListFilter[] {
    if (state !== 'true' && state !== 'false') return []
    if (this._nullable) return [{ column: this.name, op: 'null', value: state === 'true' ? 'false' : 'true' }]
    return [{ column: this.name, op: 'eq', value: state }]
  }

  get operators(): ListFilter['op'][] {
    return [this._nullable ? 'null' : 'eq']
  }
}

const DATE = /^\d{4}-\d{2}-\d{2}$/

/** Local midnight of a "YYYY-MM-DD" day, plus some days. */
function localDay(day: string, plusDays = 0): Date | null {
  if (!DATE.test(day)) return null
  const [y, m, d] = day.split('-').map(Number)
  const date = new Date(y, m - 1, d + plusDays)
  return Number.isNaN(date.getTime()) ? null : date
}

/**
 * From and until days, both included. The state is "from..until" with Gregorian days
 * ("2026-11-01..2026-11-19"); either side may be empty.
 */
export class DateRangeFilter<T = ResourceRecord> extends Filter<T> {
  readonly kind = 'date-range'
  _fromLabel = t('common.from')
  _untilLabel = t('common.until')

  labels(from: string, until: string): this {
    this._fromLabel = from
    this._untilLabel = until
    return this
  }

  range(state: FilterState | undefined): { from: string; until: string } {
    const [from = '', until = ''] = (state ?? '').split('..')
    return { from: DATE.test(from) ? from : '', until: DATE.test(until) ? until : '' }
  }

  static state(from: string, until: string): FilterState | undefined {
    return from || until ? `${from}..${until}` : undefined
  }

  // Days are the user's days: local midnight, sent as an instant. "Until" includes the
  // whole day, so it is "before the next midnight".
  params(state: FilterState): ListFilter[] {
    const { from, until } = this.range(state)
    const out: ListFilter[] = []
    const start = localDay(from)
    const end = localDay(until, 1)
    if (start) out.push({ column: this.name, op: 'gte', value: start.toISOString() })
    if (end) out.push({ column: this.name, op: 'lt', value: end.toISOString() })
    return out
  }

  get operators(): ListFilter['op'][] {
    return ['gte', 'lt']
  }
}

/** A checkbox: on sends filter[name]=1. Usually a custom filter, like "my posts". */
export class ToggleFilter<T = ResourceRecord> extends Filter<T> {
  readonly kind = 'toggle'

  params(state: FilterState): ListFilter[] {
    return state === '1' ? [{ column: this.name, op: 'eq', value: '1' }] : []
  }

  get operators(): ListFilter['op'][] {
    return ['eq']
  }
}

export function filterBuilder<T>() {
  return {
    select: (name: FilterName<T>) => new SelectFilter<T>(name),
    ternary: (name: FilterName<T>) => new TernaryFilter<T>(name),
    dateRange: (name: FilterName<T>) => new DateRangeFilter<T>(name),
    toggle: (name: FilterName<T>) => new ToggleFilter<T>(name),
  }
}

export type FilterBuilder<T> = ReturnType<typeof filterBuilder<T>>

/** The server parameters for the active filters. */
export function filterParams(filters: Filter<any>[], states: Record<string, FilterState | undefined>): ListFilter[] {
  return filters.flatMap((f) => {
    const state = states[f.name]
    return state ? f.params(state) : []
  })
}

/** Names of filters whose state means something (an empty range is not active). */
export function activeFilters(filters: Filter<any>[], states: Record<string, FilterState | undefined>): Filter<any>[] {
  return filters.filter((f) => {
    const state = states[f.name]
    return !!state && f.params(state).length > 0
  })
}

// --- URL ----------------------------------------------------------------------------------

const FILTER_KEY = /^filter\[([a-z_][a-z0-9_]*)\]$/

export const filterKey = (name: string) => `filter[${name}]`

/** Filter states from the URL search object. */
export function readFilterStates(search: Record<string, unknown>): Record<string, FilterState> {
  const out: Record<string, FilterState> = {}
  for (const [key, value] of Object.entries(search)) {
    const m = FILTER_KEY.exec(key)
    if (!m || value === undefined || value === null) continue
    const s = Array.isArray(value) ? value.join(',') : String(value)
    if (s !== '') out[m[1]] = s
  }
  return out
}

/**
 * The value written to the URL. The router writes JSON-looking strings ("1", "true")
 * quoted, to keep their type; storing them as numbers and booleans keeps URLs clean.
 */
export function urlValue(state: FilterState): string | number | boolean {
  try {
    const v: unknown = JSON.parse(state)
    if ((typeof v === 'number' || typeof v === 'boolean') && String(v) === state) return v
  } catch {
    // not JSON: a plain string
  }
  return state
}
