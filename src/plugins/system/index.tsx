import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { ApiError, Card, defineResource, definePage, definePlugin, PageHeader, tr, usePageData, usePageSubmit, usePanel, type PanelPlugin } from '../../index'

interface Check {
  key: string
  label: string
  status: 'ok' | 'warning' | 'failed'
  detail: string
}

export interface ConsoleLog {
  id: number
  user_id: string | null
  command: string
  output: string | null
  exit_code: number
  duration_ms: number
  created_at: string | null
}

const PulseIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M3 12h4l2-6 4 12 2-6h6" />
  </svg>
)
const TerminalIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <path d="M4 5h16v14H4zM8 9l3 3-3 3M13 15h3" />
  </svg>
)

const tone = { ok: 'bg-success-500', warning: 'bg-warning-500', failed: 'bg-danger-500' }
const statusLabel = { ok: tr({ fa: 'سالم', en: 'OK' }), warning: tr({ fa: 'هشدار', en: 'Warning' }), failed: tr({ fa: 'خطا', en: 'Failed' }) }

function HealthPage() {
  const data = usePageData<{ checks: Check[] }>()
  const checks = data.data?.checks ?? []
  const problems = checks.filter((c) => c.status !== 'ok').length
  return (
    <div className="space-y-6">
      <PageHeader title={tr({ fa: 'سلامت سیستم', en: 'System health' })} description={problems ? tr({ fa: `${problems} مورد نیاز به توجه دارد.`, en: `${problems} need attention.` }) : tr({ fa: 'همه‌چیز سالم است.', en: 'All good.' })} />
      <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        {checks.map((c) => (
          <Card key={c.key}>
            <div className="flex items-start gap-3">
              <span className={`mt-1.5 size-2.5 shrink-0 rounded-full ${tone[c.status]}`} aria-hidden="true" />
              <div className="min-w-0">
                <p className="font-semibold">{c.label} <span className="text-xs font-normal text-gray-500">— {statusLabel[c.status]}</span></p>
                <p className="mt-1 text-sm text-gray-600 dark:text-gray-400">{c.detail}</p>
              </div>
            </div>
          </Card>
        ))}
      </div>
      <button type="button" onClick={() => data.refetch()} className="rounded-lg px-3 py-1.5 text-sm font-semibold ring-1 ring-gray-950/10 hover:bg-gray-50 dark:ring-white/10 dark:hover:bg-white/5">
        {tr({ fa: 'بررسی دوباره', en: 'Check again' })}
      </button>
    </div>
  )
}

interface Run {
  id: string
  command: string
  status: 'running' | 'completed' | 'failed' | 'stopped'
  exit_code: number | null
  started_at: string
  finished_at: string | null
  output?: string
  offset?: number
  cut?: boolean
}

const runStatus: Record<Run['status'], { label: string; cls: string }> = {
  running: { label: tr({ fa: 'در حال اجرا', en: 'Running' }), cls: 'bg-primary-50 text-primary-700 dark:bg-primary-500/10 dark:text-primary-300' },
  completed: { label: tr({ fa: 'پایان', en: 'Done' }), cls: 'bg-success-50 text-success-700 dark:bg-success-500/10 dark:text-success-300' },
  failed: { label: tr({ fa: 'ناموفق', en: 'Failed' }), cls: 'bg-danger-50 text-danger-700 dark:bg-danger-500/10 dark:text-danger-300' },
  stopped: { label: tr({ fa: 'متوقف', en: 'Stopped' }), cls: 'bg-gray-100 text-gray-700 dark:bg-white/5 dark:text-gray-300' },
}

const L = (fa: string, en: string) => tr({ fa, en })

interface QueueInfo {
  connection: string
  driver: string
  queue: string
  pending: number
  failed: number
  oldest: string | null
}

interface WorkerInfo {
  id: string
  command: string
  started_at: string
}

interface SchedulerInfo {
  running: boolean
  id: string
  last_run: string | null
  cron: string
  can_start: boolean
}

interface ConsoleData {
  commands: string[]
  queue: QueueInfo
  workers: { items: WorkerInfo[]; can_start: boolean }
  scheduler: SchedulerInfo
}

const Icon = ({ d, className = 'size-4' }: { d: string; className?: string }) => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.8} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className={className}>
    <path d={d} />
  </svg>
)
const ListIcon = () => <Icon d="M4 6h16M4 12h16M4 18h10" />
const QueueIcon = () => <Icon d="M4 7h16M4 12h16M4 17h10M18 15v6M15 18h6" />
const WorkerIcon = () => <Icon d="M9 3v4M15 3v4M6 7h12v5a6 6 0 0 1-12 0zM12 18v3" />
const CalendarIcon = () => <Icon d="M4 6h16v14H4zM8 3v5M16 3v5M4 11h16" />
const ClockIcon = () => <Icon d="M12 4a8 8 0 1 0 0 16 8 8 0 0 0 0-16zM12 8v4l3 2" />
const PlayIcon = ({ className }: { className?: string }) => <Icon d="M8 5v14l11-7z" className={className ?? 'size-4'} />
const StopIcon = () => <Icon d="M7 7h10v10H7z" />
const CopyIcon = () => <Icon d="M9 9h10v10H9zM5 15V5h10" />
const CheckIcon = () => <Icon d="m5 12 4 4L19 6" />
const SearchIcon = () => <Icon d="M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM20 20l-4-4" />
const CloseIcon = () => <Icon d="M6 6l12 12M18 6 6 18" />
const HistoryIcon = () => <Icon d="M3 12a9 9 0 1 0 3-6.7M3 4v5h5M12 8v4l3 2" />
const ConsoleIcon = ({ className }: { className?: string }) => <Icon d="M4 5h16v14H4zM8 9l3 3-3 3M13 15h3" className={className ?? 'size-5'} />
const BanIcon = () => <Icon d="M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18zM5.6 5.6l12.8 12.8" />

/** "42 seconds ago", "2 weeks ago" (MKSine's history line). */
function ago(iso: string | null, withSeconds = false): string {
  if (!iso) return '—'
  const seconds = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000))
  if (seconds < 60) return withSeconds ? L(`${seconds} ثانیه پیش`, `${seconds}s ago`) : L('چند لحظه پیش', 'moments ago')
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return L(`${minutes} دقیقه پیش`, `${minutes}m ago`)
  const hours = Math.round(minutes / 60)
  if (hours < 24) return L(`${hours} ساعت پیش`, `${hours}h ago`)
  const days = Math.round(hours / 24)
  if (days < 30) return L(`${days} روز پیش`, `${days}d ago`)
  return new Date(iso).toLocaleDateString()
}

function errorText(e: unknown): string {
  if (e instanceof ApiError) return Object.values(e.errors)[0]?.[0] ?? e.message
  return e instanceof Error ? e.message : String(e)
}

const card = 'rounded-xl bg-white p-4 shadow-sm ring-1 ring-gray-200 dark:bg-gray-900/70 dark:ring-white/10'
const outlineButton = 'inline-flex items-center justify-center gap-1.5 rounded-md bg-white px-3 py-1.5 text-sm font-medium text-gray-700 ring-1 ring-gray-200 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-white/5 dark:text-gray-200 dark:ring-white/10 dark:hover:bg-white/10'
const blueButton = 'inline-flex items-center justify-center gap-1.5 rounded-md bg-blue-600 px-3 py-1.5 text-sm font-medium text-white shadow-sm hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-50'

function MiniStat({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="rounded-lg bg-gray-50 px-2 py-2 text-center dark:bg-white/5">
      <span className="block text-[11px] text-gray-500 dark:text-gray-400">{label}</span>
      <span className="mt-0.5 block text-base font-bold tabular-nums text-gray-900 dark:text-white">{value}</span>
    </div>
  )
}

function QueuePanel({ q }: { q?: QueueInfo }) {
  return (
    <section className={card}>
      <div className="flex items-center gap-2">
        <span className="flex size-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-white/5 dark:text-gray-300">
          <QueueIcon />
        </span>
        <h2 className="font-bold text-gray-900 dark:text-white">{L('صف', 'Queue')}</h2>
      </div>
      <p className="mt-1 text-sm text-gray-500 dark:text-gray-400" dir="ltr">
        {q ? `${q.connection} - ${q.queue}` : '—'}
      </p>
      <div className="mt-3 grid grid-cols-3 gap-2">
        <MiniStat label={L('در انتظار', 'Pending')} value={q?.pending ?? 0} />
        <MiniStat label={L('ناموفق', 'Failed')} value={q?.failed ?? 0} />
        <MiniStat label={L('قدیمی‌ترین', 'Oldest')} value={q?.oldest ? ago(q.oldest) : '—'} />
      </div>
    </section>
  )
}

function WorkersPanel({ workers, onStart, onStop }: { workers?: ConsoleData['workers']; onStart: () => void; onStop: (id: string) => void }) {
  const items = workers?.items ?? []
  return (
    <section className={card + ' flex flex-col'}>
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="flex size-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-white/5 dark:text-gray-300">
            <WorkerIcon />
          </span>
          <h2 className="font-bold text-gray-900 dark:text-white">{L('Workerهای صف', 'Queue workers')}</h2>
        </div>
        {items.length > 0 && <span className="rounded-full bg-emerald-50 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-400">{items.length}</span>}
      </div>
      <div className="flex-1">
        {items.length === 0 ? (
          <p className="py-10 text-center text-sm text-gray-500 dark:text-gray-400">{L('هیچ Workerی فعال نیست (صف خالی است)', 'No worker is running (the queue is empty)')}</p>
        ) : (
          <ul className="mt-3 divide-y divide-gray-100 text-sm dark:divide-white/5">
            {items.map((w) => (
              <li key={w.id} className="flex items-center gap-2 py-2">
                <span className="size-2 shrink-0 rounded-full bg-emerald-500" aria-hidden="true" />
                <code dir="ltr" className="min-w-0 flex-1 truncate font-mono text-xs">{w.command}</code>
                <span className="shrink-0 text-xs text-gray-500">{ago(w.started_at)}</span>
                <button type="button" onClick={() => onStop(w.id)} className="rounded-md p-1 text-gray-400 hover:text-danger-600" title={L('توقف', 'Stop')} aria-label={L('توقف', 'Stop')}>
                  <StopIcon />
                </button>
              </li>
            ))}
          </ul>
        )}
      </div>
      <div className="mt-4 flex justify-end">
        <button
          type="button"
          className={blueButton}
          disabled={!workers?.can_start}
          title={workers?.can_start ? L('اجرای queue:work در پس‌زمینه', 'Run queue:work in the background') : L('فرمان queue:work در این پروژه در دسترس نیست', 'queue:work is not available in this project')}
          onClick={onStart}
        >
          {L('شروع worker', 'Start worker')} <PlayIcon />
        </button>
      </div>
    </section>
  )
}

function SchedulerPanel({ s, copied, onCopy, onNow, onTests, onStart, onStop }: { s?: SchedulerInfo; copied: boolean; onCopy: () => void; onNow: () => void; onTests: () => void; onStart: () => void; onStop: () => void }) {
  const running = !!s?.running
  return (
    <section className={card}>
      <div className="flex items-start justify-between gap-2">
        <div className="flex items-center gap-2">
          <span className="flex size-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 dark:bg-white/5 dark:text-gray-300">
            <CalendarIcon />
          </span>
          <h2 className="font-bold text-gray-900 dark:text-white">{L('زمان‌بند (Scheduler)', 'Scheduler')}</h2>
        </div>
        <span className={'inline-flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs font-medium ring-1 ring-inset ' + (running ? 'bg-emerald-50 text-emerald-700 ring-emerald-100 dark:bg-emerald-500/10 dark:text-emerald-400 dark:ring-emerald-500/20' : 'bg-danger-50 text-danger-700 ring-danger-100 dark:bg-danger-500/10 dark:text-danger-400 dark:ring-danger-500/20')}>
          <span className={'size-1.5 rounded-full ' + (running ? 'bg-emerald-500' : 'bg-danger-500')} aria-hidden="true" />
          {running ? L('فعال', 'Active') : L('غیرفعال', 'Inactive')}
        </span>
      </div>
      <p className="mt-1 text-xs text-gray-500 dark:text-gray-400">{L(`آخرین اجرا ${s?.last_run ? ago(s.last_run) : '—'}`, `Last run ${s?.last_run ? ago(s.last_run) : '—'}`)}</p>
      <div className="mt-3 flex items-center gap-2 rounded-lg bg-gray-50 px-2 py-1.5 dark:bg-white/5" dir="ltr">
        <code className="min-w-0 flex-1 truncate font-mono text-[11px] text-gray-600 dark:text-gray-300">{s?.cron ?? ''}</code>
        <button type="button" onClick={onCopy} className="shrink-0 rounded p-1 text-gray-400 hover:text-gray-700 dark:hover:text-white" title={L('کپی', 'Copy')} aria-label={L('کپی', 'Copy')}>
          {copied ? <CheckIcon /> : <CopyIcon />}
        </button>
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <button type="button" className={outlineButton} onClick={onTests}>
          <ListIcon /> {L('تست‌ها', 'Tests')}
        </button>
        <button type="button" className={outlineButton} onClick={onNow}>
          <PlayIcon /> {L('اجرای الان', 'Run now')}
        </button>
      </div>
      <button
        type="button"
        className={outlineButton + ' mt-2 w-full'}
        disabled={!s?.can_start && !running}
        title={s?.can_start ? L('اجرای schedule:work در پس‌زمینه', 'Run schedule:work in the background') : L('فرمان schedule:work در این پروژه در دسترس نیست', 'schedule:work is not available in this project')}
        onClick={running ? onStop : onStart}
      >
        {running ? L('توقف schedule:work', 'Stop schedule:work') : L('شروع schedule:work', 'Start schedule:work')} {running ? <StopIcon /> : <PlayIcon />}
      </button>
    </section>
  )
}

type Line = { kind: 'command' | 'output' | 'error' | 'info'; text: string }

function ConsolePage() {
  const { api, notify } = usePanel()
  const data = usePageData<ConsoleData>()
  const submit = usePageSubmit()
  const [input, setInput] = useState('')
  const [entries, setEntries] = useState<Line[]>([])
  const [status, setStatus] = useState<'idle' | 'running' | 'error'>('idle')
  const [background, setBackground] = useState(false)
  const [search, setSearch] = useState('')
  const [showHistory, setShowHistory] = useState(true)
  const [recentRuns, setRecentRuns] = useState<string[]>([])
  const [copied, setCopied] = useState(false)
  const activeRun = useRef<string | null>(null)
  const stopFlag = useRef(false)
  const alive = useRef(true)
  const output = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const historyIndex = useRef(-1)

  useEffect(() => {
    return () => {
      alive.current = false
      stopFlag.current = true
    }
  }, [])

  const history = useQuery({
    queryKey: ['system', 'console-history'],
    queryFn: () => api.list<ConsoleLog>('console_logs', { perPage: 100, sort: ['-id'] }),
  })
  const logs = history.data?.data ?? []
  const total = history.data?.meta?.total ?? logs.length
  const filter = search.trim().toLowerCase()
  const shownLogs = filter ? logs.filter((l) => l.command.toLowerCase().includes(filter)) : logs
  const recent = Array.from(new Set(logs.map((l) => l.command))).slice(0, 8)
  const suggestions = recent.length > 0 ? recent : (data.data?.commands ?? []).filter((c) => !c.endsWith('*')).slice(0, 8)

  useEffect(() => {
    const el = output.current
    if (el) el.scrollTop = el.scrollHeight
  }, [entries, status])

  useEffect(() => {
    if (!data.data?.workers.items.length) return
    const timer = setInterval(() => void data.refetch(), 3000)
    return () => clearInterval(timer)
  }, [data.data?.workers.items.length])

  function append(...lines: Line[]) {
    setEntries((current) => [...current, ...lines])
  }

  function fail(e: unknown) {
    append({ kind: 'error', text: errorText(e) })
    setStatus('error')
  }

  async function execute(raw: string, runInBackground = background) {
    const command = raw.trim()
    if (!command || status === 'running') return
    setInput('')
    setRecentRuns((list) => [command, ...list.filter((c) => c !== command)].slice(0, 50))
    historyIndex.current = -1
    append({ kind: 'command', text: command })
    setStatus('running')
    if (runInBackground) {
      stopFlag.current = false
      try {
        const started = await api.plugin<{ id: string }>('system', '/processes', {
          method: 'POST',
          body: JSON.stringify({ command }),
          headers: { 'Content-Type': 'application/json' },
        })
        activeRun.current = started.id
        let offset = 0
        while (alive.current && !stopFlag.current) {
          const run = await api.plugin<Run>('system', `/processes/${started.id}?offset=${offset}`)
          offset = run.offset ?? offset
          if (run.output) append({ kind: 'output', text: run.output })
          if (run.status !== 'running') {
            setStatus(run.status === 'failed' ? 'error' : 'idle')
            append({ kind: run.status === 'failed' ? 'error' : 'info', text: runStatus[run.status].label + (run.exit_code != null ? ` — ${run.exit_code}` : '') })
            break
          }
          await new Promise((resolve) => setTimeout(resolve, 1000))
        }
      } catch (e) {
        fail(e)
      } finally {
        activeRun.current = null
        void history.refetch()
        void data.refetch()
      }
      return
    }
    try {
      const res = await submit({ command })
      const result = (res.data as { run?: ConsoleLog } | undefined)?.run
      if (result?.output) append({ kind: result.exit_code === 0 ? 'output' : 'error', text: result.output })
      setStatus(result && result.exit_code !== 0 ? 'error' : 'idle')
      if (result) append({ kind: 'info', text: L(`پایان در ${result.duration_ms} میلی‌ثانیه`, `Done in ${result.duration_ms} ms`) })
    } catch (e) {
      fail(e)
    } finally {
      void history.refetch()
      void data.refetch()
    }
  }

  async function stop() {
    const id = activeRun.current
    if (!id) return
    stopFlag.current = true
    try {
      await api.plugin('system', `/processes/${id}/stop`, { method: 'POST' })
    } catch {
      // already finished
    }
    setStatus('idle')
    append({ kind: 'info', text: L('دستور متوقف شد.', 'Stopped.') })
  }

  function clear() {
    setEntries([])
    setStatus('idle')
    inputRef.current?.focus()
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Tab') {
      e.preventDefault()
      const all = (data.data?.commands ?? []).filter((c) => !c.endsWith('*'))
      const match = all.find((c) => c.startsWith(input.trim()) && c !== input.trim())
      if (match) setInput(match)
      return
    }
    if (e.key === 'ArrowUp' || e.key === 'ArrowDown') {
      if (recentRuns.length === 0) return
      e.preventDefault()
      if (e.key === 'ArrowUp') historyIndex.current = Math.min(historyIndex.current + 1, recentRuns.length - 1)
      else historyIndex.current = Math.max(historyIndex.current - 1, -1)
      setInput(recentRuns[historyIndex.current] ?? '')
      return
    }
    if (e.ctrlKey && e.key.toLowerCase() === 'c' && status === 'running' && activeRun.current) {
      e.preventDefault()
      void stop()
      return
    }
    if (e.ctrlKey && e.key.toLowerCase() === 'l') {
      e.preventDefault()
      clear()
    }
  }

  async function startProcess(command: string) {
    try {
      await api.plugin<{ id: string }>('system', '/processes', {
        method: 'POST',
        body: JSON.stringify({ command }),
        headers: { 'Content-Type': 'application/json' },
      })
      notify(L(`«${command}» شروع شد.`, `"${command}" started.`), 'success')
      await data.refetch()
    } catch (e) {
      notify(errorText(e), 'danger')
    }
  }

  async function stopProcess(id: string) {
    try {
      await api.plugin('system', `/processes/${id}/stop`, { method: 'POST' })
      await data.refetch()
    } catch (e) {
      notify(errorText(e), 'danger')
    }
  }

  async function copyCron() {
    const cron = data.data?.scheduler.cron ?? ''
    if (!cron) return
    try {
      await navigator.clipboard.writeText(cron)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      // clipboard blocked
    }
  }

  function showLog(log: ConsoleLog) {
    setEntries([
      { kind: 'command', text: log.command },
      { kind: log.exit_code === 0 ? 'output' : 'error', text: log.output ?? '' },
    ])
    setStatus(log.exit_code === 0 ? 'idle' : 'error')
    setInput(log.command)
    inputRef.current?.focus()
  }

  const running = status === 'running'
  const dot = running ? 'bg-amber-400 animate-pulse' : status === 'error' ? 'bg-danger-500' : 'bg-emerald-500'
  const statusLabel = running ? L('در حال اجرا', 'Running') : status === 'error' ? L('خطا', 'Error') : L('آماده', 'Ready')

  const hints = [
    { key: 'Enter', label: L('اجرای دستور', 'Run the command') },
    { key: 'Tab', label: L('تکمیل خودکار', 'Autocomplete') },
    { key: '↑↓', label: L('دستورات قبلی', 'Previous commands') },
    { key: 'Ctrl+C', label: L('توقف دستور در حال اجرا', 'Stop the running command') },
    { key: 'Ctrl+L', label: L('پاک کردن صفحه', 'Clear the screen') },
  ]

  return (
    <div className="space-y-4">
      <div className="grid gap-4 xl:grid-cols-3">
        <QueuePanel q={data.data?.queue} />
        <WorkersPanel workers={data.data?.workers} onStart={() => void startProcess('queue:work')} onStop={(id) => void stopProcess(id)} />
        <SchedulerPanel
          s={data.data?.scheduler}
          copied={copied}
          onCopy={() => void copyCron()}
          onNow={() => void execute('schedule:run')}
          onTests={() => void execute('schedule:list')}
          onStart={() => void startProcess('schedule:work')}
          onStop={() => void stopProcess(data.data?.scheduler.id ?? '')}
        />
      </div>

      <div className={showHistory ? 'grid gap-4 xl:grid-cols-[minmax(0,1fr)_22rem]' : 'grid gap-4'}>
        <section className="flex h-[600px] flex-col overflow-hidden rounded-xl bg-[#0b1017] text-gray-100 shadow-lg ring-1 ring-black/10">
          <header className="flex flex-wrap items-center justify-between gap-2 border-b border-white/10 px-4 py-2.5">
            <div className="flex items-center gap-1">
              {running && activeRun.current && (
                <button type="button" onClick={() => void stop()} className="rounded-md p-1.5 text-gray-400 hover:bg-white/10 hover:text-danger-400" title={L('توقف', 'Stop')} aria-label={L('توقف', 'Stop')}>
                  <StopIcon />
                </button>
              )}
              <button
                type="button"
                onClick={() => setShowHistory((shown) => !shown)}
                className={'rounded-md p-1.5 hover:bg-white/10 hover:text-white ' + (showHistory ? 'text-gray-200' : 'text-gray-500')}
                title={L('تاریخچه', 'History')}
                aria-label={L('تاریخچه', 'History')}
                aria-pressed={showHistory}
              >
                <HistoryIcon />
              </button>
              <button
                type="button"
                onClick={() => setBackground((on) => !on)}
                className={'rounded-md p-1.5 hover:bg-white/10 hover:text-white ' + (background ? 'text-primary-300' : 'text-gray-500')}
                title={L('اجرا در پس‌زمینه', 'Run in the background')}
                aria-label={L('اجرا در پس‌زمینه', 'Run in the background')}
                aria-pressed={background}
              >
                <ClockIcon />
              </button>
              <button type="button" onClick={clear} className="rounded-md p-1.5 text-gray-500 hover:bg-white/10 hover:text-white" title={L('پاک کردن', 'Clear')} aria-label={L('پاک کردن', 'Clear')}>
                <BanIcon />
              </button>
            </div>
            <div className="flex items-center gap-2 text-xs text-gray-400">
              <span className={'size-2 rounded-full ' + dot} aria-hidden="true" />
              <span aria-live="polite">{statusLabel}</span>
              {background && <span className="rounded bg-white/10 px-1.5 py-0.5 text-[10px] text-gray-300">{L('پس‌زمینه', 'Background')}</span>}
            </div>
          </header>

          <div ref={output} className="flex-1 overflow-y-auto p-5">
            {entries.length === 0 ? (
              <div className="mx-auto flex max-w-xl flex-col items-center pt-8 text-center">
                <span className="flex size-14 items-center justify-center rounded-xl bg-white/5 text-primary-300 ring-1 ring-white/10">
                  <ConsoleIcon className="size-6" />
                </span>
                <h2 className="mt-4 text-lg font-bold text-white">{L('MKS کنسول', 'MKS console')}</h2>
                <p className="mt-1 text-sm text-gray-400">{L('یک دستور artisan بنویسید یا از میان‌برها انتخاب کنید.', 'Type an artisan command or pick a shortcut.')}</p>
                <div className="mt-6 w-full max-w-sm space-y-2">
                  {hints.map((hint) => (
                    <div key={hint.key} className="flex items-center justify-between gap-6 text-xs text-gray-400">
                      <span>{hint.label}</span>
                      <kbd className="rounded-md bg-white/10 px-1.5 py-0.5 font-mono text-[11px] text-gray-300 ring-1 ring-white/15">{hint.key}</kbd>
                    </div>
                  ))}
                </div>
                {suggestions.length > 0 && (
                  <div className="mt-7 flex flex-wrap justify-center gap-2">
                    {suggestions.map((command) => {
                      const long = /^(queue:work|schedule:work)/.test(command)
                      return (
                        <button
                          key={command}
                          type="button"
                          onClick={() => void execute(command)}
                          className="flex items-center gap-2 rounded-md bg-white/5 px-2.5 py-1 font-mono text-[11px] text-gray-300 ring-1 ring-white/10 hover:bg-white/10"
                        >
                          {long && <span className="size-1.5 rounded-full bg-amber-400" aria-hidden="true" />}
                          <span dir="ltr">{command}</span>
                        </button>
                      )
                    })}
                  </div>
                )}
              </div>
            ) : (
              <div dir="ltr" className="whitespace-pre-wrap font-mono text-xs leading-relaxed" aria-live="polite">
                {entries.map((line, i) => (
                  <div key={i} className={line.kind === 'command' ? 'text-primary-300' : line.kind === 'error' ? 'text-danger-400' : line.kind === 'info' ? 'text-gray-500' : 'text-gray-200'}>
                    {line.kind === 'command' ? '❯ ' : ''}
                    {line.text}
                    {'\n'}
                  </div>
                ))}
              </div>
            )}
          </div>

          <form
            className="border-t border-white/10 p-3"
            onSubmit={(e) => {
              e.preventDefault()
              void execute(input)
            }}
          >
            <div dir="ltr" className="flex items-center gap-2 rounded-lg bg-white/[0.04] px-3 py-1.5 ring-1 ring-white/10 focus-within:ring-2 focus-within:ring-primary-500">
              <span className="shrink-0 font-mono text-sm text-primary-300" aria-hidden="true">
                ❯
              </span>
              <input
                ref={inputRef}
                dir="ltr"
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={onKeyDown}
                placeholder="migrate:status"
                aria-label={L('فرمان', 'Command')}
                autoComplete="off"
                spellCheck={false}
                className="min-w-0 flex-1 bg-transparent py-1 font-mono text-sm text-gray-100 placeholder:text-gray-500 focus:outline-none"
              />
              <button type="submit" disabled={running || !input.trim()} className="inline-flex shrink-0 items-center gap-1.5 rounded-md bg-blue-600 px-3 py-1 text-xs font-medium text-white hover:bg-blue-500 disabled:cursor-not-allowed disabled:opacity-50">
                <PlayIcon className="size-3.5" /> {running ? L('در حال اجرا…', 'Running…') : L('اجرا', 'Run')}
              </button>
            </div>
          </form>
        </section>

        {showHistory && (
          <aside className="flex flex-col rounded-xl bg-white shadow-sm ring-1 ring-gray-200 dark:bg-gray-900/70 dark:ring-white/10">
            <header className="flex items-center justify-between gap-2 border-b border-gray-100 px-4 py-3 dark:border-white/5">
              <div className="flex items-center gap-2">
                <h2 className="font-bold text-gray-900 dark:text-white">{L('تاریخچه', 'History')}</h2>
                <span className="rounded-md bg-gray-100 px-1.5 text-xs font-medium text-gray-600 dark:bg-white/10 dark:text-gray-300">{total}</span>
              </div>
              <button type="button" onClick={() => setShowHistory(false)} className="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-white/10 dark:hover:text-white" title={L('بستن', 'Close')} aria-label={L('بستن', 'Close')}>
                <CloseIcon />
              </button>
            </header>
            <div className="p-3">
              <label className="relative block">
                <span className="pointer-events-none absolute inset-y-0 start-2.5 flex items-center text-gray-400" aria-hidden="true">
                  <SearchIcon />
                </span>
                <input
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                  placeholder={L('فیلتر دستورات…', 'Filter commands…')}
                  aria-label={L('فیلتر دستورات', 'Filter commands')}
                  className="block w-full rounded-lg border-0 bg-gray-50 py-1.5 ps-8 pe-3 text-sm placeholder:text-gray-400 focus:ring-2 focus:ring-primary-500 focus:outline-none dark:bg-white/5"
                />
              </label>
            </div>
            <ul className="max-h-[520px] flex-1 divide-y divide-gray-100 overflow-y-auto dark:divide-white/5">
              {shownLogs.length === 0 ? (
                <li className="px-4 py-8 text-center text-sm text-gray-500 dark:text-gray-400">{L('تاریخچه‌ای نیست.', 'Nothing yet.')}</li>
              ) : (
                shownLogs.map((log) => (
                  <li key={log.id}>
                    <button type="button" onClick={() => showLog(log)} className="flex w-full flex-col gap-1 px-4 py-3 text-start hover:bg-gray-50 dark:hover:bg-white/[0.04]">
                      <span className="flex items-center justify-between gap-2">
                        <span className={'size-2 shrink-0 rounded-full ' + (log.exit_code === 0 ? 'bg-emerald-500' : 'bg-danger-500')} aria-hidden="true" />
                        <code dir="ltr" className="min-w-0 flex-1 truncate font-mono text-xs font-semibold text-gray-800 dark:text-gray-100">
                          {log.command}
                        </code>
                      </span>
                      <span className="flex items-center justify-between gap-2 text-[11px] text-gray-500 dark:text-gray-400">
                        <span>{log.user_id ? L(`کاربر ${log.user_id}`, `User ${log.user_id}`) : L('سیستم', 'System')}</span>
                        <span>{ago(log.created_at)}</span>
                      </span>
                    </button>
                  </li>
                ))
              )}
            </ul>
          </aside>
        )}
      </div>
    </div>
  )
}

const consoleLogs = defineResource<ConsoleLog>({
  name: 'console_logs',
  label: tr({ fa: 'اجرای کنسول', en: 'Console run' }),
  pluralLabel: tr({ fa: 'تاریخچهٔ کنسول', en: 'Console history' }),
  icon: <TerminalIcon />,
  defaultSort: '-id',
  recordTitle: 'command',
  table: (t) => [
    t.text('command').label(tr({ fa: 'فرمان', en: 'Command' })).searchable().copyable(),
    t.badge('exit_code').label(tr({ fa: 'کد پایان', en: 'Exit code' })).colors({ 0: 'success' }),
    t.text('duration_ms').label(tr({ fa: 'زمان (ms)', en: 'Time (ms)' })).numeric(),
    t.date('created_at').label(tr({ fa: 'زمان', en: 'When' })).since().sortable(),
  ],
  infolist: (i) => [
    i.text('command').label(tr({ fa: 'فرمان', en: 'Command' })),
    i.text('exit_code').label(tr({ fa: 'کد پایان', en: 'Exit code' })),
    i.text('output').label(tr({ fa: 'خروجی', en: 'Output' })).columnSpan('full'),
  ],
})

/** System health and the admin console. Install the Go plugin (system.Plugin) too. */
/**
 * System health and the console. MKSine puts the console under "Tools" and health under
 * "System": consoleGroup and healthGroup (both default to group). Console history opens
 * from the console page.
 */
export function system(options: { group?: string; consoleGroup?: string; healthGroup?: string } = {}): PanelPlugin {
  return definePlugin({
    name: 'system',
    resources: [{ ...consoleLogs, group: options.consoleGroup ?? options.group, navigation: false }],
    pages: [
      definePage({ name: 'system_health', label: tr({ fa: 'سلامت سیستم', en: 'System health' }), icon: <PulseIcon />, group: options.healthGroup ?? options.group, navigationSort: 9998, component: HealthPage }),
      definePage({ name: 'console', label: tr({ fa: 'کنسول', en: 'Console' }), icon: <TerminalIcon />, group: options.consoleGroup ?? options.group, navigationSort: 30, component: ConsolePage }),
    ],
  })
}
