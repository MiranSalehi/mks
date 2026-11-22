// Types for the API contract in docs/PROTOCOL.md. Every backend adapter speaks this.

export type RecordKey = string | number

export type ResourceRecord = Record<string, unknown>

export interface PageMeta {
  page: number
  per_page: number
  total: number
  last_page: number
}

/** What the user may do with one record. Hides buttons; the server checks again. */
export interface RecordAbilities {
  update: boolean
  delete: boolean
  actions: Record<string, boolean>
}

export interface ListResponse<T = ResourceRecord> {
  data: T[]
  meta: PageMeta
  /** By record key. */
  abilities?: Record<string, RecordAbilities>
  /** The requested aggregates: column -> function -> value (null without records). */
  summary?: Record<string, Partial<Record<'sum' | 'avg' | 'min' | 'max', number | null>>>
}

/** What a relation manager may do, from GET .../relations/{relation}/records. */
export interface RelationMeta {
  type: 'has_many' | 'many_to_many'
  /** has-many: the column the server fills with the parent's key on create. */
  foreign_key?: string
  create: boolean
  attach: boolean
  detach: boolean
}

export interface RelatedListResponse<T = ResourceRecord> extends ListResponse<T> {
  relation: RelationMeta
}

export interface RecordResponse<T = ResourceRecord> {
  data: T
  abilities?: RecordAbilities
}

export type FilterOperator = 'eq' | 'in' | 'gt' | 'gte' | 'lt' | 'lte' | 'null'

export interface ListFilter {
  /** A filterable column, or a custom filter's name. */
  column: string
  op: FilterOperator
  value: string | string[]
}

export interface ListParams {
  page?: number
  perPage?: number
  /** Column names; prefix with "-" for descending. */
  sort?: string[]
  search?: string
  filters?: ListFilter[]
  include?: string[]
  /** Aggregates over all filtered records, e.g. "views.sum". */
  summarize?: string[]
}

/** Field errors keyed by field path, e.g. "title" or "tags.0.name". */
export type FieldErrors = Record<string, string[]>

export interface ErrorBody {
  message: string
  errors?: FieldErrors
}

/** The protocol version this UI speaks. /meta reports the server's. */
export const PROTOCOL_VERSION = 0

export interface UserInfo {
  id: RecordKey
  name: string
  email: string
}

export type FieldType = 'string' | 'integer' | 'number' | 'boolean' | 'datetime'

export interface ResourceMeta {
  key: string
  fields: Record<string, { type: FieldType; nullable: boolean }>
  relations: string[]
  capabilities: {
    searchable: string[]
    sortable: string[]
    filterable: Record<string, FilterOperator[]>
    /** Custom filters defined on the server. */
    filters: string[]
    summaries: Record<string, ('sum' | 'avg' | 'min' | 'max')[]>
    /** The manual order column ("" when the resource has none). */
    reorderable: string
    fillable: string[]
  }
  deletable: boolean
  actions: { name: string; without_records: boolean; fields: string[] }[]
  abilities: { create: boolean; delete: boolean; reorder?: boolean; header_actions: Record<string, boolean> }
  /** Sidebar count (Config.NavigationBadge); absent when the resource has none. Zero shows nothing. */
  badge?: { count: number; color?: 'primary' | 'success' | 'warning' | 'danger' | 'gray' | 'info' }
}

export interface MetaResponse {
  protocol: number
  panel: {
    path: string
    brand: string
    locale: string
    direction: 'rtl' | 'ltr'
    /** Database notifications, if on: how often (seconds) to look for new ones. */
    notifications: { poll: number } | null
  }
  user: UserInfo
  resources: Record<string, ResourceMeta>
  order: string[]
  /** Dashboard widgets (panel.Widget) the user may see. */
  widgets: string[]
  /** Custom pages (panel.Page) the user may open. */
  pages: string[]
  /** Sidebar counts of pages (panel.Page NavigationBadge), by page name. */
  page_badges?: Record<string, { count: number; color?: 'primary' | 'success' | 'warning' | 'danger' | 'gray' | 'info' }>
  /** Whether the global search has anything to search for this user. */
  search: boolean
  /** Values plugins add on the server (Panel.MetaValue), by key. */
  extra?: Record<string, unknown>
  /** Multi-tenancy: the user's tenants and the one this request worked in; null when off. */
  tenancy: { tenants: { id: RecordKey; name: string }[]; current: RecordKey | null } | null
}

export interface SearchResult {
  resource: string
  key: RecordKey
  title: string
  /** Extra columns (SearchDetails). */
  details?: Record<string, unknown>
}

/** A database notification of the logged-in user. */
export interface PanelNotification {
  id: number
  type: NotificationType
  title: string
  body: string | null
  /** A path inside the panel. */
  url: string | null
  read_at: string | null
  created_at: string | null
}

export interface NotificationsResponse {
  data: PanelNotification[]
  meta: { page: number; has_more: boolean; unread: number }
}

export interface RelationOption {
  value: RecordKey
  label: string
}

export interface OptionsResponse {
  data: RelationOption[]
  meta: { page: number; has_more: boolean }
}

export interface UploadResponse {
  token: string
  name: string
  size: number
  mime: string
}

export type NotificationType = 'success' | 'warning' | 'danger' | 'info'

/** Response of an action (and of a bulk delete). */
export interface ActionResponse {
  notification?: { type: NotificationType; title: string }
  /** A path inside the panel to open, e.g. "/posts". */
  redirect?: string
  /** A public URL returned by a server action, e.g. a short-lived preview. */
  open_url?: string
  /** Resources whose data should be reloaded. */
  invalidate: string[]
  /** A result for the page's UI to show (panel.ActionResult.Data). */
  data?: unknown
}

export interface LoginResponse {
  user: UserInfo
  csrf_token: string
}
