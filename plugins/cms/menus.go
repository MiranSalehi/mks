package cms

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	mks "github.com/MiranSalehi/mks"
)

// Menu limits: levels, entries in one list, entries in all, characters of a label.
const (
	MaxMenuDepth   = 6
	maxMenuList    = 50
	maxMenuEntries = 200
	maxMenuLabel   = 100
)

// menusResource is the site navigation (menus in @mks/mks/cms). A menu is a two-level
// tree in one JSON column; the hooks check every entry, keep only known fields and make
// sure page entries point at pages of the same site.
func (pl *Plugin) menusResource() mks.ModelResource[Menu] {
	return mks.Define[Menu](mks.Config{
		Name:        "menus",
		Tenant:      "site_id",
		Searchable:  []string{"name"},
		Sortable:    []string{"id", "name", "updated_at"},
		Fillable:    []string{"name", "items"},
		TitleColumn: "name",
		DefaultSort: "name",
		Deletable:   true,
		Rules: map[string]string{
			"name":  "required|max_len:100",
			"items": "array",
		},
	}).
		BeforeCreate(func(_ *mks.HookContext, data map[string]any) error {
			if _, sent := data["items"]; !sent {
				data["items"] = []MenuItem{}
			}
			return pl.validateMenu(data, nil, pl.dbMenuLookups())
		}).
		BeforeUpdate(func(_ *mks.HookContext, menu *Menu, data map[string]any) error {
			return pl.validateMenu(data, menu, pl.dbMenuLookups())
		}).
		Policy(pl.menusPolicy())
}

// menusPolicy: everyone may read menus; ManageMenus writes them.
func (pl *Plugin) menusPolicy() mks.Policy[Menu] {
	return mks.Policy[Menu]{
		Create: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManageMenus) },
		Update: func(ctx *mks.HookContext, _ *Menu) bool { return pl.can(ctx, ManageMenus) },
		Delete: func(ctx *mks.HookContext, _ *Menu) bool { return pl.can(ctx, ManageMenus) },
	}
}

// menuLookups are the database questions a menu save asks, apart so tests can answer them.
type menuLookups struct {
	// siteRefs are the records among refs that exist in the site (pages not in the trash),
	// as "kind:id" keys.
	siteRefs func(siteID any, refs MenuRefs) (map[string]bool, error)
}

func (pl *Plugin) dbMenuLookups() menuLookups {
	return menuLookups{
		siteRefs: func(siteID any, refs MenuRefs) (map[string]bool, error) {
			found := map[string]bool{}
			for _, k := range []struct {
				kind  string
				model any
				ids   []uint
				where string
			}{
				{"page", &[]Page{}, refs.Pages, "site_id = ? AND deleted_at IS NULL"},
				{"post", &[]Post{}, refs.Posts, "site_id = ?"},
				{"category", &[]Category{}, refs.Categories, "site_id = ?"},
				{"tag", &[]Tag{}, refs.Tags, "site_id = ?"},
			} {
				if len(k.ids) == 0 {
					continue
				}
				list := make([]any, len(k.ids))
				for i, id := range k.ids {
					list[i] = id
				}
				var rows []struct{ ID uint }
				table := map[string]string{"page": PagesTable, "post": PostsTable, "category": CategoriesTable, "tag": TagsTable}[k.kind]
				if err := pl.query().Table(table).Select("id").Where(k.where, siteID).WhereIn("id", list).Scan(&rows); err != nil {
					return nil, err
				}
				for _, r := range rows {
					found[refKey(k.kind, r.ID)] = true
				}
			}
			return found, nil
		},
	}
}

func refKey(kind string, id uint) string { return kind + ":" + strconv.FormatUint(uint64(id), 10) }

// validateMenu checks and normalizes the sent fields of a menu: the name is trimmed and
// the items become their stored shape with records of this site only. Where a menu shows
// is chosen on its locations (menu_locations).
func (pl *Plugin) validateMenu(data map[string]any, stored *Menu, lookups menuLookups) error {
	siteID := data["site_id"]
	if stored != nil {
		siteID = stored.SiteID
	}
	if siteID == nil {
		return mks.Reject("Menus need an active site.")
	}
	if name, ok := data["name"].(string); ok {
		data["name"] = strings.TrimSpace(name)
	}

	raw, sent := data["items"]
	if !sent {
		return nil
	}
	items, errs := normalizeMenuItems(raw)
	if len(errs) > 0 {
		return errs
	}
	refs := Refs(items)
	found := map[string]bool{}
	if !refs.Empty() {
		var err error
		if found, err = lookups.siteRefs(siteID, refs); err != nil {
			return err
		}
	}
	missing := mks.ValidationErrors{}
	check := func(path, kind string, page, post, category, tag *uint) {
		k, id := refOf(kind, page, post, category, tag)
		if id != nil && !found[refKey(k, *id)] {
			missing[path+"."+k+"_id"] = refNotFound[k]
		}
	}
	var walk func(list []MenuItem, path string)
	walk = func(list []MenuItem, path string) {
		for i, item := range list {
			p := fmt.Sprintf("%s.%d", path, i)
			check(p, item.Type, item.PageID, item.PostID, item.CategoryID, item.TagID)
			walk(item.Children, p+".children")
		}
	}
	walk(items, "items")
	if len(missing) > 0 {
		return missing
	}
	data["items"] = items
	return nil
}

// normalizeMenuItems checks a submitted menu tree (decoded JSON) and returns it in its
// stored shape: known fields only, trimmed text, record ids as numbers, and only the
// fields of each entry's type. Error keys are form paths (items.0.children.1.url).
func normalizeMenuItems(raw any) ([]MenuItem, mks.ValidationErrors) {
	errs := mks.ValidationErrors{}
	if raw == nil {
		return []MenuItem{}, nil
	}
	count := 0
	items := normalizeMenuList(raw, "items", 1, errs, &count)
	if len(errs) > 0 {
		return nil, errs
	}
	return items, nil
}

// normalizeMenuList checks one list of the tree at a depth (1 is the top).
func normalizeMenuList(raw any, path string, depth int, errs mks.ValidationErrors, count *int) []MenuItem {
	list, ok := raw.([]any)
	if !ok {
		errs[path] = "Invalid menu items structure."
		return nil
	}
	if len(list) > maxMenuList {
		errs[path] = fmt.Sprintf("Each menu level has at most %s items.", faDigits(maxMenuList))
		return nil
	}
	items := make([]MenuItem, 0, len(list))
	for i, rawItem := range list {
		p := fmt.Sprintf("%s.%d", path, i)
		if *count++; *count > maxMenuEntries {
			errs["items"] = fmt.Sprintf("A menu has at most %s items.", faDigits(maxMenuEntries))
			return nil
		}
		item, ok := normalizeMenuEntry(rawItem, p, errs)
		if !ok {
			continue
		}
		obj, _ := rawItem.(map[string]any)
		childFailed := false
		switch children := obj["children"].(type) {
		case nil:
		case []any:
			if len(children) > 0 && depth >= MaxMenuDepth {
				errs[p+".children"] = fmt.Sprintf("A menu has at most %s levels.", faDigits(MaxMenuDepth))
				childFailed = true
				break
			}
			before := len(errs)
			item.Children = normalizeMenuList(children, p+".children", depth+1, errs, count)
			childFailed = len(errs) > before
		default:
			errs[p+".children"] = "Invalid submenu structure."
			childFailed = true
		}
		if item.Children == nil {
			item.Children = []MenuItem{}
		}
		// An empty group is an error of its own; with failed children those errors say enough.
		if item.Type == "group" && len(item.Children) == 0 && !childFailed {
			errs[p+".children"] = "A group must have at least one submenu."
		}
		items = append(items, item)
	}
	return items
}

// normalizeMenuEntry checks the fields of one entry (not its children).
func normalizeMenuEntry(raw any, path string, errs mks.ValidationErrors) (MenuItem, bool) {
	obj, ok := raw.(map[string]any)
	if !ok {
		errs[path] = "Invalid menu item structure."
		return MenuItem{}, false
	}
	valid := true
	fail := func(field, msg string) {
		errs[path+"."+field] = msg
		valid = false
	}

	kind, _ := obj["type"].(string)
	switch {
	case kind == "page", kind == "post", kind == "category", kind == "tag", kind == "link", kind == "group":
	default:
		fail("type", "Invalid menu item type.")
	}

	label, isString := obj["label"].(string)
	if obj["label"] != nil && !isString {
		fail("label", "The title must be text.")
	}
	label = strings.TrimSpace(label)
	if utf8.RuneCountInString(label) > maxMenuLabel {
		fail("label", fmt.Sprintf("The title is at most %s characters.", faDigits(maxMenuLabel)))
	} else if label == "" && (kind == "link" || kind == "group") {
		fail("label", "The title is required.")
	}

	entry := MenuItem{Type: kind, Label: label}
	switch kind {
	case "page", "post", "category", "tag":
		id, ok := menuPageID(obj[kind+"_id"])
		if !ok {
			fail(kind+"_id", refMissing[kind])
		}
		switch kind {
		case "page":
			entry.PageID = &id
		case "post":
			entry.PostID = &id
		case "category":
			entry.CategoryID = &id
		case "tag":
			entry.TagID = &id
		}
		entry.NewTab, _ = obj["new_tab"].(bool)
	case "link":
		link, _ := obj["url"].(string)
		link = strings.TrimSpace(link)
		switch {
		case link == "":
			fail("url", "The link URL is required.")
		case !SafeLinkURL(link):
			fail("url", "The URL must start with https://, http://, mailto:, tel: or /.")
		}
		entry.URL = link
		entry.NewTab, _ = obj["new_tab"].(bool)
	}
	return entry, valid
}

var refMissing = map[string]string{
	"page": "Select a page.", "post": "Select a post.",
	"category": "Select a category.", "tag": "Select a tag.",
}

var refNotFound = map[string]string{
	"page":     "This page was not found on this site, or it is in the trash.",
	"post":     "This post was not found on this site.",
	"category": "This category was not found on this site.",
	"tag":      "This tag was not found on this site.",
}

// menuPageID reads a record id sent as a JSON number or a numeric string.
func menuPageID(raw any) (uint, bool) {
	switch v := raw.(type) {
	case float64:
		if v >= 1 && v <= math.MaxUint32 && v == math.Trunc(v) {
			return uint(v), true
		}
	case string:
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 32)
		if err == nil && n > 0 {
			return uint(n), true
		}
	}
	return 0, false
}
