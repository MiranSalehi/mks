package cms

import (
	"errors"
	"fmt"
	"html/template"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	frameworkerrors "github.com/goravel/framework/errors"
	sessionmiddleware "github.com/goravel/framework/session/middleware"

	mks "github.com/MiranSalehi/mks"
)

// PerPage is the size of public post lists (as in MKSine).
const PerPage = 12

// siteNav holds the menus of a site, by location key.
type siteNav map[string][]NavItem

// SiteRoutes mounts the public sites: GET {PublicPath}/{site} and everything under it,
// dispatched by the permalinks. The session is started so a page preview link can
// recognise the panel user.
func (pl *Plugin) SiteRoutes(r route.Route) {
	r.Prefix(pl.publicPath()).Middleware(sessionmiddleware.StartSession()).Group(func(public route.Router) {
		public.Get("/{site}", pl.ShowSite)
		public.Post("/{site}/comments", pl.SubmitComment)
		// A catch-all: gin's "*path" segment, read as the "path" parameter.
		public.Get("/{site}/*path", pl.ShowSite)
	})
}

func plain(ctx http.Context, status int, text string) http.Response {
	return ctx.Response().Data(status, "text/plain; charset=utf-8", []byte(text))
}

func notFound(ctx http.Context) http.Response { return plain(ctx, 404, "Page not found") }

// notFoundFor answers 404 for a site address and tells the listeners of
// mksine.storefront.not_found.
func (pl *Plugin) notFoundFor(req *request) http.Response {
	if err := pl.emit(Event{Name: "mksine.storefront.not_found", Site: req.site.ID, Data: map[string]any{"path": req.ctx.Request().Path()}}); err != nil {
		pl.logf("cms: not_found listener: %v", err)
	}
	return notFound(req.ctx)
}

func (pl *Plugin) logf(format string, args ...any) {
	if pl.Log != nil {
		pl.Log(fmt.Sprintf(format, args...))
	}
}

// request is one public request, resolved to its site.
type request struct {
	ctx      http.Context
	site     *Site
	now      time.Time
	page     int
	settings map[string]string
	view     string
	record   any
	// admin is the signed-in panel user of this site (the admin bar; no page cache).
	admin    *mks.HookContext
	cacheKey string
	cacheTTL time.Duration
}

// shortcodeCtx is the shortcode context of this request.
func (r *request) shortcodeCtx() ShortcodeContext {
	if r == nil {
		return ShortcodeContext{Now: time.Now()}
	}
	return ShortcodeContext{Site: r.site, Calendar: r.calendar(), Now: r.now, Record: r.record}
}

func (r *request) calendar() string {
	if r == nil {
		return ""
	}
	return r.settings["date_calendar"]
}

// ShowSite answers every public address of a site.
func (pl *Plugin) ShowSite(ctx http.Context) http.Response {
	site, err := pl.Sites.BySlug(ctx.Request().Route("site"))
	if err != nil {
		pl.logf("cms: load site: %v", err)
		return plain(ctx, 500, "Unable to load page")
	}
	if site == nil {
		return notFound(ctx)
	}
	// Files of uploaded themes and plugins.
	if rest, ok := strings.CutPrefix(strings.TrimPrefix(ctx.Request().Route("path"), "/"), "_packages/"); ok {
		return pl.servePackageFile(ctx, rest)
	}
	req := &request{ctx: ctx, site: site, now: pl.now(), page: 1, settings: pl.settings(site)}
	// The site_name setting renames the site on its pages.
	if name := strings.TrimSpace(req.settings["site_name"]); name != "" {
		named := *site
		named.Name = name
		req.site = &named
	}
	if p, err := strconv.Atoi(ctx.Request().Query("page")); err == nil && p > 1 && p < 100000 {
		req.page = p
	}
	req.admin = pl.adminUser(req)
	if key, ttl := pl.pageCacheKey(req); key != "" {
		if resp := pl.cachedResponse(req, key); resp != nil {
			return resp
		}
		req.cacheKey, req.cacheTTL = key, ttl
	}
	path := strings.Trim(ctx.Request().Route("path"), "/")
	kind, value := pl.matchRoute(site.Slug, path)
	var resp http.Response
	switch {
	case path == "sitemap.xml":
		resp, err = pl.showSitemap(req)
		kind = -1
	case path == "robots.txt":
		resp, err = pl.showRobots(req)
		kind = -1
	}
	// A root page permalink ("/{slug}") also matches content-type archives and entries:
	// those win when no page has the slug.
	if kind == routePage {
		if _, _, _, ok := pl.matchEntryRoute(path); ok {
			n, cerr := pl.query().Model(&Page{}).Where("site_id = ? AND slug = ? AND deleted_at IS NULL", site.ID, value).Count()
			if cerr != nil {
				pl.logf("cms: find page: %v", cerr)
				return plain(ctx, 500, "Unable to render page")
			}
			if n == 0 {
				kind = routeNone
			}
		}
	}
	switch kind {
	case -1:
	case routeHome:
		req.view = "home"
		resp, err = pl.showHome(req)
	case routePage:
		req.view = "page"
		resp, err = pl.showPage(req, value)
	case routePosts:
		req.view = "posts"
		resp, err = pl.showPostList(req, "", "Posts", "", nil, pl.listURL(site.Slug, pl.permalinksFor(site.Slug).Posts))
	case routePost:
		req.view = "single"
		resp, err = pl.showPost(req, value)
	case routeCategories:
		req.view = "categories"
		resp, err = pl.showCategories(req)
	case routeCategory:
		req.view = "category"
		resp, err = pl.showCategory(req, value)
	case routeTags:
		req.view = "tags"
		resp, err = pl.showTags(req)
	case routeTag:
		req.view = "tag"
		resp, err = pl.showTag(req, value)
	case routeAuthor:
		req.view = "author"
		resp, err = pl.showAuthor(req, value)
	default:
		// Custom content types come after the core routes (MKSine skips colliding ones).
		ct, slug, archive, ok := pl.matchEntryRoute(path)
		switch {
		case ok && archive:
			req.view = "entries"
			resp, err = pl.showEntryArchive(req, ct)
		case ok:
			req.view = "entry"
			resp, err = pl.showEntry(req, ct, slug)
		default:
			return pl.notFoundFor(req)
		}
	}
	if err != nil {
		pl.logf("cms: render %s: %v", ctx.Request().Path(), err)
		return plain(ctx, 500, "Unable to render page")
	}
	return resp
}

// html sends a rendered view. Public pages may be cached briefly; previews never.
func (pl *Plugin) html(req *request, view layoutView, private bool) (http.Response, error) {
	nav, err := pl.loadSiteNav(req.site, string(view.Canonical), req.now)
	if err != nil {
		// A page without its menus is better than no page.
		pl.logf("cms: load menus: %v", err)
		nav = siteNav{}
	}
	view.SiteName = req.site.Name
	view.Tagline = req.settings["tagline"]
	if logo := req.settings["logo"]; logo != "" {
		view.Logo = pl.fileURL(&logo)
	}
	if icon := req.settings["favicon"]; icon != "" {
		view.Favicon = pl.fileURL(&icon)
	}
	view.HomeURL = safeURL(pl.SiteURL(req.site.Slug))
	view.HeaderNav, view.FooterNav = navViews(nav["header"]), navViews(nav["footer"])
	view.Nav = allNavViews(nav)
	if view.Title == "" {
		view.Title = req.site.Name
	}
	if view.Description == "" {
		view.Description = req.settings["tagline"]
	}
	pl.social(req, &view)
	view.View = req.view
	view.themeID, view.site = req.settings["theme"], req.site
	if css, msg := customCSS(req.settings["custom_css"]); msg == "" && css != "" {
		view.CustomCSS = template.CSS(css) //nolint:gosec -- checked by customCSS: no markup
	}
	if js, msg := customJS(req.settings["custom_js"]); pl.CustomJS && msg == "" && js != "" {
		view.CustomJS = template.JS(js) //nolint:gosec -- checked by customJS; only ManageCode with Plugin.CustomJS may set it
	}
	view.Hooks = pl.actionsFor(ActionContext{Site: req.site, View: req.view, Record: req.record})
	if req.admin != nil {
		view.AdminBar = pl.adminBar(req, req.admin)
	}
	body, err := pl.renderView(view)
	if err != nil {
		return nil, err
	}
	h := req.ctx.Response()
	h.Header("X-Content-Type-Options", "nosniff")
	if !private && req.admin == nil {
		pl.storePage(req, body)
	}
	switch {
	case private:
		h.Header("Cache-Control", "private, no-store")
		h.Header("Referrer-Policy", "no-referrer")
		h.Header("X-Robots-Tag", "noindex, nofollow")
	case req.admin != nil:
		// The admin bar is per user: never in a shared cache.
		h.Header("Cache-Control", "private, no-store")
		h.Header("Referrer-Policy", "same-origin")
	default:
		h.Header("Referrer-Policy", "same-origin")
		h.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	}
	if err := pl.emit(Event{Name: "mksine.storefront.viewed", Site: req.site.ID, Record: req.record, Data: map[string]any{"path": req.ctx.Request().Path(), "view": req.view}}); err != nil {
		pl.logf("cms: viewed listener: %v", err)
	}
	return h.Data(200, "text/html; charset=utf-8", body), nil
}

// visible asks the mksine.content.visible filters whether the public may see a record.
func (pl *Plugin) visible(req *request, record any) bool {
	v, ok := pl.filter("mksine.content.visible", true, req.ctx, record).(bool)
	return !ok || v
}

// documentTitle: the SEO title, or "Title — Site".
func documentTitle(title string, metaTitle *string, site *Site) string {
	if metaTitle != nil && strings.TrimSpace(*metaTitle) != "" {
		return strings.TrimSpace(*metaTitle)
	}
	if site.Name == "" {
		return title
	}
	return title + " — " + site.Name
}

// description: the meta description, or the excerpt, or the text, cut to 160 characters.
func description(meta, excerpt, content *string) string {
	for _, s := range []*string{meta, excerpt} {
		if s != nil && strings.TrimSpace(*s) != "" {
			return plainText(*s, 160)
		}
	}
	if content != nil {
		return plainText(*content, 160)
	}
	return ""
}

func first[T any](q orm.Query, dest *T) (bool, error) {
	err := q.First(dest)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

// --- pages ---------------------------------------------------------------------------------

// showHome shows the front page (the front_page_id setting, else a page with the slug
// "home") when it is public, else the latest posts.
func (pl *Plugin) showHome(req *request) (http.Response, error) {
	var page Page
	q := pl.query().Model(&Page{}).Where("site_id = ? AND deleted_at IS NULL", req.site.ID)
	if id, err := strconv.ParseUint(req.settings["front_page_id"], 10, 64); err == nil {
		q = q.Where("id = ?", id)
	} else {
		q = q.Where("slug = ?", "home")
	}
	found, err := first(q, &page)
	if err != nil {
		return nil, err
	}
	if found && publicPageAvailable(page.Status, page.PublishedAt, req.now) {
		view, err := pl.pageView(req, &page)
		if err != nil {
			return nil, err
		}
		view.Canonical = safeURL(pl.SiteURL(req.site.Slug))
		return pl.html(req, view, false)
	}
	return pl.showPostList(req, req.settings["tagline"], req.site.Name, "", nil, pl.SiteURL(req.site.Slug))
}

// showPage renders a page. Drafts, trashed pages and pages scheduled for later are not
// found, unless a valid preview token for that page and user comes with the request.
func (pl *Plugin) showPage(req *request, slug string) (http.Response, error) {
	var grant *previewGrant
	if token := req.ctx.Request().Query("preview"); token != "" {
		g, ok, err := pl.checkPreview(req.ctx, token)
		if err != nil {
			return nil, err
		}
		if !ok {
			req.ctx.Response().Header("Cache-Control", "private, no-store")
			return pl.notFoundFor(req), nil
		}
		grant = g
	}
	var page Page
	q := pl.query().Model(&Page{}).Where("site_id = ? AND slug = ? AND deleted_at IS NULL", req.site.ID, slug)
	if grant != nil {
		q = q.Where("id = ? AND site_id = ?", grant.PageID, grant.SiteID)
	}
	found, err := first(q, &page)
	if err != nil {
		return nil, err
	}
	if !found || page.ID == 0 || (grant == nil && !publicPageAvailable(page.Status, page.PublishedAt, req.now)) {
		return pl.notFoundFor(req), nil
	}
	req.record = &page
	if grant == nil && !pl.visible(req, &page) {
		return pl.notFoundFor(req), nil
	}
	view, err := pl.pageView(req, &page)
	if err != nil {
		return nil, err
	}
	return pl.html(req, view, grant != nil)
}

func (pl *Plugin) pageView(req *request, page *Page) (layoutView, error) {
	width := "48rem"
	switch page.BuilderContentWidth {
	case "wide":
		width = "64rem"
	case "full":
		width = "90rem"
	}
	pv := &pageView{Title: page.Title, ShowHeader: page.ShowPageHeader}
	script := false
	if page.Type == "builder" {
		body, needsScript, err := pl.renderBuilder(req, page.BuilderPayload.V)
		if err != nil {
			return layoutView{}, err
		}
		pv.Builder, script = body, needsScript
	} else if page.Content != nil {
		pv.Content = template.HTML(pl.responsiveContent(req, pl.renderShortcodes(mks.SanitizeHTML(*page.Content), req.shortcodeCtx()))) //nolint:gosec -- sanitized; shortcodes are code
	}
	if page.ID != 0 && req != nil {
		gallery, err := pl.galleryViews(req, OwnerPage, page.ID)
		if err != nil {
			return layoutView{}, err
		}
		pv.Gallery = gallery
		tags, err := pl.tagLinks(req, PageTagTable, "page_id", page.ID)
		if err != nil {
			return layoutView{}, err
		}
		pv.Tags = tags
		if pl.CommentsOnPages && req.ctx != nil {
			if pv.Comments, err = pl.commentsFor(req, "page", page.ID, pl.PageURL(req.site.Slug, page.Slug)); err != nil {
				return layoutView{}, err
			}
		}
	}
	return layoutView{
		Title:       documentTitle(page.Title, page.MetaTitle, siteOf(req)),
		Description: description(page.MetaDescription, nil, page.Content),
		Canonical:   safeURL(pl.PageURL(siteOf(req).Slug, page.Slug)),
		Width:       width,
		Page:        pv,
		Script:      script,
	}, nil
}

func siteOf(req *request) *Site {
	if req == nil {
		return &Site{}
	}
	return req.site
}

// --- posts ---------------------------------------------------------------------------------

// publishedPosts are the posts of the site the public may see at now.
func (pl *Plugin) publishedPosts(req *request) orm.Query {
	return pl.query().Model(&Post{}).
		Where(PostsTable+".site_id = ? AND "+PostsTable+".status = ?", req.site.ID, "published").
		Where("("+PostsTable+".published_at IS NULL OR "+PostsTable+".published_at <= ?)", req.now)
}

func (pl *Plugin) showPost(req *request, slug string) (http.Response, error) {
	var post Post
	found, err := first(pl.publishedPosts(req).Where(PostsTable+".slug = ?", slug), &post)
	if err != nil {
		return nil, err
	}
	if !found || post.ID == 0 {
		return pl.notFoundFor(req), nil
	}
	req.record = &post
	if !pl.visible(req, &post) {
		return pl.notFoundFor(req), nil
	}
	date, iso := formatDate(publishedOrCreated(&post), req.calendar())
	images, err := pl.mediaImages(req, post.FeaturedImage)
	if err != nil {
		return nil, err
	}
	pv := &postView{Title: post.Title, Date: date, DateISO: iso, Image: images.get(post.FeaturedImage, post.Title)}
	if post.Content != nil {
		pv.Content = template.HTML(pl.responsiveContent(req, pl.renderShortcodes(mks.SanitizeHTML(*post.Content), req.shortcodeCtx()))) //nolint:gosec -- sanitized; shortcodes are code
	}
	if pv.Gallery, err = pl.galleryViews(req, OwnerPost, post.ID); err != nil {
		return nil, err
	}
	if post.AuthorID != nil && *post.AuthorID != "" {
		names, err := pl.authorNames([]string{*post.AuthorID})
		if err != nil {
			return nil, err
		}
		if name := names[*post.AuthorID]; name != "" {
			pv.Author, pv.AuthorURL = name, safeURL(pl.AuthorURL(req.site.Slug, *post.AuthorID))
		}
	}
	cats, err := pl.postCategories(req, post.ID)
	if err != nil {
		return nil, err
	}
	var catIDs []any
	for _, c := range cats {
		pv.Categories = append(pv.Categories, linkView{Label: c.Name, URL: safeURL(pl.CategoryURL(req.site.Slug, c.Path))})
		catIDs = append(catIDs, c.ID)
	}
	if pv.Tags, err = pl.tagLinks(req, PostTagTable, "post_id", post.ID); err != nil {
		return nil, err
	}
	if pv.Comments, err = pl.commentsFor(req, "post", post.ID, pl.PostURL(req.site.Slug, post.Slug)); err != nil {
		return nil, err
	}
	// Related: three other posts sharing a category, newest first.
	if len(catIDs) > 0 {
		var related []Post
		err := pl.publishedPosts(req).
			Where(PostsTable+".id <> ?", post.ID).
			Where(PostsTable+".id IN (SELECT post_id FROM "+CategoryPostTable+" WHERE category_id IN ?)", catIDs).
			OrderByDesc("published_at").Limit(3).Find(&related)
		if err != nil {
			return nil, err
		}
		if pv.Related, err = pl.cards(req, related); err != nil {
			return nil, err
		}
	}
	return pl.html(req, layoutView{
		Title:       documentTitle(post.Title, post.MetaTitle, req.site),
		Description: description(post.MetaDescription, post.Excerpt, post.Content),
		Canonical:   safeURL(pl.PostURL(req.site.Slug, post.Slug)),
		Post:        pv,
	}, false)
}

func publishedOrCreated(p *Post) *time.Time {
	if p.PublishedAt != nil {
		return p.PublishedAt
	}
	if p.CreatedAt != nil {
		t := p.CreatedAt.StdTime()
		return &t
	}
	return nil
}

func (pl *Plugin) fileURL(path *string) template.URL {
	if path == nil || *path == "" {
		return ""
	}
	if pl.FileURL != nil {
		return safeURL(pl.FileURL(*path))
	}
	parts := strings.Split(strings.TrimLeft(*path, "/"), "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return safeURL("/storage/" + strings.Join(parts, "/"))
}

func (pl *Plugin) authorNames(keys []string) (map[string]string, error) {
	if pl.Authors == nil || len(keys) == 0 {
		return map[string]string{}, nil
	}
	return pl.Authors(keys)
}

func (pl *Plugin) cards(req *request, posts []Post) ([]cardView, error) {
	paths := make([]*string, len(posts))
	for i := range posts {
		paths[i] = posts[i].FeaturedImage
	}
	images, err := pl.mediaImages(req, paths...)
	if err != nil {
		return nil, err
	}
	out := make([]cardView, 0, len(posts))
	for i := range posts {
		p := &posts[i]
		date, iso := formatDate(publishedOrCreated(p), req.calendar())
		out = append(out, cardView{Title: p.Title, URL: safeURL(pl.PostURL(req.site.Slug, p.Slug)),
			Excerpt: description(nil, p.Excerpt, p.Content), Date: date, DateISO: iso, Image: images.get(p.FeaturedImage, "")})
	}
	return out, nil
}

// mediaSet are library entries by path, for one page.
type mediaSet struct {
	pl    *Plugin
	byKey map[string]Media
}

// mediaImages loads the library entries of the given paths in one query.
func (pl *Plugin) mediaImages(req *request, paths ...*string) (mediaSet, error) {
	set := mediaSet{pl: pl, byKey: map[string]Media{}}
	var list []any
	for _, p := range paths {
		if p != nil && *p != "" {
			list = append(list, *p)
		}
	}
	if len(list) == 0 {
		return set, nil
	}
	var rows []Media
	if err := pl.query().Model(&Media{}).Where("site_id = ?", req.site.ID).WhereIn("path", list).Find(&rows); err != nil {
		return set, err
	}
	for _, m := range rows {
		set.byKey[m.Path] = m
	}
	return set, nil
}

// galleryViews are the images of a record's gallery (other media types are left out).
func (pl *Plugin) galleryViews(req *request, ownerType string, ownerID uint) ([]imageView, error) {
	items, err := pl.Attachments(ownerType, ownerID, GalleryCollection)
	if err != nil {
		return nil, err
	}
	set := mediaSet{pl: pl, byKey: map[string]Media{}}
	var out []imageView
	for _, m := range items {
		if m.SiteID != req.site.ID || !strings.HasPrefix(m.MimeType, "image/") {
			continue
		}
		set.byKey[m.Path] = m
		path := m.Path
		alt := ""
		if m.Title != nil {
			alt = *m.Title
		}
		out = append(out, set.get(&path, alt))
	}
	return out, nil
}

// get is the image of a path: its copies and alt text when it is in the library.
// fallbackAlt is used when the library has no alt text.
func (s mediaSet) get(path *string, fallbackAlt string) imageView {
	if path == nil || *path == "" {
		return imageView{}
	}
	img := imageView{Src: s.pl.fileURL(path), Alt: fallbackAlt}
	if m, ok := s.byKey[*path]; ok {
		if m.Alt != nil && strings.TrimSpace(*m.Alt) != "" {
			img.Alt = *m.Alt
		}
		if m.Width != nil && m.Height != nil {
			img.Width, img.Height = *m.Width, *m.Height
		}
		url := func(p string) string { return string(s.pl.fileURL(&p)) }
		webp, same := splitCopies(m.Conversions.V)
		if len(webp) > 0 {
			// WebP copies go in a <source>; the img keeps the original as its fallback.
			img.WebP = srcset(webp, url)
		} else if set := srcset(same, url); set != "" {
			if img.Width > 0 {
				set += ", " + string(img.Src) + " " + strconv.Itoa(img.Width) + "w"
			}
			img.Srcset = set
		}
	}
	return img
}

// showPostList lists published posts, filtered by where (nil for all), PerPage at a time.
func (pl *Plugin) showPostList(req *request, kicker, heading string, desc template.HTML, where func(orm.Query) orm.Query, base string) (http.Response, error) {
	q := pl.publishedPosts(req)
	if where != nil {
		q = where(q)
	}
	var posts []Post
	var total int64
	if err := q.OrderByDesc(PostsTable+".published_at").OrderByDesc(PostsTable+".id").Paginate(req.page, PerPage, &posts, &total); err != nil {
		return nil, err
	}
	if req.page > 1 && len(posts) == 0 {
		return pl.notFoundFor(req), nil
	}
	items, err := pl.cards(req, posts)
	if err != nil {
		return nil, err
	}
	lv := &listView{Kicker: kicker, Heading: heading, Description: desc, Items: items,
		Pagination: pagination(base, req.page, total), Empty: "No post has been published yet."}
	canonical := base
	if req.page > 1 {
		canonical = base + "?page=" + strconv.Itoa(req.page)
	}
	return pl.html(req, layoutView{Title: documentTitle(heading, nil, req.site), Canonical: safeURL(canonical), List: lv}, false)
}

func pagination(base string, current int, total int64) []pageLink {
	pages := int(math.Ceil(float64(total) / PerPage))
	if pages <= 1 {
		return nil
	}
	var out []pageLink
	for i := 1; i <= pages; i++ {
		u := base
		if i > 1 {
			u = base + "?page=" + strconv.Itoa(i)
		}
		out = append(out, pageLink{Label: strconv.Itoa(i), URL: safeURL(u), Current: i == current})
	}
	return out
}

// --- taxonomies ----------------------------------------------------------------------------

func (pl *Plugin) postCategories(req *request, postID uint) ([]Category, error) {
	var cats []Category
	err := pl.query().Model(&Category{}).
		Join("JOIN "+CategoryPostTable+" cp ON cp.category_id = "+CategoriesTable+".id").
		Where("cp.post_id = ? AND "+CategoriesTable+".site_id = ? AND "+CategoriesTable+".is_active = ?", postID, req.site.ID, true).
		Order("cp.sort_order").Find(&cats)
	return cats, err
}

func (pl *Plugin) tagLinks(req *request, pivot, column string, id uint) ([]linkView, error) {
	var tags []Tag
	err := pl.query().Model(&Tag{}).
		Join("JOIN "+pivot+" pt ON pt.tag_id = "+TagsTable+".id").
		Where("pt."+column+" = ? AND "+TagsTable+".site_id = ? AND "+TagsTable+".is_active = ?", id, req.site.ID, true).
		Order(TagsTable + ".name").Find(&tags)
	if err != nil {
		return nil, err
	}
	out := make([]linkView, 0, len(tags))
	for _, t := range tags {
		out = append(out, linkView{Label: t.Name, URL: safeURL(pl.TagURL(req.site.Slug, t.Slug))})
	}
	return out, nil
}

func (pl *Plugin) showCategory(req *request, path string) (http.Response, error) {
	var cat Category
	found, err := first(pl.query().Model(&Category{}).Where("site_id = ? AND path = ? AND is_active = ?", req.site.ID, strings.Trim(path, "/"), true), &cat)
	if err != nil {
		return nil, err
	}
	if !found || cat.ID == 0 {
		return pl.notFoundFor(req), nil
	}
	req.record = &cat
	var desc template.HTML
	if cat.Description != nil {
		desc = template.HTML(mks.SanitizeHTML(*cat.Description)) //nolint:gosec -- sanitized
	}
	resp, err := pl.showPostList(req, "Category", cat.Name, desc, func(q orm.Query) orm.Query {
		return q.Where(PostsTable+".id IN (SELECT post_id FROM "+CategoryPostTable+" WHERE category_id = ?)", cat.ID)
	}, pl.CategoryURL(req.site.Slug, cat.Path))
	return resp, err
}

func (pl *Plugin) showTag(req *request, slug string) (http.Response, error) {
	var tag Tag
	found, err := first(pl.query().Model(&Tag{}).Where("site_id = ? AND slug = ? AND is_active = ?", req.site.ID, slug, true), &tag)
	if err != nil {
		return nil, err
	}
	if !found || tag.ID == 0 {
		return pl.notFoundFor(req), nil
	}
	req.record = &tag
	var desc template.HTML
	if tag.Description != nil {
		desc = template.HTML(mks.SanitizeHTML(*tag.Description)) //nolint:gosec -- sanitized
	}
	return pl.showPostList(req, "Tag", tag.Name, desc, func(q orm.Query) orm.Query {
		return q.Where(PostsTable+".id IN (SELECT post_id FROM "+PostTagTable+" WHERE tag_id = ?)", tag.ID)
	}, pl.TagURL(req.site.Slug, tag.Slug))
}

// showAuthor lists an author's posts. Only authors with a public post on this site have
// a page: otherwise any user's name could be read by walking keys.
func (pl *Plugin) showAuthor(req *request, key string) (http.Response, error) {
	n, err := pl.query().Model(&Post{}).
		Where("site_id = ? AND author_id = ? AND status = ? AND (published_at IS NULL OR published_at <= ?)", req.site.ID, key, "published", req.now).
		Count()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return pl.notFoundFor(req), nil
	}
	names, err := pl.authorNames([]string{key})
	if err != nil {
		return nil, err
	}
	name := names[key]
	if name == "" {
		return pl.notFoundFor(req), nil
	}
	return pl.showPostList(req, "Author", name, "", func(q orm.Query) orm.Query {
		return q.Where(PostsTable+".author_id = ?", key)
	}, pl.AuthorURL(req.site.Slug, key))
}

// showCategories lists the active category tree with counts of published posts.
func (pl *Plugin) showCategories(req *request) (http.Response, error) {
	var cats []Category
	if err := pl.query().Model(&Category{}).Where("site_id = ? AND is_active = ?", req.site.ID, true).
		Order("sort_order").Order("name").Find(&cats); err != nil {
		return nil, err
	}
	counts, err := pl.publishedCounts(req, CategoryPostTable, "category_id")
	if err != nil {
		return nil, err
	}
	children := map[uint][]Category{}
	byID := map[uint]bool{}
	for _, c := range cats {
		byID[c.ID] = true
	}
	var roots []Category
	for _, c := range cats {
		if c.ParentID != nil && byID[*c.ParentID] {
			children[*c.ParentID] = append(children[*c.ParentID], c)
		} else if c.ParentID == nil {
			roots = append(roots, c)
		}
	}
	var items []termView
	var walk func(list []Category, depth int)
	walk = func(list []Category, depth int) {
		for _, c := range list {
			items = append(items, termView{Name: c.Name, URL: safeURL(pl.CategoryURL(req.site.Slug, c.Path)), Count: countLabel(counts[c.ID]), Depth: depth})
			if depth < 16 {
				walk(children[c.ID], depth+1)
			}
		}
	}
	walk(roots, 0)
	return pl.html(req, layoutView{Title: documentTitle("Categories", nil, req.site),
		Canonical: safeURL(pl.listURL(req.site.Slug, pl.permalinksFor(req.site.Slug).Categories)),
		Terms:     &termsView{Heading: "Categories", Items: items}}, false)
}

func (pl *Plugin) showTags(req *request) (http.Response, error) {
	var tags []Tag
	if err := pl.query().Model(&Tag{}).Where("site_id = ? AND is_active = ?", req.site.ID, true).Order("name").Find(&tags); err != nil {
		return nil, err
	}
	counts, err := pl.publishedCounts(req, PostTagTable, "tag_id")
	if err != nil {
		return nil, err
	}
	var items []termView
	for _, t := range tags {
		items = append(items, termView{Name: t.Name, URL: safeURL(pl.TagURL(req.site.Slug, t.Slug)), Count: countLabel(counts[t.ID])})
	}
	return pl.html(req, layoutView{Title: documentTitle("Tags", nil, req.site),
		Canonical: safeURL(pl.listURL(req.site.Slug, pl.permalinksFor(req.site.Slug).Tags)),
		Terms:     &termsView{Heading: "Tags", Items: items}}, false)
}

// publishedCounts counts the public posts of each term through a pivot table.
func (pl *Plugin) publishedCounts(req *request, pivot, column string) (map[uint]int64, error) {
	var rows []struct {
		ID    uint
		Total int64
	}
	err := pl.query().Table(pivot+" x").
		Select("x."+column+" AS id", "COUNT(*) AS total").
		Join("JOIN "+PostsTable+" p ON p.id = x.post_id").
		Where("p.site_id = ? AND p.status = ? AND (p.published_at IS NULL OR p.published_at <= ?)", req.site.ID, "published", req.now).
		Group("x." + column).Scan(&rows)
	out := map[uint]int64{}
	for _, r := range rows {
		out[r.ID] = r.Total
	}
	return out, err
}

func countLabel(n int64) string {
	if n == 0 {
		return ""
	}
	return strconv.FormatInt(n, 10)
}

// --- menus ---------------------------------------------------------------------------------

// loadSiteNav reads the menus of the site's locations and resolves their entries to the
// records that are publicly visible at now. current is the address of the viewed page.
func (pl *Plugin) loadSiteNav(site *Site, current string, now time.Time) (siteNav, error) {
	menus, err := pl.siteMenus(site)
	if err != nil {
		return siteNav{}, err
	}
	var all []MenuItem
	for _, m := range menus {
		all = append(all, m.Items.V...)
	}
	targets, err := pl.navTargets(site, Refs(all), now)
	if err != nil {
		return siteNav{}, err
	}
	nav := siteNav{}
	for key, m := range menus {
		nav[key] = BuildNav(m.Items.V, targets, current)
	}
	return nav, nil
}

func idList(ids []uint) []any {
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

// navTargets loads the public records a menu points at.
func (pl *Plugin) navTargets(site *Site, refs MenuRefs, now time.Time) (NavTargets, error) {
	t := NavTargets{"page": {}, "post": {}, "category": {}, "tag": {}}
	if len(refs.Pages) > 0 {
		var rows []Page
		if err := pl.query().Model(&Page{}).Select("id", "slug", "title", "status", "published_at").
			Where("site_id = ? AND deleted_at IS NULL", site.ID).WhereIn("id", idList(refs.Pages)).Find(&rows); err != nil {
			return nil, err
		}
		for _, p := range rows {
			if publicPageAvailable(p.Status, p.PublishedAt, now) {
				t["page"][p.ID] = NavTarget{URL: pl.PageURL(site.Slug, p.Slug), Title: p.Title}
			}
		}
	}
	if len(refs.Posts) > 0 {
		var rows []Post
		if err := pl.query().Model(&Post{}).Select("id", "slug", "title").
			Where("site_id = ? AND status = ? AND (published_at IS NULL OR published_at <= ?)", site.ID, "published", now).
			WhereIn("id", idList(refs.Posts)).Find(&rows); err != nil {
			return nil, err
		}
		for _, p := range rows {
			t["post"][p.ID] = NavTarget{URL: pl.PostURL(site.Slug, p.Slug), Title: p.Title}
		}
	}
	if len(refs.Categories) > 0 {
		var rows []Category
		if err := pl.query().Model(&Category{}).Select("id", "name", "path").
			Where("site_id = ? AND is_active = ?", site.ID, true).WhereIn("id", idList(refs.Categories)).Find(&rows); err != nil {
			return nil, err
		}
		for _, c := range rows {
			t["category"][c.ID] = NavTarget{URL: pl.CategoryURL(site.Slug, c.Path), Title: c.Name}
		}
	}
	if len(refs.Tags) > 0 {
		var rows []Tag
		if err := pl.query().Model(&Tag{}).Select("id", "name", "slug").
			Where("site_id = ? AND is_active = ?", site.ID, true).WhereIn("id", idList(refs.Tags)).Find(&rows); err != nil {
			return nil, err
		}
		for _, tag := range rows {
			t["tag"][tag.ID] = NavTarget{URL: pl.TagURL(site.Slug, tag.Slug), Title: tag.Name}
		}
	}
	return t, nil
}

// renderPage renders a page without a request (tests and callers outside the router).
func (pl *Plugin) renderPage(page *Page, site *Site, nav siteNav) ([]byte, error) {
	view, err := pl.pageView(nil, page)
	if err != nil {
		return nil, err
	}
	view.Title = documentTitle(page.Title, page.MetaTitle, site)
	view.Canonical = safeURL(pl.PageURL(site.Slug, page.Slug))
	view.SiteName = site.Name
	view.HomeURL = safeURL(pl.SiteURL(site.Slug))
	view.HeaderNav, view.FooterNav = navViews(nav["header"]), navViews(nav["footer"])
	view.Nav = allNavViews(nav)
	return pl.renderView(view)
}
