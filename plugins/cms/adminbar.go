package cms

import (
	"fmt"
	"html/template"
	"net/url"
	"strings"

	mks "github.com/MiranSalehi/mks"
)

// The front-end admin bar (MKSine's frontend admin bar): a strip at the top of the public
// site for signed-in panel users of that site, with links into the panel (the dashboard,
// editing what is on screen, writing a post). Such pages are never cached, by the page
// cache or by browsers, since they differ per user. The "frontend_admin_bar.items" filter
// can change the links ([]AdminBarItem).

// AdminBarItem is a link of the admin bar.
type AdminBarItem struct {
	Key   string
	Label string
	URL   string
}

type adminBarView struct {
	User  string
	Items []adminBarLink
}

type adminBarLink struct {
	Label string
	URL   template.URL
}

// adminUser is the signed-in panel user of the request when they belong to its site.
func (pl *Plugin) adminUser(req *request) *mks.HookContext {
	if pl.panel == nil || req == nil || req.ctx == nil || req.site == nil {
		return nil
	}
	hc := pl.panel.Context(req.ctx)
	if hc == nil || hc.User == nil || !hc.BelongsTo(req.site.ID) {
		return nil
	}
	return hc
}

// adminBar builds the bar for the page being shown.
func (pl *Plugin) adminBar(req *request, hc *mks.HookContext) *adminBarView {
	base := strings.TrimRight(pl.panel.Path(), "/")
	link := func(path string) string {
		return base + path + "?tenant=" + url.QueryEscape(fmt.Sprint(req.site.ID))
	}
	allowed := func(permission string) bool { return pl.panel.Allowed(hc, permission) }
	items := []AdminBarItem{{Key: "dashboard", Label: "Dashboard", URL: link("/")}}

	edit := func(key, label, resource string, ability Ability, id uint) {
		if id != 0 && allowed(resource+".update") && pl.can(hc, ability) {
			items = append(items, AdminBarItem{Key: key, Label: label, URL: link(fmt.Sprintf("/%s/%d/edit", resource, id))})
		}
	}
	switch r := req.record.(type) {
	case *Post:
		edit("edit_post", "Edit post", "cms_posts", ManagePosts, r.ID)
	case *Page:
		edit("edit_page", "Edit page", "pages", ManagePages, r.ID)
	case *Category:
		edit("edit_category", "Edit category", "cms_categories", ManageTaxonomies, r.ID)
	case *Tag:
		edit("edit_tag", "Edit tag", "cms_tags", ManageTaxonomies, r.ID)
	case *Entry:
		edit("edit_entry", "Edit", "entries_"+r.ContentType, ManagePosts, r.ID)
	}
	if allowed("cms_posts.create") && pl.can(hc, ManagePosts) {
		items = append(items, AdminBarItem{Key: "new_post", Label: "New post", URL: link("/cms_posts/create")})
	}
	if allowed("cms_posts.view_any") {
		items = append(items, AdminBarItem{Key: "manage_posts", Label: "Posts", URL: link("/cms_posts")})
	}
	if allowed("pages.view_any") {
		items = append(items, AdminBarItem{Key: "manage_pages", Label: "Pages", URL: link("/pages")})
	}
	if out, ok := pl.filter("frontend_admin_bar.items", items, req.site, req.record).([]AdminBarItem); ok {
		items = out
	}

	view := &adminBarView{User: hc.User.PanelUser().Name}
	for _, it := range items {
		u := safeURL(it.URL)
		if u == "" || strings.TrimSpace(it.Label) == "" {
			continue
		}
		view.Items = append(view.Items, adminBarLink{Label: it.Label, URL: u})
	}
	return view
}
