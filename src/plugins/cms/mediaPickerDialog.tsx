import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { createPortal, defaultFileUrl, direction, formatNumber, locale, tr, usePanel } from '../../index'
import type { Media } from './types'

// MKSine's media library modal (media-picker-modal.blade.php): search and type filter, a
// drop zone, the grid, an attachment-details column and a footer that confirms the choice.

export type MediaAccept = 'image' | 'any'

/** The types the library takes (the server checks the content too). */
export const mediaAccept = 'image/jpeg,image/png,image/webp,image/gif,application/pdf,video/mp4,video/webm,audio/mpeg,audio/wav,audio/ogg'

const families = {
  image: ['image/jpeg', 'image/png', 'image/webp', 'image/gif'],
  video: ['video/mp4', 'video/webm'],
  audio: ['audio/mpeg', 'audio/wave', 'audio/wav', 'audio/ogg', 'application/ogg'],
  pdf: ['application/pdf'],
  document: ['application/pdf', 'application/ogg'],
}
type TypeFilter = '' | keyof typeof families

const L = (fa: string, en: string) => tr({ fa, en })

export function humanSize(bytes: unknown): string {
  const n = Number(bytes) || 0
  if (n >= 1 << 20) return `${formatNumber(Math.round((n / (1 << 20)) * 10) / 10)} MB`
  return `${formatNumber(Math.max(1, Math.round(n / 1024)))} KB`
}

export type MediaKind = 'image' | 'video' | 'audio' | 'document'

export function kindOf(mime: string): MediaKind {
  if (mime.startsWith('image/')) return 'image'
  if (mime.startsWith('video/')) return 'video'
  if (mime.startsWith('audio/') || mime === 'application/ogg') return 'audio'
  return 'document'
}

/** The kind of a stored path, by its extension (a field's value is only the path). */
export function kindOfPath(path: string): MediaKind {
  const ext = path.split('?')[0].split('.').pop()?.toLowerCase() ?? ''
  if (['jpg', 'jpeg', 'png', 'webp', 'gif'].includes(ext)) return 'image'
  if (['mp4', 'webm'].includes(ext)) return 'video'
  if (['mp3', 'wav', 'ogg'].includes(ext)) return 'audio'
  return 'document'
}

export const mediaName = (m: Media) => m.title || m.file_name.replace(/\.[^.]+$/, '')

// Heroicons (outline, 24px) as MKSine uses them.
const paths = {
  photo:
    'm2.25 15.75 5.159-5.159a2.25 2.25 0 0 1 3.182 0l5.159 5.159m-1.5-1.5 1.409-1.409a2.25 2.25 0 0 1 3.182 0l2.909 2.909m-18 3.75h16.5a1.5 1.5 0 0 0 1.5-1.5V6a1.5 1.5 0 0 0-1.5-1.5H3.75A1.5 1.5 0 0 0 2.25 6v12a1.5 1.5 0 0 0 1.5 1.5Zm10.5-11.25h.008v.008h-.008V8.25Zm.375 0a.375.375 0 1 1-.75 0 .375.375 0 0 1 .75 0Z',
  x: 'M6 18 18 6M6 6l12 12',
  search: 'm21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z',
  chevronDown: 'm19.5 8.25-7.5 7.5-7.5-7.5',
  chevronLeft: 'M15.75 19.5 8.25 12l7.5-7.5',
  chevronRight: 'm8.25 4.5 7.5 7.5-7.5 7.5',
  cloudUp:
    'M12 16.5V9.75m0 0 3 3m-3-3-3 3M6.75 19.5a4.5 4.5 0 0 1-1.41-8.775 5.25 5.25 0 0 1 10.233-2.33 3 3 0 0 1 3.758 3.848A3.752 3.752 0 0 1 18 19.5H6.75Z',
  check: 'm4.5 12.75 6 6 9-13.5',
  checkCircle: 'M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z',
  document:
    'M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z',
  music:
    'm9 9 10.5-3m0 6.553v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 1 1-.99-3.467l2.31-.66a2.25 2.25 0 0 0 1.632-2.163Zm0 0V2.25L9 5.25v10.303m0 0v3.75a2.25 2.25 0 0 1-1.632 2.163l-1.32.377a1.803 1.803 0 0 1-.99-3.467l2.31-.66A2.25 2.25 0 0 0 9 15.553Z',
  play: 'M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 0 1 0 1.972l-11.54 6.347a1.125 1.125 0 0 1-1.667-.986V5.653Z',
}

export function HeroIcon({ name, className = 'size-5', strokeWidth = 1.5, solid }: { name: keyof typeof paths; className?: string; strokeWidth?: number; solid?: boolean }) {
  return (
    <svg viewBox="0 0 24 24" fill={solid ? 'currentColor' : 'none'} stroke="currentColor" strokeWidth={strokeWidth} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
      <path d={paths[name]} />
    </svg>
  )
}

const Spinner = ({ className = 'size-4' }: { className?: string }) => (
  <svg className={'animate-spin ' + className} xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" aria-hidden="true">
    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
  </svg>
)

/** A file's thumbnail (grid, the field's tiles) or its large preview (details column). */
export function MediaPreview({ url, kind, name, variant }: { url: string; kind: MediaKind; name: string; variant: 'grid' | 'tile' | 'detail' }) {
  const small = variant !== 'detail'
  if (kind === 'image') {
    return small ? (
      <img src={url} alt={name} loading="lazy" className={'pointer-events-none h-full w-full object-cover' + (variant === 'grid' ? ' transition-transform duration-200 group-hover:scale-105' : '')} />
    ) : (
      <img src={url} alt={name} className="aspect-video w-full bg-gray-50 object-contain dark:bg-gray-900" />
    )
  }
  if (kind === 'video') {
    return small ? (
      <div className="relative h-full w-full bg-black">
        <video src={url} muted preload="metadata" playsInline className="pointer-events-none h-full w-full object-cover" />
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center bg-black/25">
          <HeroIcon name="play" solid className={(variant === 'grid' ? 'size-8' : 'size-6') + ' text-white/90 drop-shadow'} />
        </div>
      </div>
    ) : (
      <video src={url} className="aspect-video w-full bg-black" controls preload="metadata" playsInline />
    )
  }
  if (kind === 'audio') {
    return small ? (
      <div className="flex h-full w-full flex-col items-center justify-center gap-1 bg-gradient-to-br from-info-50 to-primary-100 px-2 dark:from-info-950 dark:to-primary-950">
        <HeroIcon name="music" className={(variant === 'grid' ? 'size-10' : 'size-8') + ' text-info-500 dark:text-info-400'} />
        {variant === 'grid' && <span className="max-w-full truncate text-[10px] font-medium text-info-800 dark:text-info-200">{name}</span>}
      </div>
    ) : (
      <div className="space-y-3 bg-gradient-to-br from-info-50 to-primary-100 p-4 dark:from-info-950 dark:to-primary-950">
        <HeroIcon name="music" className="mx-auto size-10 text-info-500 dark:text-info-400" />
        <audio src={url} className="w-full" controls preload="metadata" />
      </div>
    )
  }
  return small ? (
    <div className="flex h-full w-full flex-col items-center justify-center gap-1 bg-gradient-to-br from-gray-100 to-gray-200 px-2 dark:from-gray-700 dark:to-gray-800">
      <HeroIcon name="document" className={(variant === 'grid' ? 'size-10' : 'size-8') + ' text-gray-400 dark:text-gray-500'} />
      {variant === 'grid' && <span className="max-w-full truncate text-[10px] font-medium text-gray-600 dark:text-gray-300">{name}</span>}
    </div>
  ) : (
    <div className="flex aspect-video items-center justify-center bg-gray-100 dark:bg-gray-800">
      <HeroIcon name="document" className="size-14 text-gray-400 dark:text-gray-500" />
    </div>
  )
}

const thumbUrl = (m: Media) => defaultFileUrl(kindOf(m.mime_type) === 'image' ? (m.conversions?.['320'] ?? m.path) : m.path)

function formatDate(v: string | null): string {
  if (!v) return '—'
  const d = new Date(v)
  if (Number.isNaN(d.getTime())) return v
  return new Intl.DateTimeFormat(locale(), { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(d)
}

const dimensions = (m: Media) =>
  m.width && m.height ? L(`${formatNumber(m.width)} × ${formatNumber(m.height)} پیکسل`, `${formatNumber(m.width)} × ${formatNumber(m.height)} px`) : '—'

/** Page numbers around the current page (one each side), with gaps: 1 … 4 5 6 … 12. */
export function pageList(current: number, last: number): (number | '…')[] {
  const out: (number | '…')[] = []
  for (let p = 1; p <= last; p++) {
    if (p === 1 || p === last || Math.abs(p - current) <= 1 || (current <= 3 && p <= 4) || (current >= last - 2 && p >= last - 3)) out.push(p)
    else if (out[out.length - 1] !== '…') out.push('…')
  }
  return out
}

const PER_PAGE = 24
const CLOSE_MS = 220

export function MediaPickerDialog({
  multiple,
  accept,
  selected,
  known = [],
  onConfirm,
  onClose,
}: {
  multiple: boolean
  accept: MediaAccept
  /** The ids chosen now: they start selected. */
  selected: number[]
  /** Records already loaded for those ids, so the details column can show them at once. */
  known?: Media[]
  /** The chosen files, in the order they were picked. */
  onConfirm: (media: Media[]) => void
  onClose: () => void
}) {
  const { api } = usePanel()
  const qc = useQueryClient()
  const dialog = useRef<HTMLDialogElement>(null)
  const fileInput = useRef<HTMLInputElement>(null)
  const [shown, setShown] = useState(false)
  const closing = useRef(false)
  const [ids, setIds] = useState<number[]>(selected)
  const [focusId, setFocusId] = useState<number | null>(selected[0] ?? null)
  const [cards, setCards] = useState<Record<number, Media>>(() => Object.fromEntries(known.map((m) => [m.id, m])))
  const [typed, setTyped] = useState('')
  const [search, setSearch] = useState('')
  const [type, setType] = useState<TypeFilter>('')
  const [page, setPage] = useState(1)
  const [dragged, setDragged] = useState(false)
  const [uploading, setUploading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    dialog.current?.showModal()
    const id = requestAnimationFrame(() => setShown(true))
    return () => cancelAnimationFrame(id)
  }, [])
  useEffect(() => {
    const id = window.setTimeout(() => {
      setSearch(typed.trim())
      setPage(1)
    }, 300)
    return () => window.clearTimeout(id)
  }, [typed])

  const mimes = type ? families[type] : accept === 'image' ? families.image : []
  const list = useQuery({
    queryKey: ['records', 'cms_media', { picker: true, search, page, mimes }],
    queryFn: () =>
      api.list<Media>('cms_media', {
        search: search || undefined,
        page,
        perPage: PER_PAGE,
        sort: ['-created_at'],
        filters: mimes.length ? [{ column: 'mime_type', op: 'in' as const, value: mimes }] : [],
      }),
    placeholderData: keepPreviousData,
  })
  useEffect(() => {
    const rows = list.data?.data
    if (rows?.length) setCards((c) => ({ ...c, ...Object.fromEntries(rows.map((m) => [m.id, m])) }))
  }, [list.data])

  function close() {
    if (closing.current) return
    closing.current = true
    setShown(false)
    dialog.current?.close() // gives focus back to the field's button
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    window.setTimeout(onClose, reduced ? 0 : CLOSE_MS)
  }

  function toggle(m: Media) {
    setFocusId(m.id)
    if (multiple) setIds((cur) => (cur.includes(m.id) ? cur.filter((x) => x !== m.id) : [...cur, m.id]))
    else setIds([m.id])
  }

  async function confirm() {
    if (ids.length === 0) return
    let byId = cards
    const missing = ids.filter((id) => !byId[id])
    if (missing.length) {
      const res = await api.list<Media>('cms_media', { perPage: 100, filters: [{ column: 'id', op: 'in' as const, value: missing.map(String) }] })
      byId = { ...byId, ...Object.fromEntries(res.data.map((m) => [m.id, m])) }
    }
    onConfirm(ids.map((id) => byId[id]).filter(Boolean))
    close()
  }

  const allowed = accept === 'image' ? families.image : mediaAccept.split(',').concat(families.audio)
  async function upload(files: File[]) {
    setError(null)
    const ok = files.filter((f) => !f.type || allowed.includes(f.type))
    if (ok.length < files.length) setError(L('این نوع فایل مجاز نیست.', 'This file type is not allowed.'))
    if (ok.length === 0 || uploading) return
    setUploading(true)
    try {
      for (const file of ok) {
        const up = await api.upload('cms_media', 'path', file)
        const created = (await api.create<Media>('cms_media', { path: up.token })).data
        setCards((c) => ({ ...c, [created.id]: created }))
        setIds((cur) => (multiple ? [...cur.filter((x) => x !== created.id), created.id] : [created.id]))
        setFocusId(created.id)
      }
      setPage(1)
      await qc.invalidateQueries({ queryKey: ['records', 'cms_media'] })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setUploading(false)
    }
  }

  const items = list.data?.data ?? []
  const meta = list.data?.meta
  const total = meta?.total ?? 0
  const lastPage = meta?.last_page ?? 1
  const detail = focusId !== null ? cards[focusId] : undefined
  const typeOptions: [TypeFilter, string][] = [
    ['', L('همه انواع', 'All types')],
    ['image', L('تصاویر', 'Images')],
    ...(accept === 'any'
      ? ([
          ['video', L('ویدیوها', 'Videos')],
          ['audio', L('صوتی', 'Audio')],
          ['pdf', 'PDF'],
          ['document', L('اسناد', 'Documents')],
        ] as [TypeFilter, string][])
      : []),
  ]
  const field =
    'w-full rounded-xl border border-gray-200 bg-white text-sm transition-colors focus:border-primary-500 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-gray-600 dark:bg-gray-800 dark:text-white'
  const pageBtn =
    'inline-flex h-9 min-w-9 items-center justify-center rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 transition hover:border-primary-300 hover:bg-primary-50 hover:text-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-500/20 dark:border-gray-700 dark:bg-gray-800 dark:text-gray-300 dark:hover:bg-primary-500/10'

  return createPortal(
    <dialog
      ref={dialog}
      dir={direction()}
      aria-labelledby="media-picker-title"
      onCancel={(e) => {
        e.preventDefault()
        close()
      }}
      className="m-0 h-dvh max-h-none w-screen max-w-none overflow-hidden bg-transparent p-0 text-gray-950 backdrop:bg-transparent dark:text-white"
    >
      <div
        onClick={close}
        className={'fixed inset-0 bg-gray-900/60 backdrop-blur-sm transition-opacity dark:bg-gray-950/80 ' + (shown ? 'opacity-100 duration-300 ease-out' : 'opacity-0 duration-200 ease-in')}
      />
      <div className="pointer-events-none fixed inset-0 z-10 flex items-center justify-center p-4">
        <div
          className={
            'pointer-events-auto relative flex max-h-[min(90vh,920px)] w-full max-w-7xl flex-col overflow-hidden rounded-2xl border border-gray-200/50 bg-white shadow-2xl ring-1 ring-gray-200/50 transition ease-[cubic-bezier(0.32,0.72,0,1)] dark:border-gray-700/50 dark:bg-gray-900 dark:ring-white/10 ' +
            (shown ? 'translate-y-0 scale-100 opacity-100 duration-300' : 'translate-y-4 scale-95 opacity-0 duration-200')
          }
        >
          {/* Header */}
          <div className="flex items-center justify-between gap-4 border-b border-gray-200/80 bg-gray-50/80 px-6 py-4 rtl:flex-row-reverse dark:border-gray-700/50 dark:bg-gray-800/50">
            <div className="flex min-w-0 items-center gap-3 rtl:flex-row-reverse">
              <div className="flex size-10 items-center justify-center rounded-xl bg-primary-100 dark:bg-primary-500/20">
                <HeroIcon name="photo" className="size-5 text-primary-600 dark:text-primary-400" />
              </div>
              <div>
                <h2 id="media-picker-title" className="text-lg font-semibold text-gray-900 dark:text-white">
                  {L('کتابخانه رسانه', 'Media library')}
                </h2>
                <p className="text-sm text-gray-500 dark:text-gray-400">
                  {multiple ? L('یک یا چند رسانه انتخاب کنید', 'Choose one or more files') : L('یک رسانه انتخاب کنید', 'Choose a file')}
                </p>
              </div>
            </div>
            <button
              type="button"
              onClick={close}
              aria-label={L('بستن', 'Close')}
              className="flex size-9 items-center justify-center rounded-lg text-gray-400 transition-colors hover:bg-gray-200 hover:text-gray-600 dark:hover:bg-gray-700 dark:hover:text-gray-300"
            >
              <HeroIcon name="x" className="size-5" />
            </button>
          </div>

          {/* Body */}
          <div className="flex min-h-0 flex-1 overflow-hidden">
            <div className="flex min-h-0 min-w-0 flex-1 flex-col">
              <div className="shrink-0 space-y-4 border-b border-gray-200/80 p-5 dark:border-gray-700/50">
                <div className="flex flex-col gap-4 sm:flex-row sm:items-center">
                  <div className="relative flex-1">
                    <HeroIcon name="search" className="pointer-events-none absolute start-3 top-1/2 size-4 -translate-y-1/2 text-gray-400" />
                    <input
                      type="text"
                      value={typed}
                      onChange={(e) => setTyped(e.target.value)}
                      onKeyDown={(e) => e.key === 'Enter' && e.preventDefault()}
                      placeholder={L('جستجو با نام...', 'Search by name...')}
                      aria-label={L('جستجو با نام...', 'Search by name...')}
                      className={field + ' py-2.5 ps-10 pe-4 text-gray-900 placeholder-gray-400 dark:placeholder-gray-500'}
                    />
                  </div>
                  <div className="relative sm:w-44">
                    <select
                      value={type}
                      onChange={(e) => {
                        setType(e.target.value as TypeFilter)
                        setPage(1)
                      }}
                      aria-label={L('نوع فایل', 'File type')}
                      className={field + ' appearance-none py-2.5 ps-4 pe-10 text-gray-700 dark:text-gray-300'}
                    >
                      {typeOptions.map(([v, label]) => (
                        <option key={v} value={v}>
                          {label}
                        </option>
                      ))}
                    </select>
                    <HeroIcon name="chevronDown" className="pointer-events-none absolute end-3 top-1/2 size-4 -translate-y-1/2 text-gray-400" />
                  </div>
                </div>

                {/* Upload zone: click or drop */}
                <div
                  onDragEnter={(e) => {
                    e.preventDefault()
                    setDragged(true)
                  }}
                  onDragOver={(e) => {
                    e.preventDefault()
                    e.dataTransfer.dropEffect = 'copy'
                    setDragged(true)
                  }}
                  onDragLeave={(e) => {
                    e.preventDefault()
                    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragged(false)
                  }}
                  onDrop={(e) => {
                    e.preventDefault()
                    setDragged(false)
                    void upload(Array.from(e.dataTransfer.files ?? []))
                  }}
                  className={
                    'relative rounded-xl border border-dashed bg-gray-50/80 p-5 transition-colors dark:bg-gray-800/40 ' +
                    (dragged
                      ? 'border-primary-500 bg-primary-50/90 ring-2 ring-primary-500/20 dark:border-primary-400 dark:bg-primary-500/10 dark:ring-primary-400/20'
                      : 'border-gray-300 dark:border-gray-600')
                  }
                >
                  <div className="flex flex-col items-center justify-center gap-3 text-center sm:flex-row sm:items-center sm:gap-4 sm:text-start">
                    <div className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-white shadow-sm dark:bg-gray-700/50">
                      <HeroIcon name="cloudUp" className="size-6 text-gray-400 dark:text-gray-500" />
                    </div>
                    <div className="min-w-0">
                      <p className="text-sm font-medium text-gray-700 dark:text-gray-300">{L('برای آپلود فایل کلیک کنید', 'Click to upload a file')}</p>
                      <p className="mt-0.5 text-xs text-gray-500 dark:text-gray-400">{L('یا فایل‌ها را اینجا بکشید و رها کنید', 'or drag and drop files here')}</p>
                    </div>
                    <input
                      ref={fileInput}
                      type="file"
                      multiple
                      accept={allowed.join(',')}
                      className="hidden"
                      disabled={uploading}
                      onChange={(e) => {
                        const files = Array.from(e.target.files ?? [])
                        e.target.value = ''
                        void upload(files)
                      }}
                    />
                    <button
                      type="button"
                      disabled={uploading}
                      onClick={() => fileInput.current?.click()}
                      className={
                        'inline-flex h-10 shrink-0 items-center justify-center gap-2 whitespace-nowrap rounded-xl bg-white px-5 text-sm font-medium text-gray-700 shadow-sm ring-1 ring-gray-200 transition-colors hover:bg-gray-50 hover:shadow dark:bg-gray-700 dark:text-gray-200 dark:ring-gray-600 dark:hover:bg-gray-600 ' +
                        (uploading ? 'pointer-events-none opacity-70' : '')
                      }
                    >
                      {uploading && <Spinner className="size-4 shrink-0 text-gray-500" />}
                      <span className="whitespace-nowrap">{uploading ? L('در حال آپلود...', 'Uploading...') : L('مرور فایل‌ها', 'Browse files')}</span>
                    </button>
                  </div>
                  {uploading && (
                    <div className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-xl bg-white/75 dark:bg-gray-900/70">
                      <span className="inline-flex items-center gap-2 text-sm font-medium text-gray-700 dark:text-gray-200">
                        <Spinner />
                        {L('در حال آپلود...', 'Uploading...')}
                      </span>
                    </div>
                  )}
                </div>
                {error && (
                  <p role="alert" className="text-sm text-danger-600 dark:text-danger-400">
                    {error}
                  </p>
                )}
              </div>

              {/* Grid */}
              <div className="min-h-0 flex-1 overflow-y-auto p-5">
                <div className="grid grid-cols-3 gap-2 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-7">
                  {list.isLoading ? (
                    Array.from({ length: 14 }, (_, i) => <div key={i} className="aspect-square animate-pulse rounded-lg bg-gray-100 dark:bg-gray-800" />)
                  ) : items.length === 0 ? (
                    <div className="col-span-full flex flex-col items-center justify-center rounded-2xl border-2 border-dashed border-gray-200 py-16 dark:border-gray-700">
                      <HeroIcon name="photo" className="mx-auto size-14 text-gray-300 dark:text-gray-600" />
                      <p className="mt-3 text-sm font-medium text-gray-500 dark:text-gray-400">{L('رسانه‌ای یافت نشد', 'No media found')}</p>
                      <p className="mt-1 text-xs text-gray-400 dark:text-gray-500">{L('با دکمهٔ بالا فایل آپلود کنید', 'Upload a file with the button above')}</p>
                    </div>
                  ) : (
                    items.map((m) => {
                      const isSelected = ids.includes(m.id)
                      return (
                        <button
                          key={m.id}
                          type="button"
                          onClick={() => toggle(m)}
                          aria-pressed={isSelected}
                          aria-label={mediaName(m)}
                          title={mediaName(m)}
                          className="group relative aspect-square overflow-hidden rounded-lg bg-gray-100 transition-all duration-150 focus:outline-none focus-visible:ring-2 focus-visible:ring-primary-500 focus-visible:ring-offset-2 dark:bg-gray-800 dark:focus-visible:ring-offset-gray-900"
                        >
                          <MediaPreview url={thumbUrl(m)} kind={kindOf(m.mime_type)} name={mediaName(m)} variant="grid" />
                          {isSelected ? (
                            <div className="absolute inset-0 bg-primary-500/15">
                              <div className="absolute end-1.5 top-1.5 flex size-6 items-center justify-center rounded-full bg-primary-500 shadow-md">
                                <HeroIcon name="check" strokeWidth={3} className="size-3.5 text-white" />
                              </div>
                            </div>
                          ) : (
                            focusId === m.id && <div className="absolute inset-0 rounded-lg ring-2 ring-primary-400/40 ring-inset" />
                          )}
                        </button>
                      )
                    })
                  )}
                </div>

                {lastPage > 1 && (
                  <div className="mt-4 border-t border-gray-200/80 pt-4 dark:border-gray-700/50">
                    <nav aria-label={L('صفحه‌بندی رسانه', 'Media pages')} className="flex flex-col items-center gap-3 sm:flex-row sm:justify-between">
                      <p className="text-xs text-gray-500 dark:text-gray-400">
                        {L(
                          `نمایش ${formatNumber((page - 1) * PER_PAGE + 1)} تا ${formatNumber(Math.min(page * PER_PAGE, total))} از ${formatNumber(total)} مورد`,
                          `Showing ${(page - 1) * PER_PAGE + 1} to ${Math.min(page * PER_PAGE, total)} of ${total}`,
                        )}
                      </p>
                      <div className="inline-flex items-center gap-1.5 rtl:flex-row-reverse">
                        <button
                          type="button"
                          disabled={page <= 1}
                          onClick={() => setPage(page - 1)}
                          aria-label={L('قبلی', 'Previous')}
                          className={pageBtn + ' w-9 px-0 disabled:cursor-not-allowed disabled:border-transparent disabled:bg-gray-100 disabled:text-gray-400 dark:disabled:bg-gray-800 dark:disabled:text-gray-600'}
                        >
                          <HeroIcon name="chevronLeft" className="size-4 rtl:rotate-180" />
                        </button>
                        <div className="inline-flex items-center gap-1 rtl:flex-row-reverse">
                          {pageList(page, lastPage).map((p, i) =>
                            p === '…' ? (
                              <span key={'gap' + i} aria-hidden="true" className="inline-flex h-9 min-w-9 items-center justify-center px-1 text-sm text-gray-400 dark:text-gray-500">
                                …
                              </span>
                            ) : p === page ? (
                              <span key={p} aria-current="page" className="inline-flex h-9 min-w-9 items-center justify-center rounded-lg bg-primary-600 px-3 text-sm font-semibold text-white shadow-sm dark:bg-primary-500">
                                {formatNumber(p)}
                              </span>
                            ) : (
                              <button key={p} type="button" onClick={() => setPage(p)} className={pageBtn}>
                                {formatNumber(p)}
                              </button>
                            ),
                          )}
                        </div>
                        <button
                          type="button"
                          disabled={page >= lastPage}
                          onClick={() => setPage(page + 1)}
                          aria-label={L('بعدی', 'Next')}
                          className={pageBtn + ' w-9 px-0 disabled:cursor-not-allowed disabled:border-transparent disabled:bg-gray-100 disabled:text-gray-400 dark:disabled:bg-gray-800 dark:disabled:text-gray-600'}
                        >
                          <HeroIcon name="chevronRight" className="size-4 rtl:rotate-180" />
                        </button>
                      </div>
                    </nav>
                  </div>
                )}

                {/* Details on small screens, under the grid */}
                {detail && (
                  <div className="mt-4 rounded-xl bg-gray-50 p-4 ring-1 ring-gray-950/5 lg:hidden dark:bg-gray-800/50 dark:ring-white/10">
                    <h4 className="text-sm font-semibold break-all text-gray-900 dark:text-white">{mediaName(detail)}</h4>
                    <dl className="mt-3 grid grid-cols-2 gap-3 text-xs">
                      <div>
                        <dt className="text-gray-500 dark:text-gray-400">{L('نوع فایل', 'File type')}</dt>
                        <dd className="mt-0.5 font-medium text-gray-800 dark:text-gray-200" dir="ltr">{detail.mime_type}</dd>
                      </div>
                      <div>
                        <dt className="text-gray-500 dark:text-gray-400">{L('حجم فایل', 'File size')}</dt>
                        <dd className="mt-0.5 font-medium text-gray-800 dark:text-gray-200">{humanSize(detail.size)}</dd>
                      </div>
                      {detail.width && detail.height ? (
                        <div className="col-span-2">
                          <dt className="text-gray-500 dark:text-gray-400">{L('ابعاد', 'Dimensions')}</dt>
                          <dd className="mt-0.5 font-medium text-gray-800 dark:text-gray-200">{dimensions(detail)}</dd>
                        </div>
                      ) : null}
                    </dl>
                  </div>
                )}
              </div>
            </div>

            {/* Attachment details */}
            <aside className="hidden w-72 shrink-0 overflow-y-auto border-s border-gray-200/80 lg:block xl:w-80 dark:border-gray-700/50">
              <div className="px-4 pt-4 pb-2">
                <h3 className="text-sm font-semibold text-gray-900 dark:text-white">{L('جزئیات پیوست', 'Attachment details')}</h3>
              </div>
              {detail ? (
                <div className="space-y-4 p-4">
                  <div className="overflow-hidden rounded-lg bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-800 dark:ring-white/10">
                    <MediaPreview key={detail.id} url={defaultFileUrl(detail.path)} kind={kindOf(detail.mime_type)} name={mediaName(detail)} variant="detail" />
                  </div>
                  <p className="text-sm font-medium break-all text-gray-900 dark:text-white">{mediaName(detail)}</p>
                  <dl className="space-y-3 text-sm">
                    {(
                      [
                        [L('نام فایل', 'File name'), detail.file_name, 'break-all'],
                        [L('نوع فایل', 'File type'), detail.mime_type, ''],
                        [L('ابعاد', 'Dimensions'), dimensions(detail), ''],
                        [L('حجم فایل', 'File size'), humanSize(detail.size), ''],
                        [L('تاریخ آپلود', 'Uploaded on'), formatDate(detail.created_at), ''],
                      ] as const
                    ).map(([label, value, extra]) => (
                      <div key={label}>
                        <dt className="text-xs font-medium tracking-wide text-gray-500 uppercase dark:text-gray-400">{label}</dt>
                        <dd className={'mt-1 text-gray-800 dark:text-gray-200 ' + extra}>{value}</dd>
                      </div>
                    ))}
                  </dl>
                </div>
              ) : (
                <div className="flex min-h-48 items-center justify-center p-6 text-center">
                  <p className="text-sm text-gray-500 dark:text-gray-400">{L('برای مشاهده جزئیات، یک فایل انتخاب کنید', 'Choose a file to see its details')}</p>
                </div>
              )}
            </aside>
          </div>

          {/* Footer */}
          <div className="flex flex-wrap items-center justify-between gap-4 border-t border-gray-200/80 bg-gray-50/50 px-6 py-4 rtl:flex-row-reverse dark:border-gray-700/50 dark:bg-gray-800/50">
            <div className="order-2 rtl:order-1">
              {ids.length > 0 && (
                <span className="inline-flex items-center gap-2 rounded-lg bg-primary-100 px-3 py-1.5 text-sm font-medium text-primary-700 dark:bg-primary-500/20 dark:text-primary-400">
                  <HeroIcon name="checkCircle" strokeWidth={2} className="size-4" />
                  <span>{L(`${formatNumber(ids.length)} مورد انتخاب شده`, `${ids.length} selected`)}</span>
                </span>
              )}
            </div>
            <div className="order-1 flex gap-3 rtl:order-2 rtl:flex-row-reverse">
              <button
                type="button"
                onClick={close}
                className="rounded-xl border border-gray-200 bg-white px-4 py-2.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 dark:border-gray-600 dark:bg-gray-800 dark:text-gray-200 dark:hover:bg-gray-700"
              >
                {L('لغو', 'Cancel')}
              </button>
              <button
                type="button"
                disabled={ids.length === 0}
                onClick={() => void confirm()}
                className="rounded-xl bg-primary-600 px-5 py-2.5 text-sm font-medium text-white shadow-sm transition-all hover:bg-primary-700 disabled:cursor-not-allowed disabled:opacity-50 disabled:shadow-none"
              >
                {L('تأیید انتخاب', 'Confirm selection')}
              </button>
            </div>
          </div>
        </div>
      </div>
    </dialog>,
    document.body,
  )
}
