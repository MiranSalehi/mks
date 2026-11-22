import { useQuery } from '@tanstack/react-query'
import {
  defineField,
  definePlugin,
  defineResource,
  tr,
  usePanel,
  type CustomFieldProps,
  type PanelPlugin,
} from '../../index'

/** A role, as the server's "roles" resource sends it. */
export interface Role {
  id: number
  name: string
  description: string | null
  super: boolean
  permissions: string[]
  users: string[]
  created_at: string | null
  updated_at: string | null
}

/** Permissions of one resource, widget list, page list or custom set. */
interface PermissionGroup {
  kind: 'resource' | 'widgets' | 'pages' | 'custom'
  name?: string
  permissions: string[]
}

const abilityLabels: Record<string, string> = {
  view_any: tr({ fa: 'دیدن فهرست', en: 'List' }),
  view: tr({ fa: 'دیدن رکورد', en: 'View' }),
  create: tr({ fa: 'ساختن', en: 'Create' }),
  update: tr({ fa: 'ویرایش', en: 'Edit' }),
  delete: tr({ fa: 'حذف', en: 'Delete' }),
}

const groupLabels = {
  widgets: tr({ fa: 'ویجت‌های داشبورد', en: 'Dashboard widgets' }),
  pages: tr({ fa: 'صفحه‌ها', en: 'Pages' }),
  custom: tr({ fa: 'سایر مجوزها', en: 'Other permissions' }),
}

const ShieldIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M12 3l8 3v6c0 4.5-3.4 8.3-8 9-4.6-.7-8-4.5-8-9V6zM9 12l2 2 4-4" />
  </svg>
)

/** The label of one permission: an ability of a resource, a widget, a page or a custom key. */
function permissionLabel(group: PermissionGroup, permission: string): string {
  if (group.kind === 'resource' && group.name) {
    const rest = permission.slice(group.name.length + 1)
    if (rest.startsWith('action.')) return tr({ fa: 'عملیات: ', en: 'Action: ' }) + rest.slice(7)
    return abilityLabels[rest] ?? rest
  }
  return permission.replace(/^(widget|page)\./, '')
}

/** Checkboxes for every permission of the panel, grouped by resource. */
function PermissionsControl({ id, value, onChange, disabled }: CustomFieldProps<string[] | null, Record<string, never>>) {
  const { api, resources } = usePanel()
  const catalog = useQuery({
    queryKey: ['access', 'permissions'],
    queryFn: () => api.plugin<{ data: PermissionGroup[] }>('access', '/permissions'),
    staleTime: 60_000,
  })
  const selected = new Set(Array.isArray(value) ? value : [])
  const set = (perms: string[], on: boolean) => {
    const next = new Set(selected)
    for (const p of perms) {
      if (on) next.add(p)
      else next.delete(p)
    }
    onChange([...next])
  }
  if (catalog.isLoading) return <p className="text-sm text-gray-500">{tr({ fa: 'در حال بارگذاری…', en: 'Loading…' })}</p>
  if (catalog.isError) return <p className="text-sm text-danger-600">{tr({ fa: 'فهرست مجوزها بارگذاری نشد.', en: 'Could not load permissions.' })}</p>
  return (
    <div id={id} role="group" className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
      {(catalog.data?.data ?? []).map((group) => {
        const title =
          group.kind === 'resource'
            ? (resources.find((r) => r.name === group.name)?.pluralLabel ?? group.name ?? '')
            : groupLabels[group.kind]
        const all = group.permissions.every((p) => selected.has(p))
        return (
          <fieldset key={`${group.kind}:${group.name ?? ''}`} className="rounded-xl p-3 ring-1 ring-gray-950/10 dark:ring-white/10">
            <legend className="flex w-full items-center justify-between gap-2 px-1 text-sm font-semibold">
              <span>{title}</span>
            </legend>
            <label className="mb-1 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <input type="checkbox" className="mks-check" checked={all} disabled={disabled} onChange={(e) => set(group.permissions, e.target.checked)} />
              {tr({ fa: 'همه', en: 'All' })}
            </label>
            <div className="space-y-1">
              {group.permissions.map((p) => (
                <label key={p} className="flex items-center gap-2 text-sm">
                  <input type="checkbox" className="mks-check" checked={selected.has(p)} disabled={disabled} onChange={(e) => set([p], e.target.checked)} />
                  <span>{permissionLabel(group, p)}</span>
                </label>
              ))}
            </div>
          </fieldset>
        )
      })}
    </div>
  )
}

const permissionsField = defineField<string[] | null, Record<string, never>>({ component: PermissionsControl, defaults: {}, empty: [] })

/** The roles resource; usersResource is the resource the users come from. */
export function rolesResource(usersResource = 'users') {
  return defineResource<Role>({
    name: 'roles',
    label: tr({ fa: 'نقش', en: 'Role' }),
    pluralLabel: tr({ fa: 'نقش‌ها', en: 'Roles' }),
    icon: <ShieldIcon />,
    defaultSort: 'name',
    recordTitle: 'name',
    table: (t) => [
      t.text('name').label(tr({ fa: 'نام', en: 'Name' })).searchable().sortable().weight('medium'),
      t.boolean('super').label(tr({ fa: 'دسترسی کامل', en: 'Full access' })),
      t.text('permissions').label(tr({ fa: 'مجوزها', en: 'Permissions' })).numeric()
        .format((v, r) => (r.super ? tr({ fa: 'همه', en: 'All' }) : String(Array.isArray(v) ? v.length : 0))),
      t.text('users').label(tr({ fa: 'کاربران', en: 'Users' })).numeric().format((v) => String(Array.isArray(v) ? v.length : 0)),
      t.date('updated_at').label(tr({ fa: 'آخرین تغییر', en: 'Last changed' })).since().sortable(),
    ],
    form: (f) => [
      f.section(tr({ fa: 'نقش', en: 'Role' })).schema([
        f.textInput('name').label(tr({ fa: 'نام', en: 'Name' })).required().maxLength(100),
        f.toggle('super')
          .label(tr({ fa: 'دسترسی کامل', en: 'Full access' }))
          .helperText(tr({ fa: 'همه‌ی مجوزها، حتی مجوزهای آینده. فقط کاربری با دسترسی کامل می‌تواند آن را بدهد.', en: 'Every permission, including future ones. Only a full-access user can grant it.' })),
        f.textarea('description').label(tr({ fa: 'توضیح', en: 'Description' })).rows(2).columnSpan('full'),
        f.select('users')
          .label(tr({ fa: 'کاربران', en: 'Users' }))
          .optionsFrom(usersResource)
          .multiple()
          .columnSpan('full'),
      ]),
      f.section(tr({ fa: 'مجوزها', en: 'Permissions' }))
        .visible(({ get }) => !get('super'))
        .schema([permissionsField<Role>('permissions').label('').columnSpan('full')]),
    ],
  })
}

/**
 * Roles and permissions: a "Roles" resource with a permission picker. Install the Go
 * plugin (access.Plugin) too.
 */
export function access(options: { group?: string; usersResource?: string; navigationSort?: number } = {}): PanelPlugin {
  const roles = { ...rolesResource(options.usersResource), navigationSort: options.navigationSort ?? 30 }
  return definePlugin({ name: 'access', resources: [options.group ? { ...roles, group: options.group } : roles] })
}
