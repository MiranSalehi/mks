import { useQuery, useQueryClient } from '@tanstack/react-query'
import { createContext, useContext, useId, useMemo, useState, type FormEvent, type ReactNode } from 'react'
import { ApiError } from '../api'
import { flattenFields, formBuilder, visibleFields, type FormBuilder, type SchemaNode } from '../form'
import { applyChange, contextFor, groupErrors, initialValues, missingRequired, payload, REQUIRED_MESSAGE } from '../formState'
import type { ActionResponse, FieldErrors, ResourceRecord } from '../protocol'
import { actionButtonColors, buttonBase } from './ActionModal'
import { usePanel } from './Panel'
import { FormControlsContext, FormResourceContext, Schema } from './Schema'
import { t } from '../i18n'

// Building blocks for custom pages (definePage).

export const PageNameContext = createContext<string | null>(null)

function usePageName(name?: string): string {
  const current = useContext(PageNameContext)
  const page = name ?? current
  if (!page) throw new Error('usePageData/usePageSubmit: use them inside a page, or pass the page name')
  return page
}

/** The page's server data (panel.Page Data), with loading and error state. */
export function usePageData<T = unknown>(name?: string) {
  const { api } = usePanel()
  const page = usePageName(name)
  return useQuery({ queryKey: ['page', page], queryFn: () => api.page<T>(page), select: (r) => r.data })
}

/**
 * Sends data to the page's Handle. Shows the result's notification and refreshes the
 * page data and any resources the result names. Throws ApiError on validation errors.
 */
export function usePageSubmit(name?: string) {
  const { api, notify } = usePanel()
  const queryClient = useQueryClient()
  const page = usePageName(name)
  return async (data: ResourceRecord): Promise<ActionResponse> => {
    const res = await api.submitPage(page, data)
    if (res.notification) notify(res.notification.title, res.notification.type)
    await queryClient.invalidateQueries({ queryKey: ['page', page] })
    for (const r of res.invalidate) await queryClient.invalidateQueries({ predicate: (q) => q.queryKey[1] === r })
    return res
  }
}

export function PageHeader({ title, description, actions }: { title: string; description?: ReactNode; actions?: ReactNode }) {
  return (
    <header className="flex flex-wrap items-end justify-between gap-4">
      <div>
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{title}</h1>
        {description && <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{description}</p>}
      </div>
      {actions && <div className="flex flex-wrap items-center gap-2">{actions}</div>}
    </header>
  )
}

/** A white card, optionally with a heading. */
export function Card({ heading, description, children, className = '' }: { heading?: string; description?: string; children: ReactNode; className?: string }) {
  const id = useId()
  return (
    <section
      aria-labelledby={heading ? id : undefined}
      className={'rounded-2xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10 ' + className}
    >
      {heading && (
        <header className="border-b border-gray-100 px-6 py-4 dark:border-white/5">
          <h2 id={id} className="text-base font-semibold tracking-tight text-gray-950 dark:text-white">
            {heading}
          </h2>
          {description && <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{description}</p>}
        </header>
      )}
      <div className="p-6">{children}</div>
    </section>
  )
}

/**
 * A form outside a resource, with the panel's fields and layouts:
 *
 *	<PanelForm schema={(f) => [f.textInput('title').required()]} onSubmit={usePageSubmit()} />
 *
 * Required fields are checked here first; the server's 422 errors show under the fields.
 */
export function PanelForm({
  schema,
  initial,
  submitLabel = t('common.save'),
  resetOnSuccess = false,
  columns = 1,
  onSubmit,
  actions,
  initialErrors,
}: {
  schema: (f: FormBuilder<any>) => SchemaNode<any>[]
  initial?: ResourceRecord
  submitLabel?: string
  resetOnSuccess?: boolean
  columns?: 1 | 2
  onSubmit: (data: ResourceRecord) => Promise<unknown>
  /** Buttons in place of the submit button (one of them must submit the form). */
  actions?: (state: { saving: boolean }) => ReactNode
  /** Errors to show from the start, by field name. */
  initialErrors?: FieldErrors
}) {
  // The schema is built once; the function may be an inline arrow.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  const nodes = useMemo(() => schema(formBuilder<any>()), [])
  const fields = useMemo(() => flattenFields(nodes), [nodes])
  const start = () => initialValues(fields, initial)
  const [values, setValues] = useState<ResourceRecord>(start)
  const [errors, setErrors] = useState<FieldErrors>(initialErrors ?? {})
  const [general, setGeneral] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const ctx = contextFor(values, 'create')
  const visible = visibleFields(nodes, ctx)

  async function submit(e: FormEvent) {
    e.preventDefault()
    // A form inside another (a panel over a resource form): React passes submit events up
    // the component tree, so stop it here or the outer form is sent too.
    e.stopPropagation()
    const missing = missingRequired(visible, values, ctx)
    if (missing.length > 0) {
      setErrors(Object.fromEntries(missing.map((n) => [n, [REQUIRED_MESSAGE]])))
      return
    }
    setSaving(true)
    setErrors({})
    setGeneral(null)
    try {
      await onSubmit(payload('create', visible, values, {}, ctx))
      if (resetOnSuccess) setValues(start())
    } catch (err) {
      if (err instanceof ApiError && err.isValidation && Object.keys(err.errors).length > 0) setErrors(groupErrors(fields, err.errors))
      else setGeneral(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <form onSubmit={submit} noValidate className="space-y-5">
      {general && (
        <p role="alert" className="rounded-xl bg-danger-50 p-3 text-sm text-danger-700 ring-1 ring-danger-600/10 dark:bg-danger-500/10 dark:text-danger-300">
          {general}
        </p>
      )}
      <FormControlsContext.Provider value={{ setErrors, saving }}>
        <FormResourceContext.Provider value="">
          <Schema
            nodes={nodes}
            columns={columns}
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
      </FormControlsContext.Provider>
      {actions ? (
        actions({ saving })
      ) : (
        <button type="submit" disabled={saving} className={buttonBase + ' ' + actionButtonColors.primary}>
          {saving ? t('form.sending') : submitLabel}
        </button>
      )}
    </form>
  )
}
