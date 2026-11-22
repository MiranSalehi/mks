package cms

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"html/template"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// The built-in theme: one layout and a view per kind of public page. html/template escapes
// every value; HTML content arrives already sanitized as template.HTML, and addresses as
// template.URL only after SafeLinkURL.

type layoutView struct {
	SiteName    string
	Tagline     string
	Logo        template.URL
	Favicon     template.URL
	HomeURL     template.URL
	Title       string
	Description string
	Canonical   template.URL
	HeaderNav   []navLinkView
	// Nav is the menu of every location by key (HeaderNav and FooterNav are two of them).
	Nav       map[string][]navLinkView
	FooterNav []navLinkView
	Width     string

	Social *socialView
	// Script: the page has tabs or a slider.
	Script bool
	// Base, for the editor's preview: the iframe has no address, so relative URLs need it.
	Base template.URL
	// View names the kind of page for template hooks: home, page, single, posts,
	// category, tag, author, categories, tags.
	View string
	// Hooks are the template actions (AddAction) by hook name.
	Hooks map[string]template.HTML
	// CSS is the active theme's (plus the blocks'); CustomCSS and CustomJS the site's.
	CSS       template.CSS
	CustomCSS template.CSS
	CustomJS  template.JS
	// Scripts are the JavaScript files of the theme and the active plugins.
	Scripts []template.URL
	themeID string
	site    *Site

	// Bare: only the page body, without the site's header and footer (the builder canvas).
	Bare bool

	// AdminBar is shown to signed-in panel users of the site.
	AdminBar *adminBarView

	Page  *pageView
	Post  *postView
	List  *listView
	Terms *termsView
}

type linkView struct {
	Label string
	URL   template.URL
}

type pageView struct {
	Title      string
	ShowHeader bool
	Content    template.HTML
	Builder    template.HTML
	Gallery    []imageView
	Tags       []linkView
	Comments   *commentsView
}

type cardView struct {
	Title   string
	URL     template.URL
	Excerpt string
	Date    string
	DateISO string
	Image   imageView
}

// imageView is an image with its responsive copies and alt text from the media library.
type imageView struct {
	Src    template.URL
	Srcset string // copies in the original's format, for the img
	WebP   string // WebP copies, for a <source type="image/webp">
	Alt    string
	Width  int
	Height int
}

// pictureTemplate shows an image: WebP copies in a <source>, the original (with copies in
// its own format) as the img. Shared by the site layout and the block templates.
const pictureTemplate = `{{define "picture"}}{{if .WebP}}<picture><source type="image/webp" srcset="{{.WebP}}"{{with .Sizes}} sizes="{{.}}"{{end}}>{{end}}<img src="{{.Src}}"{{if .Srcset}} srcset="{{.Srcset}}"{{with .Sizes}} sizes="{{.}}"{{end}}{{end}} alt="{{.Alt}}"{{with .Class}} class="{{.}}"{{end}}{{if .Width}} width="{{.Width}}" height="{{.Height}}"{{end}}{{if .Lazy}} loading="lazy"{{end}}>{{if .WebP}}</picture>{{end}}{{end}}`

// Decorative is the image without alt text (backgrounds).
func (i imageView) Decorative() imageView { i.Alt = ""; return i }

// WithAlt is the image with this alt text.
func (i imageView) WithAlt(alt any) imageView {
	i.Alt = ""
	if alt != nil {
		i.Alt = fmt.Sprint(alt)
	}
	return i
}

// Sized is the image for the "picture" template: with sizes, a class and lazy loading.
func (i imageView) Sized(sizes, class string, lazy bool) sizedImage {
	return sizedImage{imageView: i, Sizes: sizes, Class: class, Lazy: lazy}
}

type sizedImage struct {
	imageView
	Sizes string
	Class string
	Lazy  bool
}

type postView struct {
	Title      string
	Date       string
	DateISO    string
	Author     string
	AuthorURL  template.URL
	Image      imageView
	Content    template.HTML
	Gallery    []imageView
	Categories []linkView
	Tags       []linkView
	Related    []cardView
	Comments   *commentsView
	Hooks      map[string]template.HTML
}

type pageLink struct {
	Label   string
	URL     template.URL
	Current bool
}

type listView struct {
	Kicker      string
	Heading     string
	Description template.HTML
	Items       []cardView
	Pages       []cardView
	Pagination  []pageLink
	Empty       string
}

type termView struct {
	Name  string
	URL   template.URL
	Count string
	Depth int
}

type termsView struct {
	Heading string
	Items   []termView
}

// navLinkView is a menu entry for the template.
type navLinkView struct {
	Label    string
	URL      template.URL
	NewTab   bool
	Current  bool
	Children []navLinkView
}

const themeCSS = `:root{color-scheme:light;--ink:#17201f;--muted:#687471;--line:#dce4e0;--accent:#087e72;--paper:#fbfcfa;--card:#fff}*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);font:18px/1.9 system-ui,-apple-system,"Segoe UI",sans-serif}a{color:var(--accent)}.topline{height:5px;background:var(--accent)}.wrap{width:min(calc(100% - 40px),72rem);margin:auto}main{width:min(calc(100% - 40px),var(--content-width,48rem));margin:clamp(40px,8vh,96px) auto}.site-head{background:#fff;border-bottom:1px solid var(--line)}.site-head .wrap{display:flex;flex-wrap:wrap;align-items:center;gap:.5rem 2rem;padding:.75rem 0}.brand{font-weight:800;color:var(--ink);text-decoration:none;font-size:1.05rem}.logo{display:block;height:2.25rem;width:auto}.nav,.nav ul,.foot-nav,.foot-nav ul{list-style:none;margin:0;padding:0}.nav{display:flex;flex-wrap:wrap;gap:.25rem 1.75rem;font-size:.95rem}.nav a,.nav summary{color:var(--ink);text-decoration:none;cursor:pointer}.nav a:hover,.nav summary:hover,.nav [aria-current=page],.nav .current>summary{color:var(--accent)}.nav [aria-current=page]{font-weight:700}.nav details{position:relative}.nav summary{list-style:none}.nav summary::-webkit-details-marker{display:none}.nav summary::after{content:"▾";margin-inline-start:.35rem;font-size:.75em}.nav details ul{position:absolute;inset-inline-start:-.7rem;top:calc(100% + .6rem);min-width:13rem;background:#fff;border:1px solid var(--line);border-radius:10px;padding:.4rem;box-shadow:0 12px 32px rgba(23,32,31,.1);z-index:10}.nav details a{display:block;padding:.35rem .7rem;border-radius:6px}.nav details details summary{display:block;padding:.35rem .7rem}.nav details details ul{position:static;min-width:0;border:0;box-shadow:none;padding:0 .75rem 0 0;padding-inline:.75rem 0;background:none}.foot-nav ul ul{padding-inline-start:.75rem}.nav details a:hover{background:#eef5f2}a:focus-visible,summary:focus-visible{outline:2px solid var(--accent);outline-offset:3px;border-radius:4px}.eyebrow{color:var(--accent);font-size:.8rem;font-weight:700;letter-spacing:.08em}.title{font-size:clamp(2rem,6vw,3.6rem);line-height:1.25;letter-spacing:-.03em;margin:.5rem 0 1.5rem}.meta{color:var(--muted);font-size:.9rem;display:flex;flex-wrap:wrap;gap:.25rem 1rem;margin-bottom:2rem}.meta a{color:inherit}.hero-wrap{margin:0 0 2rem}.hero{width:100%;height:auto;border-radius:16px;aspect-ratio:16/9;object-fit:cover}.content{font-size:1.05rem}.content h1,.content h2,.content h3{line-height:1.4}.content img{max-width:100%;height:auto;border-radius:12px}.content a{text-decoration-thickness:1px;text-underline-offset:4px}.content table{border-collapse:collapse;width:100%;margin:1.5rem 0}.content td,.content th{border:1px solid var(--line);padding:.4em .7em;vertical-align:top}.content th{background:#eef5f2}.content video,.content audio{max-width:100%}.content ul[data-type=taskList]{list-style:none;padding-inline-start:.25rem}.content ul[data-type=taskList] li{display:flex;gap:.5rem;align-items:baseline}.content mark{color:inherit;border-radius:2px}.content .marker-yellow{background:#fdfd77}.content .marker-green{background:#62f962}.content .marker-pink{background:#fc7899}.content .marker-blue{background:#72ccfd}.content .pen-red{background:none;color:#e71313}.content .pen-green{background:none;color:#128a00}.content .category{color:var(--muted);font-size:.85em;font-weight:700;letter-spacing:.08em;text-transform:uppercase}.content .document-title{font-size:1.75em;font-weight:800}.content .document-subtitle{color:var(--muted);font-weight:400}.content .info-box{border:1px solid #e91e63;border-radius:10px;background:#fdf0f4;padding:1em 1.25em}.content .side-quote{float:inline-end;width:40%;margin-inline-start:1.25em;border-radius:8px;background:#ebf8fd;padding:1em 1.25em;font-family:Georgia,serif}.content span.marker{background:yellow}.content .spoiler{background:#000;color:#000}.content .spoiler:hover{color:#fff}.content pre{overflow:auto;direction:ltr;text-align:left;background:#f3f5f4;border-radius:10px;padding:1rem}.content pre.fancy-code{box-shadow:0 4px 14px rgba(0,0,0,.2)}.content pre.fancy-code-dark{background:#272822;color:#fff}.content pre.fancy-code-bright{background:#dddfe0;color:#000}.block{margin:2rem 0}.block h2{font-size:1.7rem}.quote{border-inline-start:3px solid var(--accent);margin:2rem 0;padding:.1rem 1.25rem;color:#3d4a47}.quote footer{color:var(--muted);font-size:.88rem}.callout{padding:1rem 1.25rem;border-radius:10px;background:#eaf5f1}.callout.warning{background:#fff4dd}.chips{display:flex;flex-wrap:wrap;gap:.5rem;margin:2rem 0;padding:0;list-style:none}.chips a{display:inline-block;padding:.15rem .75rem;border-radius:999px;background:#eaf5f1;text-decoration:none;font-size:.85rem}.cards{display:grid;gap:1.25rem;grid-template-columns:repeat(auto-fill,minmax(15rem,1fr));margin:2rem 0;padding:0;list-style:none}.card{background:var(--card);border:1px solid var(--line);border-radius:14px;overflow:hidden;display:flex;flex-direction:column}.card img{width:100%;height:auto;aspect-ratio:16/9;object-fit:cover}.card div{padding:1rem 1.1rem 1.2rem}.card h3{margin:0 0 .35rem;font-size:1.1rem;line-height:1.5}.card h3 a{color:var(--ink);text-decoration:none}.card h3 a:hover{color:var(--accent)}.card p{margin:.25rem 0 0;color:var(--muted);font-size:.92rem;line-height:1.7}.card time{color:var(--muted);font-size:.8rem}.pager{display:flex;flex-wrap:wrap;gap:.4rem;list-style:none;padding:0;margin:2rem 0}.pager a{display:inline-block;min-width:2.4rem;text-align:center;padding:.2rem .6rem;border-radius:8px;border:1px solid var(--line);text-decoration:none}.pager [aria-current=page]{background:var(--accent);color:#fff;border-color:var(--accent)}.terms{list-style:none;padding:0}.terms li{padding:.35rem 0;border-bottom:1px solid var(--line)}.terms .count{color:var(--muted);font-size:.85rem;margin-inline-start:.5rem}.admin-bar{position:sticky;top:0;z-index:100;display:flex;flex-wrap:wrap;align-items:center;gap:.25rem 1.25rem;padding:.4rem 1rem;background:#1d2422;color:#fff;font-size:.82rem;line-height:1.6}.admin-bar a{color:#fff;text-decoration:none;opacity:.9}.admin-bar a:hover{opacity:1;text-decoration:underline}.admin-bar .who{margin-inline-start:auto;opacity:.7}.related h2{font-size:1.3rem;margin-top:3rem}.gallery{display:grid;gap:.6rem;grid-template-columns:repeat(auto-fill,minmax(10rem,1fr));list-style:none;padding:0;margin:2rem 0}.gallery img{display:block;width:100%;height:auto;aspect-ratio:1;object-fit:cover;border-radius:10px}.empty{color:var(--muted)}.site-foot{border-top:1px solid var(--line)}.site-foot .wrap{padding:2rem 0 2.5rem;display:flex;flex-wrap:wrap;justify-content:space-between;gap:1rem 3rem}.foot-nav{display:flex;flex-wrap:wrap;gap:1rem 3rem;font-size:.9rem}.foot-nav a{color:var(--muted);text-decoration:none}.foot-nav a:hover,.foot-nav [aria-current=page]{color:var(--accent)}.foot-nav .group{display:block;font-weight:700;margin-bottom:.4rem}.foot-nav ul{display:grid;gap:.3rem}.copy{color:var(--muted);font-size:.8rem}.comments{margin-top:3.5rem;border-top:1px solid var(--line);padding-top:1rem}.comments h2{font-size:1.3rem}.avg{color:var(--muted);font-size:.9rem}.thread{margin:1.25rem 0;padding-bottom:1rem;border-bottom:1px solid var(--line)}.comment header{display:flex;flex-wrap:wrap;gap:.25rem 1rem;align-items:baseline;font-size:.9rem}.comment time{color:var(--muted);font-size:.8rem}.stars{color:#c98a00;letter-spacing:.1em}.comment p{margin:.35rem 0;white-space:pre-line}.reply{margin-inline-start:1.5rem;padding-inline-start:1rem;border-inline-start:2px solid var(--line)}.reply-link{font-size:.85rem}.notice{background:#eaf5f1;padding:.75rem 1rem;border-radius:10px}.error{background:#fdecec;color:#8a1c1c;padding:.75rem 1rem;border-radius:10px}.comment-form{display:grid;gap:.75rem;grid-template-columns:repeat(auto-fit,minmax(14rem,1fr));margin-top:2rem}.comment-form h3,.comment-form .wide,.comment-form p,.comment-form button{grid-column:1/-1}.comment-form label{display:grid;gap:.25rem;font-size:.9rem}.comment-form input,.comment-form select,.comment-form textarea{font:inherit;font-size:1rem;padding:.55rem .75rem;border:1px solid var(--line);border-radius:10px;background:#fff}.comment-form button{justify-self:start;font:inherit;background:var(--accent);color:#fff;border:0;border-radius:10px;padding:.55rem 1.4rem;cursor:pointer}.hp{position:absolute;width:1px;height:1px;margin:-1px;padding:0;border:0;overflow:hidden;clip:rect(0 0 0 0);clip-path:inset(50%);white-space:nowrap}@media(max-width:600px){body{font-size:16px}main,.wrap{width:calc(100% - 28px)}}`

const themeLayout = `<!doctype html>
<html lang="fa" dir="rtl">
<head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">{{if .Base}}<base href="{{.Base}}">{{end}}
<title>{{.Title}}</title>{{if .Description}}<meta name="description" content="{{.Description}}">{{end}}
<link rel="canonical" href="{{.Canonical}}">{{if .Favicon}}<link rel="icon" href="{{.Favicon}}">{{end}}
{{with .Social}}<meta property="og:type" content="{{.Type}}"><meta property="og:title" content="{{$.Title}}">{{if .Description}}<meta property="og:description" content="{{.Description}}">{{end}}{{if .URL}}<meta property="og:url" content="{{.URL}}">{{end}}<meta property="og:site_name" content="{{.SiteName}}"><meta property="og:locale" content="fa_IR">{{if .Image}}<meta property="og:image" content="{{.Image}}"><meta name="twitter:card" content="summary_large_image">{{else}}<meta name="twitter:card" content="summary">{{end}}{{if .Published}}<meta property="article:published_time" content="{{.Published}}">{{end}}{{if .JSONLD}}<script type="application/ld+json">{{.JSONLD}}</script>{{end}}{{end}}
<style>{{.CSS}}</style>{{if .CustomCSS}}<style>{{.CustomCSS}}</style>{{end}}</head><body>{{with .AdminBar}}<nav class="admin-bar" aria-label="Admin bar">{{range .Items}}<a href="{{.URL}}">{{.Label}}</a>{{end}}{{if .User}}<span class="who">{{.User}}</span>{{end}}</nav>{{end}}{{index .Hooks "layout.body_start"}}<div class="topline"></div>
{{if not .Bare}}<header class="site-head"><div class="wrap"><a class="brand" href="{{.HomeURL}}">{{if .Logo}}<img src="{{.Logo}}" alt="{{.SiteName}}" class="logo">{{else}}{{.SiteName}}{{end}}</a>{{if .HeaderNav}}<nav aria-label="Main menu"><ul class="nav">{{range .HeaderNav}}<li>{{template "navitem" .}}</li>{{end}}</ul></nav>{{end}}</div></header>{{end}}
<main style="--content-width:{{.Width}}">
{{index .Hooks (printf "%s.before_content" .View)}}{{with .Page}}{{template "page" .}}{{end}}{{with .Post}}{{template "post" .}}{{end}}{{with .List}}{{template "list" .}}{{end}}{{with .Terms}}{{template "terms" .}}{{end}}{{index .Hooks (printf "%s.after_content" .View)}}
</main>
{{if not .Bare}}<footer class="site-foot"><div class="wrap">{{if .FooterNav}}<nav aria-label="Footer menu"><ul class="foot-nav">{{range .FooterNav}}<li>{{template "footitem" .}}</li>{{end}}</ul></nav>{{end}}<div class="copy">{{.SiteName}}{{if .Tagline}} — {{.Tagline}}{{end}}</div></div></footer>{{end}}
{{if .Script}}<script>` + blockScript + `</script>{{end}}{{range .Scripts}}<script src="{{.}}" defer></script>{{end}}{{if .CustomJS}}<script>{{.CustomJS}}</script>{{end}}</body></html>
{{define "navitem"}}{{if .Children}}<details{{if .Current}} class="current"{{end}}><summary>{{.Label}}</summary><ul>{{range .Children}}<li>{{template "navitem" .}}</li>{{end}}</ul></details>{{else}}{{template "link" .}}{{end}}{{end}}
{{define "footitem"}}{{if .Children}}<span class="group">{{.Label}}</span><ul>{{range .Children}}<li>{{template "footitem" .}}</li>{{end}}</ul>{{else}}{{template "link" .}}{{end}}{{end}}
{{define "link"}}<a href="{{.URL}}"{{if .NewTab}} target="_blank" rel="noopener noreferrer"{{end}}{{if .Current}} aria-current="page"{{end}}>{{.Label}}</a>{{end}}
{{define "chips"}}{{if .}}<ul class="chips">{{range .}}<li><a href="{{.URL}}">{{.Label}}</a></li>{{end}}</ul>{{end}}{{end}}
{{define "img"}}<img src="{{.Src}}"{{if .Srcset}} srcset="{{.Srcset}}"{{end}} alt="{{.Alt}}"{{if .Width}} width="{{.Width}}" height="{{.Height}}"{{end}}{{end}}
{{define "gallery"}}{{if .}}<ul class="gallery">{{range .}}<li>{{template "picture" (.Sized "(max-width: 600px) 50vw, 16rem" "" true)}}</li>{{end}}</ul>{{end}}{{end}}
{{define "cards"}}<ul class="cards">{{range .}}<li class="card">{{if .Image.Src}}{{template "picture" (.Image.Sized "(max-width: 600px) 100vw, 22rem" "" true)}}{{end}}<div>{{if .Date}}<time datetime="{{.DateISO}}">{{.Date}}</time>{{end}}<h3><a href="{{.URL}}">{{.Title}}</a></h3>{{if .Excerpt}}<p>{{.Excerpt}}</p>{{end}}</div></li>{{end}}</ul>{{end}}
{{define "page"}}<article>{{if .ShowHeader}}<div class="eyebrow">Page</div><h1 class="title">{{.Title}}</h1>{{end}}
{{if .Content}}<div class="content">{{.Content}}</div>{{end}}
{{if .Builder}}<div class="builder">{{.Builder}}</div>{{end}}
{{template "gallery" .Gallery}}{{template "chips" .Tags}}{{with .Comments}}{{template "comments" .}}{{end}}</article>{{end}}
{{define "comment"}}<article class="comment" id="comment-{{.ID}}"><header><strong>{{.Author}}</strong>{{if .Date}}<time datetime="{{.DateISO}}">{{.Date}}</time>{{end}}{{if .Stars}}<span class="stars" aria-label="Rating">{{.Stars}}</span>{{end}}</header><p>{{.Content}}</p></article>{{end}}
{{define "comments"}}<section class="comments" id="comments"><h2>Comments ({{.Count}})</h2>{{if .Average}}<p class="avg">Average rating: {{.Average}}</p>{{end}}
{{if .Notice}}<p class="notice" role="status">{{.Notice}}</p>{{end}}
{{range .Items}}<div class="thread">{{template "comment" .}}{{range .Replies}}<div class="reply">{{template "comment" .}}</div>{{end}}<a class="reply-link" href="{{$.FormLink}}?reply={{.ID}}#comment-form">Reply</a></div>{{else}}<p class="empty">No comments yet.</p>{{end}}
<form class="comment-form" id="comment-form" method="post" action="{{.Action}}"><h3>{{if .ReplyTo}}Reply to {{.ReplyTo.Author}}{{else}}Write your comment{{end}}</h3>
{{if .Error}}<p class="error" role="alert">{{.Error}}</p>{{end}}
<input type="hidden" name="target" value="{{.Target}}">{{if .ReplyTo}}<input type="hidden" name="parent_id" value="{{.ReplyTo.ID}}">{{end}}
<div class="hp" aria-hidden="true"><label>Website <input name="website" tabindex="-1" autocomplete="off"></label></div>
<label>Name<input name="name" required maxlength="100" autocomplete="name"></label>
<label>Email (not published)<input name="email" type="email" required maxlength="255" autocomplete="email" dir="ltr"></label>
{{if not .ReplyTo}}<label>Rating<select name="rating"><option value="">No rating</option><option value="5">5 — Excellent</option><option value="4">4</option><option value="3">3</option><option value="2">2</option><option value="1">1</option></select></label>{{end}}
<label class="wide">Comment<textarea name="content" required minlength="3" maxlength="5000" rows="5"></textarea></label>
<button type="submit">Post comment</button></form></section>{{end}}
{{define "post"}}<article><h1 class="title">{{.Title}}</h1>
<div class="meta">{{if .Date}}<time datetime="{{.DateISO}}">{{.Date}}</time>{{end}}{{if .Author}}{{if .AuthorURL}}<a href="{{.AuthorURL}}">{{.Author}}</a>{{else}}<span>{{.Author}}</span>{{end}}{{end}}{{range .Categories}}<a href="{{.URL}}">{{.Label}}</a>{{end}}</div>
{{if .Image.Src}}<figure class="hero-wrap">{{template "picture" (.Image.Sized "(max-width: 800px) 100vw, 48rem" "hero" false)}}</figure>{{end}}
<div class="content">{{.Content}}</div>
{{template "gallery" .Gallery}}{{template "chips" .Tags}}
{{index .Hooks "single.after_article"}}{{if .Related}}{{index .Hooks "single.before_related"}}<section class="related"><h2>Related posts</h2>{{template "cards" .Related}}</section>{{index .Hooks "single.after_related"}}{{end}}{{with .Comments}}{{template "comments" .}}{{end}}</article>{{end}}
{{define "list"}}<section>{{if .Kicker}}<div class="eyebrow">{{.Kicker}}</div>{{end}}<h1 class="title">{{.Heading}}</h1>
{{if .Description}}<div class="content">{{.Description}}</div>{{end}}
{{if .Items}}{{template "cards" .Items}}{{else}}<p class="empty">{{.Empty}}</p>{{end}}
{{if .Pagination}}<nav aria-label="Pages"><ul class="pager">{{range .Pagination}}<li><a href="{{.URL}}"{{if .Current}} aria-current="page"{{end}}>{{.Label}}</a></li>{{end}}</ul></nav>{{end}}
{{if .Pages}}<h2>Pages</h2>{{template "cards" .Pages}}{{end}}</section>{{end}}
{{define "terms"}}<section><h1 class="title">{{.Heading}}</h1>{{if .Items}}<ul class="terms">{{range .Items}}<li style="padding-inline-start:{{.Depth}}.5rem"><a href="{{.URL}}">{{.Name}}</a>{{if .Count}}<span class="count">{{.Count}}</span>{{end}}</li>{{end}}</ul>{{else}}<p class="empty">Nothing here.</p>{{end}}</section>{{end}}`

var siteTemplate = template.Must(template.Must(template.New("site").Parse(themeLayout)).Parse(pictureTemplate))

func (pl *Plugin) renderView(view layoutView) ([]byte, error) {
	if view.Width == "" {
		view.Width = "48rem"
	}
	if view.View == "" {
		view.View = "page"
	}
	theme := pl.themeForSite(pl.themeByID(view.themeID), view.site)
	if view.CSS == "" {
		css, scripts := pl.pluginAssets(view.site)
		view.CSS = template.CSS(theme.CSS + blockCSS + themeBlockCSS + css) //nolint:gosec -- the theme's and plugins' CSS (installed by administrators)
		for _, u := range append(append([]string{}, theme.Scripts...), scripts...) {
			view.Scripts = append(view.Scripts, template.URL(u)) //nolint:gosec -- addresses of installed package files
		}
	}
	tpl, err := pl.templateFor(theme)
	if err != nil {
		return nil, err
	}
	var body bytes.Buffer
	if view.Post != nil && view.Post.Hooks == nil {
		view.Post.Hooks = view.Hooks
	}
	if err := tpl.Execute(&body, view); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

// safeURL turns an address into a template.URL, or "" when it is not a safe link.
func safeURL(u string) template.URL {
	if !SafeLinkURL(u) {
		return ""
	}
	return template.URL(u) //nolint:gosec -- checked by SafeLinkURL
}

// allNavViews are every location's menu, for themes with locations of their own:
// {{index .Nav "sidebar"}}.
func allNavViews(nav siteNav) map[string][]navLinkView {
	out := map[string][]navLinkView{}
	for key, items := range nav {
		out[key] = navViews(items)
	}
	return out
}

// navViews prepares menu entries for the template, dropping unsafe addresses.
func navViews(items []NavItem) []navLinkView {
	var out []navLinkView
	for _, item := range items {
		view := navLinkView{Label: item.Label, NewTab: item.NewTab, Current: item.Current}
		if len(item.Children) > 0 {
			view.Children = navViews(item.Children)
			if len(view.Children) == 0 {
				continue
			}
		} else {
			if view.URL = safeURL(item.URL); view.URL == "" {
				continue
			}
		}
		out = append(out, view)
	}
	return out
}

var tagsRE = regexp.MustCompile(`<[^>]*>`)

// plainText strips markup and cuts to at most n characters, for descriptions and excerpts.
func plainText(html string, n int) string {
	text := strings.Join(strings.Fields(stdhtml.UnescapeString(tagsRE.ReplaceAllString(html, " "))), " ")
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	r := []rune(text)
	return strings.TrimSpace(string(r[:n])) + "…"
}

var jalaliMonths = []string{"Farvardin", "Ordibehesht", "Khordad", "Tir", "Mordad", "Shahrivar", "Mehr", "Aban", "Azar", "Dey", "Bahman", "Esfand"}

// toJalali converts a Gregorian date to the Jalali (Solar Hijri) calendar.
func toJalali(gy, gm, gd int) (int, int, int) {
	gDays := []int{0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334}
	gy2 := gy
	if gm > 2 {
		gy2 = gy + 1
	}
	days := 355666 + 365*gy + (gy2+3)/4 - (gy2+99)/100 + (gy2+399)/400 + gd + gDays[gm-1]
	jy := -1595 + 33*(days/12053)
	days %= 12053
	jy += 4 * (days / 1461)
	days %= 1461
	if days > 365 {
		jy += (days - 1) / 365
		days = (days - 1) % 365
	}
	var jm, jd int
	if days < 186 {
		jm, jd = 1+days/31, 1+days%31
	} else {
		jm, jd = 7+(days-186)/30, 1+(days-186)%30
	}
	return jy, jm, jd
}

// formatDate is a date for readers: Jalali ("30 Aban 1405"), or with
// the site's date_calendar setting "gregorian", MKSine's Y-m-d ("2026-11-21").
func formatDate(t *time.Time, calendar ...string) (string, string) {
	if t == nil || t.IsZero() {
		return "", ""
	}
	if len(calendar) > 0 && calendar[0] == "gregorian" {
		return t.Format("2006-01-02"), t.Format(time.RFC3339)
	}
	jy, jm, jd := toJalali(t.Year(), int(t.Month()), t.Day())
	return fmt.Sprintf("%d %s %d", jd, jalaliMonths[jm-1], jy), t.Format(time.RFC3339)
}
