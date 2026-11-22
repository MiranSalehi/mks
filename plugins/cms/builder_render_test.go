package cms

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestEveryBlockRenders(t *testing.T) {
	pl := &Plugin{}
	payload := decodeJSON(t, `[
		{"type": "heading", "data": {"text": "<b>Hi</b>", "level": "h3", "alignment": "center", "anchor": "top", "text_color": "#123456"}},
		{"type": "text", "data": {"content": "<p>Body</p><script>bad()</script>"}},
		{"type": "quote", "data": {"text": "Q", "author": "A"}},
		{"type": "callout", "data": {"text": "C", "tone": "warning"}},
		{"type": "features", "data": {"heading": "F", "columns": 2, "features": [{"icon": "★", "title": "One", "description": "d", "link": "/x"}]}},
		{"type": "testimonial", "data": {"testimonials": [{"content": "Great", "author_name": "Sara", "rating": 4}]}},
		{"type": "spacer", "data": {"size": "lg"}},
		{"type": "divider", "data": {"style": "dashed", "line_width": "50"}},
		{"type": "columns", "data": {"layout": "1-2", "children": [{"blocks": [{"type": "heading", "data": {"text": "L"}}]}, {"blocks": [{"type": "text", "data": {"content": "<p>R</p>"}}]}]}},
		{"type": "grid_layout", "data": {"children": [{"blocks": []}]}},
		{"type": "container_inset", "data": {"max_width": "prose", "inset_background": "#eee", "children": [{"blocks": [{"type": "heading", "data": {"text": "In"}}]}]}},
		{"type": "card", "data": {"show_footer": true, "children": [{"blocks": [{"type": "heading", "data": {"text": "Head"}}]}, {"blocks": []}, {"blocks": [{"type": "button", "data": {"text": "More", "url": "/more"}}]}]}},
		{"type": "hero", "data": {"title": "Hero", "subtitle": "Sub", "button_text": "Go", "button_url": "javascript:x", "show_secondary_button": true, "secondary_button_text": "Read", "secondary_button_url": "/read"}},
		{"type": "tabs", "data": {"tabs": [{"title": "T1", "content": "<p>one</p>"}, {"title": "T2", "content": "<p>two</p>"}]}},
		{"type": "button", "data": {"text": "Buy", "url": "https://shop.example", "new_tab": true, "style": "outline", "size": "lg"}},
		{"type": "cta", "data": {"title": "Join", "button_text": "Sign up", "button_url": "/join"}},
		{"type": "accordion", "data": {"items": [{"question": "Why?", "answer": "<p>Because</p>"}, {"question": "How?", "answer": "<p>So</p>"}]}},
		{"type": "slider", "data": {"slides": [{"title": "S1"}, {"title": "S2", "button_text": "B", "button_url": "/b"}]}},
		{"type": "section_ref", "data": {"section_id": 9}}
	]`)
	blocks, errs := pl.NormalizeBuilder(payload, 1, "b")
	if len(errs) > 0 {
		t.Fatalf("normalize: %v", errs)
	}
	if len(blocks) != 19 {
		t.Fatalf("blocks = %d", len(blocks))
	}
	html, script, err := pl.renderBuilder(&request{site: &Site{ID: 1, Slug: "s"}}, blocks)
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	if !script {
		t.Error("tabs and slider need the script")
	}
	for _, want := range []string{
		`id="top" style="color:#123456"`, `<h3>&lt;b&gt;Hi&lt;/b&gt;</h3>`, `<p>Body</p>`,
		`<footer>A</footer>`, `callout warning`, `style="--cols:2"`, `<a href="/x">One</a>`, `★★★★☆`,
		`spacer-lg`, `pb-divider dashed w-50`, `grid-template-columns:1fr 2fr`, `<div><div class="pb pb-heading`,
		`mw-prose" style="background-color:#eee"`, `<footer><div class="pb pb-button`,
		`<a class="btn btn-primary" href="#">Go</a>`, `href="/read">Read</a>`, `role="tab"`, `role="tabpanel"`,
		`target="_blank" rel="noopener noreferrer"`, `btn-outline btn-lg`, `href="/join"`,
		`<details name="`, `<summary>Why?</summary>`, `data-slider`, `href="/b">B</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, "<script") || strings.Contains(out, "javascript:") || strings.Contains(out, "<b>Hi") {
		t.Errorf("unsafe output:\n%s", out)
	}
	// The first accordion item is open by default; exclusive accordions share a name.
	if !strings.Contains(out, `open><summary>Why?`) || strings.Contains(out, `open><summary>How?`) {
		t.Error("first_open")
	}
}

func TestSectionsExpandWithoutLoops(t *testing.T) {
	pl := &Plugin{}
	self, _ := pl.NormalizeBuilder(decodeJSON(t, `[{"type": "heading", "data": {"text": "From section"}}, {"type": "section_ref", "data": {"section_id": 1}}]`), 1, "b")
	r := &renderer{pl: pl, seen: map[uint]bool{}, sections: map[uint][]Block{1: self}}
	page, _ := pl.NormalizeBuilder(decodeJSON(t, `[{"type": "section_ref", "data": {"section_id": 1}}, {"type": "section_ref", "data": {"section_id": 2}}]`), 1, "b")
	html, err := r.blocks(page, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(html), "From section"); n != 1 {
		t.Fatalf("a section that includes itself rendered %d times", n)
	}
	if ids := sectionIDs(page); len(ids) != 2 {
		t.Fatalf("sectionIDs = %v", ids)
	}
}

func TestBlockFieldsDoNotShadowStyleSettings(t *testing.T) {
	style := map[string]bool{}
	for _, f := range presentationFields {
		style[f.Name] = true
	}
	for _, b := range BlockSpecs {
		for _, f := range b.Fields {
			if style[f.Name] {
				t.Errorf("block %s field %q has the name of a style setting", b.Type, f.Name)
			}
		}
	}
}

func TestTemplatesAreValid(t *testing.T) {
	pl := &Plugin{}
	for name, raw := range PageTemplates {
		blocks, errs := pl.NormalizeBuilder(decodeJSON(t, raw), 1, "b")
		if len(errs) > 0 {
			t.Errorf("template %s: %v", name, errs)
		}
		if name != "blank" && !hasContent(blocks) {
			t.Errorf("template %s has no content", name)
		}
		if _, _, err := pl.renderBuilder(&request{site: &Site{ID: 1}}, blocks); err != nil {
			t.Errorf("template %s: %v", name, err)
		}
	}
}

// In the editor's preview every block carries its place in the form, nested ones too;
// the blocks of a reusable section belong to the reference.
func TestPreviewMarksBlockPaths(t *testing.T) {
	pl := &Plugin{}
	blocks, errs := pl.NormalizeBuilder(decodeJSON(t, `[
		{"type": "heading", "data": {"text": "Top"}},
		{"type": "columns", "data": {"layout": "1-1", "children": [{"blocks": []}, {"blocks": [{"type": "heading", "data": {"text": "Inner"}}]}]}}
	]`), 1, "b")
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	html, _, err := pl.renderBuilder(&request{site: &Site{ID: 1}}, blocks, true)
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	for _, want := range []string{`data-pb-path="0"`, `data-pb-path="1"`, `data-pb-path="1.1.0"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	public, _, _ := pl.renderBuilder(&request{site: &Site{ID: 1}}, blocks)
	if strings.Contains(string(public), "data-pb-path") {
		t.Error("public pages carry no editor marks")
	}

	r := &renderer{pl: pl, preview: true, seen: map[uint]bool{}, sections: map[uint][]Block{5: blocks}}
	ref, err := r.blocks([]Block{{Type: "section_ref", Data: map[string]any{"section_id": float64(5)}}}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(ref), "data-pb-path") != 1 || !strings.Contains(string(ref), `data-pb-path="0"`) {
		t.Fatalf("a section is one mark (the reference): %s", ref)
	}
}

// Grid column spans (MKSine's column_spans): each column's width of 12 per screen size,
// a missing size keeping the smaller one's; without spans the automatic layout stays.
func TestGridColumnSpans(t *testing.T) {
	pl := &Plugin{}
	blocks, errs := pl.NormalizeBuilder(decodeJSON(t, `[
		{"type": "grid_layout", "data": {"stack_on_mobile": "never", "column_spans": [{"span": 12, "span_md": 8}, {"span": 12, "span_md": 4, "span_lg": "3"}, {"span": 99}],
			"children": [{"blocks": [{"type": "heading", "data": {"text": "A"}}]}, {"blocks": []}, {"blocks": []}, {"blocks": []}]}},
		{"type": "grid_layout", "data": {"children": [{"blocks": []}, {"blocks": []}]}}
	]`), 1, "b")
	if len(errs) != 1 {
		t.Fatalf("a span over 12 is an error: %v", errs)
	}
	blocks, errs = pl.NormalizeBuilder(decodeJSON(t, `[
		{"type": "grid_layout", "data": {"stack_on_mobile": "never", "column_spans": [{"span": 12, "span_md": 8}, {"span": 12, "span_md": 4, "span_lg": "3"}],
			"children": [{"blocks": [{"type": "heading", "data": {"text": "A"}}]}, {"blocks": []}, {"blocks": []}]}},
		{"type": "grid_layout", "data": {"children": [{"blocks": []}, {"blocks": []}]}}
	]`), 1, "b")
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	html, _, err := pl.renderBuilder(&request{site: &Site{ID: 1}}, blocks)
	if err != nil {
		t.Fatal(err)
	}
	out := string(html)
	for _, want := range []string{`class="pb grid-12 stack-never`, `style="--s:12;--s-md:8"`, `style="--s:12;--s-md:4;--s-lg:3"`, `class="pb grid-auto`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in %s", want, out)
		}
	}
	// The third column takes the last entry's spans.
	if strings.Count(out, `--s-lg:3`) != 2 {
		t.Errorf("columns past the list repeat the last spans: %s", out)
	}
}

func TestCanvasChrome(t *testing.T) {
	var blocks []Block
	if err := json.Unmarshal([]byte(`[
		{"type":"heading","data":{"text":"Hi"}},
		{"type":"columns","data":{"layout":"equal","children":[{"blocks":[{"type":"text","data":{"content":"<p>A</p>"}}]},{"blocks":[]}]}}
	]`), &blocks); err != nil {
		t.Fatal(err)
	}
	r := &renderer{pl: &Plugin{}, seen: map[uint]bool{}, sections: map[uint][]Block{}, canvas: true, labels: map[string]string{"heading": `عنوان <x>`}}
	h, err := r.blocks(blocks, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	out := string(h)
	for _, want := range []string{
		`<div class="mks-list" data-pb-list="">`,
		`data-pb-insert="|0"`, `data-pb-insert="|1"`, `data-pb-insert="|2"`,
		`<div class="mks-block" data-pb-path="0"><div class="mks-tools"><span class="mks-name" data-pb-drag>عنوان &lt;x&gt;</span>`,
		`data-pb-act="edit"`, `data-pb-act="delete"`,
		// Each column is a list of its own; an empty one shows its insert row open.
		`data-pb-list="1.0"`, `data-pb-path="1.0.0"`, `<span class="mks-name" data-pb-drag>text</span>`,
		`<div class="mks-insert is-visible" data-pb-insert="1.1|0"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
}
