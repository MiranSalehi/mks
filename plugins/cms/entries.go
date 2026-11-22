package cms

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/contracts/http"
	frameworkorm "github.com/goravel/framework/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// Custom content types (MKSine's Entries): types registered in code (projects, events,
// recipes, ...) whose entries live in one table. Each type gets its own panel resource
// ("entries_{key}") and, when public, an archive and a single page on the site.

// EntriesTable holds the entries of every content type.
const EntriesTable = "cms_entries"

// ContentType describes a custom content type.
type ContentType struct {
	// Key names the type: lowercase letters, digits, "-" and "_", not a reserved word.
	Key      string
	Singular string
	Plural   string
	// Public types are shown on the site; HasArchive adds their list page.
	Public     bool
	HasArchive bool
	// ArchiveURI and SingleURI are the site paths, defaults "/{key}" and
	// "/{key}/{slug}".
	ArchiveURI string
	SingleURI  string
}

// Entry is one item of a custom content type.
type Entry struct {
	frameworkorm.Model
	SiteID          uint       `json:"site_id"`
	ContentType     string     `json:"content_type"`
	Title           string     `json:"title"`
	Slug            string     `json:"slug"`
	Excerpt         *string    `json:"excerpt"`
	Content         *string    `json:"content"`
	Status          string     `json:"status"`
	FeaturedImage   *string    `json:"featured_image"`
	AuthorID        *string    `json:"author_id"`
	PublishedAt     *time.Time `json:"published_at"`
	MetaTitle       *string    `json:"meta_title"`
	MetaDescription *string    `json:"meta_description"`
	FocusKeyphrase  *string    `json:"focus_keyphrase"`
}

func (Entry) TableName() string { return EntriesTable }

var (
	contentTypeKey       = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)
	reservedContentTypes = map[string]bool{
		"post": true, "page": true, "posts": true, "pages": true, "category": true, "categories": true,
		"tag": true, "tags": true, "author": true, "authors": true, "user": true, "users": true,
		"media": true, "comment": true, "comments": true, "menu": true, "menus": true, "home": true,
		"admin": true, "api": true, "storage": true, "login": true, "register": true, "entry": true,
		"entries": true, "sitemap.xml": true, "robots.txt": true,
	}
)

// CreateEntriesTable creates the entries table; call it from a migration.
func CreateEntriesTable(s schema.Schema, siteTable string) error {
	return s.Create(EntriesTable, func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("site_id")
		if siteTable != "" {
			t.Foreign("site_id").References("id").On(siteTable).CascadeOnDelete()
		}
		t.String("content_type", 64)
		t.String("title")
		t.String("slug")
		t.Text("excerpt").Nullable()
		t.LongText("content").Nullable()
		t.Enum("status", []any{"draft", "published", "archived"}).Default("draft")
		t.String("featured_image").Nullable()
		t.String("author_id", 64).Nullable()
		t.DateTime("published_at").Nullable()
		t.String("meta_title").Nullable()
		t.Text("meta_description").Nullable()
		t.String("focus_keyphrase", 191).Nullable()
		t.Timestamps()
		t.Unique("site_id", "content_type", "slug")
		t.Index("site_id", "content_type", "status", "published_at")
	})
}

// DropEntriesTable removes the entries table.
func DropEntriesTable(s schema.Schema) error { return s.DropIfExists(EntriesTable) }

// codeContentTypes are Plugin.ContentTypes, checked and with their default addresses.
func (pl *Plugin) codeContentTypes() []ContentType {
	out := make([]ContentType, 0, len(pl.ContentTypes))
	for _, t := range pl.ContentTypes {
		if !contentTypeKey.MatchString(t.Key) || reservedContentTypes[t.Key] {
			panic(fmt.Sprintf("cms: invalid or reserved content type key %q", t.Key))
		}
		out = append(out, withTypeDefaults(t))
	}
	return out
}

func withTypeDefaults(t ContentType) ContentType {
	if t.ArchiveURI == "" {
		t.ArchiveURI = "/" + t.Key
	}
	if t.SingleURI == "" {
		t.SingleURI = strings.TrimRight(t.ArchiveURI, "/") + "/{slug}"
	}
	t.ArchiveURI, t.SingleURI = cleanPattern(t.ArchiveURI), cleanPattern(t.SingleURI)
	return t
}

// contentTypes are the types in code and those of the active ZIP plugins (a key in code,
// or one an earlier plugin took, is not taken again).
func (pl *Plugin) contentTypes() []ContentType {
	out := pl.codeContentTypes()
	for _, t := range pl.packageContentTypes() {
		out = append(out, t)
	}
	return out
}

// packageContentTypes are the active plugins' content types that do not clash.
func (pl *Plugin) packageContentTypes() []ContentType {
	taken := map[string]bool{}
	for _, t := range pl.codeContentTypes() {
		taken[t.Key] = true
	}
	var out []ContentType
	for _, lp := range pl.activePlugins() {
		for _, t := range lp.contentTypes {
			if taken[t.Key] {
				continue
			}
			taken[t.Key] = true
			out = append(out, t)
		}
	}
	return out
}

// entriesResourceName is the panel resource of a content type.
func entriesResourceName(key string) string { return "entries_" + strings.ReplaceAll(key, "-", "_") }

// entryResource is the panel resource of one content type.
func (pl *Plugin) entryResource(ct ContentType) mks.ModelResource[Entry] {
	manage := func(ctx *mks.HookContext, _ *Entry) bool { return pl.can(ctx, ManagePosts) }
	key := ct.Key
	return mks.Define[Entry](mks.Config{
		Name:          "entries_" + strings.ReplaceAll(key, "-", "_"),
		Tenant:        "site_id",
		Searchable:    []string{"title", "slug", "excerpt"},
		Sortable:      []string{"id", "title", "status", "published_at", "created_at", "updated_at"},
		Filterable:    map[string][]string{"status": {"eq", "in"}, "published_at": {"gte", "lt", "null"}},
		Fillable:      []string{"title", "slug", "excerpt", "content", "status", "featured_image", "published_at", "meta_title", "meta_description", "focus_keyphrase"},
		TitleColumn:   "title",
		GlobalSearch:  true,
		SearchDetails: []string{"status"},
		HTML:          []string{"content"},
		DefaultSort:   "-created_at",
		Deletable:     true,
		Rules: withSEO(map[string]string{
			"title":        "required|max_len:255",
			"slug":         "required|max_len:255|regex:^[a-z0-9-]+$",
			"excerpt":      "max_len:500",
			"status":       "in:draft,published,archived",
			"published_at": "date",
		}),
	}).
		BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
			data["content_type"] = key
			data["author_id"] = fmt.Sprint(ctx.UserID())
			if _, sent := data["status"]; !sent {
				data["status"] = "draft"
			}
			return pl.checkEntry(data, nil, key)
		}).
		BeforeUpdate(func(_ *mks.HookContext, e *Entry, data map[string]any) error {
			return pl.checkEntry(data, e, key)
		}).
		Policy(mks.Policy[Entry]{
			Create: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManagePosts) },
			Update: manage,
			Delete: manage,
			// One resource per type: the others' entries do not exist here.
			Scope: func(_ *mks.HookContext, q orm.Query) orm.Query { return q.Where("content_type = ?", key) },
		})
}

func (pl *Plugin) checkEntry(data map[string]any, stored *Entry, key string) error {
	siteID := data["site_id"]
	var id uint
	if stored != nil {
		siteID, id = stored.SiteID, stored.ID
	}
	if slug, ok := data["slug"].(string); ok && slug != "" {
		q := pl.query().Model(&Entry{}).Where("site_id = ? AND content_type = ? AND slug = ?", siteID, key, slug)
		if id != 0 {
			q = q.Where("id <> ?", id)
		}
		n, err := q.Count()
		if err != nil {
			return err
		}
		if n > 0 {
			return mks.ValidationErrors{"slug": "این نامک در این نوع محتوا قبلاً استفاده شده است."}
		}
	}
	if err := pl.checkMediaPath(siteID, data, "featured_image"); err != nil {
		return err
	}
	status, _ := data["status"].(string)
	if status == "" && stored != nil {
		status = stored.Status
	}
	if status == "published" {
		if _, sent := data["published_at"]; !sent && (stored == nil || stored.PublishedAt == nil) {
			data["published_at"] = time.Now().Truncate(time.Second)
		}
	}
	return nil
}

// matchEntryRoute finds an entry archive or single page among the public types.
func (pl *Plugin) matchEntryRoute(path string) (ContentType, string, bool, bool) {
	path = "/" + strings.Trim(path, "/")
	for _, ct := range pl.contentTypes() {
		if !ct.Public {
			continue
		}
		if ct.HasArchive && path == ct.ArchiveURI {
			return ct, "", true, true
		}
		if slug, ok := matchPattern(ct.SingleURI, path); ok {
			return ct, slug, false, true
		}
	}
	return ContentType{}, "", false, false
}

func (pl *Plugin) entryURL(siteSlug string, ct ContentType, slug string) string {
	return pl.SiteURL(siteSlug) + strings.Replace(ct.SingleURI, "{slug}", slug, 1)
}

func (pl *Plugin) publishedEntries(req *request, key string) orm.Query {
	return pl.query().Model(&Entry{}).
		Where("site_id = ? AND content_type = ? AND status = ?", req.site.ID, key, "published").
		Where("(published_at IS NULL OR published_at <= ?)", req.now)
}

func (pl *Plugin) showEntryArchive(req *request, ct ContentType) (http.Response, error) {
	var rows []Entry
	var total int64
	if err := pl.publishedEntries(req, ct.Key).OrderByDesc("published_at").OrderByDesc("id").Paginate(req.page, PerPage, &rows, &total); err != nil {
		return nil, err
	}
	if req.page > 1 && len(rows) == 0 {
		return pl.notFoundFor(req), nil
	}
	paths := make([]*string, len(rows))
	for i := range rows {
		paths[i] = rows[i].FeaturedImage
	}
	images, err := pl.mediaImages(req, paths...)
	if err != nil {
		return nil, err
	}
	items := make([]cardView, 0, len(rows))
	for i := range rows {
		e := &rows[i]
		date, iso := formatDate(e.PublishedAt, req.calendar())
		items = append(items, cardView{Title: e.Title, URL: safeURL(pl.entryURL(req.site.Slug, ct, e.Slug)),
			Excerpt: description(nil, e.Excerpt, e.Content), Date: date, DateISO: iso, Image: images.get(e.FeaturedImage, "")})
	}
	base := pl.SiteURL(req.site.Slug) + ct.ArchiveURI
	return pl.html(req, layoutView{Title: documentTitle(ct.Plural, nil, req.site), Canonical: safeURL(base),
		List: &listView{Heading: ct.Plural, Items: items, Pagination: pagination(base, req.page, total), Empty: "هنوز موردی منتشر نشده است."}}, false)
}

func (pl *Plugin) showEntry(req *request, ct ContentType, slug string) (http.Response, error) {
	var e Entry
	found, err := first(pl.publishedEntries(req, ct.Key).Where("slug = ?", slug), &e)
	if err != nil {
		return nil, err
	}
	if !found || e.ID == 0 {
		return pl.notFoundFor(req), nil
	}
	req.record = &e
	if !pl.visible(req, &e) {
		return pl.notFoundFor(req), nil
	}
	images, err := pl.mediaImages(req, e.FeaturedImage)
	if err != nil {
		return nil, err
	}
	date, iso := formatDate(e.PublishedAt, req.calendar())
	pv := &postView{Title: e.Title, Date: date, DateISO: iso, Image: images.get(e.FeaturedImage, e.Title)}
	if e.Content != nil {
		pv.Content = template.HTML(pl.responsiveContent(req, pl.renderShortcodes(mks.SanitizeHTML(*e.Content), req.shortcodeCtx()))) //nolint:gosec -- sanitized; shortcodes are code
	}
	if ct.HasArchive {
		pv.Categories = []linkView{{Label: ct.Plural, URL: safeURL(pl.SiteURL(req.site.Slug) + ct.ArchiveURI)}}
	}
	return pl.html(req, layoutView{
		Title:       documentTitle(e.Title, e.MetaTitle, req.site),
		Description: description(e.MetaDescription, e.Excerpt, e.Content),
		Canonical:   safeURL(pl.entryURL(req.site.Slug, ct, e.Slug)),
		Post:        pv,
	}, false)
}
