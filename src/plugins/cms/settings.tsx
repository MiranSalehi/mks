import { useState } from 'react'
import { Card, definePage, PageHeader, PanelForm, tr, usePageData, usePageSubmit, usePanel, type FormBuilder, type SchemaNode } from '../../index'
import { mediaPicker } from './media'
import { field, type FieldDef } from './builder'

const GearIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6zM19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
  </svg>
)

/** A settings tab a plugin adds (MKSine's SettingsTabManager); its keys are registered in Go. */
export interface SettingsTab {
  id: string
  label: string
  schema: (f: FormBuilder<any>) => SchemaNode<any>[]
}

interface SettingsData {
  values: Record<string, unknown>
  /** Settings of the active ZIP plugins, one tab each. */
  package_tabs?: { id: string; label: string; fields: FieldDef[] }[]
  defaults: { site_name: string; permalinks: Record<string, string> }
}

const permalinkFields: [string, string][] = [
  ['permalink_posts', tr({ fa: 'فهرست نوشته‌ها', en: 'Post list' })],
  ['permalink_post', tr({ fa: 'نوشته', en: 'Post' })],
  ['permalink_categories', tr({ fa: 'فهرست دسته‌بندی‌ها', en: 'Category list' })],
  ['permalink_category', tr({ fa: 'دسته‌بندی', en: 'Category' })],
  ['permalink_tags', tr({ fa: 'فهرست برچسب‌ها', en: 'Tag list' })],
  ['permalink_tag', tr({ fa: 'برچسب', en: 'Tag' })],
  ['permalink_page', tr({ fa: 'برگه', en: 'Page' })],
  ['permalink_author', tr({ fa: 'نویسنده', en: 'Author' })],
]

/** Empties the site's page cache right away. */
function ClearCacheButton() {
  const { api, notify } = usePanel()
  const [busy, setBusy] = useState(false)
  return (
    <button
      type="button"
      disabled={busy}
      onClick={async () => {
        setBusy(true)
        try {
          const r = await api.plugin<{ message: string }>('cms', '/cache/clear', { method: 'POST' })
          notify(r.message, 'success')
        } finally {
          setBusy(false)
        }
      }}
      className="rounded-lg bg-white px-3 py-1.5 text-sm font-semibold shadow-xs ring-1 ring-gray-950/10 hover:bg-gray-50 disabled:opacity-50 dark:bg-white/5 dark:ring-white/10"
    >
      {tr({ fa: 'پاک کردن cache صفحه‌ها', en: 'Clear page cache' })}
    </button>
  )
}

/** Where the settings pages go: a cluster (e.g. "settings") or a sidebar group. */
export interface SettingsPlacement {
  cluster?: string
  group?: string
}

type Section = 'general' | 'permalinks' | 'performance' | 'extensions'

const sections: Record<Section, { name: string; label: string; sort: number; description: string }> = {
  general: {
    name: 'site_settings',
    label: tr({ fa: 'عمومی', en: 'General' }),
    sort: 10,
    description: tr({ fa: 'نام، شعار، نشان، آیکن و تقویم سایت.', en: 'Site name, tagline, logo, icon and calendar.' }),
  },
  permalinks: {
    name: 'site_permalinks',
    label: tr({ fa: 'نشانی‌های ثابت', en: 'Permalinks' }),
    sort: 20,
    description: tr({ fa: 'صفحهٔ اصلی و الگوی نشانی نوشته‌ها، دسته‌ها، برچسب‌ها و صفحات.', en: 'The front page and the address patterns of posts, categories, tags and pages.' }),
  },
  performance: {
    name: 'site_performance',
    label: tr({ fa: 'عملکرد', en: 'Performance' }),
    sort: 25,
    description: tr({ fa: 'cache صفحه‌ها برای مهمان‌ها.', en: 'The page cache for guests.' }),
  },
  extensions: {
    name: 'site_extensions',
    label: tr({ fa: 'افزونه‌ها', en: 'Extensions' }),
    sort: 200,
    description: tr({ fa: 'تنظیماتی که افزونه‌ها اضافه کرده‌اند.', en: 'Settings that plugins add.' }),
  },
}

function fieldsOf(section: Section, f: FormBuilder<any>, d: SettingsData, extraTabs: SettingsTab[]): SchemaNode<any>[] {
  switch (section) {
    case 'general':
      return [
        f.textInput('site_name').label(tr({ fa: 'نام سایت', en: 'Site name' })).maxLength(255)
          .placeholder(d.defaults.site_name)
          .helperText(tr({ fa: 'خالی: نام تیم.', en: "Empty: the team's name." })),
        f.textInput('short_site_name').label(tr({ fa: 'نام کوتاه', en: 'Short name' })).maxLength(100),
        f.textarea('tagline').label(tr({ fa: 'شعار یا توضیح کوتاه', en: 'Tagline' })).rows(2).columnSpan('full'),
        mediaPicker('logo').label(tr({ fa: 'نشان (لوگو)', en: 'Logo' })),
        mediaPicker('favicon').label(tr({ fa: 'آیکن مرورگر', en: 'Favicon' })),
        f.radio('date_calendar').label(tr({ fa: 'تقویم تاریخ‌ها', en: 'Calendar' }))
          .options({ shamsi: tr({ fa: 'شمسی', en: 'Solar Hijri' }), gregorian: tr({ fa: 'میلادی', en: 'Gregorian' }) })
          .default('shamsi').inline(),
      ]
    case 'permalinks':
      return [
        f.select('front_page_id').label(tr({ fa: 'صفحهٔ اصلی', en: 'Front page' })).optionsFrom('pages').searchable()
          .helperText(tr({ fa: 'خالی: برگه‌ای با نامک home، وگرنه آخرین نوشته‌ها.', en: 'Empty: a page with the slug "home", else the latest posts.' }))
          .columnSpan('full'),
        ...permalinkFields.map(([key, label]) => f.textInput(key).label(label).dir('ltr').placeholder(d.defaults.permalinks[key] ?? '')),
      ]
    case 'performance':
      return [
        f.toggle('page_cache_enabled').label(tr({ fa: 'cache صفحه‌ها برای مهمان‌ها', en: 'Page cache for guests' })),
        f.textInput('page_cache_ttl_minutes').label(tr({ fa: 'ماندگاری (دقیقه)', en: 'Lifetime (minutes)' })).numeric()
          .placeholder('60').visible(({ get }) => !!get('page_cache_enabled')),
        f.placeholder('cache_clear').label(tr({ fa: 'پاک کردن', en: 'Clear' }))
          .content(() => <ClearCacheButton />)
          .helperText(tr({ fa: 'با هر ذخیره در محتوا، منو، رسانه یا تنظیمات خودکار پاک می‌شود.', en: 'Cleared automatically by every save of content, menus, media or settings.' })),
      ]
    case 'extensions': {
      const tabs = [
        ...extraTabs.map((tab) => f.tab(tab.label).schema(tab.schema(f))),
        ...(d.package_tabs ?? []).map((tab) => f.tab(tab.label).schema(tab.fields.map((def) => field(f, def)))),
      ]
      return tabs.length > 0 ? [f.tabs(tabs)] : [f.placeholder('none').content(() => tr({ fa: 'هیچ افزونه‌ای تنظیماتی ندارد.', en: 'No plugin has settings.' }))]
    }
  }
}

/** The keys a section's form sends: only its own, so pages never overwrite each other. */
function keysOf(section: Section, values: Record<string, unknown>): string[] | null {
  switch (section) {
    case 'general':
      return ['site_name', 'short_site_name', 'tagline', 'logo', 'favicon', 'date_calendar']
    case 'permalinks':
      return ['front_page_id', ...permalinkFields.map(([k]) => k)]
    case 'performance':
      return ['page_cache_enabled', 'page_cache_ttl_minutes']
    case 'extensions':
      return Object.keys(values).filter((k) => !coreKeys.has(k))
  }
}

const coreKeys = new Set(['site_name', 'short_site_name', 'tagline', 'logo', 'favicon', 'date_calendar', 'front_page_id', 'page_cache_enabled', 'page_cache_ttl_minutes', 'theme', 'custom_css', 'custom_js', ...permalinkFields.map(([k]) => k)])

function settingsSection(section: Section, extraTabs: SettingsTab[], placement: SettingsPlacement) {
  const info = sections[section]
  function SettingsSectionPage() {
    const data = usePageData<SettingsData>()
    const submit = usePageSubmit()
    const d = data.data
    return (
      <div className="space-y-6">
        <PageHeader title={info.label} description={info.description} />
        {data.error && <p className="text-sm text-danger-600">{data.error.message}</p>}
        {data.isPending || !d ? (
          <div className="h-64 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
        ) : (
          <Card>
            <PanelForm
              key={JSON.stringify(d.values)}
              initial={d.values}
              columns={2}
              onSubmit={(values) => {
                const keys = keysOf(section, values)
                return submit(keys ? Object.fromEntries(keys.filter((k) => k in values).map((k) => [k, values[k]])) : values)
              }}
              schema={(f) => fieldsOf(section, f, d, extraTabs)}
            />
          </Card>
        )}
      </div>
    )
  }
  return definePage({
    name: info.name,
    label: placement.cluster ? info.label : tr({ fa: `تنظیمات: ${info.label}`, en: `Settings: ${info.label}` }),
    icon: <GearIcon />,
    group: placement.group,
    cluster: placement.cluster,
    navigationSort: info.sort,
    // Extensions only when a plugin adds settings (MKSine hides it otherwise).
    visible: section === 'extensions' ? (meta) => extraTabs.length > 0 || meta.extra?.cms_extension_settings === true : undefined,
    component: SettingsSectionPage,
  })
}

/** The settings pages, as MKSine splits them; "Extensions" shows when plugins add settings. */
function settingsPages(extraTabs: SettingsTab[], placement: SettingsPlacement = {}) {
  const list: Section[] = ['general', 'permalinks', 'performance', 'extensions']
  return list.map((s) => settingsSection(s, extraTabs, placement))
}

export { settingsPages }
