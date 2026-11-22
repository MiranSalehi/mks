import { useState, type ReactNode } from 'react'
import { defaultFileUrl } from '../form'
import type { ResourceRecord } from '../protocol'
import {
  BadgeColumn,
  BooleanColumn,
  DateColumn,
  ImageColumn,
  resolve,
  TextColumn,
  type Color,
  type Column,
} from '../table'
import { CustomColumn } from '../extensions'
import { CheckIcon, CopyIcon, XIcon } from './icons'
import { dateFormat, localeTag, numberFormat, t } from '../i18n'

export const badgeColors: Record<Color, string> = {
  gray: 'bg-gray-100/80 text-gray-700 ring-gray-600/10 dark:bg-white/5 dark:text-gray-300 dark:ring-white/10',
  primary: 'bg-primary-50 text-primary-700 ring-primary-600/15 dark:bg-primary-500/10 dark:text-primary-300 dark:ring-primary-400/20',
  success: 'bg-success-50 text-success-700 ring-success-600/15 dark:bg-success-500/10 dark:text-success-300 dark:ring-success-400/20',
  warning: 'bg-warning-50 text-warning-700 ring-warning-600/20 dark:bg-warning-500/10 dark:text-warning-300 dark:ring-warning-400/20',
  danger: 'bg-danger-50 text-danger-700 ring-danger-600/15 dark:bg-danger-500/10 dark:text-danger-300 dark:ring-danger-400/20',
  info: 'bg-info-50 text-info-700 ring-info-600/15 dark:bg-info-500/10 dark:text-info-300 dark:ring-info-400/20',
}

const textColors: Record<Color, string> = {
  gray: 'text-gray-500 dark:text-gray-400',
  primary: 'text-primary-600 dark:text-primary-400',
  success: 'text-success-600 dark:text-success-400',
  warning: 'text-warning-600 dark:text-warning-400',
  danger: 'text-danger-600 dark:text-danger-400',
  info: 'text-info-600 dark:text-info-400',
}

const weights = { medium: 'font-medium', semibold: 'font-semibold', bold: 'font-bold' } as const

// Shared formatters: creating Intl objects per cell is slow.
const dateFmt = dateFormat({ dateStyle: 'medium' })
const dateTimeFmt = dateFormat({ dateStyle: 'medium', timeStyle: 'short' })
const relativeFmt = new Intl.RelativeTimeFormat(localeTag(), { numeric: 'auto' })

const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ['year', 365 * 24 * 3600],
  ['month', 30 * 24 * 3600],
  ['week', 7 * 24 * 3600],
  ['day', 24 * 3600],
  ['hour', 3600],
  ['minute', 60],
]

export function relativeTime(date: Date, now = Date.now()): string {
  const seconds = Math.round((date.getTime() - now) / 1000)
  for (const [unit, size] of units) {
    // Round the magnitude so past and future round the same way (Math.round(-1.5) is -1).
    if (Math.abs(seconds) >= size) return relativeFmt.format(Math.sign(seconds) * Math.round(Math.abs(seconds) / size), unit)
  }
  return relativeFmt.format(0, 'minute')
}

export function Cell({ column, record }: { column: Column; record: ResourceRecord }): ReactNode {
  const value = column.value(record)
  const description = column.descriptionOf(record)
  const empty = value == null || value === ''

  let content: ReactNode
  if (column instanceof CustomColumn) {
    // A plugin's column draws everything itself, empty values included.
    const Render = column.component
    content = <Render value={value} record={record} options={column._options} />
  } else if (empty && !(column instanceof BooleanColumn)) {
    content = <span className="text-gray-400 dark:text-gray-600">{column._placeholder}</span>
  } else if (column instanceof BooleanColumn) {
    content = <BooleanCell column={column} value={Boolean(value)} />
  } else if (column instanceof ImageColumn) {
    content = <ImageCell column={column} value={value} record={record} />
  } else if (column instanceof BadgeColumn) {
    const key = String(value)
    content = (
      <span className={'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ' + badgeColors[column._colors[key] ?? 'gray']}>
        <span aria-hidden="true" className="size-1.5 rounded-full bg-current opacity-70" />
        {column._format ? column._format(value, record) : (column._labels[key] ?? key)}
      </span>
    )
  } else if (column instanceof DateColumn) {
    content = <DateCell column={column} value={String(value)} record={record} />
  } else if (column instanceof TextColumn) {
    content = <TextCell column={column} value={value} record={record} />
  } else {
    content = column._format ? column._format(value, record) : String(value)
  }

  if (!description) return content
  return (
    <div>
      <div>{content}</div>
      <div className="text-xs text-gray-500 dark:text-gray-400">{description}</div>
    </div>
  )
}

function TextCell({ column, value, record }: { column: TextColumn; value: unknown; record: ResourceRecord }) {
  let text: string
  if (column._format) text = column._format(value, record)
  else if (column._numeric && typeof value === 'number') text = numberFormat(column._numeric).format(value)
  else text = String(value)

  const full = text
  if (column._limit && text.length > column._limit) text = text.slice(0, column._limit) + '…'

  const color = resolve(column._color, value, record)
  const cls = [color ? textColors[color] : '', column._weight ? weights[column._weight] : ''].join(' ').trim()

  const url = column._url?.(record)
  let node: ReactNode = (
    <span className={cls || undefined} title={full !== text ? full : undefined}>
      {text}
    </span>
  )
  if (url) {
    node = (
      <a
        href={url}
        className={'hover:underline ' + (cls || 'text-primary-600 dark:text-primary-400')}
        {...(column._openInNewTab ? { target: '_blank', rel: 'noopener noreferrer' } : {})}
      >
        {text}
      </a>
    )
  }
  if (!column._copyable) return node
  return (
    <span className="inline-flex items-center gap-1">
      {node}
      <CopyButton text={String(value)} />
    </span>
  )
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false)
  return (
    <button
      type="button"
      onClick={async () => {
        await navigator.clipboard.writeText(text)
        setCopied(true)
        window.setTimeout(() => setCopied(false), 1500)
      }}
      className="rounded p-0.5 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200"
      aria-label={copied ? t('common.copied') : t('common.copy')}
      title={copied ? t('common.copied') : t('common.copy')}
    >
      {copied ? <CheckIcon small /> : <CopyIcon />}
    </button>
  )
}

function BooleanCell({ column, value }: { column: BooleanColumn; value: boolean }) {
  const label = value ? column._trueLabel : column._falseLabel
  return (
    <span className={'inline-flex ' + textColors[value ? column._trueColor : column._falseColor]} title={label}>
      {value ? <CheckIcon /> : <XIcon />}
      <span className="sr-only">{label}</span>
    </span>
  )
}

export function ImageCell({ column, value, record }: { column: ImageColumn; value: unknown; record: ResourceRecord }) {
  const [broken, setBroken] = useState(false)
  const src = column._src ? column._src(value, record) : defaultFileUrl(String(value))
  if (!src || broken) return <span className="text-gray-400 dark:text-gray-600">{column._placeholder}</span>
  return (
    <img
      src={src}
      alt=""
      loading="lazy"
      onError={() => setBroken(true)}
      width={column._size}
      height={column._size}
      style={{ width: column._size, height: column._size }}
      className={'object-cover ring-1 ring-gray-950/5 dark:ring-white/10 ' + (column._circular ? 'rounded-full' : 'rounded-md')}
    />
  )
}

function DateCell({ column, value, record }: { column: DateColumn; value: string; record: ResourceRecord }) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  if (column._format) return column._format(value, record)
  const full = (column._withTime ? dateTimeFmt : dateFmt).format(date)
  if (column._since) {
    return (
      <time dateTime={value} title={dateTimeFmt.format(date)}>
        {relativeTime(date)}
      </time>
    )
  }
  return <time dateTime={value}>{full}</time>
}
