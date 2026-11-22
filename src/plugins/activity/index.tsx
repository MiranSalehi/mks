import {
  defineColumn,
  definePlugin,
  defineResource,
  Link,
  tr,
  usePanel,
  widgetBuilder,
  type CustomCellProps,
  type PanelPlugin,
} from '../../index'

/** One log entry, as the server's "activities" resource sends it. */
export interface Activity {
  id: number
  user_id: string | null
  user_name: string | null
  resource: string
  record_key: string
  title: string | null
  action: 'created' | 'updated' | 'deleted'
  created_at: string | null
}

const actionLabels = { created: tr({ fa: 'ایجاد', en: 'Created' }), updated: tr({ fa: 'ویرایش', en: 'Updated' }), deleted: tr({ fa: 'حذف', en: 'Deleted' }) }
const actionColors = { created: 'success', updated: 'info', deleted: 'danger' } as const

const ClockIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M12 7v5l3 2M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z" />
  </svg>
)

/** The section a record belongs to, by the app's own resource label. */
const resourceLabel = defineColumn({
  defaults: {},
  component: function ResourceLabel({ value }: CustomCellProps) {
    const { resources } = usePanel()
    const def = resources.find((r) => r.name === value)
    return <span className="text-gray-600 dark:text-gray-300">{def?.pluralLabel ?? String(value)}</span>
  },
})

/** The record's title, linked to it while it exists. */
const recordLink = defineColumn({
  defaults: {},
  component: function RecordLink({ record }: CustomCellProps) {
    const { resources } = usePanel()
    const a = record as unknown as Activity
    const def = resources.find((r) => r.name === a.resource)
    const text = a.title || `#${a.record_key}`
    if (!def || a.action === 'deleted') return <span className="font-medium">{text}</span>
    const to = def.infolist.length > 0 ? '/$resource/$id' : '/$resource/$id/edit'
    return (
      <Link to={to} params={{ resource: a.resource, id: a.record_key }} className="font-medium text-primary-600 hover:underline dark:text-primary-400">
        {text}
      </Link>
    )
  },
})

export const activities = defineResource<Activity>({
  name: 'activities',
  label: tr({ fa: 'فعالیت', en: 'Activity' }),
  pluralLabel: tr({ fa: 'گزارش فعالیت', en: 'Activity log' }),
  icon: <ClockIcon />,
  defaultSort: '-id',
  recordTitle: (a) => a.title ?? `#${a.record_key}`,
  table: (t) => [
    recordLink<Activity>('title').label(tr({ fa: 'رکورد', en: 'Record' })).searchable(),
    t.badge('action').label(tr({ fa: 'کار', en: 'Action' })).colors(actionColors).labels(actionLabels),
    resourceLabel<Activity>('resource').label(tr({ fa: 'بخش', en: 'Section' })),
    t.text('user_name').label(tr({ fa: 'کاربر', en: 'User' })),
    t.date('created_at').label(tr({ fa: 'زمان', en: 'Time' })).since().sortable(),
  ],
  filters: (f) => [f.select('action').label(tr({ fa: 'کار', en: 'Action' })).options(actionLabels).multiple()],
})

/**
 * The activity log: a read-only "Activity log" resource and, unless turned off, the latest
 * entries on the dashboard. Install the Go plugin (activity.Plugin) too.
 */
export function activityLog(options: { group?: string; dashboard?: boolean } = {}): PanelPlugin {
  const resource = { ...activities, group: options.group }
  const w = widgetBuilder()
  return definePlugin({
    name: 'activity-log',
    resources: [resource],
    widgets:
      options.dashboard === false
        ? []
        : [
            w.table<Activity>('activity_latest')
              .resource('activities')
              .heading(tr({ fa: 'آخرین فعالیت‌ها', en: 'Latest activity' }))
              .sort('-id')
              .limit(6)
              .columns((t) => [
                recordLink<Activity>('title').label(tr({ fa: 'رکورد', en: 'Record' })),
                t.badge('action').label(tr({ fa: 'کار', en: 'Action' })).colors(actionColors).labels(actionLabels),
                t.text('user_name').label(tr({ fa: 'کاربر', en: 'User' })),
                t.date('created_at').label(tr({ fa: 'زمان', en: 'Time' })).since(),
              ]),
          ],
  })
}
