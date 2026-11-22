import { useEffect, useId, useRef, useState, type KeyboardEvent, type PointerEvent } from 'react'
import type { Color } from '../table'
import { formatNumber, numberFormat, t } from '../i18n'

export interface Series {
  label: string
  color: Color
  data: number[]
}

const nf = numberFormat({ maximumFractionDigits: 1 })
const compact = numberFormat({ notation: 'compact', maximumFractionDigits: 1 })
export const colorVar = (c: Color, shade = 500) => `var(--color-${c}-${shade})`

/** The container's width, for crisp text and lines at any size. */
function useWidth<E extends HTMLElement>() {
  const ref = useRef<E>(null)
  const [width, setWidth] = useState(0)
  useEffect(() => {
    const el = ref.current
    if (!el) return
    const ro = new ResizeObserver(([entry]) => setWidth(Math.floor(entry.contentRect.width)))
    ro.observe(el)
    return () => ro.disconnect()
  }, [])
  return [ref, width] as const
}

/**
 * Axis ticks with a round step (1, 2 or 5 times a power of ten), about four of them, so
 * counts never get fractional ticks: a maximum of 25 gives 0, 10, 20, 30.
 */
export function axisTicks(max: number): number[] {
  if (max <= 0) return [0, 1]
  const raw = max / 4
  const pow = 10 ** Math.floor(Math.log10(raw))
  const step = ([1, 2, 5, 10].find((m) => m * pow >= raw) ?? 10) * pow
  const ticks: number[] = []
  for (let t = 0; t < max + step; t += step) ticks.push(Number(t.toPrecision(12)))
  return ticks
}

/** The numbers as a table, for screen readers (and anyone who prefers one). */
function DataTable({ caption, labels, series }: { caption: string; labels: string[]; series: Series[] }) {
  return (
    <table className="sr-only">
      <caption>{caption}</caption>
      <thead>
        <tr>
          <th scope="col">{t('chart.label')}</th>
          {series.map((s) => (
            <th key={s.label} scope="col">
              {s.label}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {labels.map((l, i) => (
          <tr key={l + i}>
            <th scope="row">{l}</th>
            {series.map((s) => (
              <td key={s.label}>{nf.format(s.data[i] ?? 0)}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function Legend({ series }: { series: Series[] }) {
  if (series.length < 2) return null
  return (
    <ul className="mt-3 flex flex-wrap gap-4 text-xs text-gray-600 dark:text-gray-400" aria-hidden="true">
      {series.map((s) => (
        <li key={s.label} className="flex items-center gap-1.5">
          <span className="size-2.5 rounded-full" style={{ background: colorVar(s.color) }} />
          {s.label}
        </li>
      ))}
    </ul>
  )
}

const PAD_Y = { top: 12, bottom: 28 }
const AXIS = 40
const EDGE = 12

/**
 * Line (with a soft area) or grouped bars over labelled points. Pointer and arrow keys
 * move a highlight with a tooltip.
 */
export function CartesianChart({
  type,
  caption,
  labels,
  series,
  height = 260,
}: {
  type: 'line' | 'bar'
  caption: string
  labels: string[]
  series: Series[]
  height?: number
}) {
  const [ref, width] = useWidth<HTMLDivElement>()
  const [active, setActive] = useState<number | null>(null)
  const gradientId = useId()
  // In a right-to-left page the axis reads from the right: the first label is on the
  // right and the y axis sits on the right.
  const rtl = typeof document !== 'undefined' && document.documentElement.dir === 'rtl'
  const PAD = { ...PAD_Y, left: rtl ? EDGE : AXIS, right: rtl ? AXIS : EDGE }
  const n = labels.length
  const ticks = axisTicks(Math.max(0, ...series.flatMap((s) => s.data)))
  const max = ticks[ticks.length - 1]
  const plotW = Math.max(0, width - PAD.left - PAD.right)
  const plotH = height - PAD.top - PAD.bottom
  const band = n > 0 ? plotW / n : 0
  // The band of a label, counted from the left of the drawing.
  const pos = (i: number) => (rtl ? n - 1 - i : i)
  // Lines run between band centers; bars sit in bands.
  const x = (i: number) => PAD.left + band * pos(i) + band / 2
  const y = (v: number) => PAD.top + plotH - (v / max) * plotH
  // Show as many x labels as fit (about one per 56px).
  const every = Math.max(1, Math.ceil(n / Math.max(1, Math.floor(plotW / 56))))

  function onPointerMove(e: PointerEvent<SVGSVGElement>) {
    const rect = e.currentTarget.getBoundingClientRect()
    const b = Math.floor((e.clientX - rect.left - PAD.left) / band)
    setActive(b >= 0 && b < n ? pos(b) : null)
  }
  function onKeyDown(e: KeyboardEvent) {
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault()
      // Arrows move the way they point on screen.
      const step = (e.key === 'ArrowRight') !== rtl ? 1 : -1
      setActive((a) => Math.min(n - 1, Math.max(0, (a ?? (step > 0 ? -1 : n)) + step)))
    } else if (e.key === 'Escape') setActive(null)
  }

  const barGap = 4
  const groupW = Math.max(2, band * 0.64)
  const barW = Math.max(2, (groupW - barGap * (series.length - 1)) / series.length)

  return (
    <div ref={ref} dir="ltr" className="relative">
      {width > 0 && (
        <svg
          width={width}
          height={height}
          role="img"
          aria-label={caption}
          tabIndex={0}
          onKeyDown={onKeyDown}
          onPointerMove={onPointerMove}
          onPointerLeave={() => setActive(null)}
          onBlur={() => setActive(null)}
          className="block rounded-lg focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-primary-500"
        >
          <defs>
            {series.map((s, si) => (
              <linearGradient key={si} id={`${gradientId}-${si}`} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={colorVar(s.color)} stopOpacity="0.28" />
                <stop offset="100%" stopColor={colorVar(s.color)} stopOpacity="0" />
              </linearGradient>
            ))}
          </defs>

          {/* Grid and y axis. */}
          {ticks.map((t) => (
            <g key={t}>
              <line x1={PAD.left} x2={width - PAD.right} y1={y(t)} y2={y(t)} className="stroke-gray-200 dark:stroke-white/10" strokeDasharray={t === 0 ? undefined : '3 4'} />
              <text
                x={rtl ? width - PAD.right + 8 : PAD.left - 8}
                y={y(t)}
                dy="0.32em"
                textAnchor={rtl ? 'start' : 'end'}
                className="fill-gray-400 text-[11px] dark:fill-gray-500"
              >
                {compact.format(t)}
              </text>
            </g>
          ))}
          {labels.map((l, i) =>
            i % every === 0 ? (
              <text key={i} x={x(i)} y={height - 8} textAnchor="middle" className="fill-gray-400 text-[11px] dark:fill-gray-500">
                {l}
              </text>
            ) : null,
          )}

          {active !== null && (
            <rect x={PAD.left + band * pos(active)} y={PAD.top} width={band} height={plotH} className="fill-gray-900/[0.04] dark:fill-white/[0.04]" />
          )}

          {type === 'line'
            ? series.map((s, si) => {
                const points = s.data.map((v, i) => `${x(i)},${y(v)}`)
                if (points.length === 0) return null
                const line = `M${points.join('L')}`
                const area = `${line}L${x(n - 1)},${y(0)}L${x(0)},${y(0)}Z`
                return (
                  <g key={si}>
                    <path d={area} fill={`url(#${gradientId}-${si})`} />
                    <path d={line} fill="none" stroke={colorVar(s.color)} strokeWidth={2.25} strokeLinejoin="round" strokeLinecap="round" />
                    {active !== null && (
                      <circle cx={x(active)} cy={y(s.data[active] ?? 0)} r={4.5} fill={colorVar(s.color)} className="stroke-white dark:stroke-gray-900" strokeWidth={2} />
                    )}
                  </g>
                )
              })
            : labels.map((_, i) => (
                <g key={i}>
                  {series.map((s, si) => {
                    const v = s.data[i] ?? 0
                    const h = Math.max(0, y(0) - y(v))
                    return (
                      <rect
                        key={si}
                        x={PAD.left + band * pos(i) + (band - groupW) / 2 + (rtl ? series.length - 1 - si : si) * (barW + barGap)}
                        y={y(v)}
                        width={barW}
                        height={h}
                        rx={Math.min(6, barW / 3)}
                        fill={colorVar(s.color)}
                        opacity={active === null || active === i ? 1 : 0.45}
                        className="transition-opacity duration-150"
                      />
                    )
                  })}
                </g>
              ))}
        </svg>
      )}
      {width === 0 && <div style={{ height }} />}

      {active !== null && width > 0 && (
        <div
          role="status"
          className="pointer-events-none absolute top-2 z-10 min-w-32 rounded-xl bg-gray-950/90 px-3 py-2 text-xs text-white shadow-lg backdrop-blur dark:bg-white/90 dark:text-gray-950"
          style={x(active) > width / 2 ? { right: width - x(active) + 12 } : { left: x(active) + 12 }}
          dir="rtl"
        >
          <p className="mb-1 font-semibold">{labels[active]}</p>
          {series.map((s) => (
            <p key={s.label} className="flex items-center gap-2">
              <span className="size-2 rounded-full" style={{ background: colorVar(s.color) }} />
              <span className="flex-1 opacity-80">{s.label}</span>
              <span className="font-semibold">{nf.format(s.data[active] ?? 0)}</span>
            </p>
          ))}
        </div>
      )}
      <div dir="rtl">
        <Legend series={series} />
      </div>
      <DataTable caption={caption} labels={labels} series={series} />
    </div>
  )
}

/** Parts of a whole, with the total in the middle and a legend with shares. */
export function DoughnutChart({
  caption,
  labels,
  values,
  colors,
  size = 200,
}: {
  caption: string
  labels: string[]
  values: number[]
  colors: Color[]
  size?: number
}) {
  const [active, setActive] = useState<number | null>(null)
  const total = values.reduce((a, b) => a + b, 0)
  const r = size / 2 - 10
  const stroke = 22
  const c = 2 * Math.PI * r
  let offset = 0
  const segments = values.map((v, i) => {
    const len = total > 0 ? (v / total) * c : 0
    const seg = { i, len, offset }
    offset += len
    return seg
  })

  return (
    <div className="flex flex-col items-center gap-6 sm:flex-row sm:justify-center">
      <div className="relative shrink-0" style={{ width: size, height: size }}>
        <svg width={size} height={size} role="img" aria-label={caption} className="-rotate-90">
          <circle cx={size / 2} cy={size / 2} r={r} fill="none" strokeWidth={stroke} className="stroke-gray-100 dark:stroke-white/5" />
          {segments.map((s) =>
            s.len > 0 ? (
              <circle
                key={s.i}
                cx={size / 2}
                cy={size / 2}
                r={r}
                fill="none"
                stroke={colorVar(colors[s.i] ?? 'gray')}
                strokeWidth={active === s.i ? stroke + 6 : stroke}
                // A small gap between segments.
                strokeDasharray={`${Math.max(0, s.len - 3)} ${c}`}
                strokeDashoffset={-s.offset}
                onPointerEnter={() => setActive(s.i)}
                onPointerLeave={() => setActive(null)}
                className="transition-[stroke-width] duration-150"
              />
            ) : null,
          )}
        </svg>
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center text-center">
          <span className="text-2xl font-bold text-gray-950 dark:text-white">{nf.format(active === null ? total : values[active])}</span>
          <span className="text-xs text-gray-500 dark:text-gray-400">{active === null ? t('chart.total') : labels[active]}</span>
        </div>
      </div>
      <ul className="w-full max-w-56 space-y-2 text-sm">
        {labels.map((l, i) => (
          <li
            key={l + i}
            onPointerEnter={() => setActive(i)}
            onPointerLeave={() => setActive(null)}
            className={'flex items-center gap-2 rounded-lg px-2 py-1 ' + (active === i ? 'bg-gray-100 dark:bg-white/5' : '')}
          >
            <span className="size-2.5 shrink-0 rounded-full" style={{ background: colorVar(colors[i] ?? 'gray') }} />
            <span className="flex-1 text-gray-700 dark:text-gray-300">{l}</span>
            <span className="font-semibold text-gray-950 tabular-nums dark:text-white">{nf.format(values[i] ?? 0)}</span>
            <span className="w-12 text-end text-xs text-gray-500 tabular-nums dark:text-gray-400">
              {total > 0 ? formatNumber((values[i] ?? 0) / total, { style: 'percent', maximumFractionDigits: 1 }) : '—'}
            </span>
          </li>
        ))}
      </ul>
      <DataTable caption={caption} labels={labels} series={[{ label: t('chart.value'), color: 'gray', data: values }]} />
    </div>
  )
}

/** A tiny area chart for a stat card. Decorative: the value is written next to it. */
export function Sparkline({ data, color, height = 40 }: { data: number[]; color: Color; height?: number }) {
  const [ref, width] = useWidth<HTMLDivElement>()
  const gradientId = useId()
  if (data.length < 2) return null
  const max = Math.max(...data, 1)
  const step = width / (data.length - 1)
  const pts = data.map((v, i) => `${i * step},${height - 2 - (v / max) * (height - 4)}`)
  return (
    <div ref={ref} dir="ltr" aria-hidden="true">
      {width > 0 && (
        <svg width={width} height={height} className="block">
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={colorVar(color)} stopOpacity="0.3" />
              <stop offset="100%" stopColor={colorVar(color)} stopOpacity="0" />
            </linearGradient>
          </defs>
          <path d={`M${pts.join('L')}L${width},${height}L0,${height}Z`} fill={`url(#${gradientId})`} />
          <path d={`M${pts.join('L')}`} fill="none" stroke={colorVar(color)} strokeWidth={1.75} strokeLinejoin="round" strokeLinecap="round" />
        </svg>
      )}
    </div>
  )
}
