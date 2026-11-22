import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import type { KeyValue, TagsInput } from '../form'
import { inputClass } from './fields'
import { CloseIcon } from './icons'
import { numberFormat, t } from '../i18n'

interface Base {
  id: string
  value: unknown
  onChange: (value: unknown) => void
  disabled?: boolean
  invalid?: boolean
  a11y: Record<string, unknown>
}

export function TagsInputControl({ id, field, value, onChange, disabled, invalid, a11y }: Base & { field: TagsInput<any> }) {
  const tags = Array.isArray(value) ? (value as string[]) : []
  const [draft, setDraft] = useState('')

  function add(text: string) {
    const parts = text
      .split(/[,،]/)
      .map((t) => t.trim())
      .filter((t) => t && !tags.includes(t))
    if (parts.length) onChange([...tags, ...parts])
    setDraft('')
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' || e.key === ',' || e.key === '،') {
      e.preventDefault() // Enter must not submit the form
      add(draft)
    } else if (e.key === 'Backspace' && draft === '' && tags.length > 0) {
      onChange(tags.slice(0, -1))
    }
  }

  return (
    <div
      className={
        'flex min-h-9 w-full flex-wrap items-center gap-1 rounded-lg bg-white px-2 py-1 text-sm shadow-sm ring-1 focus-within:ring-2 focus-within:ring-primary-600 dark:bg-white/5 ' +
        (invalid ? 'ring-danger-600 dark:ring-danger-400' : 'ring-gray-950/10 dark:ring-white/20')
      }
    >
      {tags.map((tag) => (
        <span key={tag} className="inline-flex items-center gap-1 rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium dark:bg-white/10">
          {tag}
          {!disabled && (
            <button type="button" onClick={() => onChange(tags.filter((t) => t !== tag))} aria-label={t('common.deleteNamed', { name: tag })} className="[&_svg]:size-3.5">
              <CloseIcon />
            </button>
          )}
        </span>
      ))}
      <input
        id={id}
        value={draft}
        disabled={disabled}
        placeholder={tags.length === 0 ? field._placeholder : undefined}
        onChange={(e) => setDraft(e.target.value)}
        onKeyDown={onKeyDown}
        onBlur={() => draft.trim() && add(draft)}
        {...a11y}
        className="min-w-24 flex-1 border-0 bg-transparent px-1 py-0.5 text-gray-950 outline-none placeholder:text-gray-400 dark:text-white"
      />
    </div>
  )
}

type Row = { id: number; key: string; value: string }

const fa = numberFormat()

export function KeyValueControl({ id, field, value, onChange, disabled, a11y }: Base & { field: KeyValue<any> }) {
  const nextId = useRef(0)
  const toRows = (v: unknown): Row[] =>
    Object.entries(v && typeof v === 'object' ? (v as Record<string, unknown>) : {}).map(([k, val]) => ({
      id: nextId.current++,
      key: k,
      value: String(val ?? ''),
    }))
  // Rows are kept locally so half-typed or empty keys survive while editing; the value
  // only gets rows with a key.
  const [rows, setRows] = useState<Row[]>(() => toRows(value))
  const lastEmitted = useRef<string>(JSON.stringify(value ?? {}))

  // A value set from outside (e.g. a reset) replaces the rows.
  useEffect(() => {
    const incoming = JSON.stringify(value ?? {})
    if (incoming !== lastEmitted.current) {
      lastEmitted.current = incoming
      setRows(toRows(value))
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value])

  function update(next: Row[]) {
    setRows(next)
    const obj: Record<string, string> = {}
    for (const r of next) if (r.key.trim()) obj[r.key.trim()] = r.value
    lastEmitted.current = JSON.stringify(obj)
    onChange(obj)
  }

  const describedBy = a11y['aria-describedby'] as string | undefined
  return (
    <div className="space-y-2" id={id} role="group" aria-describedby={describedBy}>
      {rows.length > 0 && (
        <div className="grid grid-cols-[1fr_1fr_auto] gap-2 text-xs font-medium text-gray-500 dark:text-gray-400">
          <span>{field._keyLabel}</span>
          <span>{field._valueLabel}</span>
          <span className="w-6" />
        </div>
      )}
      {rows.map((row, i) => (
        <div key={row.id} className="grid grid-cols-[1fr_1fr_auto] items-center gap-2">
          <input
            aria-label={`${field._keyLabel} ${fa.format(i + 1)}`}
            value={row.key}
            disabled={disabled}
            dir="auto"
            onChange={(e) => update(rows.map((r) => (r.id === row.id ? { ...r, key: e.target.value } : r)))}
            className={inputClass}
          />
          <input
            aria-label={`${field._valueLabel} ${fa.format(i + 1)}`}
            value={row.value}
            disabled={disabled}
            dir="auto"
            onChange={(e) => update(rows.map((r) => (r.id === row.id ? { ...r, value: e.target.value } : r)))}
            className={inputClass}
          />
          <button
            type="button"
            disabled={disabled}
            onClick={() => update(rows.filter((r) => r.id !== row.id))}
            aria-label={t('keyValue.removeRow', { n: i + 1 })}
            className="text-gray-400 hover:text-danger-600 [&_svg]:size-4"
          >
            <CloseIcon />
          </button>
        </div>
      ))}
      {!disabled && (
        <button
          type="button"
          onClick={() => setRows([...rows, { id: nextId.current++, key: '', value: '' }])}
          className="text-sm font-semibold text-primary-600 hover:text-primary-500 dark:text-primary-400"
        >
          + {field._addLabel}
        </button>
      )}
    </div>
  )
}
