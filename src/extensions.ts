import type { ClusterDefinition, NavigationLinkDefinition, UserMenuItemDefinition } from './navigation'
import type { ComponentType, ReactNode } from 'react'
import { Field, type FieldName } from './form'
import type { PageDefinition } from './pages'
import type { MetaResponse } from './protocol'
import type { FieldErrors, ResourceRecord } from './protocol'
import type { ResourceDefinition } from './resource'
import { Column, type ColumnPath } from './table'
import type { Widget } from './widgets'

// --- custom fields ------------------------------------------------------------------------

/** What a custom field's component receives. Render the control only: the label, help and
 * errors around it are the panel's. */
export interface CustomFieldProps<V = unknown, O = Record<string, unknown>> {
  /** Put it on the focusable element: the label points at it. */
  id: string
  value: V
  onChange: (value: V) => void
  disabled?: boolean
  invalid: boolean
  options: O
  /** The record being edited, or the record of a single-record action's form. */
  record?: ResourceRecord
  /** Errors under this field ("payload.2.data.title"), for fields that hold lists. */
  errors?: FieldErrors
  /** Another field's current value. */
  get?: (name: string) => unknown
  /** aria-invalid, aria-describedby and aria-required, for the focusable element. */
  a11y: Record<string, unknown>
}

/**
 * A form field drawn by your own component. Usually made by defineField; subclass it for
 * typed option methods.
 */
export class CustomField<T = ResourceRecord, O extends object = Record<string, unknown>, V = unknown> extends Field<T> {
  readonly kind = 'custom'
  readonly component: ComponentType<CustomFieldProps<V, O>>
  _options: O
  _empty: V | null

  constructor(name: FieldName<T>, component: ComponentType<CustomFieldProps<V, O>>, options: O, empty: V | null = null) {
    super(name)
    this.component = component
    this._options = options
    this._empty = empty
  }

  /** Sets one of the field's options. */
  option<K extends keyof O>(key: K, value: O[K]): this {
    this._options = { ...this._options, [key]: value }
    return this
  }

  override get emptyValue(): unknown {
    return this._empty
  }
}

/**
 * Makes a field factory for use in any form:
 *
 *	export const colorPicker = defineField({ component: ColorControl, defaults: { swatches: [...] }, empty: null })
 *	form: (f) => [f.textInput('name'), colorPicker<Category>('color').label('رنگ')]
 */
export function defineField<V, O extends object>(spec: { component: ComponentType<CustomFieldProps<V, O>>; defaults: O; empty?: V | null }) {
  return <T = ResourceRecord>(name: FieldName<T>) => new CustomField<T, O, V>(name, spec.component, { ...spec.defaults }, spec.empty ?? null)
}

// --- custom columns -----------------------------------------------------------------------

/** What a custom column's component receives (in tables, infolists and widgets). */
export interface CustomCellProps<O = Record<string, unknown>> {
  value: unknown
  record: ResourceRecord
  options: O
}

/** A table column (and infolist entry) drawn by your own component. */
export class CustomColumn<T = ResourceRecord, O extends object = Record<string, unknown>> extends Column<T> {
  readonly kind = 'custom'
  readonly component: ComponentType<CustomCellProps<O>>
  _options: O

  constructor(name: ColumnPath<T>, component: ComponentType<CustomCellProps<O>>, options: O) {
    super(name)
    this.component = component
    this._options = options
  }

  option<K extends keyof O>(key: K, value: O[K]): this {
    this._options = { ...this._options, [key]: value }
    return this
  }
}

/** Makes a column factory: table: (t) => [t.text('name'), colorSwatch<Category>('color')]. */
export function defineColumn<O extends object>(spec: { component: ComponentType<CustomCellProps<O>>; defaults: O }) {
  return <T = ResourceRecord>(name: ColumnPath<T>) => new CustomColumn<T, O>(name, spec.component, { ...spec.defaults })
}

// --- plugins ------------------------------------------------------------------------------

/**
 * A bundle of panel features to install at once: <Panel plugins={[activityLog()]} />.
 * Its server half is a panel.Plugin in Go; names must match.
 */
export interface PanelPlugin {
  name: string
  resources?: ResourceDefinition[]
  pages?: PageDefinition[]
  /** Added to the end of the dashboard. */
  widgets?: Widget[]
  /** Extra buttons in the topbar (before the bell). */
  topbar?: ReactNode
  /** Extra sidebar links. */
  navigationLinks?: NavigationLinkDefinition[]
  /** Clusters its pages (or the app's) can join. */
  clusters?: ClusterDefinition[]
  /** Entries of the user menu. */
  userMenuItems?: UserMenuItemDefinition[]
  /**
   * Called with every meta response before the panel renders (e.g. to read what the
   * server installed: blocks of uploaded plugins). Keep it quick and idempotent.
   */
  onMeta?: (meta: MetaResponse) => void
  /**
   * Resources that exist only some of the time (the server adds them while it runs, e.g.
   * content types of uploaded plugins), built from what meta reports. Return the same
   * definition objects for the same input, so open forms keep their state.
   */
  metaResources?: (meta: MetaResponse) => ResourceDefinition[]
}

export function definePlugin(plugin: PanelPlugin): PanelPlugin {
  return plugin
}
