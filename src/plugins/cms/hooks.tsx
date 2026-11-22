import { useState } from 'react'
import { Card, definePage, formatNumber, PageHeader, tr, usePageData, usePageSubmit } from '../../index'

// The hooks page (MKSine's hook management): turn listeners, filters and template actions
// off, or change their order. The settings apply to every site.

interface Hook {
  kind: 'event' | 'filter' | 'action'
  name: string
  id: string
  priority: number
  enabled: boolean
  override: number | null
  system: boolean
}

const HookIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M7 4v9a5 5 0 0 0 10 0V9M17 9l-3 3M17 9l3 3" />
  </svg>
)

const kinds: Record<Hook['kind'], string> = {
  event: tr({ fa: 'رویدادها', en: 'Events' }),
  filter: tr({ fa: 'فیلترها', en: 'Filters' }),
  action: tr({ fa: 'نقاط قالب', en: 'Template actions' }),
}

const keyOf = (h: Hook) => `${h.kind}\u0000${h.name}\u0000${h.id}`

function HooksPage() {
  const data = usePageData<{ hooks: Hook[] }>()
  const submit = usePageSubmit()
  const [edits, setEdits] = useState<Record<string, { enabled: boolean; priority: number | null }>>({})
  const [busy, setBusy] = useState(false)
  const hooks = data.data?.hooks ?? []
  const value = (h: Hook) => edits[keyOf(h)] ?? { enabled: h.enabled, priority: h.override }
  const change = (h: Hook, next: Partial<{ enabled: boolean; priority: number | null }>) =>
    setEdits((all) => ({ ...all, [keyOf(h)]: { ...value(h), ...next } }))
  const changed = Object.keys(edits).length

  async function save() {
    setBusy(true)
    try {
      const rows = hooks.filter((h) => edits[keyOf(h)]).map((h) => ({ kind: h.kind, name: h.name, id: h.id, ...edits[keyOf(h)] }))
      await submit({ hooks: rows })
      setEdits({})
    } finally {
      setBusy(false)
    }
  }

  const input = 'w-20 rounded-lg border-0 bg-white px-2 py-1 text-sm shadow-xs ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5 dark:ring-white/10'
  return (
    <div className="space-y-6">
      <PageHeader
        title={tr({ fa: 'Hookها', en: 'Hooks' })}
        description={tr({
          fa: 'رویدادها، فیلترها و نقاط قالبی که برنامه و پلاگین‌ها ثبت کرده‌اند. خاموش کردن یا تغییر اولویت برای همهٔ سایت‌هاست؛ اولویت کمتر زودتر اجرا می‌شود.',
          en: 'Events, filters and template actions registered by the app and plugins. Turning one off or reordering applies to every site; a lower priority runs first.',
        })}
      />
      {data.isPending ? (
        <div className="h-48 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
      ) : hooks.length === 0 ? (
        <Card>
          <p className="text-sm text-gray-500">{tr({ fa: 'هنوز hookی ثبت نشده است.', en: 'No hooks are registered.' })}</p>
        </Card>
      ) : (
        <>
          {(Object.keys(kinds) as Hook['kind'][]).map((kind) => {
            const list = hooks.filter((h) => h.kind === kind)
            if (list.length === 0) return null
            return (
              <Card key={kind} heading={kinds[kind]}>
                <table className="w-full text-sm">
                  <thead className="text-xs text-gray-500">
                    <tr>
                      <th className="py-1 text-start">{tr({ fa: 'نام', en: 'Name' })}</th>
                      <th className="py-1 text-start">{tr({ fa: 'شناسه', en: 'Id' })}</th>
                      <th className="py-1 text-start">{tr({ fa: 'اولویت', en: 'Priority' })}</th>
                      <th className="py-1 text-start">{tr({ fa: 'روشن', en: 'On' })}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {list.map((h) => {
                      const v = value(h)
                      return (
                        <tr key={keyOf(h)} className="border-t border-gray-950/5 dark:border-white/5">
                          <td className="py-2 pe-2 font-mono text-xs" dir="ltr">{h.name}</td>
                          <td className="py-2 pe-2 font-mono text-xs text-gray-500" dir="ltr">
                            {h.id}
                            {h.system && <span className="ms-2 rounded bg-gray-100 px-1.5 text-[10px] dark:bg-white/10">system</span>}
                          </td>
                          <td className="py-2 pe-2">
                            <input
                              type="number"
                              className={input}
                              dir="ltr"
                              aria-label={tr({ fa: `اولویت ${h.id}`, en: `Priority of ${h.id}` })}
                              placeholder={formatNumber(h.priority)}
                              value={v.priority ?? ''}
                              onChange={(e) => change(h, { priority: e.target.value === '' ? null : Number(e.target.value) })}
                            />
                          </td>
                          <td className="py-2">
                            <input
                              type="checkbox"
                              className="mks-check"
                              checked={v.enabled}
                              disabled={h.system}
                              aria-label={tr({ fa: `روشن بودن ${h.id}`, en: `${h.id} on` })}
                              onChange={(e) => change(h, { enabled: e.target.checked })}
                            />
                          </td>
                        </tr>
                      )
                    })}
                  </tbody>
                </table>
              </Card>
            )
          })}
          <button
            type="button"
            disabled={busy || changed === 0}
            onClick={() => void save()}
            className="rounded-lg bg-primary-600 px-4 py-2 text-sm font-semibold text-white hover:bg-primary-500 disabled:opacity-50"
          >
            {tr({ fa: 'ذخیره', en: 'Save' })}
            {changed > 0 ? ` (${formatNumber(changed)})` : ''}
          </button>
        </>
      )}
    </div>
  )
}

export function hooksPage(group?: string) {
  return definePage({ name: 'hooks', label: tr({ fa: 'Hookها', en: 'Hooks' }), icon: <HookIcon />, group, component: HooksPage })
}
