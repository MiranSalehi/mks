import type { ReactNode } from 'react'
import { actionBuilder, type Action, type ActionBuilder } from './actions'
import { flattenFields, formBuilder, type Field, type FormBuilder, type SchemaNode } from './form'
import { infolistBuilder, type InfolistBuilder, type InfoNode } from './infolist'
import { groupBuilder, type Group, type GroupBuilder } from './groups'
import { filterBuilder, type Filter, type FilterBuilder } from './filters'
import { relationBuilder, type RelationBuilder, type RelationManager } from './relations'
import { tableBuilder, type Column, type TableBuilder } from './table'

export interface ResourceOptions<T> {
  /** Matches the backend resource name, e.g. "posts". */
  name: string
  label: string
  pluralLabel: string
  /** Shown in the navigation, e.g. <DocumentIcon /> (exported by the panel). */
  icon?: ReactNode
  /** A navigation group heading, e.g. "محتوا". */
  group?: string
  /** Order in its group: lower first. Default: after the items that set one, in definition order. */
  navigationSort?: number
  /** false: not in the sidebar (still reachable by its URL and links). */
  navigation?: boolean
  table: (t: TableBuilder<T>) => Column<T>[]
  form?: (f: FormBuilder<T>) => SchemaNode<T>[]
  /** A read-only view of one record; with it, the resource gets a view page. */
  infolist?: (i: InfolistBuilder<T>) => InfoNode<T>[]
  actions?: (a: ActionBuilder<T>) => Action<T>[]
  /** Filters above the table. The server must allow each (Filterable or a custom filter). */
  filters?: (f: FilterBuilder<T>) => Filter<T>[]
  /** Ways to group the table's rows; the user picks one (or none). */
  groups?: (g: GroupBuilder<T>) => Group<T>[]
  /**
   * Tabs above the table, each a value of one of the filters (no value: all records), e.g.
   * `{ filter: 'status', tabs: [{ label: 'All' }, { label: 'Pending', value: 'pending' }] }`.
   */
  tabs?: ListTabs
  /** Tables of related records under the edit form. */
  relations?: (r: RelationBuilder<T>) => RelationManager<any>[]
  defaultSort?: string
  perPage?: number
  /** How a record is named in titles and breadcrumbs: a field name or a function. */
  recordTitle?: Extract<keyof T, string> | ((record: T) => string)
}

export interface ListTabs {
  /** The name of a filter of the resource; a tab sets its value. */
  filter: string
  tabs: { label: string; value?: string }[]
}

export interface ResourceDefinition<T = any> {
  name: string
  label: string
  pluralLabel: string
  icon?: ReactNode
  group?: string
  navigationSort?: number
  navigation?: boolean
  columns: Column<T>[]
  /** The form as written: fields and layouts. */
  schema: SchemaNode<T>[]
  /** Every field of the form, flattened. */
  fields: Field<T>[]
  infolist: InfoNode<T>[]
  actions: Action<T>[]
  filters: Filter<T>[]
  relations: RelationManager<any>[]
  groups: Group<T>[]
  tabs?: ListTabs
  defaultSort?: string
  perPage: number
  recordTitle: (record: T) => string
}

export function defineResource<T>(options: ResourceOptions<T>): ResourceDefinition<T> {
  const schema = options.form ? options.form(formBuilder<T>()) : []
  return {
    name: options.name,
    label: options.label,
    pluralLabel: options.pluralLabel,
    icon: options.icon,
    group: options.group,
    navigationSort: options.navigationSort,
    navigation: options.navigation,
    columns: options.table(tableBuilder<T>()),
    schema,
    fields: flattenFields(schema),
    infolist: options.infolist ? options.infolist(infolistBuilder<T>()) : [],
    actions: options.actions ? options.actions(actionBuilder<T>()) : [],
    filters: options.filters ? options.filters(filterBuilder<T>()) : [],
    relations: options.relations ? options.relations(relationBuilder<T>()) : [],
    groups: options.groups ? options.groups(groupBuilder<T>()) : [],
    tabs: options.tabs,
    defaultSort: options.defaultSort,
    perPage: options.perPage ?? 10,
    recordTitle: (record: T) => {
      const rt = options.recordTitle
      if (typeof rt === 'function') return rt(record)
      const v = rt ? (record as Record<string, unknown>)[rt] : (record as Record<string, unknown>).id
      return v == null ? '' : String(v)
    },
  }
}

export function humanize(name: string): string {
  const s = name.replace(/[._]/g, ' ')
  return s.charAt(0).toUpperCase() + s.slice(1)
}
