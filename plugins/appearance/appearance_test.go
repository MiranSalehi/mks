package appearance

import "testing"

func TestParseLook(t *testing.T) {
	l, err := parseLook(map[string]any{"preset": "classic", "primary": "emerald", "rail": "", "unknown": "x", "field": "floating", "label": "side"})
	if err != nil || l.Preset != "classic" || l.Primary != "emerald" || l.Rail != "" || l.Field != "floating" || l.Label != "side" {
		t.Fatalf("look = %+v, %v", l, err)
	}
	for _, bad := range []any{"x", map[string]any{"primary": "red"}, map[string]any{"mode": "<script>"}, map[string]any{"field": "boxed"}} {
		if _, err := parseLook(bad); err == nil {
			t.Errorf("%v should be refused", bad)
		}
	}
}
