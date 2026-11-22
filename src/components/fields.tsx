import { CustomField } from '../extensions'
import { useQuery } from '@tanstack/react-query'
import { lazy, Suspense, useContext, type ReactNode } from 'react'
import {
  Builder,
  Checkbox,
  DatePicker,
  Field,
  FileUpload,
  Hidden,
  KeyValue,
  MarkdownEditor,
  Placeholder,
  Radio,
  Repeater,
  RichEditor,
  Select,
  TagsInput,
  Textarea,
  TextInput,
  Toggle,
  type FieldContext,
} from '../form'
import type { FieldErrors } from '../protocol'
import { humanize } from '../resource'
import { Combobox } from './Combobox'
import { DatePickerControl } from './DatePicker'
import { FileUploadControl } from './FileUpload'
import { KeyValueControl, TagsInputControl } from './JsonFields'
import { MarkdownEditorControl } from './MarkdownEditor'
import { BuilderControl, RepeaterControl } from './Repeater'
import { usePanel } from './Panel'
import { FormResourceContext } from './formContext'
import { t } from '../i18n'

const RichEditorControl = lazy(() => import('./RichEditor').then((m) => ({ default: m.RichEditorControl })))

// "mks-input" lets the look (appearance) restyle form fields: filled, underline, floating.
export const inputClass =
  'mks-input block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-950 shadow-xs ring-1 ring-gray-950/10 placeholder:text-gray-400 ' +
  'hover:ring-gray-950/20 focus:ring-2 focus:ring-primary-500 focus:shadow-[0_0_0_4px_color-mix(in_oklch,var(--color-primary-500)_15%,transparent)] focus:outline-none ' +
  'disabled:cursor-not-allowed disabled:bg-gray-50 disabled:text-gray-500 disabled:shadow-none dark:disabled:bg-transparent dark:disabled:text-gray-400 ' +
  'dark:bg-white/[0.04] dark:text-white dark:ring-white/10 dark:hover:ring-white/20 dark:focus:ring-primary-400 dark:placeholder:text-gray-500 dark:[&>option]:bg-gray-900'

interface FieldProps {
  field: Field<any>
  value: unknown
  errors?: string[]
  /** Computed by the form from the field's conditions. */
  required?: boolean
  disabled?: boolean
  ctx?: FieldContext<any>
  /** For repeaters and builders: errors of the fields inside their items. */
  nestedErrors?: FieldErrors
  /** Makes ids unique for fields inside repeater and builder items, e.g. "faqs-0-". */
  idPrefix?: string
  onChange: (value: unknown) => void
}

export function fieldLabel(field: Field<any>): string {
  return field._label ?? humanize(field.name)
}

function Errors({ id, errors }: { id: string; errors?: string[] }) {
  if (!errors?.length) return null
  return (
    <div id={id} className="space-y-1">
      {errors.map((e) => (
        <p key={e} className="text-sm text-danger-600 dark:text-danger-400">
          {e}
        </p>
      ))}
    </div>
  )
}

function Required({ on }: { on?: boolean }) {
  return on ? (
    <sup className="ms-0.5 font-medium text-danger-600" aria-hidden="true">
      *
    </sup>
  ) : null
}

export function FormField({ field, value, errors, required, disabled, ctx, nestedErrors, idPrefix = '', onChange }: FieldProps): ReactNode {
  if (field instanceof Hidden) return null
  const id = `field-${idPrefix}${field.name}`
  const errorId = `${id}-error`
  const helpId = `${id}-help`
  const invalid = !!errors?.length
  const describedBy = [field._helperText ? helpId : '', invalid ? errorId : ''].filter(Boolean).join(' ') || undefined
  const a11y = {
    'aria-invalid': invalid || undefined,
    'aria-describedby': describedBy,
    'aria-required': required || undefined,
    disabled: disabled || undefined,
  }
  const help = field._helperText && (
    <p id={helpId} className="text-sm text-gray-500 dark:text-gray-400">
      {field._helperText}
    </p>
  )

  if (field instanceof Placeholder) {
    return (
      <div className="space-y-2">
        <p className="text-sm font-medium text-gray-950 dark:text-white">{fieldLabel(field)}</p>
        <div className="text-sm text-gray-700 dark:text-gray-300" aria-live="polite">
          {ctx ? field._content(ctx) : ''}
        </div>
        {help}
      </div>
    )
  }

  // Booleans: the control sits beside its label.
  if (field instanceof Toggle || field instanceof Checkbox) {
    const checked = Boolean(value)
    return (
      <div className="space-y-2">
        <div className="flex items-center gap-3">
          {field instanceof Toggle ? (
            <button
              id={id}
              type="button"
              role="switch"
              aria-checked={checked}
              {...a11y}
              onClick={() => onChange(!checked)}
              className={
                'relative inline-flex h-6 w-11 shrink-0 rounded-full transition-colors focus:ring-2 focus:ring-primary-600 focus:ring-offset-2 focus:outline-none disabled:cursor-not-allowed disabled:opacity-50 dark:focus:ring-offset-gray-900 ' +
                (checked ? 'bg-primary-600' : 'bg-gray-200 dark:bg-gray-700')
              }
            >
              <span
                aria-hidden="true"
                className={
                  'absolute top-0.5 size-5 rounded-full bg-white shadow transition-[inset-inline-start] ' +
                  (checked ? 'start-[1.375rem]' : 'start-0.5')
                }
              />
            </button>
          ) : (
            <input
              id={id}
              type="checkbox"
              checked={checked}
              {...a11y}
              onChange={(e) => onChange(e.target.checked)}
              className="mks-check"
            />
          )}
          <label htmlFor={id} className="text-sm font-medium text-gray-950 dark:text-white">
            {fieldLabel(field)}
            <Required on={required} />
          </label>
        </div>
        {help}
        <Errors id={errorId} errors={errors} />
      </div>
    )
  }

  if (field instanceof Radio) {
    return (
      <fieldset className="space-y-2" {...a11y}>
        <legend className="text-sm font-medium text-gray-950 dark:text-white">
          {fieldLabel(field)}
          <Required on={required} />
        </legend>
        <div className={field._inline ? 'flex flex-wrap gap-x-6 gap-y-2' : 'space-y-2'}>
          {Object.entries(field._options).map(([optionValue, optionLabel]) => (
            <label key={optionValue} className="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <input
                type="radio"
                name={id}
                value={optionValue}
                checked={String(value ?? '') === optionValue}
                disabled={disabled}
                aria-invalid={invalid || undefined}
                onChange={() => onChange(optionValue)}
                className="mks-radio"
              />
              {optionLabel}
            </label>
          ))}
        </div>
        {help}
        <Errors id={errorId} errors={errors} />
      </fieldset>
    )
  }

  const cls = inputClass + (invalid ? ' ring-danger-600 dark:ring-danger-400' : '')
  let control: ReactNode
  if (field instanceof Textarea) {
    control = (
      <textarea id={id} rows={field._rows} className={cls} value={String(value ?? '')} {...a11y} onChange={(e) => onChange(e.target.value)} />
    )
  } else if (field instanceof MarkdownEditor) {
    control = <MarkdownEditorControl id={id} field={field} value={value} onChange={onChange} disabled={disabled} invalid={invalid} a11y={a11y} />
  } else if (field instanceof TagsInput) {
    control = <TagsInputControl id={id} field={field} value={value} onChange={onChange} disabled={disabled} invalid={invalid} a11y={a11y} />
  } else if (field instanceof KeyValue) {
    control = <KeyValueControl id={id} field={field} value={value} onChange={onChange} disabled={disabled} invalid={invalid} a11y={a11y} />
  } else if ((field instanceof Repeater || field instanceof Builder) && ctx) {
    const props = { id, value, onChange, disabled, errors: nestedErrors ?? {}, ctx, name: field.name, idPrefix: `${idPrefix}${field.name}-` }
    control = field instanceof Repeater ? <RepeaterControl field={field} {...props} /> : <BuilderControl field={field} {...props} />
  } else if (field instanceof RichEditor) {
    control = (
      <Suspense fallback={<div className={inputClass + ' min-h-48 animate-pulse'} />}>
        <RichEditorControl
          labelId={`${id}-label`}
          value={value}
          onChange={onChange}
          disabled={disabled}
          invalid={invalid}
          describedBy={describedBy}
          placeholder={field._placeholder}
          height={field._height}
        />
      </Suspense>
    )
  } else if (field instanceof FileUpload) {
    control = (
      <FileUploadControl
        id={id}
        field={field}
        value={value}
        onChange={onChange}
        disabled={disabled}
        invalid={invalid}
        a11y={{ 'aria-invalid': a11y['aria-invalid'], 'aria-describedby': a11y['aria-describedby'], 'aria-required': a11y['aria-required'] }}
      />
    )
  } else if (field instanceof DatePicker) {
    control = (
      <DatePickerControl
        id={id}
        field={field}
        value={value}
        onChange={onChange}
        disabled={disabled}
        invalid={invalid}
        a11y={{ 'aria-invalid': a11y['aria-invalid'], 'aria-describedby': a11y['aria-describedby'], 'aria-required': a11y['aria-required'] }}
      />
    )
  } else if (field instanceof CustomField) {
    // A plugin's field: its own control inside the panel's label, help and errors.
    const Control = field.component
    control = (
      <Control
        id={id}
        value={value}
        onChange={onChange}
        disabled={disabled}
        invalid={invalid}
        options={field._options}
        record={ctx?.record}
        errors={nestedErrors}
        get={ctx ? (name: string) => ctx.get(name as never) : undefined}
        a11y={{ 'aria-invalid': a11y['aria-invalid'], 'aria-describedby': a11y['aria-describedby'], 'aria-required': a11y['aria-required'] }}
      />
    )
  } else if (field instanceof Select) {
    control = <SelectControl id={id} field={field} className={cls} value={value} a11y={a11y} invalid={invalid} onChange={onChange} />
  } else if (field instanceof TextInput) {
    control = (
      <input
        id={id}
        type={field._type}
        dir={field._dir}
        maxLength={field._maxLength}
        placeholder={field._placeholder}
        autoComplete={field._type === 'password' ? 'new-password' : undefined}
        className={cls}
        value={String(value ?? '')}
        {...a11y}
        onChange={(e) => onChange(field._type === 'number' && e.target.value !== '' ? Number(e.target.value) : e.target.value)}
      />
    )
  } else {
    control = <p className="text-sm text-danger-600">Unknown field kind: {field.kind}</p>
  }

  // "mks-field" and "mks-label": the look can put labels beside fields or float them.
  return (
    <div className="mks-field space-y-2">
      <label id={`${id}-label`} htmlFor={id} className="mks-label block text-sm font-medium text-gray-950 dark:text-white">
        {fieldLabel(field)}
        <Required on={required} />
      </label>
      {control}
      {help}
      <Errors id={errorId} errors={errors} />
    </div>
  )
}

interface Option {
  value: unknown
  label: string
}

function SelectControl({
  id,
  field,
  className,
  value,
  a11y,
  invalid,
  onChange,
}: {
  id: string
  field: Select<any>
  className: string
  value: unknown
  a11y: Record<string, unknown>
  invalid: boolean
  onChange: (value: unknown) => void
}) {
  const { api } = usePanel()
  const formResource = useContext(FormResourceContext)
  // Server options: a relation of the form's resource, or the records of another resource.
  const resource = field._optionsFrom ?? formResource
  const rel = field._optionsFrom ? undefined : field._relationship
  const remote = !!field._optionsFrom || !!rel
  const useCombobox = remote && (field._searchable || field._multiple)

  // Short lists: the first page of options, in a native select.
  const related = useQuery({
    queryKey: ['options', resource, rel, { search: '' }],
    queryFn: () => api.options(resource, rel),
    enabled: remote && !useCombobox,
    staleTime: 60_000,
  })

  if (useCombobox) {
    return (
      <Combobox
        id={id}
        resource={resource}
        relation={rel}
        multiple={field._multiple}
        createIn={field._createIn}
        value={value}
        onChange={onChange}
        disabled={a11y.disabled as boolean | undefined}
        invalid={invalid}
        a11y={{ 'aria-invalid': a11y['aria-invalid'], 'aria-describedby': a11y['aria-describedby'], 'aria-required': a11y['aria-required'] }}
      />
    )
  }

  const options: Option[] = remote
    ? (related.data?.data ?? []).map((o) => ({ value: o.value, label: o.label }))
    : Object.entries(field._options).map(([value, label]) => ({ value, label }))

  return (
    <select
      id={id}
      className={className}
      value={value == null ? '' : String(value)}
      {...a11y}
      // Keep the option's original type (e.g. numeric ids) instead of the DOM's string.
      onChange={(e) => onChange(options.find((o) => String(o.value) === e.target.value)?.value ?? null)}
    >
      <option value="">{related.isLoading ? t('common.loading') : t('common.select')}</option>
      {options.map((o) => (
        <option key={String(o.value)} value={String(o.value)}>
          {o.label}
        </option>
      ))}
    </select>
  )
}
