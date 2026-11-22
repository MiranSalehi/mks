import { keepPreviousData, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useSearch } from '@tanstack/react-router'
import {
  rowSelectionFeature,
  rowSortingFeature,
  tableFeatures,
  useTable,
  type ColumnDef,
  type RowSelectionState,
  type SortingState,
  type Updater,
} from '@tanstack/react-table'
import { Fragment, useEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import type { RecordAbilities, ResourceRecord } from '../protocol'
import { humanize, type ResourceDefinition } from '../resource'
import type { ListSearch } from '../router'
import type { Align } from '../table'
import { LinkAction, type Action } from '../actions'
import { actionButtonColors, actionLabel, actionTextColors, buttonBase, buttonSmall, useActionRunner } from './ActionModal'
import { filterKey, filterParams, readFilterStates, urlValue, type FilterState } from '../filters'
import { Cell } from './cells'
import { RecordLinks } from './RecordLinks'
import { ActiveFilters, FiltersMenu } from './Filters'
import { ColumnsMenu, formatSummary, summaryLabels, toolButton, useHiddenColumns } from './TableTools'
import { ChevronIcon, GroupIcon, PlusIcon, ReorderIcon, SearchIcon } from './icons'
import { inputClass } from './fields'
import { usePanel } from './Panel'
import { numberFormat, t } from '../i18n'

const nf = numberFormat()
const features = tableFeatures({ rowSortingFeature, rowSelectionFeature })
const emptyRows: ResourceRecord[] = []
const PER_PAGE_OPTIONS = [10, 25, 50, 100]
const ACTIONS = '__actions'
const SELECT = '__select'
const HANDLE = '__handle'

const alignClass: Record<Align, string> = { start: 'text-start', center: 'text-center', end: 'text-end' }

export function parseSort(sort: string | undefined): SortingState {
  return (sort ?? '')
    .split(',')
    .filter(Boolean)
    .map((s) => ({ id: s.replace(/^-/, ''), desc: s.startsWith('-') }))
}

export function formatSort(sorting: SortingState): string | undefined {
  return sorting.length ? sorting.map((s) => (s.desc ? '-' : '') + s.id).join(',') : undefined
}

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const id = window.setTimeout(() => setV(value), ms)
    return () => window.clearTimeout(id)
  }, [value, ms])
  return v
}

export function ResourceList({ resource }: { resource: ResourceDefinition }) {
  const { api, meta, notify } = usePanel()
  const queryClient = useQueryClient()
  const urlState = useSearch({ strict: false }) as ListSearch
  const navigate = useNavigate()

  const page = urlState.page ?? 1
  const perPage = urlState.perPage ?? resource.perPage
  const sort = urlState.sort ?? resource.defaultSort
  const sorting = useMemo(() => parseSort(sort), [sort])

  // Only filters this resource defines; unknown filter[...] keys in the URL are ignored.
  const filterStates = useMemo(() => {
    const all = readFilterStates(urlState as Record<string, unknown>)
    const out: Record<string, FilterState> = {}
    for (const f of resource.filters) if (all[f.name]) out[f.name] = all[f.name]
    return out
  }, [urlState, resource])
  const filters = useMemo(() => filterParams(resource.filters, filterStates), [resource, filterStates])

  // Writes list state to the URL. Defaults are left out to keep URLs short.
  function update(patch: Partial<Pick<ListSearch, 'page' | 'perPage' | 'sort' | 'search' | 'group'>>, filterPatch?: Record<string, FilterState | undefined>) {
    void navigate({
      to: '.',
      replace: true,
      search: (prev: ListSearch) => {
        const next = { ...prev, ...patch }
        const out: ListSearch = {
          page: next.page && next.page > 1 ? next.page : undefined,
          perPage: next.perPage && next.perPage !== resource.perPage ? next.perPage : undefined,
          sort: next.sort && next.sort !== resource.defaultSort ? next.sort : undefined,
          search: next.search || undefined,
          group: next.group || undefined,
        }
        const states = { ...readFilterStates(prev as Record<string, unknown>), ...filterPatch }
        for (const [name, state] of Object.entries(states)) if (state) out[filterKey(name) as `filter[${string}]`] = urlValue(state)
        return out
      },
    })
  }
  const setFilter = (name: string, state: FilterState | undefined) => update({ page: 1 }, { [name]: state })
  // Tabs only for a filter the resource defines.
  const listTabs = resource.tabs && resource.filters.some((f) => f.name === resource.tabs!.filter) ? resource.tabs : undefined
  const resetFilters = () => update({ page: 1 }, Object.fromEntries(resource.filters.map((f) => [f.name, undefined] as const)))

  const [searchInput, setSearchInput] = useState(urlState.search ?? '')
  const debouncedSearch = useDebounced(searchInput.trim(), 300)
  useEffect(() => {
    if (debouncedSearch !== (urlState.search ?? '')) update({ search: debouncedSearch, page: 1 })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedSearch])

  // Columns the user has not hidden.
  const [hidden, setHidden] = useHiddenColumns(resource)
  const hiddenKey = [...hidden].sort().join(',')
  const shownColumns = useMemo(
    () => resource.columns.filter((c) => !hidden.has(c.name)),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [resource, hiddenKey],
  )
  const group = resource.groups.find((g) => g.name === urlState.group)

  // Manual order mode: rows sorted by the order column, moved by drag and drop.
  // The order column, when the user may reorder (its own permission, as in MKSine).
  const reorderColumn = meta.resources[resource.name]?.abilities.reorder === false ? '' : (meta.resources[resource.name]?.capabilities.reorderable ?? '')
  const [reordering, setReordering] = useState(false)

  // What the server sorts by: the manual order while reordering; with a group, the group
  // column first so its rows come together.
  const effectiveSort = reordering
    ? reorderColumn
    : group
      ? [group.name, ...(sort ?? '').split(',').filter((s) => s && s.replace(/^-/, '') !== group.name)].join(',')
      : sort
  const summarize = shownColumns.flatMap((c) => c._summaries.map((fn) => `${c.name}.${fn}`))

  const include = useMemo(
    () => [...new Set([...shownColumns.flatMap((c) => c.relations), ...(group?.relations ?? [])])],
    [shownColumns, group],
  )
  const searchable = resource.columns.some((c) => c._searchable)
  const key = meta.resources[resource.name]?.key ?? 'id'

  const resourceAbilities = meta.resources[resource.name]?.abilities
  const rowActions = useMemo(() => resource.actions.filter((a) => a._placements.has('row')), [resource])
  const bulkActions = resource.actions.filter((a) => a._placements.has('bulk'))
  const headerActions = resource.actions.filter(
    (a) => a._placements.has('header') && resourceAbilities?.header_actions[a.name] !== false,
  )
  const canCreate = resource.fields.length > 0 && resourceAbilities?.create !== false

  // Selection is per page view: it resets when the page, sort, search or page size changes.
  const [rowSelection, setRowSelection] = useState<RowSelectionState>({})
  const viewKey = JSON.stringify([page, perPage, effectiveSort, urlState.search, filters, reordering])
  useEffect(() => setRowSelection({}), [viewKey])

  const runner = useActionRunner(resource, () => setRowSelection({}))

  const query = useQuery({
    queryKey: ['records', resource.name, { page, perPage, search: urlState.search, sort: effectiveSort, include, filters, summarize }],
    queryFn: () =>
      api.list(resource.name, {
        page,
        perPage,
        search: urlState.search,
        sort: effectiveSort ? effectiveSort.split(',') : undefined,
        filters,
        include,
        summarize,
      }),
    placeholderData: keepPreviousData,
  })

  // Per-record abilities of the current page; read by cells at render time.
  const abilitiesRef = useRef<Record<string, RecordAbilities>>({})
  abilitiesRef.current = query.data?.abilities ?? {}
  const abilitiesOf = (record: ResourceRecord) => abilitiesRef.current[String(record[key])]

  // The rows as the user arranged them while reordering (shown before the server confirms).
  const [order, setOrder] = useState<ResourceRecord[] | null>(null)
  useEffect(() => {
    if (!reordering) setOrder(null)
    else if (query.data && !query.isPlaceholderData) setOrder(query.data.data)
  }, [reordering, query.data, query.isPlaceholderData])
  const orderRef = useRef(order)
  orderRef.current = order
  const dragFrom = useRef<number | null>(null)
  const [dragging, setDragging] = useState<number | null>(null)

  /** Moves a row and saves the new order of the rows between the old and new place. */
  async function move(from: number, to: number) {
    const before = orderRef.current
    if (!before || from === to || to < 0 || to >= before.length) return
    const next = [...before]
    const [item] = next.splice(from, 1)
    next.splice(to, 0, item)
    setOrder(next)
    const changed = next.slice(Math.min(from, to), Math.max(from, to) + 1)
    try {
      await api.reorder(resource.name, changed.map((r) => r[key] as string | number))
      await queryClient.invalidateQueries({ queryKey: ['records', resource.name] })
    } catch (err) {
      setOrder(before)
      notify(err instanceof Error ? err.message : String(err), 'danger')
    }
  }
  const moveRef = useRef(move)
  moveRef.current = move

  const columns = useMemo<ColumnDef<typeof features, ResourceRecord>[]>(() => {
    const cols: ColumnDef<typeof features, ResourceRecord>[] = shownColumns.map((c) => ({
      id: c.name,
      accessorFn: (row: ResourceRecord) => c.value(row),
      header: c._label ?? humanize(c.name),
      enableSorting: c._sortable && !reordering,
      cell: ({ row }) => <Cell column={c} record={row.original} />,
    }))
    if (reordering) {
      cols.unshift({
        id: HANDLE,
        enableSorting: false,
        header: () => <span className="sr-only">{t('list.reorderHandle')}</span>,
        cell: ({ row, table }) => (
          <ReorderHandle
            title={resource.recordTitle(row.original)}
            index={row.index}
            count={table.getRowModel().rows.length}
            disabled={abilitiesOf(row.original)?.update === false}
            onMove={(to) => void moveRef.current(row.index, to)}
          />
        ),
      })
      return cols
    }
    if (resource.actions.some((a) => a._placements.has('bulk'))) {
      cols.unshift({
        id: SELECT,
        enableSorting: false,
        header: ({ table }) => (
          <input
            type="checkbox"
            aria-label={t('list.selectAll')}
            className="mks-check"
            checked={table.getIsAllPageRowsSelected()}
            disabled={!table.getRowModel().rows.some((r) => r.getCanSelect())}
            ref={(el) => {
              if (el) el.indeterminate = table.getIsSomePageRowsSelected() && !table.getIsAllPageRowsSelected()
            }}
            onChange={table.getToggleAllPageRowsSelectedHandler()}
          />
        ),
        cell: ({ row }) => (
          <input
            type="checkbox"
            aria-label={t('list.selectRow', { name: resource.recordTitle(row.original) })}
            className="mks-check"
            checked={row.getIsSelected()}
            disabled={!row.getCanSelect()}
            onChange={row.getToggleSelectedHandler()}
          />
        ),
      })
    }
    if (resource.fields.length > 0 || rowActions.length > 0) {
      cols.push({
        id: ACTIONS,
        header: () => <span className="sr-only">{t('common.actions')}</span>,
        enableSorting: false,
        cell: ({ row }) => (
          <RowActions
            resource={resource}
            record={row.original}
            recordKey={String(row.original[key])}
            actions={rowActions}
            abilities={abilitiesOf(row.original)}
            onRun={(action) => runner.start(action, [row.original])}
          />
        ),
      })
    }
    return cols
    // runner.start is stable enough: it only reads state through setters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [resource, key, rowActions, shownColumns, reordering])

  const table = useTable({
    features,
    columns,
    data: (reordering ? order : null) ?? query.data?.data ?? emptyRows,
    manualSorting: true,
    enableMultiSort: true,
    state: { sorting, rowSelection },
    // A row can be selected when at least one bulk action is allowed for it.
    enableRowSelection: (row) => bulkActions.some((a) => allowedFor(a, abilitiesOf(row.original))),
    onRowSelectionChange: (updater: Updater<RowSelectionState>) =>
      setRowSelection((prev) => (typeof updater === 'function' ? updater(prev) : updater)),
    onSortingChange: (updater: Updater<SortingState>) => {
      const next = typeof updater === 'function' ? updater(sorting) : updater
      update({ sort: formatSort(next), page: 1 })
    },
    getRowId: (row: ResourceRecord, index: number) => String(row[key] ?? index),
  })

  const pageMeta = query.data?.meta
  const selected = table.getSelectedRowModel().rows.map((r) => r.original)
  // Bulk buttons for actions allowed on every selected record.
  const selectedActions = bulkActions.filter((a) => selected.every((rec) => allowedFor(a, abilitiesOf(rec))))
  // A page past the end (e.g. after deleting rows) jumps back to the last page.
  useEffect(() => {
    if (pageMeta && page > pageMeta.last_page) update({ page: pageMeta.last_page })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pageMeta?.last_page, page])

  const colCount = columns.length
  const defs = useMemo(() => new Map(resource.columns.map((c) => [c.name, c])), [resource])

  return (
    <div className="space-y-6">
      <header className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">{resource.pluralLabel}</h1>
          {pageMeta && (
            <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {nf.format(pageMeta.total)} {resource.label}
            </p>
          )}
        </div>
        <div className="flex flex-wrap items-center gap-2">
        {headerActions.map((action) =>
          action instanceof LinkAction ? (
            // A header link: a path inside the panel ("/page_trash") or an address.
            <PanelLink key={action.name} url={action._url({} as never)} newTab={action._newTab} className={buttonBase + ' ' + actionButtonColors[action._color === 'primary' ? 'gray' : action._color]}>
              {actionLabel(action)}
            </PanelLink>
          ) : (
            <button
              key={action.name}
              type="button"
              onClick={() => runner.start(action)}
              className={buttonBase + ' ' + actionButtonColors[action._color === 'primary' ? 'gray' : action._color]}
            >
              {actionLabel(action)}
            </button>
          ),
        )}
        {canCreate && (
          <Link
            to="/$resource/create"
            params={{ resource: resource.name }}
            className={buttonBase + ' ' + actionButtonColors.primary}
          >
            <PlusIcon />
            {t('common.new', { label: resource.label })}
          </Link>
        )}
        </div>
      </header>

      {listTabs && (
        <div role="tablist" aria-label={resource.pluralLabel} className="mb-4 flex flex-wrap gap-1 rounded-xl bg-gray-900/[0.04] p-1 sm:inline-flex dark:bg-white/5">
          {listTabs.tabs.map((tab) => {
            const active = (filterStates[listTabs.filter] || undefined) === (tab.value || undefined)
            return (
              <button
                key={tab.value ?? ''}
                type="button"
                role="tab"
                aria-selected={active}
                onClick={() => setFilter(listTabs.filter, tab.value || undefined)}
                className={
                  'rounded-lg px-3 py-1.5 text-sm font-medium transition ' +
                  (active
                    ? 'bg-white text-primary-700 shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-800 dark:text-primary-300 dark:ring-white/10'
                    : 'text-gray-600 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white')
                }
              >
                {tab.label}
              </button>
            )
          })}
        </div>
      )}

      {/* Not overflow-hidden: the filter panel must be able to leave the card. The first and
          last parts round their own corners instead. */}
      <div className="rounded-2xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10 [&>*:first-child]:rounded-t-2xl [&>*:last-child]:rounded-b-2xl">
        {selected.length > 0 && (
          <div
            role="region"
            aria-label={t('list.bulk')}
            className="flex animate-pop flex-wrap items-center gap-3 border-b border-primary-600/10 bg-primary-50/80 px-4 py-2.5 text-sm dark:bg-primary-500/10"
          >
            <span className="font-medium">{t('list.selected', { count: selected.length })}</span>
            {selectedActions.map((action) => (
              <button
                key={action.name}
                type="button"
                onClick={() => runner.start(action, selected)}
                className={buttonSmall + ' ' + actionButtonColors[action._color]}
              >
                {actionLabel(action)}
              </button>
            ))}
            <button type="button" onClick={() => setRowSelection({})} className="ms-auto text-gray-600 hover:text-gray-800 dark:text-gray-300">
              {t('list.clearSelection')}
            </button>
          </div>
        )}

        {(searchable || resource.filters.length > 0 || resource.groups.length > 0 || reorderColumn || resource.columns.some((c) => c._toggleable !== 'no')) && (
          <div className="flex flex-wrap items-center gap-2 border-b border-gray-100 p-3 dark:border-white/5">
            {searchable && (
              <div className="relative w-full sm:max-w-72">
                <span className="pointer-events-none absolute inset-y-0 start-3 flex items-center text-gray-400">
                  <SearchIcon />
                </span>
                <input
                  type="search"
                  value={searchInput}
                  onChange={(e) => setSearchInput(e.target.value)}
                  placeholder={t('common.searchPlaceholder')}
                  aria-label={t('common.searchIn', { name: resource.pluralLabel })}
                  className={inputClass + ' ps-9'}
                />
              </div>
            )}
            <div className="flex-1" />
            {resource.groups.length > 0 && !reordering && (
              <label className="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
                <GroupIcon />
                <span className="sr-only sm:not-sr-only">{t('list.group')}</span>
                <select
                  value={group?.name ?? ''}
                  onChange={(e) => update({ group: e.target.value || undefined, page: 1 })}
                  className="rounded-lg border-0 bg-white py-1.5 ps-3 text-sm font-medium text-gray-700 shadow-xs ring-1 ring-gray-950/10 hover:ring-gray-950/20 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:[&>option]:bg-gray-900"
                >
                  <option value="">{t('list.noGroup')}</option>
                  {resource.groups.map((g) => (
                    <option key={g.name} value={g.name}>
                      {g._label ?? humanize(g.name)}
                    </option>
                  ))}
                </select>
              </label>
            )}
            {resource.filters.length > 0 && (
              <FiltersMenu resource={resource} states={filterStates} onChange={setFilter} onReset={resetFilters} />
            )}
            <ColumnsMenu resource={resource} hidden={hidden} onChange={setHidden} />
            {reorderColumn && (reordering || Object.values(query.data?.abilities ?? {}).some((a) => a.update)) && (
              <button
                type="button"
                aria-pressed={reordering}
                onClick={() => setReordering(!reordering)}
                className={reordering ? toolButton + ' bg-primary-50 text-primary-700 ring-primary-600/30 dark:bg-primary-400/10 dark:text-primary-300' : toolButton}
              >
                <ReorderIcon />
                {reordering ? t('list.reorderDone') : t('list.reorder')}
              </button>
            )}
          </div>
        )}
        <ActiveFilters resource={resource} states={filterStates} onChange={setFilter} onReset={resetFilters} />

        {query.isError ? (
          <p className="p-6 text-sm text-danger-600">{t('common.fetchError', { message: query.error.message })}</p>
        ) : (
          <div className="overflow-x-auto">
            <table className={'w-full text-sm whitespace-nowrap ' + (query.isFetching && !query.isPending ? 'opacity-60' : '')}>
              <thead className="border-b border-gray-100 bg-gray-50/70 dark:border-white/5 dark:bg-white/[0.02]">
                {table.getHeaderGroups().map((group) => (
                  <tr key={group.id}>
                    {group.headers.map((header) => {
                      const def = defs.get(header.column.id)
                      const sorted = header.column.getIsSorted()
                      return (
                        <th
                          key={header.id}
                          scope="col"
                          aria-sort={sorted === 'asc' ? 'ascending' : sorted === 'desc' ? 'descending' : undefined}
                          className={'px-4 py-3 text-xs font-semibold text-gray-500 dark:text-gray-400 ' + alignClass[def?._align ?? 'start']}
                        >
                          {header.column.getCanSort() ? (
                            <button
                              type="button"
                              onClick={header.column.getToggleSortingHandler()}
                              className="group inline-flex items-center gap-1 rounded hover:text-gray-900 dark:hover:text-white"
                            >
                              <table.FlexRender header={header} />
                              <span className={sorted ? 'text-gray-700 dark:text-gray-200' : 'text-gray-300 group-hover:text-gray-500 dark:text-gray-600'}>
                                <ChevronIcon dir={sorted === 'asc' ? 'up' : sorted === 'desc' ? 'down' : 'both'} />
                              </span>
                            </button>
                          ) : (
                            <table.FlexRender header={header} />
                          )}
                        </th>
                      )
                    })}
                  </tr>
                ))}
              </thead>
              <tbody className="divide-y divide-gray-100 dark:divide-white/5">
                {query.isPending &&
                  Array.from({ length: 5 }, (_, i) => (
                    <tr key={i}>
                      <td colSpan={colCount} className="px-4 py-3">
                        <div className="h-5 animate-pulse rounded-md bg-gray-100 dark:bg-white/5" />
                      </td>
                    </tr>
                  ))}
                {table.getRowModel().rows.map((row, i, rows) => {
                  // A heading row where a new group starts.
                  const heading =
                    group && !reordering && (i === 0 || group.keyOf(rows[i - 1].original) !== group.keyOf(row.original)) ? (
                      <tr key={`group-${group.keyOf(row.original)}-${i}`} className="bg-gray-50/80 dark:bg-white/[0.03]">
                        <th colSpan={colCount} scope="rowgroup" className="px-4 py-2 text-start text-xs font-semibold text-gray-600 dark:text-gray-300">
                          {group._label ?? humanize(group.name)}: {group.titleOf(row.original)}
                        </th>
                      </tr>
                    ) : null
                  const canDrag = reordering && abilitiesOf(row.original)?.update !== false
                  // Keyed by record, so a moved row keeps its DOM (and the focused button).
                  return (
                    <Fragment key={row.id}>
                    {heading}
                    <tr
                      draggable={canDrag}
                      onDragStart={(e) => {
                        dragFrom.current = row.index
                        setDragging(row.index)
                        e.dataTransfer.effectAllowed = 'move'
                      }}
                      onDragEnd={() => setDragging(null)}
                      onDragOver={(e) => {
                        if (reordering && dragFrom.current !== null) e.preventDefault()
                      }}
                      onDrop={(e) => {
                        e.preventDefault()
                        if (dragFrom.current !== null) void move(dragFrom.current, row.index)
                        dragFrom.current = null
                        setDragging(null)
                      }}
                      className={
                        'transition-colors hover:bg-gray-50/80 dark:hover:bg-white/[0.03] ' +
                        (row.getIsSelected() ? 'bg-primary-50/50 dark:bg-primary-500/5 ' : '') +
                        (canDrag ? 'cursor-grab active:cursor-grabbing ' : '') +
                        (dragging === row.index ? 'opacity-40' : '')
                      }
                    >
                      {row.getAllCells().map((cell) => (
                        <td
                          key={cell.id}
                          className={'px-4 py-3 text-gray-700 dark:text-gray-300 ' + (cell.column.id === ACTIONS ? 'text-end' : alignClass[defs.get(cell.column.id)?._align ?? 'start'])}
                        >
                          <table.FlexRender cell={cell} />
                        </td>
                      ))}
                    </tr>
                    </Fragment>
                  )
                })}
                {query.data?.data.length === 0 && (
                  <tr>
                    <td colSpan={colCount} className="px-4 py-16 text-center">
                      <span className="mx-auto mb-4 flex size-12 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-white/5 dark:text-gray-500 [&_svg]:size-6">
                        <SearchIcon />
                      </span>
                      <p className="font-semibold text-gray-950 dark:text-white">{t('list.empty')}</p>
                      <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{t('list.emptyHint')}</p>
                      {urlState.search && (
                        <button
                          type="button"
                          onClick={() => setSearchInput('')}
                          className="mt-2 text-sm text-primary-600 hover:text-primary-500"
                        >
                          {t('list.clearSearch')}
                        </button>
                      )}
                      {filters.length > 0 && (
                        <button type="button" onClick={resetFilters} className="ms-4 mt-2 text-sm text-primary-600 hover:text-primary-500">
                          {t('list.clearFilters')}
                        </button>
                      )}
                    </td>
                  </tr>
                )}
              </tbody>
              {query.data?.summary && shownColumns.some((c) => c._summaries.length > 0) && (
                // Aggregates of every record that matches the search and filters, not just this page.
                <tfoot className="border-t border-gray-100 bg-gray-50/70 text-sm dark:border-white/5 dark:bg-white/[0.02]">
                  <tr>
                    {columns.map((col, i) => {
                      const def = defs.get(col.id ?? '')
                      const values = def && query.data.summary?.[def.name]
                      if (def && def._summaries.length > 0) {
                        return (
                          <td key={col.id} className={'px-4 py-2.5 align-top text-gray-700 dark:text-gray-300 ' + alignClass[def._align]}>
                            {def._summaries.map((fn) => (
                              <div key={fn}>
                                <span className="text-gray-500 dark:text-gray-400">{summaryLabels[fn]}: </span>
                                <span className="font-medium">{formatSummary(def, fn, values?.[fn])}</span>
                              </div>
                            ))}
                          </td>
                        )
                      }
                      return i === columns.findIndex((c) => !defs.get(c.id ?? '')?._summaries.length) ? (
                        <th key={col.id} scope="row" className="px-4 py-2.5 text-start align-top font-medium text-gray-500 dark:text-gray-400">
                          {t('list.allResults')}
                        </th>
                      ) : (
                        <td key={col.id} />
                      )
                    })}
                  </tr>
                </tfoot>
              )}
            </table>
          </div>
        )}

        {pageMeta && (
          <div className="flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 px-4 py-3 text-sm text-gray-600 dark:border-white/5 dark:text-gray-400">
            <div className="flex items-center gap-3">
              <span>{t('list.total', { count: pageMeta.total })}</span>
              <label className="flex items-center gap-2">
                <span className="sr-only sm:not-sr-only">{t('list.perPage')}</span>
                <select
                  value={perPage}
                  onChange={(e) => update({ perPage: Number(e.target.value), page: 1 })}
                  className="rounded-lg border-0 bg-white py-1 ps-2.5 text-sm shadow-xs ring-1 ring-gray-950/10 hover:ring-gray-950/20 dark:bg-white/5 dark:ring-white/10 dark:[&>option]:bg-gray-900"
                >
                  {PER_PAGE_OPTIONS.map((n) => (
                    <option key={n} value={n}>
                      {nf.format(n)}
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <Pager page={pageMeta.page} last={pageMeta.last_page} onChange={(p) => update({ page: p })} />
          </div>
        )}
      </div>
      {runner.modal}
    </div>
  )
}

/** Whether the abilities allow an action; unknown abilities allow (the server decides). */
function allowedFor(action: Action<any>, abilities: RecordAbilities | undefined): boolean {
  if (!abilities) return true
  if (action.kind === 'delete') return abilities.delete
  if (action.kind === 'server') return abilities.actions[action.name] !== false
  return true
}

function RowActions({
  resource,
  record,
  recordKey,
  actions,
  abilities,
  onRun,
}: {
  resource: ResourceDefinition
  record: ResourceRecord
  recordKey: string
  actions: Action<any>[]
  abilities?: RecordAbilities
  onRun: (action: Action<any>) => void
}) {
  const title = resource.recordTitle(record)
  const canUpdate = abilities?.update !== false
  return (
    <div className="flex items-center justify-end gap-3">
      {actions
        .filter((a) => a.isVisibleFor(record) && allowedFor(a, abilities))
        .map((action) =>
          action instanceof LinkAction ? (
            <PanelLink key={action.name} url={action._url(record)} newTab={action._newTab} className={'text-sm font-semibold ' + actionTextColors[action._color]}>
              {actionLabel(action)}
            </PanelLink>
          ) : (
            <button
              key={action.name}
              type="button"
              onClick={() => onRun(action)}
              aria-label={`${actionLabel(action)} ${title}`}
              className={'text-sm font-semibold ' + actionTextColors[action._color]}
            >
              {actionLabel(action)}
            </button>
          ),
        )}
      <RecordLinks resource={resource} recordKey={recordKey} title={title} canUpdate={canUpdate} />
    </div>
  )
}

/** Page numbers around the current page, with the first and last always shown. */
export function pageItems(page: number, last: number): (number | 'gap')[] {
  const pages = new Set([1, last, page - 1, page, page + 1].filter((p) => p >= 1 && p <= last))
  const sorted = [...pages].sort((a, b) => a - b)
  const out: (number | 'gap')[] = []
  sorted.forEach((p, i) => {
    if (i > 0 && p - sorted[i - 1] > 1) out.push('gap')
    out.push(p)
  })
  return out
}

function Pager({ page, last, onChange }: { page: number; last: number; onChange: (page: number) => void }) {
  if (last <= 1) return null
  const btn =
    'min-w-8 rounded-lg px-2 py-1 text-sm font-medium text-gray-600 hover:bg-gray-900/5 disabled:opacity-40 dark:text-gray-300 dark:hover:bg-white/5'
  return (
    <nav className="flex items-center gap-1" aria-label={t('list.pagination')}>
      <button type="button" className={btn} disabled={page <= 1} onClick={() => onChange(page - 1)}>
        {t('common.previous')}
      </button>
      {pageItems(page, last).map((p, i) =>
        p === 'gap' ? (
          <span key={`gap-${i}`} className="px-1">…</span>
        ) : (
          <button
            key={p}
            type="button"
            onClick={() => onChange(p)}
            aria-current={p === page ? 'page' : undefined}
            className={btn + (p === page ? ' bg-primary-600 text-white! shadow-sm shadow-primary-600/30 hover:bg-primary-600 dark:bg-primary-500' : '')}
          >
            {nf.format(p)}
          </button>
        ),
      )}
      <button type="button" className={btn} disabled={page >= last} onClick={() => onChange(page + 1)}>
        {t('common.next')}
      </button>
    </nav>
  )
}

/** Moves a row in manual order mode: drag the row, or use the buttons (keyboard). */
function ReorderHandle({ title, index, count, disabled, onMove }: { title: string; index: number; count: number; disabled: boolean; onMove: (to: number) => void }) {
  const button = 'rounded p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 aria-disabled:opacity-30 dark:hover:bg-white/10 [&_svg]:size-4'
  // aria-disabled, not disabled: a row moved to the top keeps focus on its button.
  const props = (to: number, off: boolean) => ({
    'aria-disabled': off || undefined,
    onClick: () => !off && onMove(to),
  })
  return (
    <div className="flex items-center gap-1">
      <span aria-hidden="true" className="px-1 text-gray-400 select-none">
        ⠿
      </span>
      <button type="button" className={button} {...props(index - 1, disabled || index === 0)} aria-label={t('common.moveUp', { name: title })}>
        <ChevronIcon dir="up" />
      </button>
      <button type="button" className={button} {...props(index + 1, disabled || index === count - 1)} aria-label={t('common.moveDown', { name: title })}>
        <ChevronIcon dir="down" />
      </button>
    </div>
  )
}

/**
 * A link action's address: a path inside the panel ("/menu_builder#3", without the base
 * path) goes through the router; anything else is a plain link.
 */
function PanelLink({ url, newTab, className, children }: { url: string; newTab: boolean; className: string; children: ReactNode }) {
  if (url.startsWith('/') && !url.startsWith('//') && !newTab) {
    const [path, hash] = url.split('#', 2)
    return (
      <Link to={path} hash={hash} className={className}>
        {children}
      </Link>
    )
  }
  return (
    <a href={url} {...(newTab ? { target: '_blank', rel: 'noopener noreferrer' } : {})} className={className}>
      {children}
    </a>
  )
}
