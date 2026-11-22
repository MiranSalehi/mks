package cms

import (
	"html"
	"html/template"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// MKSine's default-theme blocks ("sections"): a domain hero, a finance showcase, a
// services trio, featured domains, a features grid, a testimonials grid and a feed of
// the site's approved comments. Their data keeps MKSine's field names, so pages move
// over as they are. Gradients and icons are names from fixed lists (MKSine stored
// Tailwind classes and Lucide icon names); they map to CSS and SVG here, so no saved
// text reaches a style or markup.

// ThemeGradients are the card gradients (MKSine's Tailwind class pairs).
var ThemeGradients = []string{
	"from-blue-500 to-blue-600", "from-emerald-500 to-emerald-600", "from-violet-500 to-violet-600",
	"from-yellow-500 to-yellow-600", "from-purple-500 to-purple-600", "from-pink-500 to-pink-600",
	"from-violet-500 to-purple-600", "from-indigo-500 to-blue-600", "from-fuchsia-500 to-pink-600",
	"from-amber-500 to-orange-600", "from-emerald-500 to-teal-600", "from-sky-500 to-indigo-600",
}

var gradientCSS = map[string]template.CSS{
	"from-blue-500 to-blue-600":       "--g1:#3b82f6;--g2:#2563eb",
	"from-emerald-500 to-emerald-600": "--g1:#10b981;--g2:#059669",
	"from-violet-500 to-violet-600":   "--g1:#8b5cf6;--g2:#7c3aed",
	"from-yellow-500 to-yellow-600":   "--g1:#eab308;--g2:#ca8a04",
	"from-purple-500 to-purple-600":   "--g1:#a855f7;--g2:#9333ea",
	"from-pink-500 to-pink-600":       "--g1:#ec4899;--g2:#db2777",
	"from-violet-500 to-purple-600":   "--g1:#8b5cf6;--g2:#9333ea",
	"from-indigo-500 to-blue-600":     "--g1:#6366f1;--g2:#2563eb",
	"from-fuchsia-500 to-pink-600":    "--g1:#d946ef;--g2:#db2777",
	"from-amber-500 to-orange-600":    "--g1:#f59e0b;--g2:#ea580c",
	"from-emerald-500 to-teal-600":    "--g1:#10b981;--g2:#0d9488",
	"from-sky-500 to-indigo-600":      "--g1:#0ea5e9;--g2:#4f46e5",
}

// ThemeIcons are the card icons (Lucide names, as in MKSine).
var ThemeIcons = []string{"users", "calendar", "chart-column", "clipboard-list", "credit-card", "settings",
	"heart", "shield", "star", "bell", "globe", "phone"}

var iconPaths = map[string]string{
	"users":          `<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75"/>`,
	"calendar":       `<rect x="3" y="4" width="18" height="18" rx="2"/><path d="M16 2v4M8 2v4M3 10h18"/>`,
	"chart-column":   `<path d="M3 3v18h18"/><path d="M18 17V9M13 17V5M8 17v-3"/>`,
	"clipboard-list": `<rect x="8" y="2" width="8" height="4" rx="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2M12 11h4M12 16h4M8 11h.01M8 16h.01"/>`,
	"credit-card":    `<rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/>`,
	"settings":       `<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.7 1.7 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.8-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.8.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.8 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.8l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.8.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.8-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.8V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1z"/>`,
	"heart":          `<path d="M19 14c1.5-1.5 3-3.2 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.8 0-3 .5-4.5 2-1.5-1.5-2.7-2-4.5-2A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4 3 5.5l7 7z"/>`,
	"shield":         `<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>`,
	"star":           `<path d="m12 2 3.1 6.3 6.9 1-5 4.9 1.2 6.8L12 17.8 5.8 21l1.2-6.8-5-4.9 6.9-1z"/>`,
	"bell":           `<path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9M10.3 21a1.9 1.9 0 0 0 3.4 0"/>`,
	"globe":          `<circle cx="12" cy="12" r="10"/><path d="M2 12h20M12 2a15.3 15.3 0 0 1 4 10 15.3 15.3 0 0 1-4 10 15.3 15.3 0 0 1-4-10 15.3 15.3 0 0 1 4-10z"/>`,
	"phone":          `<path d="M22 16.9v3a2 2 0 0 1-2.2 2 19.8 19.8 0 0 1-8.6-3.1 19.5 19.5 0 0 1-6-6A19.8 19.8 0 0 1 2.1 4.2 2 2 0 0 1 4.1 2h3a2 2 0 0 1 2 1.7c.1.9.4 1.8.7 2.7a2 2 0 0 1-.5 2.1L8 9.8a16 16 0 0 0 6 6l1.3-1.3a2 2 0 0 1 2.1-.4c.9.3 1.8.6 2.7.7a2 2 0 0 1 1.7 2z"/>`,
}

// themeIcon is a fixed SVG for an icon name ("" for an unknown one).
func themeIcon(name string) template.HTML {
	p, ok := iconPaths[name]
	if !ok {
		return ""
	}
	return template.HTML(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">` + p + `</svg>`) //nolint:gosec -- constant markup
}

// highlight escapes text and wraps the first occurrence of phrase in a span.
func highlight(text, phrase, class string) template.HTML {
	i := -1
	if phrase != "" {
		i = strings.Index(text, phrase)
	}
	if i < 0 {
		return template.HTML(html.EscapeString(text)) //nolint:gosec -- escaped
	}
	return template.HTML(html.EscapeString(text[:i]) + `<span class="` + class + `">` + html.EscapeString(phrase) + `</span>` + html.EscapeString(text[i+len(phrase):])) //nolint:gosec -- escaped; class is a constant
}

// initial is the first letter of a name, for an avatar.
func initial(name string) string {
	r, _ := utf8.DecodeRuneInString(strings.TrimSpace(name))
	if r == utf8.RuneError {
		return "?"
	}
	return strings.ToUpper(string(r))
}

// feedItem is a comment of the comments feed.
type feedItem struct {
	Name    string
	Initial string
	Quote   string
	Date    string
	DateISO string
	Stars   string
	URL     template.URL
	Grad    template.CSS
}

// themeBlockView fills what the theme blocks need beyond their fields.
func (r *renderer) themeBlockView(v *blockView) error {
	d := v.D
	v.Hi = map[string]template.HTML{}
	switch v.Type {
	case "mksine_finance_showcase":
		v.Hi["heading"] = highlight(str(d, "heading"), str(d, "heading_accent"), "accent")
		v.Hi["paragraph_1"] = highlight(str(d, "paragraph_1"), str(d, "paragraph_1_highlight"), "mark")
		v.Hi["paragraph_2"] = highlight(str(d, "paragraph_2"), str(d, "paragraph_2_highlight"), "mark")
	case "mksine_clinic_features_grid", "mksine_testimonials_grid":
		for i := range v.Items {
			it := &v.Items[i]
			it.Grad = gradientCSS[str(it.D, "gradient")]
			it.Icon = themeIcon(str(it.D, "icon"))
			it.Initial = initial(str(it.D, "name"))
		}
	case "mksine_post_comments_feed":
		return r.commentsFeed(v)
	}
	return nil
}

// commentsFeed loads approved root comments of the site (of one post, or the latest
// across published posts), shortened to quote_max_chars.
func (r *renderer) commentsFeed(v *blockView) error {
	if r.req == nil || r.req.site == nil || r.pl.Orm == nil {
		return nil
	}
	d := v.D
	limit := intOf(d, "max_root_comments", 12)
	maxChars := intOf(d, "quote_max_chars", 220)
	now := r.req.now
	if now.IsZero() {
		now = time.Now()
	}
	q := r.pl.query().Model(&Comment{}).
		Join("JOIN "+PostsTable+" p ON p.id = "+CommentsTable+".post_id").
		Where(CommentsTable+".site_id = ? AND "+CommentsTable+".status = ? AND "+CommentsTable+".parent_id IS NULL", r.req.site.ID, "approved").
		Where("p.status = ? AND (p.published_at IS NULL OR p.published_at <= ?)", "published", now)
	if id := intOf(d, "post_id", 0); id > 0 {
		q = q.Where(CommentsTable+".post_id = ?", id)
	}
	var rows []struct {
		Comment
		PostSlug string
	}
	if err := q.Select(CommentsTable + ".*, p.slug AS post_slug").OrderByDesc(CommentsTable + ".created_at").Limit(limit).Find(&rows); err != nil {
		return err
	}
	for i, c := range rows {
		quote := c.Content
		if utf8.RuneCountInString(quote) > maxChars {
			quote = strings.TrimSpace(string([]rune(quote)[:maxChars])) + "…"
		}
		item := feedItem{Name: c.AuthorName, Initial: initial(c.AuthorName), Quote: quote,
			URL:  safeURL(r.pl.PostURL(r.req.site.Slug, c.PostSlug) + "#comment-" + strconv.FormatUint(uint64(c.ID), 10)),
			Grad: gradientCSS[ThemeGradients[i%len(ThemeGradients)]]}
		if c.Rating != nil && *c.Rating >= 1 && *c.Rating <= 5 {
			n := int(*c.Rating)
			item.Stars = strings.Repeat("★", n) + strings.Repeat("☆", 5-n)
		}
		if c.CreatedAt != nil {
			t := c.CreatedAt.StdTime()
			item.Date, item.DateISO = formatDate(&t, r.req.calendar())
		}
		v.Feed = append(v.Feed, item)
	}
	return nil
}

// themeBlockCSS styles the theme blocks (on top of the theme's CSS).
const themeBlockCSS = `.tb{--tb-accent:var(--accent);margin-block:2.5rem}.tb .accent{color:var(--tb-accent)}.tb .mark{background:linear-gradient(transparent 60%,rgba(8,126,114,.18) 0);padding-inline:.1em}.tb .badge{display:inline-block;padding:.2rem .8rem;border-radius:999px;background:#eaf5f1;color:var(--tb-accent);font-size:.8rem;font-weight:700;margin-bottom:.75rem}.tb h2{font-size:clamp(1.7rem,4vw,2.6rem);line-height:1.3;margin:0 0 .75rem}.tb .sub{color:var(--muted);margin:0 0 1.75rem}.tb-head{text-align:center;max-width:46rem;margin:0 auto 2rem}.tb-hero{position:relative;display:grid;grid-template-columns:1.1fr .9fr;gap:2rem;align-items:center;padding:clamp(1.5rem,5vw,3.5rem);border-radius:24px;background:#efeaff center/cover no-repeat;overflow:hidden}.tb-hero.img-left{grid-template-columns:.9fr 1.1fr}.tb-hero.img-left .art{order:-1}.tb-hero>.bgimg{position:absolute;inset:0;width:100%;height:100%;object-fit:cover;z-index:0}.tb-hero>*:not(.bgimg){position:relative}.tb-hero h2{font-size:clamp(2rem,5vw,3.2rem)}.tb-hero .art img{width:100%;height:auto;display:block}.tb-show{display:grid;grid-template-columns:1fr 1fr;gap:2.5rem;align-items:center}.tb-show p{font-size:1.08rem}.tb-show .trust{color:var(--muted);font-size:.9rem}.tb-show .social{display:flex;align-items:center;gap:.75rem;margin-top:1.5rem}.tb-show .avatars{display:flex}.tb-show .avatars span{display:grid;place-items:center;width:2.25rem;height:2.25rem;border-radius:999px;border:2px solid #fff;margin-inline-start:-.5rem;background:var(--tb-accent);color:#fff;font-weight:700;font-size:.85rem}.tb-show .avatars span:first-child{margin-inline-start:0}.tb-chart{background:#fff;border:1px solid var(--line);border-radius:20px;padding:1.25rem;box-shadow:0 18px 40px rgba(23,32,31,.1)}.tb-chart .bars{display:flex;align-items:end;gap:.5rem;height:10rem;margin-top:.75rem}.tb-chart .bars i{flex:1;border-radius:6px 6px 0 0;background:linear-gradient(180deg,var(--tb-accent),#3b6ea8)}.tb-chart small{color:var(--muted)}.tb-cards{display:grid;gap:1.25rem;grid-template-columns:repeat(auto-fit,minmax(15rem,1fr));list-style:none;padding:0;margin:0}.tb-card{background:#fff;border:1px solid var(--line);border-radius:18px;padding:1.4rem;display:flex;flex-direction:column;gap:.5rem}.tb-card img.thumb{width:3.5rem;height:3.5rem;object-fit:contain}.tb-card h3{margin:0;font-size:1.15rem}.tb-card p{margin:0;color:var(--muted);font-size:.95rem}.tb-card .more{margin-top:auto;font-weight:700;text-decoration:none}.tb-ico{display:grid;place-items:center;width:3rem;height:3rem;border-radius:14px;color:#fff;background:linear-gradient(135deg,var(--g1,#3b82f6),var(--g2,#2563eb))}.tb-ico svg{width:1.5rem;height:1.5rem}.tb-card a.stretch{color:inherit;text-decoration:none}.tb-domains .top{display:flex;flex-wrap:wrap;justify-content:space-between;align-items:baseline;gap:1rem;margin-bottom:1.25rem}.tb-domains .top h2{margin:0}.tb-dom{position:relative;text-align:center;align-items:center}.tb-dom .tld{font-size:1.6rem;font-weight:800;direction:ltr}.tb-dom .cat{color:var(--muted);font-size:.85rem}.tb-dom .price{font-size:1.2rem;font-weight:800;direction:ltr}.tb-dom del{color:var(--muted);font-size:.85rem;direction:ltr}.tb-dom .off{position:absolute;top:.75rem;inset-inline-end:.75rem;background:#dc2626;color:#fff;border-radius:999px;padding:.1rem .55rem;font-size:.75rem;font-weight:700}.tb-dom img{max-height:2.5rem;width:auto}.tb-quotes .layout{display:grid;grid-template-columns:2fr 1fr;gap:1.5rem;align-items:start}.tb-quote{background:#fff;border:1px solid var(--line);border-radius:18px;padding:1.25rem;margin:0}.tb-quote blockquote{margin:.75rem 0 0;font-size:.98rem}.tb-quote .who{display:flex;align-items:center;gap:.65rem}.tb-quote .ava{display:grid;place-items:center;width:2.6rem;height:2.6rem;border-radius:999px;color:#fff;font-weight:800;background:linear-gradient(135deg,var(--g1,#8b5cf6),var(--g2,#9333ea))}.tb-quote .who small{display:block;color:var(--muted);font-size:.8rem}.tb-quote .stars{color:#c98a00;font-size:.85rem}.tb-quote a{color:inherit;text-decoration:none}.tb-aside{background:linear-gradient(135deg,var(--tb-accent),#3b6ea8);color:#fff;border-radius:18px;padding:1.5rem;font-weight:600}@media(max-width:860px){.tb-hero,.tb-hero.img-left,.tb-show,.tb-quotes .layout{grid-template-columns:1fr}.tb-hero.img-left .art{order:0}}`

// themeBlockTemplates are the templates of the theme blocks.
const themeBlockTemplates = `
{{define "tb-dir"}}{{if and .D.text_direction (ne .D.text_direction "auto")}} dir="{{.D.text_direction}}"{{end}}{{end}}
{{define "block-mksine_hero_domain"}}<section class="pb tb tb-hero{{if eq .D.illustration_position "left"}} img-left{{end}} {{.Class}}"{{template "open" .}}>{{with .Img.background_image}}{{if .Src}}<img class="bgimg" src="{{.Src}}" alt="" loading="lazy">{{end}}{{end}}<div class="text"><h2>{{.D.heading_line1}}{{if or .D.heading_line2_prefix .D.heading_accent .D.heading_after}}<br>{{.D.heading_line2_prefix}} <span class="accent">{{.D.heading_accent}}</span> {{.D.heading_after}}{{end}}</h2>{{if .D.subheading}}<p class="sub">{{.D.subheading}}</p>{{end}}{{if .D.cta_label}}<div class="btns"><a class="btn btn-primary btn-lg" href="{{.URL.cta_url}}">{{.D.cta_label}}</a></div>{{end}}</div>{{with .Img.illustration}}{{if .Src}}<div class="art">{{template "picture" ((.WithAlt $.D.illustration_alt).Sized "(max-width: 860px) 100vw, 40vw" "" true)}}</div>{{end}}{{end}}</section>{{end}}
{{define "block-mksine_finance_showcase"}}<section class="pb tb tb-show {{.Class}}"{{template "open" .}}><div><h2>{{.Hi.heading}}</h2>{{if .D.paragraph_1}}<p>{{.Hi.paragraph_1}}</p>{{end}}{{if .D.paragraph_2}}<p>{{.Hi.paragraph_2}}</p>{{end}}<div class="btns">{{if .D.cta_primary_label}}<a class="btn btn-primary" href="{{.URL.cta_primary_url}}">{{.D.cta_primary_label}}</a>{{end}}{{if .D.cta_secondary_label}}<a class="btn btn-outline" href="{{.URL.cta_secondary_url}}">{{.D.cta_secondary_label}}</a>{{end}}</div>{{if .D.trust_line}}<p class="trust">{{.D.trust_line}}</p>{{end}}{{if or .Items .D.social_count}}<div class="social">{{if .Items}}<div class="avatars" aria-hidden="true">{{range .Items}}<span>{{.D.letter}}</span>{{end}}</div>{{end}}<div>{{if .D.social_count}}<strong>{{.D.social_count}}</strong>{{end}}{{if .D.social_caption}}<br><small>{{.D.social_caption}}</small>{{end}}</div></div>{{end}}</div><div class="tb-chart" aria-hidden="true"><small>{{.D.mock_chart_caption}}</small><div class="bars"><i style="height:35%"></i><i style="height:52%"></i><i style="height:44%"></i><i style="height:68%"></i><i style="height:60%"></i><i style="height:82%"></i><i style="height:95%"></i></div></div></section>{{end}}
{{define "block-mksine_services_trio"}}<section class="pb tb {{.Class}}"{{template "open" .}}>{{if .D.section_title}}<div class="tb-head"><h2>{{.D.section_title}}</h2></div>{{end}}<ul class="tb-cards">{{range .Items}}<li class="tb-card">{{if .Img.Src}}<img class="thumb" src="{{.Img.Src}}" alt="{{.D.image_alt}}" loading="lazy">{{end}}<h3>{{.D.title}}</h3>{{if .D.body}}<p>{{.D.body}}</p>{{end}}{{if .D.cta_label}}<a class="more" href="{{if .URL}}{{.URL}}{{else}}#{{end}}">{{.D.cta_label}} ←</a>{{end}}</li>{{end}}</ul></section>{{end}}
{{define "block-mksine_featured_domains"}}<section class="pb tb tb-domains {{.Class}}"{{template "open" .}}><div class="top">{{if .D.title}}<h2>{{.D.title}}</h2>{{end}}{{if .D.view_all_label}}<a href="{{.URL.view_all_url}}">{{.D.view_all_label}}</a>{{end}}</div><ul class="tb-cards">{{range .Items}}<li class="tb-card tb-dom">{{if .D.discount_percent}}<span class="off">−{{.D.discount_percent}}%</span>{{end}}{{if .Img.Src}}<img src="{{.Img.Src}}" alt="" loading="lazy">{{end}}{{if .URL}}<a class="stretch tld" href="{{.URL}}">{{.D.tld}}</a>{{else}}<span class="tld">{{.D.tld}}</span>{{end}}{{if .D.category}}<span class="cat">{{.D.category}}</span>{{end}}{{if .D.original_price}}<del>{{.D.original_price}}</del>{{end}}{{if .D.price}}<span class="price">{{.D.price}}</span>{{end}}</li>{{end}}</ul></section>{{end}}
{{define "block-mksine_clinic_features_grid"}}<section class="pb tb {{.Class}}"{{template "open" .}}{{template "tb-dir" .}}><div class="tb-head">{{if .D.badge}}<span class="badge">{{.D.badge}}</span>{{end}}<h2>{{.D.heading_prefix}} <span class="accent">{{.D.heading_accent}}</span></h2>{{if .D.subheading}}<p class="sub">{{.D.subheading}}</p>{{end}}</div><ul class="tb-cards">{{range .Items}}<li class="tb-card"><span class="tb-ico" style="{{.Grad}}">{{.Icon}}</span><h3>{{if .URL}}<a class="stretch" href="{{.URL}}"{{if .D.link_new_tab}} target="_blank" rel="noopener noreferrer"{{end}}>{{.D.title}}</a>{{else}}{{.D.title}}{{end}}</h3>{{if .D.body}}<p>{{.D.body}}</p>{{end}}</li>{{end}}</ul></section>{{end}}
{{define "tb-quote"}}<figure class="tb-quote" style="{{.Grad}}"><figcaption class="who"><span class="ava" aria-hidden="true">{{.Initial}}</span><span><strong>{{.Name}}</strong><small>{{.Sub}}</small></span></figcaption>{{if .Stars}}<span class="stars">{{.Stars}}</span>{{end}}<blockquote>{{if .URL}}<a href="{{.URL}}">{{.Quote}}</a>{{else}}{{.Quote}}{{end}}</blockquote></figure>{{end}}
{{define "block-mksine_testimonials_grid"}}<section class="pb tb tb-quotes {{.Class}}"{{template "open" .}}{{template "tb-dir" .}}><div class="tb-head">{{if .D.badge}}<span class="badge">{{.D.badge}}</span>{{end}}<h2>{{.D.title_prefix}} <span class="accent">{{.D.title_accent}}</span></h2>{{if .D.subheading}}<p class="sub">{{.D.subheading}}</p>{{end}}</div><div class="layout"><ul class="tb-cards">{{range .Items}}<li>{{template "tb-quote" (quoteOf .)}}</li>{{end}}</ul>{{if .D.aside}}<aside class="tb-aside">{{.D.aside}}</aside>{{end}}</div></section>{{end}}
{{define "block-mksine_post_comments_feed"}}{{if .Feed}}<section class="pb tb tb-quotes {{.Class}}"{{template "open" .}}{{template "tb-dir" .}}><div class="tb-head">{{if .D.badge}}<span class="badge">{{.D.badge}}</span>{{end}}<h2>{{.D.title_prefix}} <span class="accent">{{.D.title_accent}}</span></h2>{{if .D.subheading}}<p class="sub">{{.D.subheading}}</p>{{end}}</div><div class="layout"><ul class="tb-cards">{{range .Feed}}<li>{{template "tb-quote" (feedQuote .)}}</li>{{end}}</ul>{{if .D.aside}}<aside class="tb-aside">{{.D.aside}}</aside>{{end}}</div></section>{{end}}{{end}}
`

// quoteView is one card of the testimonials and comments blocks.
type quoteView struct {
	Name, Sub, Initial, Quote, Stars string
	URL                              template.URL
	Grad                             template.CSS
}

var themeFuncs = template.FuncMap{
	"quoteOf": func(it itemView) quoteView {
		return quoteView{Name: str(it.D, "name"), Sub: str(it.D, "city"), Initial: it.Initial, Quote: str(it.D, "quote"), URL: it.URL, Grad: it.Grad}
	},
	"feedQuote": func(f feedItem) quoteView {
		return quoteView{Name: f.Name, Sub: f.Date, Initial: f.Initial, Quote: f.Quote, Stars: f.Stars, URL: f.URL, Grad: f.Grad}
	},
}

func init() {
	template.Must(blockTemplate.Funcs(themeFuncs).Parse(themeBlockTemplates))
}
