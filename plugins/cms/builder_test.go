package cms

import (
	"encoding/json"
	"strings"
	"testing"
)

func decodeJSON(t *testing.T, s string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestNormalizeBuilderCleansEveryBlock(t *testing.T) {
	pl := &Plugin{}
	blocks, errs := pl.NormalizeBuilder(decodeJSON(t, `[
		{"type": "heading", "data": {"text": " Hello ", "level": "h9", "evil": "<script>", "spacing": "lg", "anchor": "intro"}},
		{"type": "script", "data": {"text": "dropped"}},
		{"type": "text", "data": {"content": "<p>ok</p><script>alert(1)</script><img src=x onerror=alert(2)>"}},
		{"type": "button", "data": {"text": "Go", "url": "javascript:alert(1)", "style": "neon", "new_tab": "true"}},
		{"type": "columns", "data": {"layout": "1-2", "children": [
			{"blocks": [{"type": "heading", "data": {"text": "Left"}}]},
			{"blocks": [{"type": "unknown"}]},
			{"blocks": []}
		]}},
		{"type": "features", "data": {"features": [{"title": "A", "link": "/about", "x": 1}, {"title": "B", "link": "data:text/html,1"}]}},
		{"type": "card", "data": {}}
	]`), 7, "builder_payload")
	if len(errs) > 0 {
		t.Fatalf("errors: %v", errs)
	}
	if len(blocks) != 6 {
		t.Fatalf("blocks = %d, want 6 (unknown dropped)", len(blocks))
	}
	h := blocks[0].Data
	if h["text"] != "Hello" || h["level"] != "h2" || h["evil"] != nil || h["spacing"] != "lg" || h["anchor"] != "intro" || h["visibility"] != "always" {
		t.Fatalf("heading = %#v", h)
	}
	if c := blocks[1].Data["content"].(string); strings.Contains(c, "script") || strings.Contains(c, "onerror") || !strings.Contains(c, "<p>ok</p>") {
		t.Fatalf("text not sanitized: %q", c)
	}
	b := blocks[2].Data
	if b["url"] != "#" || b["style"] != "primary" || b["new_tab"] != true {
		t.Fatalf("button = %#v", b)
	}
	slots := blocks[3].Data["children"].([]any)
	if len(slots) != 3 || len(slots[0].(map[string]any)["blocks"].([]Block)) != 1 || len(slots[1].(map[string]any)["blocks"].([]Block)) != 0 {
		t.Fatalf("columns = %#v", slots)
	}
	feats := blocks[4].Data["features"].([]any)
	if feats[0].(map[string]any)["link"] != "/about" || feats[1].(map[string]any)["link"] != "#" || feats[0].(map[string]any)["x"] != nil {
		t.Fatalf("features = %#v", feats)
	}
	if card := blocks[5].Data["children"].([]any); len(card) != 1 {
		t.Fatalf("card keeps its minimum slot: %#v", card)
	}
}

func TestNormalizeBuilderErrors(t *testing.T) {
	pl := &Plugin{}
	_, errs := pl.NormalizeBuilder(decodeJSON(t, `[
		{"type": "heading", "data": {"text": ""}},
		{"type": "heading", "data": {"text": "ok", "anchor": "1bad"}},
		{"type": "image", "data": {"image": "elsewhere/x.jpg"}},
		{"type": "slider", "data": {"autoplay_speed": 10}},
		{"type": "hero", "data": {"title": "t", "background_color": "red"}},
		"nope"
	]`), 7, "builder_payload")
	for _, key := range []string{
		"builder_payload.0.data.text", "builder_payload.1.data.anchor", "builder_payload.2.data.image",
		"builder_payload.3.data.autoplay_speed", "builder_payload.4.data.background_color", "builder_payload.5",
	} {
		if errs[key] == "" {
			t.Errorf("no error at %s (got %v)", key, errs)
		}
	}
	// Containers nest only so deep.
	deep := `{"type": "container_inset", "data": {"children": [{"blocks": [%s]}]}}`
	payload := `{"type": "heading", "data": {"text": "x"}}`
	for i := 0; i < maxBuilderDepth+1; i++ {
		payload = strings.Replace(deep, "%s", payload, 1)
	}
	if _, errs := pl.NormalizeBuilder(decodeJSON(t, "["+payload+"]"), 7, "b"); len(errs) == 0 {
		t.Fatal("unbounded nesting accepted")
	}
}

func TestHasContent(t *testing.T) {
	pl := &Plugin{}
	empty, _ := pl.NormalizeBuilder(decodeJSON(t, `[{"type": "spacer"}, {"type": "divider"}, {"type": "columns", "data": {}}]`), 1, "b")
	if hasContent(empty) {
		t.Fatal("spacers and empty columns are not content")
	}
	nested, _ := pl.NormalizeBuilder(decodeJSON(t, `[{"type": "columns", "data": {"children": [{"blocks": [{"type": "heading", "data": {"text": "x"}}]}]}}]`), 1, "b")
	if !hasContent(nested) {
		t.Fatal("a heading inside columns is content")
	}
}
