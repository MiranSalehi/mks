package cms

import (
	"html/template"
	"strings"
	"sync"

	mks "github.com/MiranSalehi/mks"
)

// Theme is a look of the public site: its CSS, optionally its own templates for the
// views ({{define "page"}}, "post", "list", "terms", "comments", ...) parsed over the
// defaults, and its menu locations. Themes are Go code registered with the plugin
// or ZIP packages uploaded on the themes page (theme.json, templates/, assets/); a site
// activates one on the themes page.
type Theme struct {
	ID          string
	Name        string
	Description string
	// CSS replaces the default theme's CSS; the blocks' CSS is always added.
	CSS string
	// Templates override views, e.g. `{{define "post"}}...{{end}}`.
	Templates string
	// Locations are extra menu locations this theme shows (with Plugin.Locations).
	Locations []Location
	// Swatch is a color for the theme picker.
	Swatch string
	// Version, Author and Screenshot (an address) describe the theme on the themes page.
	Version    string
	Author     string
	Screenshot string
	// Requires are ids of ZIP plugins that must be active to use the theme.
	Requires []string
	// Scripts are addresses of JavaScript files added to every page (uploaded themes).
	Scripts []string
	// Uploaded: installed from a ZIP file (it can be updated, rolled back and deleted).
	Uploaded bool

	// The installed version and files of an uploaded theme (addresses depend on the site).
	pkgVersion    string
	pkgScreenshot string
	pkgJS         []string
}

// DefaultTheme is the built-in theme.
var DefaultTheme = Theme{ID: "default", Name: "پیش‌فرض", Description: "روشن و ساده، با رنگ سبز.", CSS: themeCSS, Swatch: "#087e72"}

// MagazineTheme is a second built-in theme: a darker header, serif titles and wider pages.
var MagazineTheme = Theme{
	ID: "magazine", Name: "مجله", Description: "سربرگ تیره، تیترهای درشت و فهرست کارت‌های بزرگ.", Swatch: "#1f2937",
	CSS: strings.NewReplacer(
		"--accent:#087e72", "--accent:#b4232a",
		"--paper:#fbfcfa", "--paper:#f7f5f2",
		".site-head{background:#fff;", ".site-head{background:#111827;",
		".brand{font-weight:800;color:var(--ink);", ".brand{font-weight:900;color:#fff;letter-spacing:.02em;",
		".nav a,.nav summary{color:var(--ink);", ".nav a,.nav summary{color:#e5e7eb;",
		".topline{height:5px;background:var(--accent)}", ".topline{height:3px;background:var(--accent)}",
		".title{font-size:clamp(2rem,6vw,3.6rem);", ".title{font-family:Georgia,'Times New Roman',serif;font-size:clamp(2.2rem,7vw,4.2rem);",
		".cards{display:grid;gap:1.25rem;grid-template-columns:repeat(auto-fill,minmax(15rem,1fr));", ".cards{display:grid;gap:2rem;grid-template-columns:repeat(auto-fill,minmax(20rem,1fr));",
	).Replace(themeCSS),
	Templates: `{{define "terms"}}<section><div class="eyebrow">فهرست</div><h1 class="title">{{.Heading}}</h1>{{if .Items}}<ul class="terms">{{range .Items}}<li style="padding-inline-start:{{.Depth}}.5rem"><a href="{{.URL}}">{{.Name}}</a>{{if .Count}}<span class="count">{{.Count}}</span>{{end}}</li>{{end}}</ul>{{else}}<p class="empty">موردی نیست.</p>{{end}}</section>{{end}}`,
}

// themes are the code themes and the uploaded ones (an uploaded theme never replaces a
// code theme).
func (pl *Plugin) themes() []Theme {
	list := pl.codeThemes()
	uploaded, _ := pl.loadedPackages()
	for _, t := range uploaded {
		if !hasThemeID(list, t.ID) {
			list = append(list, t)
		}
	}
	return list
}

func hasThemeID(list []Theme, id string) bool {
	for _, t := range list {
		if t.ID == id {
			return true
		}
	}
	return false
}

func (pl *Plugin) themeByID(id string) Theme {
	for _, t := range pl.themes() {
		if t.ID == id {
			if t.CSS == "" {
				t.CSS = themeCSS
			}
			return t
		}
	}
	return DefaultTheme
}

var themeTemplates sync.Map // theme id → *template.Template

// templateFor is the default template set with the theme's overrides, parsed once.
func (pl *Plugin) templateFor(t Theme) (*template.Template, error) {
	if strings.TrimSpace(t.Templates) == "" {
		return siteTemplate, nil
	}
	if cached, ok := themeTemplates.Load(t.ID + "\x00" + t.Templates); ok {
		return cached.(*template.Template), nil
	}
	// A fresh parse: html/template cannot clone a template that has already run.
	tpl, err := template.New("site").Parse(themeLayout + pictureTemplate)
	if err != nil {
		return nil, err
	}
	if _, err := tpl.Parse(t.Templates); err != nil {
		return nil, err
	}
	themeTemplates.Store(t.ID+"\x00"+t.Templates, tpl)
	return tpl, nil
}

// ManageCode: change a site's custom JavaScript (it runs on every page of the site). It
// needs Plugin.CustomJS too.
const ManageCode Ability = "cms.themes.code"

// canCode: may the user set custom JavaScript?
func (pl *Plugin) canCode(ctx *mks.HookContext) bool {
	return pl.CustomJS && pl.can(ctx, ManageCode)
}

// customCSS is safe to put in a <style> element: no markup at all.
func customCSS(v string) (string, string) {
	v = strings.TrimSpace(v)
	if len(v) > 50000 {
		return "", "حداکثر ۵۰٬۰۰۰ نویسه."
	}
	if strings.Contains(v, "<") {
		return "", "CSS نمی‌تواند نشانه‌گذاری (<) داشته باشد."
	}
	return v, ""
}

// customJS must not close its <script> element or open an HTML comment.
func customJS(v string) (string, string) {
	v = strings.TrimSpace(v)
	if len(v) > 50000 {
		return "", "حداکثر ۵۰٬۰۰۰ نویسه."
	}
	low := strings.ToLower(v)
	if strings.Contains(low, "</script") || strings.Contains(low, "<!--") || strings.Contains(low, "<script") {
		return "", "JavaScript نمی‌تواند تگ script یا توضیح HTML داشته باشد."
	}
	return v, ""
}
