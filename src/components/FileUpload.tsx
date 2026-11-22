import { useContext, useEffect, useRef, useState, type DragEvent } from 'react'
import { ApiError } from '../api'
import type { FileUpload } from '../form'
import { FormResourceContext, RecordKeyContext } from './formContext'
import { CloseIcon } from './icons'
import { usePanel } from './Panel'
import { numberFormat, t } from '../i18n'

const fa = numberFormat({ maximumFractionDigits: 1 })

export function formatBytes(n: number): string {
  if (n >= 1 << 20) return t('upload.mb', { n: fa.format(n / (1 << 20)) })
  return t('upload.kb', { n: fa.format(Math.ceil(n / 1024)) })
}

interface Props {
  id: string
  field: FileUpload<any>
  value: unknown
  onChange: (value: unknown) => void
  disabled?: boolean
  invalid?: boolean
  a11y: Record<string, unknown>
}

export function FileUploadControl(props: Props) {
  return props.field._multiple ? <MultiFileUpload {...props} /> : <SingleFileUpload {...props} />
}

function SingleFileUpload({ id, field, value, onChange, disabled, invalid, a11y }: Props) {
  const { api } = usePanel()
  const resource = useContext(FormResourceContext)
  const recordKey = useContext(RecordKeyContext)
  const input = useRef<HTMLInputElement>(null)
  const [progress, setProgress] = useState<number | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [local, setLocal] = useState<{ url: string; name: string } | null>(null)
  const [dragging, setDragging] = useState(false)

  // Object URLs of picked files are freed when replaced or unmounted.
  useEffect(() => () => void (local && URL.revokeObjectURL(local.url)), [local])

  const stored = typeof value === 'string' && value !== '' && !value.startsWith('upl_') ? value : null
  const preview = local?.url ?? (stored ? field._url(stored) : null)
  const name = local?.name ?? (stored ? stored.split('/').pop() : null)

  async function send(file: File) {
    setError(null)
    if (field._maxSize && file.size > field._maxSize) {
      setError(t('upload.tooBig', { max: formatBytes(field._maxSize) }))
      return
    }
    setProgress(0)
    try {
      const res = await api.upload(resource, field.name, file, setProgress, recordKey)
      setLocal({ url: URL.createObjectURL(file), name: file.name })
      onChange(res.token)
    } catch (err) {
      if (err instanceof ApiError) setError(err.errors[field.name]?.[0] ?? err.message)
      else setError(err instanceof Error ? err.message : String(err))
    } finally {
      setProgress(null)
      if (input.current) input.current.value = ''
    }
  }

  function onDrop(e: DragEvent) {
    e.preventDefault()
    setDragging(false)
    const file = e.dataTransfer.files[0]
    if (file && !disabled) void send(file)
  }

  const picker = (
    <input
      ref={input}
      id={id}
      type="file"
      accept={field._accept}
      disabled={disabled || progress !== null}
      className="sr-only"
      onChange={(e) => {
        const file = e.target.files?.[0]
        if (file) void send(file)
      }}
      {...a11y}
    />
  )

  return (
    <div className="space-y-2">
      {preview || name ? (
        <div className="flex items-center gap-4 rounded-lg p-3 ring-1 ring-gray-950/10 dark:ring-white/20">
          {field._image && preview && (
            <img src={preview} alt="" className="size-20 rounded-md object-cover ring-1 ring-gray-950/5 dark:ring-white/10" />
          )}
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm text-gray-700 dark:text-gray-300" dir="ltr">
              {name}
            </p>
            {local && <p className="text-xs text-gray-500">{t('upload.keptAfterSave')}</p>}
          </div>
          {!disabled && (
            <div className="flex gap-3 text-sm font-semibold">
              <label htmlFor={id} className="cursor-pointer text-primary-600 hover:text-primary-500 dark:text-primary-400">
                {t('upload.change')}
              </label>
              <button
                type="button"
                onClick={() => {
                  setLocal(null)
                  onChange(null)
                }}
                className="text-danger-600 hover:text-danger-500 dark:text-danger-400"
              >
                {t('common.delete')}
              </button>
            </div>
          )}
          {picker}
        </div>
      ) : (
        <label
          htmlFor={id}
          onDragOver={(e) => {
            e.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={onDrop}
          className={
            'flex cursor-pointer flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed px-4 py-6 text-center text-sm ' +
            (dragging
              ? 'border-primary-600 bg-primary-50 dark:bg-primary-400/10'
              : invalid
                ? 'border-danger-600'
                : 'border-gray-300 hover:border-gray-400 dark:border-white/20') +
            (disabled ? ' cursor-not-allowed opacity-60' : '')
          }
        >
          <span className="font-medium text-primary-600 dark:text-primary-400">{t('upload.choose')}</span>
          <span className="text-gray-500 dark:text-gray-400">{t('upload.drop')}</span>
          {field._maxSize && <span className="text-xs text-gray-400">{t('upload.max', { max: formatBytes(field._maxSize) })}</span>}
          {picker}
        </label>
      )}

      {progress !== null && (
        <div
          role="progressbar"
          aria-label={t('upload.progress')}
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={Math.round(progress * 100)}
          className="h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-white/10"
        >
          <div className="h-full bg-primary-600 transition-[width]" style={{ width: `${progress * 100}%` }} />
        </div>
      )}
      {error && (
        <p role="alert" className="text-sm text-danger-600 dark:text-danger-400">
          {error}
        </p>
      )}
    </div>
  )
}

interface Pending {
  name: string
  progress: number
}

/** A list of files: thumbnails (or names) of saved and new files, and a drop zone to add. */
function MultiFileUpload({ id, field, value, onChange, disabled, invalid, a11y }: Props) {
  const { api } = usePanel()
  const resource = useContext(FormResourceContext)
  const recordKey = useContext(RecordKeyContext)
  const items = Array.isArray(value) ? (value as string[]) : []
  // The latest list, for uploads that finish after other changes.
  const latest = useRef(items)
  latest.current = items
  const [previews, setPreviews] = useState<Record<string, { url: string; name: string }>>({})
  const [pending, setPending] = useState<Pending | null>(null)
  const [errors, setErrors] = useState<string[]>([])
  const [dragging, setDragging] = useState(false)
  const input = useRef<HTMLInputElement>(null)
  const previewsRef = useRef(previews)
  previewsRef.current = previews
  useEffect(() => () => Object.values(previewsRef.current).forEach((p) => URL.revokeObjectURL(p.url)), [])

  const room = field._maxFiles === undefined ? Infinity : field._maxFiles - items.length

  async function send(files: File[]) {
    const problems: string[] = []
    if (files.length > room) {
      problems.push(t('upload.tooMany', { count: field._maxFiles! }))
      files = files.slice(0, Math.max(0, room))
    }
    // One at a time, so the progress bar means something and the server is not flooded.
    for (const file of files) {
      if (field._maxSize && file.size > field._maxSize) {
        problems.push(t('upload.fileTooBig', { name: file.name, max: formatBytes(field._maxSize) }))
        continue
      }
      setPending({ name: file.name, progress: 0 })
      try {
        const res = await api.upload(resource, field.name, file, (p) => setPending({ name: file.name, progress: p }), recordKey)
        setPreviews((all) => ({ ...all, [res.token]: { url: URL.createObjectURL(file), name: file.name } }))
        latest.current = [...latest.current, res.token]
        onChange(latest.current)
      } catch (err) {
        const msg = err instanceof ApiError ? (err.errors[field.name]?.[0] ?? err.message) : String(err)
        problems.push(`${file.name}: ${msg}`)
      }
    }
    setPending(null)
    setErrors(problems)
    if (input.current) input.current.value = ''
  }

  const urlOf = (item: string) => previews[item]?.url ?? (item.startsWith('upl_') ? null : field._url(item))
  const nameOf = (item: string) => previews[item]?.name ?? item.split('/').pop() ?? item

  return (
    <div className="space-y-2">
      {items.length > 0 && (
        <ul className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
          {items.map((item) => (
            <li key={item} className="group relative overflow-hidden rounded-lg ring-1 ring-gray-950/10 dark:ring-white/20">
              {field._image && urlOf(item) ? (
                <img src={urlOf(item)!} alt={nameOf(item)} className="aspect-video w-full object-cover" />
              ) : (
                <p className="truncate p-3 text-sm" dir="ltr">
                  {nameOf(item)}
                </p>
              )}
              {!disabled && (
                <button
                  type="button"
                  onClick={() => onChange(items.filter((i) => i !== item))}
                  aria-label={t('common.deleteNamed', { name: nameOf(item) })}
                  className="absolute end-1 top-1 rounded-full bg-white/90 p-1 text-danger-600 shadow ring-1 ring-gray-950/10 hover:bg-white [&_svg]:size-4 dark:bg-gray-900/90"
                >
                  <CloseIcon />
                </button>
              )}
            </li>
          ))}
        </ul>
      )}

      {room > 0 && !disabled && (
        <label
          htmlFor={id}
          onDragOver={(e) => {
            e.preventDefault()
            setDragging(true)
          }}
          onDragLeave={() => setDragging(false)}
          onDrop={(e) => {
            e.preventDefault()
            setDragging(false)
            if (pending === null) void send([...e.dataTransfer.files])
          }}
          className={
            'flex cursor-pointer flex-col items-center justify-center gap-1 rounded-lg border-2 border-dashed px-4 py-5 text-center text-sm ' +
            (dragging
              ? 'border-primary-600 bg-primary-50 dark:bg-primary-400/10'
              : invalid
                ? 'border-danger-600'
                : 'border-gray-300 hover:border-gray-400 dark:border-white/20')
          }
        >
          <span className="font-medium text-primary-600 dark:text-primary-400">{t('upload.add')}</span>
          <span className="text-gray-500 dark:text-gray-400">
            {t('upload.dropMany')}
            {field._maxFiles !== undefined && t('upload.count', { count: items.length, max: field._maxFiles })}
          </span>
          <input
            ref={input}
            id={id}
            type="file"
            multiple
            accept={field._accept}
            disabled={pending !== null}
            className="sr-only"
            onChange={(e) => void send([...(e.target.files ?? [])])}
            {...a11y}
          />
        </label>
      )}

      {pending && (
        <div className="space-y-1">
          <p className="text-xs text-gray-500" dir="auto">
            {pending.name}
          </p>
          <div
            role="progressbar"
            aria-label={t('upload.uploading', { name: pending.name })}
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(pending.progress * 100)}
            className="h-1.5 overflow-hidden rounded-full bg-gray-200 dark:bg-white/10"
          >
            <div className="h-full bg-primary-600 transition-[width]" style={{ width: `${pending.progress * 100}%` }} />
          </div>
        </div>
      )}
      {errors.length > 0 && (
        <div role="alert" className="space-y-1 text-sm text-danger-600 dark:text-danger-400">
          {errors.map((e) => (
            <p key={e}>{e}</p>
          ))}
        </div>
      )}
    </div>
  )
}
