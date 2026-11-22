import { Outlet, useLocation, useParams } from '@tanstack/react-router'
import { Fragment, useEffect, useRef, useState, type ReactNode } from 'react'
import type { Theme } from '../theme'
import { ChevronIcon, CloseIcon, HomeIcon, LogoutIcon, MenuIcon, MonitorIcon, MoonIcon, SquaresIcon, SunIcon } from './icons'
import { GlobalSearch } from './GlobalSearch'
import { NotificationBell } from './Notifications'
import { TenantSwitcher } from './TenantSwitcher'
import { usePanel, type PanelNavigation } from './Panel'
import { SidebarNav } from './SidebarNav'
import type { MetaResponse } from '../protocol'
import type { ResourceDefinition } from '../resource'
import type { PageDefinition } from '../pages'
import type { DashboardDefinition } from '../widgets'
import type { NavigationGroupDefinition, UserMenuItemDefinition } from '../navigation'
import { Modal } from './Modal'
import { iconButton } from './styles'
import { customLocales, type Locale, locale, setLocale, t } from '../i18n'
import type { Color } from '../table'
import { useAppearance } from '../appearance'

const DESKTOP = '(min-width: 1024px)'
const SIDEBAR_KEY = 'mks-sidebar-open'


function useMediaQuery(query: string): boolean {
  const [matches, setMatches] = useState(() => window.matchMedia(query).matches)
  useEffect(() => {
    const m = window.matchMedia(query)
    const onChange = () => setMatches(m.matches)
    m.addEventListener('change', onChange)
    return () => m.removeEventListener('change', onChange)
  }, [query])
  return matches
}

export function Layout() {
  const { meta, resources, dashboard, pages, navigation } = usePanel()
  const { rail } = useAppearance()
  const { resource: current } = useParams({ strict: false }) as { resource?: string }
  const { pathname } = useLocation()
  const desktop = useMediaQuery(DESKTOP)
  // On desktop the sidebar can shrink to icons (remembered); on mobile it is a drawer.
  const [desktopOpen, setDesktopOpen] = useState(() => localStorage.getItem(SIDEBAR_KEY) !== 'false')
  const railMode = desktop && !desktopOpen
  const [mobileOpen, setMobileOpen] = useState(false)

  function toggleSidebar() {
    if (desktop) {
      setDesktopOpen((open) => {
        localStorage.setItem(SIDEBAR_KEY, String(!open))
        return !open
      })
    } else {
      setMobileOpen((open) => !open)
    }
  }

  useEffect(() => {
    if (!mobileOpen) return
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setMobileOpen(false)
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [mobileOpen])

  const brand = meta.panel.brand

  const all = navigationItems({ meta, resources, pages, dashboard, navigation }, current, pathname)
  const navGroups = groupNavigation(all, t('nav.menu'), meta.extra?.navigation as NavOverrides | undefined, navigation.groups)

  return (
    <div className="relative flex min-h-screen text-gray-950 dark:text-white">
      {/* A soft glow behind the top of the page. */}
      <div
        aria-hidden="true"
        className="pointer-events-none fixed inset-x-0 top-0 -z-10 h-[28rem] bg-[radial-gradient(60rem_24rem_at_70%_-6rem,color-mix(in_oklch,var(--color-primary-400)_18%,transparent),transparent)] dark:bg-[radial-gradient(60rem_24rem_at_70%_-6rem,color-mix(in_oklch,var(--color-primary-500)_16%,transparent),transparent)]"
      />

      {/* Mobile drawer backdrop. */}
      <div
        aria-hidden="true"
        onClick={() => setMobileOpen(false)}
        className={
          'fixed inset-0 z-30 bg-gray-950/40 backdrop-blur-sm transition-opacity duration-300 lg:hidden ' +
          (mobileOpen ? 'opacity-100' : 'pointer-events-none opacity-0')
        }
      />

      <aside
        aria-label={t('nav.main')}
        inert={!desktop && !mobileOpen}
        className={
          // A dark or brand sidebar is drawn in dark mode (the dark variant follows .dark).
          (rail === 'light' ? '' : 'dark ') +
          (rail === 'dark' ? 'mks-rail-dark ' : '') +
          (rail === 'brand' ? 'mks-rail-brand ' : '') +
          'fixed inset-y-0 start-0 z-40 flex shrink-0 flex-col border-e border-gray-200/80 bg-white/90 backdrop-blur-xl text-gray-950 dark:text-white ' +
          (railMode ? 'w-[4.75rem] ' : 'w-72 ') +
          'transition-transform duration-300 ease-out-soft dark:border-white/5 dark:bg-gray-900/90 ' +
          'lg:sticky lg:top-0 lg:z-auto lg:h-screen lg:bg-white/60 lg:dark:bg-gray-900/40 ' +
          (mobileOpen ? '' : 'max-lg:ltr:-translate-x-full max-lg:rtl:translate-x-full ')
        }
      >
        <div className={'flex h-16 items-center gap-3 ' + (railMode ? 'justify-center px-2' : 'px-5')}>
          <span
            aria-hidden="true"
            className="flex size-9 items-center justify-center rounded-xl bg-gradient-to-br from-primary-400 to-primary-700 text-base font-bold text-white shadow-md shadow-primary-600/30 ring-1 ring-inset ring-white/20"
          >
            {brand.trim().charAt(0)}
          </span>
          {!railMode && <span className="min-w-0 flex-1 truncate text-[15px] font-bold tracking-tight">{brand}</span>}
          <button type="button" className={iconButton + ' lg:hidden'} onClick={() => setMobileOpen(false)} aria-label={t('nav.closeMenu')}>
            <CloseIcon />
          </button>
        </div>

        {!railMode && <TenantSwitcher />}
        <SidebarNav groups={navGroups} mode={!desktop ? 'mobile' : railMode ? 'rail' : 'open'} onNavigate={() => setMobileOpen(false)} />
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        <Topbar onToggleSidebar={toggleSidebar} />
        <main className="mx-auto w-full max-w-7xl flex-1 px-4 py-6 sm:px-6 lg:px-8 lg:py-8">
          {/* Keyed by path: each page fades in. */}
          <div key={pathname} className="animate-page">
            <Outlet />
          </div>
        </main>
      </div>
    </div>
  )
}

export interface NavItem {
  key: string
  to: string
  params?: Record<string, string>
  label: string
  icon: ReactNode
  active: boolean
  group?: string
  /** Order in its group: lower first (default: after explicit sorts, in definition order). */
  sort?: number
  /** Never hidden by the menu editor (it holds the editor itself). */
  protected?: boolean
  /** A count from the server (Config.NavigationBadge), e.g. pending comments. */
  badge?: { count: number; color?: Color }
}

interface NavSources {
  meta: MetaResponse
  resources: ResourceDefinition[]
  pages: PageDefinition[]
  dashboard?: DashboardDefinition
  navigation: PanelNavigation
}

/** The pages of a cluster the user may open, in their order. */
export function clusterPages(pages: PageDefinition[], cluster: string, meta?: MetaResponse): PageDefinition[] {
  return pages
    .map((p, i) => ({ p, i }))
    .filter((x) => x.p.cluster === cluster && (!meta || !x.p.visible || x.p.visible(meta)))
    .sort((a, b) => (a.p.navigationSort ?? 1000 + a.i) - (b.p.navigationSort ?? 1000 + b.i))
    .map((x) => x.p)
}

/**
 * Every sidebar entry, before overrides: the dashboard, resources, pages, clusters (one
 * entry each, opening its first page) and extra links. current is the first URL segment.
 */
export function navigationItems({ meta, resources, pages, dashboard, navigation }: NavSources, current?: string, pathname = ''): NavItem[] {
  const link = navigation.links.filter((l) => !l.visible || l.visible(meta)).find((l) => pathname === l.to || pathname.endsWith(l.to))
  const items: NavItem[] = [
    ...resources
      .filter((r) => r.navigation !== false)
      .map((r) => ({
        key: `r:${r.name}`,
        to: '/$resource',
        params: { resource: r.name },
        label: r.pluralLabel,
        icon: r.icon ?? <SquaresIcon />,
        active: r.name === current && !link,
        group: r.group,
        sort: r.navigationSort,
        badge: meta.resources[r.name]?.badge,
      })),
    ...pages
      .filter((p) => p.navigation !== false && !p.cluster && (!p.visible || p.visible(meta)))
      .map((p) => ({
        key: `p:${p.name}`,
        to: '/$resource',
        params: { resource: p.name },
        label: p.label,
        icon: p.icon ?? <SquaresIcon />,
        active: p.name === current,
        group: p.group,
        sort: p.navigationSort,
        badge: meta.page_badges?.[p.name],
      })),
    ...navigation.clusters.flatMap((c) => {
      const inside = clusterPages(pages, c.name, meta)
      if (inside.length === 0) return []
      return [
        {
          key: `c:${c.name}`,
          to: '/$resource',
          params: { resource: inside[0].name },
          label: c.label,
          icon: c.icon ?? <SquaresIcon />,
          active: inside.some((p) => p.name === current),
          group: c.group,
          sort: c.sort,
          protected: inside.some((p) => p.name === 'admin_menu'),
        },
      ]
    }),
    ...navigation.links
      .filter((l) => !l.visible || l.visible(meta))
      .map((l) => ({ key: `l:${l.key}`, to: l.to, label: l.label, icon: l.icon ?? <SquaresIcon />, active: l === link, group: l.group, sort: l.sort })),
  ]
  return dashboard ? [{ key: 'dashboard', to: '/', label: dashboard.heading, icon: <HomeIcon />, active: !current, sort: -2 }, ...items] : items
}

/** Where an item sits after a manager edited it (MKSine's "placement"). */
export type NavPlacement = 'default' | 'hidden' | 'group'

export interface NavItemOverride {
  /** "default", "hidden" or "group" (moved into NavItemOverride.group). */
  placement?: NavPlacement
  /** The older shape of placement: hidden; still honoured. */
  hidden?: boolean
  label?: string
  /** The group an item is moved into. */
  group?: string
  sort?: number
}

/** Sidebar changes a manager made (the navigation plugin): by item key and group heading. */
export interface NavOverrides {
  items?: Record<string, NavItemOverride>
  groups?: Record<string, { hidden?: boolean; label?: string; sort?: number }>
}

/**
 * The sidebar's groups: items without a group first (under the menu heading), then the
 * groups in the panel's order (navigationGroups), then the others in order of first
 * appearance; a manager's overrides are applied. The entry that holds the menu editor,
 * and its group, are never hidden, so the menu can always be repaired.
 */
export function groupNavigation(
  items: NavItem[],
  menuHeading: string,
  nav?: NavOverrides,
  order: NavigationGroupDefinition[] = [],
): { heading: string; icon?: ReactNode; ungrouped: boolean; items: NavItem[] }[] {
  const groups: { key: string; heading: string; icon?: ReactNode; items: { item: NavItem; sort: number }[]; sort: number }[] = []
  const keeps = (i: NavItem) => i.key === 'p:admin_menu' || !!i.protected
  items.forEach((original, index) => {
    const o = nav?.items?.[original.key]
    // "hidden" takes the item out of the sidebar (the URL and its permissions stay);
    // "group" moves it to o.group, which the key below picks up.
    const placement = o?.placement ?? (o?.hidden ? 'hidden' : 'default')
    if (placement === 'hidden' && !keeps(original)) return
    const item = o?.label ? { ...original, label: o.label } : original
    const key = o?.group || original.group || ''
    let g = groups.find((x) => x.key === key)
    if (!g) {
      const at = order.findIndex((d) => d.label === key)
      const sort = key === '' ? -1 : at >= 0 ? (at + 1) * 10 : 1000 + (groups.length + 1) * 10
      groups.push((g = { key, heading: key || menuHeading, icon: at >= 0 ? order[at].icon : undefined, items: [], sort }))
    }
    g.items.push({ item, sort: o?.sort ?? original.sort ?? (index + 1) * 10 })
  })
  return groups
    .map((g) => {
      const o = g.key ? nav?.groups?.[g.key] : undefined
      const keepsEditor = g.items.some((x) => keeps(x.item))
      return { ...g, hidden: !!o?.hidden && !keepsEditor, heading: o?.label || g.heading, sort: o?.sort ?? g.sort }
    })
    .filter((g) => !g.hidden && g.items.length > 0)
    .sort((a, b) => a.sort - b.sort)
    .map((g) => ({ heading: g.heading, icon: g.icon, ungrouped: g.key === '', items: [...g.items].sort((a, b) => a.sort - b.sort).map((x) => x.item) }))
}

function Topbar({ onToggleSidebar }: { onToggleSidebar: () => void }) {
  const { topbar } = usePanel()
  return (
    <header className="sticky top-0 z-20 flex h-16 items-center gap-2 border-b border-gray-200/70 bg-white/70 px-4 backdrop-blur-xl sm:px-6 dark:border-white/5 dark:bg-gray-950/60">
      <button type="button" className={iconButton} onClick={onToggleSidebar} aria-label={t('nav.toggleMenu')}>
        <MenuIcon />
      </button>
      <GlobalSearch />
      <div className="flex-1" />
      {topbar.map((item, i) => (
        <Fragment key={i}>{item}</Fragment>
      ))}
      <NotificationBell />
      <ThemeMenu />
      <UserMenu />
    </header>
  )
}

// Each language named in itself; languages managers added come after the built-in ones.
const BUILT_IN_LANGUAGES: { value: Locale; label: string }[] = [
  { value: 'fa', label: 'Persian' },
  { value: 'en', label: 'English' },
]

const themes: { value: Theme; label: string; icon: () => React.JSX.Element }[] = [
  { value: 'light', label: t('theme.light'), icon: SunIcon },
  { value: 'dark', label: t('theme.dark'), icon: MoonIcon },
  { value: 'system', label: t('theme.system'), icon: MonitorIcon },
]

function ThemeMenu() {
  const { theme, setTheme } = usePanel()
  const current = themes.find((t) => t.value === theme) ?? themes[2]
  const next = themes[(themes.indexOf(current) + 1) % themes.length]
  const CurrentIcon = current.icon
  return (
    <button
      type="button"
      className={iconButton}
      onClick={() => setTheme(next.value)}
      aria-label={t('theme.label', { current: current.label, next: next.label })}
      title={t('theme.title', { current: current.label })}
    >
      <CurrentIcon />
    </button>
  )
}

function UserMenu() {
  const { meta, logout, navigation } = usePanel()
  const [open, setOpen] = useState(false)
  const [dialog, setDialog] = useState<UserMenuItemDefinition | null>(null)
  const extra = navigation.userMenu
    .map((item, i) => ({ item, i }))
    .filter((x) => !x.item.visible || x.item.visible(meta))
    .sort((a, b) => (a.item.sort ?? 100 + a.i) - (b.item.sort ?? 100 + b.i))
    .map((x) => x.item)
  const ref = useRef<HTMLDivElement>(null)
  const user = meta.user

  useEffect(() => {
    if (!open) return
    const onClick = (e: MouseEvent) => ref.current && !ref.current.contains(e.target as Node) && setOpen(false)
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onClick)
    window.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onClick)
      window.removeEventListener('keydown', onKey)
    }
  }, [open])

  const initials = user.name.trim().split(/\s+/).map((w) => w[0]).slice(0, 2).join('')
  const languages = [...BUILT_IN_LANGUAGES, ...customLocales().map((l) => ({ value: l.code as Locale, label: l.name }))]

  return (
    <div ref={ref} className="relative">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={t('user.menu')}
        className="flex items-center gap-2 rounded-full p-1 pe-1 hover:bg-gray-900/5 sm:pe-3 dark:hover:bg-white/5"
      >
        <span className="flex size-8 items-center justify-center rounded-full bg-gradient-to-br from-primary-400 to-primary-700 text-xs font-semibold text-white ring-2 ring-white dark:ring-gray-900">
          {initials}
        </span>
        <span className="hidden text-sm font-medium text-gray-700 sm:block dark:text-gray-200">{user.name}</span>
        <span className="hidden text-gray-400 sm:block [&_svg]:size-4">
          <ChevronIcon dir="down" />
        </span>
      </button>
      {open && (
        <div
          role="menu"
          className="absolute end-0 mt-2 w-64 origin-top animate-pop rounded-2xl bg-white p-1.5 shadow-lg ring-1 ring-gray-950/5 dark:bg-gray-900 dark:ring-white/10"
        >
          <div className="flex items-center gap-3 px-3 py-2.5">
            <span className="flex size-9 shrink-0 items-center justify-center rounded-full bg-gradient-to-br from-primary-400 to-primary-700 text-sm font-semibold text-white">
              {initials}
            </span>
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold">{user.name}</p>
              <p className="truncate text-xs text-gray-500 dark:text-gray-400" dir="ltr">
                {user.email}
              </p>
            </div>
          </div>
          <div className="my-1 border-t border-gray-100 dark:border-white/5" />
          {extra.length > 0 && (
            <>
              {extra.map((item) => {
                const href = typeof item.href === 'function' ? item.href(meta) : item.href
                const cls = 'flex w-full items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium text-gray-700 hover:bg-gray-900/5 dark:text-gray-200 dark:hover:bg-white/5 [&_svg]:size-4 [&_svg]:text-gray-400'
                return href ? (
                  <a key={item.key} role="menuitem" href={href} target={item.newTab ? '_blank' : undefined} rel={item.newTab ? 'noopener' : undefined} className={cls} onClick={() => setOpen(false)}>
                    {item.icon}
                    {item.label}
                  </a>
                ) : (
                  <button
                    key={item.key}
                    type="button"
                    role="menuitem"
                    className={cls}
                    onClick={() => {
                      setOpen(false)
                      if (item.component) setDialog(item)
                    }}
                  >
                    {item.icon}
                    {item.label}
                  </button>
                )
              })}
              <div className="my-1 border-t border-gray-100 dark:border-white/5" />
            </>
          )}
          <div className="flex items-center gap-2 px-3 py-2">
            <span className="flex-1 text-sm text-gray-600 dark:text-gray-300">{t('user.language')}</span>
            {languages.length > 2 ? (
              <select
                aria-label={t('user.language')}
                value={locale()}
                onChange={(e) => setLocale(e.target.value)}
                className="rounded-lg border-0 bg-gray-100 py-1 ps-2 pe-7 text-xs font-medium dark:bg-white/5"
              >
                {languages.map((l) => (
                  <option key={l.value} value={l.value} lang={l.value.replace('_', '-')}>
                    {l.label}
                  </option>
                ))}
              </select>
            ) : (
            <div role="group" aria-label={t('user.language')} className="flex rounded-lg bg-gray-100 p-0.5 dark:bg-white/5">
              {languages.map((l) => (
                <button
                  key={l.value}
                  type="button"
                  lang={l.value}
                  aria-pressed={locale() === l.value}
                  onClick={() => locale() !== l.value && setLocale(l.value)}
                  className={
                    'rounded-md px-2.5 py-1 text-xs font-medium ' +
                    (locale() === l.value ? 'bg-white text-gray-950 shadow-xs dark:bg-white/10 dark:text-white' : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200')
                  }
                >
                  {l.label}
                </button>
              ))}
            </div>
            )}
          </div>
          <div className="my-1 border-t border-gray-100 dark:border-white/5" />
          <button
            type="button"
            role="menuitem"
            onClick={() => void logout()}
            className="flex w-full items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium text-danger-600 hover:bg-danger-50 dark:text-danger-400 dark:hover:bg-danger-500/10"
          >
            <LogoutIcon />
            {t('user.logout')}
          </button>
        </div>
      )}
      {dialog?.component && (
        <Modal title={dialog.label} onClose={() => setDialog(null)} size="lg">
          <dialog.component close={() => setDialog(null)} />
        </Modal>
      )}
    </div>
  )
}
