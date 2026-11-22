import { createContext, useContext, useEffect, useId, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import {
  Field,
  Fieldset,
  flattenFields,
  Grid,
  isLayout,
  Section,
  Step,
  Tab,
  Tabs,
  visibleFields,
  Wizard,
  type ColumnSpan,
  type Columns,
  type FieldContext,
  type SchemaNode,
} from '../form'
import { missingRequired, REQUIRED_MESSAGE } from '../formState'
import type { FieldErrors, ResourceRecord } from '../protocol'
import { ChevronIcon } from './icons'
import { FormField } from './fields'
import { numberFormat, t } from '../i18n'

export { FormResourceContext } from './formContext'

const gridCols: Record<Columns, string> = {
  1: 'grid gap-6',
  2: 'grid gap-6 md:grid-cols-2',
  3: 'grid gap-6 md:grid-cols-2 lg:grid-cols-3',
}

const spanClass = (span: ColumnSpan): string | undefined =>
  span === 'full' ? 'col-span-full' : span === 2 ? 'md:col-span-2' : span === 3 ? 'md:col-span-2 lg:col-span-3' : undefined

export interface SchemaProps {
  nodes: SchemaNode<any>[]
  columns?: Columns
  ctx: FieldContext<any>
  values: ResourceRecord
  errors: FieldErrors
  onChange: (name: string, value: unknown) => void
  /** Prefix for field ids: repeater and builder items, or a second form on the page. */
  idPrefix?: string
  /**
   * Fields left out, e.g. the parent's key in a relation manager. Applies to this level
   * and its layouts, not inside repeater items (they render their own Schema).
   */
  omit?: ReadonlySet<string>
}

export function Schema({ nodes, columns = 2, ...rest }: SchemaProps) {
  return (
    <div className={gridCols[columns]}>
      {nodes.map((node, i) => (
        <Node key={isLayout(node) ? `${node.kind}-${i}` : node.name} node={node} {...rest} />
      ))}
    </div>
  )
}

function Node({ node, ...rest }: Omit<SchemaProps, 'nodes' | 'columns'> & { node: SchemaNode<any> }): ReactNode {
  const { ctx, values, errors, onChange } = rest
  // A read-only form (the user may view but not edit) disables every field.
  const { readOnly } = useContext(FormControlsContext)
  if (!node.isVisible(ctx)) return null

  if (node instanceof Field) {
    if (node.kind === 'hidden') return null
    if (rest.omit?.has(node.name)) return null
    return (
      <div className={spanClass(node._columnSpan)}>
        <FormField
          field={node}
          value={values[node.name]}
          errors={errors[node.name]}
          required={node.isRequired(ctx)}
          disabled={readOnly || node.isDisabled(ctx)}
          ctx={ctx}
          nestedErrors={node.kind === 'repeater' || node.kind === 'builder' ? errorsUnder(errors, node.name) : undefined}
          idPrefix={rest.idPrefix}
          onChange={(v) => onChange(node.name, v)}
        />
      </div>
    )
  }

  const span = spanClass(node._columnSpan)
  if (node instanceof Section) return <SectionView section={node} className={span} {...rest} />
  if (node instanceof Fieldset) {
    return (
      <fieldset className={'rounded-xl bg-gray-50/50 p-4 ring-1 ring-gray-950/5 dark:bg-white/[0.02] dark:ring-white/10 ' + (span ?? '')}>
        <legend className="px-2 text-sm font-medium text-gray-950 dark:text-white">{node.legend}</legend>
        <Schema nodes={node._children} columns={node._columns} {...rest} />
      </fieldset>
    )
  }
  if (node instanceof Tabs) return <TabsView tabs={node._children as Tab<any>[]} className={span} {...rest} />
  if (node instanceof Wizard) return <WizardView wizard={node} className={span} {...rest} />
  if (node instanceof Grid || node instanceof Tab) {
    return (
      <div className={span}>
        <Schema nodes={node._children} columns={node._columns} {...rest} />
      </div>
    )
  }
  return null
}

const card = 'rounded-2xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10'

function SectionView({
  section,
  className,
  ...rest
}: Omit<SchemaProps, 'nodes' | 'columns'> & { section: Section<any>; className?: string }) {
  const [collapsed, setCollapsed] = useState(section._collapsed)
  const bodyId = useId()
  const errorCount = countErrors(section._children, rest.errors)
  // A folded section opens when its fields get errors, so they are not missed.
  useEffect(() => {
    if (errorCount > 0) setCollapsed(false)
  }, [errorCount])

  const header = (
    <div>
      <h2 className="text-base font-semibold text-gray-950 dark:text-white">{section.heading}</h2>
      {section._description && <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{section._description}</p>}
    </div>
  )
  return (
    <section className={card + ' ' + (className ?? '')}>
      <div className="border-b border-gray-100 px-6 py-4 dark:border-white/5">
        {section._collapsible ? (
          <button
            type="button"
            onClick={() => setCollapsed((c) => !c)}
            aria-expanded={!collapsed}
            aria-controls={bodyId}
            className="flex w-full items-center justify-between gap-4 text-start"
          >
            {header}
            <span className="text-gray-400">
              <ChevronIcon dir={collapsed ? 'down' : 'up'} />
            </span>
          </button>
        ) : (
          header
        )}
      </div>
      <div id={bodyId} hidden={collapsed} className="p-6">
        <Schema nodes={section._children} columns={section._columns} {...rest} />
      </div>
    </section>
  )
}

/** Errors of a field and of anything inside it (e.g. "faqs.0.question"). */
function errorsUnder(errors: FieldErrors, name: string): FieldErrors {
  return Object.fromEntries(Object.entries(errors).filter(([k]) => k === name || k.startsWith(name + '.')))
}

function countErrors(nodes: SchemaNode<any>[], errors: FieldErrors): number {
  return flattenFields(nodes).filter((f) => Object.keys(errorsUnder(errors, f.name)).length > 0).length
}

const faNumber = numberFormat()

function TabsView({
  tabs,
  className,
  ...rest
}: Omit<SchemaProps, 'nodes' | 'columns'> & { tabs: Tab<any>[]; className?: string }) {
  const visible = tabs.filter((t) => t.isVisible(rest.ctx))
  const [active, setActive] = useState(0)
  const baseId = useId()
  const buttons = useRef<(HTMLButtonElement | null)[]>([])
  const errorCounts = visible.map((t) => countErrors(t._children, rest.errors))
  const index = Math.min(active, visible.length - 1)

  // After a failed save, show the first tab that has errors.
  const firstWithErrors = errorCounts.findIndex((n) => n > 0)
  const errorsKey = errorCounts.join(',')
  useEffect(() => {
    if (firstWithErrors >= 0 && errorCounts[index] === 0) setActive(firstWithErrors)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [errorsKey])

  function onKeyDown(e: KeyboardEvent) {
    // Arrow keys follow reading direction: in RTL, ArrowLeft goes to the next tab.
    const rtl = getComputedStyle(e.currentTarget).direction === 'rtl'
    const step = { ArrowRight: rtl ? -1 : 1, ArrowLeft: rtl ? 1 : -1 }[e.key as 'ArrowRight' | 'ArrowLeft']
    let next = index
    if (step) next = (index + step + visible.length) % visible.length
    else if (e.key === 'Home') next = 0
    else if (e.key === 'End') next = visible.length - 1
    else return
    e.preventDefault()
    setActive(next)
    buttons.current[next]?.focus()
  }

  if (visible.length === 0) return null
  return (
    <div className={card + ' ' + (className ?? '')}>
      <div className="border-b border-gray-100 p-3 dark:border-white/5">
      <div role="tablist" onKeyDown={onKeyDown} className="flex w-fit max-w-full gap-1 overflow-x-auto rounded-xl bg-gray-100/80 p-1 dark:bg-white/5">
        {visible.map((tab, i) => (
          <button
            key={tab.label}
            ref={(el) => {
              buttons.current[i] = el
            }}
            id={`${baseId}-tab-${i}`}
            type="button"
            role="tab"
            aria-selected={i === index}
            aria-controls={`${baseId}-panel-${i}`}
            tabIndex={i === index ? 0 : -1}
            onClick={() => setActive(i)}
            className={
              'inline-flex items-center gap-2 rounded-lg px-3.5 py-1.5 text-sm font-medium whitespace-nowrap ' +
              (i === index
                ? 'bg-white text-gray-950 shadow-sm ring-1 ring-gray-950/5 dark:bg-white/10 dark:text-white dark:ring-white/10'
                : 'text-gray-500 hover:bg-white/60 hover:text-gray-800 dark:text-gray-400 dark:hover:bg-white/5 dark:hover:text-gray-200')
            }
          >
            {tab.label}
            {errorCounts[i] > 0 && (
              <span className="min-w-5 rounded-full bg-danger-600 px-1.5 text-center text-xs leading-5 text-white" aria-label={t('form.errors', { count: errorCounts[i] })}>
                {faNumber.format(errorCounts[i])}
              </span>
            )}
          </button>
        ))}
      </div>
      </div>
      {/* Inactive panels stay mounted (hidden) so their values and errors are kept. */}
      {visible.map((tab, i) => (
        <div
          key={tab.label}
          id={`${baseId}-panel-${i}`}
          role="tabpanel"
          aria-labelledby={`${baseId}-tab-${i}`}
          hidden={i !== index}
          className="p-6"
        >
          <Schema nodes={tab._children} columns={tab._columns} {...rest} />
        </div>
      ))}
    </div>
  )
}

/**
 * Lets layouts that check fields themselves (the wizard) report errors to the form, and
 * tells them whether the form is saving.
 */
export const FormControlsContext = createContext<{
  setErrors: (e: FieldErrors) => void
  saving: boolean
  readOnly?: boolean
}>({
  setErrors: () => {},
  saving: false,
})

function WizardView({
  wizard,
  className,
  ...rest
}: Omit<SchemaProps, 'nodes' | 'columns'> & { wizard: Wizard<any>; className?: string }) {
  const { setErrors, saving, readOnly } = useContext(FormControlsContext)
  const steps = (wizard._children as Step<any>[]).filter((s) => s.isVisible(rest.ctx))
  const [current, setCurrent] = useState(0)
  const index = Math.min(current, steps.length - 1)
  const headingRef = useRef<HTMLHeadingElement>(null)

  // After a failed save, go back to the first step with errors.
  const errorCounts = steps.map((s) => countErrors(s._children, rest.errors))
  const errorsKey = errorCounts.join(',')
  useEffect(() => {
    const first = errorCounts.findIndex((n) => n > 0)
    if (first >= 0 && first < index) setCurrent(first)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [errorsKey])

  function go(to: number) {
    setCurrent(to)
    window.setTimeout(() => headingRef.current?.focus(), 0)
  }

  function next() {
    // Only forward when this step's required fields are filled; the server checks again.
    const missing = missingRequired(visibleFields(steps[index]._children, rest.ctx), rest.values, rest.ctx)
    if (missing.length > 0) {
      setErrors(Object.fromEntries(missing.map((n) => [n, [REQUIRED_MESSAGE]])))
      return
    }
    go(index + 1)
  }

  if (steps.length === 0) return null
  const step = steps[index]
  const last = index === steps.length - 1
  return (
    <div className={card + ' ' + (className ?? '')}>
      <ol className="flex flex-wrap items-center gap-x-6 gap-y-3 border-b border-gray-100 px-6 py-4 text-sm dark:border-white/5">
        {steps.map((s, i) => (
          <li key={s.label} aria-current={i === index ? 'step' : undefined}>
            <StepLink
              // Earlier steps can always be revisited; when editing, any step can be opened.
              enabled={i !== index && (i < index || rest.ctx.operation === 'edit')}
              onClick={() => go(i)}
            >
            <span
              className={
                'flex size-7 items-center justify-center rounded-full text-xs font-semibold ' +
                (i < index
                  ? 'bg-primary-600 text-white shadow-sm shadow-primary-600/30'
                  : i === index
                    ? 'bg-primary-50 text-primary-700 ring-2 ring-primary-500 dark:bg-primary-500/10 dark:text-primary-300'
                    : 'bg-gray-100 text-gray-500 dark:bg-white/5 dark:text-gray-400')
              }
            >
              {i < index ? '✓' : faNumber.format(i + 1)}
            </span>
            <span className={i === index ? 'font-semibold' : 'text-gray-500 dark:text-gray-400'}>{s.label}</span>
            {errorCounts[i] > 0 && <span className="rounded-full bg-danger-600 px-1.5 text-xs text-white">{faNumber.format(errorCounts[i])}</span>}
            </StepLink>
          </li>
        ))}
      </ol>
      <div className="space-y-6 p-6">
        <div>
          <h2 ref={headingRef} tabIndex={-1} className="text-base font-semibold outline-none">
            {step.label}
          </h2>
          {step._description && <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{step._description}</p>}
        </div>
        {/* Other steps stay mounted (hidden) so their values and errors are kept. */}
        {steps.map((s, i) => (
          <div key={s.label} hidden={i !== index}>
            <Schema nodes={s._children} columns={s._columns} {...rest} />
          </div>
        ))}
        <div className="flex gap-3">
          {index > 0 && (
            <button type="button" onClick={() => go(index - 1)} className="rounded-lg bg-white px-4 py-2 text-sm font-semibold text-gray-700 shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10">
              {t('wizard.prev')}
            </button>
          )}
          {last && readOnly ? null : last ? (
            <button
              type="submit"
              disabled={saving}
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/30 ring-1 ring-inset ring-white/10 hover:bg-primary-500 active:scale-[0.98] dark:bg-primary-500 dark:hover:bg-primary-400 disabled:opacity-60"
            >
              {saving ? t('form.saving') : wizard._submitLabel}
            </button>
          ) : (
            <button type="button" onClick={next} className="inline-flex items-center justify-center gap-2 rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-sm shadow-primary-600/30 ring-1 ring-inset ring-white/10 hover:bg-primary-500 active:scale-[0.98] dark:bg-primary-500 dark:hover:bg-primary-400">
              {t('wizard.next')}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}

function StepLink({ enabled, onClick, children }: { enabled: boolean; onClick: () => void; children: ReactNode }) {
  return enabled ? (
    <button type="button" onClick={onClick} className="flex items-center gap-2 rounded-lg hover:opacity-80">
      {children}
    </button>
  ) : (
    <span className="flex items-center gap-2">{children}</span>
  )
}
