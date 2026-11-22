package cms

import (
	"reflect"
	"strings"
	"testing"
)

func TestSafeLinkURL(t *testing.T) {
	ok := []string{
		"https://example.com", "http://example.com/a?b=c#d", "HTTPS://EXAMPLE.COM",
		"/", "/sites/team/about", "#contact", "mailto:info@example.com", "tel:+982100000000",
		"https://example.com/فارسی",
	}
	bad := []string{
		"", " https://example.com", "https://example.com ", "javascript:alert(1)", "JavaScript:alert(1)",
		"data:text/html,<script>1</script>", "vbscript:x", "//evil.example", "/\\evil.example",
		"https:\\\\evil.example", "https://user:pass@example.com", "https://", "mailto:", "ftp://example.com",
		"about", "example.com", "https://exa mple.com", "/a\nb", "/a\tb", "/a\x00", "/a\x7f",
		"/" + strings.Repeat("a", MaxURLLength),
	}
	for _, u := range ok {
		if !SafeLinkURL(u) {
			t.Errorf("SafeLinkURL(%q) = false, want true", u)
		}
	}
	for _, u := range bad {
		if SafeLinkURL(u) {
			t.Errorf("SafeLinkURL(%q) = true, want false", u)
		}
	}
}

func TestPageURLEscapesSlugs(t *testing.T) {
	pl := &Plugin{}
	if got := pl.PageURL("my site", "a/b"); got != "/sites/my%20site/page/a%2Fb" {
		t.Fatalf("PageURL = %q", got)
	}
	if got := (&Plugin{PublicPath: "/s"}).PageURL("x", "y"); got != "/s/x/page/y" {
		t.Fatalf("PageURL with PublicPath = %q", got)
	}
	if !pl.isLocation("header") || !pl.isLocation("footer") || pl.isLocation("sidebar") || pl.isLocation("") {
		t.Fatal("unexpected default locations")
	}
	custom := &Plugin{Locations: []Location{{Key: "sidebar"}}}
	if !custom.isLocation("sidebar") || custom.isLocation("header") {
		t.Fatal("Locations should replace the defaults")
	}
}

func id(n uint) *uint { return &n }

func TestRefsAreUniqueAcrossLevelsAndKinds(t *testing.T) {
	items := []MenuItem{
		{Type: "page", PageID: id(3), Children: []MenuItem{{Type: "page", PageID: id(5)}, {Type: "page", PageID: id(3)}}},
		{Type: "link", URL: "/x"},
		{Type: "page", PageID: id(0)},
	}
	if got := Refs(items).Pages; !reflect.DeepEqual(got, []uint{3, 5}) {
		t.Fatalf("Refs().Pages = %v", got)
	}
	mixed := Refs([]MenuItem{{Type: "post", PostID: id(3), Children: []MenuItem{{Type: "category", CategoryID: id(3)}, {Type: "tag", TagID: id(4)}, {Type: "post", PostID: id(3)}}},
		{Type: "page", PostID: id(9)}})
	want := MenuRefs{Posts: []uint{3}, Categories: []uint{3}, Tags: []uint{4}}
	if !reflect.DeepEqual(mixed, want) {
		t.Fatalf("Refs = %#v, want %#v (an id of another kind than the entry type is ignored)", mixed, want)
	}
}

func TestBuildNav(t *testing.T) {
	pages := NavTargets{"page": {
		1: {URL: "/sites/s/about", Title: "About"},
		2: {URL: "/sites/s/team", Title: "Team"},
	}, "post": {5: {URL: "/sites/s/post/hello", Title: "Hello"}}, "category": {6: {URL: "/sites/s/category/news", Title: "News"}}}
	items := []MenuItem{
		// A visible page without a label shows the page title and marks the current page.
		{Type: "page", PageID: id(1)},
		// A page that is not public (absent from pages) is left out.
		{Type: "page", Label: "Draft", PageID: id(9)},
		// Unsafe links are left out even if they were stored somehow.
		{Type: "link", Label: "Bad", URL: "javascript:alert(1)"},
		{Type: "link", Label: "Blog", URL: "https://blog.example", NewTab: true},
		// A group with visible children is a trigger; hidden children are dropped.
		{Type: "group", Label: "Company", Children: []MenuItem{
			{Type: "page", PageID: id(2), Label: "Our team"},
			{Type: "page", PageID: id(9)},
			{Type: "link", Label: "", URL: "/no-label"},
		}},
		// A group whose children are all hidden disappears.
		{Type: "group", Label: "Empty", Children: []MenuItem{{Type: "page", PageID: id(9)}}},
		// A hidden page with visible children keeps its own label as the trigger.
		{Type: "page", Label: "Services", PageID: id(9), Children: []MenuItem{{Type: "link", Label: "Mail", URL: "mailto:a@example.com"}}},
		// ...but without a label of its own there is nothing to show.
		{Type: "page", PageID: id(9), Children: []MenuItem{{Type: "link", Label: "Lost", URL: "/lost"}}},
		{Type: "group", Label: "No children"},
		// Posts and categories resolve like pages; a tag that is not public is left out.
		{Type: "post", PostID: id(5)},
		{Type: "category", Label: "All news", CategoryID: id(6)},
		{Type: "tag", TagID: id(7)},
	}
	got := BuildNav(items, pages, "/sites/s/team")
	want := []NavItem{
		{Label: "About", URL: "/sites/s/about"},
		{Label: "Blog", URL: "https://blog.example", NewTab: true},
		{Label: "Company", Current: true, Children: []NavItem{{Label: "Our team", URL: "/sites/s/team", Current: true}}},
		{Label: "Services", Children: []NavItem{{Label: "Mail", URL: "mailto:a@example.com"}}},
		{Label: "Hello", URL: "/sites/s/post/hello"},
		{Label: "All news", URL: "/sites/s/category/news"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildNav =\n%#v\nwant\n%#v", got, want)
	}
	if BuildNav(nil, pages, "") != nil {
		t.Fatal("an empty menu should render nothing")
	}
}

func TestBuildNavNested(t *testing.T) {
	id := func(n uint) *uint { return &n }
	targets := NavTargets{"page": {1: {URL: "/a", Title: "A"}, 2: {URL: "/b", Title: "B"}}}
	nav := BuildNav([]MenuItem{{Type: "group", Label: "Top", Children: []MenuItem{
		{Type: "group", Label: "Mid", Children: []MenuItem{{Type: "page", PageID: id(2)}, {Type: "page", PageID: id(9)}}},
		{Type: "page", PageID: id(1)},
	}}}, targets, "/b")
	if len(nav) != 1 || len(nav[0].Children) != 2 || nav[0].Children[0].Label != "Mid" || len(nav[0].Children[0].Children) != 1 {
		t.Fatalf("nav = %+v", nav)
	}
	// The current page deep inside marks every trigger above it.
	if !nav[0].Current || !nav[0].Children[0].Current || !nav[0].Children[0].Children[0].Current {
		t.Fatalf("current not marked up the tree: %+v", nav)
	}
	html, err := (&Plugin{}).renderView(layoutView{HeaderNav: navViews(nav), Page: &pageView{Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `<summary>Mid</summary><ul><li><a href="/b"`) {
		t.Fatalf("nested menu not rendered:\n%s", html)
	}
}
