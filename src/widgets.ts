import type { ReactNode } from 'react'
import type { ResourceRecord } from './protocol'
import { tableBuilder, type Color, type Column, type TableBuilder } from './table'
import { dateFormat, localeTag, t } from './i18n'

/** Width on the dashboard: one of two columns, or the whole row. */
export type WidgetSpan = 1 | 'full'

/**
 * A dashboard widget. Stats and charts show numbers from the server widget with the same
 * name (panel.Widget in Go); a table widget lists records of a resource.
 */
export abstract class Widget {
  abstract readonly kind: string
  readonly name: string
  _heading?: string
  _description?: string
  _columnSpan: WidgetSpan = 1
  /** Seconds between refreshes; none by default. */
  _poll?: number

  constructor(name: string) {
    this.name = name
  }

  heading(text: string): this {
    this._heading = text
    return this
  }

  description(text: string): this {
    this._description = text
    return this
  }

  columnSpan(span: WidgetSpan): this {
    this._columnSpan = span
    return this
  }

  /** Refresh every few seconds, e.g. poll(30). */
  poll(seconds: number): this {
    this._poll = seconds
    return this
  }
}

// --- stats --------------------------------------------------------------------------------

export class Stat {
  readonly key: string
  _label?: string
  _icon?: ReactNode
  _color: Color = 'primary'
  _format?: (value: unknown) => string
  _description?: string

  constructor(key: string) {
    this.key = key
  }

  label(label: string): this {
    this._label = label
    return this
  }

  icon(icon: ReactNode): this {
    this._icon = icon
    return this
  }

  color(color: Color): this {
    this._color = color
    return this
  }

  /** How the value is written; numbers get Persian digits by default. */
  format(fn: (value: unknown) => string): this {
    this._format = fn
    return this
  }

  /** A short line under the value, e.g. "vs last month". */
  description(text: string): this {
    this._description = text
    return this
  }
}

export class StatsWidget extends Widget {
  readonly kind = 'stats'
  _stats: Stat[] = []
  override _columnSpan: WidgetSpan = 'full'

  stats(fn: (s: { make: (key: string) => Stat }) => Stat[]): this {
    this._stats = fn({ make: (key) => new Stat(key) })
    return this
  }
}

// --- charts -------------------------------------------------------------------------------

export type ChartType = 'line' | 'bar' | 'doughnut'

export interface DatasetStyle {
  label: string
  color: Color
}

export class ChartWidget extends Widget {
  readonly kind = 'chart'
  _type: ChartType = 'line'
  _datasets: Record<string, DatasetStyle> = {}
  _labelFormat?: 'day' | 'month' | ((label: string) => string)
  _labels: Record<string, string> = {}
  _colors: Record<string, Color> = {}
  _height = 260

  type(type: ChartType): this {
    this._type = type
    return this
  }

  /** Label and color of a dataset the server sends (by key). */
  dataset(key: string, label: string, color: Color = 'primary'): this {
    this._datasets[key] = { label, color }
    return this
  }

  /** Labels are dates: "day" ("2026-11-19") or "month" ("2026-11"), or format them yourself. */
  formatLabels(format: 'day' | 'month' | ((label: string) => string)): this {
    this._labelFormat = format
    return this
  }

  /** Words for label values, e.g. { draft: 'Draft' }. */
  labels(map: Record<string, string>): this {
    this._labels = map
    return this
  }

  /** Colors for label values (doughnut segments, bars), e.g. { published: 'success' }. */
  colors(map: Record<string, Color>): this {
    this._colors = map
    return this
  }

  height(px: number): this {
    this._height = px
    return this
  }
}

// --- table --------------------------------------------------------------------------------

/** The latest records of a resource, with the resource's own list API and permissions. */
export class TableWidget<T = ResourceRecord> extends Widget {
  readonly kind = 'table'
  _resource: string
  _columns?: Column<T>[]
  _limit = 5
  _sort?: string
  override _columnSpan: WidgetSpan = 'full'

  constructor(name: string) {
    super(name)
    this._resource = name
  }

  resource(name: string): this {
    this._resource = name
    return this
  }

  columns(fn: (t: TableBuilder<T>) => Column<T>[]): this {
    this._columns = fn(tableBuilder<T>())
    return this
  }

  limit(n: number): this {
    this._limit = n
    return this
  }

  /** E.g. "-created_at" (must be sortable on the server). */
  sort(sort: string): this {
    this._sort = sort
    return this
  }
}

/**
 * The signed-in user with a sign-out button (Filament's account widget). It needs no server
 * widget and every user sees it.
 */
export class AccountWidget extends Widget {
  readonly kind = 'account'
}

export function widgetBuilder() {
  return {
    account: (name = 'account') => new AccountWidget(name),
    stats: (name: string) => new StatsWidget(name),
    chart: (name: string) => new ChartWidget(name),
    table: <T = ResourceRecord>(name: string) => new TableWidget<T>(name),
  }
}

export type WidgetBuilder = ReturnType<typeof widgetBuilder>

export interface DashboardDefinition {
  heading: string
  widgets: Widget[]
}

export function defineDashboard(options: { heading?: string; widgets: (w: WidgetBuilder) => Widget[] }): DashboardDefinition {
  return { heading: options.heading ?? t('panel.dashboard'), widgets: options.widgets(widgetBuilder()) }
}

/** Server data of a stats widget. */
export type StatsData = Record<string, { value: unknown; change?: number; trend?: number[] }>

/** Server data of a chart widget. */
export interface ChartData {
  labels: string[]
  datasets: { key: string; data: number[] }[]
}

// Server months are calendar (Gregorian) months; days map one to one, so they use the locale's calendar.
const monthFmt = () => new Intl.DateTimeFormat(localeTag() + '-u-ca-gregory', { month: 'short', year: '2-digit', timeZone: 'UTC' })
const dayFmt = () => dateFormat({ day: 'numeric', month: 'short', timeZone: 'UTC' })

/** A chart label as shown: translated, or a formatted date. */
export function chartLabel(widget: ChartWidget, label: string): string {
  if (widget._labels[label]) return widget._labels[label]
  const f = widget._labelFormat
  if (typeof f === 'function') return f(label)
  // Server months are calendar (Gregorian) months, so they are named as such.
  if (f === 'month' && /^\d{4}-\d{2}$/.test(label)) return monthFmt().format(new Date(`${label}-15T00:00:00Z`))
  if (f === 'day' && /^\d{4}-\d{2}-\d{2}$/.test(label)) return dayFmt().format(new Date(`${label}T00:00:00Z`))
  return label
}
