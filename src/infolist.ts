import type { ResourceRecord } from './protocol'
import {
  BadgeColumn,
  BooleanColumn,
  Column,
  DateColumn,
  ImageColumn,
  readPath,
  TextColumn,
  type Color,
  type ColumnPath,
  type Span,
} from './table'
import { t } from './i18n'

/**
 * Infolists show one record, read-only, like Filament's view page. Entries for plain
 * values are the table's columns (same options: text, badge, boolean, date, image);
 * richer values have their own entries; sections and grids lay them out.
 */
export type InfoNode<T = ResourceRecord> = Column<T> | Entry<T> | InfoLayout<T>


/** Entries that are not table columns. */
export abstract class Entry<T = ResourceRecord> {
  abstract readonly kind: string
  readonly name: string
  _label?: string
  _placeholder = '—'
  _columnSpan?: Span
  _visible?: (record: T) => boolean

  constructor(name: ColumnPath<T>) {
    this.name = name
  }

  /** Grid columns the entry takes, or "full" for the whole row. */
  columnSpan(span: Span): this {
    this._columnSpan = span
    return this
  }

  /** Shows the entry only for some records. */
  visible(fn: (record: T) => boolean): this {
    this._visible = fn
    return this
  }

  label(label: string): this {
    this._label = label
    return this
  }

  placeholder(text: string): this {
    this._placeholder = text
    return this
  }

  value(record: ResourceRecord): unknown {
    return readPath(record, this.name)
  }
}

/** Sanitized HTML, e.g. a rich text field. */
export class HtmlEntry<T = ResourceRecord> extends Entry<T> {
  readonly kind = 'html'
}

/** Markdown, rendered and sanitized. */
export class MarkdownEntry<T = ResourceRecord> extends Entry<T> {
  readonly kind = 'markdown'
}

/**
 * A list as chips: strings (e.g. keywords) or records of a relation (e.g. tags, with
 * title('name')).
 */
export class ListEntry<T = ResourceRecord> extends Entry<T> {
  readonly kind = 'list'
  _title?: string
  _color: Color = 'gray'

  /** For lists of records: the field to show, e.g. "name". */
  title(field: string): this {
    this._title = field
    return this
  }

  color(color: Color): this {
    this._color = color
    return this
  }

  items(record: ResourceRecord): string[] {
    const v = this.value(record)
    if (!Array.isArray(v)) return []
    return v
      .map((item) => {
        if (item != null && typeof item === 'object') return this._title ? (item as Record<string, unknown>)[this._title] : undefined
        return item
      })
      .filter((x) => x != null && x !== '')
      .map(String)
  }
}

/** An object as a two-column table (e.g. a KeyValue field). */
export class KeyValueEntry<T = ResourceRecord> extends Entry<T> {
  readonly kind = 'key-value'
  _keyLabel = t('common.key')
  _valueLabel = t('common.value')

  labels(key: string, value: string): this {
    this._keyLabel = key
    this._valueLabel = value
    return this
  }
}

/** The item type of a list field, e.g. FAQ for faqs: FAQ[]. */
export type ItemOf<V> = NonNullable<V> extends readonly (infer I)[] ? I : ResourceRecord

/** A list of objects (e.g. a Repeater field), each shown with its own entries. */
export class RepeatableEntry<T = ResourceRecord, I = ResourceRecord> extends Entry<T> {
  readonly kind = 'repeatable'
  _schema: InfoNode<any>[] = []
  _columns = 1

  /** Entries for one item, with paths of the item type: i.text('question'). */
  schema(fn: (i: InfolistBuilder<I>) => InfoNode<I>[]): this {
    this._schema = fn(infolistBuilder<I>())
    return this
  }

  columns(n: number): this {
    this._columns = n
    return this
  }
}

// --- layouts ------------------------------------------------------------------------------

export abstract class InfoLayout<T = ResourceRecord> {
  abstract readonly kind: string
  _children: InfoNode<T>[] = []
  _columns = 2

  schema(nodes: InfoNode<T>[]): this {
    this._children = nodes
    return this
  }

  columns(n: number): this {
    this._columns = n
    return this
  }
}

/** A card with a heading. */
export class InfoSection<T = ResourceRecord> extends InfoLayout<T> {
  readonly kind = 'section'
  readonly heading: string
  _description?: string
  /** Shown beside the main column on wide screens. */
  _aside = false

  constructor(heading: string) {
    super()
    this.heading = heading
  }

  description(text: string): this {
    this._description = text
    return this
  }

  /** Put the section in the side column (e.g. status and dates beside the content). */
  aside(aside = true): this {
    this._aside = aside
    return this
  }
}

/** Columns without a card. */
export class InfoGrid<T = ResourceRecord> extends InfoLayout<T> {
  readonly kind = 'grid'

  constructor(columns = 2) {
    super()
    this._columns = columns
  }
}

/** Every entry path, for working out which relations to include. */
export function entryPaths(nodes: InfoNode<any>[]): string[] {
  return nodes.flatMap((n) => {
    if (n instanceof InfoLayout) return entryPaths(n._children)
    if (n instanceof RepeatableEntry) return [n.name]
    const own = n instanceof Column ? [n.name, typeof n._description === 'string' ? n._description : ''] : [n.name]
    return own.filter(Boolean)
  })
}

export function infolistBuilder<T>() {
  return {
    text: (name: ColumnPath<T>) => new TextColumn<T>(name),
    badge: (name: ColumnPath<T>) => new BadgeColumn<T>(name),
    boolean: (name: ColumnPath<T>) => new BooleanColumn<T>(name),
    date: (name: ColumnPath<T>) => new DateColumn<T>(name),
    image: (name: ColumnPath<T>) => new ImageColumn<T>(name),
    html: (name: ColumnPath<T>) => new HtmlEntry<T>(name),
    markdown: (name: ColumnPath<T>) => new MarkdownEntry<T>(name),
    list: (name: ColumnPath<T>) => new ListEntry<T>(name),
    keyValue: (name: ColumnPath<T>) => new KeyValueEntry<T>(name),
    repeatable: <K extends Extract<keyof T, string>>(name: K) => new RepeatableEntry<T, ItemOf<T[K]>>(name),
    section: (heading: string) => new InfoSection<T>(heading),
    grid: (columns = 2) => new InfoGrid<T>(columns),
  }
}

export type InfolistBuilder<T> = ReturnType<typeof infolistBuilder<T>>
