package cms

import (
	"errors"
	"html/template"
	"strings"
	"testing"
	"time"

	mks "github.com/MiranSalehi/mks"
)

func TestEventsRunInPriorityOrderAndCanStop(t *testing.T) {
	pl := &Plugin{}
	var got []string
	pl.On("post.creating", func(Event) error { got = append(got, "b"); return nil })
	pl.On("post.creating", func(Event) error { got = append(got, "a"); return nil }, 1)
	pl.On("post.creating", func(e Event) error {
		if e.Data["title"] == "forbidden" {
			return errors.New("no")
		}
		return nil
	}, 20)
	if err := pl.beforeSave(nil, "post", 1, nil, map[string]any{"title": "ok"}, "", ""); err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "") != "ab" {
		t.Fatalf("order = %v", got)
	}
	err := pl.beforeSave(nil, "post", 1, nil, map[string]any{"title": "forbidden"}, "", "")
	if err == nil || err.Error() != "no" {
		t.Fatalf("a listener could not stop the save: %v", err)
	}
}

func TestAfterSaveEvents(t *testing.T) {
	pl := &Plugin{}
	var got []string
	for _, n := range []string{"post.created", "post.updated", "post.published", "mksine.content.slug_changed"} {
		name := n
		pl.On(name, func(e Event) error {
			got = append(got, name+fmtAny(e.Data["old_slug"]))
			return nil
		})
	}
	_ = pl.afterSave(nil, "post", 1, &Post{}, true, "a", "published")
	if strings.Join(got, ",") != "post.created<nil>,post.published<nil>" {
		t.Fatalf("create: %v", got)
	}
	got = nil
	// The state of the record before the update comes from beforeSave through the context;
	// without one, an update is not a new publication and no slug change is known.
	_ = pl.afterSave(nil, "post", 1, &Post{}, false, "b", "published")
	if strings.Join(got, ",") != "post.updated<nil>,post.published<nil>" {
		t.Fatalf("update: %v", got)
	}
}

func TestFiltersAndActions(t *testing.T) {
	pl := &Plugin{}
	pl.AddFilter("x", func(v any, _ ...any) any { return v.(string) + "1" }, "one", 20)
	pl.AddFilter("x", func(v any, _ ...any) any { return v.(string) + "0" }, "zero", 5)
	if got := pl.filter("x", ">"); got != ">01" || !pl.HasFilter("x") {
		t.Fatalf("filter = %v", got)
	}
	pl.RemoveFilter("x", "zero")
	if got := pl.filter("x", ">"); got != ">1" {
		t.Fatalf("after remove = %v", got)
	}
	pl.AddAction("single.after_article", func(c ActionContext) template.HTML { return "<p>share " + template.HTML(c.View) + "</p>" }, "share")
	html := pl.actionsFor(ActionContext{View: "single"})
	if html["single.after_article"] != "<p>share single</p>" {
		t.Fatalf("actions = %v", html)
	}
	body, err := pl.renderView(layoutView{View: "single", Hooks: html, Post: &postView{Title: "T"}})
	if err != nil || !strings.Contains(string(body), "<p>share single</p>") {
		t.Fatalf("action not rendered: %v", err)
	}
	pl.RemoveAction("single.after_article", "share")
	if len(pl.actionsFor(ActionContext{View: "single"})) != 0 {
		t.Fatal("action not removed")
	}
}

func TestShortcodes(t *testing.T) {
	pl := &Plugin{}
	pl.coreShortcodes()
	pl.AddShortcode("box", func(attrs map[string]string, content string, _ ShortcodeContext) string {
		return `<div class="box ` + template.HTMLEscapeString(attrs["tone"]) + `">` + content + `</div>`
	})
	pl.AddShortcode("year", func(map[string]string, string, ShortcodeContext) string { return "ignored: first wins" })
	ctx := ShortcodeContext{Site: &Site{Name: "<b>Shop</b>"}, Now: time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC)}
	cases := map[string]string{
		"© [year] [site_name]":                        "© 1405 &lt;b&gt;Shop&lt;/b&gt;",
		`[box tone="info"]Hi [year][/box]`:            `<div class="box info">Hi 1405</div>`,
		`[box tone=&quot;x&quot;/]`:                   `<div class="box x"></div>`,
		`[BOX tone='warn']x[/box]`:                    `<div class="box warn">x</div>`,
		"[unknown a=1] [year/]":                       "[unknown a=1] 1405",
		`[box tone="a&quot; onclick=&quot;x"]y[/box]`: `<div class="box a">y</div>`,
		"no codes": "no codes",
	}
	for in, want := range cases {
		if got := pl.renderShortcodes(in, ctx); got != want {
			t.Errorf("%q → %q, want %q", in, got, want)
		}
	}
	// Never inside a tag: an attribute keeps its entities and its shortcodes.
	attr := `<a title="[box tone=&quot;x&quot; onmouseover=&quot;alert(1)&quot;]" href="/y">[year]</a>`
	if got := pl.renderShortcodes(attr, ctx); got != `<a title="[box tone=&quot;x&quot; onmouseover=&quot;alert(1)&quot;]" href="/y">1405</a>` {
		t.Errorf("attribute changed: %q", got)
	}
	// Wrapping HTML still works: the tags go back in place.
	if got := pl.renderShortcodes(`[box tone="i"]<p>a</p>[/box]`, ctx); got != `<div class="box i"><p>a</p></div>` {
		t.Errorf("wrapped HTML = %q", got)
	}
	ctx.Calendar = "gregorian"
	if got := pl.renderShortcodes("[year]", ctx); got != "2026" {
		t.Errorf("gregorian year = %q", got)
	}
	// Bounded: a shortcode producing itself stops after the passes.
	pl.AddShortcode("loop", func(map[string]string, string, ShortcodeContext) string { return "[loop]" })
	if got := pl.renderShortcodes("[loop]", ctx); got != "[loop]" {
		t.Errorf("loop = %q", got)
	}
	if got := StripShortcodes(`a [box tone="x"]b[/box] [year]`); got != "a b " {
		t.Errorf("strip = %q", got)
	}
}

func TestThemes(t *testing.T) {
	pl := &Plugin{Themes: []Theme{{ID: "custom", Name: "C", CSS: "body{color:red}", Templates: `{{define "page"}}<p class="custom">{{.Title}}</p>{{end}}`}}}
	if len(pl.themes()) != 3 || pl.themeByID("nope").ID != "default" || pl.themeByID("magazine").ID != "magazine" {
		t.Fatal("theme registry")
	}
	body, err := pl.renderView(layoutView{themeID: "custom", Page: &pageView{Title: "Hello"}})
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	if !strings.Contains(html, `<p class="custom">Hello</p>`) || !strings.Contains(html, "body{color:red}") || !strings.Contains(html, ".pb-hero") {
		t.Fatalf("custom theme not applied:\n%.600s", html)
	}
	body, _ = pl.renderView(layoutView{themeID: "magazine", Page: &pageView{Title: "M", ShowHeader: true}})
	if !strings.Contains(string(body), "--accent:#b4232a") {
		t.Fatal("magazine CSS")
	}
	for _, bad := range []string{"a{}</style><script>x</script>", "x<y"} {
		if _, msg := customCSS(bad); msg == "" {
			t.Errorf("custom CSS %q accepted", bad)
		}
	}
	for _, bad := range []string{"x</script><b>", "<!-- x", "</SCRIPT>"} {
		if _, msg := customJS(bad); msg == "" {
			t.Errorf("custom JS %q accepted", bad)
		}
	}
	if v, msg := customJS("console.log(1 < 2)"); msg != "" || v == "" {
		t.Error("plain JS refused")
	}
	body, _ = pl.renderView(layoutView{CustomCSS: "a{color:blue}", CustomJS: "var a = 1;", Page: &pageView{}})
	if !strings.Contains(string(body), "<style>a{color:blue}</style>") || !strings.Contains(string(body), "<script>var a = 1;</script>") {
		t.Fatal("custom code not rendered")
	}
	_ = mks.SanitizeHTML
}
