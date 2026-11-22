package cms

import (
	"testing"
	"time"
)

func TestPermalinkRule(t *testing.T) {
	slug, none, path := permalinkRule("slug"), permalinkRule(""), permalinkRule("path")
	cases := []struct {
		rule  SettingRule
		in    string
		out   string
		error bool
	}{
		{slug, "", "", false},
		{slug, "blog/{slug}", "/blog/{slug}", false},
		{slug, "/blog/{slug}.html", "/blog/{slug}.html", false},
		{slug, "/blog/{slug}/", "/blog/{slug}", false},
		{slug, "/blog/x{slug}", "", true},
		{slug, "/blog", "", true},
		{slug, "/{slug}/{slug}", "", true},
		{slug, "/blog/{id}", "", true},
		{slug, "/Blog/{slug}", "", true},
		{slug, "/بلاگ/{slug}", "", true},
		{slug, "javascript:alert(1)", "", true},
		{none, "/articles", "/articles", false},
		{none, "/articles/{slug}", "", true},
		{path, "/topics/{path}", "/topics/{path}", false},
	}
	for _, c := range cases {
		out, msg := c.rule(nil, nil, c.in)
		if (msg != "") != c.error || (!c.error && out != c.out) {
			t.Errorf("rule(%q) = %q, %q", c.in, out, msg)
		}
	}
}

func TestCoreSettingRules(t *testing.T) {
	rules := (&Plugin{}).coreSettings()
	check := func(key, in, want string, fails bool) {
		t.Helper()
		out, msg := rules[key](nil, nil, in)
		if (msg != "") != fails || (!fails && out != want) {
			t.Errorf("%s(%q) = %q, %q", key, in, out, msg)
		}
	}
	check("site_name", "  My site ", "My site", false)
	check("date_calendar", "shamsi", "shamsi", false)
	check("date_calendar", "lunar", "", true)
	check("page_cache_enabled", "true", "1", false)
	check("page_cache_enabled", "", "0", false)
	check("page_cache_ttl_minutes", "60", "60", false)
	check("page_cache_ttl_minutes", "0", "", true)
	check("page_cache_ttl_minutes", "10081", "", true)
	if len(rules) != 12+len(permalinkKeys) {
		t.Errorf("unexpected number of core settings: %d", len(rules))
	}
	pl := &Plugin{}
	pl.RegisterSettings(map[string]SettingRule{"site_name": nil, "shop_currency": func(*Plugin, any, string) (string, string) { return "IRR", "" }})
	if r := pl.settingRules(); r["site_name"] == nil || r["shop_currency"] == nil {
		t.Error("extensions add keys but cannot replace core ones")
	}
}

func TestGregorianDates(t *testing.T) {
	d := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	if got, _ := formatDate(&d, "gregorian"); got != "۲۰۲۶-۱۱-۲۱" {
		t.Errorf("gregorian = %q", got)
	}
	if got, _ := formatDate(&d, "shamsi"); got != "۳۰ آبان ۱۴۰۵" {
		t.Errorf("shamsi = %q", got)
	}
}
