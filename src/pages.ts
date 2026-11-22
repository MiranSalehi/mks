import type { ComponentType, ReactNode } from 'react'
import type { MetaResponse } from './protocol'

/**
 * A custom page: any React component, in the navigation and at /{name}. The server
 * registers a panel.Page with the same name; it decides who sees the page and supplies
 * its data and form handling (usePageData, usePageSubmit).
 */
export interface PageDefinition {
  name: string
  label: string
  icon?: ReactNode
  /** A navigation group heading, e.g. "گزارش‌ها". */
  group?: string
  /** Order in its group (or cluster): lower first. */
  navigationSort?: number
  /** false: not in the sidebar (still reachable by its URL and links). */
  navigation?: boolean
  /** The cluster (defineCluster) the page belongs to: it shows under the cluster's entry. */
  cluster?: string
  /** Shown in the sidebar and cluster tabs only when this says so (from what the server sent). */
  visible?: (meta: MetaResponse) => boolean
  component: ComponentType
}

export function definePage(page: PageDefinition): PageDefinition {
  return page
}
