import type { SVGProps } from 'react'

// A few outline icons drawn for the panel (24×24, stroke-based).
function Icon({ d, ...props }: SVGProps<SVGSVGElement> & { d: string }) {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round"
      strokeLinejoin="round" aria-hidden="true" className="size-5" {...props}>
      <path d={d} />
    </svg>
  )
}

export const MenuIcon = () => <Icon d="M4 6h16M4 12h16M4 18h16" />
export const CloseIcon = () => <Icon d="M6 6l12 12M18 6L6 18" />
export const SunIcon = () => (
  <Icon d="M12 3v2M12 19v2M5 12H3M21 12h-2M6.3 6.3 4.9 4.9M19.1 19.1l-1.4-1.4M6.3 17.7l-1.4 1.4M19.1 4.9l-1.4 1.4M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8z" />
)
export const MoonIcon = () => <Icon d="M20 14.5A8 8 0 0 1 9.5 4 8 8 0 1 0 20 14.5z" />
export const MonitorIcon = () => <Icon d="M3 5h18v11H3zM8 20h8M12 16v4" />
export const LogoutIcon = () => <Icon d="M15 4h4v16h-4M10 8l-4 4 4 4M6 12h10" />
export const CheckIcon = ({ small }: { small?: boolean }) => (
  <Icon d="M5 12.5l4.5 4.5L19 7.5" className={small ? 'size-4' : 'size-5'} />
)
export const XIcon = () => <Icon d="M7 7l10 10M17 7L7 17" />
export const CopyIcon = () => <Icon d="M9 9h10v10H9zM5 15V5h10" className="size-4" />
export const ChevronIcon = ({ dir }: { dir: 'up' | 'down' | 'both' | 'left' | 'right' }) => (
  <Icon
    d={dir === 'up' ? 'M7 14l5-5 5 5' : dir === 'down' ? 'M7 10l5 5 5-5' : dir === 'left' ? 'M14 7l-5 5 5 5' : dir === 'right' ? 'M10 7l5 5-5 5' : 'M8 9l4-4 4 4M8 15l4 4 4-4'}
    className="size-4"
  />
)
export const WarningIcon = () => <Icon d="M12 9v4M12 16.5v.01M10.3 4.2 2.6 18a2 2 0 0 0 1.7 3h15.4a2 2 0 0 0 1.7-3L13.7 4.2a2 2 0 0 0-3.4 0z" />
export const FunnelIcon = () => <Icon d="M4 5h16l-6 7.5V19l-4-2v-4.5z" className="size-4" />
export const SearchIcon = () => <Icon d="M11 4a7 7 0 1 0 0 14 7 7 0 0 0 0-14zM20 20l-4-4" className="size-4" />
export const PlusIcon = () => <Icon d="M12 5v14M5 12h14" className="size-4" />
export const ColumnsIcon = () => <Icon d="M4 5h16v14H4zM9.5 5v14M14.5 5v14" className="size-4" />
export const ReorderIcon = () => <Icon d="M8 4v16M5 7l3-3 3 3M16 20V4M13 17l3 3 3-3" className="size-4" />
export const GroupIcon = () => <Icon d="M4 6h16M4 12h10M4 18h7" className="size-4" />
export const BreadcrumbSeparator = () => <Icon d="M14 6l-6 6 6 6" className="size-3.5 rtl:rotate-0 ltr:rotate-180" />

// Resource icons: pass one as `icon` in defineResource.
export const DocumentIcon = () => <Icon d="M7 3h7l5 5v13H7zM14 3v5h5M10 13h6M10 17h6" />
export const FolderIcon = () => <Icon d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" />
export const TagIcon = () => <Icon d="M3 12V4h8l9 9-8 8zM7.5 7.5h.01" />
export const UsersIcon = () => <Icon d="M16 19v-1a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v1M9 10a3 3 0 1 0 0-6 3 3 0 0 0 0 6zM22 19v-1a4 4 0 0 0-3-3.87M16 4.13a3 3 0 0 1 0 5.74" />
export const SquaresIcon = () => <Icon d="M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z" />
export const HomeIcon = () => <Icon d="M3 10.5 12 3l9 7.5M5 9v11h5v-6h4v6h5V9" />
export const ChartIcon = () => <Icon d="M4 20V10M10 20V4M16 20v-7M22 20H2" />
export const EyeIcon = () => <Icon d="M2 12s3.5-7 10-7 10 7 10 7-3.5 7-10 7S2 12 2 12zM12 9a3 3 0 1 0 0 6 3 3 0 0 0 0-6z" />
export const BellIcon = () => <Icon d="M6 8a6 6 0 1 1 12 0c0 7 3 9 3 9H3s3-2 3-9M10.3 21a1.94 1.94 0 0 0 3.4 0" />
export const TrashIcon = () => <Icon d="M3 6h18M8 6V4h8v2M6 6l1 14h10l1-14" className="size-4" />
export const InfoIcon = () => <Icon d="M12 8h.01M11 12h1v4h1M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18z" />
