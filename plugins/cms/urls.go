package cms

import (
	"net/url"
	"strings"
)

// Permalinks are the public URL patterns of a site, relative to its base
// ({PublicPath}/{site}). {slug} is one segment, {path} is a category's full path and {id}
// an author key. The defaults are MKSine's.
type Permalinks struct {
	Posts      string
	Post       string
	Categories string
	Category   string
	Tags       string
	Tag        string
	Page       string
	Author     string
}

// DefaultPermalinks are MKSine's defaults.
var DefaultPermalinks = Permalinks{
	Posts:      "/posts",
	Post:       "/post/{slug}",
	Categories: "/categories",
	Category:   "/category/{path}",
	Tags:       "/tags",
	Tag:        "/tag/{slug}",
	Page:       "/page/{slug}",
	Author:     "/author/{id}",
}

// permalinksFor are a site's patterns: its settings over Plugin.Permalinks over the defaults.
func (pl *Plugin) permalinksFor(siteSlug string) Permalinks {
	p := pl.permalinks()
	if siteSlug == "" {
		return p
	}
	set := pl.settingsBySlug(siteSlug)
	for _, f := range []struct {
		dst *string
		key string
	}{
		{&p.Posts, "permalink_posts"}, {&p.Post, "permalink_post"}, {&p.Categories, "permalink_categories"},
		{&p.Category, "permalink_category"}, {&p.Tags, "permalink_tags"}, {&p.Tag, "permalink_tag"},
		{&p.Page, "permalink_page"}, {&p.Author, "permalink_author"},
	} {
		if v := cleanPattern(set[f.key]); v != "" {
			*f.dst = v
		}
	}
	return p
}

func (pl *Plugin) permalinks() Permalinks {
	p := DefaultPermalinks
	if pl.Permalinks != nil {
		c := *pl.Permalinks
		for _, f := range []struct {
			dst *string
			v   string
		}{
			{&p.Posts, c.Posts}, {&p.Post, c.Post}, {&p.Categories, c.Categories}, {&p.Category, c.Category},
			{&p.Tags, c.Tags}, {&p.Tag, c.Tag}, {&p.Page, c.Page}, {&p.Author, c.Author},
		} {
			if v := cleanPattern(f.v); v != "" {
				*f.dst = v
			}
		}
	}
	return p
}

// cleanPattern gives a pattern one leading slash and no trailing one.
func cleanPattern(p string) string {
	p = strings.Trim(strings.TrimSpace(p), "/")
	if p == "" {
		return ""
	}
	return "/" + p
}

// SiteURL is the home of a site.
func (pl *Plugin) SiteURL(siteSlug string) string {
	return pl.publicPath() + "/" + url.PathEscape(siteSlug)
}

func (pl *Plugin) fill(siteSlug, pattern, key, value string) string {
	return pl.SiteURL(siteSlug) + strings.Replace(pattern, "{"+key+"}", value, 1)
}

// PageURL is the public address of a page.
func (pl *Plugin) PageURL(siteSlug, pageSlug string) string {
	return pl.fill(siteSlug, pl.permalinksFor(siteSlug).Page, "slug", url.PathEscape(pageSlug))
}

// PostURL is the public address of a post.
func (pl *Plugin) PostURL(siteSlug, postSlug string) string {
	return pl.fill(siteSlug, pl.permalinksFor(siteSlug).Post, "slug", url.PathEscape(postSlug))
}

// CategoryURL is the public address of a category, by its full path ("news/sport").
func (pl *Plugin) CategoryURL(siteSlug, path string) string {
	parts := strings.Split(path, "/")
	for i := range parts {
		parts[i] = url.PathEscape(parts[i])
	}
	return pl.fill(siteSlug, pl.permalinksFor(siteSlug).Category, "path", strings.Join(parts, "/"))
}

// TagURL is the public address of a tag.
func (pl *Plugin) TagURL(siteSlug, tagSlug string) string {
	return pl.fill(siteSlug, pl.permalinksFor(siteSlug).Tag, "slug", url.PathEscape(tagSlug))
}

// AuthorURL is the public list of an author's posts.
func (pl *Plugin) AuthorURL(siteSlug, author string) string {
	return pl.fill(siteSlug, pl.permalinksFor(siteSlug).Author, "id", url.PathEscape(author))
}

func (pl *Plugin) listURL(siteSlug, pattern string) string { return pl.SiteURL(siteSlug) + pattern }

// route is what a public path inside a site points at.
type routeKind int

const (
	routeNone routeKind = iota
	routeHome
	routePosts
	routePost
	routeCategories
	routeCategory
	routeTags
	routeTag
	routePage
	routeAuthor
)

// matchRoute finds the view for a path inside a site ("/post/hello"), in MKSine's order:
// author, categories, category, tags, tag, posts, post, page.
func (pl *Plugin) matchRoute(siteSlug, path string) (routeKind, string) {
	path = "/" + strings.Trim(path, "/")
	if path == "/" {
		return routeHome, ""
	}
	p := pl.permalinksFor(siteSlug)
	for _, r := range []struct {
		kind    routeKind
		pattern string
	}{
		{routeAuthor, p.Author}, {routeCategories, p.Categories}, {routeCategory, p.Category},
		{routeTags, p.Tags}, {routeTag, p.Tag}, {routePosts, p.Posts}, {routePost, p.Post}, {routePage, p.Page},
	} {
		if value, ok := matchPattern(r.pattern, path); ok {
			return r.kind, value
		}
	}
	return routeNone, ""
}

// matchPattern matches a path against a pattern with at most one placeholder: {slug} and
// {id} take one segment, {path} one or more.
func matchPattern(pattern, path string) (string, bool) {
	open := strings.Index(pattern, "{")
	if open < 0 {
		return "", path == pattern
	}
	end := strings.Index(pattern, "}")
	if end < open {
		return "", false
	}
	name, prefix, suffix := pattern[open+1:end], pattern[:open], pattern[end+1:]
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) || len(path) < len(prefix)+len(suffix) {
		return "", false
	}
	value := path[len(prefix) : len(path)-len(suffix)]
	if name != "path" && strings.Contains(value, "/") {
		return "", false
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return "", false
		}
	}
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return "", false
	}
	return decoded, true
}
