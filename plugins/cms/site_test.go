package cms

import (
	"strings"
	"testing"
	"time"

	mks "github.com/MiranSalehi/mks"
)

func TestPreviewGrantRejectsMalformedTokensBeforeLookup(t *testing.T) {
	for _, token := range []string{"", "not-hex", "00", strings.Repeat("z", 64)} {
		t.Run(token, func(t *testing.T) {
			if grant, ok, err := testPlugin().checkPreview(nil, token); err != nil || ok || grant != nil {
				t.Fatalf("checkPreview(nil, %q) = (%v, %v, %v), want (nil, false, nil)", token, grant, ok, err)
			}
		})
	}
}

func TestPublicPageAvailable(t *testing.T) {
	now := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	past := now.Add(-time.Second)
	future := now.Add(time.Second)

	cases := []struct {
		name   string
		status string
		at     *time.Time
		want   bool
	}{
		{name: "published without date", status: "published", want: true},
		{name: "published in past", status: "published", at: &past, want: true},
		{name: "published exactly now", status: "published", at: &now, want: true},
		{name: "published future date", status: "published", at: &future, want: false},
		{name: "scheduled without date", status: "scheduled", want: false},
		{name: "scheduled in past", status: "scheduled", at: &past, want: true},
		{name: "scheduled exactly now", status: "scheduled", at: &now, want: true},
		{name: "scheduled future date", status: "scheduled", at: &future, want: false},
		{name: "draft", status: "draft", want: false},
		{name: "unknown", status: "other", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := publicPageAvailable(tc.status, tc.at, now); got != tc.want {
				t.Fatalf("publicPageAvailable(%q, %v) = %v, want %v", tc.status, tc.at, got, tc.want)
			}
		})
	}
}

func TestRenderCMSPageSanitizesAndEscapesContent(t *testing.T) {
	richText := `<p>Hello</p><script>alert(1)</script><img src=x onerror=alert(2)>`
	page := Page{
		Title: "<img src=x onerror=alert(3)>", Slug: "welcome", Type: "simple",
		Status: "published", Content: &richText, ShowPageHeader: true,
		BuilderContentWidth: "contained",
		MetaTitle:           ptr("<script>meta()</script>"),
	}
	team := Site{Slug: "my-site"}
	body, err := testPlugin().renderPage(&page, &team, siteNav{})
	if err != nil {
		t.Fatalf("renderCMSPage: %v", err)
	}
	html := string(body)
	if strings.Contains(html, "<script>") || strings.Contains(html, "onerror=\"") || strings.Contains(html, "<img src=x onerror=alert(3)>") {
		t.Fatalf("unsafe HTML survived sanitizing/escaping: %s", html)
	}
	if !strings.Contains(html, "&lt;img src=x onerror=alert(3)&gt;") || !strings.Contains(html, "href=\"/sites/my-site/page/welcome\"") {
		t.Fatalf("escaped title or canonical URL missing: %s", html)
	}
	if !strings.Contains(html, "<p>Hello</p>") {
		t.Fatalf("safe rich text was not retained: %s", html)
	}
}

func TestRenderCMSPageEscapesBuilderText(t *testing.T) {
	page := Page{
		Title: "Page", Slug: "builder", Type: "builder", Status: "published",
		BuilderContentWidth: "wide",
		BuilderPayload:      mks.NewJSON([]Block{{Type: "quote", Data: map[string]any{"text": "<script>bad()</script>", "author": "<b>Author</b>"}}}),
	}
	team := Site{Slug: "site"}
	body, err := testPlugin().renderPage(&page, &team, siteNav{})
	if err != nil {
		t.Fatalf("renderCMSPage: %v", err)
	}
	html := string(body)
	if strings.Contains(html, "<script>") || strings.Contains(html, "<b>Author</b>") {
		t.Fatalf("builder text was not escaped: %s", html)
	}
	if !strings.Contains(html, "&lt;script&gt;bad()&lt;/script&gt;") || !strings.Contains(html, "&lt;b&gt;Author&lt;/b&gt;") {
		t.Fatalf("escaped builder text is missing: %s", html)
	}
}

func TestRenderCMSPageMenus(t *testing.T) {
	content := "<p>body</p>"
	page := Page{Title: "Home", Slug: "home", Type: "simple", Status: "published", Content: &content, BuilderContentWidth: "contained"}
	team := Site{Slug: "s"}

	body, err := testPlugin().renderPage(&page, &team, siteNav{})
	if err != nil {
		t.Fatal(err)
	}
	if html := string(body); strings.Contains(html, "<nav") {
		t.Fatalf("a site without menus rendered empty navigation: %s", html)
	}

	nav := siteNav{
		"header": []NavItem{
			{Label: "<b>Home</b>", URL: "/sites/s/home", Current: true},
			{Label: "Company", Current: true, Children: []NavItem{
				{Label: "Call", URL: "tel:+982100000000"},
				{Label: "Blog", URL: "https://blog.example/?a=1&b=2", NewTab: true},
			}},
			// Never produced by BuildNav, but the renderer refuses it on its own too.
			{Label: "Evil", URL: "javascript:alert(1)"},
			{Label: "Empty group", Children: []NavItem{{Label: "Evil child", URL: "//evil.example"}}},
		},
		"footer": []NavItem{{Label: "Mail", URL: "mailto:info@example.com"}},
	}
	body, err = testPlugin().renderPage(&page, &team, nav)
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	for _, want := range []string{
		`<nav aria-label="Main menu">`,
		`<a href="/sites/s/home" aria-current="page">&lt;b&gt;Home&lt;/b&gt;</a>`,
		`<details class="current"><summary>Company</summary>`,
		`<a href="tel:&#43;982100000000">Call</a>`,
		`<a href="https://blog.example/?a=1&amp;b=2" target="_blank" rel="noopener noreferrer">Blog</a>`,
		`<nav aria-label="Footer menu">`,
		`<a href="mailto:info@example.com">Mail</a>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s in:\n%s", want, html)
		}
	}
	for _, bad := range []string{"<b>Home</b>", "javascript:", "Evil", "evil.example", "ZgotmplZ"} {
		if strings.Contains(html, bad) {
			t.Errorf("rendered %q:\n%s", bad, html)
		}
	}
}

func ptr(value string) *string { return &value }
