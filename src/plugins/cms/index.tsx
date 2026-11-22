import { definePlugin, tr, usePanel, type PanelPlugin, type ResourceDefinition } from '../../index'
import { categories } from './categories'
import { comments, commentStatusLabels } from './comments'
import { transferPage } from './transfer'
import { hooksPage } from './hooks'
import { registerEditorButtons } from './editorButtons'
import { media, mediaAccept, mediaGallery, mediaPicker } from './media'
import { entryResource, type ContentTypeDef } from './entries'
import { menus } from './menus'
import { pageRevisions } from './page-revisions'
import { pageTrash } from './page-trash'
import { pages } from './pages'
import { postsResource, postStatusLabels } from './posts'
import { sections } from './sections'
import { settingsPages, type SettingsTab } from './settings'
import { menuLocations } from './menuLocations'
import { menuBuilderPage } from './menuBuilder'
import { pluginsPage, themesPage } from './packages'
import { setPackageBlocks, type BlockDef } from './builder'
import { tags } from './tags'

export * from './types'
export type { ContentTypeDef, SettingsTab }
export { entryResource }
export { blockDefs, builderBlocks, LivePreview } from './builder'
export { addSeoCheck, analyzeSeo, type SeoCheck, type SeoCheckContext } from './seoAnalysis'
export { categories, comments, commentStatusLabels, media, mediaAccept, mediaGallery, mediaPicker, menuLocations, menus, pageRevisions, pageTrash, pages, pluginsPage, postsResource, postStatusLabels, sections, settingsPages, tags, themesPage }

export interface CmsOptions {
  /** Sidebar group of posts, pages, categories, sections, comments and tags. Default: "محتوا". */
  contentGroup?: string
  /** Sidebar group of the media library. Default: "رسانه". */
  mediaGroup?: string
  /** Sidebar group of themes, menus and menu locations. Default: "ظاهر". */
  appearanceGroup?: string
  /**
   * The cluster of the settings pages (General, Permalinks, Performance, Extensions).
   * Default: "settings", a "تنظیمات" entry the plugin adds; other plugins' pages can join it.
   */
  settingsCluster?: string
  /** The resource authors are picked from. Default: "users". */
  usersResource?: string
  /** Extra tabs on the Extensions settings page (their keys are registered in Go: RegisterSettings). */
  settingsTabs?: SettingsTab[]
  /** Custom content types, the same as cms.Plugin.ContentTypes in Go. */
  contentTypes?: ContentTypeDef[]
  /** The import/export page (Plugin.ImportExport in Go). */
  importExport?: boolean
  /** The hooks page (Plugin.HooksPage in Go). */
  hooks?: boolean
}

/** Resources of uploaded plugins' content types, kept so forms keep their state. */
const packageTypes = new Map<string, ResourceDefinition>()

const GearIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-5">
    <circle cx="12" cy="12" r="3" />
    <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-2.8 1.2V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-2.9-1.2l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0-1.2-2.9H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.2-2.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1A1.7 1.7 0 0 0 10 3.1V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 2.9 1.2l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0 1.2 2.9H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z" />
  </svg>
)

const ExternalIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" className="size-4">
    <path d="M14 4h6v6M20 4l-9 9M18 14v5a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V7a1 1 0 0 1 1-1h5" />
  </svg>
)

/** "View site" in the topbar (MKSine's topbar link). */
function ViewSiteLink() {
  const { meta } = usePanel()
  const url = meta.extra?.cms_site_url
  if (typeof url !== 'string') return null
  return (
    <a href={url} target="_blank" rel="noopener" className="hidden items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 hover:bg-gray-900/5 sm:flex dark:text-gray-300 dark:hover:bg-white/5">
      <ExternalIcon />
      {tr({ fa: 'مشاهده سایت', en: 'View site' })}
    </a>
  )
}

const PlusIcon = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" aria-hidden="true" className="size-5">
    <circle cx="12" cy="12" r="9" />
    <path d="M12 8v8M8 12h8" />
  </svg>
)

/**
 * The CMS, laid out like MKSine's admin: media (library, upload), content (posts, pages,
 * categories, saved sections, comments, tags), appearance (themes, menus, menu
 * locations), plugins and settings. Install the Go plugin (cms.Plugin) too.
 */
export function cms(options: CmsOptions = {}): PanelPlugin {
  // "Insert Media" and "Insert shortcode" at the start of the rich editors' toolbar.
  registerEditorButtons()
  const contentGroup = options.contentGroup ?? tr({ fa: 'محتوا', en: 'Content' })
  const mediaGroup = options.mediaGroup ?? tr({ fa: 'رسانه', en: 'Media' })
  const appearanceGroup = options.appearanceGroup ?? tr({ fa: 'ظاهر', en: 'Appearance' })
  const cluster = options.settingsCluster ?? 'settings'
  const content = <R extends { group?: string }>(r: R): R => ({ ...r, group: contentGroup })
  return definePlugin({
    name: 'cms',
    resources: [
      content(postsResource(options.usersResource)),
      content(pages),
      content(categories),
      content(sections),
      content(comments),
      content(tags),
      ...(options.contentTypes ?? []).map((ct, i) => ({ ...content(entryResource(ct)), navigationSort: 5 + i })),
      { ...media, group: mediaGroup },
      content(pageRevisions),
      content(pageTrash),
      { ...menus, group: appearanceGroup },
      { ...menuLocations, group: appearanceGroup },
    ],
    pages: [
      themesPage({ group: appearanceGroup }),
      pluginsPage(),
      menuBuilderPage,
      ...settingsPages(options.settingsTabs ?? [], { cluster }),
      ...(options.importExport ? [transferPage(contentGroup)] : []),
      ...(options.hooks ? [hooksPage(appearanceGroup)] : []),
    ],
    clusters: options.settingsCluster ? [] : [{ name: 'settings', label: tr({ fa: 'تنظیمات', en: 'Settings' }), icon: <GearIcon />, sort: 80 }],
    navigationLinks: [
      {
        key: 'media_upload',
        label: tr({ fa: 'افزودن عکس / فایل', en: 'Add a photo / file' }),
        icon: <PlusIcon />,
        group: mediaGroup,
        sort: 11,
        to: '/cms_media/create',
        visible: (meta) => !!meta.resources.cms_media?.abilities.create,
      },
    ],
    userMenuItems: [
      {
        key: 'view_site',
        label: tr({ fa: 'مشاهده سایت', en: 'View site' }),
        icon: <ExternalIcon />,
        sort: 20,
        newTab: true,
        href: (meta) => meta.extra?.cms_site_url as string | undefined,
        visible: (meta) => typeof meta.extra?.cms_site_url === 'string',
      },
    ],
    topbar: <ViewSiteLink />,
    // Content types of uploaded plugins: their list and form screens, made once per type.
    metaResources: (meta) => {
      const types = (meta.extra?.cms_content_types as ContentTypeDef[] | undefined) ?? []
      const codeKeys = new Set((options.contentTypes ?? []).map((ct) => ct.key))
      return types
        .filter((ct) => !codeKeys.has(ct.key))
        .map((ct, i) => {
          const id = `${ct.key}\u0000${ct.singular}\u0000${ct.plural}`
          let def = packageTypes.get(id)
          if (!def) {
            def = { ...content(entryResource(ct)), navigationSort: 20 + i }
            packageTypes.set(id, def)
          }
          return def
        })
    },
    // Blocks of uploaded plugins, into every builder before forms render.
    onMeta: (meta) => setPackageBlocks(meta.extra?.cms_blocks as BlockDef[] | undefined),
  })
}
