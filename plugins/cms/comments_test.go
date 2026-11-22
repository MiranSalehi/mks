package cms

import (
	"testing"
	"time"

	"github.com/goravel/framework/contracts/cache"
)

// mapCache implements the two cache methods the comment limit uses.
type mapCache struct {
	cache.Cache
	values map[string]int
}

func (m *mapCache) GetInt(key string, def ...int) int {
	if v, ok := m.values[key]; ok {
		return v
	}
	if len(def) > 0 {
		return def[0]
	}
	return 0
}

func (m *mapCache) Put(key string, value any, _ time.Duration) error {
	m.values[key] = value.(int)
	return nil
}

func TestCommentLimitPerIPAndMinute(t *testing.T) {
	now := time.Date(2026, 11, 21, 10, 0, 5, 0, time.UTC)
	pl := &Plugin{Cache: &mapCache{values: map[string]int{}}, CommentsPerMinute: 3, Now: func() time.Time { return now }}
	for i := 0; i < 3; i++ {
		if !pl.allowComment("1.2.3.4") {
			t.Fatalf("comment %d refused", i+1)
		}
	}
	if pl.allowComment("1.2.3.4") {
		t.Fatal("the 4th comment in a minute was allowed")
	}
	if !pl.allowComment("5.6.7.8") {
		t.Fatal("another address shares the limit")
	}
	now = now.Add(time.Minute)
	if !pl.allowComment("1.2.3.4") {
		t.Fatal("the next minute should start over")
	}
	pl.CommentsPerMinute = 0 // the default, 5
	now = now.Add(time.Minute)
	for i := 0; i < 5; i++ {
		if !pl.allowComment("9.9.9.9") {
			t.Fatalf("default limit refused comment %d", i+1)
		}
	}
	if pl.allowComment("9.9.9.9") {
		t.Fatal("default limit is 5")
	}
}

func TestValidEmail(t *testing.T) {
	for _, ok := range []string{"a@example.com", "first.last@sub.example.ir"} {
		if !validEmail(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"", "nope", "a@b", "Ali <a@example.com>", "a@example.com\n", " a@example.com"} {
		if validEmail(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
}
