package mks

import "testing"

type afterListRow struct {
	ID    int `json:"id"`
	Count int `gorm:"-" json:"count"`
}

// AfterRead gets pointers into the page, so what it fills is what the list sends.
func TestAfterReadFillsTheRecords(t *testing.T) {
	r := Define[afterListRow](Config{Name: "rows"}).AfterRead(func(_ *HookContext, rows []*afterListRow) error {
		for _, row := range rows {
			row.Count = row.ID * 10
		}
		return nil
	})
	a, b := &afterListRow{ID: 1}, &afterListRow{ID: 2}
	if err := afterRead(r, &HookContext{}, a, b); err != nil {
		t.Fatal(err)
	}
	if a.Count != 10 || b.Count != 20 {
		t.Fatalf("records = %+v %+v", a, b)
	}
}
