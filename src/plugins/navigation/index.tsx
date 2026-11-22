import { useState } from 'react'
import {
  definePage,
  definePlugin,
  groupNavigation,
  MenuIcon,
  Modal,
  navigationItems,
  PageHeader,
  tr,
  usePageData,
  usePageSubmit,
  usePanel,
  type NavItemOverride,
  type NavOverrides,
  type NavPlacement,
  type PanelPlugin,
} from '../../index'

// MKSine's Admin menu editor, rebuilt: one row per sidebar entry, grouped by the group
// the code gives it; "placement" says whether the entry stays, is hidden or moves to
// another group; every action saves at once, like MKSine, and the sidebar redraws.

interface Row {
  key: string
  label: string
  /** The group the code puts this item in ('' = top level). */
  group: string
  sort?: number
  /** The entry that holds this editor, and its group: never hidden. */
  protected?: boolean
}

interface GroupOption {
  key: string
  title: string
}

interface GroupOverride {
  hidden?: boolean
  label?: string
  sort?: number
}

function T(fa: string, en: string): string {
  return tr({ fa, en })
}

const input =
  'block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm shadow-xs ring-1 ring-gray-950/10 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5 dark:ring-white/10'
const fieldLabel = 'mb-1.5 block text-sm font-medium text-gray-950 dark:text-white'
const helper = 'mt-1 block text-xs text-gray-500 dark:text-gray-400'
const buttonBase = 'inline-flex items-center justify-center gap-1.5 rounded-lg px-3.5 py-2 text-sm font-semibold disabled:opacity-50'
const buttonPrimary = buttonBase + ' bg-primary-600 text-white hover:bg-primary-500'
const buttonGray = buttonBase + ' text-gray-700 ring-1 ring-gray-950/10 hover:bg-gray-50 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/5'
const buttonDanger = buttonBase + ' bg-danger-600 text-white hover:bg-danger-500'
const iconButtonRow = 'inline-flex items-center gap-1.5 rounded-lg px-2 py-1 text-xs font-semibold text-gray-600 hover:bg-gray-900/5 hover:text-gray-900 dark:text-gray-300 dark:hover:bg-white/5 dark:hover:text-white'
const iconButton = 'inline-flex items-center justify-center rounded-lg p-1.5 text-gray-500 hover:text-gray-900 disabled:opacity-50 dark:text-gray-400 dark:hover:text-white'

/** Every sidebar entry as the code defines it, grouped the way the panel orders groups. */
function useMenu(): { rows: Row[]; groups: GroupOption[] } {
  const { meta, resources, pages, dashboard, navigation } = usePanel()
  const rows: Row[] = navigationItems({ meta, resources, pages, dashboard, navigation }).map((i) => ({
    key: i.key,
    label: i.label,
    group: i.group ?? '',
    sort: i.sort,
    protected: i.protected,
  }))
  const seen: string[] = []
  for (const g of navigation.groups) if (g.label && !seen.includes(g.label)) seen.push(g.label)
  for (const r of rows) if (r.group && !seen.includes(r.group)) seen.push(r.group)
  return {
    rows,
    groups: [
      { key: '', title: T('سطح اول', 'Top level') },
      ...seen.filter((g) => rows.some((r) => r.group === g)).map((g) => ({ key: g, title: g })),
    ],
  }
}

function placementOf(o?: NavItemOverride): NavPlacement {
  return o?.placement === 'group' || o?.placement === 'hidden' ? o.placement : o?.hidden ? 'hidden' : 'default'
}

/** One stored override, without the fields that mean "the default". */
function clean(o: NavItemOverride): NavItemOverride {
  const placement = placementOf(o)
  const out: NavItemOverride = { placement }
  if (placement === 'group' && o.group) out.group = o.group
  if (o.sort !== undefined) out.sort = o.sort
  if (o.label) out.label = o.label
  return out
}

function placementLabel(p: NavPlacement): string {
  if (p === 'hidden') return T('مخفی', 'Hidden')
  if (p === 'group') return T('گروه دیگر', 'Another group')
  return T('پیش‌فرض', 'Default')
}

const placementTone: Record<NavPlacement, string> = {
  default: 'bg-gray-100 text-gray-700 ring-gray-950/5 dark:bg-white/10 dark:text-gray-200 dark:ring-white/10',
  hidden: 'bg-danger-50 text-danger-700 ring-danger-600/20 dark:bg-danger-500/10 dark:text-danger-300 dark:ring-danger-400/20',
  group: 'bg-info-50 text-info-700 ring-info-600/15 dark:bg-info-500/10 dark:text-info-300 dark:ring-info-400/20',
}

function MenuEditor() {
  const data = usePageData<{ config: NavOverrides | null }>()
  const { rows, groups } = useMenu()
  if (data.isPending) return <div className="h-64 animate-pulse rounded-2xl bg-gray-100 dark:bg-white/5" />
  return <Editor key={JSON.stringify(data.data?.config ?? {})} rows={rows} groups={groups} initial={data.data?.config ?? {}} />
}

function Editor({ rows, groups: groupOptions, initial }: { rows: Row[]; groups: GroupOption[]; initial: NavOverrides }) {
  const submit = usePageSubmit()
  const { meta, resources, pages, dashboard, navigation, refreshMeta, notify } = usePanel()
  const [items, setItems] = useState<Record<string, NavItemOverride>>(initial.items ?? {})
  const [groupOverrides, setGroupOverrides] = useState<Record<string, GroupOverride>>(initial.groups ?? {})
  const [busy, setBusy] = useState(false)
  const [editing, setEditing] = useState<Row | null>(null)
  const [groupsOpen, setGroupsOpen] = useState(false)
  const [resetOpen, setResetOpen] = useState(false)

  // The manager's own menu, after the changes on screen: the preview below the table.
  const preview = groupNavigation(
    navigationItems({ meta, resources, pages, dashboard, navigation }),
    T('منو', 'Menu'),
    { items, groups: groupOverrides },
    navigation.groups,
  )

  async function persist(next: NavOverrides, action: 'save' | 'reset' = 'save') {
    setBusy(true)
    try {
      await submit({ config: next, action })
      setItems(next.items ?? {})
      setGroupOverrides(next.groups ?? {})
      // The sidebar lives in meta: fetch it again so the menu on the left redraws.
      await refreshMeta()
    } catch (e) {
      notify(e instanceof Error ? e.message : String(e), 'danger')
    } finally {
      setBusy(false)
    }
  }

  function saveItem(row: Row, values: { placement: NavPlacement; group: string; sort: string; label: string }) {
    const sort = Number.parseInt(values.sort, 10)
    const next = {
      ...items,
      [row.key]: clean({
        placement: values.placement,
        group: values.group,
        sort: Number.isNaN(sort) ? undefined : sort,
        label: values.label.trim(),
      }),
    }
    setEditing(null)
    void persist({ items: next, groups: groupOverrides })
  }

  function toggleHidden(row: Row) {
    const current = items[row.key] ?? {}
    const hidden = placementOf(current) === 'hidden'
    const next = {
      ...items,
      [row.key]: clean({ ...current, placement: hidden ? 'default' : 'hidden' }),
    }
    void persist({ items: next, groups: groupOverrides })
  }

  function saveGroups(next: Record<string, GroupOverride>) {
    const cleaned = Object.fromEntries(
      Object.entries(next).filter(([, g]) => g.hidden || g.label || g.sort !== undefined),
    )
    setGroupsOpen(false)
    void persist({ items, groups: cleaned })
  }

  const rowsOf = (group: string) => rows.filter((r) => r.group === group)

  return (
    <div className="space-y-6">
      <PageHeader
        title={T('منوی مدیریت', 'Admin menu')}
        description={T(
          'آیتم‌ها و گروه‌های سایدبار را مخفی، جابه‌جا، مرتب یا تغییر نام دهید. اگر همه‌چیز پیش‌فرض بماند، همان منوی تعریف‌شده در کد نمایش داده می‌شود.',
          'Hide, move, order or rename sidebar items and groups. If everything stays at its default, the menu the code defines is shown.',
        )}
        actions={
          <>
            <button type="button" className={buttonGray} onClick={() => setGroupsOpen(true)} disabled={busy}>
              <StackIcon />
              {T('ویرایش گروه‌ها', 'Edit groups')}
            </button>
            <button type="button" className={buttonDanger} onClick={() => setResetOpen(true)} disabled={busy}>
              <UndoIcon />
              {T('بازگشت به پیش‌فرض', 'Back to default')}
            </button>
          </>
        }
      />

      <p className="rounded-xl bg-warning-50 p-3 text-sm leading-6 text-warning-800 ring-1 ring-warning-600/20 dark:bg-warning-500/10 dark:text-warning-200 dark:ring-warning-400/20">
        {T(
          'مخفی کردن یا جابه‌جا کردن یک آیتم فقط منو را تغییر می‌دهد. نشانی و قوانین دسترسی صفحه عوض نمی‌شود و هر کسی که اجازهٔ ورود دارد، همچنان با نشانی مستقیم به آن می‌رسد.',
          'Hiding or moving an item only changes the menu. The page URL and its access rules stay the same, and anyone allowed in still reaches it directly by its address.',
        )}
      </p>

      <section className="overflow-x-auto rounded-2xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10">
        <table className="w-full text-sm">
          <thead className="border-b border-gray-100 bg-gray-50/50 text-xs text-gray-500 dark:border-white/5 dark:bg-white/[0.02] dark:text-gray-400">
            <tr>
              <th className="px-4 py-3 text-start font-medium">{T('آیتم', 'Item')}</th>
              <th className="px-4 py-3 text-start font-medium">{T('گروه پیش‌فرض', 'Default group')}</th>
              <th className="px-4 py-3 text-start font-medium">{T('جایگاه', 'Placement')}</th>
              <th className="px-4 py-3 text-start font-medium">{T('گروه مقصد', 'Target group')}</th>
              <th className="px-4 py-3 text-start font-medium">{T('ترتیب', 'Order')}</th>
              <th className="px-4 py-3 text-start font-medium">{T('نام جایگزین', 'Custom name')}</th>
              <th className="px-4 py-3">
                <span className="sr-only">{T('کارها', 'Actions')}</span>
              </th>
            </tr>
          </thead>
          {groupOptions.map((g) => {
            const inGroup = rowsOf(g.key)
            if (inGroup.length === 0) return null
            const hiddenGroup = g.key !== '' && groupOverrides[g.key]?.hidden
            return (
              <tbody key={g.key || 'top'}>
                <tr>
                  <th colSpan={7} scope="colgroup" className="bg-gray-50/80 px-4 py-2 text-start text-xs font-semibold text-gray-600 dark:bg-white/5 dark:text-gray-300">
                    {g.title}
                  </th>
                </tr>
                {inGroup.map((row) => {
                  const o = items[row.key] ?? {}
                  const placement = placementOf(o)
                  const target = placement === 'group' && o.group ? groupOptions.find((x) => x.key === o.group)?.title ?? o.group : null
                  const order = o.sort ?? row.sort
                  return (
                    <tr key={row.key} className="border-t border-gray-100 align-top hover:bg-gray-50/70 dark:border-white/5 dark:hover:bg-white/[0.03]">
                      <td className="px-4 py-3">
                        <span className="font-medium text-gray-950 dark:text-white" title={row.key}>
                          {o.label || row.label}
                        </span>
                        {o.label && <span className="block text-xs text-gray-400">{row.label}</span>}
                      </td>
                      <td className="px-4 py-3 text-gray-600 dark:text-gray-300">
                        {g.title}
                        {hiddenGroup && (
                          <span className="mt-0.5 block text-xs text-danger-600 dark:text-danger-400">{T('گروه مخفی است', 'The group is hidden')}</span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <span className={'inline-flex rounded-md px-2 py-0.5 text-[11px] font-medium ring-1 ' + placementTone[placement]}>
                          {placementLabel(placement)}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-gray-600 dark:text-gray-300">{target ?? '—'}</td>
                      <td className={'px-4 py-3 tabular-nums ' + (o.sort === undefined ? 'text-gray-400' : 'text-gray-950 dark:text-white')}>{order ?? '—'}</td>
                      <td className="px-4 py-3 text-gray-600 dark:text-gray-300">{o.label ?? '—'}</td>
                      <td className="px-4 py-3">
                        <div className="flex items-center justify-end gap-1 whitespace-nowrap">
                          {!row.protected && (
                            <button
                              type="button"
                              className={iconButtonRow + (placement === 'hidden' ? ' text-success-600 dark:text-success-400' : '')}
                              onClick={() => toggleHidden(row)}
                              disabled={busy}
                            >
                              {placement === 'hidden' ? <EyeIcon /> : <EyeSlashIcon />}
                              {placement === 'hidden' ? T('نمایش', 'Show') : T('مخفی کردن', 'Hide')}
                            </button>
                          )}
                          <button
                            type="button"
                            className={iconButton + ' hover:bg-gray-900/5 dark:hover:bg-white/5'}
                            onClick={() => setEditing(row)}
                            disabled={busy}
                            title={T('ویرایش', 'Edit')}
                            aria-label={T('ویرایش', 'Edit')}
                          >
                            <PencilIcon />
                          </button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            )
          })}
        </table>
      </section>

      <details className="group rounded-2xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10">
        <summary className="flex cursor-pointer list-none items-start gap-3 px-6 py-4 [&::-webkit-details-marker]:hidden">
          <span className="mt-1 text-gray-400 transition-transform group-open:rotate-90">
            <ChevronIcon />
          </span>
          <span className="min-w-0 flex-1">
            <span className="block text-base font-semibold tracking-tight text-gray-950 dark:text-white">{T('پیش‌نمایش سایدبار', 'Sidebar preview')}</span>
            <span className="mt-0.5 block text-sm text-gray-500 dark:text-gray-400">
              {T('سایدبار با اعمال تغییرات بالا، همان‌طور که برای شما نمایش داده می‌شود.', 'The sidebar after the changes above, as you would see it.')}
            </span>
          </span>
        </summary>
        <div className="px-6 pb-6">
          <div className="grid gap-5 sm:grid-cols-2 xl:grid-cols-3">
            {preview.map((g) => (
              <div key={g.heading}>
                <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase dark:text-gray-400">{g.heading}</p>
                <ul className="mt-2 space-y-1 text-sm text-gray-700 dark:text-gray-300">
                  {g.items.map((i) => (
                    <li key={i.key} className="flex items-center gap-2">
                      <span className="size-1 rounded-full bg-gray-400" aria-hidden="true" />
                      {i.label}
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
          {preview.length === 0 && <p className="text-sm text-gray-500 dark:text-gray-400">{T('آیتمی وجود ندارد.', 'Nothing to show.')}</p>}
        </div>
      </details>

      {editing && (
        <ItemModal
          row={editing}
          override={items[editing.key] ?? {}}
          groups={groupOptions.filter((g) => g.key !== '' && g.key !== editing.group)}
          busy={busy}
          onClose={() => setEditing(null)}
          onSave={(values) => saveItem(editing, values)}
        />
      )}
      {groupsOpen && (
        <GroupsModal
          groups={groupOptions.filter((g) => g.key !== '')}
          values={groupOverrides}
          busy={busy}
          onClose={() => setGroupsOpen(false)}
          onSave={saveGroups}
        />
      )}
      {resetOpen && (
        <Modal
          title={T('منوی مدیریت به حالت پیش‌فرض برگردد؟', 'Reset the admin menu to its default?')}
          description={T(
            'همهٔ تغییرات منو پاک می‌شود و سایدبار به همان منوی تعریف‌شده در کد برمی‌گردد.',
            'Every menu change is removed and the sidebar returns to the menu the code defines.',
          )}
          tone="danger"
          size="md"
          onClose={() => setResetOpen(false)}
          footer={
            <>
              <button type="button" className={buttonGray} onClick={() => setResetOpen(false)} disabled={busy}>
                {T('انصراف', 'Cancel')}
              </button>
              <button
                type="button"
                className={buttonDanger}
                disabled={busy}
                onClick={() => {
                  setResetOpen(false)
                  void persist({ items: {}, groups: {} }, 'reset')
                }}
              >
                {T('بازگشت به پیش‌فرض', 'Back to default')}
              </button>
            </>
          }
        />
      )}
    </div>
  )
}

function ItemModal({
  row,
  override,
  groups,
  busy,
  onClose,
  onSave,
}: {
  row: Row
  override: NavItemOverride
  groups: GroupOption[]
  busy: boolean
  onClose: () => void
  onSave: (values: { placement: NavPlacement; group: string; sort: string; label: string }) => void
}) {
  const [placement, setPlacement] = useState<NavPlacement>(placementOf(override))
  const [group, setGroup] = useState(override.group ?? groups[0]?.key ?? '')
  const [sort, setSort] = useState(override.sort === undefined ? '' : String(override.sort))
  const [label, setLabel] = useState(override.label ?? '')

  const placements: { value: NavPlacement; label: string }[] = row.protected
    ? [{ value: 'default', label: placementLabel('default') }]
    : [
        { value: 'default', label: placementLabel('default') },
        { value: 'hidden', label: placementLabel('hidden') },
        { value: 'group', label: placementLabel('group') },
      ]

  return (
    <Modal
      title={override.label || row.label}
      size="lg"
      onClose={onClose}
      footer={
        <>
          <button type="button" className={buttonGray} onClick={onClose} disabled={busy}>
            {T('انصراف', 'Cancel')}
          </button>
          <button type="button" className={buttonPrimary} disabled={busy} onClick={() => onSave({ placement, group, sort, label })}>
            {T('ذخیره', 'Save')}
          </button>
        </>
      }
    >
      <div className="space-y-4">
        <div>
          <label className={fieldLabel} htmlFor="nav-placement">
            {T('جایگاه', 'Placement')}
          </label>
          <select id="nav-placement" className={input} value={placement} onChange={(e) => setPlacement(e.target.value as NavPlacement)}>
            {placements.map((p) => (
              <option key={p.value} value={p.value}>
                {p.label}
              </option>
            ))}
          </select>
          <span className={helper}>
            {row.protected
              ? T('این آیتم راه رسیدن به منوی مدیریت است، پس نمی‌توان آن را مخفی کرد.', 'This entry holds the menu editor, so it cannot be hidden.')
              : T('«مخفی» فقط آیتم را از سایدبار برمی‌دارد؛ نشانی و قوانین دسترسی صفحه همان می‌ماند.', '"Hidden" only takes the item out of the sidebar; its URL and access rules stay.')}
          </span>
        </div>

        {placement === 'group' && (
          <div>
            <label className={fieldLabel} htmlFor="nav-group">
              {T('گروه مقصد', 'Target group')}
            </label>
            <select id="nav-group" className={input} value={group} onChange={(e) => setGroup(e.target.value)}>
              {groups.map((g) => (
                <option key={g.key} value={g.key}>
                  {g.title}
                </option>
              ))}
            </select>
          </div>
        )}

        <div>
          <label className={fieldLabel} htmlFor="nav-sort">
            {T('ترتیب', 'Order')}
          </label>
          <input
            id="nav-sort"
            className={input}
            type="number"
            dir="ltr"
            inputMode="numeric"
            placeholder={row.sort === undefined ? '' : String(row.sort)}
            value={sort}
            onChange={(e) => setSort(e.target.value)}
          />
          <span className={helper}>
            {T(`عدد کوچک‌تر بالاتر می‌آید. برای مقدار پیش‌فرض (${row.sort ?? '—'}) خالی بگذارید.`, `A smaller number comes first. Leave it empty for the default (${row.sort ?? '—'}).`)}
          </span>
        </div>

        <div>
          <label className={fieldLabel} htmlFor="nav-label">
            {T('نام جایگزین', 'Custom name')}
          </label>
          <input
            id="nav-label"
            className={input}
            maxLength={120}
            placeholder={row.label}
            value={label}
            onChange={(e) => setLabel(e.target.value)}
          />
          <span className={helper}>{T('برای استفاده از نام پیش‌فرض خالی بگذارید.', 'Leave it empty to keep the default name.')}</span>
        </div>
      </div>
    </Modal>
  )
}

function GroupsModal({
  groups,
  values,
  busy,
  onClose,
  onSave,
}: {
  groups: GroupOption[]
  values: Record<string, GroupOverride>
  busy: boolean
  onClose: () => void
  onSave: (next: Record<string, GroupOverride>) => void
}) {
  const [draft, setDraft] = useState<Record<string, { sort: string; label: string; hidden: boolean }>>(
    Object.fromEntries(
      groups.map((g) => [g.key, { sort: values[g.key]?.sort === undefined ? '' : String(values[g.key]?.sort), label: values[g.key]?.label ?? '', hidden: !!values[g.key]?.hidden }]),
    ),
  )
  const patch = (key: string, next: Partial<{ sort: string; label: string; hidden: boolean }>) =>
    setDraft((all) => ({ ...all, [key]: { ...all[key], ...next } }))

  return (
    <Modal
      title={T('گروه‌های سایدبار', 'Sidebar groups')}
      description={T('ترتیب، نام و نمایش هر گروه را تغییر دهید. گروه‌های خالی نمایش داده نمی‌شوند.', 'Change each group’s order, name and visibility. Empty groups are not shown.')}
      size="lg"
      closeOnBackdrop={false}
      onClose={onClose}
      footer={
        <>
          <button type="button" className={buttonGray} onClick={onClose} disabled={busy}>
            {T('انصراف', 'Cancel')}
          </button>
          <button
            type="button"
            className={buttonPrimary}
            disabled={busy}
            onClick={() => {
              const next: Record<string, GroupOverride> = {}
              for (const g of groups) {
                const d = draft[g.key]
                const sort = Number.parseInt(d.sort, 10)
                next[g.key] = { sort: Number.isNaN(sort) ? undefined : sort, label: d.label.trim(), hidden: d.hidden }
              }
              onSave(next)
            }}
          >
            {T('ذخیره', 'Save')}
          </button>
        </>
      }
    >
      <div className="space-y-5">
        {groups.map((g) => (
          <fieldset key={g.key} className="rounded-xl p-4 ring-1 ring-gray-950/5 dark:ring-white/10">
            <p className="text-sm font-semibold text-gray-950 dark:text-white">{values[g.key]?.label || g.title}</p>
            <div className="mt-3 grid gap-4 sm:grid-cols-2">
              <div>
                <label className={fieldLabel} htmlFor={`group-sort-${g.key}`}>
                  {T('ترتیب', 'Order')}
                </label>
                <input
                  id={`group-sort-${g.key}`}
                  className={input}
                  type="number"
                  dir="ltr"
                  inputMode="numeric"
                  placeholder={String((groups.findIndex((x) => x.key === g.key) + 1) * 10)}
                  value={draft[g.key]?.sort ?? ''}
                  onChange={(e) => patch(g.key, { sort: e.target.value })}
                />
              </div>
              <div>
                <label className={fieldLabel} htmlFor={`group-label-${g.key}`}>
                  {T('نام جایگزین', 'Custom name')}
                </label>
                <input
                  id={`group-label-${g.key}`}
                  className={input}
                  maxLength={120}
                  placeholder={g.title}
                  value={draft[g.key]?.label ?? ''}
                  onChange={(e) => patch(g.key, { label: e.target.value })}
                />
              </div>
            </div>
            <label className="mt-3 flex items-start gap-2 text-sm text-gray-700 dark:text-gray-200">
              <input
                type="checkbox"
                className="mks-check mt-0.5"
                checked={draft[g.key]?.hidden ?? false}
                onChange={(e) => patch(g.key, { hidden: e.target.checked })}
              />
              <span>
                {T('مخفی کردن گروه', 'Hide the group')}
                <span className={helper}>
                  {T(
                    'کل گروه و آیتم‌هایش را از سایدبار برمی‌دارد. نشانی و قوانین دسترسی صفحه‌ها همان می‌ماند.',
                    'Takes the whole group and its items out of the sidebar. Page URLs and access rules stay.',
                  )}
                </span>
              </span>
            </label>
          </fieldset>
        ))}
      </div>
    </Modal>
  )
}

function AdminMenuPage() {
  return <MenuEditor />
}

/** The admin menu editor ("admin_menu" page). Install the Go plugin (navigation.Plugin) too. */
export function navigation(options: { group?: string; cluster?: string; label?: string; navigationSort?: number } = {}): PanelPlugin {
  return definePlugin({
    name: 'navigation',
    pages: [
      definePage({
        name: 'admin_menu',
        label: options.label ?? T('منوی مدیریت', 'Admin menu'),
        icon: <MenuIcon />,
        group: options.group,
        cluster: options.cluster,
        navigationSort: options.navigationSort,
        component: AdminMenuPage,
      }),
    ],
  })
}

// Icons shaped like the panel's (Heroicons' outline set), so the rows read like MKSine's.

function PencilIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" className="size-4" aria-hidden="true">
      <path d="m16.9 4.1 3 3L8.5 18.5 4 20l1.5-4.5Z" />
      <path d="m14.5 6.5 3 3" />
    </svg>
  )
}

function EyeIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" className="size-4" aria-hidden="true">
      <path d="M2.5 12S6 5.5 12 5.5 21.5 12 21.5 12 18 18.5 12 18.5 2.5 12 2.5 12Z" />
      <circle cx="12" cy="12" r="3" />
    </svg>
  )
}

function EyeSlashIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" className="size-4" aria-hidden="true">
      <path d="M3 3l18 18" />
      <path d="M10.6 6.1A9.6 9.6 0 0 1 12 6c6 0 9.5 6 9.5 6a17 17 0 0 1-3.2 3.9M6.3 8.2A16.6 16.6 0 0 0 2.5 12S6 18 12 18a9 9 0 0 0 3.2-.6" />
      <path d="M9.9 9.9a3 3 0 0 0 4.2 4.2" />
    </svg>
  )
}

function StackIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" className="size-4" aria-hidden="true">
      <rect x="3.5" y="3.5" width="17" height="6" rx="1.5" />
      <rect x="3.5" y="14.5" width="17" height="6" rx="1.5" />
    </svg>
  )
}

function ChevronIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round" className="size-4" aria-hidden="true">
      <path d="m9 6 6 6-6 6" />
    </svg>
  )
}

function UndoIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.6} strokeLinecap="round" strokeLinejoin="round" className="size-4" aria-hidden="true">
      <path d="M4 8h9.5A5.5 5.5 0 0 1 19 13.5v.5" />
      <path d="m4 8 4-4M4 8l4 4" />
    </svg>
  )
}
