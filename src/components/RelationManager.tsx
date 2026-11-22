import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useRef, useState, type FormEvent } from 'react'
import { ApiError } from '../api'
import { initialValues } from '../formState'
import type { RecordKey, ResourceRecord } from '../protocol'
import type { RelationManager } from '../relations'
import { humanize, type ResourceDefinition } from '../resource'
import { actionButtonColors, actionTextColors, buttonBase, buttonSmall } from './ActionModal'
import { Cell } from './cells'
import { Combobox } from './Combobox'
import { inputClass } from './fields'
import { ChevronIcon, PlusIcon } from './icons'
import { Modal, type ModalHandle } from './Modal'
import { usePanel } from './Panel'
import { RecordLinks } from './RecordLinks'
import { ResourceForm } from './ResourceForm'
import { numberFormat, t } from '../i18n'

const nf = numberFormat()
const smallButton = buttonSmall + ' '
const pagerButton =
  'rounded-lg bg-white px-3 py-1 text-sm font-medium text-gray-700 shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 disabled:opacity-40 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10'

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = window.setTimeout(() => setV(value), ms)
    return () => window.clearTimeout(t)
  }, [value, ms])
  return v
}

/** The relation managers of a record, under its edit form. */
export function RelationManagers({ resource, id }: { resource: ResourceDefinition; id: string }) {
  const { resources } = usePanel()
  return (
    <>
      {resource.relations.map((m) => {
        // A related resource the user cannot see (not in meta) has no manager.
        const target = resources.find((r) => r.name === m._resource)
        return target ? <RelationManagerView key={m.relation} parent={resource} id={id} manager={m} target={target} /> : null
      })}
    </>
  )
}

type Dialog = { kind: 'create' } | { kind: 'attach' } | { kind: 'detach' | 'delete'; record: ResourceRecord }

function RelationManagerView({
  parent,
  id,
  manager,
  target,
}: {
  parent: ResourceDefinition
  id: string
  manager: RelationManager<any>
  target: ResourceDefinition
}) {
  const { api, meta, notify } = usePanel()
  const queryClient = useQueryClient()
  const headingId = useId()
  const [page, setPage] = useState(1)
  const [sort, setSort] = useState<string | undefined>(target.defaultSort)
  const [searchInput, setSearchInput] = useState('')
  const search = useDebounced(searchInput.trim(), 300)
  const [dialog, setDialog] = useState<Dialog | null>(null)
  useEffect(() => setPage(1), [search, sort])

  const columns = manager.columns ?? target.columns
  const include = [...new Set(columns.flatMap((c) => c.relations))]
  const key = meta.resources[target.name]?.key ?? 'id'
  const searchable = columns.some((c) => c._searchable)
  const label = manager._label ?? target.pluralLabel
  const deleteAction = target.actions.find((a) => a.kind === 'delete')

  const query = useQuery({
    queryKey: ['related', parent.name, id, manager.relation, { page, sort, search, include }],
    queryFn: () =>
      api.relatedList(parent.name, id, manager.relation, {
        page,
        perPage: manager._perPage,
        sort: sort ? sort.split(',') : undefined,
        search: search || undefined,
        include,
      }),
    placeholderData: keepPreviousData,
  })
  const rel = query.data?.relation
  const pageMeta = query.data?.meta

  async function refresh() {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ['related', parent.name, id, manager.relation] }),
      queryClient.invalidateQueries({ queryKey: ['records', target.name] }),
    ])
  }

  const toggleSort = (name: string) =>
    setSort((s) => (s === name ? `-${name}` : s === `-${name}` ? target.defaultSort : name))

  return (
    <section aria-labelledby={headingId} className="overflow-hidden rounded-2xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10">
      <header className="flex flex-wrap items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-white/5">
        <h2 id={headingId} className="text-base font-semibold tracking-tight text-gray-950 dark:text-white">
          {label}
        </h2>
        {pageMeta && (
          <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-semibold text-gray-600 dark:bg-white/10 dark:text-gray-300">{nf.format(pageMeta.total)}</span>
        )}
        <div className="flex w-full flex-wrap items-center gap-2 sm:ms-auto sm:w-auto">
          {searchable && (
            <input
              type="search"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              placeholder={t('common.searchPlaceholder')}
              aria-label={t('common.searchIn', { name: label })}
              className={inputClass + ' w-full py-1.5 sm:w-48'}
            />
          )}
          {rel?.attach && (
            <button type="button" onClick={() => setDialog({ kind: 'attach' })} className={smallButton + actionButtonColors.gray}>
              {t('related.attach')}
            </button>
          )}
          {rel?.create && target.fields.length > 0 && (
            <button type="button" onClick={() => setDialog({ kind: 'create' })} className={smallButton + actionButtonColors.primary}>
              <PlusIcon />
              {t('common.new', { label: target.label })}
            </button>
          )}
        </div>
      </header>

      {query.isError ? (
        <p className="p-4 text-sm text-danger-600">
          {query.error instanceof ApiError && query.error.isForbidden ? t('related.forbidden') : t('common.fetchError', { message: query.error.message })}
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className={'w-full text-sm whitespace-nowrap ' + (query.isFetching && !query.isPending ? 'opacity-60' : '')}>
            <thead className="border-b border-gray-100 bg-gray-50/70 dark:border-white/5 dark:bg-white/[0.02]">
              <tr>
                {columns.map((c) => {
                  const text = c._label ?? humanize(c.name)
                  const dir = sort === c.name ? 'up' : sort === `-${c.name}` ? 'down' : 'both'
                  return (
                    <th
                      key={c.name}
                      scope="col"
                      aria-sort={dir === 'up' ? 'ascending' : dir === 'down' ? 'descending' : undefined}
                      className="px-5 py-3 text-start text-xs font-semibold text-gray-500 dark:text-gray-400"
                    >
                      {c._sortable ? (
                        <button type="button" onClick={() => toggleSort(c.name)} className="group inline-flex items-center gap-1 hover:text-gray-900 dark:hover:text-white">
                          {text}
                          <span className={dir !== 'both' ? 'text-gray-700 dark:text-gray-200' : 'text-gray-300 group-hover:text-gray-500 dark:text-gray-600'}>
                            <ChevronIcon dir={dir} />
                          </span>
                        </button>
                      ) : (
                        text
                      )}
                    </th>
                  )
                })}
                <th scope="col">
                  <span className="sr-only">{t('common.actions')}</span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-white/5">
              {query.isPending && (
                <tr>
                  <td colSpan={columns.length + 1} className="px-4 py-3">
                    <div className="h-5 animate-pulse rounded bg-gray-100 dark:bg-white/5" />
                  </td>
                </tr>
              )}
              {query.data?.data.map((record) => {
                const rk = String(record[key])
                const abilities = query.data.abilities?.[rk]
                const title = target.recordTitle(record)
                const canUpdate = abilities?.update !== false
                return (
                  <tr key={rk} className="transition-colors hover:bg-gray-50/80 dark:hover:bg-white/[0.03]">
                    {columns.map((c) => (
                      <td key={c.name} className="px-5 py-3 text-gray-700 dark:text-gray-300">
                        <Cell column={c} record={record} />
                      </td>
                    ))}
                    <td className="px-5 py-3 text-end">
                      <div className="flex items-center justify-end gap-3">
                        {rel?.detach && (
                          <button type="button" onClick={() => setDialog({ kind: 'detach', record })} className={'text-sm font-semibold ' + actionTextColors.gray} aria-label={t('related.detachNamed', { name: title })}>
                            {t('related.detach')}
                          </button>
                        )}
                        {deleteAction && abilities?.delete && deleteAction.isVisibleFor(record) && (
                          <button type="button" onClick={() => setDialog({ kind: 'delete', record })} className={'text-sm font-semibold ' + actionTextColors.danger} aria-label={t('common.deleteNamed', { name: title })}>
                            {t('common.delete')}
                          </button>
                        )}
                        <RecordLinks resource={target} recordKey={rk} title={title} canUpdate={canUpdate} />
                      </div>
                    </td>
                  </tr>
                )
              })}
              {query.data?.data.length === 0 && (
                <tr>
                  <td colSpan={columns.length + 1} className="px-4 py-10 text-center text-gray-500 dark:text-gray-400">
                    {search ? t('related.emptySearch') : t('related.empty')}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}

      {pageMeta && pageMeta.last_page > 1 && (
        <nav aria-label={t('related.pages', { label })} className="flex items-center justify-between border-t border-gray-100 px-5 py-3 text-sm text-gray-600 dark:border-white/5 dark:text-gray-400">
          <span>
            {t('related.page', { page: pageMeta.page, last: pageMeta.last_page })}
          </span>
          <div className="flex gap-2">
            <button type="button" disabled={page <= 1} onClick={() => setPage(page - 1)} className={pagerButton}>
              {t('common.previous')}
            </button>
            <button type="button" disabled={page >= pageMeta.last_page} onClick={() => setPage(page + 1)} className={pagerButton}>
              {t('common.next')}
            </button>
          </div>
        </nav>
      )}

      {dialog?.kind === 'create' && (
        <CreateDialog
          target={target}
          omit={rel?.foreign_key ? [rel.foreign_key] : undefined}
          idPrefix={`${manager.relation}-new-`}
          onClose={() => setDialog(null)}
          onCreate={async (data) => {
            await api.relatedCreate(parent.name, id, manager.relation, data)
            await refresh()
            notify(t('common.created', { label: target.label }))
          }}
        />
      )}

      {dialog?.kind === 'attach' && (
        <AttachDialog
          parent={parent}
          relation={manager.relation}
          label={label}
          onClose={() => setDialog(null)}
          onAttach={async (keys) => {
            const res = await api.attach(parent.name, id, manager.relation, keys)
            await refresh()
            notify(t('related.attached', { count: res.count }))
          }}
        />
      )}

      {(dialog?.kind === 'detach' || dialog?.kind === 'delete') && (
        <ConfirmDialog
          title={dialog.kind === 'detach' ? t('related.detachNamed', { name: target.recordTitle(dialog.record) }) : t('common.deleteNamed', { name: target.recordTitle(dialog.record) })}
          description={
            dialog.kind === 'detach'
              ? t('related.detachHelp', { parent: parent.label, target: target.label })
              : t('common.cannotUndo')
          }
          submitLabel={dialog.kind === 'detach' ? t('related.detach') : t('common.delete')}
          onClose={() => setDialog(null)}
          onConfirm={async () => {
            const rk = dialog.record[key] as RecordKey
            if (dialog.kind === 'detach') await api.detach(parent.name, id, manager.relation, [rk])
            else await api.delete(target.name, rk)
            await refresh()
            notify(dialog.kind === 'detach' ? t('related.detached') : t('common.deleted', { label: target.label }))
          }}
        />
      )}
    </section>
  )
}

// --- dialogs ------------------------------------------------------------------------------

function errorText(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

/** The related resource's own form, in a wide dialog. */
function CreateDialog({
  target,
  omit,
  idPrefix,
  onClose,
  onCreate,
}: {
  target: ResourceDefinition
  omit?: string[]
  idPrefix: string
  onClose: () => void
  onCreate: (data: ResourceRecord) => Promise<void>
}) {
  const modal = useRef<ModalHandle>(null)
  return (
    <Modal ref={modal} title={t('common.new', { label: target.label })} size="lg" closeOnBackdrop={false} onClose={onClose}>
      <ResourceForm
        resource={target}
        mode="create"
        initial={initialValues(target.fields)}
        // The server sets the parent's key.
        omit={omit}
        // The parent's form is on the page too: keep field ids unique.
        idPrefix={idPrefix}
        submitLabel={t('common.create')}
        cancel={
          <button type="button" onClick={() => modal.current?.close()} className={buttonBase + ' ' + actionButtonColors.gray}>
            {t('common.cancel')}
          </button>
        }
        onSubmit={async (data) => {
          await onCreate(data)
          modal.current?.close()
        }}
      />
    </Modal>
  )
}

function ConfirmDialog({
  title,
  description,
  submitLabel,
  onClose,
  onConfirm,
}: {
  title: string
  description: string
  submitLabel: string
  onClose: () => void
  onConfirm: () => Promise<void>
}) {
  const modal = useRef<ModalHandle>(null)
  const formId = useId()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  async function submit(e: FormEvent) {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await onConfirm()
      modal.current?.close()
    } catch (err) {
      setError(errorText(err))
      setBusy(false)
    }
  }
  return (
    <Modal
      ref={modal}
      title={title}
      description={description}
      tone="danger"
      dismissible={!busy}
      onClose={onClose}
      footer={
        <>
          <button type="submit" form={formId} disabled={busy} className={buttonBase + ' ' + actionButtonColors.danger}>
            {busy ? t('common.pleaseWait') : submitLabel}
          </button>
          <button type="button" onClick={() => modal.current?.close()} disabled={busy} className={buttonBase + ' ' + actionButtonColors.gray}>
            {t('common.cancel')}
          </button>
        </>
      }
    >
      <form id={formId} onSubmit={submit}>
        {error && (
          <p role="alert" className="rounded-xl bg-danger-50 p-3 text-sm text-danger-700 ring-1 ring-danger-600/10 dark:bg-danger-500/10 dark:text-danger-300">
            {error}
          </p>
        )}
      </form>
    </Modal>
  )
}

function AttachDialog({
  parent,
  relation,
  label,
  onClose,
  onAttach,
}: {
  parent: ResourceDefinition
  relation: string
  label: string
  onClose: () => void
  onAttach: (keys: RecordKey[]) => Promise<void>
}) {
  const modal = useRef<ModalHandle>(null)
  const formId = useId()
  const [keys, setKeys] = useState<RecordKey[]>([])
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const fieldId = useId()
  async function submit(e: FormEvent) {
    e.preventDefault()
    if (keys.length === 0) {
      setError(t('related.pickOne'))
      return
    }
    setBusy(true)
    setError(null)
    try {
      await onAttach(keys)
      modal.current?.close()
    } catch (err) {
      setError(errorText(err))
      setBusy(false)
    }
  }
  return (
    <Modal
      ref={modal}
      title={t('related.attachTitle', { label })}
      description={t('related.attachHelp')}
      dismissible={!busy}
      closeOnBackdrop={keys.length === 0}
      onClose={onClose}
      footer={
        <>
          <button type="submit" form={formId} disabled={busy} className={buttonBase + ' ' + actionButtonColors.primary}>
            {busy ? t('common.pleaseWait') : t('related.attach')}
          </button>
          <button type="button" onClick={() => modal.current?.close()} disabled={busy} className={buttonBase + ' ' + actionButtonColors.gray}>
            {t('common.cancel')}
          </button>
        </>
      }
    >
      <form id={formId} onSubmit={submit} noValidate className="space-y-2">
        <label htmlFor={fieldId} className="block text-sm font-medium text-gray-950 dark:text-white">
          {label}
        </label>
        {/* Options come from the parent's relation: the records the user may see. */}
        <Combobox
          id={fieldId}
          resource={parent.name}
          relation={relation}
          multiple
          value={keys}
          onChange={(v) => setKeys(Array.isArray(v) ? (v as RecordKey[]) : [])}
          invalid={!!error}
          a11y={{}}
        />
        {error && (
          <p role="alert" className="text-sm text-danger-600 dark:text-danger-400">
            {error}
          </p>
        )}
      </form>
    </Modal>
  )
}
