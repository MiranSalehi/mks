import { useBlocker } from '@tanstack/react-router'
import { useMemo, useRef, useState, type FormEvent, type ReactNode } from 'react'
import { ApiError } from '../api'
import { applyChange, contextFor, dirtyFields, groupErrors, missingRequired, payload, REQUIRED_MESSAGE } from '../formState'
import type { FieldErrors, ResourceRecord } from '../protocol'
import { humanize, type ResourceDefinition } from '../resource'
import { isLayout, visibleFields, Wizard } from '../form'
import { fieldLabel } from './fields'
import { RecordKeyContext } from './formContext'
import { FormControlsContext, FormResourceContext, Schema } from './Schema'
import { t } from '../i18n'

export interface ResourceFormProps {
  resource: ResourceDefinition
  mode: 'create' | 'edit'
  initial: ResourceRecord
  /** The stored record when editing; fields can read it in their conditions. */
  record?: ResourceRecord
  /** Sends the data. Resolve when saved; throw an ApiError to show field errors. */
  onSubmit: (data: ResourceRecord) => Promise<void>
  submitLabel: string
  cancel: ReactNode
  /** The user may look but not change: fields are disabled and there is no save button. */
  readOnly?: boolean
  /** Key of the record being edited. */
  recordKey?: string
  /** Fields left out of the form and the request, e.g. a key the server sets. */
  omit?: string[]
  /** Prefix for field ids, when another form is on the page (e.g. in a modal). */
  idPrefix?: string
}

const LEAVE_MESSAGE = t('form.leave')

export function ResourceForm({ resource, mode, initial, record, onSubmit, submitLabel, cancel, readOnly = false, recordKey, omit, idPrefix = '' }: ResourceFormProps) {
  const omitted = useMemo(() => new Set(omit), [omit])
  const [values, setValues] = useState<ResourceRecord>(initial)
  // What "unchanged" means: the starting values, then whatever was last saved.
  const [baseline, setBaseline] = useState<ResourceRecord>(initial)
  const [errors, setErrors] = useState<FieldErrors>({})
  const [generalError, setGeneralError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)
  const leaving = useRef(false)
  // Latest values for event handlers, so quick successive changes never see stale state.
  const valuesRef = useRef(values)
  valuesRef.current = values

  // Conditions (visible, disabled, required, ...) are evaluated in the browser on every
  // render: no server round trip.
  const ctx = contextFor(values, mode, record)
  const visible = visibleFields(resource.schema, ctx).filter((f) => !omitted.has(f.name))

  function change(name: string, value: unknown) {
    const result = applyChange(resource.fields, valuesRef.current, name, value, mode, record)
    valuesRef.current = result.values
    setValues(result.values)
    setErrors((prev) => {
      const next = { ...prev }
      for (const n of result.changed) delete next[n]
      return next
    })
  }
  const dirty = useMemo(() => dirtyFields(resource.fields, values, baseline).length > 0, [resource, values, baseline])

  useBlocker({
    shouldBlockFn: () => dirty && !leaving.current && !window.confirm(LEAVE_MESSAGE),
    enableBeforeUnload: () => dirty && !leaving.current,
  })

  async function submit(e: FormEvent) {
    e.preventDefault()
    if (readOnly) return
    // Quick feedback without a request; the server checks again.
    const missing = missingRequired(visible, values, ctx)
    if (missing.length > 0) {
      setErrors(Object.fromEntries(missing.map((name) => [name, [REQUIRED_MESSAGE]])))
      setGeneralError(null)
      // After the render (a tab or folded section with errors opens first), focus the first
      // invalid field that is on screen.
      window.setTimeout(() => {
        const el = missing
          .map((name) => document.getElementById(`field-${idPrefix}${name}`))
          .find((e) => e && e.getClientRects().length > 0)
        el?.focus()
      }, 0)
      return
    }
    const data = payload(mode, visible, values, baseline, ctx)
    if (mode === 'edit' && Object.keys(data).length === 0) return
    setSaving(true)
    setErrors({})
    setGeneralError(null)
    try {
      leaving.current = true // the caller may navigate away after saving
      await onSubmit(data)
      setBaseline(values)
    } catch (err) {
      leaving.current = false
      if (err instanceof ApiError && err.isValidation) setErrors(groupErrors(resource.fields, err.errors))
      else setGeneralError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
      if (mode === 'edit') leaving.current = false
    }
  }

  const hasLayout = resource.schema.some(isLayout)

  // Errors for fields that are not on screen still need to be shown somewhere.
  const shown = new Set(visible.filter((f) => f.kind !== 'hidden').map((f) => f.name as string))
  const orphanErrors = Object.entries(errors).filter(([field]) => !shown.has(field.split('.')[0]))
  // A wizard brings its own submit button on the last step.
  const hasWizard = resource.schema.some((n) => n instanceof Wizard)
  const labelOf = (name: string) => {
    const f = resource.fields.find((x) => x.name === name)
    return f ? fieldLabel(f) : humanize(name)
  }

  return (
    <form onSubmit={submit} className="space-y-6" noValidate>
      {(generalError || orphanErrors.length > 0) && (
        <div role="alert" className="rounded-xl bg-danger-50 p-4 text-sm text-danger-700 ring-1 ring-danger-600/20 dark:bg-danger-600/10 dark:text-danger-400">
          {generalError && <p>{generalError}</p>}
          {orphanErrors.map(([field, msgs]) => (
            <p key={field}>
              {labelOf(field)}: {msgs.join(' ')}
            </p>
          ))}
        </div>
      )}

      <FormControlsContext.Provider value={{ setErrors, saving, readOnly }}>
      <RecordKeyContext.Provider value={recordKey}>
      <FormResourceContext.Provider value={resource.name}>
        {/* Loose fields get a card; forms built from sections and tabs bring their own. */}
        <div className={hasLayout ? undefined : 'rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10'}>
          <Schema nodes={resource.schema} ctx={ctx} values={values} errors={errors} onChange={change} idPrefix={idPrefix || undefined} omit={omitted} />
        </div>
      </FormResourceContext.Provider>
      </RecordKeyContext.Provider>
      </FormControlsContext.Provider>

      <div className="flex flex-wrap items-center gap-3">
        {!hasWizard && !readOnly && (
          <button
            type="submit"
            disabled={saving || (mode === 'edit' && !dirty)}
            className="inline-flex items-center justify-center gap-2 rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/30 ring-1 ring-inset ring-white/10 hover:bg-primary-500 active:scale-[0.98] dark:bg-primary-500 dark:hover:bg-primary-400 disabled:opacity-60"
          >
            {saving ? t('form.saving') : submitLabel}
          </button>
        )}
        {cancel}
        {dirty && <span className="text-sm text-gray-500 dark:text-gray-400">{t('form.unsaved')}</span>}
      </div>
    </form>
  )
}
