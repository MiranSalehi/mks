import type { ComponentType, ReactNode } from 'react'
import type { MetaResponse } from './protocol'

/**
 * A sidebar group, in the order the panel lists them (Panel `navigationGroups`). Groups
 * not listed come after, in order of first appearance.
 */
export interface NavigationGroupDefinition {
  /** The heading; items join it with the same `group`. */
  label: string
  icon?: ReactNode
}

/** An extra sidebar entry that is not a resource or page, e.g. "Upload a file". */
export interface NavigationLinkDefinition {
  /** Unique among links; the menu editor stores overrides under "l:{key}". */
  key: string
  label: string
  icon?: ReactNode
  group?: string
  sort?: number
  /** A path inside the panel, e.g. "/cms_media/create". */
  to: string
  /** Shown only when this says so (e.g. the user may create media). */
  visible?: (meta: MetaResponse) => boolean
}

/**
 * Pages grouped under one sidebar entry (Filament's clusters), e.g. "Settings": the
 * entry opens its first page, and each page shows the others as tabs.
 */
export interface ClusterDefinition {
  name: string
  label: string
  icon?: ReactNode
  group?: string
  sort?: number
}

/** An entry of the user menu (under the avatar). */
export interface UserMenuItemDefinition {
  key: string
  label: string
  icon?: ReactNode
  /** Lower first; logout is always last. */
  sort?: number
  /** A link (opened in a new tab with newTab). */
  href?: string | ((meta: MetaResponse) => string | undefined)
  newTab?: boolean
  /** Shown in a modal instead of following a link. */
  component?: ComponentType<{ close: () => void }>
  visible?: (meta: MetaResponse) => boolean
}

export function defineCluster(cluster: ClusterDefinition): ClusterDefinition {
  return cluster
}
