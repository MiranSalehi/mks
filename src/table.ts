import type { ResourceRecord } from './protocol'
import { t } from './i18n'

/** A column path: a field of the record, or "relation.field" for an included relation. */
export type ColumnPath<T> = Extract<keyof T, string> | `${string}.${string}`

export type Color = 'gray' | 'primary' | 'success' | 'warning' | 'danger' | 'info'

/** A value, or a function of the cell value and its record. */
export type ValueOr<V, T> = V | ((value: unknown, record: T) => V)

export function resolve<V, T>(v: ValueOr<V, T> | undefined, value: unknown, record: T): V | undefined {
  return typeof v === 'function' ? (v as (value: unknown, record: T) => V)(value, record) : v
}

export type Align = 'start' | 'center' | 'end'

/** Grid columns an infolist entry takes, or "full". */
export type Span = number | 'full'

/** Aggregates the server computes over all filtered records (it must allow them: Summaries). */
export type SummaryFunction = 'sum' | 'avg' | 'min' | 'max'

export abstract class Column<T = ResourceRecord> {
  abstract readonly kind: string
  readonly name: string
  _label?: string
  _sortable = false
  _searchable = false
  _align: Align = 'start'
  _description?: ColumnPath<T> | ((record: T) => string | null | undefined)
  _format?: (value: unknown, record: T) => string
  _placeholder = '—'
  /** Whether the user can hide the column, and whether it starts hidden. */
  _toggleable: 'no' | 'shown' | 'hidden' = 'no'
  _summaries: SummaryFunction[] = []
  // Infolists only (columns double as infolist entries):
  _columnSpan?: Span
  _visible?: (record: T) => boolean

  constructor(name: ColumnPath<T>) {
    this.name = name
  }

  label(label: string): this {
    this._label = label
    return this
  }

  /** Requests server-side sorting. The server must also allow it in its whitelist. */
  sortable(sortable = true): this {
    this._sortable = sortable
    return this
  }

  /** Shows the search box. Which columns are searched is decided by the server whitelist. */
  searchable(searchable = true): this {
    this._searchable = searchable
    return this
  }

  alignEnd(): this {
    this._align = 'end'
    return this
  }

  alignCenter(): this {
    this._align = 'center'
    return this
  }

  /** A second, smaller line under the value: another column path or a function. */
  description(source: ColumnPath<T> | ((record: T) => string | null | undefined)): this {
    this._description = source
    return this
  }

  /** Custom text for the value, like Filament's formatStateUsing(). */
  format(fn: (value: unknown, record: T) => string): this {
    this._format = fn
    return this
  }

  /** Lets the user hide the column from the "Columns" menu (remembered in this browser). */
  toggleable(hiddenByDefault = false): this {
    this._toggleable = hiddenByDefault ? 'hidden' : 'shown'
    return this
  }

  /** Shows aggregates of all filtered records under the column, e.g. summarize('sum', 'avg'). */
  summarize(...fns: SummaryFunction[]): this {
    this._summaries = fns
    return this
  }

  /** In an infolist: grid columns the entry takes, or "full". */
  columnSpan(span: Span): this {
    this._columnSpan = span
    return this
  }

  /** In an infolist: shows the entry only for some records. */
  visible(fn: (record: T) => boolean): this {
    this._visible = fn
    return this
  }

  /** Shown when the value is empty. */
  placeholder(text: string): this {
    this._placeholder = text
    return this
  }

  /** Relations this column needs included. */
  get relations(): string[] {
    const out = new Set<string>()
    for (const path of [this.name, typeof this._description === 'string' ? this._description : '']) {
      const dot = path.indexOf('.')
      if (dot > 0) out.add(path.slice(0, dot))
    }
    return [...out]
  }

  value(record: ResourceRecord): unknown {
    return readPath(record, this.name)
  }

  descriptionOf(record: ResourceRecord): string | null {
    if (!this._description) return null
    const v =
      typeof this._description === 'function'
        ? this._description(record as T)
        : readPath(record, this._description)
    return v == null || v === '' ? null : String(v)
  }
}

export function readPath(record: ResourceRecord, path: string): unknown {
  return path.split('.').reduce<unknown>(
    (v, key) => (v != null && typeof v === 'object' ? (v as Record<string, unknown>)[key] : undefined),
    record,
  )
}

export class TextColumn<T = ResourceRecord> extends Column<T> {
  readonly kind = 'text'
  _limit?: number
  _numeric?: Intl.NumberFormatOptions
  _copyable = false
  _url?: (record: T) => string | null | undefined
  _openInNewTab = false
  _color?: ValueOr<Color, T>
  _weight?: 'medium' | 'semibold' | 'bold'
  _wrap = false

  limit(chars: number): this {
    this._limit = chars
    return this
  }

  /** Formats numbers for the panel locale (Persian digits in fa). */
  numeric(options: Intl.NumberFormatOptions = {}): this {
    this._numeric = options
    return this
  }

  money(currency = 'IRR'): this {
    return this.numeric({ style: 'currency', currency, maximumFractionDigits: 0 })
  }

  /** Adds a button that copies the value. */
  copyable(copyable = true): this {
    this._copyable = copyable
    return this
  }

  /** Makes the value a link. */
  url(fn: (record: T) => string | null | undefined, openInNewTab = false): this {
    this._url = fn
    this._openInNewTab = openInNewTab
    return this
  }

  color(color: ValueOr<Color, T>): this {
    this._color = color
    return this
  }

  weight(weight: 'medium' | 'semibold' | 'bold'): this {
    this._weight = weight
    return this
  }

  /** Lets long text wrap instead of staying on one line. */
  wrap(wrap = true): this {
    this._wrap = wrap
    return this
  }
}

export class BadgeColumn<T = ResourceRecord> extends Column<T> {
  readonly kind = 'badge'
  _colors: Record<string, Color> = {}
  _labels: Record<string, string> = {}

  colors(colors: Record<string, Color>): this {
    this._colors = colors
    return this
  }

  labels(labels: Record<string, string>): this {
    this._labels = labels
    return this
  }
}

export class BooleanColumn<T = ResourceRecord> extends Column<T> {
  readonly kind = 'boolean'
  _trueColor: Color = 'success'
  _falseColor: Color = 'gray'
  _trueLabel = t('common.yes')
  _falseLabel = t('common.no')

  constructor(name: ColumnPath<T>) {
    super(name)
    this._align = 'center'
  }

  colors(whenTrue: Color, whenFalse: Color): this {
    this._trueColor = whenTrue
    this._falseColor = whenFalse
    return this
  }

  /** Text for screen readers and the tooltip. */
  labels(whenTrue: string, whenFalse: string): this {
    this._trueLabel = whenTrue
    this._falseLabel = whenFalse
    return this
  }
}

export class ImageColumn<T = ResourceRecord> extends Column<T> {
  readonly kind = 'image'
  _size = 40
  _circular = false
  _src?: (value: unknown, record: T) => string | null | undefined

  size(px: number): this {
    this._size = px
    return this
  }

  circular(circular = true): this {
    this._circular = circular
    return this
  }

  /** Builds the image URL from the stored value. Default: paths under /storage, URLs as they are. */
  src(fn: (value: unknown, record: T) => string | null | undefined): this {
    this._src = fn
    return this
  }
}

export class DateColumn<T = ResourceRecord> extends Column<T> {
  readonly kind = 'date'
  _withTime = false
  _since = false

  dateTime(withTime = true): this {
    this._withTime = withTime
    return this
  }

  /** Shows relative time ("۳ روز پیش"), with the full date as a tooltip. */
  since(since = true): this {
    this._since = since
    return this
  }
}

export function tableBuilder<T>() {
  return {
    text: (name: ColumnPath<T>) => new TextColumn<T>(name),
    badge: (name: ColumnPath<T>) => new BadgeColumn<T>(name),
    boolean: (name: ColumnPath<T>) => new BooleanColumn<T>(name),
    image: (name: ColumnPath<T>) => new ImageColumn<T>(name),
    date: (name: ColumnPath<T>) => new DateColumn<T>(name),
  }
}

export type TableBuilder<T> = ReturnType<typeof tableBuilder<T>>
