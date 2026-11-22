package cms

import (
	"strings"
	"testing"
)

func TestThemeBlocksRender(t *testing.T) {
	pl := &Plugin{}
	payload := decodeJSON(t, `[
		{"type": "mksine_hero_domain", "data": {"heading_line1": "دامنهٔ شما.", "heading_line2_prefix": "برند شما.", "heading_accent": "در چند دقیقه", "cta_label": "جستجو", "cta_url": "javascript:alert(1)", "illustration_position": "left"}},
		{"type": "mksine_finance_showcase", "data": {"heading": "حساب‌ها <b>بی‌دردسر</b>", "heading_accent": "بی‌دردسر", "paragraph_1": "یک داشبورد روشن برای همه", "paragraph_1_highlight": "داشبورد روشن", "avatars": [{"letter": "A"}, {"letter": "<"}], "social_count": "۱۲ هزار"}},
		{"type": "mksine_services_trio", "data": {"section_title": "خدمات", "cards": [{"title": "میزبانی", "body": "سریع", "cta_label": "پلن‌ها", "url": "/plans"}]}},
		{"type": "mksine_featured_domains", "data": {"title": "پسوندها", "items": [{"tld": ".ir", "price": "۱۰۰", "original_price": "۲۰۰", "discount_percent": 50, "href": "/ir"}]}},
		{"type": "mksine_clinic_features_grid", "data": {"text_direction": "ltr", "heading_prefix": "Tools for ", "heading_accent": "clinics", "cards": [{"gradient": "from-pink-500 to-pink-600", "icon": "calendar", "title": "Booking", "link_url": "/b", "link_new_tab": true}, {"gradient": "url(x)", "icon": "<script>", "title": "Bad"}]}},
		{"type": "mksine_testimonials_grid", "data": {"title_prefix": "نظر ", "title_accent": "مشتریان", "aside": "پشتیبانی", "items": [{"name": "سارا", "city": "تهران", "quote": "عالی", "gradient": "from-sky-500 to-indigo-600"}]}},
		{"type": "mksine_post_comments_feed", "data": {"title_accent": "دیدگاه‌ها"}}
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
		`class="pb tb tb-hero img-left`, `<span class="accent">در چند دقیقه</span>`, `href="#"`, // unsafe link
		`حساب‌ها &lt;b&gt;<span class="accent">بی‌دردسر</span>&lt;/b&gt;`, `<span class="mark">داشبورد روشن</span>`, `<span>&lt;</span>`,
		`href="/plans"`, `−50٪`, `<del>۲۰۰</del>`,
		`dir="ltr"`, `--g1:#ec4899;--g2:#db2777`, `<svg viewBox="0 0 24 24"`, `target="_blank" rel="noopener noreferrer"`,
		`<aside class="tb-aside">پشتیبانی</aside>`, `--g1:#0ea5e9`, `>س</span>`,
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
	if strings.Contains(out, "دیدگاه‌ها") {
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
	if initial("  سارا") != "س" || initial("") != "؟" || initial("bob") != "B" {
		t.Fatal("initial")
	}
}
