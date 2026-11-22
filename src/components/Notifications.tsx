import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useId, useRef, useState, type ReactNode } from 'react'
import type { NotificationType, PanelNotification } from '../protocol'
import { relativeTime } from './cells'
import { BellIcon, CheckIcon, CloseIcon, InfoIcon, TrashIcon, WarningIcon, XIcon } from './icons'
import { usePanel } from './Panel'
import { iconButton } from './styles'
import { numberFormat, t } from '../i18n'

const nf = numberFormat()
const KEY = ['notifications']
const EXIT_MS = 320

const tone: Record<NotificationType, { icon: ReactNode; className: string }> = {
  success: { icon: <CheckIcon small />, className: 'bg-success-50 text-success-600 dark:bg-success-500/15 dark:text-success-400' },
  danger: { icon: <XIcon />, className: 'bg-danger-50 text-danger-600 dark:bg-danger-500/15 dark:text-danger-400' },
  warning: { icon: <WarningIcon />, className: 'bg-warning-50 text-warning-600 dark:bg-warning-500/15 dark:text-warning-400' },
  info: { icon: <InfoIcon />, className: 'bg-info-50 text-info-600 dark:bg-info-500/15 dark:text-info-400' },
}

/**
 * The bell: unread count, a drawer with the user's notifications, and a toast when new
 * ones arrive. New ones are fetched every few seconds while the tab is visible.
 */
/** The user's notifications, page by page, refreshed every `poll` seconds. */
function useNotifications(poll: number | undefined) {
  const { api } = usePanel()
  return useInfiniteQuery({
    queryKey: KEY,
    queryFn: ({ pageParam }) => api.notifications(pageParam),
    initialPageParam: 1,
    getNextPageParam: (last) => (last.meta.has_more ? last.meta.page + 1 : undefined),
    // Only while the tab is visible (react-query pauses intervals in the background),
    // and again as soon as the user comes back.
    refetchInterval: poll ? poll * 1000 : false,
    refetchOnWindowFocus: true,
    enabled: !!poll,
  })
}

type NotificationsQuery = ReturnType<typeof useNotifications>

export function NotificationBell() {
  const { meta, notify } = usePanel()
  const poll = meta.panel.notifications?.poll
  const [open, setOpen] = useState(false)
  const query = useNotifications(poll)

  // A toast for notifications that arrived since the last look (not for the first load).
  const seen = useRef<number | null>(null)
  const first = query.data?.pages[0]
  useEffect(() => {
    if (!first) return
    const newest = first.data[0]?.id ?? 0
    if (seen.current !== null) {
      const fresh = first.data.filter((n) => n.id > seen.current! && !n.read_at)
      for (const n of fresh.slice(0, 3).reverse()) notify(n.title, n.type)
    }
    seen.current = Math.max(seen.current ?? 0, newest)
  }, [first, notify])

  if (!poll) return null
  const unread = first?.meta.unread ?? 0
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={iconButton + ' relative'}
        aria-label={unread > 0 ? t('notifications.bellUnread', { count: unread }) : t('notifications.title')}
        aria-haspopup="dialog"
      >
        <BellIcon />
        {unread > 0 && (
          <span
            aria-hidden="true"
            className="absolute -top-0.5 -end-0.5 flex h-4.5 min-w-4.5 animate-pop items-center justify-center rounded-full bg-danger-500 px-1 text-[10px] leading-none font-bold text-white ring-2 ring-white dark:ring-gray-950"
          >
            {unread > 99 ? '۹۹+' : nf.format(unread)}
          </span>
        )}
      </button>
      {open && <NotificationsDrawer query={query} onClose={() => setOpen(false)} />}
    </>
  )
}

function NotificationsDrawer({ query, onClose }: { query: NotificationsQuery; onClose: () => void }) {
  const { api } = usePanel()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const dialog = useRef<HTMLDialogElement>(null)
  const closing = useRef(false)
  const titleId = useId()
  const [confirmClear, setConfirmClear] = useState(false)

  useEffect(() => {
    dialog.current?.showModal()
  }, [])
  useEffect(() => {
    if (!confirmClear) return
    const t = window.setTimeout(() => setConfirmClear(false), 4000)
    return () => window.clearTimeout(t)
  }, [confirmClear])

  function close() {
    if (closing.current || !dialog.current) return
    closing.current = true
    dialog.current.close()
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    window.setTimeout(onClose, reduced ? 0 : EXIT_MS)
  }

  const refresh = () => queryClient.invalidateQueries({ queryKey: KEY })
  const read = useMutation({ mutationFn: (id?: number) => api.readNotifications(id), onSuccess: refresh })
  const remove = useMutation({ mutationFn: (id?: number) => api.deleteNotifications(id), onSuccess: refresh })

  const items = query.data?.pages.flatMap((p) => p.data) ?? []
  const unread = query.data?.pages[0]?.meta.unread ?? 0

  function open(n: PanelNotification) {
    if (!n.read_at) read.mutate(n.id)
    if (n.url) {
      close()
      void navigate({ to: n.url })
    }
  }

  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault()
        close()
      }}
      onMouseDown={(e) => e.target === e.currentTarget && close()}
      className="mks-drawer bg-transparent p-0 text-gray-950 dark:text-white"
    >
      <div className="flex h-full flex-col bg-white shadow-xl ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10">
        <header className="flex items-center gap-3 border-b border-gray-100 px-5 py-4 dark:border-white/5">
          <h2 id={titleId} className="text-base font-semibold tracking-tight">
            {t('notifications.title')}
          </h2>
          {unread > 0 && (
            <span className="rounded-full bg-danger-50 px-2 py-0.5 text-xs font-semibold text-danger-700 dark:bg-danger-500/10 dark:text-danger-300">
              {t('notifications.new', { count: unread })}
            </span>
          )}
          <div className="flex-1" />
          {unread > 0 && (
            <button
              type="button"
              onClick={() => read.mutate(undefined)}
              disabled={read.isPending}
              className="text-sm font-medium text-primary-600 hover:text-primary-500 dark:text-primary-400"
            >
              {t('notifications.readAll')}
            </button>
          )}
          <button type="button" onClick={close} aria-label={t('common.close')} className={iconButton + ' -me-2'}>
            <CloseIcon />
          </button>
        </header>

        <div className="flex-1 overflow-y-auto">
          {query.isPending ? (
            <div className="space-y-3 p-5">
              {[0, 1, 2].map((i) => (
                <div key={i} className="h-16 animate-pulse rounded-xl bg-gray-100 dark:bg-white/5" />
              ))}
            </div>
          ) : items.length === 0 ? (
            <div className="flex h-full flex-col items-center justify-center px-8 py-16 text-center">
              <span className="flex size-14 items-center justify-center rounded-2xl bg-gray-100 text-gray-400 dark:bg-white/5 dark:text-gray-500 [&_svg]:size-7">
                <BellIcon />
              </span>
              <p className="mt-4 font-semibold">{t('notifications.empty')}</p>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400">{t('notifications.emptyHint')}</p>
            </div>
          ) : (
            <ul className="divide-y divide-gray-100 dark:divide-white/5">
              {items.map((n) => (
                <li key={n.id} className={'group relative flex gap-3 px-5 py-4 transition-colors ' + (n.read_at ? '' : 'bg-primary-50/40 dark:bg-primary-500/[0.04]')}>
                  <span className={'flex size-9 shrink-0 items-center justify-center rounded-xl [&_svg]:size-5 ' + tone[n.type].className}>{tone[n.type].icon}</span>
                  <button
                    type="button"
                    onClick={() => open(n)}
                    className="min-w-0 flex-1 text-start after:absolute after:inset-0 after:content-['']"
                  >
                    <span className="flex items-start gap-2">
                      <span className={'flex-1 text-sm ' + (n.read_at ? 'font-medium text-gray-700 dark:text-gray-300' : 'font-semibold')}>{n.title}</span>
                      {!n.read_at && <span className="mt-1.5 size-2 shrink-0 rounded-full bg-primary-500" aria-label={t('notifications.unread')} />}
                    </span>
                    {n.body && <span className="mt-0.5 block text-sm text-gray-500 dark:text-gray-400">{n.body}</span>}
                    {n.created_at && (
                      <time dateTime={n.created_at} className="mt-1 block text-xs text-gray-400 dark:text-gray-500">
                        {relativeTime(new Date(n.created_at))}
                      </time>
                    )}
                  </button>
                  <button
                    type="button"
                    onClick={() => remove.mutate(n.id)}
                    aria-label={t('notifications.delete', { title: n.title })}
                    className="relative z-10 self-start rounded-lg p-1.5 text-gray-400 opacity-0 group-hover:opacity-100 hover:bg-gray-100 hover:text-danger-600 focus-visible:opacity-100 dark:hover:bg-white/5"
                  >
                    <TrashIcon />
                  </button>
                </li>
              ))}
            </ul>
          )}
          {query.hasNextPage && (
            <div className="p-4 text-center">
              <button
                type="button"
                onClick={() => void query.fetchNextPage()}
                disabled={query.isFetchingNextPage}
                className="text-sm font-medium text-primary-600 hover:text-primary-500 dark:text-primary-400"
              >
                {query.isFetchingNextPage ? t('common.loading') : t('notifications.older')}
              </button>
            </div>
          )}
        </div>

        {items.length > 0 && (
          <footer className="border-t border-gray-100 px-5 py-3 dark:border-white/5">
            <button
              type="button"
              onClick={() => (confirmClear ? (remove.mutate(undefined), setConfirmClear(false)) : setConfirmClear(true))}
              className={'text-sm font-medium ' + (confirmClear ? 'text-danger-600 dark:text-danger-400' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200')}
            >
              {confirmClear ? t('notifications.confirmClear') : t('notifications.clearAll')}
            </button>
          </footer>
        )}
      </div>
    </dialog>
  )
}
