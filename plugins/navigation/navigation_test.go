package navigation

import (
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	c, err := parse(map[string]any{
		"items": map[string]any{
			"r:posts":      map[string]any{"label": "  Articles ", "sort": 3},
			"r:empty":      map[string]any{},
			"p:admin_menu": map[string]any{"placement": "hidden", "sort": 1},
			"r:legacy":     map[string]any{"hidden": true},
			"r:moved":      map[string]any{"placement": "group", "group": " Content "},
			"r:stray":      map[string]any{"placement": "group"},
			"r:unknown":    map[string]any{"placement": "settings"},
		},
		"groups": map[string]any{"Content": map[string]any{"label": "Writing", "hidden": true}, "Empty": map[string]any{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Items["r:posts"].Label != "Articles" || c.Items["r:posts"].Sort != 3 {
		t.Fatalf("items = %#v", c.Items)
	}
	if _, kept := c.Items["r:empty"]; kept {
		t.Fatal("empty overrides are dropped")
	}
	if c.Items["p:admin_menu"].Placement != PlacementDefault || c.Items["p:admin_menu"].Sort != 1 {
		t.Fatal("the editor cannot be hidden, but can be moved")
	}
	if c.Items["r:legacy"].Placement != PlacementHidden {
		t.Fatal("the older hidden field becomes a placement")
	}
	if c.Items["r:moved"].Placement != PlacementGroup || c.Items["r:moved"].Group != "Content" {
		t.Fatalf("move = %#v", c.Items["r:moved"])
	}
	if c.Items["r:stray"].Placement != PlacementDefault {
		t.Fatal("a move without a target falls back to the default group")
	}
	if c.Items["r:unknown"].Placement != PlacementDefault {
		t.Fatal("an unknown placement keeps the item where the code puts it")
	}
	if !c.Groups["Content"].Hidden || len(c.Groups) != 1 {
		t.Fatalf("groups = %#v", c.Groups)
	}
	if _, err := parse(map[string]any{"items": map[string]any{"x": map[string]any{"label": strings.Repeat("a", 121)}}}); err == nil {
		t.Fatal("long label accepted")
	}
	if _, err := parse(map[string]any{"items": map[string]any{"x": map[string]any{"label": strings.Repeat("a", 9000)}}}); err == nil {
		t.Fatal("config over 8 KB accepted")
	}
	if _, err := parse("nope"); err == nil {
		t.Fatal("a string is not a config")
	}
}
