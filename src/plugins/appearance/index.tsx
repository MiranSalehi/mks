import { useEffect, useState } from 'react'
import {
  ApiError,
  Card,
  definePage,
  definePlugin,
  PageHeader,
  PRESETS,
  PRIMARIES,
  resolveAppearance,
  setAppearance,
  tr,
  usePageData,
  usePageSubmit,
  type Appearance,
  type PanelPlugin,
  type Preset,
} from '../../index'

// The panel look editor (MKSine's admin appearance): every change previews at once; save
// it for yourself, or (managers) for everyone or under a name.

interface Named {
  name: string
  look: Appearance
}

interface Data {
  company: Appearance | null
  user: Appearance | null
  saved: Named[]
  can_manage: boolean
}

const PaletteIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M12 3a9 9 0 1 0 0 18c1 0 1.5-.8 1.5-1.6 0-.9-.6-1.4-.6-2.2 0-.9.7-1.6 1.6-1.6H16a5 5 0 0 0 5-5c0-4.1-4-7.6-9-7.6z" />
    <circle cx="7.5" cy="11" r="1" />
    <circle cx="10" cy="7" r="1" />
    <circle cx="14.5" cy="7" r="1" />
  </svg>
)

const L = (fa: string, en: string) => tr({ fa, en })

const presetLabels: Record<Preset, string> = {
  modern: L('مدرن', 'Modern'),
  classic: L('کلاسیک', 'Classic'),
  minimal: L('مینیمال', 'Minimal'),
  soft: L('نرم', 'Soft'),
  bold: L('پررنگ', 'Bold'),
}

const swatches: Record<string, string> = {
  iris: 'oklch(0.552 0.228 275)',
  indigo: 'oklch(0.511 0.262 276.966)',
  blue: 'oklch(0.546 0.245 262.881)',
  cyan: 'oklch(0.609 0.126 221.723)',
  emerald: 'oklch(0.596 0.145 163.225)',
  violet: 'oklch(0.541 0.281 293.009)',
  orange: 'oklch(0.646 0.222 41.116)',
  fuchsia: 'oklch(0.591 0.293 322.896)',
  slate: 'oklch(0.446 0.043 257.281)',
}

const parts: { key: keyof Appearance; label: string; options: Record<string, string> }[] = [
  { key: 'mode', label: L('حالت رنگ', 'Color mode'), options: { system: L('مثل سیستم', 'System'), light: L('روشن', 'Light'), dark: L('تیره', 'Dark') } },
  { key: 'rail', label: L('منوی کناری', 'Sidebar'), options: { light: L('روشن', 'Light'), dark: L('تیره', 'Dark'), brand: L('رنگ اصلی', 'Brand') } },
  { key: 'radius', label: L('گوشه‌ها', 'Corners'), options: { sharp: L('تیز', 'Sharp'), medium: L('متوسط', 'Medium'), round: L('گرد', 'Round') } },
  { key: 'font', label: L('اندازهٔ نوشته', 'Text size'), options: { small: L('کوچک', 'Small'), normal: L('عادی', 'Normal'), large: L('بزرگ', 'Large') } },
  { key: 'density', label: L('فشردگی', 'Density'), options: { comfortable: L('راحت', 'Comfortable'), compact: L('فشرده', 'Compact') } },
  { key: 'field', label: L('سبک فیلدها', 'Field style'), options: { outlined: L('قاب‌دار', 'Outlined'), filled: L('توپر', 'Filled'), underline: L('خط زیرین', 'Underline'), floating: L('برچسب شناور', 'Floating label') } },
  { key: 'label', label: L('جای برچسب‌ها', 'Labels'), options: { top: L('بالای فیلد', 'Above'), side: L('کنار فیلد', 'Beside') } },
]

const chip = (on: boolean) =>
  'rounded-lg px-3 py-1.5 text-sm ring-1 ' +
  (on ? 'bg-primary-50 font-semibold text-primary-700 ring-primary-600/30 dark:bg-primary-500/10 dark:text-primary-300' : 'ring-gray-950/10 hover:bg-gray-50 dark:ring-white/10 dark:hover:bg-white/5')
const button = 'rounded-lg px-4 py-2 text-sm font-semibold disabled:opacity-50'

type Mode = 'user' | 'company'

function Editor({ d, mode, page }: { d: Data; mode: Mode; page: string }) {
  const submit = usePageSubmit(page)
  const start = mode === 'user' ? (d.user ?? d.company ?? {}) : (d.company ?? {})
  const [look, setLook] = useState<Appearance>(start)
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const resolved = resolveAppearance(look)

  // Every change previews at once; leaving puts the saved look back.
  useEffect(() => setAppearance(look), [look])
  useEffect(() => () => setAppearance(d.user ?? d.company ?? null), [d])

  const set = (k: keyof Appearance, v: string) => setLook((l) => ({ ...l, [k]: v }))
  const choosePreset = (p: Preset) => setLook((l) => ({ preset: p, mode: l.mode }))

  async function send(data: Record<string, unknown>) {
    setBusy(true)
    setError('')
    try {
      await submit(data)
    } catch (e) {
      setError(e instanceof ApiError ? (Object.values(e.errors)[0]?.[0] ?? e.message) : String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <>
      <Card heading={L('پیش‌تنظیم', 'Preset')}>
        <div className="flex flex-wrap gap-2">
          {(Object.keys(PRESETS) as Preset[]).map((p) => (
            <button key={p} type="button" aria-pressed={resolved.preset === p} className={chip(resolved.preset === p)} onClick={() => choosePreset(p)}>
              {presetLabels[p]}
            </button>
          ))}
        </div>
      </Card>
      <Card heading={L('جزئیات', 'Details')}>
        <div className="space-y-5">
          <div>
            <p className="mb-2 text-sm font-medium">{L('رنگ اصلی', 'Primary color')}</p>
            <div className="flex flex-wrap gap-2" role="radiogroup" aria-label={L('رنگ اصلی', 'Primary color')}>
              {PRIMARIES.map((c) => (
                <button
                  key={c}
                  type="button"
                  role="radio"
                  aria-checked={resolved.primary === c}
                  aria-label={c}
                  title={c}
                  onClick={() => set('primary', c)}
                  className={'size-9 rounded-full ring-offset-2 ring-offset-white dark:ring-offset-gray-900 ' + (resolved.primary === c ? 'ring-2 ring-gray-900 dark:ring-white' : '')}
                  style={{ background: swatches[c] }}
                />
              ))}
            </div>
          </div>
          {parts.map((part) => {
            const current = part.key === 'mode' ? (look.mode ?? 'system') : String(resolved[part.key as keyof typeof resolved])
            return (
              <div key={part.key}>
                <p className="mb-2 text-sm font-medium">{part.label}</p>
                <div className="flex flex-wrap gap-2" role="radiogroup" aria-label={part.label}>
                  {Object.entries(part.options).map(([v, label]) => (
                    <button key={v} type="button" role="radio" aria-checked={current === v} className={chip(current === v)} onClick={() => set(part.key, v)}>
                      {label}
                    </button>
                  ))}
                </div>
              </div>
            )
          })}
        </div>
      </Card>
      <Card>
        {mode === 'user' ? (
          <div className="flex flex-wrap items-center gap-2">
            <button type="button" disabled={busy} onClick={() => void send({ action: 'save_user', look })} className={button + ' bg-primary-600 text-white hover:bg-primary-500'}>
              {L('ذخیره برای من', 'Save for me')}
            </button>
            {d.user && (
              <button type="button" disabled={busy} onClick={() => void send({ action: 'reset_user' }).then(() => setLook(d.company ?? {}))} className={button + ' ring-1 ring-gray-950/10 dark:ring-white/10'}>
                {L('برگشت به ظاهر سازمان', 'Back to the company look')}
              </button>
            )}
          </div>
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-2">
              <button type="button" disabled={busy} onClick={() => void send({ action: 'save_company', look })} className={button + ' bg-primary-600 text-white hover:bg-primary-500'}>
                {L('ذخیره برای همه (ظاهر سازمان)', 'Save for everyone (company look)')}
              </button>
            </div>
            <div className="mt-4 flex flex-wrap items-center gap-2">
              <input
                value={name}
                onChange={(e) => setName(e.target.value)}
                maxLength={40}
                placeholder={L('نام ظاهر', 'Look name')}
                aria-label={L('نام ظاهر', 'Look name')}
                className="rounded-lg border-0 bg-white px-3 py-1.5 text-sm shadow-xs ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5 dark:ring-white/10"
              />
              <button type="button" disabled={busy || !name.trim()} onClick={() => void send({ action: 'save_named', name, look }).then(() => setName(''))} className={button + ' ring-1 ring-gray-950/10 dark:ring-white/10'}>
                {L('ذخیره با این نام', 'Save under this name')}
              </button>
            </div>
          </>
        )}
        {error && <p className="mt-3 text-sm text-danger-600" role="alert">{error}</p>}
      </Card>
      {d.saved.length > 0 && (
        <Card heading={L('ظاهرهای نام‌دار', 'Saved looks')}>
          <ul className="flex flex-wrap gap-2">
            {d.saved.map((s) => (
              <li key={s.name} className="flex items-center gap-1 rounded-lg ring-1 ring-gray-950/10 dark:ring-white/10">
                <button type="button" className="px-3 py-1.5 text-sm" onClick={() => setLook(s.look)}>
                  <span className="me-2 inline-block size-3 rounded-full align-middle" style={{ background: swatches[resolveAppearance(s.look).primary] }} />
                  {s.name}
                </button>
                {mode === 'company' && (
                  <button type="button" aria-label={L(`حذف ${s.name}`, `Delete ${s.name}`)} className="px-2 text-gray-400 hover:text-danger-600" onClick={() => void send({ action: 'delete_named', name: s.name })}>
                    ×
                  </button>
                )}
              </li>
            ))}
          </ul>
        </Card>
      )}
    </>
  )
}

/** Settings › Appearance: the company look and the named looks (managers). */
function CompanyAppearancePage() {
  const data = usePageData<Data>()
  return (
    <div className="space-y-6">
      <PageHeader
        title={L('ظاهر', 'Appearance')}
        description={L('ظاهر سازمان: رنگ، گوشه‌ها، اندازهٔ نوشته، فشردگی، منوی کناری و سبک فیلدها برای همه. هر تغییر همین‌جا دیده می‌شود.', 'The company look for everyone. Every change shows at once.')}
      />
      {data.isPending || !data.data ? (
        <div className="h-64 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
      ) : (
        <Editor key={JSON.stringify(data.data)} d={data.data} mode="company" page="company_appearance" />
      )}
    </div>
  )
}

/** "My appearance" in the user menu: the user's own look. */
function MyAppearance() {
  const data = usePageData<Data>('appearance')
  return (
    <div className="space-y-4">
      {data.isPending || !data.data ? (
        <div className="h-64 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
      ) : (
        <Editor key={JSON.stringify(data.data)} d={data.data} mode="user" page="appearance" />
      )}
    </div>
  )
}

const SwatchIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-4">
    <path d="M4 20h7V4H4zM11 8l5-3 4 7-9 5M11 20l9-5" />
  </svg>
)

/**
 * The panel look (Install the Go plugin, appearance.Plugin, too): the company look as a
 * page (in a cluster such as "settings", or a sidebar group) and "My appearance" in the
 * user menu.
 */
export function appearance(options: { group?: string; cluster?: string; navigationSort?: number } = {}): PanelPlugin {
  return definePlugin({
    name: 'appearance',
    pages: [
      definePage({
        name: 'company_appearance',
        label: L('ظاهر', 'Appearance'),
        icon: <PaletteIcon />,
        group: options.group,
        cluster: options.cluster,
        navigationSort: options.navigationSort ?? 15,
        component: CompanyAppearancePage,
      }),
    ],
    userMenuItems: [{ key: 'my_appearance', label: L('ظاهر من', 'My appearance'), icon: <SwatchIcon />, sort: 25, component: MyAppearance }],
  })
}
