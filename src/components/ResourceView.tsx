import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useMemo } from 'react'
import { ApiError } from '../api'
import { entryPaths } from '../infolist'
import type { ResourceDefinition } from '../resource'
import { actionButtonColors, actionLabel, buttonBase, useActionRunner } from './ActionModal'
import { Breadcrumbs } from './Breadcrumbs'
import { Infolist } from './Infolist'
import { Forbidden, NotFound } from './NotFound'
import { usePanel } from './Panel'
import { RelationManagers } from './RelationManager'
import { t } from '../i18n'

/** The view page: the record's infolist, its actions and its relation managers. */
export function ResourceView({ resource, id }: { resource: ResourceDefinition; id: string }) {
  const { api, meta } = usePanel()
  const navigate = useNavigate()
  const runner = useActionRunner(resource, (action) => {
    if (action.kind === 'delete') void navigate({ to: '/$resource', params: { resource: resource.name } })
  })

  // Relations the entries read (e.g. "category.name", "tags"), as far as the server offers them.
  const include = useMemo(() => {
    const offered = new Set(meta.resources[resource.name]?.relations ?? [])
    return [...new Set(entryPaths(resource.infolist).map((p) => p.split('.')[0]))].filter((r) => offered.has(r))
  }, [resource, meta])

  const record = useQuery({
    queryKey: ['record', resource.name, id, { include }],
    queryFn: () => api.get(resource.name, id, include),
  })

  if (record.isPending) {
    return (
      <div className="space-y-6">
        <div className="h-10 w-64 animate-pulse rounded-lg bg-gray-200/70 dark:bg-white/5" />
        <div className="h-72 animate-pulse rounded-2xl bg-white ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10" />
      </div>
    )
  }
  if (record.error) {
    if (record.error instanceof ApiError && record.error.status === 404) return <NotFound />
    if (record.error instanceof ApiError && record.error.isForbidden) return <Forbidden />
    return <p className="text-sm text-danger-600">{t('common.recordError', { message: record.error.message })}</p>
  }

  const data = record.data.data
  const abilities = record.data.abilities
  const title = resource.recordTitle(data) || `#${id}`
  const canEdit = resource.fields.length > 0 && abilities?.update !== false
  // Row actions (and delete) as page buttons, when allowed and visible for this record.
  const actions = resource.actions.filter((a) => {
    if (!a._placements.has('row') && a.kind !== 'delete') return false
    if (!a.isVisibleFor(data)) return false
    if (a.kind === 'delete') return abilities?.delete !== false
    if (a.kind === 'server') return abilities?.actions[a.name] !== false
    return true
  })

  return (
    <div className="space-y-6">
      <header>
        <Breadcrumbs resource={resource.name} label={resource.pluralLabel} items={[title]} />
        <div className="mt-3 flex flex-wrap items-center justify-between gap-4">
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{title}</h1>
          <div className="flex flex-wrap items-center gap-2">
            {actions.map((action) => (
              <button
                key={action.name}
                type="button"
                onClick={() => runner.start(action, [data])}
                className={
                  buttonBase +
                  ' ' +
                  (action.kind === 'delete'
                    ? 'bg-white text-danger-600 shadow-xs ring-1 ring-danger-600/20 hover:bg-danger-50 dark:bg-danger-500/10 dark:text-danger-400 dark:ring-danger-400/20 dark:hover:bg-danger-500/20'
                    : actionButtonColors.gray)
                }
              >
                {actionLabel(action)}
              </button>
            ))}
            {canEdit && (
              <Link to="/$resource/$id/edit" params={{ resource: resource.name, id }} className={buttonBase + ' ' + actionButtonColors.primary}>
                {t('common.edit')}
              </Link>
            )}
          </div>
        </div>
      </header>

      <Infolist nodes={resource.infolist} record={data} />

      {resource.relations.length > 0 && <RelationManagers resource={resource} id={id} />}
      {runner.modal}
    </div>
  )
}
