import type { ResourceRecord } from './protocol'
import { tableBuilder, type Column, type TableBuilder } from './table'

/** Fields of T holding lists of records: the relations a manager can show. */
export type ManyRelation<T> = Extract<{ [K in keyof T]-?: NonNullable<T[K]> extends readonly unknown[] ? K : never }[keyof T], string>

/**
 * A table of related records on the edit page, like Filament's relation managers: the
 * posts of a category (has-many) or the posts of a tag (many-to-many). The server
 * decides what is allowed; editing a related record opens its own edit page.
 */
export class RelationManager<R = ResourceRecord> {
  readonly relation: string
  _resource: string
  _label?: string
  _table?: (t: TableBuilder<R>) => Column<R>[]
  _perPage = 5
  private _columns?: Column<R>[]

  constructor(relation: string) {
    this.relation = relation
    this._resource = relation
  }

  /** The frontend resource of the related records (its table and form). Default: the relation's name. */
  resource(name: string): this {
    this._resource = name
    return this
  }

  label(label: string): this {
    this._label = label
    return this
  }

  /** Columns for this table; default: the related resource's table. */
  table(fn: (t: TableBuilder<R>) => Column<R>[]): this {
    this._table = fn
    this._columns = undefined
    return this
  }

  perPage(n: number): this {
    this._perPage = n
    return this
  }

  /** Own columns, if set. */
  get columns(): Column<R>[] | undefined {
    if (this._table && !this._columns) this._columns = this._table(tableBuilder<R>())
    return this._columns
  }
}

export function relationBuilder<T>() {
  return {
    manager: <R = ResourceRecord>(relation: ManyRelation<T>) => new RelationManager<R>(relation),
  }
}

export type RelationBuilder<T> = ReturnType<typeof relationBuilder<T>>
