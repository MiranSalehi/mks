import { lazy, Suspense, type ReactNode } from 'react'
import {
  Entry,
  HtmlEntry,
  InfoGrid,
  InfoLayout,
  InfoSection,
  KeyValueEntry,
  ListEntry,
  MarkdownEntry,
  RepeatableEntry,
  type InfoNode,
} from '../infolist'
import type { ResourceRecord } from '../protocol'
import { humanize } from '../resource'
import { BooleanColumn, Column, ImageColumn, type Span } from '../table'
import { Cell, ImageCell } from './cells'
import { numberFormat } from '../i18n'

const RichContent = lazy(() => import('./richContent'))
const nf = numberFormat()

const gridCols: Record<number, string> = {
  1: 'grid-cols-1',
  2: 'grid-cols-1 sm:grid-cols-2',
  3: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3',
  4: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-4',
}
const spanClass = (span: Span | undefined) =>
  span === 'full' ? 'col-span-full' : span === 2 ? 'sm:col-span-2' : span === 3 ? 'sm:col-span-3' : span === 4 ? 'sm:col-span-4' : ''

const card = 'rounded-2xl bg-white shadow-sm ring-1 ring-gray-950/5 dark:bg-gray-900/70 dark:ring-white/10'

/** Shows a record read-only. Sections marked aside() go to a side column on wide screens. */
export function Infolist({ nodes, record }: { nodes: InfoNode<any>[]; record: ResourceRecord }) {
  const aside = nodes.filter((n) => n instanceof InfoSection && n._aside)
  const main = nodes.filter((n) => !aside.includes(n))
  const body = <Nodes nodes={main} record={record} columns={2} top />
  if (aside.length === 0) return body
  return (
    <div className="grid items-start gap-6 lg:grid-cols-3">
      <div className="space-y-6 lg:col-span-2">{body}</div>
      <div className="space-y-6">
        <Nodes nodes={aside} record={record} columns={1} top />
      </div>
    </div>
  )
}

/**
 * Entries in a grid. At the top level, loose entries share a card and sections bring
 * their own.
 */
function Nodes({ nodes, record, columns, top = false }: { nodes: InfoNode<any>[]; record: ResourceRecord; columns: number; top?: boolean }) {
  if (top) {
    // Group runs of loose entries into cards between sections.
    const blocks: ReactNode[] = []
    let loose: InfoNode<any>[] = []
    const flush = () => {
      if (loose.length) {
        blocks.push(
          <div key={`loose-${blocks.length}`} className={card + ' p-6'}>
            <Grid nodes={loose} record={record} columns={columns} />
          </div>,
        )
        loose = []
      }
    }
    for (const n of nodes) {
      if (n instanceof InfoSection) {
        flush()
        blocks.push(<SectionView key={`section-${blocks.length}`} section={n} record={record} />)
      } else {
        loose.push(n)
      }
    }
    flush()
    return <div className="space-y-6">{blocks}</div>
  }
  return <Grid nodes={nodes} record={record} columns={columns} />
}

function Grid({ nodes, record, columns }: { nodes: InfoNode<any>[]; record: ResourceRecord; columns: number }) {
  return (
    <dl className={'grid gap-x-8 gap-y-6 ' + (gridCols[columns] ?? gridCols[2])}>
      {nodes.map((n, i) => (
        <NodeView key={i} node={n} record={record} />
      ))}
    </dl>
  )
}

function NodeView({ node, record }: { node: InfoNode<any>; record: ResourceRecord }) {
  if (node instanceof InfoSection) {
    return (
      <div className="col-span-full">
        <SectionView section={node} record={record} nested />
      </div>
    )
  }
  if (node instanceof InfoGrid || node instanceof InfoLayout) {
    return (
      <div className="col-span-full">
        <Grid nodes={node._children} record={record} columns={node._columns} />
      </div>
    )
  }
  if (node._visible && !node._visible(record)) return null
  const label = node._label ?? humanize(node.name)
  // Long content takes the whole row unless the entry says otherwise.
  const wide = node instanceof HtmlEntry || node instanceof MarkdownEntry || node instanceof RepeatableEntry || node instanceof KeyValueEntry
  return (
    <div className={spanClass(node._columnSpan ?? (wide ? 'full' : undefined)) + ' min-w-0'}>
      {label && <dt className="text-sm font-medium text-gray-500 dark:text-gray-400">{label}</dt>}
      <dd className="mt-1.5 text-sm text-gray-950 dark:text-white">
        <EntryValue node={node} record={record} />
      </dd>
    </div>
  )
}

function SectionView({ section, record, nested = false }: { section: InfoSection<any>; record: ResourceRecord; nested?: boolean }) {
  return (
    <section className={nested ? 'rounded-xl bg-gray-50/60 p-4 ring-1 ring-gray-950/5 dark:bg-white/[0.02] dark:ring-white/10' : card}>
      <header className={nested ? 'mb-4' : 'border-b border-gray-100 px-6 py-4 dark:border-white/5'}>
        <h2 className="text-base font-semibold tracking-tight text-gray-950 dark:text-white">{section.heading}</h2>
        {section._description && <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{section._description}</p>}
      </header>
      <div className={nested ? '' : 'p-6'}>
        <Grid nodes={section._children} record={record} columns={section._columns} />
      </div>
    </section>
  )
}

const empty = (placeholder: string) => <span className="text-gray-400 dark:text-gray-600">{placeholder}</span>

function EntryValue({ node, record }: { node: Column<any> | Entry<any>; record: ResourceRecord }): ReactNode {
  if (node instanceof Column) {
    const value = node.value(record)
    // Several images (e.g. a gallery) side by side.
    if (node instanceof ImageColumn && Array.isArray(value)) {
      if (value.length === 0) return empty(node._placeholder)
      return (
        <div className="flex flex-wrap gap-2">
          {value.map((v, i) => (
            <ImageCell key={i} column={node} value={v} record={record} />
          ))}
        </div>
      )
    }
    // There is room for words here: a boolean shows its label beside the icon.
    if (node instanceof BooleanColumn) {
      const on = Boolean(value)
      return (
        <span className="inline-flex items-center gap-1.5">
          <Cell column={node} record={record} />
          <span aria-hidden="true">{on ? node._trueLabel : node._falseLabel}</span>
        </span>
      )
    }
    return <Cell column={node} record={record} />
  }

  const value = node.value(record)
  if (node instanceof ListEntry) {
    const items = node.items(record)
    if (items.length === 0) return empty(node._placeholder)
    return (
      <ul className="flex flex-wrap gap-1.5">
        {items.map((item, i) => (
          <li key={i} className={'rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ' + chipColors[node._color]}>
            {item}
          </li>
        ))}
      </ul>
    )
  }
  if (node instanceof HtmlEntry || node instanceof MarkdownEntry) {
    if (typeof value !== 'string' || value.trim() === '') return empty(node._placeholder)
    return (
      <Suspense fallback={<div className="h-20 animate-pulse rounded-lg bg-gray-100 dark:bg-white/5" />}>
        {node instanceof HtmlEntry ? <RichContent html={value} /> : <RichContent markdown={value} />}
      </Suspense>
    )
  }
  if (node instanceof KeyValueEntry) {
    const pairs = value && typeof value === 'object' && !Array.isArray(value) ? Object.entries(value as Record<string, unknown>) : []
    if (pairs.length === 0) return empty(node._placeholder)
    return (
      <div className="overflow-hidden rounded-xl ring-1 ring-gray-950/5 dark:ring-white/10">
        <table className="w-full text-sm">
          <thead className="bg-gray-50/70 text-xs text-gray-500 dark:bg-white/[0.02] dark:text-gray-400">
            <tr>
              <th scope="col" className="px-4 py-2 text-start font-semibold">
                {node._keyLabel}
              </th>
              <th scope="col" className="px-4 py-2 text-start font-semibold">
                {node._valueLabel}
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-100 dark:divide-white/5">
            {pairs.map(([k, v]) => (
              <tr key={k}>
                <th scope="row" className="px-4 py-2 text-start font-medium text-gray-700 dark:text-gray-300" dir="auto">
                  {k}
                </th>
                <td className="px-4 py-2 text-gray-950 dark:text-white" dir="auto">
                  {String(v ?? '')}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    )
  }
  if (node instanceof RepeatableEntry) {
    const items = Array.isArray(value) ? (value as ResourceRecord[]) : []
    if (items.length === 0) return empty(node._placeholder)
    return (
      <ol className="space-y-3">
        {items.map((item, i) => (
          <li key={i} className="relative rounded-xl bg-gray-50/60 p-4 ps-12 ring-1 ring-gray-950/5 dark:bg-white/[0.02] dark:ring-white/10">
            <span aria-hidden="true" className="absolute start-4 top-4 flex size-6 items-center justify-center rounded-full bg-white text-xs font-semibold text-gray-500 ring-1 ring-gray-950/10 dark:bg-white/10 dark:text-gray-300 dark:ring-white/10">
              {nf.format(i + 1)}
            </span>
            <Grid nodes={node._schema} record={item && typeof item === 'object' ? item : {}} columns={node._columns} />
          </li>
        ))}
      </ol>
    )
  }
  return String(value ?? '')
}

const chipColors = {
  gray: 'bg-gray-100/80 text-gray-700 ring-gray-600/10 dark:bg-white/5 dark:text-gray-300 dark:ring-white/10',
  primary: 'bg-primary-50 text-primary-700 ring-primary-600/15 dark:bg-primary-500/10 dark:text-primary-300 dark:ring-primary-400/20',
  success: 'bg-success-50 text-success-700 ring-success-600/15 dark:bg-success-500/10 dark:text-success-300 dark:ring-success-400/20',
  warning: 'bg-warning-50 text-warning-700 ring-warning-600/20 dark:bg-warning-500/10 dark:text-warning-300 dark:ring-warning-400/20',
  danger: 'bg-danger-50 text-danger-700 ring-danger-600/15 dark:bg-danger-500/10 dark:text-danger-300 dark:ring-danger-400/20',
  info: 'bg-info-50 text-info-700 ring-info-600/15 dark:bg-info-500/10 dark:text-info-300 dark:ring-info-400/20',
}
