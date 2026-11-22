package cms

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"

	mks "github.com/MiranSalehi/mks"
)

// The page cache (MKSine's PageCache): whole public pages are kept in the app cache when
// the site's page_cache_enabled setting is on. Only GET requests whose query holds nothing
// but "page" (utm_* and click ids are ignored) are cached; previews never. Every CMS save,
// delete or action bumps the site's cache version, which retires all its pages at once.

var ignoredQueryParams = map[string]bool{"fbclid": true, "gclid": true, "yclid": true, "msclkid": true, "_ga": true, "ref": true}

type cachedPage struct {
	Body    string `json:"body"`
	Type    string `json:"type"`
	Created int64  `json:"created"`
	Record  string `json:"view"`
}

func versionKey(siteID any) string { return "cms:pagever:" + fmt.Sprint(siteID) }

// bump retires every cached page of a site.
func (pl *Plugin) bump(siteID any) {
	if pl.Cache == nil || siteID == nil {
		return
	}
	key := versionKey(siteID)
	pl.Cache.Forever(key, pl.Cache.GetInt(key, 0)+1)
}

// bumpAfterCommit retires the site's pages once the current save commits: a request
// rendering in between would otherwise store the old content under the new version.
func (pl *Plugin) bumpAfterCommit(ctx *mks.HookContext) {
	id := ctx.TenantID()
	ctx.AfterCommit(func() { pl.bump(id) })
}

// pageCacheKey is the cache key of a request, or "" when it must not be cached.
func (pl *Plugin) pageCacheKey(req *request) (string, time.Duration) {
	if pl.Cache == nil || req.settings["page_cache_enabled"] != "1" || req.ctx.Request().Method() != "GET" || req.admin != nil {
		return "", 0
	}
	if !pl.filterBool("mksine.page_cache.should_cache", true, req) {
		return "", 0
	}
	parts := []string{req.ctx.Request().Path()}
	if pl.BaseURL == "" {
		// Absolute URLs in the page come from the Host header: one copy per host, so a
		// request with a forged Host never serves its links to anyone else.
		parts = append(parts, pl.origin(req.ctx))
	}
	query := req.ctx.Request().Origin().URL.Query()
	keys := make([]string, 0, len(query))
	for k := range query {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		switch {
		case k == "page":
			// Only the page number as the site reads it, so ?page=x1, x2… cannot fill the
			// cache with copies of page one.
			if raw := query[k]; len(raw) != 1 || raw[0] != strconv.Itoa(req.page) {
				return "", 0
			}
			if req.page > 1 {
				parts = append(parts, "page="+strconv.Itoa(req.page))
			}
		case strings.HasPrefix(k, "utm_") || ignoredQueryParams[k]:
		default:
			return "", 0 // preview, reply, comment notices, anything else: no cache
		}
	}
	if extra, ok := pl.filter("mksine.page_cache.key_parts", parts, req).([]string); ok {
		parts = extra
	}
	sum := sha1.Sum([]byte(strings.Join(parts, "\x00")))
	key := "cms:page:" + fmt.Sprint(req.site.ID) + ":" + strconv.Itoa(pl.Cache.GetInt(versionKey(req.site.ID), 0)) + ":" + hex.EncodeToString(sum[:])
	ttl := 60 * time.Minute
	if m, err := strconv.Atoi(req.settings["page_cache_ttl_minutes"]); err == nil && m >= 1 {
		ttl = time.Duration(m) * time.Minute
	}
	return key, ttl
}

func (pl *Plugin) filterBool(name string, v bool, args ...any) bool {
	out, ok := pl.filter(name, v, args...).(bool)
	return !ok || out
}

// cachedResponse answers from the cache, or nil.
func (pl *Plugin) cachedResponse(req *request, key string) http.Response {
	raw, ok := pl.Cache.Get(key).(string)
	if !ok || raw == "" {
		return nil
	}
	var page cachedPage
	if json.Unmarshal([]byte(raw), &page) != nil {
		return nil
	}
	h := req.ctx.Response()
	h.Header("X-Cache", "HIT")
	h.Header("Age", strconv.FormatInt(max(0, int64(time.Since(time.Unix(page.Created, 0)).Seconds())), 10))
	h.Header("X-Content-Type-Options", "nosniff")
	h.Header("Referrer-Policy", "same-origin")
	h.Header("Cache-Control", "public, max-age=60, stale-while-revalidate=300")
	if err := pl.emit(Event{Name: "mksine.storefront.viewed", Site: req.site.ID, Data: map[string]any{"path": req.ctx.Request().Path(), "view": page.Record, "cache": "hit"}}); err != nil {
		pl.logf("cms: viewed listener: %v", err)
	}
	return h.Data(200, page.Type, []byte(page.Body))
}

// storePage keeps a rendered page (HTML up to 2 MB).
func (pl *Plugin) storePage(req *request, body []byte) {
	if req.cacheKey == "" || len(body) > 2<<20 {
		return
	}
	ttl := req.cacheTTL
	if next, ok := pl.nextScheduled(req); ok {
		if next <= 0 {
			return
		}
		ttl = min(ttl, next)
	}
	raw, err := json.Marshal(cachedPage{Body: string(body), Type: "text/html; charset=utf-8", Created: time.Now().Unix(), Record: req.view})
	if err != nil {
		return
	}
	if err := pl.Cache.Put(req.cacheKey, string(raw), ttl); err != nil {
		pl.logf("cms: page cache: %v", err)
	}
}

// nextScheduled is how long until the site's next scheduled post, page or entry goes
// public (nothing saves then, so cached lists must not outlive it).
func (pl *Plugin) nextScheduled(req *request) (time.Duration, bool) {
	var soonest *time.Time
	for _, table := range []string{PostsTable, PagesTable, EntriesTable} {
		var rows []struct{ PublishedAt *time.Time }
		err := pl.query().Table(table).Select("published_at").
			Where("site_id = ? AND status IN ? AND published_at > ?", req.site.ID, []string{"published", "scheduled"}, req.now).
			Order("published_at").Limit(1).Find(&rows)
		if err != nil {
			pl.logf("cms: page cache: scheduled %s: %v", table, err)
			return 0, true // unknown: do not cache
		}
		if len(rows) == 1 && rows[0].PublishedAt != nil && (soonest == nil || rows[0].PublishedAt.Before(*soonest)) {
			soonest = rows[0].PublishedAt
		}
	}
	if soonest == nil {
		return 0, false
	}
	return soonest.Sub(req.now), true
}

// cmsResources are the resources whose changes show on the public site.
var cmsResources = map[string]bool{
	"pages": true, "page_trash": true, "page_revisions": true, "cms_posts": true, "cms_categories": true,
	"cms_tags": true, "cms_comments": true, "cms_media": true, "cms_sections": true, "menus": true, "menu_locations": true,
}

// watchChanges retires a site's cached pages on every save or delete of a CMS record.
func (pl *Plugin) watchChanges(p *mks.Panel) {
	p.OnSaved(func(ctx *mks.HookContext, resource string, _ any, _ bool) error {
		if pl.isWatched(resource) && ctx != nil {
			pl.bumpAfterCommit(ctx)
		}
		return nil
	})
	p.OnDeleted(func(ctx *mks.HookContext, resource string, _ any) error {
		if pl.isWatched(resource) && ctx != nil {
			pl.bumpAfterCommit(ctx)
		}
		return nil
	})
}

// bumping wraps an action so a success retires the site's cached pages (actions write
// records themselves, without the panel's save events).
func bumping[T any](pl *Plugin, h func(*mks.ActionContext[T]) (*mks.ActionResult, error)) func(*mks.ActionContext[T]) (*mks.ActionResult, error) {
	return func(ctx *mks.ActionContext[T]) (*mks.ActionResult, error) {
		r, err := h(ctx)
		if err == nil && ctx != nil && ctx.HookContext != nil {
			pl.bumpAfterCommit(ctx.HookContext)
		}
		return r, err
	}
}

// isWatched: does a change of this resource show on the site? Content types of ZIP
// plugins come and go, so every entries_ resource counts.
func (pl *Plugin) isWatched(resource string) bool {
	return pl.watched[resource] || strings.HasPrefix(resource, "entries_")
}
