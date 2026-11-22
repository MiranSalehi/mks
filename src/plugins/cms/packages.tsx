import { useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { ApiError, CodeEditor, definePage, Modal, tr, usePageData, usePageSubmit, usePanel } from '../../index'

// The theme manager and the plugin manager (MKSine's ThemeManager and ManagePlugins,
// without the marketplace): ZIP files are uploaded, updated, rolled back and removed.

const L = (fa: string, en: string) => tr({ fa, en })

const button = 'rounded-lg px-3 py-1.5 text-sm font-semibold disabled:opacity-50'
const primary = button + ' bg-primary-600 text-white hover:bg-primary-500'
const plain = button + ' bg-white ring-1 ring-gray-950/10 hover:bg-gray-50 dark:bg-white/5 dark:ring-white/10 dark:hover:bg-white/10'
const input = 'block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm shadow-xs ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5 dark:ring-white/10'

const BrushIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M9.5 14.5 4 20M14 4l6 6-7.5 7.5a3 3 0 0 1-4.2 0l-1.8-1.8a3 3 0 0 1 0-4.2z" />
  </svg>
)

const PuzzleIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M10 4a2 2 0 1 1 4 0v1h4a1 1 0 0 1 1 1v4h-1a2 2 0 1 0 0 4h1v4a1 1 0 0 1-1 1h-4v-1a2 2 0 1 0-4 0v1H6a1 1 0 0 1-1-1v-4h1a2 2 0 1 0 0-4H5V6a1 1 0 0 1 1-1h4z" />
  </svg>
)

function errorText(e: unknown): string {
  if (e instanceof ApiError) return Object.values(e.errors)[0]?.[0] ?? e.message
  return e instanceof Error ? e.message : String(e)
}

/** Sends a ZIP to POST /plugins/cms/packages, then reloads the page and the meta. */
function UploadDialog({ kind, page, update, ids, onClose }: { kind: 'theme' | 'plugin'; page: string; update: boolean; ids?: string[]; onClose: () => void }) {
  const { api, notify } = usePanel()
  const queryClient = useQueryClient()
  const [file, setFile] = useState<File | null>(null)
  const [force, setForce] = useState(false)
  const [target, setTarget] = useState(ids?.[0] ?? '')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const what = kind === 'theme' ? L('قالب', 'theme') : L('افزونه', 'plugin')

  async function send() {
    if (!file) return
    setBusy(true)
    setError('')
    const body = new FormData()
    body.append('kind', kind)
    body.append('file', file)
    body.append('update', update ? '1' : '0')
    body.append('force', force ? '1' : '0')
    try {
      const r = await api.plugin<{ message: string }>('cms', '/packages', { method: 'POST', body })
      notify(r.message, 'success')
      await queryClient.invalidateQueries({ queryKey: ['page', page] })
      await queryClient.invalidateQueries({ queryKey: ['meta'] })
      onClose()
    } catch (e) {
      setError(errorText(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={update ? L(`به‌روزرسانی ${what}`, `Update a ${what}`) : L(`بارگذاری ${what}`, `Upload a ${what}`)}
      description={
        update
          ? L('فایل ZIP نسخهٔ تازه را انتخاب کنید. نسخهٔ فعلی برای بازگشت نگه داشته می‌شود.', 'Choose the ZIP of the new version. The current one is kept for a rollback.')
          : L(`فایل ZIP ${what} را انتخاب کنید (حداکثر ۲۰ مگابایت).`, `Choose the ${what} ZIP (at most 20 MB).`)
      }
      closeOnBackdrop={false}
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <button type="button" className={plain} onClick={onClose}>
            {L('انصراف', 'Cancel')}
          </button>
          <button type="button" className={primary} disabled={!file || busy} onClick={() => void send()}>
            {update ? L('به‌روزرسانی', 'Update') : L('بارگذاری و نصب', 'Upload and install')}
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        {update && ids && ids.length > 0 && (
          <label className="block text-sm">
            <span className="mb-1 block font-medium">{L(`${what} نصب‌شده`, `Installed ${what}`)}</span>
            <select className={input} value={target} onChange={(e) => setTarget(e.target.value)}>
              {ids.map((id) => (
                <option key={id} value={id}>
                  {id}
                </option>
              ))}
            </select>
            <span className="mt-1 block text-xs text-gray-500">{L('شناسهٔ داخل فایل ZIP باید همین باشد.', 'The id inside the ZIP must be this one.')}</span>
          </label>
        )}
        <label className="block text-sm">
          <span className="mb-1 block font-medium">{L('فایل ZIP', 'ZIP file')}</span>
          <input type="file" accept=".zip,application/zip" className={input} onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
        </label>
        {update && (
          <label className="flex items-center gap-2 text-sm">
            <input type="checkbox" className="mks-check" checked={force} onChange={(e) => setForce(e.target.checked)} />
            {L('اجبار: نصب حتی اگر نسخه تازه‌تر نباشد', 'Force: install even if the version is not newer')}
          </label>
        )}
        {error && (
          <p className="text-sm text-danger-600" role="alert">
            {error}
          </p>
        )}
      </div>
    </Modal>
  )
}

/** Runs a page action, asking first when confirm is set; errors come back as a toast. */
function useRun() {
  const submit = usePageSubmit()
  const { notify } = usePanel()
  const queryClient = useQueryClient()
  const [busy, setBusy] = useState(false)
  async function run(data: Record<string, unknown>, confirm?: string) {
    if (confirm && !window.confirm(confirm)) return
    setBusy(true)
    try {
      await submit(data)
      await queryClient.invalidateQueries({ queryKey: ['meta'] })
    } catch (e) {
      notify(errorText(e), 'danger')
    } finally {
      setBusy(false)
    }
  }
  return { run, busy }
}

// --- shared pieces for the manager pages ------------------------------------------------

const Icon = ({ d, className = 'size-4' }: { d: string; className?: string }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
    <path d={d} />
  </svg>
)

const UploadIcon = () => <Icon d="M12 16V4M7 9l5-5 5 5M4 15v3a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-3" />
const ScanIcon = () => <Icon d="M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM20 20l-4-4M8 11h6M11 8v6" />
const RefreshIcon = () => <Icon d="M20 11a8 8 0 0 0-14.9-3M4 13a8 8 0 0 0 14.9 3M20 4v4h-4M4 20v-4h4" />
const CheckCircleIcon = ({ className }: { className?: string }) => <Icon d="M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM8 12l2.5 2.5L16 9" className={className ?? 'size-4'} />
const PauseIcon = () => <Icon d="M9 5v14M15 5v14" />
const UndoIcon = () => <Icon d="M9 14 4 9l5-5M4 9h9a6 6 0 0 1 0 12h-3" />
const TrashIcon = () => <Icon d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" />
const DocIcon = () => <Icon d="M7 3h7l5 5v13H7zM14 3v5h5M10 13h6M10 17h6" />
const CodeIcon = () => <Icon d="m8 8-4 4 4 4M16 8l4 4-4 4M13 5l-2 14" />
const BoxIcon = ({ className }: { className?: string }) => <Icon d="M12 3 4 7v10l8 4 8-4V7zM4 7l8 4 8-4M12 11v10" className={className ?? 'size-4'} />

const mkButton = 'inline-flex items-center justify-center gap-1.5 rounded-md px-3 py-2 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-50'
const btnAmber = mkButton + ' bg-amber-500 text-white shadow-sm hover:bg-amber-600'
const btnBlue = mkButton + ' bg-blue-600 text-white shadow-sm hover:bg-blue-500'
const btnWhite = mkButton + ' bg-white text-gray-700 ring-1 ring-gray-200 hover:bg-gray-50 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10'
const btnFullAmber = mkButton + ' w-full bg-amber-500 text-white shadow-sm hover:bg-amber-600'
const btnFullWhite = mkButton + ' w-full bg-white text-gray-700 ring-1 ring-gray-200 hover:bg-gray-50 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10'
const btnFullMuted = mkButton + ' w-full bg-gray-100 text-gray-500 ring-1 ring-gray-200 dark:bg-white/5 dark:text-gray-400 dark:ring-white/10'
const btnIcon = 'inline-flex size-8 items-center justify-center rounded-md bg-white text-gray-500 ring-1 ring-gray-200 hover:bg-gray-50 dark:bg-white/5 dark:text-gray-300 dark:ring-white/10 dark:hover:bg-white/10'

const versionChip = 'inline-block rounded-md bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600 dark:bg-white/10 dark:text-gray-300'
const installedPill = 'inline-flex items-center gap-1.5 rounded-md bg-white px-3 py-1.5 text-sm font-medium text-gray-700 shadow-xs ring-1 ring-gray-200 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10'
const assetChip = 'inline-flex items-center gap-1.5 rounded-md bg-gray-50 px-2.5 py-1 text-xs text-gray-600 ring-1 ring-gray-200 dark:bg-white/5 dark:text-gray-300 dark:ring-white/10'

function pluginStatus(status: PluginRow['status']) {
  if (status === 'active') return { label: L('فعال', 'Active'), cls: 'bg-emerald-50 text-emerald-700 ring-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-400 dark:ring-emerald-500/20', dot: 'bg-emerald-500' }
  if (status === 'inactive') return { label: L('غیرفعال', 'Inactive'), cls: 'bg-amber-50 text-amber-700 ring-amber-100 dark:bg-amber-500/10 dark:text-amber-400 dark:ring-amber-500/20', dot: 'bg-amber-500' }
  return { label: L('نصب‌شده', 'Installed'), cls: 'bg-gray-100 text-gray-600 ring-gray-200 dark:bg-white/10 dark:text-gray-300 dark:ring-white/10', dot: 'bg-gray-400' }
}

// --- themes -----------------------------------------------------------------------------

interface ThemeRow {
  id: string
  name: string
  description: string
  swatch: string
  version: string
  author: string
  screenshot: string
  requires: string[] | null
  missing: string[] | null
  uploaded: boolean
  active: boolean
  previous_version: string
}

interface ThemesData {
  themes: ThemeRow[]
  active: string
  custom_css: string
  custom_js: string
  can_code: boolean
  can_install: boolean
}

function CodeDialog({ d, onClose }: { d: ThemesData; onClose: () => void }) {
  const { run, busy } = useRun()
  const [css, setCss] = useState(d.custom_css ?? '')
  const [js, setJs] = useState(d.custom_js ?? '')
  return (
    <Modal
      title={L('CSS و JavaScript سفارشی', 'Custom CSS and JavaScript')}
      description={L('روی همهٔ صفحه‌های این سایت، بعد از فایل‌های قالب.', 'On every page of this site, after the theme files.')}
      size="xl"
      closeOnBackdrop={false}
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <button type="button" className={plain} onClick={onClose}>
            {L('انصراف', 'Cancel')}
          </button>
          <button type="button" className={primary} disabled={busy} onClick={() => void run({ action: 'save_code', custom_css: css, custom_js: js }).then(onClose)}>
            {L('ذخیره', 'Save')}
          </button>
        </div>
      }
    >
      <div className="space-y-5">
        <div>
          <div className="mb-1.5 flex items-center justify-between text-sm">
            <span className="font-medium">{L('CSS سفارشی', 'Custom CSS')}</span>
            <span className="text-xs text-gray-400">CSS</span>
          </div>
          <CodeEditor value={css} onChange={setCss} language="css" label={L('CSS سفارشی', 'Custom CSS')} placeholder="/* your custom CSS */" />
          <p className="mt-1.5 text-xs text-gray-500 dark:text-gray-400">{L('نشانه‌گذاری (<) پذیرفته نیست.', 'No markup (<).')}</p>
        </div>
        <div>
          <div className="mb-1.5 flex items-center justify-between text-sm">
            <span className="font-medium">{L('JavaScript سفارشی', 'Custom JavaScript')}</span>
            <span className="text-xs text-gray-400">JavaScript</span>
          </div>
          <CodeEditor value={js} onChange={setJs} language="javascript" label={L('JavaScript سفارشی', 'Custom JavaScript')} disabled={!d.can_code} placeholder="// your custom JavaScript" />
          <p className="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {d.can_code
              ? L('برای هر بازدیدکننده اجرا می‌شود؛ فقط کد مطمئن بگذارید.', 'Runs for every visitor; only trusted code.')
              : L('JavaScript سفارشی در این نصب خاموش است (Plugin.CustomJS).', 'Custom JavaScript is off in this install (Plugin.CustomJS).')}
          </p>
        </div>
      </div>
    </Modal>
  )
}

function ThemeCard({ t, canInstall, busy, onCode, onRun }: {
  t: ThemeRow
  canInstall: boolean
  busy: boolean
  onCode: () => void
  onRun: (data: Record<string, unknown>, confirm?: string) => Promise<void>
}) {
  const meta = [t.author, t.version && `v${t.version}`].filter(Boolean).join(' · ') || L('قالب داخلی', 'Built-in theme')
  const swatch = t.swatch || '#0f766e'
  return (
    <article
      className={
        'overflow-hidden rounded-xl bg-white shadow-sm ring-1 dark:bg-gray-900/70 ' +
        (t.active ? 'ring-2 ring-amber-400 dark:ring-amber-500/60' : 'ring-gray-200 dark:ring-white/10')
      }
    >
      <div className="relative aspect-[16/9] overflow-hidden bg-gray-100 dark:bg-white/5">
        {t.screenshot ? (
          <img src={t.screenshot} alt="" className="size-full object-cover" loading="lazy" />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center p-6" style={{ background: `linear-gradient(135deg, ${swatch}, color-mix(in oklab, ${swatch} 55%, #0b1220))` }}>
            <span className="text-center text-3xl font-black tracking-tight text-white/90 drop-shadow-sm sm:text-4xl">{t.name}</span>
          </div>
        )}
        <div className="absolute inset-x-3 top-3 flex items-start justify-between gap-2">
          <div>
            {t.active && (
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500 px-2.5 py-1 text-xs font-medium text-white shadow-sm">
                <CheckCircleIcon className="size-3.5" /> {L('فعال', 'Active')}
              </span>
            )}
          </div>
          <span className="inline-flex items-center gap-1 rounded-full bg-teal-500 px-2.5 py-1 text-xs font-medium text-white shadow-sm">
            <BoxIcon className="size-3.5" /> {t.uploaded ? L('پکیج', 'Package') : L('پروژه', 'Project')}
          </span>
        </div>
      </div>

      <div className="space-y-4 p-5">
        <div>
          <h3 className="text-lg font-bold tracking-tight">{t.name}</h3>
          <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{meta}</p>
        </div>
        {t.description && <p className="text-sm text-gray-600 dark:text-gray-400">{t.description}</p>}
        {t.missing && t.missing.length > 0 && (
          <p className="text-xs text-amber-700 dark:text-amber-400">
            {L(`افزونه‌های لازم: ${t.missing.join('، ')}`, `Needs plugins: ${t.missing.join(', ')}`)}
          </p>
        )}
        <div className="flex justify-start">
          <span className={assetChip}>
            <span className="size-1.5 rounded-full bg-emerald-500" aria-hidden="true" />
            {L('دارایی‌ها آماده', 'Assets ready')}
          </span>
        </div>
        <div className="space-y-2">
          {t.active ? (
            <button type="button" disabled className={btnFullMuted}>
              {L('در حال حاضر فعال', 'Currently active')}
            </button>
          ) : (
            <button type="button" disabled={busy || (t.missing?.length ?? 0) > 0} className={btnFullAmber} onClick={() => void onRun({ action: 'activate', id: t.id })}>
              {L('فعال‌سازی', 'Activate')}
            </button>
          )}
          {canInstall && t.uploaded && t.previous_version && (
            <button
              type="button"
              disabled={busy}
              className={btnFullWhite}
              onClick={() => void onRun({ action: 'rollback', id: t.id }, L(`به نسخهٔ ${t.previous_version} برگردد؟`, `Roll back to ${t.previous_version}?`))}
            >
              <UndoIcon /> {L('بازگردانی', 'Roll back')}
            </button>
          )}
          <div className="flex gap-2">
            <button type="button" className={btnFullWhite + ' flex-1'} onClick={onCode}>
              <CodeIcon /> {L('CSS/JS سفارشی', 'Custom CSS/JS')}
            </button>
            {canInstall && t.uploaded && !t.active && (
              <button
                type="button"
                disabled={busy}
                title={L('حذف قالب', 'Delete theme')}
                aria-label={L('حذف قالب', 'Delete theme')}
                className={btnIcon + ' size-9 shrink-0 hover:text-danger-600 dark:hover:text-danger-400'}
                onClick={() => void onRun({ action: 'delete', id: t.id }, L(`قالب «${t.name}» و فایل‌هایش حذف شود؟`, `Delete the theme "${t.name}" and its files?`))}
              >
                <TrashIcon />
              </button>
            )}
          </div>
        </div>
      </div>
    </article>
  )
}

function ThemesPage() {
  const data = usePageData<ThemesData>()
  const { run, busy } = useRun()
  const [dialog, setDialog] = useState<'upload' | 'update' | 'code' | null>(null)
  const d = data.data
  const themes = d?.themes ?? []
  const uploaded = themes.filter((t) => t.uploaded).map((t) => t.id)
  const packages = themes.filter((t) => t.uploaded).length
  const projects = themes.length - packages

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="text-start">
          <h1 className="text-2xl font-bold tracking-tight">{L('مدیریت قالب‌ها', 'Manage themes')}</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {L(
              `${themes.length} قالب (${packages} پکیج، ${projects} پروژه)`,
              `${themes.length} themes (${packages} packages, ${projects} projects)`,
            )}
          </p>
        </div>
        {d && (
          <div className="flex flex-wrap items-center gap-2">
            {d.can_install && (
              <button type="button" className={btnAmber} onClick={() => setDialog('upload')}>
                <UploadIcon /> {L('بارگذاری قالب', 'Upload theme')}
              </button>
            )}
            <button type="button" className={btnBlue} disabled={busy} onClick={() => void run({ action: 'scan' })}>
              <ScanIcon /> {L('شناسایی قالب‌ها', 'Detect themes')}
            </button>
            {d.can_install && uploaded.length > 0 && (
              <button type="button" className={btnAmber} onClick={() => setDialog('update')}>
                <RefreshIcon /> {L('به‌روزرسانی قالب', 'Update theme')}
              </button>
            )}
          </div>
        )}
      </header>

      <div className="flex items-center gap-2">
        <span className={installedPill}>
          <CheckCircleIcon className="size-4 text-gray-500 dark:text-gray-400" /> {L('نصب‌شده', 'Installed')}
        </span>
      </div>

      {data.isPending || !d ? (
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          <div className="h-96 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
          <div className="h-96 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
          <div className="h-96 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
        </div>
      ) : (
        <div className="grid gap-6 sm:grid-cols-2 lg:grid-cols-3">
          {themes.map((t) => (
            <ThemeCard key={t.id} t={t} canInstall={d.can_install} busy={busy} onCode={() => setDialog('code')} onRun={run} />
          ))}
        </div>
      )}
      {dialog === 'upload' && <UploadDialog kind="theme" page="themes" update={false} onClose={() => setDialog(null)} />}
      {dialog === 'update' && <UploadDialog kind="theme" page="themes" update ids={uploaded} onClose={() => setDialog(null)} />}
      {dialog === 'code' && d && <CodeDialog d={d} onClose={() => setDialog(null)} />}
    </div>
  )
}

export function themesPage(options: { group?: string; navigationSort?: number } = {}) {
  return definePage({
    name: 'themes',
    label: L('قالب‌ها', 'Themes'),
    icon: <BrushIcon />,
    group: options.group ?? L('ظاهر', 'Appearance'),
    navigationSort: options.navigationSort ?? 1,
    component: ThemesPage,
  })
}

// --- plugins ----------------------------------------------------------------------------

interface LogEntry {
  at: string
  level: string
  message: string
}

interface PluginRow {
  id: string
  name: string
  description?: string
  author?: string
  version: string
  previous_version: string | null
  status: 'installed' | 'active' | 'inactive'
  requires?: string[]
  boot_error: string | null
  log: LogEntry[] | null
  blocks: number
  actions: number
  shortcodes: number
  settings: number
  content_types: number
}

interface PluginsData {
  plugins: PluginRow[]
  counts: Record<string, number>
}

function LogDialog({ p, onClose }: { p: PluginRow; onClose: () => void }) {
  const { run, busy } = useRun()
  const log = [...(p.log ?? [])].reverse()
  return (
    <Modal
      title={L(`گزارش «${p.name}»`, `Log of "${p.name}"`)}
      size="lg"
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <button type="button" className={plain} disabled={busy || log.length === 0} onClick={() => void run({ action: 'clear_log', id: p.id }).then(onClose)}>
            {L('پاک کردن گزارش', 'Clear the log')}
          </button>
          <button type="button" className={primary} onClick={onClose}>
            {L('بستن', 'Close')}
          </button>
        </div>
      }
    >
      {log.length === 0 ? (
        <p className="text-sm text-gray-500">{L('گزارشی نیست.', 'Nothing logged.')}</p>
      ) : (
        <ul className="max-h-96 space-y-2 overflow-y-auto text-sm">
          {log.map((e, i) => (
            <li key={i} className="flex gap-2">
              <time className="shrink-0 text-xs text-gray-500 tabular-nums" dir="ltr">
                {new Date(e.at).toLocaleString()}
              </time>
              <span className={e.level === 'error' ? 'text-danger-600' : ''}>{e.message}</span>
            </li>
          ))}
        </ul>
      )}
    </Modal>
  )
}

type PluginFilter = 'all' | 'active' | 'inactive' | 'review'

function PluginStat({ label, value, tone, selected, onClick }: { label: string; value: number; tone: 'plain' | 'success' | 'warning'; selected: boolean; onClick: () => void }) {
  const tones = {
    plain: 'bg-white text-gray-900 ring-gray-200 dark:bg-white/5 dark:text-white dark:ring-white/10',
    success: 'bg-emerald-50/70 text-emerald-700 ring-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-400 dark:ring-emerald-500/20',
    warning: 'bg-amber-50/70 text-amber-700 ring-amber-100 dark:bg-amber-500/10 dark:text-amber-400 dark:ring-amber-500/20',
  }
  return (
    <button
      type="button"
      aria-pressed={selected}
      onClick={onClick}
      className={'rounded-xl p-4 text-start shadow-xs ring-1 transition ' + tones[tone] + (selected ? ' ring-2 ring-amber-400/70' : '')}
    >
      <span className="block text-sm opacity-80">{label}</span>
      <span className="mt-1 block text-2xl font-bold tabular-nums">{value}</span>
    </button>
  )
}

function PluginsPage() {
  const data = usePageData<PluginsData>()
  const { run, busy } = useRun()
  const [dialog, setDialog] = useState<'upload' | 'update' | null>(null)
  const [log, setLog] = useState<PluginRow | null>(null)
  const [filter, setFilter] = useState<PluginFilter>('all')
  const d = data.data
  const all = d?.plugins ?? []
  const total = all.length
  const active = all.filter((p) => p.status === 'active').length
  const inactive = total - active
  const review = all.filter((p) => p.boot_error).length
  const list = all.filter((p) => {
    if (filter === 'active') return p.status === 'active'
    if (filter === 'inactive') return p.status !== 'active'
    if (filter === 'review') return !!p.boot_error
    return true
  })

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="text-start">
          <h1 className="text-2xl font-bold tracking-tight">{L('مدیریت افزونه‌ها', 'Manage plugins')}</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {L(`${active} از ${total} افزونه فعال`, `${active} of ${total} plugins active`)}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className={btnAmber} onClick={() => setDialog('upload')}>
            <UploadIcon /> {L('بارگذاری افزونه', 'Upload plugin')}
          </button>
          <button type="button" className={btnBlue} disabled={busy} onClick={() => void run({ action: 'scan' })}>
            <ScanIcon /> {L('شناسایی افزونه‌ها', 'Detect plugins')}
          </button>
          {total > 0 && (
            <button type="button" className={btnAmber} onClick={() => setDialog('update')}>
              <RefreshIcon /> {L('به‌روزرسانی افزونه', 'Update plugin')}
            </button>
          )}
        </div>
      </header>

      <div className="flex items-center gap-2">
        <span className={installedPill}>
          <CheckCircleIcon className="size-4 text-gray-500 dark:text-gray-400" /> {L('نصب‌شده', 'Installed')}
        </span>
      </div>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <PluginStat label={L('همه', 'All')} value={total} tone="plain" selected={filter === 'all'} onClick={() => setFilter('all')} />
        <PluginStat label={L('فعال', 'Active')} value={active} tone="success" selected={filter === 'active'} onClick={() => setFilter('active')} />
        <PluginStat label={L('غیرفعال / در انتظار', 'Inactive / pending')} value={inactive} tone="warning" selected={filter === 'inactive'} onClick={() => setFilter('inactive')} />
        <PluginStat label={L('نیاز به بررسی', 'Needs review')} value={review} tone="plain" selected={filter === 'review'} onClick={() => setFilter('review')} />
      </div>

      {data.isPending || !d ? (
        <div className="h-48 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
      ) : (
        <div className="overflow-x-auto rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-900/70 dark:ring-white/10">
          <table className="w-full min-w-[880px] text-start text-sm">
            <thead className="border-b border-gray-100 bg-gray-50/70 text-xs text-gray-500 dark:border-white/5 dark:bg-white/5 dark:text-gray-400">
              <tr>
                <th className="px-5 py-3 text-start font-medium">{L('افزونه', 'Plugin')}</th>
                <th className="w-24 px-5 py-3 text-start font-medium">{L('نسخه', 'Version')}</th>
                <th className="w-36 px-5 py-3 text-start font-medium">{L('نویسنده', 'Author')}</th>
                <th className="w-24 px-5 py-3 text-start font-medium">{L('وضعیت', 'Status')}</th>
                <th className="w-80 px-5 py-3 text-start font-medium">{L('عملیات', 'Actions')}</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-white/5">
              {list.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-5 py-12 text-center text-sm text-gray-500 dark:text-gray-400">
                    {total === 0
                      ? L('هنوز افزونه‌ای نصب نشده است. یک فایل ZIP بارگذاری کنید.', 'No plugins are installed yet. Upload a ZIP file.')
                      : L('افزونه‌ای با این وضعیت نیست.', 'No plugins with this status.')}
                  </td>
                </tr>
              ) : (
                list.map((p) => {
                  const st = pluginStatus(p.status)
                  return (
                    <tr key={p.id} className="align-top hover:bg-gray-50/60 dark:hover:bg-white/[0.03]">
                      <td className="px-5 py-4">
                        <div className="flex items-start gap-3">
                          <span className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-white/5 dark:text-gray-300">
                            <PuzzleIcon />
                          </span>
                          <div className="min-w-0">
                            <p className="font-semibold text-gray-900 dark:text-white">{p.name}</p>
                            <code className="text-xs text-gray-400 dark:text-gray-500" dir="ltr">
                              {p.id}
                            </code>
                            {p.description && <p className="mt-1 line-clamp-2 text-xs leading-5 text-gray-500 dark:text-gray-400">{p.description}</p>}
                          </div>
                        </div>
                      </td>
                      <td className="px-5 py-4">{p.version ? <span className={versionChip}>v{p.version}</span> : <span className="text-gray-400">—</span>}</td>
                      <td className="px-5 py-4 text-gray-600 dark:text-gray-300">{p.author || <span className="text-gray-400">—</span>}</td>
                      <td className="px-5 py-4">
                        <span className={'inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium ring-1 ring-inset ' + st.cls}>
                          <span className={'size-1.5 rounded-full ' + st.dot} aria-hidden="true" />
                          {st.label}
                        </span>
                        {p.boot_error && (
                          <p className="mt-1 max-w-40 text-xs text-danger-600" role="alert">
                            {L(`راه‌اندازی نشد: ${p.boot_error}`, `Failed to start: ${p.boot_error}`)}
                          </p>
                        )}
                      </td>
                      <td className="px-5 py-4">
                        <div className="flex flex-wrap items-center gap-2">
                          {p.status === 'active' ? (
                            <button type="button" className={btnAmber} disabled={busy} onClick={() => void run({ action: 'deactivate', id: p.id })}>
                              <PauseIcon /> {L('غیرفعال‌سازی', 'Deactivate')}
                            </button>
                          ) : (
                            <button type="button" className={btnAmber} disabled={busy} onClick={() => void run({ action: 'activate', id: p.id })}>
                              {L('فعال‌سازی', 'Activate')}
                            </button>
                          )}
                          {p.previous_version && (
                            <button type="button" className={btnWhite} disabled={busy} onClick={() => void run({ action: 'rollback', id: p.id }, L(`به نسخهٔ ${p.previous_version} برگردد؟`, `Roll back to ${p.previous_version}?`))}>
                              <UndoIcon /> {L('بازگردانی', 'Roll back')}
                            </button>
                          )}
                          {(p.boot_error || (p.log?.length ?? 0) > 0) && (
                            <button type="button" className={btnIcon} title={L('گزارش', 'Log')} aria-label={L('گزارش', 'Log')} onClick={() => setLog(p)}>
                              <DocIcon />
                            </button>
                          )}
                          {p.status !== 'active' && (
                            <button
                              type="button"
                              className={btnIcon + ' hover:text-danger-600 dark:hover:text-danger-400'}
                              title={L('حذف', 'Delete')}
                              aria-label={L('حذف', 'Delete')}
                              disabled={busy}
                              onClick={() => {
                                if (!window.confirm(L(`افزونهٔ «${p.name}» و فایل‌هایش حذف شود؟`, `Delete the plugin "${p.name}" and its files?`))) return
                                const data = window.confirm(L('داده‌های این افزونه هم پاک شود؟ (تنظیماتش در سایت‌ها و مطالب انواع محتوایش)', "Also delete its data? (its settings on the sites and its content types' entries)"))
                                void run({ action: 'delete', id: p.id, delete_data: data })
                              }}
                            >
                              <TrashIcon />
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  )
                })
              )}
            </tbody>
          </table>
        </div>
      )}
      {dialog === 'upload' && <UploadDialog kind="plugin" page="plugins" update={false} onClose={() => setDialog(null)} />}
      {dialog === 'update' && <UploadDialog kind="plugin" page="plugins" update ids={all.map((p) => p.id)} onClose={() => setDialog(null)} />}
      {log && <LogDialog p={log} onClose={() => setLog(null)} />}
    </div>
  )
}

export function pluginsPage(options: { group?: string; navigationSort?: number } = {}) {
  return definePage({
    name: 'plugins',
    label: L('افزونه‌ها', 'Plugins'),
    icon: <PuzzleIcon />,
    group: options.group,
    navigationSort: options.navigationSort ?? 70,
    component: PluginsPage,
  })
}
