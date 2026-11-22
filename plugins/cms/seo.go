package cms

import (
	"encoding/json"
	"encoding/xml"
	"html/template"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/support/carbon"
)

// SEO beyond MKSine (which outputs only <title> and the meta description): canonical,
// Open Graph and Twitter tags, JSON-LD for posts, and a sitemap.xml and robots.txt per
// site.

// origin is the scheme and host for absolute URLs: Plugin.BaseURL, else the request's.
// Set BaseURL in production: the Host header comes from the client.
func (pl *Plugin) origin(ctx http.Context) string {
	if pl.BaseURL != "" {
		return strings.TrimRight(pl.BaseURL, "/")
	}
	if ctx == nil {
		return ""
	}
	scheme := "http"
	if strings.EqualFold(ctx.Request().Header("X-Forwarded-Proto", ""), "https") {
		scheme = "https"
	}
	host := ctx.Request().Host()
	if host == "" || strings.ContainsAny(host, "/\\ @") {
		return ""
	}
	return scheme + "://" + host
}

// absolute turns a site path into a full URL ("" when there is no origin).
func absolute(origin, path string) string {
	if origin == "" || path == "" {
		return ""
	}
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	return origin + path
}

// socialView are the share tags of a page.
type socialView struct {
	Type        string
	URL         string
	Image       string
	SiteName    string
	Published   string
	JSONLD      template.JS
	Description string
}

// social fills the share tags from the finished view.
func (pl *Plugin) social(req *request, view *layoutView) {
	origin := pl.origin(req.ctx)
	s := &socialView{Type: "website", URL: absolute(origin, string(view.Canonical)), SiteName: req.site.Name, Description: view.Description}
	if view.Post != nil {
		s.Type = "article"
		s.Image = absolute(origin, string(view.Post.Image.Src))
		s.Published = view.Post.DateISO
		ld := map[string]any{
			"@context":      "https://schema.org",
			"@type":         "BlogPosting",
			"headline":      view.Post.Title,
			"datePublished": view.Post.DateISO,
			"url":           s.URL,
			"publisher":     map[string]any{"@type": "Organization", "name": req.site.Name},
		}
		if s.Image != "" {
			ld["image"] = s.Image
		}
		if view.Post.Author != "" {
			ld["author"] = map[string]any{"@type": "Person", "name": view.Post.Author}
		}
		if view.Description != "" {
			ld["description"] = view.Description
		}
		if raw, err := json.Marshal(ld); err == nil {
			// html/template's JS context escapes it safely inside <script>.
			s.JSONLD = template.JS(raw) //nolint:gosec -- JSON from our own map
		}
	}
	if s.Image == "" && view.Logo != "" {
		s.Image = absolute(origin, string(view.Logo))
	}
	view.Social = s
}

// --- sitemap.xml and robots.txt ------------------------------------------------------------

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapSet struct {
	XMLName xml.Name     `xml:"urlset"`
	NS      string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// showSitemap lists the public addresses of a site: home, pages, posts, active categories
// and tags (at most 50,000, the sitemap limit).
func (pl *Plugin) showSitemap(req *request) (http.Response, error) {
	origin := pl.origin(req.ctx)
	if origin == "" {
		return plain(req.ctx, 400, "Bad request"), nil
	}
	set := sitemapSet{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	add := func(path string, mod *time.Time) {
		if len(set.URLs) >= 50000 {
			return
		}
		u := sitemapURL{Loc: absolute(origin, path)}
		if mod != nil && !mod.IsZero() {
			u.LastMod = mod.UTC().Format("2006-01-02")
		}
		set.URLs = append(set.URLs, u)
	}
	slug := req.site.Slug
	add(pl.SiteURL(slug), nil)

	var pages []Page
	if err := pl.query().Model(&Page{}).Select("id", "slug", "status", "published_at", "updated_at").
		Where("site_id = ? AND deleted_at IS NULL", req.site.ID).Find(&pages); err != nil {
		return nil, err
	}
	for _, p := range pages {
		if publicPageAvailable(p.Status, p.PublishedAt, req.now) {
			add(pl.PageURL(slug, p.Slug), stdTime(p.UpdatedAt))
		}
	}
	var posts []Post
	if err := pl.publishedPosts(req).Select("id", "slug", "updated_at").OrderByDesc("published_at").Limit(45000).Find(&posts); err != nil {
		return nil, err
	}
	for _, p := range posts {
		add(pl.PostURL(slug, p.Slug), stdTime(p.UpdatedAt))
	}
	var cats []Category
	if err := pl.query().Model(&Category{}).Select("id", "path", "updated_at").Where("site_id = ? AND is_active = ?", req.site.ID, true).Find(&cats); err != nil {
		return nil, err
	}
	for _, c := range cats {
		add(pl.CategoryURL(slug, c.Path), stdTime(c.UpdatedAt))
	}
	var tags []Tag
	if err := pl.query().Model(&Tag{}).Select("id", "slug", "updated_at").Where("site_id = ? AND is_active = ?", req.site.ID, true).Find(&tags); err != nil {
		return nil, err
	}
	for _, t := range tags {
		add(pl.TagURL(slug, t.Slug), stdTime(t.UpdatedAt))
	}
	for _, ct := range pl.contentTypes() {
		if !ct.Public {
			continue
		}
		if ct.HasArchive {
			add(pl.SiteURL(slug)+ct.ArchiveURI, nil)
		}
		var entries []Entry
		if err := pl.publishedEntries(req, ct.Key).Select("id", "slug", "updated_at").Limit(10000).Find(&entries); err != nil {
			return nil, err
		}
		for _, e := range entries {
			add(pl.entryURL(slug, ct, e.Slug), stdTime(e.UpdatedAt))
		}
	}
	body, err := xml.MarshalIndent(set, "", " ")
	if err != nil {
		return nil, err
	}
	h := req.ctx.Response()
	h.Header("Cache-Control", "public, max-age=600")
	return h.Data(200, "application/xml; charset=utf-8", append([]byte(xml.Header), body...)), nil
}

func (pl *Plugin) showRobots(req *request) (http.Response, error) {
	origin := pl.origin(req.ctx)
	lines := []string{"User-agent: *", "Allow: " + pl.SiteURL(req.site.Slug) + "/"}
	if origin != "" {
		lines = append(lines, "Sitemap: "+absolute(origin, pl.SiteURL(req.site.Slug)+"/sitemap.xml"))
	}
	h := req.ctx.Response()
	h.Header("Cache-Control", "public, max-age=3600")
	return h.Data(200, "text/plain; charset=utf-8", []byte(strings.Join(lines, "\n")+"\n")), nil
}

func stdTime(t *carbon.DateTime) *time.Time {
	if t == nil {
		return nil
	}
	v := t.StdTime()
	return &v
}
