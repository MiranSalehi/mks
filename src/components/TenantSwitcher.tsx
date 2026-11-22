import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useEffect, useId, useRef, useState } from 'react'
import { CheckIcon, ChevronIcon } from './icons'
import { usePanel } from './Panel'
import { t } from '../i18n'

/**
 * Which tenant this tab works in (multi-tenancy). Switching forgets every cached record,
 * so nothing of the previous tenant stays on screen, and opens the home page.
 */
export function TenantSwitcher() {
  const { api, meta, tenantLabel } = usePanel()
  const queryClient = useQueryClient()
  const navigate = useNavigate()
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)
  const listId = useId()
  const tenancy = meta.tenancy

  useEffect(() => {
    if (!open) return
    const onDown = (e: MouseEvent) => root.current && !root.current.contains(e.target as Node) && setOpen(false)
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onDown)
    window.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDown)
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  if (!tenancy) return null
  const current = tenancy.tenants.find((t) => String(t.id) === String(tenancy.current))
  if (!current) {
    return <p className="mx-3 rounded-xl bg-warning-50 px-3 py-2 text-xs text-warning-700 dark:bg-warning-500/10 dark:text-warning-300">{t('tenant.none', { label: tenantLabel })}</p>
  }
  const single = tenancy.tenants.length < 2

  async function choose(id: string) {
    setOpen(false)
    if (id === String(tenancy!.current)) return
    api.setTenant(id)
    await navigate({ to: '/' })
    // Everything was loaded for the other tenant: start over (meta first).
    await queryClient.resetQueries()
  }

  return (
    <div ref={root} className="relative px-3 pb-2">
      <button
        type="button"
        disabled={single}
        onClick={() => setOpen(!open)}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-controls={listId}
        aria-label={`${tenantLabel}: ${current.name}`}
        className="flex w-full items-center gap-3 rounded-xl bg-gray-900/[0.03] px-3 py-2 text-start ring-1 ring-gray-950/5 hover:bg-gray-900/[0.06] disabled:cursor-default disabled:hover:bg-gray-900/[0.03] dark:bg-white/[0.03] dark:ring-white/10 dark:hover:bg-white/[0.06]"
      >
        <span aria-hidden="true" className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-gray-900 text-xs font-bold text-white dark:bg-white dark:text-gray-900">
          {current.name.trim().charAt(0)}
        </span>
        <span className="min-w-0 flex-1">
          <span className="block text-[11px] text-gray-500 dark:text-gray-400">{tenantLabel}</span>
          <span className="block truncate text-sm font-semibold">{current.name}</span>
        </span>
        {!single && (
          <span className="text-gray-400 [&_svg]:size-4">
            <ChevronIcon dir="both" />
          </span>
        )}
      </button>
      {open && (
        <ul
          id={listId}
          role="listbox"
          aria-label={tenantLabel}
          className="absolute inset-x-3 top-full z-30 mt-1 origin-top animate-pop rounded-xl bg-white p-1 shadow-lg ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10"
        >
          {tenancy.tenants.map((t) => {
            const selected = String(t.id) === String(tenancy.current)
            return (
              <li key={String(t.id)} role="option" aria-selected={selected}>
                <button
                  type="button"
                  onClick={() => void choose(String(t.id))}
                  className="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-start text-sm hover:bg-gray-100 dark:hover:bg-white/5"
                >
                  <span aria-hidden="true" className="flex size-6 items-center justify-center rounded-md bg-gray-100 text-[11px] font-bold text-gray-600 dark:bg-white/10 dark:text-gray-300">
                    {t.name.trim().charAt(0)}
                  </span>
                  <span className="flex-1 truncate">{t.name}</span>
                  {selected && (
                    <span className="text-primary-600 dark:text-primary-400">
                      <CheckIcon small />
                    </span>
                  )}
                </button>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
