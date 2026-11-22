package cms

import (
	"bytes"
	"fmt"
	"html"
	"html/template"
	"strconv"
	"strings"

	mks "github.com/MiranSalehi/mks"
)

// blockView is what a block template sees.
type blockView struct {
	ID       string
	Type     string
	D        map[string]any
	Class    string
	Style    template.CSS
	Anchor   string
	HTML     map[string]template.HTML // sanitized HTML fields
	Img      map[string]imageView     // media fields
	Src      map[string]template.URL  // media files (video, audio)
	URL      map[string]template.URL  // links
	Items    []itemView               // list fields (features, slides, tabs, items, testimonials)
	Children []template.HTML          // rendered slots
	Cols     template.CSS             // grid template of columns
	Spans    []template.CSS           // grid: each column's spans (CSS variables), when set
	Hi       map[string]template.HTML // theme blocks: text with a highlighted phrase
	Feed     []feedItem               // the comments feed block
	Inner    template.HTML            // a plugin block: its template's output
}

type itemView struct {
	D     map[string]any
	HTML  template.HTML
	Img   imageView
	URL   template.URL
	ID    string
	Stars string
	// Theme blocks: the card's gradient (fixed CSS), icon (fixed SVG) and initial.
	Grad    template.CSS
	Icon    template.HTML
	Initial string
}

// renderer renders the blocks of one page.
type renderer struct {
	pl       *Plugin
	req      *request
	media    mediaSet
	sections map[uint][]Block
	seen     map[uint]bool // section ids being expanded (loop detection)
	n        int
	script   bool
	// preview marks each block with its place in the form (data-pb-path), so a click in
	// the editor's preview can open that block. inSection > 0: inside a reusable section,
	// whose blocks are not the form's.
	preview   bool
	inSection int
	// canvas draws the editor's chrome (MKSine's builder canvas): each block in a frame
	// with a bar (its name from labels and its tools) and an insert row between blocks.
	canvas bool
	labels map[string]string
}

func (r *renderer) nextID() string {
	r.n++
	return "b" + strconv.Itoa(r.n)
}

func str(d map[string]any, k string) string {
	s, _ := d[k].(string)
	return s
}

func boolOf(d map[string]any, k string) bool {
	b, _ := d[k].(bool)
	return b
}

func intOf(d map[string]any, k string, def int) int {
	switch v := d[k].(type) {
	case int:
		return v
	case float64:
		return int(v)
	}
	return def
}

// presentation turns the shared style settings into classes and an inline style.
func presentation(d map[string]any) (string, template.CSS) {
	classes := []string{}
	for _, k := range []string{"spacing", "padding", "background", "align", "width", "border_width", "border_radius", "visibility"} {
		if v := str(d, k); v != "" {
			classes = append(classes, k+"-"+v)
		}
	}
	var style []string
	for prop, key := range map[string]string{"color": "text_color", "background-color": "background_color", "border-color": "border_color"} {
		if v := str(d, key); colorRE.MatchString(v) {
			style = append(style, prop+":"+v)
		}
	}
	// Map order is random; keep the attribute stable.
	sortStrings(style)
	return strings.Join(classes, " "), template.CSS(strings.Join(style, ";")) //nolint:gosec -- colors checked by colorRE
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// columnTemplate is the grid-template-columns of a columns block.
func columnTemplate(layout string, n int) template.CSS {
	presets := map[string]string{"1-2": "1fr 2fr", "2-1": "2fr 1fr", "1-3": "1fr 3fr", "3-1": "3fr 1fr", "1-2-1": "1fr 2fr 1fr"}
	if p, ok := presets[layout]; ok && strings.Count(p, "fr") == n {
		return template.CSS("grid-template-columns:" + p)
	}
	return template.CSS("grid-template-columns:repeat(" + strconv.Itoa(max(1, n)) + ",minmax(0,1fr))")
}

// collectMedia gathers every media path of a payload (and its sections) for one query.
func (pl *Plugin) collectMedia(blocks []Block, out *[]*string) {
	for _, b := range blocks {
		spec, _ := pl.blockSpec(b.Type)
		for _, f := range spec.Fields {
			switch f.Kind {
			case fMedia:
				if s := str(b.Data, f.Name); s != "" {
					*out = append(*out, &s)
				}
			case fList:
				items, _ := b.Data[f.Name].([]any)
				for _, it := range items {
					m, _ := it.(map[string]any)
					for _, inner := range f.Item {
						if inner.Kind == fMedia {
							if s := str(m, inner.Name); s != "" {
								*out = append(*out, &s)
							}
						}
					}
				}
			}
		}
		for _, slot := range slotsOf(b) {
			pl.collectMedia(slot, out)
		}
	}
}

// slotsOf are the child block lists of a container.
func slotsOf(b Block) [][]Block {
	slots, _ := b.Data["children"].([]any)
	out := make([][]Block, 0, len(slots))
	for _, s := range slots {
		obj, _ := s.(map[string]any)
		switch inner := obj["blocks"].(type) {
		case []Block:
			out = append(out, inner)
		case []any:
			var list []Block
			for _, x := range inner {
				if m, ok := x.(map[string]any); ok {
					t, _ := m["type"].(string)
					d, _ := m["data"].(map[string]any)
					list = append(list, Block{Type: t, Data: d})
				}
			}
			out = append(out, list)
		default:
			out = append(out, nil)
		}
	}
	return out
}

func (r *renderer) blocks(blocks []Block, depth int, prefix string) (template.HTML, error) {
	if r.canvas && r.inSection == 0 {
		return r.canvasList(blocks, depth, prefix)
	}
	var buf bytes.Buffer
	for i, b := range blocks {
		path := prefix + strconv.Itoa(i)
		h, err := r.block(b, depth, path)
		if err != nil {
			return "", err
		}
		if r.preview && r.inSection == 0 && h != "" {
			// display:contents: no box of its own, so the layout is the page's.
			buf.WriteString(`<div data-pb-path="` + path + `" style="display:contents">` + string(h) + `</div>`)
			continue
		}
		buf.WriteString(string(h))
	}
	return template.HTML(buf.String()), nil //nolint:gosec -- built from escaped templates; path is digits and dots
}

func (r *renderer) block(b Block, depth int, path string) (template.HTML, error) {
	spec, ok := r.pl.blockSpec(b.Type)
	if !ok || depth > maxBuilderDepth {
		return "", nil
	}
	d := b.Data
	if d == nil {
		d = map[string]any{}
	}
	if b.Type == "section_ref" {
		id := uint(intOf(d, "section_id", 0))
		inner, ok := r.sections[id]
		if !ok || r.seen[id] {
			return "", nil // a missing section, or one that contains itself
		}
		r.seen[id] = true
		defer delete(r.seen, id)
		// A section's blocks belong to the section: a click selects the reference.
		r.inSection++
		defer func() { r.inSection-- }()
		return r.blocks(inner, depth+1, "")
	}
	v := blockView{ID: r.nextID(), Type: b.Type, D: d, Anchor: str(d, "anchor"),
		HTML: map[string]template.HTML{}, Img: map[string]imageView{}, Src: map[string]template.URL{}, URL: map[string]template.URL{}}
	v.Class, v.Style = presentation(d)
	for _, f := range spec.Fields {
		switch f.Kind {
		case fHTML:
			v.HTML[f.Name] = template.HTML(r.pl.renderShortcodes(mks.SanitizeHTML(str(d, f.Name)), r.req.shortcodeCtx())) //nolint:gosec -- sanitized again on render; shortcodes are code
		case fMedia:
			p := str(d, f.Name)
			v.Img[f.Name] = r.media.get(&p, str(d, "alt"))
			v.Src[f.Name] = r.pl.fileURL(&p)
		case fURL:
			v.URL[f.Name] = linkOrHash(str(d, f.Name))
		case fList:
			items, _ := d[f.Name].([]any)
			for i, it := range items {
				m, _ := it.(map[string]any)
				iv := itemView{D: m, ID: v.ID + "-" + strconv.Itoa(i)}
				if r := intOf(m, "rating", 0); r >= 1 && r <= 5 {
					iv.Stars = strings.Repeat("★", r) + strings.Repeat("☆", 5-r)
				}
				for _, inner := range f.Item {
					switch inner.Kind {
					case fHTML:
						iv.HTML = template.HTML(r.pl.renderShortcodes(mks.SanitizeHTML(str(m, inner.Name)), r.req.shortcodeCtx())) //nolint:gosec -- sanitized; shortcodes are code
					case fMedia:
						p := str(m, inner.Name)
						iv.Img = r.media.get(&p, str(m, "alt"))
					case fURL:
						iv.URL = linkOrHash(str(m, inner.Name))
					}
				}
				v.Items = append(v.Items, iv)
			}
		}
	}
	if spec.Slots > 0 {
		for _, slot := range slotsOf(b) {
			h, err := r.blocks(slot, depth+1, path+"."+strconv.Itoa(len(v.Children))+".")
			if err != nil {
				return "", err
			}
			v.Children = append(v.Children, h)
		}
		v.Cols = columnTemplate(str(d, "layout"), len(v.Children))
		if b.Type == "grid_layout" {
			v.Spans = gridSpans(d, len(v.Children))
		}
	}
	if b.Type == "tabs" || b.Type == "slider" {
		r.script = true
	}
	if spec.Category == "sections" {
		if err := r.themeBlockView(&v); err != nil {
			return "", err
		}
	}
	name := "block-" + b.Type
	if pb, lp := r.pl.packageBlock(b.Type); pb != nil {
		if err := r.renderPackageBlock(pb, lp, &v); err != nil {
			r.pl.logf("cms: plugin block %s: %v", b.Type, err)
			return "", nil
		}
		name = "block-pkg"
	}
	var buf bytes.Buffer
	if err := blockTemplate.ExecuteTemplate(&buf, name, v); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil //nolint:gosec -- escaped by html/template
}

// gridSpans are the CSS variables of each grid column (--s, --s-sm, --s-md, --s-lg), or
// nil without column_spans. Columns past the list take the last entry's spans.
func gridSpans(d map[string]any, n int) []template.CSS {
	list, _ := d["column_spans"].([]any)
	if len(list) == 0 || n == 0 {
		return nil
	}
	out := make([]template.CSS, n)
	for i := range n {
		m, _ := list[min(i, len(list)-1)].(map[string]any)
		css := "--s:" + strconv.Itoa(clampSpan(intOf(m, "span", 12)))
		for _, size := range []string{"sm", "md", "lg"} {
			if v := intOf(m, "span_"+size, 0); v > 0 {
				css += ";--s-" + size + ":" + strconv.Itoa(clampSpan(v))
			}
		}
		out[i] = template.CSS(css) //nolint:gosec -- numbers only
	}
	return out
}

func clampSpan(v int) int { return max(1, min(12, v)) }

func linkOrHash(u string) template.URL {
	if u == "" {
		return ""
	}
	if s := safeURL(u); s != "" {
		return s
	}
	return "#"
}

// renderBuilder renders a block-based page body with its sections and media.
func (pl *Plugin) renderBuilder(req *request, blocks []Block, preview ...bool) (template.HTML, bool, error) {
	r := &renderer{pl: pl, req: req, seen: map[uint]bool{}, sections: map[uint][]Block{}, preview: len(preview) > 0 && preview[0]}
	return pl.renderWith(r, blocks)
}

// renderCanvas renders blocks for the editor canvas (block names from labels).
func (pl *Plugin) renderCanvas(req *request, blocks []Block, labels map[string]string) (template.HTML, bool, error) {
	r := &renderer{pl: pl, req: req, seen: map[uint]bool{}, sections: map[uint][]Block{}, canvas: true, labels: labels}
	return pl.renderWith(r, blocks)
}

func (pl *Plugin) renderWith(r *renderer, blocks []Block) (template.HTML, bool, error) {
	if err := r.loadSections(blocks); err != nil {
		return "", false, err
	}
	var paths []*string
	pl.collectMedia(blocks, &paths)
	for _, s := range r.sections {
		pl.collectMedia(s, &paths)
	}
	set, err := pl.mediaImages(r.req, paths...)
	if err != nil {
		return "", false, err
	}
	r.media = set
	h, err := r.blocks(blocks, 0, "")
	return h, r.script, err
}

// loadSections loads the reusable sections a payload refers to, and theirs, in a few
// queries (a section may use other sections).
func (r *renderer) loadSections(blocks []Block) error {
	if r.req == nil || r.req.site == nil || r.pl.Orm == nil {
		return nil
	}
	pending := sectionIDs(blocks)
	for round := 0; round < maxBuilderDepth && len(pending) > 0; round++ {
		var rows []Section
		if err := r.pl.query().Model(&Section{}).Where("site_id = ?", r.req.site.ID).WhereIn("id", idList(pending)).Find(&rows); err != nil {
			return err
		}
		pending = nil
		for _, s := range rows {
			r.sections[s.ID] = s.BuilderPayload.V
			for _, id := range sectionIDs(s.BuilderPayload.V) {
				if _, done := r.sections[id]; !done {
					pending = append(pending, id)
				}
			}
		}
	}
	return nil
}

func sectionIDs(blocks []Block) []uint {
	var out []uint
	for _, b := range blocks {
		if b.Type == "section_ref" {
			if id := intOf(b.Data, "section_id", 0); id > 0 {
				out = append(out, uint(id))
			}
		}
		for _, slot := range slotsOf(b) {
			out = append(out, sectionIDs(slot)...)
		}
	}
	return out
}

// blockScript makes tabs and the slider work (without it, tabs show stacked and slides
// scroll). Inline and small; it only touches elements it finds.
const blockScript = `(function(){document.querySelectorAll('[data-tabs]').forEach(function(t){var b=t.querySelectorAll('[role=tab]'),p=t.querySelectorAll('[role=tabpanel]');t.classList.add('js');function s(i){b.forEach(function(x,j){x.setAttribute('aria-selected',j===i);x.tabIndex=j===i?0:-1});p.forEach(function(x,j){x.hidden=j!==i})}b.forEach(function(x,i){x.addEventListener('click',function(){s(i)});x.addEventListener('keydown',function(e){var k=e.key==='ArrowLeft'?1:e.key==='ArrowRight'?-1:0;if(k){var n=(i+k+b.length)%b.length;s(n);b[n].focus()}})});s(0)});document.querySelectorAll('[data-slider]').forEach(function(s){var track=s.querySelector('.track'),n=track.children.length,i=0,ms=+s.dataset.speed||5000;function go(k){i=(k+n)%n;track.scrollTo({left:track.children[i].offsetLeft-track.offsetLeft,behavior:'smooth'})}s.querySelectorAll('[data-go]').forEach(function(b){b.addEventListener('click',function(){go(i+(+b.dataset.go))})});if(s.dataset.autoplay==='1'&&n>1&&!matchMedia('(prefers-reduced-motion: reduce)').matches){setInterval(function(){if(!s.matches(':hover'))go(i+1)},ms)}})})();`

const blockCSS = `picture{display:contents}.pb>*{box-sizing:border-box}.pb{display:block}.spacing-none{margin-block:0}.spacing-sm{margin-block:.5rem}.spacing-md{margin-block:1rem}.spacing-lg{margin-block:2rem}.padding-sm{padding:.5rem}.padding-md{padding:1rem}.padding-lg{padding:2rem}.background-muted{background:#f4f4f5}.background-brand{background:#eaf5f1}.align-center{text-align:center}.align-left{text-align:left}.align-right{text-align:right}.align-end{text-align:end}.width-contained{max-width:56rem;margin-inline:auto}.border_width-sm{border:1px solid #e4e4e7}.border_width-md{border:2px solid #e4e4e7}.border_width-lg{border:4px solid #e4e4e7}.border_radius-sm{border-radius:.375rem}.border_radius-md{border-radius:.75rem}.border_radius-lg{border-radius:1.25rem}@media(max-width:768px){.visibility-hide_mobile{display:none!important}}@media(min-width:769px){.visibility-hide_desktop{display:none!important}}.pb-heading.al-center{text-align:center}.pb-heading.al-left{text-align:left}.pb-heading.al-right{text-align:right}.pb-features .grid,.pb-testimonial .grid{display:grid;gap:1.25rem;grid-template-columns:repeat(var(--cols,3),minmax(0,1fr))}@media(max-width:768px){.pb-features .grid,.pb-testimonial .grid{grid-template-columns:1fr}}.feature{padding:1.25rem;border-radius:14px}.style-bordered .feature,.style-bordered .quote-card{border:1px solid var(--line)}.style-shadowed .feature,.style-shadowed .quote-card{box-shadow:0 10px 30px rgba(23,32,31,.08);background:#fff}.style-filled .feature{background:#eaf5f1}.feature .icon{display:inline-grid;place-items:center;width:2.75rem;height:2.75rem;font-size:1.3rem;background:#eaf5f1;margin-bottom:.5rem}.icon-circle .icon{border-radius:999px}.icon-square .icon{border-radius:10px}.icon-none .icon{background:none;width:auto;height:auto}.feature h3{margin:.25rem 0;font-size:1.1rem}.feature p{margin:0;color:var(--muted);font-size:.95rem}.quote-card{padding:1.25rem;border-radius:14px;margin:0}.quote-card blockquote{margin:0 0 .75rem}.quote-card figcaption{display:flex;gap:.6rem;align-items:center;font-size:.9rem}.quote-card img{width:2.5rem;height:2.5rem;border-radius:999px;object-fit:cover}.pb-image{margin-inline:auto}.size-small{max-width:20rem}.size-medium{max-width:32rem}.size-large{max-width:48rem}.size-full{max-width:none}.pb-image.al-left{margin-inline:0 auto}.pb-image.al-right{margin-inline:auto 0}.pb-image img{width:100%;height:auto;display:block}.rounded img{border-radius:16px}.shadow img{box-shadow:0 18px 40px rgba(23,32,31,.18)}.pb-image figcaption,.pb-media figcaption{color:var(--muted);font-size:.85rem;margin-top:.4rem}.pb-media video{width:100%;border-radius:12px}.pb-media audio{width:100%}.spacer-xs{height:.5rem}.spacer-sm{height:1rem}.spacer-md{height:2rem}.spacer-lg{height:3rem}.spacer-xl{height:4.5rem}.spacer-2xl{height:6rem}.pb-divider{border:0;border-top:1px solid var(--line);margin-block:1.5rem}.pb-divider.dashed{border-top-style:dashed}.pb-divider.dotted{border-top-style:dotted}.pb-divider.w-25{width:25%}.pb-divider.w-50{width:50%}.pb-divider.w-75{width:75%}.pb-divider.al-center{margin-inline:auto}.pb-divider.al-left{margin-inline:0 auto}.pb-divider.al-right{margin-inline:auto 0}.cols{display:grid}.gap-none{gap:0}.gap-sm{gap:.5rem}.gap-md{gap:1.25rem}.gap-lg{gap:2.5rem}.va-center{align-items:center}.va-end{align-items:end}.va-stretch{align-items:stretch}.cols>div>*:first-child{margin-top:0}@media(max-width:1024px){.stack-tablet{grid-template-columns:1fr!important}}@media(max-width:640px){.stack-mobile{grid-template-columns:1fr!important}}.stack-always{grid-template-columns:1fr!important}.grid-auto{display:grid;grid-template-columns:repeat(auto-fit,minmax(14rem,1fr))}.grid-12{display:grid;grid-template-columns:repeat(12,minmax(0,1fr))}.grid-12>div{grid-column:span var(--s,12)/span var(--s,12);min-width:0}@media(min-width:640px){.grid-12>div{grid-column:span var(--s-sm,var(--s,12))/span var(--s-sm,var(--s,12))}}@media(min-width:768px){.grid-12>div{grid-column:span var(--s-md,var(--s-sm,var(--s,12)))/span var(--s-md,var(--s-sm,var(--s,12)))}}@media(min-width:1024px){.grid-12>div{grid-column:span var(--s-lg,var(--s-md,var(--s-sm,var(--s,12))))/span var(--s-lg,var(--s-md,var(--s-sm,var(--s,12))))}}.grid-12.stack-always>div{grid-column:1/-1!important}@media(max-width:639px){.grid-12.stack-mobile>div{grid-column:1/-1!important}}@media(max-width:1023px){.grid-12.stack-tablet>div{grid-column:1/-1!important}}.inset{margin-inline:auto}.mw-prose{max-width:42rem}.mw-3xl{max-width:48rem}.mw-5xl{max-width:64rem}.mw-6xl{max-width:72rem}.mw-7xl{max-width:80rem}.bleed{margin-inline:calc(50% - 50vw);padding-inline:calc(50vw - 50%)}.pb-card{border:1px solid var(--line);background:#fff;overflow:hidden}.pb-card.radius-sm{border-radius:.375rem}.pb-card.radius-md{border-radius:.75rem}.pb-card.radius-lg{border-radius:1.25rem}.pb-card.shadow-sm{box-shadow:0 2px 8px rgba(23,32,31,.06)}.pb-card.shadow-md{box-shadow:0 14px 34px rgba(23,32,31,.12)}.pb-card>header,.pb-card>footer{padding:.75rem 1.25rem;background:#fafafa}.pb-card>header{border-bottom:1px solid var(--line);font-weight:700}.pb-card>footer{border-top:1px solid var(--line)}.pb-card>.body{padding:1.25rem}.pb-hero{position:relative;display:grid;place-items:center;overflow:hidden;border-radius:18px;background:#17201f center/cover no-repeat;padding:3rem 1.5rem}.pb-hero.h-small{min-height:16rem}.pb-hero.h-medium{min-height:24rem}.pb-hero.h-large{min-height:34rem}.pb-hero.h-full{min-height:90vh}.pb-hero>img,.pb-hero>picture>img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}.pb-hero .shade{position:absolute;inset:0}.ov-light .shade{background:rgba(255,255,255,.55)}.ov-dark .shade{background:rgba(0,0,0,.5)}.ov-gradient .shade{background:linear-gradient(180deg,rgba(0,0,0,.1),rgba(0,0,0,.65))}.pb-hero .inner{position:relative;max-width:46rem;width:100%}.pb-hero.tc-white{color:#fff}.pb-hero.tc-dark{color:var(--ink)}.pb-hero h2{font-size:clamp(2rem,5vw,3.4rem);line-height:1.25;margin:0 0 .75rem}.pb-hero p{font-size:1.15rem;margin:0 0 1.5rem;opacity:.92}.ta-center{text-align:center}.ta-left{text-align:left}.ta-right{text-align:right}.btns{display:flex;flex-wrap:wrap;gap:.75rem}.ta-center .btns,.cta-center .btns{justify-content:center}.btn{display:inline-block;padding:.65rem 1.4rem;border-radius:10px;font-weight:700;text-decoration:none;border:2px solid transparent}.btn-primary{background:var(--accent);color:#fff}.btn-secondary{background:#f2c14e;color:#17201f}.btn-outline{border-color:currentColor;color:inherit}.btn-ghost{color:var(--accent)}.btn-sm{padding:.35rem .9rem;font-size:.85rem}.btn-lg{padding:.85rem 1.8rem;font-size:1.1rem}.btn-full{display:block;text-align:center}.pb-button.al-center{text-align:center}.pb-button.al-left{text-align:left}.pb-cta{padding:2rem;border-radius:18px}.pb-cta h2{margin:0 0 .5rem}.pb-cta p{margin:0 0 1.25rem;opacity:.9}.cta-boxed{border:1px solid var(--line);background:#fff}.cta-gradient{background:linear-gradient(135deg,var(--accent),#3b6ea8);color:#fff}.cta-gradient .btn-outline,.cta-dark .btn-outline{color:#fff}.cta-dark{background:#17201f;color:#fff}.cta-center{text-align:center}.cta-between{display:flex;flex-wrap:wrap;justify-content:space-between;align-items:center;gap:1rem}.cta-between p{margin:0}.pb-accordion details{padding:.75rem 1rem}.acc-bordered details{border:1px solid var(--line);border-bottom:0}.acc-bordered details:last-child{border-bottom:1px solid var(--line)}.acc-separated details{border:1px solid var(--line);border-radius:12px;margin-bottom:.6rem}.acc-simple details{border-bottom:1px solid var(--line)}.pb-accordion summary{cursor:pointer;font-weight:700}.pb-tabs [role=tablist]{display:flex;flex-wrap:wrap;gap:.4rem;border-bottom:1px solid var(--line);margin-bottom:1rem}.pb-tabs:not(.js) [role=tablist]{display:none}.pb-tabs:not(.js) [role=tabpanel] h3{display:block}.pb-tabs [role=tabpanel] h3{display:none}.pb-tabs [role=tab]{font:inherit;background:none;border:0;padding:.5rem 1rem;cursor:pointer;color:var(--muted);border-bottom:2px solid transparent}.pb-tabs [role=tab][aria-selected=true]{color:var(--accent);border-bottom-color:var(--accent);font-weight:700}.tabs-pills [role=tab][aria-selected=true]{background:var(--accent);color:#fff;border-radius:999px}.tabs-boxed [role=tab][aria-selected=true]{border:1px solid var(--line);border-bottom-color:#fff;border-radius:10px 10px 0 0}.tabs-buttons [role=tab]{border:1px solid var(--line);border-radius:10px}.pb-slider{position:relative}.pb-slider .track{display:flex;overflow-x:auto;scroll-snap-type:x mandatory;scrollbar-width:none;border-radius:16px}.pb-slider .track::-webkit-scrollbar{display:none}.pb-slider .slide{flex:0 0 100%;scroll-snap-align:start;position:relative;display:grid;place-items:center;background:#17201f;color:#fff;overflow:hidden}.sh-small .slide{height:16rem}.sh-medium .slide{height:24rem}.sh-large .slide{height:34rem}.pb-slider .slide img{position:absolute;inset:0;width:100%;height:100%;object-fit:cover}.pb-slider .cap{position:relative;text-align:center;padding:1.5rem;background:rgba(0,0,0,.35);border-radius:12px;max-width:40rem}.pb-slider .cap h3{margin:0 0 .25rem}.pb-slider .arrows{position:absolute;inset-inline:0;top:50%;display:flex;justify-content:space-between;padding:0 .75rem;pointer-events:none}.pb-slider .arrows button{pointer-events:auto;width:2.5rem;height:2.5rem;border-radius:999px;border:0;background:rgba(255,255,255,.85);cursor:pointer;font-size:1.2rem}.pb-slider .dots{display:flex;justify-content:center;gap:.4rem;margin-top:.6rem}.pb-slider .dots a{width:.6rem;height:.6rem;border-radius:999px;background:var(--line)}.stars{color:#c98a00}`

var blockTemplate = template.Must(template.Must(template.New("blocks").Parse(pictureTemplate)).Parse(`
{{define "block-pkg"}}<div class="pb pb-pkg {{.Type}} {{.Class}}"{{template "open" .}}>{{.Inner}}</div>{{end}}
{{define "open"}}{{if .Anchor}} id="{{.Anchor}}"{{end}}{{if .Style}} style="{{.Style}}"{{end}}{{end}}
{{define "block-heading"}}<div class="pb pb-heading al-{{.D.alignment}} {{.Class}}"{{template "open" .}}>{{$l := .D.level}}{{if eq $l "h1"}}<h1>{{.D.text}}</h1>{{else if eq $l "h3"}}<h3>{{.D.text}}</h3>{{else if eq $l "h4"}}<h4>{{.D.text}}</h4>{{else if eq $l "h5"}}<h5>{{.D.text}}</h5>{{else if eq $l "h6"}}<h6>{{.D.text}}</h6>{{else}}<h2>{{.D.text}}</h2>{{end}}</div>{{end}}
{{define "block-text"}}<div class="pb pb-text content {{.Class}}"{{template "open" .}}>{{index .HTML "content"}}</div>{{end}}
{{define "block-quote"}}<blockquote class="pb quote {{.Class}}"{{template "open" .}}><p>{{.D.text}}</p>{{if .D.author}}<footer>{{.D.author}}</footer>{{end}}</blockquote>{{end}}
{{define "block-callout"}}<aside class="pb callout {{.D.tone}} {{.Class}}"{{template "open" .}}>{{.D.text}}</aside>{{end}}
{{define "block-features"}}<section class="pb pb-features style-{{.D.style}} icon-{{.D.icon_style}} {{.Class}}"{{template "open" .}}>{{if .D.heading}}<h2>{{.D.heading}}</h2>{{end}}{{if .D.subheading}}<p class="sub">{{.D.subheading}}</p>{{end}}<div class="grid" style="--cols:{{.D.columns}}">{{range .Items}}<div class="feature">{{if .D.icon}}<span class="icon" aria-hidden="true">{{.D.icon}}</span>{{end}}<h3>{{if .URL}}<a href="{{.URL}}">{{.D.title}}</a>{{else}}{{.D.title}}{{end}}</h3>{{if .D.description}}<p>{{.D.description}}</p>{{end}}</div>{{end}}</div></section>{{end}}
{{define "block-testimonial"}}<section class="pb pb-testimonial style-{{.D.style}} {{.Class}}"{{template "open" .}}>{{if .D.heading}}<h2>{{.D.heading}}</h2>{{end}}<div class="grid" style="--cols:{{.D.columns}}">{{range .Items}}<figure class="quote-card"><blockquote>{{.D.content}}</blockquote><figcaption>{{if .Img.Src}}<img src="{{.Img.Src}}" alt="">{{end}}<span><strong>{{.D.author_name}}</strong>{{if .D.author_title}} — {{.D.author_title}}{{end}}</span>{{if .Stars}}<span class="stars">{{.Stars}}</span>{{end}}</figcaption></figure>{{end}}</div></section>{{end}}
{{define "block-image"}}<figure class="pb pb-image size-{{.D.size}} al-{{.D.alignment}}{{if .D.rounded}} rounded{{end}}{{if .D.shadow}} shadow{{end}} {{.Class}}"{{template "open" .}}>{{with index .Img "image"}}{{template "picture" (.Sized "(max-width: 800px) 100vw, 48rem" "" true)}}{{end}}{{if .D.caption}}<figcaption>{{.D.caption}}</figcaption>{{end}}</figure>{{end}}
{{define "block-video"}}<figure class="pb pb-media {{.Class}}"{{template "open" .}}><video src="{{index .Src "video"}}"{{if .D.controls}} controls{{end}}{{if .D.autoplay}} autoplay muted playsinline{{end}}{{if .D.loop}} loop{{end}} preload="metadata"></video>{{if .D.caption}}<figcaption>{{.D.caption}}</figcaption>{{end}}</figure>{{end}}
{{define "block-audio"}}<figure class="pb pb-media {{.Class}}"{{template "open" .}}><audio src="{{index .Src "audio"}}"{{if .D.controls}} controls{{end}}{{if .D.autoplay}} autoplay{{end}}{{if .D.loop}} loop{{end}} preload="metadata"></audio>{{if .D.caption}}<figcaption>{{.D.caption}}</figcaption>{{end}}</figure>{{end}}
{{define "block-slider"}}<div class="pb pb-slider sh-{{.D.height}} {{.Class}}"{{template "open" .}} data-slider data-speed="{{.D.autoplay_speed}}" data-autoplay="{{if .D.autoplay}}1{{else}}0{{end}}" role="region" aria-roledescription="carousel"><div class="track">{{range .Items}}<div class="slide" id="{{.ID}}">{{if .Img.Src}}{{template "picture" (.Img.Sized "100vw" "" true)}}{{end}}{{if or .D.title .D.subtitle .D.button_text}}<div class="cap">{{if .D.title}}<h3>{{.D.title}}</h3>{{end}}{{if .D.subtitle}}<p>{{.D.subtitle}}</p>{{end}}{{if and .D.button_text .URL}}<a class="btn btn-primary" href="{{.URL}}">{{.D.button_text}}</a>{{end}}</div>{{end}}</div>{{end}}</div>{{if .D.show_arrows}}<div class="arrows"><button type="button" data-go="-1" aria-label="قبلی">›</button><button type="button" data-go="1" aria-label="بعدی">‹</button></div>{{end}}{{if .D.show_dots}}<div class="dots">{{range .Items}}<a href="#{{.ID}}" aria-label="اسلاید"></a>{{end}}</div>{{end}}</div>{{end}}
{{define "block-spacer"}}<div class="pb spacer-{{.D.size}} {{.Class}}" aria-hidden="true"{{template "open" .}}></div>{{end}}
{{define "block-divider"}}<hr class="pb pb-divider {{.D.style}} w-{{.D.line_width}} al-{{.D.alignment}} {{.Class}}"{{template "open" .}}>{{end}}
{{define "block-columns"}}<div class="pb cols gap-{{.D.gap}} va-{{.D.vertical_alignment}} stack-{{.D.stack_on_mobile}} {{.Class}}"{{if .Anchor}} id="{{.Anchor}}"{{end}} style="{{.Cols}}{{if .Style}};{{.Style}}{{end}}">{{range .Children}}<div>{{.}}</div>{{end}}</div>{{end}}
{{define "block-grid_layout"}}{{if .Spans}}<div class="pb grid-12 stack-{{.D.stack_on_mobile}} gap-{{.D.gap}} va-{{.D.vertical_alignment}} {{.Class}}"{{template "open" .}}>{{range $i, $c := .Children}}<div style="{{index $.Spans $i}}">{{$c}}</div>{{end}}</div>{{else}}<div class="pb grid-auto gap-{{.D.gap}} va-{{.D.vertical_alignment}} {{.Class}}"{{template "open" .}}>{{range .Children}}<div>{{.}}</div>{{end}}</div>{{end}}{{end}}
{{define "block-container_inset"}}<div class="pb {{if .D.background_full_bleed}}bleed{{end}} {{.Class}}"{{template "open" .}}><div class="inset mw-{{.D.max_width}}"{{with .D.inset_background}} style="background-color:{{.}}"{{end}}>{{range .Children}}{{.}}{{end}}</div></div>{{end}}
{{define "block-card"}}<div class="pb pb-card shadow-{{.D.shadow}} radius-{{.D.radius}} {{.Class}}"{{template "open" .}}>{{range $i, $c := .Children}}{{if eq $i 0}}{{if $.D.show_header}}<header>{{$c}}</header>{{end}}{{else if eq $i 1}}<div class="body">{{$c}}</div>{{else if eq $i 2}}{{if $.D.show_footer}}<footer>{{$c}}</footer>{{end}}{{end}}{{end}}</div>{{end}}
{{define "buttons"}}{{if or (and .D.button_text (index .URL "button_url")) (and .D.show_secondary_button .D.secondary_button_text)}}<div class="btns">{{if .D.button_text}}<a class="btn btn-{{or .D.button_style "primary"}}" href="{{index .URL "button_url"}}">{{.D.button_text}}</a>{{end}}{{if and .D.show_secondary_button .D.secondary_button_text}}<a class="btn btn-outline" href="{{index .URL "secondary_button_url"}}">{{.D.secondary_button_text}}</a>{{end}}</div>{{end}}{{end}}
{{define "block-hero"}}<section class="pb pb-hero h-{{.D.height}} ov-{{.D.overlay}} tc-{{.D.hero_text_color}} {{.Class}}"{{template "open" .}}>{{with index .Img "background_image"}}{{if .Src}}{{template "picture" (.Decorative.Sized "100vw" "" false)}}{{end}}{{end}}<div class="shade"></div><div class="inner ta-{{.D.text_alignment}}"><h2>{{.D.title}}</h2>{{if .D.subtitle}}<p>{{.D.subtitle}}</p>{{end}}{{template "buttons" .}}</div></section>{{end}}
{{define "block-tabs"}}<div class="pb pb-tabs tabs-{{.D.style}} {{.Class}}"{{template "open" .}} data-tabs><div role="tablist">{{range .Items}}<button type="button" role="tab" id="{{.ID}}-t" aria-controls="{{.ID}}">{{.D.title}}</button>{{end}}</div>{{range .Items}}<div role="tabpanel" id="{{.ID}}" aria-labelledby="{{.ID}}-t"><h3>{{.D.title}}</h3><div class="content">{{.HTML}}</div></div>{{end}}</div>{{end}}
{{define "block-button"}}<div class="pb pb-button al-{{.D.alignment}} {{.Class}}"{{template "open" .}}><a class="btn btn-{{.D.style}} btn-{{.D.size}}{{if .D.full_width}} btn-full{{end}}" href="{{or (index .URL "url") "#"}}"{{if .D.new_tab}} target="_blank" rel="noopener noreferrer"{{end}}>{{.D.text}}</a></div>{{end}}
{{define "block-cta"}}<section class="pb pb-cta cta-{{.D.style}} cta-{{.D.alignment}} {{.Class}}"{{template "open" .}}><div><h2>{{.D.title}}</h2>{{if .D.description}}<p>{{.D.description}}</p>{{end}}</div>{{template "buttons" .}}</section>{{end}}
{{define "block-accordion"}}<section class="pb pb-accordion acc-{{.D.style}} {{.Class}}"{{template "open" .}}>{{if .D.heading}}<h2>{{.D.heading}}</h2>{{end}}{{$name := .ID}}{{$multi := .D.allow_multiple}}{{$first := .D.first_open}}{{range $i, $it := .Items}}<details{{if not $multi}} name="{{$name}}"{{end}}{{if and $first (eq $i 0)}} open{{end}}><summary>{{$it.D.question}}</summary><div class="content">{{$it.HTML}}</div></details>{{end}}</section>{{end}}
{{define "block-section_ref"}}{{end}}
`))

func fmtAny(v any) string { return fmt.Sprint(v) }

// canvasList draws a list of blocks for the editor canvas: every block framed with its bar
// (name, edit, up, down, copy, delete) and an insert row before each block and after the
// last. The parent page attaches the behavior (the frame runs no scripts).
func (r *renderer) canvasList(blocks []Block, depth int, prefix string) (template.HTML, error) {
	list := strings.TrimSuffix(prefix, ".")
	var buf bytes.Buffer
	buf.WriteString(`<div class="mks-list" data-pb-list="` + list + `">`)
	insert := func(i int, empty bool) {
		cls := "mks-insert"
		if empty {
			cls += " is-visible"
		}
		buf.WriteString(`<div class="` + cls + `" data-pb-insert="` + list + `|` + strconv.Itoa(i) + `" role="button" tabindex="0"><span class="mks-insert-pill"><span class="mks-plus">+</span><span class="mks-insert-label">کامپوننت اینجا اضافه شود</span></span></div>`)
	}
	for i, b := range blocks {
		insert(i, false)
		path := prefix + strconv.Itoa(i)
		h, err := r.block(b, depth, path)
		if err != nil {
			return "", err
		}
		label := r.labels[b.Type]
		if label == "" {
			label = b.Type
		}
		name := html.EscapeString(label)
		buf.WriteString(`<div class="mks-block" data-pb-path="` + path + `"><div class="mks-tools"><span class="mks-name" data-pb-drag>` + name + `</span>` +
			`<button type="button" data-pb-act="edit" title="ویرایش">✎</button><button type="button" data-pb-act="up" title="انتقال به بالا">↑</button>` +
			`<button type="button" data-pb-act="down" title="انتقال به پایین">↓</button><button type="button" data-pb-act="copy" title="کپی">⧉</button>` +
			`<button type="button" data-pb-act="delete" title="حذف">×</button></div>`)
		buf.WriteString(string(h))
		buf.WriteString(`</div>`)
	}
	insert(len(blocks), len(blocks) == 0)
	buf.WriteString(`</div>`)
	return template.HTML(buf.String()), nil //nolint:gosec -- escaped label; paths are digits and dots; block HTML from escaped templates
}
