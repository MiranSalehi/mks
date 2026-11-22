import type { ResourceRecord } from './protocol'
import { readPath, type ColumnPath } from './table'

/**
 * Groups rows under headings by a column, like Filament's table groups. Rows are sorted by
 * the column first, so the server must allow sorting by it.
 */
export class Group<T = ResourceRecord> {
  readonly name: string
  _label?: string
  _titles?: Record<string, string>
  _title?: ColumnPath<T> | ((record: T) => string)

  constructor(name: ColumnPath<T>) {
    this.name = name
  }

  label(label: string): this {
    this._label = label
    return this
  }

  /** Headings for values, e.g. { draft: 'پیش‌نویس' }. */
  titles(titles: Record<string, string>): this {
    this._titles = titles
    return this
  }

  /** The heading from another path (e.g. "category.name") or a function of the first record. */
  title(source: ColumnPath<T> | ((record: T) => string)): this {
    this._title = source
    return this
  }

  /** Relations the heading needs included. */
  get relations(): string[] {
    const path = typeof this._title === 'string' ? this._title : ''
    const dot = path.indexOf('.')
    return dot > 0 ? [path.slice(0, dot)] : []
  }

  keyOf(record: ResourceRecord): string {
    const v = readPath(record, this.name)
    return v == null ? '' : String(v)
  }

  titleOf(record: ResourceRecord): string {
    const key = this.keyOf(record)
    let title: unknown
    if (typeof this._title === 'function') title = this._title(record as T)
    else if (typeof this._title === 'string') title = readPath(record, this._title)
    else title = this._titles?.[key] ?? key
    return title == null || title === '' ? '—' : String(title)
  }
}

export function groupBuilder<T>() {
  return { make: (name: ColumnPath<T>) => new Group<T>(name) }
}

export type GroupBuilder<T> = ReturnType<typeof groupBuilder<T>>
