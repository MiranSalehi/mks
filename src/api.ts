import type {
  ErrorBody,
  FieldErrors,
  ListParams,
  ListResponse,
  ActionResponse,
  LoginResponse,
  MetaResponse,
  NotificationsResponse,
  OptionsResponse,
  UploadResponse,
  RecordKey,
  RecordResponse,
  RelatedListResponse,
  SearchResult,
  ResourceRecord,
  UserInfo,
} from './protocol'
import { baseLocale, t } from './i18n'

export class ApiError extends Error {
  readonly status: number
  readonly errors: FieldErrors

  constructor(status: number, body: ErrorBody) {
    super(body.message)
    this.status = status
    this.errors = body.errors ?? {}
  }

  get isValidation(): boolean {
    return this.status === 422
  }

  get isUnauthenticated(): boolean {
    return this.status === 401
  }

  get isForbidden(): boolean {
    return this.status === 403
  }
}

export function listQuery(params: ListParams): string {
  const q = new URLSearchParams()
  if (params.page) q.set('page', String(params.page))
  if (params.perPage) q.set('per_page', String(params.perPage))
  if (params.sort?.length) q.set('sort', params.sort.join(','))
  if (params.search) q.set('search', params.search)
  if (params.include?.length) q.set('include', params.include.join(','))
  if (params.summarize?.length) q.set('summarize', params.summarize.join(','))
  for (const f of params.filters ?? []) {
    const value = Array.isArray(f.value) ? f.value.join(',') : f.value
    q.set(f.op === 'eq' ? `filter[${f.column}]` : `filter[${f.column}][${f.op}]`, value)
  }
  const s = q.toString()
  return s ? `?${s}` : ''
}

/** Parameters of an options request: a search page, or the labels of given keys. */
export interface OptionsParams {
  search?: string
  page?: number
  values?: RecordKey[]
}

function optionsQuery(params: OptionsParams): string {
  const q = new URLSearchParams()
  if (params.search) q.set('search', params.search)
  if (params.page) q.set('page', String(params.page))
  if (params.values) q.set('values', params.values.join(','))
  const qs = q.toString()
  return qs ? `?${qs}` : ''
}

const CSRF_HEADER = 'X-CSRF-TOKEN'
const TENANT_HEADER = 'X-Mks-Tenant'
const TENANT_KEY = 'mks-tenant'

/**
 * The tab's tenant. A link may choose it with ?tenant= (e.g. "edit" on a tenant's public
 * site); the parameter is then taken out of the address. The server still checks it.
 */
function readTenant(): string | null {
  try {
    const url = new URL(window.location.href)
    const fromLink = url.searchParams.get('tenant')
    if (fromLink && /^[\w-]{1,64}$/.test(fromLink)) {
      sessionStorage.setItem(TENANT_KEY, fromLink)
      url.searchParams.delete('tenant')
      window.history.replaceState(window.history.state, '', url)
    }
    return sessionStorage.getItem(TENANT_KEY)
  } catch {
    return null
  }
}
const READING = new Set(['GET', 'HEAD', 'OPTIONS'])

export class ApiClient {
  readonly baseUrl: string
  /** Called when the server says the session is gone (401). */
  onUnauthenticated?: () => void
  private csrfToken: string | null = null
  /**
   * The tenant this tab works in (multi-tenancy), sent with every request. Kept per tab
   * (sessionStorage), so two tabs can work in two tenants.
   */
  tenant: string | null = readTenant()

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl.replace(/\/$/, '')
  }

  setTenant(id: string | null) {
    this.tenant = id
    try {
      if (id === null) sessionStorage.removeItem(TENANT_KEY)
      else sessionStorage.setItem(TENANT_KEY, id)
    } catch {
      // storage off: the choice lasts until reload
    }
  }

  /** The tenant, and the UI language so the server answers in it. */
  private extraHeaders(): Record<string, string> {
    // An added language is answered in its base (the server has fa and en).
    const h: Record<string, string> = { 'X-Panel-Locale': baseLocale() }
    if (this.tenant) h[TENANT_HEADER] = this.tenant
    return h
  }

  // --- auth ---

  async login(email: string, password: string): Promise<UserInfo> {
    const res = await this.request<LoginResponse>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    })
    // Login rotates the token; the response carries the new one.
    this.csrfToken = res.csrf_token
    return res.user
  }

  async logout(): Promise<void> {
    const res = await this.request<{ csrf_token: string }>('/auth/logout', { method: 'POST' })
    this.csrfToken = res.csrf_token
  }

  meta(): Promise<MetaResponse> {
    return this.request('/meta')
  }

  // --- notifications ---

  notifications(page = 1): Promise<NotificationsResponse> {
    return this.request(`/notifications?page=${page}`)
  }

  /** Marks one notification read, or all of them without an id. */
  readNotifications(id?: number): Promise<{ count: number }> {
    return this.request(id === undefined ? '/notifications/read' : `/notifications/${id}/read`, { method: 'POST' })
  }

  /** Deletes one notification, or all of them without an id. */
  deleteNotifications(id?: number): Promise<{ count: number }> {
    return this.request(id === undefined ? '/notifications' : `/notifications/${id}`, { method: 'DELETE' })
  }

  /** The data of a custom page (its panel.Page Data). */
  page<T = unknown>(name: string): Promise<{ data: T }> {
    return this.request(`/pages/${encodeURIComponent(name)}`)
  }

  /** Sends a custom page's form (its panel.Page Handle). */
  submitPage(name: string, data: ResourceRecord): Promise<ActionResponse> {
    return this.request(`/pages/${encodeURIComponent(name)}`, { method: 'POST', body: JSON.stringify({ data }) })
  }

  /** Panel-wide search across resources with GlobalSearch. */
  /** A plugin's own endpoint: {panel}/api/plugins/{plugin}{path}, with session and CSRF. */
  plugin<T>(plugin: string, path: string, init?: RequestInit): Promise<T> {
    return this.request(`/plugins/${encodeURIComponent(plugin)}${path}`, init)
  }

  search(q: string): Promise<{ data: SearchResult[] }> {
    return this.request(`/search?q=${encodeURIComponent(q)}`)
  }

  /** The data of a dashboard widget. */
  widget<T = unknown>(name: string): Promise<{ data: T }> {
    return this.request(`/widgets/${encodeURIComponent(name)}`)
  }

  // --- records ---

  list<T = ResourceRecord>(resource: string, params: ListParams = {}): Promise<ListResponse<T>> {
    return this.request(`/resources/${resource}/records${listQuery(params)}`)
  }

  get<T = ResourceRecord>(resource: string, key: RecordKey, include: string[] = []): Promise<RecordResponse<T>> {
    return this.request(`/resources/${resource}/records/${encodeURIComponent(String(key))}${listQuery({ include })}`)
  }

  create<T = ResourceRecord>(resource: string, data: ResourceRecord): Promise<RecordResponse<T>> {
    return this.request(`/resources/${resource}/records`, { method: 'POST', body: JSON.stringify({ data }) })
  }

  /** Partial update: only the given fields are validated and written. */
  update<T = ResourceRecord>(resource: string, key: RecordKey, data: ResourceRecord): Promise<RecordResponse<T>> {
    return this.request(`/resources/${resource}/records/${encodeURIComponent(String(key))}`, {
      method: 'PATCH',
      body: JSON.stringify({ data }),
    })
  }

  /** Options for a relation select: a search page, or the labels of given keys. */
  relationOptions(resource: string, relation: string, params: OptionsParams = {}): Promise<OptionsResponse> {
    return this.request(`/resources/${resource}/relations/${encodeURIComponent(relation)}/options${optionsQuery(params)}`)
  }

  /** Options from the records of a resource itself (Select.optionsFrom). */
  resourceOptions(resource: string, params: OptionsParams = {}): Promise<OptionsResponse> {
    return this.request(`/resources/${resource}/options${optionsQuery(params)}`)
  }

  /** Options of a relation of resource, or of resource itself when relation is undefined. */
  options(resource: string, relation: string | undefined, params: OptionsParams = {}): Promise<OptionsResponse> {
    return relation === undefined ? this.resourceOptions(resource, params) : this.relationOptions(resource, relation, params)
  }

  // --- relation managers: the related records of one record ---

  private relatedPath(resource: string, key: RecordKey, relation: string): string {
    return `/resources/${resource}/records/${encodeURIComponent(String(key))}/relations/${encodeURIComponent(relation)}`
  }

  relatedList<T = ResourceRecord>(resource: string, key: RecordKey, relation: string, params: ListParams = {}): Promise<RelatedListResponse<T>> {
    return this.request(`${this.relatedPath(resource, key, relation)}/records${listQuery(params)}`)
  }

  /** Creates a related record; the server links it to the parent. */
  relatedCreate<T = ResourceRecord>(resource: string, key: RecordKey, relation: string, data: ResourceRecord): Promise<RecordResponse<T>> {
    return this.request(`${this.relatedPath(resource, key, relation)}/records`, { method: 'POST', body: JSON.stringify({ data }) })
  }

  /** Many-to-many: links existing records (already linked ones are left alone). */
  attach(resource: string, key: RecordKey, relation: string, records: RecordKey[]): Promise<{ count: number }> {
    return this.request(`${this.relatedPath(resource, key, relation)}/attach`, { method: 'POST', body: JSON.stringify({ records }) })
  }

  /** Many-to-many: removes links; the records stay. */
  detach(resource: string, key: RecordKey, relation: string, records: RecordKey[]): Promise<{ count: number }> {
    return this.request(`${this.relatedPath(resource, key, relation)}/detach`, { method: 'POST', body: JSON.stringify({ records }) })
  }

  /** Puts records in this order (the manual order column); the server reuses their order values. */
  reorder(resource: string, records: RecordKey[]): Promise<{ count: number }> {
    return this.request(`/resources/${resource}/reorder`, { method: 'POST', body: JSON.stringify({ records }) })
  }

  delete(resource: string, key: RecordKey): Promise<null> {
    return this.request(`/resources/${resource}/records/${encodeURIComponent(String(key))}`, { method: 'DELETE' })
  }

  /** Runs a server action; "delete" deletes the given records. */
  action(resource: string, name: string, records: RecordKey[], data: ResourceRecord = {}): Promise<ActionResponse> {
    return this.request(`/resources/${resource}/actions/${encodeURIComponent(name)}`, {
      method: 'POST',
      body: JSON.stringify({ records, data }),
    })
  }

  /**
   * Uploads a file for a form field and reports progress (0–1). Uses XMLHttpRequest, as
   * fetch cannot report upload progress.
   */
  async upload(
    resource: string,
    field: string,
    file: File,
    onProgress?: (fraction: number) => void,
    /** The record being edited: the server then checks Update on it instead of Create. */
    record?: RecordKey,
  ): Promise<UploadResponse> {
    const send = (token: string) =>
      new Promise<{ status: number; body: unknown }>((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        const query = record === undefined ? '' : `?record=${encodeURIComponent(String(record))}`
        xhr.open('POST', `${this.baseUrl}/resources/${resource}/uploads/${encodeURIComponent(field)}${query}`)
        xhr.withCredentials = true
        xhr.setRequestHeader('Accept', 'application/json')
        xhr.setRequestHeader(CSRF_HEADER, token)
        for (const [k, v] of Object.entries(this.extraHeaders())) xhr.setRequestHeader(k, v)
        xhr.upload.onprogress = (e) => e.lengthComputable && onProgress?.(e.loaded / e.total)
        xhr.onload = () => {
          const fresh = xhr.getResponseHeader(CSRF_HEADER)
          if (fresh) this.csrfToken = fresh
          let body: unknown = null
          try {
            body = JSON.parse(xhr.responseText)
          } catch {
            /* not JSON */
          }
          resolve({ status: xhr.status, body })
        }
        xhr.onerror = () => reject(new Error(t('common.uploadNetwork')))
        const form = new FormData()
        form.append('file', file)
        xhr.send(form)
      })

    let res = await send(this.csrfToken ?? (await this.fetchToken()))
    if (res.status === 419) res = await send(await this.fetchToken())
    if (res.status < 200 || res.status >= 300) {
      const error = new ApiError(res.status, (res.body as ErrorBody | null) ?? { message: 'Upload failed.' })
      if (error.isUnauthenticated) this.onUnauthenticated?.()
      throw error
    }
    return res.body as UploadResponse
  }

  // --- transport ---

  private async fetchToken(): Promise<string> {
    const res = await fetch(this.baseUrl + '/auth/csrf', { credentials: 'same-origin', headers: { Accept: 'application/json' } })
    const body = (await res.json()) as { token: string }
    this.csrfToken = body.token
    return body.token
  }

  private async request<T>(path: string, init: RequestInit = {}, retried = false): Promise<T> {
    const method = (init.method ?? 'GET').toUpperCase()
    const headers: Record<string, string> = {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...this.extraHeaders(),
      ...(init.headers as Record<string, string>),
    }
    // Files (FormData): the browser sets the multipart Content-Type with its boundary.
    if (typeof FormData !== 'undefined' && init.body instanceof FormData) delete headers['Content-Type']
    if (!READING.has(method)) {
      headers[CSRF_HEADER] = this.csrfToken ?? (await this.fetchToken())
    }

    const res = await fetch(this.baseUrl + path, { ...init, method, headers, credentials: 'same-origin' })
    const fresh = res.headers.get(CSRF_HEADER)
    if (fresh) this.csrfToken = fresh

    // The session may have been renewed elsewhere (another tab logged in or out):
    // get a fresh token and try once more.
    if (res.status === 419 && !retried) {
      await this.fetchToken()
      return this.request(path, init, true)
    }

    const body = res.status === 204 ? null : await res.json().catch(() => null)
    if (!res.ok) {
      const error = new ApiError(res.status, body ?? { message: res.statusText })
      if (error.isUnauthenticated && path !== '/auth/login') this.onUnauthenticated?.()
      throw error
    }
    return body as T
  }
}
