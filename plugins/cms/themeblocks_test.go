package cms

import (
	"strings"
	"testing"
)

func TestThemeBlocksRender(t *testing.T) {
	pl := &Plugin{}
	payload := decodeJSON(t, `[
		{"type": "mksine_hero_domain", "data": {"heading_line1": "Your domain.", "heading_line2_prefix": "Your brand.", "heading_accent": "in minutes", "cta_label": "Search", "cta_url": "javascript:alert(1)", "illustration_position": "left"}},
		{"type": "mksine_finance_showcase", "data": {"heading": "Accounts <b>without the pain</b>", "heading_accent": "without the pain", "paragraph_1": "A clear dashboard for everyone", "paragraph_1_highlight": "clear dashboard", "avatars": [{"letter": "A"}, {"letter": "<"}], "social_count": "12 thousand"}},
		{"type": "mksine_services_trio", "data": {"section_title": "Services", "cards": [{"title": "Hosting", "body": "Fast", "cta_label": "Plans", "url": "/plans"}]}},
		{"type": "mksine_featured_domains", "data": {"title": "Extensions", "items": [{"tld": ".ir", "price": "100", "original_price": "200", "discount_percent": 50, "href": "/ir"}]}},
		{"type": "mksine_clinic_features_grid", "data": {"text_direction": "ltr", "heading_prefix": "Tools for ", "heading_accent": "clinics", "cards": [{"gradient": "from-pink-500 to-pink-600", "icon": "calendar", "title": "Booking", "link_url": "/b", "link_new_tab": true}, {"gradient": "url(x)", "icon": "<script>", "title": "Bad"}]}},
		{"type": "mksine_testimonials_grid", "data": {"title_prefix": "Reviews ", "title_accent": "customers", "aside": "Support", "items": [{"name": "Sara", "city": "Tehran", "quote": "Great", "gradient": "from-sky-500 to-indigo-600"}]}},
		{"type": "mksine_post_comments_feed", "data": {"title_accent": "Comments"}}
	]`)
	blocks, errs := pl.NormalizeBuilder(payload, 1, "b")
	if len(errs) > 0 {
		t.Fatalf("normalize: %v", errs)
	}
	// Unknown gradient and icon names fall back to the first ones.
	card := blocks[4].Data["cards"].([]any)[1].(map[string]any)
	if card["gradient"] != ThemeGradients[0] || card["icon"] != ThemeIcons[0] {
		t.Fatalf("unknown names = %v / %v", card["gradient"], card["icon"])
	}
	html, _, err := pl.renderBuilder(&request{site: &Site{ID: 1, Slug: "s"}}, blocks)
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	for _, want := range []string{
		`class="pb tb tb-hero img-left`, `<span class="accent">in minutes</span>`, `href="#"`, // unsafe link
		`Accounts &lt;b&gt;<span class="accent">without the pain</span>&lt;/b&gt;`, `<span class="mark">clear dashboard</span>`, `<span>&lt;</span>`,
		`href="/plans"`, `−50%`, `<del>200</del>`,
		`dir="ltr"`, `--g1:#ec4899;--g2:#db2777`, `<svg viewBox="0 0 24 24"`, `target="_blank" rel="noopener noreferrer"`,
		`<aside class="tb-aside">Support</aside>`, `--g1:#0ea5e9`, `>S</span>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q", want)
		}
	}
	for _, bad := range []string{"<script>", "javascript:", "url(x)"} {
		if strings.Contains(out, bad) {
			t.Errorf("output contains %q", bad)
		}
	}
	// Without comments the feed shows nothing.
	if strings.Contains(out, "Comments") {
		t.Error("an empty comments feed is not shown")
	}
}

func TestHighlight(t *testing.T) {
	if got := string(highlight("a <b> c", "<b>", "x")); got != `a <span class="x">&lt;b&gt;</span> c` {
		t.Fatalf("highlight = %s", got)
	}
	if got := string(highlight("plain & text", "missing", "x")); got != "plain &amp; text" {
		t.Fatalf("no phrase = %s", got)
	}
	if initial("  Sara") != "S" || initial("") != "?" || initial("bob") != "B" {
		t.Fatal("initial")
	}
}
