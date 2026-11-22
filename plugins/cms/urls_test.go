package cms

import (
	"testing"
	"time"
)

func TestMatchRouteUsesPermalinksInOrder(t *testing.T) {
	pl := &Plugin{}
	cases := []struct {
		path  string
		kind  routeKind
		value string
	}{
		{"", routeHome, ""},
		{"/", routeHome, ""},
		{"/posts", routePosts, ""},
		{"/posts/", routePosts, ""},
		{"/post/hello-world", routePost, "hello-world"},
		{"/post/a/b", routeNone, ""},
		{"/category/news", routeCategory, "news"},
		{"/category/news/sport", routeCategory, "news/sport"},
		{"/category//x", routeNone, ""},
		{"/categories", routeCategories, ""},
		{"/tags", routeTags, ""},
		{"/tag/go", routeTag, "go"},
		{"/tag/caf%C3%A9", routeTag, "café"},
		{"/page/about", routePage, "about"},
		{"/author/12", routeAuthor, "12"},
		{"/nothing", routeNone, ""},
		{"/tag/%zz", routeNone, ""},
	}
	for _, c := range cases {
		kind, value := pl.matchRoute("", c.path)
		if kind != c.kind || value != c.value {
			t.Errorf("matchRoute(%q) = %v %q, want %v %q", c.path, kind, value, c.kind, c.value)
		}
	}

	custom := &Plugin{Permalinks: &Permalinks{Post: "blog/{slug}.html", Page: "/{slug}/"}}
	if kind, v := custom.matchRoute("", "/blog/hi.html"); kind != routePost || v != "hi" {
		t.Errorf("custom post route = %v %q", kind, v)
	}
	if kind, v := custom.matchRoute("", "/about"); kind != routePage || v != "about" {
		t.Errorf("custom page route = %v %q", kind, v)
	}
	// Built-in routes keep priority over a catch-all page pattern.
	if kind, _ := custom.matchRoute("", "/posts"); kind != routePosts {
		t.Errorf("/posts went to %v", kind)
	}
	if got := custom.PostURL("s", "hi"); got != "/sites/s/blog/hi.html" {
		t.Errorf("PostURL = %q", got)
	}
	if got := pl.CategoryURL("s", "news/the sport"); got != "/sites/s/category/news/the%20sport" {
		t.Errorf("CategoryURL = %q", got)
	}
}

func TestToJalali(t *testing.T) {
	cases := []struct{ gy, gm, gd, jy, jm, jd int }{
		{2026, 11, 21, 1405, 8, 30},
		{2025, 3, 21, 1404, 1, 1},
		{2024, 3, 20, 1403, 1, 1},
		{2000, 1, 1, 1378, 10, 11},
	}
	for _, c := range cases {
		jy, jm, jd := toJalali(c.gy, c.gm, c.gd)
		if jy != c.jy || jm != c.jm || jd != c.jd {
			t.Errorf("toJalali(%d-%d-%d) = %d-%d-%d, want %d-%d-%d", c.gy, c.gm, c.gd, jy, jm, jd, c.jy, c.jm, c.jd)
		}
	}
	d := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	if got, _ := formatDate(&d); got != "30 Aban 1405" {
		t.Errorf("formatDate = %q", got)
	}
	if got, iso := formatDate(nil); got != "" || iso != "" {
		t.Error("nil date should be empty")
	}
}

func TestPlainTextAndPagination(t *testing.T) {
	if got := plainText("<p>café &amp; <b>naïve</b></p>\n<script>x</script>", 160); got != "café & naïve x" {
		t.Errorf("plainText = %q", got)
	}
	if got := plainText("abcdefghij", 4); got != "abcd…" {
		t.Errorf("cut = %q", got)
	}
	if pagination("/b", 1, PerPage) != nil {
		t.Error("one page needs no pager")
	}
	p := pagination("/b", 2, PerPage*2+1)
	if len(p) != 3 || p[0].URL != "/b" || p[1].URL != "/b?page=2" || !p[1].Current || p[2].Label != "3" {
		t.Errorf("pagination = %#v", p)
	}
}
