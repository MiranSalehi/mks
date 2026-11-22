package mks

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/goravel/framework/contracts/database/orm"
)

type optionTestPage struct {
	ID     int    `json:"id"`
	TeamID int    `json:"team_id"`
	Title  string `json:"title"`
	Hidden bool   `json:"hidden"`
}

// Options of a resource itself (GET /resources/{resource}/options) go through the same
// scope as its list: the current tenant and the policy scope both apply.
func TestResourceOptionsApplyTenantAndPolicyScope(t *testing.T) {
	o := newSecurityTestORM(t)
	seed := []string{
		`CREATE TABLE option_test_pages (id INTEGER PRIMARY KEY, team_id INTEGER, title TEXT, hidden BOOLEAN)`,
		`INSERT INTO option_test_pages (id, team_id, title, hidden) VALUES
			(1, 10, 'About', FALSE),
			(2, 10, 'Contact', FALSE),
			(3, 20, 'Other team', FALSE),
			(4, 10, 'Secret draft', TRUE)`,
	}
	for i := 0; i < 30; i++ {
		seed = append(seed, fmt.Sprintf(`INSERT INTO option_test_pages (id, team_id, title, hidden) VALUES (%d, 30, 'Page %02d', FALSE)`, 100+i, i))
	}
	for _, sql := range seed {
		if _, err := o.Query().Exec(sql); err != nil {
			t.Fatalf("seed options test: %v", err)
		}
	}

	p := &Panel{opts: Options{Orm: o, Tenancy: &TenancyOptions{}}}
	c := Config{Name: "pages", Key: "id", Tenant: "team_id", TitleColumn: "title", Searchable: []string{"title"}}
	r := Define[optionTestPage](c).Policy(Policy[optionTestPage]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
		return q.Where("hidden = ?", false)
	}})
	target := registered{res: r, cfg: c}
	inTeam := func(id uint) *HookContext { return &HookContext{tenant: &tenantState{current: &Tenant{ID: id}}} }
	labels := func(opts []option) []string {
		out := make([]string, len(opts))
		for i, o := range opts {
			out[i] = o.Label
		}
		return out
	}

	q := func(team uint) orm.Query { return p.scope(r, c, inTeam(team), o.Query()) }

	got, more, err := optionsFor(q(10), target, "", nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"About", "Contact"}; !reflect.DeepEqual(labels(got), want) || more {
		t.Fatalf("team 10 options = %v (more %v), want %v without hidden or other-team pages", labels(got), more, want)
	}
	if got[0].Value != 1 {
		t.Fatalf("option value = %#v, want the record key 1", got[0].Value)
	}

	got, _, err = optionsFor(q(10), target, "cont", nil, 1)
	if err != nil || !reflect.DeepEqual(labels(got), []string{"Contact"}) {
		t.Fatalf("search = %v, %v", labels(got), err)
	}

	// Labels by key: keys of another team or outside the policy scope are not resolved.
	got, more, err = optionsFor(q(10), target, "", []string{"1", "3", "4"}, 1)
	if err != nil || !reflect.DeepEqual(labels(got), []string{"About"}) || more {
		t.Fatalf("values = %v (more %v), %v; want only the visible key", labels(got), more, err)
	}

	got, more, err = optionsFor(q(30), target, "", nil, 1)
	if err != nil || len(got) != optionsPageSize || !more {
		t.Fatalf("first page: %d options, more %v, %v", len(got), more, err)
	}
	got, more, err = optionsFor(q(30), target, "", nil, 2)
	if err != nil || len(got) != 30-optionsPageSize || more || got[0].Label != fmt.Sprintf("Page %02d", optionsPageSize) {
		t.Fatalf("second page: %v, more %v, %v", labels(got), more, err)
	}
}
