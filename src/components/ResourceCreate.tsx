import { useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { useState } from 'react'
import { initialValues } from '../formState'
import type { ResourceDefinition } from '../resource'
import { Breadcrumbs } from './Breadcrumbs'
import { Forbidden } from './NotFound'
import { usePanel } from './Panel'
import { ResourceForm } from './ResourceForm'
import { t } from '../i18n'

export const secondaryButton =
  'inline-flex items-center justify-center gap-2 rounded-lg bg-white px-4 py-2 text-sm font-semibold text-gray-700 shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 hover:ring-gray-950/20 active:scale-[0.98] dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10'

export function ResourceCreate({ resource }: { resource: ResourceDefinition }) {
  const { api, notify, meta } = usePanel()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [initial] = useState(() => initialValues(resource.fields))
  const key = meta.resources[resource.name]?.key ?? 'id'
  if (meta.resources[resource.name]?.abilities?.create === false) return <Forbidden />

  return (
    <div className="space-y-6">
      <header>
        <Breadcrumbs resource={resource.name} label={resource.pluralLabel} items={[t('common.create')]} />
        <h1 className="mt-3 text-2xl font-bold tracking-tight sm:text-3xl">{t('common.new', { label: resource.label })}</h1>
      </header>

      <ResourceForm
        resource={resource}
        mode="create"
        initial={initial}
        submitLabel={t('common.create')}
        cancel={
          <Link to="/$resource" params={{ resource: resource.name }} className={secondaryButton}>
            {t('common.cancel')}
          </Link>
        }
        onSubmit={async (data) => {
          const res = await api.create(resource.name, data)
          await queryClient.invalidateQueries({ queryKey: ['records', resource.name] })
          notify(t('common.created', { label: resource.label }))
          // Like Filament: continue on the new record's edit page.
          await navigate({ to: '/$resource/$id/edit', params: { resource: resource.name, id: String(res.data[key]) } })
        }}
      />
    </div>
  )
}
