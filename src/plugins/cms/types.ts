// Records of the CMS resources, as the Go plugin (packages/mks/plugins/cms)
// sends them.

export interface Block {
  type: string
  data: Record<string, unknown>
}

export interface Page {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  title: string
  slug: string
  type: string
  status: string
  content: string | null
  builder_payload: Block[]
  show_page_header: boolean
  builder_content_width: string
  meta_title: string | null
  meta_description: string | null
  focus_keyphrase: string | null
  published_at: string | null
  deleted_at: string | null
  created_by: number | null
  updated_by: number | null
  revisions?: PageRevision[]
  tags?: Tag[]
  /** Media ids of the gallery collection. */
  gallery?: number[]
}

export type PageTrash = Omit<Page, 'revisions' | 'tags'>

export interface PageRevision {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  page_id: number
  user_id: number | null
  snapshot: Record<string, unknown>
}

export interface MenuItem {
  type: string
  label: string
  page_id: number | null
  post_id: number | null
  category_id: number | null
  tag_id: number | null
  url: string
  new_tab: boolean
  /** Submenu entries, nested up to six levels. */
  children: MenuItem[]
}

/** @deprecated Submenus nest now: a child is a MenuItem. */
export type MenuChildItem = MenuItem

export interface Menu {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  name: string
  items: MenuItem[]
}

export interface MenuLocation {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  location: string
  label: string
  menu_id: number | null
  menu?: Menu
}

export interface Post {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  title: string
  slug: string
  excerpt: string | null
  content: string | null
  status: string
  featured_image: string | null
  author_id: string | null
  published_at: string | null
  meta_title: string | null
  meta_description: string | null
  focus_keyphrase: string | null
  views_count: number
  categories?: Category[]
  tags?: Tag[]
  /** Media ids of the gallery collection. */
  gallery?: number[]
}

export interface Category {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  name: string
  slug: string
  description: string | null
  image: string | null
  parent_id: number | null
  sort_order: number
  is_active: boolean
  path: string
  path_name: string
  meta_title: string | null
  meta_description: string | null
  focus_keyphrase: string | null
  parent?: Category | null
  children?: Category[]
  posts?: Post[]
  /** Number of posts (lists only). */
  posts_count?: number
}

export interface Tag {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  name: string
  slug: string
  description: string | null
  is_active: boolean
  meta_title: string | null
  meta_description: string | null
  focus_keyphrase: string | null
  /** Number of posts (lists only). */
  posts_count?: number
}

export interface Comment {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  post_id: number | null
  page_id: number | null
  parent_id: number | null
  user_id: string | null
  author_name: string
  author_email: string | null
  content: string
  rating: number | null
  status: string
  ip_address: string | null
  user_agent: string | null
  post?: Post | null
  page?: Page | null
}

export interface Media {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  path: string
  file_name: string
  mime_type: string
  size: number
  width: number | null
  height: number | null
  alt: string | null
  title: string | null
  caption: string | null
  uploaded_by: string | null
  conversions: Record<string, string>
}

export interface Section {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  name: string
  builder_payload: Block[]
  created_by: string | null
}

export interface Entry {
  created_at: string | null
  updated_at: string | null
  id: number
  site_id: number
  content_type: string
  title: string
  slug: string
  excerpt: string | null
  content: string | null
  status: string
  featured_image: string | null
  author_id: string | null
  published_at: string | null
  meta_title: string | null
  meta_description: string | null
  focus_keyphrase: string | null
}
