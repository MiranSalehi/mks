import { flattenFields, formBuilder, type Field, type FormBuilder, type SchemaNode } from './form'
import type { ResourceRecord } from './protocol'
import type { Color } from './table'

/** Where an action button appears. */
export type ActionPlacement = 'row' | 'bulk' | 'header'

export interface Confirmation {
  heading?: string
  description?: string
  submitLabel?: string
}

export abstract class Action<T = ResourceRecord> {
  abstract readonly kind: 'server' | 'delete' | 'link'
  readonly name: string
  _label?: string
  _color: Color = 'primary'
  _placements = new Set<ActionPlacement>()
  _confirmation?: Confirmation
  _visible?: (record: T) => boolean
  _schema: SchemaNode<any>[] = []
  _fields: Field<any>[] = []
  _modalHeading?: string
  _modalDescription?: string
  _modalSubmitLabel?: string

  constructor(name: string) {
    this.name = name
  }

  label(label: string): this {
    this._label = label
    return this
  }

  color(color: Color): this {
    this._color = color
    return this
  }

  /** Asks before running, in a modal. */
  requiresConfirmation(confirmation: Confirmation = {}): this {
    this._confirmation = confirmation
    return this
  }

  /** For row actions: whether the button shows for a record. */
  visible(fn: (record: T) => boolean): this {
    this._visible = fn
    return this
  }

  modalHeading(text: string): this {
    this._modalHeading = text
    return this
  }

  modalDescription(text: string): this {
    this._modalDescription = text
    return this
  }

  modalSubmitLabel(text: string): this {
    this._modalSubmitLabel = text
    return this
  }

  /** Shows a button on each table row. */
  row(): this {
    this._placements.add('row')
    return this
  }

  /** Shows a button when rows are selected; runs on all of them. */
  bulk(): this {
    this._placements.add('bulk')
    return this
  }

  isVisibleFor(record: T): boolean {
    return this._visible ? this._visible(record) : true
  }

  /** Whether running it opens a modal first. */
  get needsModal(): boolean {
    return this._confirmation !== undefined || this._fields.length > 0
  }

  get placements(): ActionPlacement[] {
    return [...this._placements]
  }
}

/** Runs a server action (panel.Action in Go) with the same name. */
export class ServerAction<T = ResourceRecord> extends Action<T> {
  readonly kind = 'server'

  /** Shows a button above the table that runs on no records (WithoutRecords in Go). */
  header(): this {
    this._placements.add('header')
    return this
  }

  /** A form in the modal; its data is validated by the action's Rules on the server. */
  form(fn: (f: FormBuilder<any>) => SchemaNode<any>[]): this {
    this._schema = fn(formBuilder<any>())
    this._fields = flattenFields(this._schema)
    return this
  }
}

/** Deletes records. The server resource must be Deletable. */
export class DeleteAction<T = ResourceRecord> extends Action<T> {
  readonly kind = 'delete'

  constructor() {
    super('delete')
    this._color = 'danger'
    this._placements = new Set(['row', 'bulk'])
    this._confirmation = {}
  }
}

/** Opens a URL; runs nothing on the server. */
export class LinkAction<T = ResourceRecord> extends Action<T> {
  readonly kind = 'link'
  _url: (record: T) => string = () => '#'
  _newTab = false

  constructor(name: string) {
    super(name)
    this._placements.add('row')
    this._color = 'gray'
  }

  url(fn: (record: T) => string, openInNewTab = false): this {
    this._url = fn
    this._newTab = openInNewTab
    return this
  }

  /** A button above the table instead of on each row; the url gets no record. */
  header(): this {
    this._placements = new Set(['header'])
    return this
  }
}

export function actionBuilder<T>() {
  return {
    /** A server action; choose where it shows with row(), bulk() and header(). */
    make: (name: string) => new ServerAction<T>(name),
    delete: () => new DeleteAction<T>(),
    link: (name: string) => new LinkAction<T>(name),
  }
}

export type ActionBuilder<T> = ReturnType<typeof actionBuilder<T>>
