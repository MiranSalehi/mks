// Package cms is the panel's content management plugin, a native port of the MKSine CMS:
// pages (simple and block-based) with revisions, trash and preview, site menus, and a
// public site renderer. Its frontend half is @mksine/mks/cms.
//
// A site is a panel tenant: every CMS table has a site_id column holding the tenant key,
// and the panel scopes every query to the current tenant. Tenancy is therefore required.
//
//	p.Use(&cms.Plugin{Orm: facades.Orm(), Cache: facades.Cache(), Sites: sites, Can: can})
//	cmsPlugin.SiteRoutes(facades.Route()) // the public pages: /sites/{site}/{slug}
//
// The tables come from a migration: cms.CreateTables(facades.Schema(), "teams").
package cms

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/goravel/framework/contracts/cache"
	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/contracts/filesystem"

	mks "github.com/MiranSalehi/mks"
)

// Ability is something a user may do in the CMS; Plugin.Can decides.
type Ability string

const (
	// ManagePages: create, edit, duplicate, preview, trash and restore pages, and restore
	// revisions.
	ManagePages Ability = "cms.pages.manage"
	// PurgePages: delete trashed pages for good, with their history.
	PurgePages Ability = "cms.pages.purge"
	// ManageMenus: create, edit and delete menus.
	ManageMenus Ability = "cms.menus.manage"
)

// Site is the public face of a tenant: its key, its address segment and its name.
type Site struct {
	ID   any
	Slug string
	Name string
}

// Sites finds sites. Both return nil, nil when there is no such site.
type Sites interface {
	BySlug(slug string) (*Site, error)
	ByID(id any) (*Site, error)
}

// Plugin is the CMS. Orm, Cache and Sites are required.
type Plugin struct {
	Orm orm.Orm
	// Cache keeps preview grants for a few minutes.
	Cache cache.Cache
	Sites Sites
	// Can decides who may do what. Reading is open to every panel user. Nil asks the
	// panel's gate (Panel.Allowed, e.g. roles from the access plugin), where the
	// abilities are registered as custom permissions; without a gate everything is allowed.
	Can func(ctx *mks.HookContext, ability Ability) bool
	// Locations are where menus can be placed. Default: DefaultLocations (header, footer).
	Locations []Location
	// Permalinks are the public URL patterns; empty fields keep DefaultPermalinks.
	Permalinks *Permalinks
	// CommentsOnPages lets readers comment on pages too (posts always take comments).
	CommentsOnPages bool
	// CommentsPerMinute is how many comments one IP address may send a minute. Default: 5.
	CommentsPerMinute int
	// Storage holds the media library's files (facades.Storage()); needed for media.
	Storage filesystem.Storage
	// Disk is the storage disk of media. Default: "public".
	Disk string
	// MediaMaxSize caps one media file, in bytes. Default: 20 MB.
	MediaMaxSize int64
	// WebP makes the smaller copies of uploaded images WebP (photo-640w.webp, plus one at the
	// image's own width), served with a JPEG/PNG fallback; WebP uploads get copies too. Nil:
	// copies keep the original's format. CWebP("") uses the cwebp tool when installed.
	WebP WebPEncoder
	// WebPQuality is the quality of WebP copies, 30–100. Default: 80.
	WebPQuality int
	// FileURL turns a stored upload path into its public address. Default: /storage/{path}.
	FileURL func(path string) string
	// Authors gives the display names of author keys, for posts and author pages. Optional.
	Authors func(keys []string) (map[string]string, error)
	// AuthorAllowed says whether the user with key may be credited with posts of the site
	// (e.g. a member of the team). Nil: any key; users may always credit themselves.
	AuthorAllowed func(siteID any, key string) bool
	// BaseURL is the public origin ("https://example.com") for absolute URLs in share tags,
	// the sitemap and robots.txt. Empty: the request's host (set it in production, where
	// the Host header comes from the client).
	BaseURL string
	// CustomJS lets holders of cms.themes.code add JavaScript to every page of a site. Off
	// by default: the script runs on the site's origin, and when that is the panel's origin
	// (the default PublicPath) it can call the panel API as any panel user who opens the
	// site, whatever their tenant. Turn it on only when sites are served from their own
	// domains, or when only full administrators hold cms.themes.code.
	CustomJS bool
	// PublicPath is where SiteRoutes mounts the public site. Default: "/sites".
	PublicPath string
	// Log receives errors of the public site (e.g. facades.Log().Error). Optional.
	Log func(message string)
	// Now is the clock (tests). Default: time.Now.
	Now func() time.Time

	// ImportExport adds the content import/export page (MKSine's mks-import-export plugin).
	ImportExport bool
	// HooksPage adds the page that turns listeners, filters and template actions off or
	// reorders them (hooks are honored either way).
	HooksPage bool

	// ContentTypes are custom content types (MKSine's Entries); each gets a resource
	// "entries_{key}" and, when public, site pages.
	ContentTypes []ContentType
	// Themes are extra themes (DefaultTheme and MagazineTheme are built in); one with a
	// built-in ID replaces it.
	Themes []Theme

	// PackagesDisk keeps the files of uploaded themes and plugins (not public). Default: "local".
	PackagesDisk string

	panel           *mks.Panel
	pkg             *packageState
	pkgOnce         sync.Once
	pkgShortcodes   map[string]bool
	shortcodeInfo   map[string]ShortcodeInfo // labels for the editor's list
	extraSettings   map[string]SettingRule
	onSettingsSaved func(site *Site)
	hooks           hookRegistry
	hookCache       hookSettingsCache
	watched         map[string]bool
	shortcodes      map[string]ShortcodeFunc
}

func (*Plugin) Name() string { return "cms" }

// Register adds the CMS resources to the panel.
func (pl *Plugin) Register(p *mks.Panel) {
	if pl.Orm == nil || pl.Cache == nil || pl.Sites == nil {
		panic("cms: Orm, Cache and Sites are required")
	}
	pl.panel = p
	p.Permission(string(ManagePages), string(PurgePages), string(ManageMenus), string(ManagePosts), string(ManageTaxonomies), string(ModerateComments), string(ManageMedia), string(ManageSettings), string(ManageCode), string(ManageThemes), string(InstallThemes), string(ManagePlugins), string(ImportContent), string(ManageHooks))
	pl.coreShortcodes()
	p.Register(
		pl.pagesResource(),
		pl.pageRevisionsResource(),
		pl.pageTrashResource(),
		pl.postsResource(),
		pl.categoriesResource(),
		pl.tagsResource(),
		pl.commentsResource(),
		pl.mediaResource(),
		pl.sectionsResource(),
		pl.menusResource(),
		pl.menuLocationsResource(),
	)
	pl.watched = map[string]bool{}
	for name := range cmsResources {
		pl.watched[name] = true
	}
	for _, ct := range pl.codeContentTypes() {
		p.Register(pl.entryResource(ct))
		pl.watched[entriesResourceName(ct.Key)] = true
	}
	// Content types of the active ZIP plugins come and go as panel resources; load them
	// once at start so the first menu has them.
	go pl.loadedPackages()
	// Settings as MKSine splits them (general, permalinks, performance) and the themes page.
	p.Pages(pl.settingsPage("site_settings"), pl.settingsPage("site_permalinks"), pl.settingsPage("site_performance"), pl.settingsPage("site_extensions"), pl.themesPage(), pl.pluginsPage(),
		// MKSine's menu builder: the items of a menu, opened from the menus list.
		mks.Page{Name: "menu_builder", Authorize: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManageMenus) }})
	// The blocks of the active ZIP plugins, for the builder form.
	p.MetaValue("cms_blocks", func(*mks.HookContext) any { return pl.packageBlockDefs() })
	// The shortcodes for the editor's "Insert shortcode" button.
	p.MetaValue("cms_shortcodes", func(*mks.HookContext) any { return pl.ShortcodeCatalog() })
	// The public address of the current site, for "View site".
	p.MetaValue("cms_site_url", func(hc *mks.HookContext) any {
		if hc == nil || hc.TenantID() == nil {
			return nil
		}
		site, err := pl.Sites.ByID(hc.TenantID())
		if err != nil || site == nil {
			return nil
		}
		return pl.SiteURL(site.Slug)
	})
	// The content types of the active ZIP plugins, for their list and form screens.
	p.MetaValue("cms_content_types", func(*mks.HookContext) any {
		out := []map[string]any{}
		for _, t := range pl.packageContentTypes() {
			out = append(out, map[string]any{"key": t.Key, "singular": t.Singular, "plural": t.Plural})
		}
		return out
	})
	// Whether Settings › Extensions has anything (MKSine hides it otherwise).
	p.MetaValue("cms_extension_settings", func(*mks.HookContext) any {
		for _, lp := range pl.activePlugins() {
			if len(lp.m.Settings) > 0 {
				return true
			}
		}
		return len(pl.extraSettings) > 0
	})
	if pl.ImportExport {
		p.Pages(pl.transferPage())
	}
	if pl.HooksPage {
		p.Pages(pl.hooksPage())
	}
	pl.watchChanges(p)
}

func (pl *Plugin) can(ctx *mks.HookContext, ability Ability) bool {
	if ctx == nil || ctx.User == nil {
		return false
	}
	if pl.Can != nil {
		return pl.Can(ctx, ability)
	}
	return pl.panel != nil && pl.panel.Allowed(ctx, string(ability))
}

func (pl *Plugin) now() time.Time {
	if pl.Now != nil {
		return pl.Now()
	}
	return time.Now()
}

func (pl *Plugin) publicPath() string {
	if pl.PublicPath != "" {
		return pl.PublicPath
	}
	return "/sites"
}

func (pl *Plugin) query() orm.Query { return pl.Orm.Query() }

// CreateTables creates the CMS tables; call it from a migration. siteTable, if set, is
// the tenants table (e.g. "teams"): site_id then references it and rows go with their
// site.
func CreateTables(s schema.Schema, siteTable string) error {
	site := func(t schema.Blueprint) {
		t.UnsignedBigInteger("site_id")
		if siteTable != "" {
			t.Foreign("site_id").References("id").On(siteTable).CascadeOnDelete()
		}
	}
	if err := s.Create(PagesTable, func(t schema.Blueprint) {
		t.ID()
		site(t)
		t.String("title")
		t.String("slug")
		t.Enum("type", []any{"simple", "builder"}).Default("simple")
		t.Enum("status", []any{"draft", "published", "scheduled"}).Default("draft")
		t.LongText("content").Nullable()
		t.Json("builder_payload").Nullable()
		t.Boolean("show_page_header").Default(true)
		t.String("builder_content_width").Default("contained")
		t.String("meta_title").Nullable()
		t.Text("meta_description").Nullable()
		t.String("focus_keyphrase").Nullable()
		t.DateTime("published_at").Nullable()
		t.UnsignedBigInteger("created_by").Nullable()
		t.UnsignedBigInteger("updated_by").Nullable()
		t.DateTime("deleted_at").Nullable()
		t.Timestamps()
		t.Unique("site_id", "slug")
		t.Index("site_id", "status", "published_at")
	}); err != nil {
		return err
	}
	if err := s.Create(PageRevisionsTable, func(t schema.Blueprint) {
		t.ID()
		site(t)
		t.UnsignedBigInteger("page_id")
		t.UnsignedBigInteger("user_id").Nullable()
		t.Json("snapshot")
		t.Timestamps()
		t.Index("site_id", "page_id", "created_at")
		t.Foreign("page_id").References("id").On(PagesTable).CascadeOnDelete()
	}); err != nil {
		return err
	}
	return s.Create(MenusTable, func(t schema.Blueprint) {
		t.ID()
		site(t)
		t.String("name", 100)
		// Dropped again by CreateMenuLocationsTable, which moves it to its own table.
		t.String("location", 50).Nullable()
		t.Json("items").Nullable()
		t.Timestamps()
		t.Unique("site_id", "location")
	})
}

// DropTables removes the CMS tables (a migration's Down).
func DropTables(s schema.Schema) error {
	for _, table := range []string{MenusTable, PageRevisionsTable, PagesTable} {
		if err := s.DropIfExists(table); err != nil {
			return err
		}
	}
	return nil
}

// userID reads a user key as a number, for the created_by style columns.
func userID(raw any) *uint {
	if raw == nil {
		return nil
	}
	id, err := strconv.ParseUint(fmt.Sprint(raw), 10, 64)
	if err != nil || id == 0 {
		return nil
	}
	value := uint(id)
	return &value
}
