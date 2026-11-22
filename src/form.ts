import type { ReactNode } from 'react'
import type { ResourceRecord } from './protocol'
import { t } from './i18n'

export type FieldName<T> = Extract<keyof T, string>

export type Operation = 'create' | 'edit'

export interface FieldContext<T> {
  /** Current value of another field in the form. */
  get: <K extends FieldName<T>>(name: K) => T[K] | undefined
  /** Whether the form creates a new record or edits one. */
  operation: Operation
  /** The stored record when editing (as last loaded or saved). */
  record?: T
}

export interface UpdateContext<T> extends FieldContext<T> {
  /** The field's new value. */
  value: unknown
  /** The field's value before this change. */
  old: unknown
  /** Sets another field. Its own afterStateUpdated does not run. */
  set: <K extends FieldName<T>>(name: K, value: unknown) => void
}

type Condition<T> = boolean | ((ctx: FieldContext<T>) => boolean)

export type ColumnSpan = 1 | 2 | 3 | 'full'
export type Columns = 1 | 2 | 3

function check<T>(c: Condition<T> | undefined, ctx: FieldContext<T>, fallback: boolean): boolean {
  if (c === undefined) return fallback
  return typeof c === 'function' ? c(ctx) : c
}

export abstract class Field<T = ResourceRecord> {
  abstract readonly kind: string
  readonly name: FieldName<T>
  _label?: string
  _required?: Condition<T>
  _disabled?: Condition<T>
  _dehydrated?: Condition<T>
  _helperText?: string
  _default?: unknown
  _visible?: (ctx: FieldContext<T>) => boolean
  _afterStateUpdated?: (ctx: UpdateContext<T>) => void
  _columnSpan: ColumnSpan = 1

  constructor(name: FieldName<T>) {
    this.name = name
  }

  label(label: string): this {
    this._label = label
    return this
  }

  /**
   * Marks the field as required: shown with an asterisk and checked before submitting.
   * Can depend on other fields. The server rules stay the source of truth.
   */
  required(required: Condition<T> = true): this {
    this._required = required
    return this
  }

  /** Disabled fields cannot be changed and are not sent, unless dehydrated(true). */
  disabled(disabled: Condition<T> = true): this {
    this._disabled = disabled
    return this
  }

  disabledOn(operation: Operation): this {
    return this.disabled((ctx) => ctx.operation === operation)
  }

  /** Whether the value is sent to the server. Default: when not disabled. */
  dehydrated(dehydrated: Condition<T> = true): this {
    this._dehydrated = dehydrated
    return this
  }

  /**
   * Runs in the browser when the user changes this field. Use set() to update other
   * fields, e.g. fill the slug from the title.
   */
  afterStateUpdated(fn: (ctx: UpdateContext<T>) => void): this {
    this._afterStateUpdated = fn
    return this
  }

  isRequired(ctx: FieldContext<T>): boolean {
    return check(this._required, ctx, false)
  }

  isDisabled(ctx: FieldContext<T>): boolean {
    return check(this._disabled, ctx, false)
  }

  isDehydrated(ctx: FieldContext<T>): boolean {
    return check(this._dehydrated, ctx, !this.isDisabled(ctx))
  }

  helperText(text: string): this {
    this._helperText = text
    return this
  }

  default(value: unknown): this {
    this._default = value
    return this
  }

  /** Evaluated in the browser on every change — no server round trip. Hidden fields are not submitted. */
  visible(fn: (ctx: FieldContext<T>) => boolean): this {
    this._visible = fn
    return this
  }

  /** Width in the surrounding grid: a number of columns, or "full". */
  columnSpan(span: ColumnSpan): this {
    this._columnSpan = span
    return this
  }

  /** Value used when the form starts empty and for comparisons; booleans start false. */
  get emptyValue(): unknown {
    return ''
  }

  hidden(fn: (ctx: FieldContext<T>) => boolean): this {
    this._visible = (ctx) => !fn(ctx)
    return this
  }

  visibleOn(operation: Operation): this {
    return this.visible((ctx) => ctx.operation === operation)
  }

  hiddenOn(operation: Operation): this {
    return this.hidden((ctx) => ctx.operation === operation)
  }

  isVisible(ctx: FieldContext<T>): boolean {
    return this._visible ? this._visible(ctx) : true
  }
}

export class TextInput<T = ResourceRecord> extends Field<T> {
  readonly kind = 'text-input'
  _type: 'text' | 'email' | 'url' | 'number' | 'password' | 'tel' = 'text'
  _dir?: 'ltr' | 'rtl'
  _maxLength?: number
  _placeholder?: string

  email(): this {
    this._type = 'email'
    return this
  }

  numeric(): this {
    this._type = 'number'
    return this
  }

  url(): this {
    this._type = 'url'
    this._dir = 'ltr'
    return this
  }

  password(): this {
    this._type = 'password'
    return this
  }

  tel(): this {
    this._type = 'tel'
    this._dir = 'ltr'
    return this
  }

  /** Text direction of the input, e.g. "ltr" for slugs and codes in an RTL panel. */
  dir(dir: 'ltr' | 'rtl'): this {
    this._dir = dir
    return this
  }

  maxLength(n: number): this {
    this._maxLength = n
    return this
  }

  placeholder(text: string): this {
    this._placeholder = text
    return this
  }
}

export class Textarea<T = ResourceRecord> extends Field<T> {
  readonly kind = 'textarea'
  _rows = 4

  rows(n: number): this {
    this._rows = n
    return this
  }
}

export class Select<T = ResourceRecord> extends Field<T> {
  readonly kind = 'select'
  _options: Record<string, string> = {}
  /** A relation name of this resource, e.g. "category" or "tags". */
  _relationship?: string
  /** A resource whose own records are the options (see optionsFrom). */
  _optionsFrom?: string
  _searchable = false
  _multiple = false

  options(options: Record<string, string>): this {
    this._options = options
    return this
  }

  /**
   * Options come from a relation of this resource, through the server (which uses the
   * related resource's TitleColumn and whitelist). Values are the related records' keys.
   */
  relationship(relation: string): this {
    this._relationship = relation
    return this
  }

  /**
   * Options are the records of another resource (its key and TitleColumn), through the
   * server and within that resource's permissions, scope and tenant. For values that point
   * at a resource without being a relation of this one, e.g. a page id inside a JSON item.
   */
  optionsFrom(resource: string): this {
    this._optionsFrom = resource
    return this
  }

  /** A search box that asks the server, for long lists. */
  searchable(searchable = true): this {
    this._searchable = searchable
    return this
  }

  /** Several values (an array), e.g. a many-to-many relation like tags. */
  multiple(multiple = true): this {
    this._multiple = multiple
    return this
  }

  _createIn?: { resource: string; field: string }

  /**
   * Offers to create a missing option from the search text: a record of resource with
   * field set to it (the server checks Create and validates as usual), e.g.
   * `.relationship('tags').createOption('cms_tags', 'name')`. Searchable selects only.
   */
  createOption(resource: string, field = 'name'): this {
    this._createIn = { resource, field }
    this._searchable = true
    return this
  }

  override get emptyValue(): unknown {
    return this._multiple ? [] : ''
  }
}

export class Hidden<T = ResourceRecord> extends Field<T> {
  readonly kind = 'hidden'
}

/**
 * A Jalali calendar. The value is ISO 8601: "2026-11-19T09:30:00.000Z" with time (in UTC,
 * picked in the browser's time zone), or "2026-11-19" for a date only.
 */
export class DatePicker<T = ResourceRecord> extends Field<T> {
  readonly kind = 'date-picker'
  _withTime = false

  withTime(withTime = true): this {
    this._withTime = withTime
    return this
  }
}

/**
 * Formatted text (HTML). List the field in the Go resource's HTML option so the server
 * sanitizes it; never trust HTML from the browser.
 */
export class RichEditor<T = ResourceRecord> extends Field<T> {
  readonly kind = 'rich-editor'
  _placeholder = t('editor.placeholder')
  _height = '300px'

  placeholder(text: string): this {
    this._placeholder = text
    return this
  }

  /** The editing area's starting height (it can be dragged taller), e.g. "400px". */
  height(css: string): this {
    this._height = css
    return this
  }
}

/** A list of short strings (JSON array), typed as chips. Enter or comma adds one. */
export class TagsInput<T = ResourceRecord> extends Field<T> {
  readonly kind = 'tags-input'
  _placeholder = t('tags.placeholder')

  placeholder(text: string): this {
    this._placeholder = text
    return this
  }

  override get emptyValue(): unknown {
    return []
  }
}

/** Pairs of strings (a JSON object), edited as rows. */
export class KeyValue<T = ResourceRecord> extends Field<T> {
  readonly kind = 'key-value'
  _keyLabel = t('common.key')
  _valueLabel = t('common.value')
  _addLabel = t('keyValue.add')

  labels(key: string, value: string, add?: string): this {
    this._keyLabel = key
    this._valueLabel = value
    if (add) this._addLabel = add
    return this
  }

  override get emptyValue(): unknown {
    return {}
  }
}

/** A list of items with the same fields (a JSON array of objects). */
export class Repeater<T = ResourceRecord> extends Field<T> {
  readonly kind = 'repeater'
  _schema: SchemaNode<any>[] = []
  _fields: Field<any>[] = []
  _columns: Columns = 2
  _minItems = 0
  _maxItems?: number
  _addLabel = t('repeater.add')
  _itemLabel?: (item: ResourceRecord, index: number) => string
  _reorderable = true
  _tree?: { children: string; maxDepth: number }

  /**
   * A tree: each item holds a repeater of items under `children` (its schema must contain
   * one, with tree() too). Items get buttons to move into the item before them and back
   * out, up to maxDepth levels (e.g. a menu with submenus).
   */
  tree(children = 'children', maxDepth = 6): this {
    this._tree = { children, maxDepth }
    return this
  }

  /** The fields of one item. Names are keys inside the item. */
  schema(fn: (f: FormBuilder<any>) => SchemaNode<any>[]): this {
    this._schema = fn(formBuilder<any>())
    this._fields = flattenFields(this._schema)
    return this
  }

  columns(columns: Columns): this {
    this._columns = columns
    return this
  }

  minItems(n: number): this {
    this._minItems = n
    return this
  }

  maxItems(n: number): this {
    this._maxItems = n
    return this
  }

  addLabel(text: string): this {
    this._addLabel = text
    return this
  }

  /** Heading of each item, e.g. its title field. */
  itemLabel(fn: (item: ResourceRecord, index: number) => string): this {
    this._itemLabel = fn
    return this
  }

  reorderable(reorderable = true): this {
    this._reorderable = reorderable
    return this
  }

  override get emptyValue(): unknown {
    return []
  }
}

/** One kind of block in a Builder. */
export class Block {
  readonly name: string
  _label?: string
  _schema: SchemaNode<any>[] = []
  _fields: Field<any>[] = []
  _columns: Columns = 2

  constructor(name: string) {
    this.name = name
  }

  label(label: string): this {
    this._label = label
    return this
  }

  schema(fn: (f: FormBuilder<any>) => SchemaNode<any>[]): this {
    this._schema = fn(formBuilder<any>())
    this._fields = flattenFields(this._schema)
    return this
  }

  columns(columns: Columns): this {
    this._columns = columns
    return this
  }
}

/**
 * A list of blocks of different kinds, e.g. headings, quotes and callouts.
 * The value is a JSON array of { type, data }.
 */
export class Builder<T = ResourceRecord> extends Field<T> {
  readonly kind = 'builder'
  _blocks: Block[] = []
  _maxItems?: number
  _addLabel = t('builder.add')

  blocks(blocks: Block[]): this {
    this._blocks = blocks
    return this
  }

  maxItems(n: number): this {
    this._maxItems = n
    return this
  }

  addLabel(text: string): this {
    this._addLabel = text
    return this
  }

  _history = false

  /** A toolbar with undo and redo, and copying or pasting the blocks as JSON. */
  history(on = true): this {
    this._history = on
    return this
  }

  override get emptyValue(): unknown {
    return []
  }
}

/** Markdown text, with a toolbar and a preview. Render it on your site with a sanitizer. */
export class MarkdownEditor<T = ResourceRecord> extends Field<T> {
  readonly kind = 'markdown-editor'
  _rows = 10

  rows(n: number): this {
    this._rows = n
    return this
  }
}

/** URL of a stored file path: paths saved by uploads live under /storage (the public disk). */
export function defaultFileUrl(path: string): string {
  return /^(\/|https?:|blob:|data:)/.test(path) ? path : `/storage/${path}`
}

/**
 * A file upload. The file goes to the server as soon as it is picked; the form sends the
 * returned token, and the server moves the file into place when the form is saved.
 * The server's Upload rule (in Go) decides what is accepted.
 */
export class FileUpload<T = ResourceRecord> extends Field<T> {
  readonly kind = 'file-upload'
  _image = false
  _accept?: string
  _maxSize?: number
  _url: (path: string) => string = defaultFileUrl

  /** Shows a preview and limits the picker to images. */
  image(): this {
    this._image = true
    this._accept ??= 'image/jpeg,image/png,image/webp,image/gif'
    return this
  }

  /** The file picker's accept attribute, e.g. "application/pdf". */
  accept(accept: string): this {
    this._accept = accept
    return this
  }

  /** Checked in the browser before uploading; keep it equal to the server's MaxSize. */
  maxSize(bytes: number): this {
    this._maxSize = bytes
    return this
  }

  /** How to turn the stored path into a URL for the preview. */
  url(fn: (path: string) => string): this {
    this._url = fn
    return this
  }

  _multiple = false
  _maxFiles?: number

  /** A list of files (Multiple in the Go Upload rule); maxFiles should match MaxFiles. */
  multiple(maxFiles?: number): this {
    this._multiple = true
    this._maxFiles = maxFiles
    return this
  }

  override get emptyValue(): unknown {
    return this._multiple ? [] : ''
  }
}

/** Shows a computed value in the form. It has no input and is never sent. */
export class Placeholder<T = ResourceRecord> extends Field<T> {
  readonly kind = 'placeholder'
  _content: (ctx: FieldContext<T>) => ReactNode = () => ''

  constructor(name: string) {
    super(name as FieldName<T>)
    this._dehydrated = false
  }

  /** What it shows, from the form's values: text, or any React node (e.g. a live report). */
  content(fn: (ctx: FieldContext<T>) => ReactNode): this {
    this._content = fn
    return this
  }
}

/** An on/off switch for boolean fields. */
export class Toggle<T = ResourceRecord> extends Field<T> {
  readonly kind = 'toggle'

  override get emptyValue(): unknown {
    return false
  }
}

/** A checkbox for boolean fields. */
export class Checkbox<T = ResourceRecord> extends Field<T> {
  readonly kind = 'checkbox'

  override get emptyValue(): unknown {
    return false
  }
}

/** Radio buttons: one choice from a few options. */
export class Radio<T = ResourceRecord> extends Field<T> {
  readonly kind = 'radio'
  _options: Record<string, string> = {}
  _inline = false

  options(options: Record<string, string>): this {
    this._options = options
    return this
  }

  /** Lays the options out in a row. */
  inline(inline = true): this {
    this._inline = inline
    return this
  }
}

// --- layout ----------------------------------------------------------------------------

/** A form schema is a tree of fields and layout components. */
export type SchemaNode<T = ResourceRecord> = Field<T> | Layout<T>

/** Groups fields visually. Layouts hold no data: only the fields inside them do. */
export abstract class Layout<T = ResourceRecord> {
  abstract readonly kind: 'section' | 'grid' | 'fieldset' | 'tabs' | 'tab' | 'wizard' | 'step'
  _children: SchemaNode<T>[] = []
  _columns: Columns = 2
  _columnSpan: ColumnSpan = 'full'
  _visible?: (ctx: FieldContext<T>) => boolean

  schema(children: SchemaNode<T>[]): this {
    this._children = children
    return this
  }

  /** Columns of the grid inside this layout (on wide screens). */
  columns(columns: Columns): this {
    this._columns = columns
    return this
  }

  columnSpan(span: ColumnSpan): this {
    this._columnSpan = span
    return this
  }

  /** Hides the layout and every field in it; hidden fields are not sent. */
  visible(fn: (ctx: FieldContext<T>) => boolean): this {
    this._visible = fn
    return this
  }

  hidden(fn: (ctx: FieldContext<T>) => boolean): this {
    this._visible = (ctx) => !fn(ctx)
    return this
  }

  isVisible(ctx: FieldContext<T>): boolean {
    return this._visible ? this._visible(ctx) : true
  }
}

export class Section<T = ResourceRecord> extends Layout<T> {
  readonly kind = 'section'
  readonly heading: string
  _description?: string
  _collapsible = false
  _collapsed = false

  constructor(heading: string) {
    super()
    this.heading = heading
  }

  description(text: string): this {
    this._description = text
    return this
  }

  /** Lets the user fold the section; collapsed(true) starts folded. */
  collapsible(collapsed = false): this {
    this._collapsible = true
    this._collapsed = collapsed
    return this
  }
}

export class Grid<T = ResourceRecord> extends Layout<T> {
  readonly kind = 'grid'
}

export class Fieldset<T = ResourceRecord> extends Layout<T> {
  readonly kind = 'fieldset'
  readonly legend: string

  constructor(legend: string) {
    super()
    this.legend = legend
  }
}

export class Tab<T = ResourceRecord> extends Layout<T> {
  readonly kind = 'tab'
  readonly label: string

  constructor(label: string) {
    super()
    this.label = label
  }
}

/** A form split into steps; the next step opens once the current one is filled in. */
export class Wizard<T = ResourceRecord> extends Layout<T> {
  readonly kind = 'wizard'
  _submitLabel = t('common.save')

  steps(steps: Step<T>[]): this {
    this._children = steps
    return this
  }

  submitLabel(text: string): this {
    this._submitLabel = text
    return this
  }
}

export class Step<T = ResourceRecord> extends Layout<T> {
  readonly kind = 'step'
  readonly label: string
  _description?: string

  constructor(label: string) {
    super()
    this.label = label
  }

  description(text: string): this {
    this._description = text
    return this
  }
}

export class Tabs<T = ResourceRecord> extends Layout<T> {
  readonly kind = 'tabs'

  tabs(tabs: Tab<T>[]): this {
    this._children = tabs
    return this
  }
}

export function isLayout<T>(node: SchemaNode<T>): node is Layout<T> {
  return node instanceof Layout
}

/** Every field in the schema, in order. */
export function flattenFields<T>(nodes: SchemaNode<T>[]): Field<T>[] {
  return nodes.flatMap((n) => (isLayout(n) ? flattenFields(n._children) : [n]))
}

/** Fields that are visible, together with every layout around them. */
export function visibleFields<T>(nodes: SchemaNode<T>[], ctx: FieldContext<T>): Field<T>[] {
  return nodes.flatMap((n) => {
    if (!n.isVisible(ctx)) return []
    return isLayout(n) ? visibleFields(n._children, ctx) : [n]
  })
}

export function formBuilder<T>() {
  return {
    textInput: (name: FieldName<T>) => new TextInput<T>(name),
    textarea: (name: FieldName<T>) => new Textarea<T>(name),
    select: (name: FieldName<T>) => new Select<T>(name),
    hidden: (name: FieldName<T>) => new Hidden<T>(name),
    toggle: (name: FieldName<T>) => new Toggle<T>(name),
    checkbox: (name: FieldName<T>) => new Checkbox<T>(name),
    radio: (name: FieldName<T>) => new Radio<T>(name),
    /** A display-only field; the name only needs to be unique in the form. */
    placeholder: (name: string) => new Placeholder<T>(name),
    datePicker: (name: FieldName<T>) => new DatePicker<T>(name),
    fileUpload: (name: FieldName<T>) => new FileUpload<T>(name),
    richEditor: (name: FieldName<T>) => new RichEditor<T>(name),
    markdownEditor: (name: FieldName<T>) => new MarkdownEditor<T>(name),
    tagsInput: (name: FieldName<T>) => new TagsInput<T>(name),
    keyValue: (name: FieldName<T>) => new KeyValue<T>(name),
    repeater: (name: FieldName<T>) => new Repeater<T>(name),
    builder: (name: FieldName<T>) => new Builder<T>(name),
    block: (name: string) => new Block(name),
    wizard: (steps: Step<T>[]) => new Wizard<T>().steps(steps),
    step: (label: string) => new Step<T>(label),
    dateTimePicker: (name: FieldName<T>) => new DatePicker<T>(name).withTime(),
    section: (heading: string) => new Section<T>(heading),
    grid: (columns: Columns = 2) => new Grid<T>().columns(columns),
    fieldset: (legend: string) => new Fieldset<T>(legend),
    tabs: (tabs: Tab<T>[]) => new Tabs<T>().tabs(tabs),
    tab: (label: string) => new Tab<T>(label),
  }
}

export type FormBuilder<T> = ReturnType<typeof formBuilder<T>>
