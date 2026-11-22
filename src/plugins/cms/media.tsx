import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { defaultFileUrl, defineField, defineResource, formatNumber, tr, usePanel, type CustomFieldProps } from '../../index'
import { imageEditor } from './imageEditor'
import { HeroIcon, humanSize, kindOf, kindOfPath, mediaAccept, MediaPickerDialog, MediaPreview, mediaName, type MediaAccept } from './mediaPickerDialog'
import type { Media } from './types'

const PhotoIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M4 5h16v14H4zM4 15l4-4 5 5 3-3 4 4M15 9h.01" />
  </svg>
)


const kindLabels = {
  'image/jpeg': 'JPEG',
  'image/png': 'PNG',
  'image/webp': 'WebP',
  'image/gif': 'GIF',
  'application/pdf': 'PDF',
  'video/mp4': 'MP4',
  'video/webm': 'WebM',
  'audio/mpeg': 'MP3',
  'audio/wave': 'WAV',
  'application/ogg': 'OGG',
}

const size = humanSize

export const media = defineResource<Media>({
  name: 'cms_media',
  label: tr({ fa: 'رسانه', en: 'Media' }),
  pluralLabel: tr({ fa: 'کتابخانه رسانه', en: 'Media library' }),
  navigationSort: 10,
  icon: <PhotoIcon />,
  group: tr({ fa: 'رسانه', en: 'Media' }),
  defaultSort: '-created_at',
  recordTitle: (m) => m.title || m.file_name,

  table: (t) => [
    t.image('path').label('').size(48),
    t.text('file_name').label(tr({ fa: 'فایل', en: 'File' })).searchable().sortable().weight('medium').description((m) => m.title ?? ''),
    t.badge('mime_type').label(tr({ fa: 'نوع', en: 'Type' })).labels(kindLabels),
    t.text('size').label(tr({ fa: 'حجم', en: 'Size' })).numeric().sortable().format((v) => size(v)),
    t.text('width').label(tr({ fa: 'ابعاد', en: 'Dimensions' })).format((_, m) => (m.width && m.height ? `${formatNumber(m.width)}×${formatNumber(m.height)}` : '')),
    t.text('conversions').label(tr({ fa: 'نسخه‌ها', en: 'Copies' })).toggleable(true)
      .format((v) => formatNumber(Object.keys((v as Record<string, string>) ?? {}).length)),
    t.date('created_at').label(tr({ fa: 'بارگذاری', en: 'Uploaded' })).since().sortable(),
  ],

  filters: (f) => [f.select('mime_type').label(tr({ fa: 'نوع', en: 'Type' })).options(kindLabels).multiple()],

  actions: (a) => [
    a.make('edit_image')
      .label(tr({ fa: 'ویرایش تصویر', en: 'Edit image' }))
      .color('primary')
      .row()
      // WebP only when the server has a WebP encoder; its per-record ability hides the rest.
      .visible((m) => m.mime_type === 'image/jpeg' || m.mime_type === 'image/png' || m.mime_type === 'image/webp')
      .modalHeading(tr({ fa: 'ویرایش تصویر', en: 'Edit image' }))
      .form((f) => [
        imageEditor('edit').label(tr({ fa: 'برش، چرخش و قرینه', en: 'Crop, rotate and mirror' })).columnSpan('full'),
        f.toggle('as_copy').label(tr({ fa: 'ذخیره به‌عنوان فایل تازه (اصل دست‌نخورده می‌ماند)', en: 'Save as a new file (keep the original)' })).default(true),
      ])
      .modalSubmitLabel(tr({ fa: 'ذخیرهٔ تصویر', en: 'Save image' })),
    a.delete(),
  ],

  form: (f) => [
    f.fileUpload('path')
      .label(tr({ fa: 'فایل', en: 'File' }))
      .accept(mediaAccept)
      .maxSize(20 * 1024 * 1024)
      .required()
      .columnSpan('full')
      .helperText(tr({ fa: 'تصویر، PDF، ویدیو یا صدا تا ۲۰ مگابایت. برای تصویرها نسخه‌های کوچک‌تر خودکار ساخته می‌شود.', en: 'Images, PDF, video or audio up to 20 MB. Smaller copies of images are made automatically.' })),
    f.textInput('alt').label(tr({ fa: 'متن جایگزین', en: 'Alt text' })).maxLength(255)
      .helperText(tr({ fa: 'برای صفحه‌خوان‌ها و موتورهای جستجو: تصویر چه نشان می‌دهد؟', en: 'For screen readers and search engines: what does the image show?' })),
    f.textInput('title').label(tr({ fa: 'عنوان', en: 'Title' })).maxLength(255),
    f.textarea('caption').label(tr({ fa: 'زیرنویس', en: 'Caption' })).rows(2).columnSpan('full'),
  ],
})

export { mediaAccept }

// --- the media picker (MKSine's MediaPicker field) -------------------------------------------

type PickerOptions = { accept: MediaAccept }

const L = (fa: string, en: string) => tr({ fa, en })

/** The dashed button under the chosen files that opens the library. */
function OpenButton({ id, label, disabled, invalid, onClick, a11y }: { id: string; label: string; disabled?: boolean; invalid?: boolean; onClick: () => void; a11y: object }) {
  return (
    <button
      id={id}
      type="button"
      disabled={disabled}
      onClick={onClick}
      {...a11y}
      className={
        'inline-flex items-center gap-2 rounded-xl border-2 border-dashed bg-gray-50/50 px-4 py-3 text-sm font-medium text-gray-600 transition-all duration-200 hover:border-primary-400 hover:bg-primary-50 hover:text-primary-700 focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 focus:outline-none disabled:pointer-events-none disabled:opacity-60 rtl:flex-row-reverse dark:bg-gray-800/30 dark:text-gray-400 dark:hover:border-primary-500 dark:hover:bg-primary-500/10 dark:hover:text-primary-400 dark:focus:ring-offset-gray-900 ' +
        (invalid ? 'border-danger-500' : 'border-gray-300 dark:border-gray-600')
      }
    >
      <HeroIcon name="photo" className="size-5 shrink-0" />
      <span>{label}</span>
    </button>
  )
}

const tile =
  'group relative aspect-square overflow-hidden rounded-xl border border-gray-200/80 bg-gray-50 shadow-sm ring-1 ring-black/5 transition-all duration-200 hover:shadow-md hover:ring-primary-500/30 dark:border-gray-600/60 dark:bg-gray-800 dark:ring-white/5'
const removeBtn =
  'absolute end-1 top-1 z-20 flex size-5 items-center justify-center rounded-full bg-white/90 text-gray-700 shadow-sm hover:bg-danger-500 hover:text-white dark:bg-gray-800/90 dark:text-gray-300 dark:hover:bg-danger-500'

/** Picks a file of the media library (or uploads one into it); the value is its path. */
function MediaPickerControl({ id, value, onChange, disabled, invalid, options, a11y }: CustomFieldProps<string | null, PickerOptions>) {
  const { api } = usePanel()
  const [open, setOpen] = useState(false)
  const current = typeof value === 'string' && value !== '' ? value : null
  // The record behind the path: the dialog starts with it selected.
  const record = useQuery({
    queryKey: ['records', 'cms_media', { path: current }],
    queryFn: () => api.list<Media>('cms_media', { perPage: 1, filters: [{ column: 'path', op: 'eq' as const, value: current ?? '' }] }),
    enabled: current !== null,
    staleTime: 60_000,
  })
  const m = current ? record.data?.data[0] : undefined
  return (
    <div className="space-y-3">
      {current && (
        <div dir="ltr" className="grid grid-cols-3 gap-2">
          <div className="flex min-w-0 flex-col gap-1">
            <div className={tile}>
              <MediaPreview
                url={defaultFileUrl(m && kindOf(m.mime_type) === 'image' ? (m.conversions?.['320'] ?? m.path) : current)}
                kind={m ? kindOf(m.mime_type) : kindOfPath(current)}
                name={m ? mediaName(m) : current.split('/').pop() ?? ''}
                variant="tile"
              />
              {!disabled && (
                <button type="button" onClick={() => onChange(null)} aria-label={L('برداشتن', 'Remove')} className={removeBtn}>
                  <HeroIcon name="x" strokeWidth={2.5} className="size-3" />
                </button>
              )}
            </div>
          </div>
        </div>
      )}
      <OpenButton
        id={id}
        label={current ? L('تغییر رسانه', 'Change media') : L('انتخاب رسانه', 'Select media')}
        disabled={disabled}
        invalid={invalid}
        onClick={() => setOpen(true)}
        a11y={a11y}
      />
      {open && (
        <MediaPickerDialog
          multiple={false}
          accept={options.accept}
          selected={m ? [m.id] : []}
          known={m ? [m] : []}
          onConfirm={(picked) => picked[0] && onChange(picked[0].path)}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  )
}

/**
 * A field that picks a file from the media library (or uploads one into it). The value is
 * the file's path; the server checks it belongs to the site's library.
 *
 *	mediaPicker<Post>('featured_image').label('Featured image')
 */
export const mediaPicker = defineField<string | null, PickerOptions>({ component: MediaPickerControl, defaults: { accept: 'image' }, empty: null })

/** An ordered set of library files (a media collection); the value is their ids. */
function MediaGalleryControl({ id, value, onChange, disabled, invalid, options, a11y }: CustomFieldProps<number[], PickerOptions>) {
  const { api } = usePanel()
  const [open, setOpen] = useState(false)
  const [dragFrom, setDragFrom] = useState<number | null>(null)
  const ids = Array.isArray(value) ? value.map(Number) : []
  const items = useQuery({
    queryKey: ['records', 'cms_media', { gallery: [...ids].sort() }],
    queryFn: () => api.list<Media>('cms_media', { perPage: 100, filters: [{ column: 'id', op: 'in' as const, value: ids.map(String) }] }),
    enabled: ids.length > 0,
    staleTime: 60_000,
    placeholderData: (prev) => prev,
  })
  const byId = new Map((items.data?.data ?? []).map((m) => [m.id, m]))
  const moveTo = (from: number, to: number) => {
    if (from === to || to < 0 || to >= ids.length) return
    const next = [...ids]
    const [moved] = next.splice(from, 1)
    next.splice(to, 0, moved)
    onChange(next)
  }
  const arrow = 'flex size-6 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 hover:text-gray-800 disabled:opacity-30 dark:text-gray-400 dark:hover:bg-gray-800 dark:hover:text-gray-200'
  return (
    <div className="space-y-3">
      {ids.length > 0 && (
        <div dir="ltr" className="grid grid-cols-3 gap-2" aria-label={L('فایل‌های انتخاب‌شده', 'Chosen files')}>
          {ids.map((mid, i) => {
            const m = byId.get(mid)
            return (
              <div
                key={mid}
                className={'flex min-w-0 flex-col gap-1 ' + (dragFrom === i ? 'opacity-40' : '')}
                draggable={!disabled}
                onDragStart={(e) => {
                  e.dataTransfer.effectAllowed = 'move'
                  setDragFrom(i)
                }}
                onDragOver={(e) => dragFrom !== null && e.preventDefault()}
                onDrop={(e) => {
                  e.preventDefault()
                  if (dragFrom !== null) moveTo(dragFrom, i)
                  setDragFrom(null)
                }}
                onDragEnd={() => setDragFrom(null)}
              >
                <div className={tile + (disabled ? '' : ' cursor-grab active:cursor-grabbing')} title={disabled ? undefined : L('برای تغییر ترتیب بکشید', 'Drag to reorder')}>
                  {m ? (
                    <MediaPreview url={defaultFileUrl(kindOf(m.mime_type) === 'image' ? (m.conversions?.['320'] ?? m.path) : m.path)} kind={kindOf(m.mime_type)} name={mediaName(m)} variant="tile" />
                  ) : (
                    <div className="h-full w-full animate-pulse bg-gray-100 dark:bg-gray-800" />
                  )}
                  <span className="pointer-events-none absolute start-1 top-1 z-10 inline-flex h-4 min-w-4 items-center justify-center rounded bg-black/55 px-1 text-[10px] font-semibold text-white">
                    {formatNumber(i + 1)}
                  </span>
                  {!disabled && (
                    <button type="button" onClick={() => onChange(ids.filter((x) => x !== mid))} aria-label={L('برداشتن', 'Remove')} className={removeBtn}>
                      <HeroIcon name="x" strokeWidth={2.5} className="size-3" />
                    </button>
                  )}
                </div>
                {!disabled && (
                  <div className="flex items-center justify-center gap-0.5">
                    <button type="button" className={arrow} disabled={i === 0} onClick={() => moveTo(i, i - 1)} title={L('انتقال به قبل', 'Move earlier')} aria-label={L('انتقال به قبل', 'Move earlier')}>
                      <HeroIcon name="chevronLeft" className="size-3.5" />
                    </button>
                    <button type="button" className={arrow} disabled={i === ids.length - 1} onClick={() => moveTo(i, i + 1)} title={L('انتقال به بعد', 'Move later')} aria-label={L('انتقال به بعد', 'Move later')}>
                      <HeroIcon name="chevronRight" className="size-3.5" />
                    </button>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      )}
      <OpenButton
        id={id}
        label={ids.length > 0 ? L('افزودن رسانه', 'Add media') : L('انتخاب رسانه', 'Select media')}
        disabled={disabled}
        invalid={invalid}
        onClick={() => setOpen(true)}
        a11y={a11y}
      />
      {open && (
        <MediaPickerDialog
          multiple
          accept={options.accept}
          selected={ids}
          known={[...byId.values()]}
          onConfirm={(picked) => onChange(picked.map((m) => m.id))}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  )
}

/**
 * An ordered collection of library files (MKSine's media attachments). The value is the
 * media ids; the server checks they are the site's and stores them in order.
 *
 *	mediaGallery<Post>('gallery').label('Gallery')
 */
export const mediaGallery = defineField<number[], PickerOptions>({ component: MediaGalleryControl, defaults: { accept: 'image' }, empty: [] })
