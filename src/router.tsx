import { readFilterStates, urlValue } from './filters'
import { t } from './i18n'
import { createRootRoute, createRoute, createRouter, lazyRouteComponent, Link, Navigate, redirect, useParams } from '@tanstack/react-router'
import { Suspense } from 'react'
import { clusterPages, Layout } from './components/Layout'
import { PageNameContext } from './components/PageKit'
import { NotFound } from './components/NotFound'
import { usePanel } from './components/Panel'

const ResourceList = lazyRouteComponent(() => import('./components/ResourceList'), 'ResourceList')
const ResourceCreate = lazyRouteComponent(() => import('./components/ResourceCreate'), 'ResourceCreate')
const ResourceEdit = lazyRouteComponent(() => import('./components/ResourceEdit'), 'ResourceEdit')
const ResourceView = lazyRouteComponent(() => import('./components/ResourceView'), 'ResourceView')
const Dashboard = lazyRouteComponent(() => import('./components/Dashboard'), 'Dashboard')

/** List page state kept in the URL, so reloads and shared links show the same view. */
export interface ListSearch {
  page?: number
  perPage?: number
  /** Comma-separated, "-" for descending, e.g. "-published_at,title". */
  sort?: string
  search?: string
  /** The name of the active row group. */
  group?: string
  /** Filter states, one key per active filter: filter[status]=draft,published */
  [filter: `filter[${string}]`]: string | number | boolean | undefined
}

function positiveInt(v: unknown): number | undefined {
  const n = typeof v === 'number' ? v : typeof v === 'string' ? Number(v) : NaN
  return Number.isInteger(n) && n > 0 ? n : undefined
}

function nonEmpty(v: unknown): string | undefined {
  if (typeof v === 'number') return String(v)
  return typeof v === 'string' && v.trim() !== '' ? v : undefined
}

export function validateListSearch(s: Record<string, unknown>): ListSearch {
  const out: ListSearch = { page: positiveInt(s.page), perPage: positiveInt(s.perPage), sort: nonEmpty(s.sort), search: nonEmpty(s.search), group: nonEmpty(s.group) }
  for (const [name, state] of Object.entries(readFilterStates(s))) out[`filter[${name}]`] = urlValue(state)
  return out
}

function useCurrentResource() {
  const { resource } = useParams({ strict: false }) as { resource?: string }
  const { resources } = usePanel()
  return resources.find((r) => r.name === resource)
}

function PagePending() {
  return (
    <div role="status" aria-label={t('common.loading')} className="space-y-4">
      <div className="h-10 w-64 animate-pulse rounded-lg bg-gray-200/70 dark:bg-white/5" />
      <div className="h-72 animate-pulse rounded-2xl bg-white ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10" />
    </div>
  )
}

function ListPage() {
  const resource = useCurrentResource()
  const { pages } = usePanel()
  const { resource: name } = useParams({ strict: false }) as { resource?: string }
  if (resource)
    return (
      <Suspense fallback={<PagePending />}>
        <ResourceList key={resource.name} resource={resource} />
      </Suspense>
    )
  // Custom pages share the first URL segment with resources.
  const page = pages.find((p) => p.name === name)
  if (!page) return <NotFound />
  const Page = page.component
  return (
    <PageNameContext.Provider value={page.name}>
      {page.cluster && <ClusterTabs cluster={page.cluster} current={page.name} />}
      <Page />
    </PageNameContext.Provider>
  )
}

/** The pages of a cluster as tabs above each of them. */
function ClusterTabs({ cluster, current }: { cluster: string; current: string }) {
  const { pages, navigation, meta } = usePanel()
  const def = navigation.clusters.find((c) => c.name === cluster)
  const list = clusterPages(pages, cluster, meta)
  if (!def || list.length < 2) return null
  return (
    <nav aria-label={def.label} className="mb-6 flex flex-wrap gap-1 border-b border-gray-200 dark:border-white/10">
      {list.map((p) => (
        <Link
          key={p.name}
          to="/$resource"
          params={{ resource: p.name }}
          aria-current={p.name === current ? 'page' : undefined}
          className={
            '-mb-px border-b-2 px-3 py-2 text-sm font-medium ' +
            (p.name === current
              ? 'border-primary-600 text-primary-700 dark:border-primary-400 dark:text-primary-300'
              : 'border-transparent text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200')
          }
        >
          {p.label}
        </Link>
      ))}
    </nav>
  )
}

function EditPage() {
  const resource = useCurrentResource()
  const { id } = useParams({ strict: false }) as { id?: string }
  return resource && id && resource.fields.length > 0 ? (
    <Suspense fallback={<PagePending />}>
      <ResourceEdit resource={resource} id={id} />
    </Suspense>
  ) : (
    <NotFound />
  )
}

function ViewPage() {
  const resource = useCurrentResource()
  const { id } = useParams({ strict: false }) as { id?: string }
  if (!resource || !id) return <NotFound />
  if (resource.infolist.length > 0)
    return (
      <Suspense fallback={<PagePending />}>
        <ResourceView resource={resource} id={id} />
      </Suspense>
    )
  // Without an infolist the record lives on its edit page.
  if (resource.fields.length > 0) return <Navigate to="/$resource/$id/edit" params={{ resource: resource.name, id }} replace />
  return <NotFound />
}

function CreatePage() {
  const resource = useCurrentResource()
  return resource && resource.fields.length > 0 ? (
    <Suspense fallback={<PagePending />}>
      <ResourceCreate key={resource.name} resource={resource} />
    </Suspense>
  ) : (
    <NotFound />
  )
}

/**
 * Routes, relative to basePath (e.g. /admin):
 *   /                    → dashboard, or the first resource without one
 *   /{resource}          → list
 *   /{resource}/create   → create form
 *   /{resource}/{id}     → view page (infolist), or the edit page without one
 *   /{resource}/{id}/edit → edit form
 */
function DashboardPage() {
  const { dashboard } = usePanel()
  return dashboard ? (
    <Suspense fallback={<PagePending />}>
      <Dashboard dashboard={dashboard} />
    </Suspense>
  ) : (
    <NotFound />
  )
}

export function createPanelRouter(basePath: string, firstResource: () => string | undefined, hasDashboard: () => boolean = () => false) {
  const root = createRootRoute({ component: Layout, notFoundComponent: NotFound })
  const index = createRoute({
    getParentRoute: () => root,
    path: '/',
    beforeLoad: () => {
      if (hasDashboard()) return
      const first = firstResource()
      if (first) throw redirect({ to: '/$resource', params: { resource: first }, replace: true })
    },
    component: DashboardPage,
  })
  const list = createRoute({
    getParentRoute: () => root,
    path: '$resource',
    component: ListPage,
    validateSearch: validateListSearch,
  })
  const create = createRoute({ getParentRoute: () => root, path: '$resource/create', component: CreatePage })
  const edit = createRoute({ getParentRoute: () => root, path: '$resource/$id/edit', component: EditPage })
  const view = createRoute({ getParentRoute: () => root, path: '$resource/$id', component: ViewPage })

  return createRouter({
    routeTree: root.addChildren([index, list, create, view, edit]),
    basepath: basePath,
    defaultPreload: 'intent',
  })
}
