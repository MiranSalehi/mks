import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useId, useRef, useState, type FormEvent } from 'react'
import type { Action } from '../actions'
import { ApiError } from '../api'
import { applyChange, contextFor, groupErrors, initialValues, missingRequired, payload, REQUIRED_MESSAGE } from '../formState'
import type { FieldErrors, RecordKey, ResourceRecord } from '../protocol'
import type { ResourceDefinition } from '../resource'
import type { Color } from '../table'
import { visibleFields } from '../form'
import { FormResourceContext, Schema } from './Schema'
import { Modal, type ModalHandle } from './Modal'
import { usePanel } from './Panel'
import { t } from '../i18n'

/** Button shapes; combine with a color from actionButtonColors. */
export const buttonBase =
  'inline-flex items-center justify-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold whitespace-nowrap active:scale-[0.98] disabled:opacity-50 disabled:active:scale-100'
export const buttonSmall =
  'inline-flex items-center justify-center gap-1.5 rounded-lg px-3 py-1.5 text-sm font-semibold whitespace-nowrap active:scale-[0.98] disabled:opacity-50 disabled:active:scale-100'

export const actionButtonColors: Record<Color, string> = {
  primary: 'bg-primary-600 text-white shadow-sm shadow-primary-600/30 ring-1 ring-inset ring-white/10 hover:bg-primary-500 dark:bg-primary-500 dark:hover:bg-primary-400',
  danger: 'bg-danger-600 text-white shadow-sm shadow-danger-600/30 ring-1 ring-inset ring-white/10 hover:bg-danger-500',
  success: 'bg-success-600 text-white shadow-sm shadow-success-600/30 ring-1 ring-inset ring-white/10 hover:bg-success-500',
  warning: 'bg-warning-500 text-white shadow-sm shadow-warning-500/30 ring-1 ring-inset ring-white/10 hover:bg-warning-400',
  info: 'bg-info-600 text-white shadow-sm shadow-info-600/30 ring-1 ring-inset ring-white/10 hover:bg-info-500',
  gray: 'bg-white text-gray-800 shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 hover:ring-gray-950/15 dark:bg-white/5 dark:text-gray-100 dark:ring-white/10 dark:hover:bg-white/10',
}

export const actionTextColors: Record<Color, string> = {
  primary: 'text-primary-600 hover:text-primary-500 dark:text-primary-400',
  danger: 'text-danger-600 hover:text-danger-500 dark:text-danger-400',
  success: 'text-success-600 hover:text-success-500 dark:text-success-400',
  warning: 'text-warning-600 hover:text-warning-500 dark:text-warning-400',
  info: 'text-info-600 hover:text-info-500 dark:text-info-400',
  gray: 'text-gray-600 hover:text-gray-500 dark:text-gray-400',
}

export function actionLabel(action: Action<any>): string {
  return action._label ?? (action.kind === 'delete' ? t('common.delete') : action.name)
}

interface Pending {
  action: Action<any>
  records: ResourceRecord[]
}

/**
 * Runs actions for a resource: straight away, or after a modal when the action asks for
 * confirmation or has a form. Render `modal` somewhere in the page.
 */
export function useActionRunner(resource: ResourceDefinition, onDone?: (action: Action<any>) => void) {
  const { api, meta, notify } = usePanel()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [pending, setPending] = useState<Pending | null>(null)
  const key = meta.resources[resource.name]?.key ?? 'id'

  async function execute(action: Action<any>, records: ResourceRecord[], data: ResourceRecord = {}) {
    const keys = records.map((r) => r[key] as RecordKey)
    const result = await api.action(resource.name, action.kind === 'delete' ? 'delete' : action.name, keys, data)
    if (result.notification) notify(result.notification.title, result.notification.type)
    for (const name of result.invalidate) {
      await queryClient.invalidateQueries({ predicate: (q) => ['records', 'record', 'options'].includes(String(q.queryKey[0])) && q.queryKey[1] === name })
    }
    onDone?.(action)
    if (result.open_url) {
      const target = new URL(result.open_url, window.location.origin)
      if (target.origin !== window.location.origin || !['http:', 'https:'].includes(target.protocol)) {
        notify('Rejected unsafe action URL', 'danger')
        return
      }
      window.location.assign(target.href)
    } else if (result.redirect) await navigate({ to: result.redirect })
  }

  /** Starts an action on the given records (none for header actions). */
  function start(action: Action<any>, records: ResourceRecord[] = []) {
    if (action.needsModal) {
      setPending({ action, records })
      return
    }
    execute(action, records).catch((err: unknown) => notify(errorText(err), 'danger'))
  }

  const modal = pending && (
    <ActionModal
      resource={resource}
      action={pending.action}
      count={pending.records.length}
      record={pending.records.length === 1 ? pending.records[0] : undefined}
      onClose={() => setPending(null)}
      onSubmit={(data) => execute(pending.action, pending.records, data)}
    />
  )

  return { start, modal }
}

function errorText(err: unknown): string {
  if (err instanceof ApiError && err.isValidation) {
    const first = Object.values(err.errors)[0]?.[0]
    return first ?? err.message
  }
  return err instanceof Error ? err.message : String(err)
}


function ActionModal({
  resource,
  action,
  count,
  record,
  onClose,
  onSubmit,
}: {
  resource: ResourceDefinition
  action: Action<any>
  count: number
  /** The record of a single-record action, for its form (FieldContext.record). */
  record?: ResourceRecord
  /** After the modal has closed (with its animation). */
  onClose: () => void
  onSubmit: (data: ResourceRecord) => Promise<void>
}) {
  const modal = useRef<ModalHandle>(null)
  const formId = useId()
  const fields = action._fields
  const [values, setValues] = useState<ResourceRecord>(() => initialValues(fields))
  const [errors, setErrors] = useState<FieldErrors>({})
  const [general, setGeneral] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const isDelete = action.kind === 'delete'
  const heading =
    action._confirmation?.heading ??
    action._modalHeading ??
    (isDelete ? (count > 1 ? t('common.deleteCount', { count, label: resource.label }) : t('common.deleteNamed', { name: resource.label })) : actionLabel(action))
  const description =
    action._confirmation?.description ??
    action._modalDescription ??
    (isDelete ? t('common.cannotUndo') : fields.length === 0 ? t('common.areYouSure') : undefined)
  const submitLabel = action._confirmation?.submitLabel ?? action._modalSubmitLabel ?? (isDelete ? t('common.delete') : t('common.confirm'))
  const color = action._color === 'gray' ? 'primary' : action._color

  const ctx = contextFor(values, 'create', record)
  const visible = visibleFields(action._schema, ctx)

  async function submit(e: FormEvent) {
    e.preventDefault()
    const missing = missingRequired(visible, values, ctx)
    if (missing.length > 0) {
      setErrors(Object.fromEntries(missing.map((n) => [n, [REQUIRED_MESSAGE]])))
      return
    }
    setBusy(true)
    setGeneral(null)
    try {
      await onSubmit(payload('create', visible, values, {}, ctx))
      modal.current?.close()
    } catch (err) {
      if (err instanceof ApiError && err.isValidation && Object.keys(err.errors).length > 0) {
        // Field errors for fields in the modal show under them; others become a message.
        const own = Object.fromEntries(Object.entries(groupErrors(fields, err.errors)).filter(([k]) => fields.some((f) => f.name === k)))
        setErrors(own)
        if (Object.keys(own).length === 0) setGeneral(errorText(err))
      } else {
        setGeneral(errorText(err))
      }
      setBusy(false)
    }
  }

  return (
    <Modal
      ref={modal}
      title={heading}
      description={description}
      tone={isDelete || color === 'danger' ? 'danger' : 'default'}
      dismissible={!busy}
      closeOnBackdrop={fields.length === 0}
      onClose={onClose}
      footer={
        <>
          <button type="submit" form={formId} disabled={busy} className={buttonBase + ' ' + actionButtonColors[color]}>
            {busy ? t('common.pleaseWait') : submitLabel}
          </button>
          <button type="button" onClick={() => modal.current?.close()} disabled={busy} className={buttonBase + ' ' + actionButtonColors.gray}>
            {t('common.cancel')}
          </button>
        </>
      }
    >
      <form id={formId} onSubmit={submit} noValidate className="space-y-5">
        {general && (
          <p role="alert" className="rounded-xl bg-danger-50 p-3 text-sm text-danger-700 ring-1 ring-danger-600/10 dark:bg-danger-500/10 dark:text-danger-300">
            {general}
          </p>
        )}

        {visible.length > 0 && (
          <FormResourceContext.Provider value={resource.name}>
            <Schema
              nodes={action._schema}
              columns={1}
              ctx={ctx}
              values={values}
              errors={errors}
              onChange={(name, v) => {
                const r = applyChange(fields, values, name, v, 'create')
                setValues(r.values)
                setErrors((prev) => {
                  const next = { ...prev }
                  for (const n of r.changed) delete next[n]
                  return next
                })
              }}
            />
          </FormResourceContext.Provider>
        )}
      </form>
    </Modal>
  )
}
