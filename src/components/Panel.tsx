import { QueryClient, QueryClientProvider, useQuery, useQueryClient } from '@tanstack/react-query'
import type { ClusterDefinition, NavigationGroupDefinition, NavigationLinkDefinition, UserMenuItemDefinition } from '../navigation'
import { RouterProvider } from '@tanstack/react-router'
import { createContext, useContext, useEffect, useMemo, useRef, useState, type ReactNode, useLayoutEffect } from 'react'
import { ApiClient, ApiError } from '../api'
import { PROTOCOL_VERSION, type MetaResponse, type NotificationType } from '../protocol'
import type { ResourceDefinition } from '../resource'
import { createPanelRouter } from '../router'
import { AccountWidget, TableWidget, type DashboardDefinition } from '../widgets'
import type { PageDefinition } from '../pages'
import type { PanelPlugin } from '../extensions'
import { useTheme, type Theme } from '../theme'
import { setAppearance, useAppearance, type Appearance } from '../appearance'
import { checkDefinitions } from '../devWarnings'
import { CheckIcon, CloseIcon, WarningIcon, XIcon } from './icons'
import { LoginPage } from './LoginPage'
import { setCustomLocales, setTranslationOverrides, direction, locale, t, type CustomLocale } from '../i18n'

interface PanelContextValue {
  api: ApiClient
  meta: MetaResponse
  /** Definitions the server also knows, in definition order. */
  resources: ResourceDefinition[]
  /** The dashboard, when the user may see at least one of its widgets. */
  dashboard?: DashboardDefinition
  /** Custom pages the user may open (the server lists them in meta). */
  pages: PageDefinition[]
  /** Topbar items from plugins. */
  topbar: readonly ReactNode[]
  /** Sidebar group order, extra links and clusters (the app's and its plugins'). */
  navigation: PanelNavigation
  tenantLabel: string
  notify: (message: string, type?: NotificationType) => void
  /** Fetches meta again, e.g. after a plugin changed the sidebar; the sidebar redraws. */
  refreshMeta: () => Promise<void>
  logout: () => Promise<void>
  theme: Theme
  setTheme: (t: Theme) => void
}

const PanelContext = createContext<PanelContextValue | null>(null)

export function usePanel(): PanelContextValue {
  const ctx = useContext(PanelContext)
  if (!ctx) throw new Error('usePanel must be used inside <Panel>')
  return ctx
}

export interface PanelProps {
  /** Shown on the login page before the server's brand is known. */
  brand: string
  /** Where the panel is mounted, e.g. "/admin". The API lives under {basePath}/api. */
  basePath: string
  resources: ResourceDefinition[]
  /** The home page; without it, "/" opens the first resource. */
  dashboard?: DashboardDefinition
  /** Custom pages (definePage), each backed by a panel.Page on the server. */
  pages?: PageDefinition[]
  /** Feature bundles: their resources, pages, widgets and topbar items are added. */
  plugins?: PanelPlugin[]
  /** What a tenant is called in the UI (multi-tenancy), e.g. "Team". */
  tenantLabel?: string
  /** The order of the sidebar groups (and their icons); groups not listed come after. */
  navigationGroups?: NavigationGroupDefinition[]
  /** Extra sidebar links. */
  navigationLinks?: NavigationLinkDefinition[]
  /** Clusters pages can join (definePage({cluster})). */
  clusters?: ClusterDefinition[]
  /** Entries of the user menu. */
  userMenuItems?: UserMenuItemDefinition[]
  /** Overrides the direction that comes with the language. */
  dir?: 'rtl' | 'ltr'
}

export function Panel(props: PanelProps) {
  const [queryClient] = useState(
    () => new QueryClient({ defaultOptions: { queries: { retry: false, refetchOnWindowFocus: false } } }),
  )
  return (
    <QueryClientProvider client={queryClient}>
      <PanelGate {...props} />
    </QueryClientProvider>
  )
}

const NO_PLUGINS: PanelPlugin[] = []

export interface PanelNavigation {
  groups: NavigationGroupDefinition[]
  links: NavigationLinkDefinition[]
  clusters: ClusterDefinition[]
  userMenu: UserMenuItemDefinition[]
}

function PanelGate({ brand, basePath, resources: ownResources, dashboard: ownDashboard, pages: ownPages, plugins = NO_PLUGINS, tenantLabel = t('panel.workspace'), dir: props_dir, navigationGroups, navigationLinks, clusters, userMenuItems }: PanelProps) {
  const navigation = useMemo(
    (): PanelNavigation => ({
      groups: navigationGroups ?? [],
      links: [...(navigationLinks ?? []), ...plugins.flatMap((p) => p.navigationLinks ?? [])],
      // The first definition of a cluster wins (the app's before its plugins').
      clusters: [...(clusters ?? []), ...plugins.flatMap((p) => p.clusters ?? [])].filter((c, i, all) => all.findIndex((x) => x.name === c.name) === i),
      userMenu: [...(userMenuItems ?? []), ...plugins.flatMap((p) => p.userMenuItems ?? [])],
    }),
    [navigationGroups, navigationLinks, clusters, userMenuItems, plugins],
  )
  // Plugins add to what the app defines.
  const [resources, pages, dashboard, topbar] = useMemo((): [ResourceDefinition[], PageDefinition[], DashboardDefinition | undefined, ReactNode[]] => {
    const widgets = plugins.flatMap((p) => p.widgets ?? [])
    const dash =
      widgets.length === 0 ? ownDashboard : { heading: ownDashboard?.heading ?? t('panel.dashboard'), widgets: [...(ownDashboard?.widgets ?? []), ...widgets] }
    return [
      [...ownResources, ...plugins.flatMap((p) => p.resources ?? [])],
      [...(ownPages ?? []), ...plugins.flatMap((p) => p.pages ?? [])],
      dash,
      plugins.map((p) => p.topbar).filter(Boolean),
    ]
  }, [ownResources, ownDashboard, ownPages, plugins])
  const queryClient = useQueryClient()
  const look = useAppearance()
  const [theme, setTheme] = useTheme(look.mode ?? 'system')
  const [api] = useState(() => new ApiClient(basePath.replace(/\/$/, '') + '/api'))

  // A 401 anywhere means the session is gone: drop all cached data and show the login page.
  // Only while logged in, so the meta request's own 401 does not loop.
  api.onUnauthenticated = () => {
    if (queryClient.getQueryData(['meta'])) void signOut(queryClient)
  }

  const meta = useQuery({
    queryKey: ['meta'],
    queryFn: async () => {
      try {
        return await api.meta()
      } catch (err) {
        // This tab's saved tenant is no longer the user's: forget it and use the default.
        if (err instanceof ApiError && err.isForbidden && api.tenant !== null) {
          api.setTenant(null)
          return api.meta()
        }
        throw err
      }
    },
    staleTime: Infinity,
  })
  // Sidebar badges count records: every write invalidates the records of a resource, so
  // refetch meta (once per burst) when a resource with a badge may have changed.
  const hasBadges = Object.values(meta.data?.resources ?? {}).some((r) => r.badge)
  useEffect(() => {
    if (!hasBadges) return
    let timer: ReturnType<typeof setTimeout> | undefined
    const unsubscribe = queryClient.getQueryCache().subscribe((e) => {
      if (e.type !== 'updated' || e.action.type !== 'invalidate' || e.query.queryKey[0] !== 'records') return
      clearTimeout(timer)
      timer = setTimeout(() => void queryClient.invalidateQueries({ queryKey: ['meta'], exact: true }), 300)
    })
    return () => {
      clearTimeout(timer)
      unsubscribe()
    }
  }, [hasBadges, queryClient])
  // The look (appearance plugin): the user's own, else the company's.
  const looks = meta.data?.extra?.appearance as { company?: Appearance | null; user?: Appearance | null } | undefined
  const lookKey = JSON.stringify(looks?.user ?? looks?.company ?? null)
  useLayoutEffect(() => setAppearance(JSON.parse(lookKey) as Appearance | null), [lookKey])
  // Plugins read what the server sent (e.g. uploaded plugins' blocks) before anything renders.
  if (meta.data) for (const p of plugins) p.onMeta?.(meta.data)
  // Languages and text edits of managers (translations plugin), before anything renders.
  setCustomLocales(meta.data?.extra?.locales as CustomLocale[] | undefined)
  setTranslationOverrides((meta.data?.extra?.translations as Record<string, Record<string, string>> | undefined)?.[locale()])
  // Remember the tenant the server settled on, so every request of this tab sends it.
  const current = meta.data?.tenancy?.current
  if (current != null && api.tenant !== String(current)) api.setTenant(String(current))

  // The page follows the UI language: Persian right to left, English left to right.
  const dir = props_dir ?? direction()
  const lang = locale()
  useEffect(() => {
    document.documentElement.dir = dir
    document.documentElement.lang = lang.replace('_', '-')
  }, [dir, lang])

  // Resources plugins build from meta (e.g. content types of uploaded plugins).
  const metaData = meta.data
  const allResources = useMemo(() => {
    const extra = metaData ? plugins.flatMap((p) => p.metaResources?.(metaData) ?? []) : []
    return extra.length > 0 ? [...resources, ...extra] : resources
  }, [resources, plugins, metaData])

  if (meta.isPending) {
    return (
      <div className="flex min-h-screen items-center justify-center" role="status" aria-label={t('common.loading')}>
        <span className="size-8 animate-spin rounded-full border-[3px] border-primary-500/20 border-t-primary-600" />
      </div>
    )
  }
  if (meta.error) {
    if (meta.error instanceof ApiError && meta.error.isUnauthenticated) {
      return <LoginPage api={api} brand={brand} onLoggedIn={() => queryClient.resetQueries({ queryKey: ['meta'] })} />
    }
    return <ServerError message={meta.error.message} onRetry={() => meta.refetch()} />
  }
  return (
    <LoggedIn api={api} meta={meta.data} definitions={allResources} dashboard={dashboard} pageDefinitions={pages} topbar={topbar} navigation={navigation} tenantLabel={tenantLabel} basePath={basePath} theme={theme} setTheme={setTheme} />
  )
}

function LoggedIn({
  api,
  meta,
  definitions,
  dashboard: dashboardDefinition,
  pageDefinitions,
  topbar,
  navigation,
  tenantLabel,
  basePath,
  theme,
  setTheme,
}: {
  api: ApiClient
  meta: MetaResponse
  definitions: ResourceDefinition[]
  dashboard?: DashboardDefinition
  pageDefinitions?: PageDefinition[]
  topbar: readonly ReactNode[]
  navigation: PanelNavigation
  tenantLabel: string
  basePath: string
  theme: Theme
  setTheme: (t: Theme) => void
}) {
  const queryClient = useQueryClient()
  const [toasts, setToasts] = useState<Toast[]>([])
  const nextToast = useRef(0)
  // A dismissed toast plays its exit animation, then leaves.
  const dismiss = (id: number) => {
    setToasts((all) => all.map((t) => (t.id === id ? { ...t, leaving: true } : t)))
    window.setTimeout(() => setToasts((all) => all.filter((t) => t.id !== id)), 220)
  }

  const resources = useMemo(() => definitions.filter((d) => meta.resources[d.name]), [definitions, meta])
  // Shown only if the user may see one of its widgets (server widgets in meta, tables of
  // visible resources).
  const dashboard = useMemo(() => {
    if (!dashboardDefinition) return undefined
    const visible = dashboardDefinition.widgets.some((w) =>
      w instanceof AccountWidget ? true : w instanceof TableWidget ? resources.some((r) => r.name === w._resource) : (meta.widgets ?? []).includes(w.name),
    )
    return visible ? dashboardDefinition : undefined
  }, [dashboardDefinition, resources, meta])
  const pages = useMemo(() => (pageDefinitions ?? []).filter((p) => (meta.pages ?? []).includes(p.name)), [pageDefinitions, meta])
  const firstRef = useRef<string | undefined>(undefined)
  firstRef.current = resources[0]?.name
  const dashboardRef = useRef(dashboard)
  dashboardRef.current = dashboard
  const [router] = useState(() => createPanelRouter(basePath, () => firstRef.current, () => !!dashboardRef.current))

  useEffect(() => {
    if (meta.protocol !== PROTOCOL_VERSION) {
      console.warn(`[panel] server speaks protocol ${meta.protocol}, this UI speaks ${PROTOCOL_VERSION}`)
    }
    checkDefinitions(definitions, meta, pageDefinitions)
  }, [definitions, meta, pageDefinitions])

  const ctx = useMemo<PanelContextValue>(
    () => ({
      api,
      meta,
      resources,
      dashboard,
      pages,
      topbar,
      navigation,
      tenantLabel,
      theme,
      setTheme,
      notify: (message, type = 'success') => {
        const id = ++nextToast.current
        setToasts((all) => [...all.slice(-3), { id, message, type }])
        window.setTimeout(() => dismiss(id), type === 'danger' ? 8000 : 4000)
      },
      refreshMeta: async () => {
        await queryClient.invalidateQueries({ queryKey: ['meta'], exact: true })
      },
      logout: async () => {
        await api.logout()
        await signOut(queryClient)
      },
    }),
    [api, meta, resources, dashboard, pages, topbar, navigation, tenantLabel, theme, setTheme, queryClient],
  )

  return (
    <PanelContext.Provider value={ctx}>
      <RouterProvider router={router} />
      <Notifications toasts={toasts} onDismiss={dismiss} />
      {meta.protocol !== PROTOCOL_VERSION && (
        <ProtocolMismatch server={meta.protocol} ui={PROTOCOL_VERSION} />
      )}
    </PanelContext.Provider>
  )
}

// ProtocolMismatch is a visible warning, not just a console line: a stale UI (an old
// go:embed, or @mks/mks not matching the Go module) fails in confusing ways, silently.
function ProtocolMismatch({ server, ui }: { server: number; ui: number }) {
  return (
    <div
      role="alert"
      style={{
        position: 'fixed',
        insetInline: 0,
        bottom: 0,
        zIndex: 100,
        padding: '8px 16px',
        background: '#991b1b',
        color: '#fff',
        fontSize: 13,
        lineHeight: 1.5,
        textAlign: 'center',
      }}
    >
      The panel's UI and server versions do not match: the server speaks protocol {server}, this UI
      speaks {ui}. Rebuild the frontend (or run <code>./artisan mks:assets</code>) so @mks/mks and
      the Go module are on the same version.
    </div>
  )
}


interface Toast {
  id: number
  message: string
  type: NotificationType
  leaving?: boolean
}

const toastIcon: Record<NotificationType, ReactNode> = {
  success: <CheckIcon small />,
  danger: <XIcon />,
  warning: <WarningIcon />,
  info: <CheckIcon small />,
}

const toastTone: Record<NotificationType, string> = {
  success: 'bg-success-50 text-success-600 dark:bg-success-500/15 dark:text-success-400',
  danger: 'bg-danger-50 text-danger-600 dark:bg-danger-500/15 dark:text-danger-400',
  warning: 'bg-warning-50 text-warning-600 dark:bg-warning-500/15 dark:text-warning-400',
  info: 'bg-info-50 text-info-600 dark:bg-info-500/15 dark:text-info-400',
}

function Notifications({ toasts, onDismiss }: { toasts: Toast[]; onDismiss: (id: number) => void }) {
  return (
    // Errors are announced right away, other messages politely.
    <div className="pointer-events-none fixed bottom-6 start-6 z-50 flex w-96 max-w-[calc(100vw-3rem)] flex-col gap-3">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          role={toast.type === 'danger' ? 'alert' : 'status'}
          className={
            'pointer-events-auto flex animate-toast items-center gap-3 rounded-2xl bg-white/95 p-3 pe-2 text-sm shadow-xl ring-1 ring-gray-950/5 backdrop-blur ' +
            'transition-[opacity,transform] duration-200 dark:bg-gray-900/95 dark:text-white dark:ring-white/10 ' +
            (toast.leaving ? 'translate-y-2 scale-95 opacity-0' : '')
          }
        >
          <span className={'flex size-9 shrink-0 items-center justify-center rounded-xl [&_svg]:size-5 ' + toastTone[toast.type]}>{toastIcon[toast.type]}</span>
          <p className="flex-1 leading-6 font-medium text-gray-800 dark:text-gray-100">{toast.message}</p>
          <button type="button" onClick={() => onDismiss(toast.id)} aria-label={t('common.close')} className="rounded-lg p-1.5 text-gray-400 hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-white/5 dark:hover:text-gray-200 [&_svg]:size-4">
            <CloseIcon />
          </button>
        </div>
      ))}
    </div>
  )
}

/** Forgets every cached record, then re-checks meta, which shows the login page. */
async function signOut(queryClient: QueryClient) {
  await queryClient.cancelQueries()
  queryClient.removeQueries({ predicate: (q) => q.queryKey[0] !== 'meta' })
  await queryClient.resetQueries({ queryKey: ['meta'] })
}

function ServerError({ message, onRetry }: { message: string; onRetry: () => void }): ReactNode {
  return (
    <div className="flex min-h-screen items-center justify-center p-6">
      <div className="max-w-sm animate-page rounded-2xl bg-white p-8 text-center shadow-lg ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10">
        <span className="mx-auto flex size-12 items-center justify-center rounded-full bg-danger-50 text-danger-600 dark:bg-danger-500/10 dark:text-danger-400">
          <WarningIcon />
        </span>
        <p className="mt-4 font-semibold text-gray-950 dark:text-white">{t('panel.serverDown')}</p>
        <p className="mt-2 text-sm text-gray-500">{message}</p>
        <button
          type="button"
          onClick={onRetry}
          className="mt-6 rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/30 hover:bg-primary-500"
        >
          {t('panel.retry')}
        </button>
      </div>
    </div>
  )
}
