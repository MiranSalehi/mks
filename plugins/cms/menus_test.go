package cms

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	mks "github.com/MiranSalehi/mks"
)

// decoded turns a JSON body into what the panel hands to hooks.
func decoded(t *testing.T, body string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func pageID(n uint) *uint { return &n }

func TestNormalizeMenuItemsKeepsOnlyTheFieldsOfEachType(t *testing.T) {
	items, errs := normalizeMenuItems(decoded(t, `[
		{"type": "page", "page_id": 4, "label": "  ", "url": "javascript:alert(1)", "new_tab": true, "extra": "x", "children": null},
		{"type": "link", "label": " Blog ", "url": " https://blog.example ", "page_id": 9},
		{"type": "group", "label": "Company", "url": "/ignored", "new_tab": true, "children": [
			{"type": "page", "page_id": "7", "label": "Team", "children": [{"type": "link", "label": "Jobs", "url": "/jobs"}]},
			{"type": "link", "label": "Mail", "url": "mailto:a@example.com"}
		]}
	]`))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	want := []MenuItem{
		{Type: "page", PageID: pageID(4), NewTab: true, Children: []MenuItem{}},
		{Type: "link", Label: "Blog", URL: "https://blog.example", Children: []MenuItem{}},
		{Type: "group", Label: "Company", Children: []MenuItem{
			{Type: "page", PageID: pageID(7), Label: "Team", Children: []MenuItem{{Type: "link", Label: "Jobs", URL: "/jobs", Children: []MenuItem{}}}},
			{Type: "link", Label: "Mail", URL: "mailto:a@example.com", Children: []MenuItem{}},
		}},
	}
	if !reflect.DeepEqual(items, want) {
		t.Fatalf("normalized =\n%#v\nwant\n%#v", items, want)
	}

	if items, errs := normalizeMenuItems(nil); len(errs) > 0 || items == nil || len(items) != 0 {
		t.Fatalf("null items = %#v, %v; want an empty list", items, errs)
	}
}

func TestNormalizeMenuItemsReportsErrorsAtFormPaths(t *testing.T) {
	_, errs := normalizeMenuItems(decoded(t, `[
		{"type": "page"},
		{"type": "link", "label": "Bad", "url": "javascript:alert(1)"},
		{"type": "link", "label": "", "url": ""},
		{"type": "group", "label": "Lonely"},
		{"type": "group", "label": "Nested", "children": [
			{"type": "group", "label": "Empty inner group"},
			{"type": "page", "page_id": -1},
			{"type": "page", "page_id": 2.5},
			"not an object"
		]},
		{"type": "other", "label": 5},
		{"type": "page", "page_id": 1, "label": "`+strings.Repeat("x", maxMenuLabel+1)+`"},
		{"type": "group", "label": "Bad children", "children": {"a": 1}}
	]`))
	want := mks.ValidationErrors{
		"items.0.page_id":             "Select a page.",
		"items.1.url":                 "The URL must start with https://, http://, mailto:, tel: or /.",
		"items.2.label":               "The title is required.",
		"items.2.url":                 "The link URL is required.",
		"items.3.children":            "A group must have at least one submenu.",
		"items.4.children.0.children": "A group must have at least one submenu.",
		"items.4.children.1.page_id":  "Select a page.",
		"items.4.children.2.page_id":  "Select a page.",
		"items.4.children.3":          "Invalid menu item structure.",
		"items.5.type":                "Invalid menu item type.",
		"items.5.label":               "The title must be text.",
		"items.6.label":               "The title is at most 100 characters.",
		"items.7.children":            "Invalid submenu structure.",
	}
	if !reflect.DeepEqual(errs, want) {
		t.Fatalf("errors =\n%#v\nwant\n%#v", errs, want)
	}
}

func TestNormalizeMenuItemsLimits(t *testing.T) {
	link := `{"type":"link","label":"a","url":"/a"}`
	list := func(n int) string { return "[" + strings.TrimSuffix(strings.Repeat(link+",", n), ",") + "]" }
	if _, errs := normalizeMenuItems(decoded(t, list(maxMenuList+1))); errs["items"] == "" {
		t.Fatalf("more than %d items in a list were accepted: %v", maxMenuList, errs)
	}
	if _, errs := normalizeMenuItems(decoded(t, `[{"type":"group","label":"g","children":`+list(maxMenuList+1)+`}]`)); errs["items.0.children"] == "" {
		t.Fatalf("more than %d children were accepted: %v", maxMenuList, errs)
	}
	// Five full groups of 45 links are more entries than a menu may have.
	groups := make([]string, 5)
	for i := range groups {
		groups[i] = `{"type":"group","label":"g","children":` + list(45) + `}`
	}
	if _, errs := normalizeMenuItems(decoded(t, "["+strings.Join(groups, ",")+"]")); errs["items"] == "" {
		t.Fatalf("more than %d entries were accepted: %v", maxMenuEntries, errs)
	}
	// Six levels are fine, a seventh is not.
	nest := func(levels int) string {
		s := link
		for range levels - 1 {
			s = `{"type":"group","label":"g","children":[` + s + `]}`
		}
		return "[" + s + "]"
	}
	if items, errs := normalizeMenuItems(decoded(t, nest(MaxMenuDepth))); len(errs) > 0 || len(items) != 1 {
		t.Fatalf("%d levels: %v", MaxMenuDepth, errs)
	}
	if _, errs := normalizeMenuItems(decoded(t, nest(MaxMenuDepth+1))); len(errs) == 0 {
		t.Fatalf("%d levels were accepted", MaxMenuDepth+1)
	}
	if _, errs := normalizeMenuItems(map[string]any{"type": "page"}); errs["items"] == "" {
		t.Fatal("a non-list value was accepted")
	}
}

type fakeMenuDB struct {
	pages      map[uint]bool // pages of the site, not trashed
	posts      map[uint]bool // posts of the site
	asked      []any         // team ids the lookups were asked about
	err        error
	pagesAsked []uint
}

func (f *fakeMenuDB) lookups() menuLookups {
	return menuLookups{
		siteRefs: func(siteID any, refs MenuRefs) (map[string]bool, error) {
			f.asked, f.pagesAsked = append(f.asked, siteID), refs.Pages
			found := map[string]bool{}
			for _, id := range refs.Pages {
				if f.pages[id] {
					found[refKey("page", id)] = true
				}
			}
			for _, id := range refs.Posts {
				if f.posts[id] {
					found[refKey("post", id)] = true
				}
			}
			return found, f.err
		},
	}
}

func TestValidateMenuOnCreate(t *testing.T) {
	db := &fakeMenuDB{pages: map[uint]bool{1: true, 2: true}}
	data := map[string]any{
		"site_id": uint(10),
		"name":    "  Main  ",
		"items":   decoded(t, `[{"type":"page","page_id":1,"children":[{"type":"page","page_id":2}]}]`),
	}
	if err := testPlugin().validateMenu(data, nil, db.lookups()); err != nil {
		t.Fatal(err)
	}
	if data["name"] != "Main" {
		t.Fatalf("name not normalized: %#v", data)
	}
	if _, ok := data["items"].([]MenuItem); !ok {
		t.Fatalf("items not normalized: %#v", data["items"])
	}
	if !reflect.DeepEqual(db.asked, []any{uint(10)}) || !reflect.DeepEqual(db.pagesAsked, []uint{1, 2}) {
		t.Fatalf("lookups asked %v (pages %v), want the new record's team", db.asked, db.pagesAsked)
	}

	// Without a current site nothing is checked.
	if err := testPlugin().validateMenu(map[string]any{"name": "x"}, nil, db.lookups()); err == nil {
		t.Fatal("a menu without a site was accepted")
	}
}

func TestValidateMenuOnUpdate(t *testing.T) {
	db := &fakeMenuDB{pages: map[uint]bool{1: true}}
	stored := &Menu{SiteID: 10, Name: "Main"}
	stored.ID = 42

	// The stored team is used, never a team id sent by the client.
	data := map[string]any{"site_id": uint(99), "items": decoded(t, `[{"type":"page","page_id":1}]`)}
	if err := testPlugin().validateMenu(data, stored, db.lookups()); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(db.asked, []any{uint(10)}) {
		t.Fatalf("asked %v, want the stored team", db.asked)
	}

	// Pages of another site, trashed or missing pages are refused where they are.
	err := testPlugin().validateMenu(map[string]any{"items": decoded(t, `[
		{"type":"page","page_id":1},
		{"type":"group","label":"G","children":[{"type":"page","page_id":1},{"type":"page","page_id":77}]},
		{"type":"page","page_id":78}
	]`)}, stored, db.lookups())
	var ve mks.ValidationErrors
	if !errors.As(err, &ve) || len(ve) != 2 || ve["items.1.children.1.page_id"] == "" || ve["items.2.page_id"] == "" {
		t.Fatalf("foreign pages: %v", err)
	}

	// Lookup failures are server errors, not validation errors.
	db.err = errors.New("db down")
	if err := testPlugin().validateMenu(map[string]any{"items": decoded(t, `[{"type":"page","page_id":1}]`)}, stored, db.lookups()); err == nil || errors.As(err, &ve) {
		t.Fatalf("lookup error: %v", err)
	}
}

func TestMenusResourcePolicyAndConfig(t *testing.T) {
	r := testPlugin().menusResource()
	c := r.Config()
	if c.Name != "menus" || c.Tenant != "site_id" || c.TitleColumn != "name" || !c.Deletable {
		t.Fatalf("unexpected menus config: %#v", c)
	}
	if !reflect.DeepEqual(c.Fillable, []string{"name", "items"}) {
		t.Fatalf("site_id or other columns must not be fillable: %v", c.Fillable)
	}
	policy := testPlugin().menusPolicy()
	for _, actorRole := range []string{"admin", "editor", "viewer", ""} {
		ctx := actionsTestContext[Menu](nil, actorRole).HookContext
		want := actorRole == "admin" || actorRole == "editor"
		if policy.Create(ctx) != want || policy.Update(ctx, nil) != want || policy.Delete(ctx, nil) != want {
			t.Fatalf("role %q: menu writes should be %v", actorRole, want)
		}
	}
}
