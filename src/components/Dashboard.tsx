import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useId, type ReactNode } from 'react'
import { humanize } from '../resource'
import {
  chartLabel,
  AccountWidget,
  ChartWidget,
  StatsWidget,
  TableWidget,
  type ChartData,
  type DashboardDefinition,
  type StatsData,
  type Widget,
} from '../widgets'
import { Cell } from './cells'
import { CartesianChart, colorVar, DoughnutChart, Sparkline, type Series } from './charts'
import { usePanel } from './Panel'
import { RecordLinks } from './RecordLinks'
import { numberFormat, t } from '../i18n'

const nf = numberFormat({ maximumFractionDigits: 1 })
const percent = numberFormat({ style: 'percent', maximumFractionDigits: 1 })
const card = 'rounded-2xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10'

/** Widgets the user may see: server widgets listed in meta, tables of visible resources. */
export function useVisibleWidgets(dashboard: DashboardDefinition | undefined): Widget[] {
  const { meta, resources } = usePanel()
  if (!dashboard) return []
  return dashboard.widgets.filter((w) =>
    w instanceof AccountWidget ? true : w instanceof TableWidget ? resources.some((r) => r.name === w._resource) : (meta.widgets ?? []).includes(w.name),
  )
}

export function Dashboard({ dashboard }: { dashboard: DashboardDefinition }) {
  const { meta } = usePanel()
  const widgets = useVisibleWidgets(dashboard)
  const firstName = meta.user.name.trim().split(/\s+/)[0]
  return (
    <div className="space-y-6">
      <header>
        <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{dashboard.heading}</h1>
        <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{t('dashboard.greeting', { name: firstName })}</p>
      </header>
      {widgets.length === 0 ? (
        <p className={card + ' p-10 text-center text-sm text-gray-500 dark:text-gray-400'}>{t('dashboard.empty')}</p>
      ) : (
        <div className="grid gap-6 lg:grid-cols-2">
          {widgets.map((w) => (
            <div key={w.name} className={w._columnSpan === 'full' ? 'lg:col-span-2' : ''}>
              <WidgetView widget={w} />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

function WidgetView({ widget }: { widget: Widget }) {
  if (widget instanceof StatsWidget) return <StatsView widget={widget} />
  if (widget instanceof ChartWidget) return <ChartView widget={widget} />
  if (widget instanceof TableWidget) return <TableView widget={widget} />
  if (widget instanceof AccountWidget) return <AccountView />
  return null
}

function AccountView() {
  const { meta, logout } = usePanel()
  const initials = meta.user.name.trim().split(/\s+/).map((w) => w[0]).slice(0, 2).join('')
  return (
    <div className={card + ' flex items-center gap-4 p-5'}>
      <span className="flex size-11 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-primary-400 to-primary-700 text-sm font-semibold text-white">{initials}</span>
      <div className="min-w-0 flex-1">
        <p className="text-sm text-gray-500 dark:text-gray-400">{t('dashboard.welcome')}</p>
        <p className="truncate font-semibold">{meta.user.name}</p>
      </div>
      <button
        type="button"
        onClick={() => void logout()}
        className="rounded-lg px-3 py-1.5 text-sm font-semibold text-gray-700 ring-1 ring-gray-950/10 hover:bg-gray-50 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/5"
      >
        {t('user.logout')}
      </button>
    </div>
  )
}

function useWidgetData<T>(widget: Widget) {
  const { api } = usePanel()
  return useQuery({
    queryKey: ['widget', widget.name],
    queryFn: () => api.widget<T>(widget.name),
    refetchInterval: widget._poll ? widget._poll * 1000 : false,
    select: (r) => r.data,
  })
}

function WidgetCard({ widget, children, action }: { widget: Widget; children: ReactNode; action?: ReactNode }) {
  const headingId = useId()
  return (
    <section aria-labelledby={widget._heading ? headingId : undefined} className={card + ' flex h-full flex-col'}>
      {(widget._heading || action) && (
        <header className="flex items-start justify-between gap-4 px-6 pt-5">
          <div>
            {widget._heading && (
              <h2 id={headingId} className="text-base font-semibold tracking-tight text-gray-950 dark:text-white">
                {widget._heading}
              </h2>
            )}
            {widget._description && <p className="mt-0.5 text-sm text-gray-500 dark:text-gray-400">{widget._description}</p>}
          </div>
          {action}
        </header>
      )}
      <div className="flex-1 p-6">{children}</div>
    </section>
  )
}

function Failed({ error }: { error: Error }) {
  return <p className="text-sm text-danger-600 dark:text-danger-400">{t('dashboard.widgetError', { message: error.message })}</p>
}

// --- stats --------------------------------------------------------------------------------

function StatsView({ widget }: { widget: StatsWidget }) {
  const data = useWidgetData<StatsData>(widget)
  return (
    <div className="space-y-3">
      {widget._heading && <h2 className="text-base font-semibold tracking-tight text-gray-950 dark:text-white">{widget._heading}</h2>}
      {data.error && <Failed error={data.error} />}
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        {widget._stats.map((stat) => {
          const d = data.data?.[stat.key]
          const value = d?.value
          const text = value == null ? '—' : stat._format ? stat._format(value) : typeof value === 'number' ? nf.format(value) : String(value)
          return (
            <div key={stat.key} className={card + ' relative overflow-hidden p-5'}>
              <div className="flex items-start justify-between gap-3">
                <p className="text-sm font-medium text-gray-500 dark:text-gray-400">{stat._label ?? humanize(stat.key)}</p>
                {stat._icon && (
                  <span
                    className="flex size-9 items-center justify-center rounded-xl [&_svg]:size-5"
                    style={{ background: `color-mix(in oklch, ${colorVar(stat._color)} 12%, transparent)`, color: colorVar(stat._color, 600) }}
                  >
                    {stat._icon}
                  </span>
                )}
              </div>
              {data.isPending ? (
                <div className="mt-3 h-8 w-24 animate-pulse rounded-lg bg-gray-100 dark:bg-white/5" />
              ) : (
                <p className="mt-2 text-3xl font-bold tracking-tight text-gray-950 tabular-nums dark:text-white">{text}</p>
              )}
              {(d?.change != null || stat._description) && (
                <p className="mt-2 flex flex-wrap items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
                  {d?.change != null && <Change value={d.change} />}
                  {stat._description}
                </p>
              )}
              {d?.trend && d.trend.length > 1 && (
                <div className="-mx-5 -mb-5 mt-4">
                  <Sparkline data={d.trend} color={stat._color} />
                </div>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}

function Change({ value }: { value: number }) {
  const up = value >= 0
  return (
    <span
      className={
        'inline-flex items-center gap-0.5 rounded-full px-1.5 py-0.5 font-semibold ' +
        (up ? 'bg-success-50 text-success-700 dark:bg-success-500/10 dark:text-success-400' : 'bg-danger-50 text-danger-700 dark:bg-danger-500/10 dark:text-danger-400')
      }
    >
      <span aria-hidden="true">{up ? '↑' : '↓'}</span>
      <span className="sr-only">{up ? t('dashboard.up') : t('dashboard.down')}</span>
      {percent.format(Math.abs(value))}
    </span>
  )
}

// --- charts -------------------------------------------------------------------------------

function ChartView({ widget }: { widget: ChartWidget }) {
  const data = useWidgetData<ChartData>(widget)
  const caption = widget._heading ?? humanize(widget.name)
  let body: ReactNode
  if (data.isPending) {
    body = <div className="animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" style={{ height: widget._height }} />
  } else if (data.error) {
    body = <Failed error={data.error} />
  } else {
    const labels = data.data.labels.map((l) => chartLabel(widget, l))
    if (widget._type === 'doughnut') {
      const set = data.data.datasets[0]
      const fallback = ['primary', 'success', 'warning', 'info', 'danger', 'gray'] as const
      body = (
        <DoughnutChart
          caption={caption}
          labels={labels}
          values={set?.data ?? []}
          colors={data.data.labels.map((l, i) => widget._colors[l] ?? fallback[i % fallback.length])}
        />
      )
    } else {
      const series: Series[] = data.data.datasets.map((d, i) => ({
        label: widget._datasets[d.key]?.label ?? humanize(d.key),
        color: widget._datasets[d.key]?.color ?? (['primary', 'success', 'warning', 'info'] as const)[i % 4],
        data: d.data,
      }))
      body = <CartesianChart type={widget._type} caption={caption} labels={labels} series={series} height={widget._height} />
    }
  }
  return <WidgetCard widget={widget}>{body}</WidgetCard>
}

// --- table --------------------------------------------------------------------------------

function TableView({ widget }: { widget: TableWidget<any> }) {
  const { api, meta, resources } = usePanel()
  const resource = resources.find((r) => r.name === widget._resource)!
  const columns = widget._columns ?? resource.columns
  const key = meta.resources[resource.name]?.key ?? 'id'
  const offered = new Set(meta.resources[resource.name]?.relations ?? [])
  const include = [...new Set(columns.flatMap((c) => c.relations))].filter((r) => offered.has(r))
  const query = useQuery({
    queryKey: ['records', resource.name, { widget: widget.name, sort: widget._sort, limit: widget._limit, include }],
    queryFn: () => api.list(resource.name, { perPage: widget._limit, sort: widget._sort ? widget._sort.split(',') : undefined, include }),
    refetchInterval: widget._poll ? widget._poll * 1000 : false,
  })
  return (
    <WidgetCard
      widget={widget}
      action={
        <Link to="/$resource" params={{ resource: resource.name }} className="text-sm font-semibold text-primary-600 hover:text-primary-500 dark:text-primary-400">
          {t('dashboard.all', { label: resource.pluralLabel })}
        </Link>
      }
    >
      {query.error ? (
        <Failed error={query.error} />
      ) : (
        <div className="-mx-6 -mb-6 overflow-x-auto">
          <table className="w-full text-sm whitespace-nowrap">
            <thead className="border-y border-gray-100 bg-gray-50/70 dark:border-white/5 dark:bg-white/[0.02]">
              <tr>
                {columns.map((c) => (
                  <th key={c.name} scope="col" className="px-6 py-2.5 text-start text-xs font-semibold text-gray-500 dark:text-gray-400">
                    {c._label ?? humanize(c.name)}
                  </th>
                ))}
                <th scope="col">
                  <span className="sr-only">{t('common.actions')}</span>
                </th>
              </tr>
            </thead>
            <tbody className="divide-y divide-gray-100 dark:divide-white/5">
              {query.isPending &&
                Array.from({ length: widget._limit }, (_, i) => (
                  <tr key={i}>
                    <td colSpan={columns.length + 1} className="px-6 py-3">
                      <div className="h-5 animate-pulse rounded-md bg-gray-100 dark:bg-white/5" />
                    </td>
                  </tr>
                ))}
              {query.data?.data.map((record) => {
                const rk = String(record[key])
                return (
                  <tr key={rk} className="transition-colors hover:bg-gray-50/80 dark:hover:bg-white/[0.03]">
                    {columns.map((c) => (
                      <td key={c.name} className="px-6 py-3 text-gray-700 dark:text-gray-300">
                        <Cell column={c} record={record} />
                      </td>
                    ))}
                    <td className="px-6 py-3">
                      <div className="flex items-center justify-end gap-3">
                        <RecordLinks
                          resource={resource}
                          recordKey={rk}
                          title={resource.recordTitle(record)}
                          canUpdate={query.data.abilities?.[rk]?.update !== false}
                        />
                      </div>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}
    </WidgetCard>
  )
}
