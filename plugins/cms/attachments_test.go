package cms

import (
	"reflect"
	"testing"
)

func TestParseMediaIDs(t *testing.T) {
	ids, err := parseMediaIDs([]any{float64(3), "1", float64(3), float64(2)})
	if err != nil || !reflect.DeepEqual(ids, []uint{3, 1, 2}) {
		t.Fatalf("ids = %v, %v (order kept, repeats dropped)", ids, err)
	}
	if ids, err := parseMediaIDs(nil); err != nil || len(ids) != 0 {
		t.Fatalf("nil clears the gallery: %v %v", ids, err)
	}
	for _, bad := range []any{"1,2", []any{"x"}, []any{float64(0)}, map[string]any{}} {
		if _, err := parseMediaIDs(bad); err == nil {
			t.Errorf("%v should be refused", bad)
		}
	}
	tooMany := make([]any, maxAttachments+1)
	for i := range tooMany {
		tooMany[i] = float64(i + 1)
	}
	if _, err := parseMediaIDs(tooMany); err == nil {
		t.Error("more than the limit is refused")
	}
}
