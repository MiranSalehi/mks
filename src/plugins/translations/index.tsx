import { useMemo, useState, type ReactNode } from 'react'
import { ApiError, Modal, definePage, definePlugin, messages, tr, usePageData, usePageSubmit, type CustomLocale, type Locale, type PanelPlugin } from '../../index'

const L = (fa: string, en: string) => tr({ fa, en })

const LangIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M4 5h9M8.5 3v2M6 5c0 4 2.5 7 6 8M11 5c-.5 3.5-3 6.5-7 8M13 21l4-9 4 9M14.5 17.5h5" />
  </svg>
)
const Icon = ({ d, className = 'size-4' }: { d: string; className?: string }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
    <path d={d} />
  </svg>
)
const CheckIcon = () => <Icon d="m5 12 4 4L19 6" />
const PlusIcon = () => <Icon d="M12 5v14M5 12h14" />
const TrashIcon = () => <Icon d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13" />
const ChevronIcon = () => <Icon d="m6 9 6 6 6-6" />
const SearchIcon = () => <Icon d="M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM20 20l-4-4" />

const input =
  'w-full rounded-lg border-0 bg-white px-3 py-2 text-sm shadow-xs ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5 dark:ring-white/10'
const amberButton = 'inline-flex items-center justify-center gap-1.5 rounded-md bg-amber-500 px-3 py-2 text-sm font-medium text-white shadow-sm hover:bg-amber-600 disabled:cursor-not-allowed disabled:opacity-50'
const outlineButton = 'inline-flex items-center justify-center gap-1.5 rounded-md bg-white px-3 py-1.5 text-sm font-medium text-gray-700 ring-1 ring-gray-200 hover:bg-gray-50 disabled:opacity-50 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10'

const KEY_RE = /^[a-zA-Z][a-zA-Z0-9_.]{0,119}$/

function text(m: unknown): string {
  if (typeof m === 'string') return m
  if (m && typeof m === 'object' && 'other' in m) return String((m as { other: string }).other)
  return ''
}

function errorText(e: unknown): string {
  if (e instanceof ApiError) return Object.values(e.errors)[0]?.[0] ?? e.message
  return e instanceof Error ? e.message : String(e)
}

function Field({ label, required, help, children }: { label: string; required?: boolean; help?: ReactNode; children: ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label className="block text-sm font-medium text-gray-900 dark:text-gray-100">
        {label}
        {required && <span className="text-danger-600"> *</span>}
      </label>
      {children}
      {help && <div className="text-xs leading-5 text-gray-500 dark:text-gray-400">{help}</div>}
    </div>
  )
}

function Select({ value, onChange, options, label }: { value: string; onChange: (value: string) => void; options: { value: string; label: string }[]; label: string }) {
  return (
    <div className="relative">
      <select aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} className={input + ' appearance-none pe-10'}>
        {options.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>
      <span className="pointer-events-none absolute inset-y-0 end-3 flex items-center text-gray-400">
        <ChevronIcon />
      </span>
    </div>
  )
}

interface Row {
  id: number
  key: string
  value: string
}

function AddLanguageModal({ locales, onClose }: { locales: CustomLocale[]; onClose: () => void }) {
  const submit = usePageSubmit()
  const [form, setForm] = useState({ code: '', name: '', direction: 'rtl', base: 'fa', tag: '' })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  const added = locales.filter((l) => !l.built_in)
  const set = (k: keyof typeof form) => (e: { target: { value: string } }) => setForm({ ...form, [k]: e.target.value })

  async function add() {
    setBusy(true)
    setErrors({})
    try {
      await submit({ action: 'add_locale', ...form })
      onClose()
    } catch (e) {
      if (e instanceof ApiError) setErrors(Object.fromEntries(Object.entries(e.errors).map(([k, v]) => [k, v[0] ?? ''])))
      else setErrors({ code: errorText(e) })
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      title={L('افزودن زبان', 'Add a language')}
      description={L('زبان تازه‌ای به پنل اضافه کنید؛ متن‌های ترجمه‌نشده به زبان پایه نشان داده می‌شوند.', 'Add a language to the panel; untranslated texts show in the base language.')}
      onClose={onClose}
      footer={
        <div className="flex justify-end gap-2">
          <button type="button" className={outlineButton} onClick={onClose}>
            {L('انصراف', 'Cancel')}
          </button>
          <button type="button" className={amberButton} disabled={busy || !form.code || !form.name} onClick={() => void add()}>
            {L('افزودن', 'Add')}
          </button>
        </div>
      }
    >
      <div className="space-y-4">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={L('کد زبان', 'Code')} required help={L('مثل ckb، ku یا ar_IQ', 'e.g. ckb, ku or ar_IQ')}>
            <input dir="ltr" className={input} value={form.code} onChange={set('code')} placeholder="ckb" maxLength={12} />
            {errors.code && <p className="text-xs text-danger-600" role="alert">{errors.code}</p>}
          </Field>
          <Field label={L('نام (به خود زبان)', 'Name (in the language)')} required>
            <input className={input} value={form.name} onChange={set('name')} placeholder="Kurdish" maxLength={60} />
            {errors.name && <p className="text-xs text-danger-600" role="alert">{errors.name}</p>}
          </Field>
          <Field label={L('جهت نوشتن', 'Direction')} required>
            <Select label={L('جهت نوشتن', 'Direction')} value={form.direction} onChange={(v) => setForm({ ...form, direction: v })} options={[{ value: 'rtl', label: L('راست به چپ', 'Right to left') }, { value: 'ltr', label: L('چپ به راست', 'Left to right') }]} />
          </Field>
          <Field label={L('زبان پایه', 'Base language')} required help={L('متن‌های ترجمه‌نشده به این زبان دیده می‌شوند.', 'Untranslated texts show in this language.')}>
            <Select label={L('زبان پایه', 'Base language')} value={form.base} onChange={(v) => setForm({ ...form, base: v })} options={[{ value: 'fa', label: 'فارسی' }, { value: 'en', label: 'English' }]} />
            {errors.base && <p className="text-xs text-danger-600" role="alert">{errors.base}</p>}
          </Field>
          <Field label={L('برچسب اعداد و تاریخ (اختیاری)', 'Number and date tag (optional)')} help={L('خالی: مثل زبان پایه', 'Empty: as the base')}>
            <input dir="ltr" className={input} value={form.tag} onChange={set('tag')} placeholder="ckb-IQ" maxLength={35} />
            {errors.tag && <p className="text-xs text-danger-600" role="alert">{errors.tag}</p>}
          </Field>
        </div>

        {added.length > 0 && (
          <div>
            <h3 className="mb-2 text-sm font-medium">{L('زبان‌های افزوده', 'Added languages')}</h3>
            <ul className="divide-y divide-gray-100 rounded-lg ring-1 ring-gray-200 dark:divide-white/5 dark:ring-white/10">
              {added.map((l) => (
                <li key={l.code} className="flex flex-wrap items-center gap-3 px-3 py-2 text-sm">
                  <span className="font-medium" lang={l.code.replace('_', '-')}>{l.name}</span>
                  <span dir="ltr" className="font-mono text-xs text-gray-500">{l.code}</span>
                  <span className="text-xs text-gray-500">
                    {l.direction === 'rtl' ? L('راست به چپ', 'RTL') : L('چپ به راست', 'LTR')} · {L('پایه', 'base')}: {l.base}
                  </span>
                  <button
                    type="button"
                    className="ms-auto rounded-md p-1 text-gray-400 hover:text-danger-600"
                    title={L('حذف زبان', 'Delete the language')}
                    aria-label={L('حذف زبان', 'Delete the language')}
                    onClick={() => {
                      if (window.confirm(L(`زبان «${l.name}» و همهٔ ترجمه‌هایش حذف شود؟`, `Delete "${l.name}" and all its texts?`))) void submit({ action: 'remove_locale', code: l.code })
                    }}
                  >
                    <TrashIcon />
                  </button>
                </li>
              ))}
            </ul>
          </div>
        )}
      </div>
    </Modal>
  )
}

function Editor({ overrides, locales }: { overrides: Record<string, Record<string, string>>; locales: CustomLocale[] }) {
  const submit = usePageSubmit()
  const [lang, setLang] = useState<Locale>(locales[0]?.code ?? 'fa')
  const [addOpen, setAddOpen] = useState(false)
  const [draft, setDraft] = useState<Record<string, string>>({})
  const [deleted, setDeleted] = useState<Set<string>>(new Set())
  const [added, setAdded] = useState<Row[]>([])
  const [nextID, setNextID] = useState(1)
  const [search, setSearch] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const info = locales.find((l) => l.code === lang)
  const dir = info?.direction ?? 'rtl'
  const builtIn = messages(info?.base ?? lang) as Record<string, unknown>
  const current = overrides[lang] ?? {}

  const keys = useMemo(() => {
    const base = Object.keys(builtIn)
    const extra = Object.keys(current).filter((k) => !(k in builtIn))
    return [...base, ...extra]
  }, [builtIn, current])

  const value = (k: string) => (k in draft ? draft[k] : (current[k] ?? text(builtIn[k])))
  const query = search.trim().toLowerCase()
  const shownKeys = query ? keys.filter((k) => k.toLowerCase().includes(query) || value(k).toLowerCase().includes(query) || text(builtIn[k]).toLowerCase().includes(query)) : keys
  const shownAdded = query ? added.filter((r) => r.key.toLowerCase().includes(query) || r.value.toLowerCase().includes(query)) : added
  const changed = Object.keys(draft).length + added.filter((r) => r.key.trim() !== '').length + deleted.size

  const path = info && !info.built_in ? `panel_translations / ${lang}` : `@mks/mks/src/locales/${lang}.ts`

  function changeLang(code: string) {
    setLang(code)
    setDraft({})
    setDeleted(new Set())
    setAdded([])
    setSearch('')
    setError('')
  }

  function remove(key: string) {
    if (key in builtIn) {
      // An empty value goes back to the built-in text.
      setDraft((all) => ({ ...all, [key]: '' }))
      return
    }
    const next = new Set(deleted)
    next.add(key)
    setDeleted(next)
    setDraft((all) => {
      const copy = { ...all }
      delete copy[key]
      return copy
    })
  }

  async function save() {
    const values: Record<string, string> = { ...draft }
    for (const key of deleted) values[key] = ''
    for (const row of added) {
      const key = row.key.trim()
      if (!key) continue
      if (!KEY_RE.test(key)) {
        setError(L(`کلید «${key}» معتبر نیست؛ با حرف شروع شود و فقط حرف، عدد، نقطه و زیرخط داشته باشد.`, `The key "${key}" is invalid; start with a letter and use letters, digits, dots and underscores.`))
        return
      }
      if (key in builtIn || key in current || key in values) {
        setError(L(`کلید «${key}» از قبل هست.`, `The key "${key}" already exists.`))
        return
      }
      values[key] = row.value
    }
    if (Object.keys(values).length === 0) return
    setSaving(true)
    setError('')
    try {
      await submit({ locale: lang, values })
      setDraft({})
      setDeleted(new Set())
      setAdded([])
    } catch (e) {
      setError(errorText(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div className="text-start">
          <h1 className="text-2xl font-bold tracking-tight text-gray-900 dark:text-white">{L('ترجمه‌ها', 'Translations')}</h1>
          <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{L('ویرایش متن‌های پنل به‌ازای هر زبان.', 'Edit the panel texts for each language.')}</p>
          <p className="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
            {L('مسیر', 'Path')}: <code dir="ltr">panel_translations</code>
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className={amberButton} onClick={() => setAddOpen(true)}>
            <PlusIcon /> {L('افزودن زبان', 'Add language')}
          </button>
          <button type="button" className={amberButton} disabled={saving} onClick={() => void save()}>
            <CheckIcon /> {L('ذخیره ترجمه‌ها', 'Save translations')}
          </button>
        </div>
      </header>

      <div className="space-y-5">
        <Field label={L('زبان', 'Language')} required>
          <Select label={L('زبان', 'Language')} value={lang} onChange={changeLang} options={locales.map((l) => ({ value: l.code, label: l.name }))} />
        </Field>

        <Field label={L('منبع ترجمه', 'Translation source')} required help={L('با ذخیره، متن‌ها در جدول panel_translations ذخیره می‌شوند و بلافاصله در پنل اعمال می‌شوند.', 'Saving writes the texts to the panel_translations table and they apply to the panel at once.')}>
          <Select label={L('منبع ترجمه', 'Translation source')} value="panel" onChange={() => undefined} options={[{ value: 'panel', label: L('پنل مدیریت', 'Admin panel') }]} />
        </Field>

        <Field
          label={L('فایل ترجمه', 'Translation file')}
          required
          help={
            <>
              <code dir="ltr">{path}</code>
              <span className="ms-2">{L(`${keys.length} کلید`, `${keys.length} keys`)}</span>
            </>
          }
        >
          <Select label={L('فایل ترجمه', 'Translation file')} value="messages" onChange={() => undefined} options={[{ value: 'messages', label: 'messages' }]} />
        </Field>
      </div>

      <section className="space-y-2">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <h2 className="text-sm font-medium text-gray-900 dark:text-white">{L('رشته‌ها', 'Strings')}</h2>
          <label className="relative block w-full max-w-xs">
            <span className="pointer-events-none absolute inset-y-0 start-2.5 flex items-center text-gray-400" aria-hidden="true">
              <SearchIcon />
            </span>
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder={L('جستجو در کلید یا متن…', 'Search keys or texts…')}
              aria-label={L('جستجو در رشته‌ها', 'Search strings')}
              className={input + ' ps-8 py-1.5'}
            />
          </label>
        </div>

        <div className="overflow-hidden rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-900/70 dark:ring-white/10">
          <div className="max-h-[560px] overflow-y-auto">
            <table className="w-full text-sm">
              <thead className="sticky top-0 z-10 bg-gray-50 text-xs text-gray-500 dark:bg-white/5 dark:text-gray-400">
                <tr>
                  <th className="w-1/3 px-4 py-2.5 text-start font-medium">{L('کلید', 'Key')}</th>
                  <th className="px-4 py-2.5 text-start font-medium">{L('ترجمه', 'Translation')}</th>
                  <th className="w-12 px-3 py-2.5" aria-label={L('حذف', 'Delete')} />
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                {shownKeys.map((key) => (
                  <tr key={key} className="hover:bg-gray-50/60 dark:hover:bg-white/[0.03]">
                    <td className="px-4 py-1.5 align-middle">
                      <code dir="ltr" className="block truncate text-end font-mono text-xs text-gray-700 dark:text-gray-300" title={key}>
                        {key}
                      </code>
                    </td>
                    <td className="px-4 py-1.5">
                      <input
                        dir={dir}
                        value={value(key)}
                        placeholder={text(builtIn[key])}
                        onChange={(e) => setDraft((all) => ({ ...all, [key]: e.target.value }))}
                        aria-label={L(`ترجمهٔ ${key}`, `Translation of ${key}`)}
                        className="w-full rounded-md border-0 bg-transparent px-2 py-1 text-sm focus:ring-2 focus:ring-primary-500 focus:outline-none"
                      />
                    </td>
                    <td className="px-3 py-1.5 text-center">
                      <button type="button" onClick={() => remove(key)} className="rounded-md p-1 text-danger-500 hover:bg-danger-50 hover:text-danger-600 dark:hover:bg-danger-500/10" title={L('حذف رشته', 'Delete the string')} aria-label={L(`حذف ${key}`, `Delete ${key}`)}>
                        <TrashIcon />
                      </button>
                    </td>
                  </tr>
                ))}
                {shownAdded.map((row) => (
                  <tr key={`new-${row.id}`} className="bg-primary-50/30 dark:bg-primary-500/5">
                    <td className="px-4 py-1.5">
                      <input
                        dir="ltr"
                        value={row.key}
                        onChange={(e) => setAdded((all) => all.map((r) => (r.id === row.id ? { ...r, key: e.target.value } : r)))}
                        placeholder={L('کلید تازه', 'New key')}
                        aria-label={L('کلید تازه', 'New key')}
                        className="w-full rounded-md border-0 bg-transparent px-2 py-1 text-end font-mono text-xs ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:ring-white/10"
                      />
                    </td>
                    <td className="px-4 py-1.5">
                      <input
                        dir={dir}
                        value={row.value}
                        onChange={(e) => setAdded((all) => all.map((r) => (r.id === row.id ? { ...r, value: e.target.value } : r)))}
                        placeholder={L('ترجمه', 'Translation')}
                        aria-label={L('ترجمه', 'Translation')}
                        className="w-full rounded-md border-0 bg-transparent px-2 py-1 text-sm ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:ring-white/10"
                      />
                    </td>
                    <td className="px-3 py-1.5 text-center">
                      <button type="button" onClick={() => setAdded((all) => all.filter((r) => r.id !== row.id))} className="rounded-md p-1 text-danger-500 hover:bg-danger-50 hover:text-danger-600 dark:hover:bg-danger-500/10" title={L('حذف رشته', 'Delete the string')} aria-label={L('حذف رشته', 'Delete the string')}>
                        <TrashIcon />
                      </button>
                    </td>
                  </tr>
                ))}
                {shownKeys.length === 0 && shownAdded.length === 0 && (
                  <tr>
                    <td colSpan={3} className="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
                      {L('رشته‌ای با این جستجو نیست.', 'No string matches this search.')}
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
          <button
            type="button"
            onClick={() => {
              setAdded((all) => [...all, { id: nextID, key: '', value: '' }])
              setNextID((n) => n + 1)
            }}
            className="flex w-full items-center justify-center gap-1.5 border-t border-gray-100 py-2.5 text-sm font-medium text-primary-600 hover:bg-gray-50 dark:border-white/5 dark:text-primary-400 dark:hover:bg-white/5"
          >
            <PlusIcon /> {L('افزودن رشته', 'Add a string')}
          </button>
        </div>
        {error && (
          <p className="text-sm text-danger-600" role="alert">
            {error}
          </p>
        )}
        {changed > 0 && <p className="text-xs text-gray-500 dark:text-gray-400">{L(`${changed} تغییر ذخیره نشده`, `${changed} unsaved changes`)}</p>}
      </section>

      {addOpen && <AddLanguageModal locales={locales} onClose={() => setAddOpen(false)} />}
    </div>
  )
}

function TranslationsPage() {
  const data = usePageData<{ overrides: Record<string, Record<string, string>>; locales: CustomLocale[] }>()
  const locales = data.data?.locales ?? []
  return data.isPending ? (
    <div className="h-64 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
  ) : (
    <Editor key={JSON.stringify([data.data?.overrides ?? {}, locales.map((l) => l.code)])} overrides={data.data?.overrides ?? {}} locales={locales} />
  )
}

/** The panel texts editor ("translations" page). Install the Go plugin (translations.Plugin) too. */
export function translations(options: { group?: string; navigationSort?: number } = {}): PanelPlugin {
  return definePlugin({
    name: 'translations',
    pages: [definePage({ name: 'translations', label: tr({ fa: 'ترجمه‌ها', en: 'Translations' }), icon: <LangIcon />, group: options.group, navigationSort: options.navigationSort ?? 10, component: TranslationsPage })],
  })
}
