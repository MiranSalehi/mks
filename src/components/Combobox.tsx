import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useId, useMemo, useRef, useState, type KeyboardEvent } from 'react'
import type { RecordKey, RelationOption } from '../protocol'
import { CloseIcon } from './icons'
import { usePanel } from './Panel'
import { t } from '../i18n'
import { ApiError } from '../api'

interface ComboboxProps {
  id: string
  resource: string
  /** A relation of resource; without one, the options are resource's own records. */
  relation?: string
  multiple: boolean
  /** Create a missing option: a record of resource with field set to the search text. */
  createIn?: { resource: string; field: string }
  value: unknown
  onChange: (value: unknown) => void
  disabled?: boolean
  invalid?: boolean
  a11y: Record<string, unknown>
}

const same = (a: unknown, b: unknown) => String(a) === String(b)

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = window.setTimeout(() => setV(value), ms)
    return () => window.clearTimeout(t)
  }, [value, ms])
  return v
}

/**
 * A searchable select for relations, or for the records of a resource (ARIA 1.2 combobox
 * with a listbox). Options are searched on the server; labels of the current value are
 * fetched by key.
 */
export function Combobox({ id, resource, relation, multiple, createIn, value, onChange, disabled, invalid, a11y }: ComboboxProps) {
  const { api, meta } = usePanel()
  const queryClient = useQueryClient()
  const [creating, setCreating] = useState(false)
  const [createError, setCreateError] = useState('')
  const listId = useId()
  const root = useRef<HTMLDivElement>(null)
  const input = useRef<HTMLInputElement>(null)
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [active, setActive] = useState(0)
  const search = useDebounced(query.trim(), 250)

  const selected: RecordKey[] = useMemo(() => {
    if (multiple) return Array.isArray(value) ? (value as RecordKey[]) : []
    return value === null || value === undefined || value === '' ? [] : [value as RecordKey]
  }, [value, multiple])

  const results = useQuery({
    queryKey: ['options', resource, relation, { search }],
    queryFn: () => api.options(resource, relation, { search: search || undefined }),
    enabled: open,
    placeholderData: keepPreviousData,
    staleTime: 30_000,
  })

  // Labels for the current value, which may not be in the search results.
  const labels = useQuery({
    queryKey: ['options', resource, relation, { values: selected.map(String).sort() }],
    queryFn: () => api.options(resource, relation, { values: selected }),
    enabled: selected.length > 0,
    staleTime: 60_000,
  })
  const labelOf = (key: RecordKey) =>
    labels.data?.data.find((o) => same(o.value, key))?.label ??
    results.data?.data.find((o) => same(o.value, key))?.label ??
    String(key)

  const options = results.data?.data ?? []
  useEffect(() => {
    setActive(0)
    setCreateError('')
  }, [search])

  // "Create …" as a last row when the text matches no option exactly (and the user may).
  const canCreate =
    !!createIn &&
    !!meta.resources[createIn.resource]?.abilities.create &&
    search !== '' &&
    search === query.trim() &&
    !!results.data &&
    !options.some((o) => o.label.trim().toLowerCase() === search.toLowerCase())
  const rows = options.length + (canCreate ? 1 : 0)

  async function create() {
    if (!createIn || creating) return
    setCreating(true)
    setCreateError('')
    try {
      const res = await api.create(createIn.resource, { [createIn.field]: search })
      const key = (res.data as Record<string, unknown>)[meta.resources[createIn.resource]?.key ?? 'id'] as RecordKey
      await queryClient.invalidateQueries({ predicate: (q) => q.queryKey[0] === 'options' || (q.queryKey[0] === 'records' && q.queryKey[1] === createIn.resource) })
      choose({ value: key, label: search })
    } catch (err) {
      const msg = err instanceof ApiError ? (Object.values(err.errors)[0]?.[0] ?? err.message) : String(err)
      setCreateError(msg)
    } finally {
      setCreating(false)
    }
  }

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => root.current && !root.current.contains(e.target as Node) && close()
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  function close() {
    setOpen(false)
    setQuery('')
  }

  function choose(option: RelationOption) {
    if (multiple) {
      const has = selected.some((k) => same(k, option.value))
      onChange(has ? selected.filter((k) => !same(k, option.value)) : [...selected, option.value])
      setQuery('')
      input.current?.focus()
    } else {
      onChange(option.value)
      close()
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      if (!open) {
        setOpen(true)
        return
      }
      const step = e.key === 'ArrowDown' ? 1 : -1
      setActive((i) => (rows ? (i + step + rows) % rows : 0))
    } else if (e.key === 'Enter') {
      if (open && options[active]) {
        e.preventDefault()
        choose(options[active])
      } else if (open && canCreate && active === options.length) {
        e.preventDefault()
        void create()
      }
    } else if (e.key === 'Escape') {
      if (open) {
        e.preventDefault()
        e.stopPropagation() // inside a modal, close the list, not the modal
        close()
      }
    } else if (e.key === 'Backspace' && multiple && query === '' && selected.length > 0) {
      onChange(selected.slice(0, -1))
    }
  }

  const activeId = open && active < rows ? `${listId}-${active}` : undefined
  const singleLabel = !multiple && selected.length === 1 ? labelOf(selected[0]) : ''

  return (
    <div ref={root} className="relative">
      <div
        className={
          'mks-input flex min-h-9 w-full flex-wrap items-center gap-1 rounded-lg bg-white px-2 py-1 text-sm shadow-sm ring-1 ' +
          'focus-within:ring-2 focus-within:ring-primary-600 dark:bg-white/5 ' +
          (invalid ? 'ring-danger-600 dark:ring-danger-400' : 'ring-gray-950/10 dark:ring-white/20') +
          (disabled ? ' cursor-not-allowed opacity-60' : '')
        }
        onClick={() => !disabled && input.current?.focus()}
      >
        {multiple &&
          selected.map((key) => (
            <span key={String(key)} className="inline-flex items-center gap-1 rounded-md bg-primary-50 px-2 py-0.5 text-xs font-medium text-primary-600 dark:bg-primary-400/10 dark:text-primary-400">
              {labelOf(key)}
              <button
                type="button"
                disabled={disabled}
                onClick={(e) => {
                  e.stopPropagation()
                  onChange(selected.filter((k) => !same(k, key)))
                }}
                aria-label={t('common.deleteNamed', { name: labelOf(key) })}
                className="rounded hover:text-primary-500 [&_svg]:size-3.5"
              >
                <CloseIcon />
              </button>
            </span>
          ))}
        <input
          ref={input}
          id={id}
          role="combobox"
          aria-expanded={open}
          aria-controls={listId}
          aria-autocomplete="list"
          aria-activedescendant={activeId}
          autoComplete="off"
          disabled={disabled}
          value={open ? query : singleLabel}
          placeholder={selected.length === 0 || multiple ? t('common.searchPlaceholder') : undefined}
          onFocus={() => setOpen(true)}
          onChange={(e) => {
            setQuery(e.target.value)
            setOpen(true)
          }}
          onKeyDown={onKeyDown}
          {...a11y}
          className="min-w-24 flex-1 border-0 bg-transparent px-1 py-0.5 text-gray-950 outline-none placeholder:text-gray-400 dark:text-white"
        />
        {!multiple && selected.length > 0 && !disabled && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onChange(null)
            }}
            aria-label={t('combobox.clear')}
            className="text-gray-400 hover:text-gray-600 [&_svg]:size-4"
          >
            <CloseIcon />
          </button>
        )}
      </div>

      {open && (
        <ul
          id={listId}
          role="listbox"
          aria-multiselectable={multiple || undefined}
          className="absolute z-30 mt-1.5 max-h-64 w-full origin-top animate-pop overflow-auto rounded-xl bg-white p-1 text-sm shadow-lg ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10"
        >
          {results.isPending && <li className="px-3 py-2 text-gray-500">{t('combobox.searching')}</li>}
          {results.data && options.length === 0 && !canCreate && <li className="px-3 py-2 text-gray-500">{t('common.nothingFound')}</li>}
          {options.map((option, i) => {
            const isSelected = selected.some((k) => same(k, option.value))
            return (
              <li
                key={String(option.value)}
                id={`${listId}-${i}`}
                role="option"
                aria-selected={isSelected}
                onMouseDown={(e) => e.preventDefault()} // keep focus in the input
                onClick={() => choose(option)}
                onMouseEnter={() => setActive(i)}
                className={
                  'flex cursor-pointer items-center justify-between rounded-md px-3 py-2 ' +
                  (i === active ? 'bg-gray-100 dark:bg-white/10' : '') +
                  (isSelected ? ' font-medium text-primary-600 dark:text-primary-400' : '')
                }
              >
                {option.label}
                {isSelected && <span aria-hidden="true">✓</span>}
              </li>
            )
          })}
          {canCreate && (
            <li
              id={`${listId}-${options.length}`}
              role="option"
              aria-selected={false}
              aria-disabled={creating || undefined}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => void create()}
              onMouseEnter={() => setActive(options.length)}
              className={'flex cursor-pointer items-center gap-2 rounded-md px-3 py-2 text-primary-600 dark:text-primary-400 ' + (active === options.length ? 'bg-gray-100 dark:bg-white/10' : '')}
            >
              <span aria-hidden="true">+</span>
              {creating ? t('combobox.creating') : t('combobox.create', { label: search })}
            </li>
          )}
          {createError && (
            <li role="alert" className="px-3 py-2 text-xs text-danger-600 dark:text-danger-400">
              {createError}
            </li>
          )}
          {results.data?.meta.has_more && (
            <li className="px-3 py-2 text-xs text-gray-500">{t('combobox.more')}</li>
          )}
        </ul>
      )}
    </div>
  )
}
