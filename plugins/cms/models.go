package cms

import (
	"time"

	"github.com/goravel/framework/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// Table names. Every CMS table carries site_id, the key of the panel tenant the row
// belongs to; a site is a tenant.
const (
	PagesTable         = "cms_pages"
	PageRevisionsTable = "cms_page_revisions"
	MenusTable         = "cms_menus"
	PostsTable         = "cms_posts"
	CategoriesTable    = "cms_categories"
	CategoryPostTable  = "cms_category_post"
	TagsTable          = "cms_tags"
	PostTagTable       = "cms_post_tag"
	PageTagTable       = "cms_page_tag"
	CommentsTable      = "cms_comments"
	MediaTable         = "cms_media"
)

// Page is a CMS page of a site: simple (rich text) or block-based. Its slug is unique
// within the site. A page in the trash keeps its slug (DeletedAt is a plain column, not
// GORM soft delete, so the trash can list it).
type Page struct {
	orm.Model
	SiteID              uint              `json:"site_id"`
	Title               string            `json:"title"`
	Slug                string            `json:"slug"`
	Type                string            `json:"type"`
	Status              string            `json:"status"`
	Content             *string           `json:"content"`
	BuilderPayload      mks.JSON[[]Block] `json:"builder_payload"`
	ShowPageHeader      bool              `json:"show_page_header"`
	BuilderContentWidth string            `json:"builder_content_width"`
	MetaTitle           *string           `json:"meta_title"`
	MetaDescription     *string           `json:"meta_description"`
	FocusKeyphrase      *string           `json:"focus_keyphrase"`
	PublishedAt         *time.Time        `json:"published_at"`
	DeletedAt           *time.Time        `json:"deleted_at"`
	CreatedBy           *uint             `json:"created_by"`
	UpdatedBy           *uint             `json:"updated_by"`
	Revisions           []*PageRevision   `gorm:"foreignKey:PageID" json:"revisions,omitempty"`
	Tags                []*Tag            `gorm:"many2many:cms_page_tag;joinForeignKey:PageID;joinReferences:TagID" json:"tags,omitempty"`
	// Gallery is the media ids of the "gallery" collection (panel forms; not a column).
	Gallery []uint `gorm:"-" json:"gallery"`
}

func (Page) TableName() string { return PagesTable }

// PageTrash is the pages table seen as the trash (its own resource and policy).
type PageTrash Page

func (PageTrash) TableName() string { return PagesTable }

// Block is one content block of a block-based page: a type and that type's data.
type Block struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// PageRevision is an immutable snapshot of a page, made on every save and restore.
type PageRevision struct {
	orm.Model
	SiteID   uint                     `json:"site_id"`
	PageID   uint                     `json:"page_id"`
	UserID   *uint                    `json:"user_id"`
	Snapshot mks.JSON[map[string]any] `json:"snapshot"`
}

func (PageRevision) TableName() string { return PageRevisionsTable }

// previewGrant is a short-lived, user-bound capability to view one unpublished page,
// kept as JSON in the cache.
type previewGrant struct {
	UserID string `json:"user_id"`
	SiteID uint   `json:"site_id"`
	PageID uint   `json:"page_id"`
}

// Menu is a navigation menu of a site. Its items are a two-level tree in one JSON column;
// Location places it on the public site (one menu per location and site).
type Menu struct {
	orm.Model
	SiteID uint                 `json:"site_id"`
	Name   string               `json:"name"`
	Items  mks.JSON[[]MenuItem] `json:"items"`
}

func (Menu) TableName() string { return MenusTable }

// MenuItem is a menu entry: a page, post, category or tag of the same site, a link, or a
// group (a label without a link). An entry with children opens its submenu instead of
// being a link. Submenus nest up to MaxMenuDepth levels (MKSine's menus have any depth).
type MenuItem struct {
	Type       string     `json:"type"`
	Label      string     `json:"label"`
	PageID     *uint      `json:"page_id"`
	PostID     *uint      `json:"post_id"`
	CategoryID *uint      `json:"category_id"`
	TagID      *uint      `json:"tag_id"`
	URL        string     `json:"url"`
	NewTab     bool       `json:"new_tab"`
	Children   []MenuItem `json:"children"`
}

// Post is a blog post of a site. Status is draft, published or archived; a published post
// with a future published_at stays hidden until then (no "scheduled" status, as in
// MKSine). Slugs are unique within the site.
type Post struct {
	orm.Model
	SiteID          uint        `json:"site_id"`
	Title           string      `json:"title"`
	Slug            string      `json:"slug"`
	Excerpt         *string     `json:"excerpt"`
	Content         *string     `json:"content"`
	Status          string      `json:"status"`
	FeaturedImage   *string     `json:"featured_image"`
	AuthorID        *string     `json:"author_id"`
	PublishedAt     *time.Time  `json:"published_at"`
	MetaTitle       *string     `json:"meta_title"`
	MetaDescription *string     `json:"meta_description"`
	FocusKeyphrase  *string     `json:"focus_keyphrase"`
	ViewsCount      uint64      `json:"views_count"`
	Categories      []*Category `gorm:"many2many:cms_category_post;joinForeignKey:PostID;joinReferences:CategoryID" json:"categories,omitempty"`
	Tags            []*Tag      `gorm:"many2many:cms_post_tag;joinForeignKey:PostID;joinReferences:TagID" json:"tags,omitempty"`
	// Gallery is the media ids of the "gallery" collection (panel forms; not a column).
	Gallery []uint `gorm:"-" json:"gallery"`
}

func (Post) TableName() string { return PostsTable }

// Category is a node of a site's category tree, of any depth. Path is the full slug path
// ("news/sport") and PathName the full name ("News › Sport"); both are kept up to date on
// save, also for the descendants, so a URL resolves with one query.
type Category struct {
	orm.Model
	SiteID          uint        `json:"site_id"`
	Name            string      `json:"name"`
	Slug            string      `json:"slug"`
	Description     *string     `json:"description"`
	Image           *string     `json:"image"`
	ParentID        *uint       `json:"parent_id"`
	SortOrder       int         `json:"sort_order"`
	IsActive        bool        `json:"is_active"`
	Path            string      `json:"path"`
	PathName        string      `json:"path_name"`
	MetaTitle       *string     `json:"meta_title"`
	MetaDescription *string     `json:"meta_description"`
	FocusKeyphrase  *string     `json:"focus_keyphrase"`
	Parent          *Category   `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
	Children        []*Category `gorm:"foreignKey:ParentID" json:"children,omitempty"`
	Posts           []*Post     `gorm:"many2many:cms_category_post;joinForeignKey:CategoryID;joinReferences:PostID" json:"posts,omitempty"`
	// PostsCount is filled in panel lists (not a column).
	PostsCount int64 `gorm:"-" json:"posts_count"`
}

func (Category) TableName() string { return CategoriesTable }

// Tag labels posts and pages of a site.
type Tag struct {
	orm.Model
	SiteID          uint    `json:"site_id"`
	Name            string  `json:"name"`
	Slug            string  `json:"slug"`
	Description     *string `json:"description"`
	IsActive        bool    `json:"is_active"`
	MetaTitle       *string `json:"meta_title"`
	MetaDescription *string `json:"meta_description"`
	FocusKeyphrase  *string `json:"focus_keyphrase"`
	Posts           []*Post `gorm:"many2many:cms_post_tag;joinForeignKey:TagID;joinReferences:PostID" json:"posts,omitempty"`
	Pages           []*Page `gorm:"many2many:cms_page_tag;joinForeignKey:TagID;joinReferences:PageID" json:"pages,omitempty"`
	// PostsCount is filled in panel lists (not a column).
	PostsCount int64 `gorm:"-" json:"posts_count"`
}

func (Tag) TableName() string { return TagsTable }

// Comment is a reader's comment on a post (or a page, with Plugin.CommentsOnPages). Exactly
// one of PostID and PageID is set. New public comments are pending until approved.
type Comment struct {
	orm.Model
	SiteID      uint     `json:"site_id"`
	PostID      *uint    `json:"post_id"`
	PageID      *uint    `json:"page_id"`
	ParentID    *uint    `json:"parent_id"`
	UserID      *string  `json:"user_id"`
	AuthorName  string   `json:"author_name"`
	AuthorEmail *string  `json:"author_email"`
	Content     string   `json:"content"`
	Rating      *uint8   `json:"rating"`
	Status      string   `json:"status"`
	IPAddress   *string  `json:"ip_address"`
	UserAgent   *string  `json:"user_agent"`
	Post        *Post    `gorm:"foreignKey:PostID" json:"post,omitempty"`
	Page        *Page    `gorm:"foreignKey:PageID" json:"page,omitempty"`
	Parent      *Comment `gorm:"foreignKey:ParentID" json:"parent,omitempty"`
}

func (Comment) TableName() string { return CommentsTable }

// Media is a file of a site's media library. Images get smaller copies for srcset
// (Conversions: width → path), made when the file is saved.
type Media struct {
	orm.Model
	SiteID      uint                        `json:"site_id"`
	Path        string                      `json:"path"`
	FileName    string                      `json:"file_name"`
	MimeType    string                      `json:"mime_type"`
	Size        int64                       `json:"size"`
	Width       *int                        `json:"width"`
	Height      *int                        `json:"height"`
	Alt         *string                     `json:"alt"`
	Title       *string                     `json:"title"`
	Caption     *string                     `json:"caption"`
	UploadedBy  *string                     `json:"uploaded_by"`
	Conversions mks.JSON[map[string]string] `json:"conversions"`
}

func (Media) TableName() string { return MediaTable }
