import { useState } from 'react'
import { ApiError, Card, definePage, formatNumber, PageHeader, tr, usePageData, usePageSubmit, usePanel } from '../../index'

// Content import and export (MKSine's mks-import-export): the "content_transfer" page.

const TransferIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M7 4v13M3 13l4 4 4-4M17 20V7M13 11l4-4 4 4" />
  </svg>
)

type Kind = 'post' | 'page' | 'media'
type Format = 'csv' | 'json' | 'xml' | 'wxr'

interface TransferData {
  formats: Format[]
  fields: Record<Kind, string[]>
  max_bytes: number
  max_rows: number
}

interface ImportResult {
  dry_run: boolean
  processed: number
  created: number
  updated: number
  skipped: number
  errors: { row: number; message: string }[]
}

const kindLabels: Record<Kind, string> = {
  post: tr({ fa: 'نوشته‌ها', en: 'Posts' }),
  page: tr({ fa: 'برگه‌ها', en: 'Pages' }),
  media: tr({ fa: 'رسانه (متن جایگزین، عنوان، زیرنویس)', en: 'Media (alt, title, caption)' }),
}

const formatLabels: Record<Format, string> = {
  csv: 'CSV',
  json: 'JSON',
  xml: 'XML',
  wxr: tr({ fa: 'خروجی وردپرس (WXR)', en: 'WordPress export (WXR)' }),
}

const accepts: Record<Format, string> = { csv: '.csv,text/csv', json: '.json,application/json', xml: '.xml,text/xml', wxr: '.xml,text/xml' }

const input =
  'block w-full rounded-lg border-0 bg-white px-3 py-1.5 text-sm shadow-xs ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5 dark:ring-white/10'
const button =
  'rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white shadow-xs hover:bg-primary-500 disabled:opacity-50'
const secondary =
  'rounded-lg bg-white px-3 py-1.5 text-sm font-semibold text-gray-700 shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10'

function Field({ label, htmlFor, children, error }: { label: string; htmlFor: string; children: React.ReactNode; error?: string }) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={htmlFor} className="block text-sm font-medium">
        {label}
      </label>
      {children}
      {error && (
        <p className="text-sm text-danger-600" role="alert">
          {error}
        </p>
      )}
    </div>
  )
}

function ImportCard({ d }: { d: TransferData }) {
  const submit = usePageSubmit()
  const [kind, setKind] = useState<Kind>('post')
  const [format, setFormat] = useState<Format>('csv')
  const [file, setFile] = useState<{ name: string; text: string } | null>(null)
  const [mapping, setMapping] = useState<{ from: string; to: string }[]>([])
  const [dryRun, setDryRun] = useState(true)
  const [busy, setBusy] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [result, setResult] = useState<ImportResult | null>(null)

  async function pickFile(f: File | undefined) {
    setErrors({})
    setResult(null)
    if (!f) return setFile(null)
    if (f.size > d.max_bytes) {
      setFile(null)
      return setErrors({ content: tr({ fa: `فایل بیش از ${formatNumber(d.max_bytes / 1048576)} مگابایت است.`, en: `The file is over ${d.max_bytes / 1048576} MB.` }) })
    }
    setFile({ name: f.name, text: await f.text() })
  }

  async function run() {
    setBusy(true)
    setErrors({})
    setResult(null)
    try {
      const map = Object.fromEntries(mapping.filter((m) => m.from.trim() && m.to).map((m) => [m.from.trim(), m.to]))
      const res = await submit({ kind, format, content: file?.text ?? '', mapping: map, dry_run: dryRun })
      setResult((res.data as ImportResult) ?? null)
    } catch (e) {
      if (e instanceof ApiError) {
        setErrors(Object.fromEntries(Object.entries(e.errors).map(([k, v]) => [k, v[0] ?? ''])))
        if (Object.keys(e.errors).length === 0) setErrors({ content: e.message })
      } else setErrors({ content: String(e) })
    } finally {
      setBusy(false)
    }
  }

  const targets = d.fields[kind] ?? []
  return (
    <Card>
      <h2 className="mb-1 text-base font-semibold">{tr({ fa: 'درون‌ریزی', en: 'Import' })}</h2>
      <p className="mb-4 text-sm text-gray-500 dark:text-gray-400">
        {tr({
          fa: `نوشته و برگه با نامک یکی می‌شوند (برگهٔ زباله‌دان برمی‌گردد)؛ رسانه با مسیر فایل. حداکثر ${formatNumber(d.max_rows)} ردیف.`,
          en: `Posts and pages match by slug (a trashed page comes back), media by file path. At most ${d.max_rows} rows.`,
        })}
      </p>
      <div className="grid gap-4 sm:grid-cols-2">
        <Field label={tr({ fa: 'قالب فایل', en: 'File format' })} htmlFor="import-format" error={errors.format}>
          <select id="import-format" className={input} value={format} onChange={(e) => setFormat(e.target.value as Format)}>
            {d.formats.map((f) => (
              <option key={f} value={f}>
                {formatLabels[f]}
              </option>
            ))}
          </select>
        </Field>
        <Field label={tr({ fa: 'نوع محتوا', en: 'Content' })} htmlFor="import-kind" error={errors.kind}>
          <select id="import-kind" className={input} value={kind} disabled={format === 'wxr'} onChange={(e) => setKind(e.target.value as Kind)}>
            {(Object.keys(kindLabels) as Kind[]).map((k) => (
              <option key={k} value={k}>
                {kindLabels[k]}
              </option>
            ))}
          </select>
          {format === 'wxr' && (
            <p className="text-xs text-gray-500">{tr({ fa: 'نوشته‌ها و برگه‌های فایل وردپرس هر دو خوانده می‌شوند.', en: 'Posts and pages of the file are both read.' })}</p>
          )}
        </Field>
        <div className="sm:col-span-2">
          <Field label={tr({ fa: 'فایل', en: 'File' })} htmlFor="import-file" error={errors.content}>
            <div className="flex flex-wrap items-center gap-3">
              <label className={secondary + ' cursor-pointer focus-within:ring-2 focus-within:ring-primary-500'}>
                {tr({ fa: 'انتخاب فایل', en: 'Choose a file' })}
                <input
                  id="import-file"
                  type="file"
                  accept={accepts[format]}
                  className="sr-only"
                  onChange={(e) => {
                    const f = e.target.files?.[0]
                    e.target.value = ''
                    void pickFile(f)
                  }}
                />
              </label>
              <span className="text-sm text-gray-500 dark:text-gray-400" dir="auto">
                {file ? file.name : tr({ fa: 'فایلی انتخاب نشده است.', en: 'No file chosen.' })}
              </span>
            </div>
          </Field>
        </div>
        {format !== 'wxr' && (
          <div className="space-y-2 sm:col-span-2">
            <p className="text-sm font-medium">{tr({ fa: 'نگاشت ستون‌ها (اختیاری)', en: 'Column mapping (optional)' })}</p>
            <p className="text-xs text-gray-500">
              {tr({ fa: 'نام ستون فایل ← فیلد. بدون نگاشت، ستون‌ها با همان نام خوانده می‌شوند: ', en: 'File column → field. Without one, columns are read by name: ' })}
              <span dir="ltr">{targets.join(', ')}</span>
            </p>
            {mapping.map((m, i) => (
              <div key={i} className="flex items-center gap-2">
                <input
                  aria-label={tr({ fa: 'ستون فایل', en: 'File column' })}
                  className={input}
                  dir="ltr"
                  value={m.from}
                  onChange={(e) => setMapping(mapping.map((x, j) => (j === i ? { ...x, from: e.target.value } : x)))}
                />
                <span aria-hidden="true">←</span>
                <select
                  aria-label={tr({ fa: 'فیلد', en: 'Field' })}
                  className={input}
                  value={m.to}
                  onChange={(e) => setMapping(mapping.map((x, j) => (j === i ? { ...x, to: e.target.value } : x)))}
                >
                  <option value="">—</option>
                  {targets.map((f) => (
                    <option key={f} value={f}>
                      {f}
                    </option>
                  ))}
                </select>
                <button type="button" className="text-sm text-gray-500 hover:text-danger-600" onClick={() => setMapping(mapping.filter((_, j) => j !== i))}>
                  {tr({ fa: 'حذف', en: 'Remove' })}
                </button>
              </div>
            ))}
            {errors.mapping && <p className="text-sm text-danger-600">{errors.mapping}</p>}
            <button type="button" className={secondary} onClick={() => setMapping([...mapping, { from: '', to: '' }])}>
              + {tr({ fa: 'افزودن نگاشت', en: 'Add a mapping' })}
            </button>
          </div>
        )}
        <label className="flex items-center gap-2 text-sm sm:col-span-2">
          <input type="checkbox" className="mks-check" checked={dryRun} onChange={(e) => setDryRun(e.target.checked)} />
          {tr({ fa: 'فقط آزمایش: نشان بده چه می‌شود، چیزی ذخیره نکن', en: 'Dry run: show what would happen, save nothing' })}
        </label>
      </div>
      <div className="mt-5">
        <button type="button" className={button} disabled={busy || !file} onClick={() => void run()}>
          {busy ? tr({ fa: 'در حال درون‌ریزی…', en: 'Importing…' }) : dryRun ? tr({ fa: 'آزمایش', en: 'Try' }) : tr({ fa: 'درون‌ریزی', en: 'Import' })}
        </button>
      </div>
      {result && (
        <div className="mt-5 rounded-xl bg-gray-50 p-4 text-sm dark:bg-white/5" role="status">
          <p className="font-medium">
            {result.dry_run ? tr({ fa: 'نتیجهٔ آزمایش (چیزی ذخیره نشد)', en: 'Dry run (nothing saved)' }) : tr({ fa: 'نتیجه', en: 'Result' })}
          </p>
          <dl className="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-4">
            {(
              [
                [tr({ fa: 'ردیف‌ها', en: 'Rows' }), result.processed],
                [tr({ fa: 'تازه', en: 'New' }), result.created],
                [tr({ fa: 'به‌روزشده', en: 'Updated' }), result.updated],
                [tr({ fa: 'ردشده', en: 'Skipped' }), result.skipped],
              ] as const
            ).map(([label, n]) => (
              <div key={label}>
                <dt className="text-gray-500">{label}</dt>
                <dd className="text-lg font-semibold">{formatNumber(n)}</dd>
              </div>
            ))}
          </dl>
          {result.errors.length > 0 && (
            <ul className="mt-3 space-y-1 text-danger-700 dark:text-danger-400">
              {result.errors.map((e) => (
                <li key={e.row + e.message}>
                  {tr({ fa: 'ردیف', en: 'Row' })} {formatNumber(e.row)}: {e.message}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </Card>
  )
}

function ExportCard() {
  const { api, notify } = usePanel()
  const [kind, setKind] = useState<Kind>('post')
  const [format, setFormat] = useState<Exclude<Format, 'wxr'>>('csv')
  const [busy, setBusy] = useState(false)

  async function download() {
    setBusy(true)
    try {
      const res = await api.plugin<{ filename: string; content: string }>('cms', `/export?kind=${kind}&format=${format}`)
      const types = { csv: 'text/csv', json: 'application/json', xml: 'application/xml' }
      const url = URL.createObjectURL(new Blob([res.content], { type: `${types[format]};charset=utf-8` }))
      const a = document.createElement('a')
      a.href = url
      a.download = res.filename
      a.click()
      URL.revokeObjectURL(url)
    } catch (e) {
      notify(e instanceof Error ? e.message : String(e), 'danger')
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <h2 className="mb-1 text-base font-semibold">{tr({ fa: 'برون‌بری', en: 'Export' })}</h2>
      <p className="mb-4 text-sm text-gray-500 dark:text-gray-400">
        {tr({ fa: 'فایلی که اینجا دریافت می‌کنید با همین صفحه دوباره درون‌ریزی می‌شود.', en: 'The file can be imported back on this page.' })}
      </p>
      <div className="grid gap-4 sm:grid-cols-3 sm:items-end">
        <Field label={tr({ fa: 'نوع محتوا', en: 'Content' })} htmlFor="export-kind">
          <select id="export-kind" className={input} value={kind} onChange={(e) => setKind(e.target.value as Kind)}>
            {(Object.keys(kindLabels) as Kind[]).map((k) => (
              <option key={k} value={k}>
                {kindLabels[k]}
              </option>
            ))}
          </select>
        </Field>
        <Field label={tr({ fa: 'قالب فایل', en: 'File format' })} htmlFor="export-format">
          <select id="export-format" className={input} value={format} onChange={(e) => setFormat(e.target.value as 'csv' | 'json' | 'xml')}>
            {(['csv', 'json', 'xml'] as const).map((f) => (
              <option key={f} value={f}>
                {formatLabels[f]}
              </option>
            ))}
          </select>
        </Field>
        <div>
          <button type="button" className={button} disabled={busy} onClick={() => void download()}>
            {tr({ fa: 'دریافت فایل', en: 'Download' })}
          </button>
        </div>
      </div>
    </Card>
  )
}

function ContentTransferPage() {
  const data = usePageData<TransferData>()
  const d = data.data
  return (
    <div className="space-y-6">
      <PageHeader
        title={tr({ fa: 'درون‌ریزی و برون‌بری', en: 'Import and export' })}
        description={tr({ fa: 'نوشته‌ها، برگه‌ها و اطلاعات رسانه‌های همین سایت از CSV، JSON، XML یا خروجی وردپرس.', en: "This site's posts, pages and media details, from CSV, JSON, XML or a WordPress export." })}
      />
      {data.error && <p className="text-sm text-danger-600">{data.error.message}</p>}
      {data.isPending || !d ? (
        <div className="h-64 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
      ) : (
        <>
          <ImportCard d={d} />
          <ExportCard />
        </>
      )}
    </div>
  )
}

export function transferPage(group?: string) {
  return definePage({
    name: 'content_transfer',
    label: tr({ fa: 'درون‌ریزی و برون‌بری', en: 'Import and export' }),
    icon: <TransferIcon />,
    group,
    component: ContentTransferPage,
  })
}
