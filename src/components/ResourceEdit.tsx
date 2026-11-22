import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { ApiError } from '../api'
import { Select } from '../form'
import { initialValues } from '../formState'
import type { ResourceDefinition } from '../resource'
import { actionButtonColors, actionLabel, buttonBase, useActionRunner } from './ActionModal'
import { Breadcrumbs } from './Breadcrumbs'
import { Forbidden, NotFound } from './NotFound'
import { usePanel } from './Panel'
import { secondaryButton } from './ResourceCreate'
import { RelationManagers } from './RelationManager'
import { ResourceForm } from './ResourceForm'
import { t } from '../i18n'

export function ResourceEdit({ resource, id }: { resource: ResourceDefinition; id: string }) {
  const { api, notify } = usePanel()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const deleteAction = resource.actions.find((a) => a.kind === 'delete')
  const runner = useActionRunner(resource, (action) => {
    if (action.kind === 'delete') void navigate({ to: '/$resource', params: { resource: resource.name } })
  })
  const toManyRelations = resource.fields
    .filter((f): f is Select<any> => f instanceof Select && f._multiple && !!f._relationship)
    .map((f) => f._relationship!)
  const record = useQuery({
    queryKey: ['record', resource.name, id],
    // To-many relation selects need the related keys.
    queryFn: () => api.get(resource.name, id, toManyRelations),
    // Keep the form's own state while editing; refetch only after saving.
    refetchOnWindowFocus: false,
  })

  if (record.isPending) {
    return <div className="h-64 animate-pulse rounded-2xl bg-white ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10" />
  }
  if (record.error) {
    if (record.error instanceof ApiError && record.error.status === 404) return <NotFound />
    if (record.error instanceof ApiError && record.error.isForbidden) return <Forbidden />
    return <p className="text-sm text-danger-600">{t('common.recordError', { message: record.error.message })}</p>
  }

  const title = resource.recordTitle(record.data.data) || `#${id}`
  // Without the right to update, the page is a read-only view.
  const canUpdate = record.data.abilities?.update !== false
  const canDelete = record.data.abilities?.delete !== false
  const verb = canUpdate ? t('common.edit') : t('common.view')

  return (
    <div className="space-y-6">
      <header>
        <Breadcrumbs resource={resource.name} label={resource.pluralLabel} items={[title, verb]} />
        <div className="mt-3 flex flex-wrap items-center justify-between gap-4">
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">
            {verb} {title}
          </h1>
          <div className="flex flex-wrap items-center gap-2">
          {resource.infolist.length > 0 && (
            <Link to="/$resource/$id" params={{ resource: resource.name, id }} className={buttonBase + ' ' + actionButtonColors.gray}>
              {t('common.view')}
            </Link>
          )}
          {deleteAction && canDelete && deleteAction.isVisibleFor(record.data.data) && (
            <button
              type="button"
              onClick={() => runner.start(deleteAction, [record.data.data])}
              className={
                buttonBase +
                ' bg-white text-danger-600 shadow-xs ring-1 ring-danger-600/20 hover:bg-danger-50 dark:bg-danger-500/10 dark:text-danger-400 dark:ring-danger-400/20 dark:hover:bg-danger-500/20'
              }
            >
              {actionLabel(deleteAction)}
            </button>
          )}
          </div>
        </div>
      </header>

      <ResourceForm
        // A new form (and baseline) when another record is opened.
        key={`${resource.name}:${id}`}
        resource={resource}
        mode="edit"
        initial={initialValues(resource.fields, record.data.data)}
        record={record.data.data}
        recordKey={id}
        readOnly={!canUpdate}
        submitLabel={t('form.saveChanges')}
        cancel={
          <Link to="/$resource" params={{ resource: resource.name }} className={secondaryButton}>
            {t('common.back')}
          </Link>
        }
        onSubmit={async (data) => {
          const res = await api.update(resource.name, id, data)
          queryClient.setQueryData(['record', resource.name, id], res)
          await queryClient.invalidateQueries({ queryKey: ['records', resource.name] })
          notify(t('form.saved'))
        }}
      />
      {resource.relations.length > 0 && <RelationManagers resource={resource} id={id} />}
      {runner.modal}
    </div>
  )
}
