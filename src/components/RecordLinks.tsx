import { Link } from '@tanstack/react-router'
import type { ResourceDefinition } from '../resource'
import { actionTextColors } from './ActionModal'
import { t } from '../i18n'

/**
 * Links from a table row to its record. With an infolist: "view", and "edit" when the
 * user may update. Without: "edit", which is a read-only page for users who may not.
 */
export function RecordLinks({ resource, recordKey, title, canUpdate }: { resource: ResourceDefinition; recordKey: string; title: string; canUpdate: boolean }) {
  const params = { resource: resource.name, id: recordKey }
  const hasForm = resource.fields.length > 0
  if (resource.infolist.length > 0) {
    return (
      <>
        <Link to="/$resource/$id" params={params} className={'text-sm font-semibold ' + actionTextColors.gray} aria-label={t('common.viewNamed', { name: title })}>
          {t('common.view')}
        </Link>
        {hasForm && canUpdate && (
          <Link to="/$resource/$id/edit" params={params} className={'text-sm font-semibold ' + actionTextColors.primary} aria-label={t('common.editNamed', { name: title })}>
            {t('common.edit')}
          </Link>
        )}
      </>
    )
  }
  if (!hasForm) return null
  return (
    <Link
      to="/$resource/$id/edit"
      params={params}
      className={'text-sm font-semibold ' + (canUpdate ? actionTextColors.primary : actionTextColors.gray)}
      aria-label={canUpdate ? t('common.editNamed', { name: title }) : t('common.viewNamed', { name: title })}
    >
      {canUpdate ? t('common.edit') : t('common.view')}
    </Link>
  )
}
