package mks

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/goravel/framework/contracts/database/orm"
)

type scopedTestParent struct {
	ID       int               `json:"id"`
	TeamID   int               `json:"team_id"`
	Children []scopedTestChild `gorm:"foreignKey:ParentID" json:"children"`
}

type scopedTestChild struct {
	ID       int  `json:"id"`
	ParentID int  `json:"parent_id"`
	TeamID   int  `json:"team_id"`
	OwnerID  int  `json:"owner_id"`
	Shared   bool `json:"shared"`
}

func makeScopeTestPanel(t *testing.T) (*Panel, Resource, Config, *HookContext) {
	t.Helper()
	o := newSecurityTestORM(t)
	for _, sql := range []string{
		`CREATE TABLE scoped_test_parents (id INTEGER PRIMARY KEY, team_id INTEGER)`,
		`CREATE TABLE scoped_test_children (id INTEGER PRIMARY KEY, parent_id INTEGER, team_id INTEGER, owner_id INTEGER, shared BOOLEAN)`,
		`INSERT INTO scoped_test_parents (id, team_id) VALUES (1, 10), (2, 20)`,
		`INSERT INTO scoped_test_children (id, parent_id, team_id, owner_id, shared) VALUES
			(1, 1, 10, 7, FALSE),
			(2, 1, 10, 8, TRUE),
			(3, 1, 20, 8, TRUE),
			(4, 2, 10, 8, TRUE),
			(5, 1, 10, 8, FALSE)`,
	} {
		if _, err := o.Query().Exec(sql); err != nil {
			t.Fatalf("seed scope test: %v", err)
		}
	}

	p := &Panel{opts: Options{Orm: o, Tenancy: &TenancyOptions{}}}
	c := Config{Name: "children", Key: "id", Tenant: "team_id"}
	r := Define[scopedTestChild](c).Policy(Policy[scopedTestChild]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
		return q.Where("owner_id = ?", 7).OrWhere("shared = ?", true)
	}})
	hc := &HookContext{tenant: &tenantState{current: &Tenant{ID: 10}}}
	return p, r, c, hc
}

func TestPolicyScopeKeepsRootConstraints(t *testing.T) {
	p, r, c, hc := makeScopeTestPanel(t)
	for _, test := range []struct {
		name      string
		key       int
		keyBefore bool
		want      []int
	}{
		{name: "allows owner branch", key: 1, keyBefore: true, want: []int{1}},
		{name: "allows shared branch", key: 2, keyBefore: true, want: []int{2}},
		{name: "denies cross-tenant key", key: 3, keyBefore: true},
		{name: "denies policy-hidden key", key: 5, keyBefore: true},
		{name: "denies cross-tenant key added after scope", key: 3},
		{name: "denies policy-hidden key added after scope", key: 5},
	} {
		t.Run(test.name, func(t *testing.T) {
			q := p.opts.Orm.Query()
			if test.keyBefore {
				q = q.Where("id = ?", test.key)
			}
			q = p.scope(r, c, hc, q)
			if !test.keyBefore {
				q = q.Where("id = ?", test.key)
			}
			var rows []scopedTestChild
			if err := q.Find(&rows); err != nil {
				t.Fatal(err)
			}
			assertScopedChildIDs(t, rows, test.want)
		})
	}
}

func TestPolicyScopeExpandsWhereOnlyScopesInsideGroupedPredicate(t *testing.T) {
	p, _, c, hc := makeScopeTestPanel(t)
	r := Define[scopedTestChild](c).Policy(Policy[scopedTestChild]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
		return q.Scopes(func(query orm.Query) orm.Query {
			return query.Where("owner_id = ?", 7).OrWhere("shared = ?", true)
		})
	}})
	var rows []scopedTestChild
	if err := p.scope(r, c, hc, p.opts.Orm.Query()).Find(&rows); err != nil {
		t.Fatal(err)
	}
	assertScopedChildIDs(t, rows, []int{1, 2, 4})
}

func TestPolicyScopeComposesPredicatesAndScopes(t *testing.T) {
	allowed := func(q orm.Query) orm.Query {
		return q.Where("owner_id = ?", 7).OrWhere("shared = ?", true)
	}
	for _, test := range []struct {
		name  string
		scope func(orm.Query) orm.Query
	}{
		{"after Where", func(q orm.Query) orm.Query {
			return q.Where("id > ?", 0).Scopes(allowed)
		}},
		{"after OrWhere", func(q orm.Query) orm.Query {
			return q.OrWhere("id > ?", 0).Scopes(allowed)
		}},
		{"multiple scopes separated by predicates", func(q orm.Query) orm.Query {
			return q.Scopes(func(q orm.Query) orm.Query {
				return q.Where("id > ?", 0)
			}).Where("parent_id > ?", 0).Scopes(allowed)
		}},
		{"nested Scopes after predicate", func(q orm.Query) orm.Query {
			return q.Scopes(func(q orm.Query) orm.Query {
				return q.Where("id > ?", 0).Scopes(func(q orm.Query) orm.Query {
					return q.Where("parent_id > ?", 0).Scopes(allowed)
				})
			})
		}},
		{"Scopes inside Where group", func(q orm.Query) orm.Query {
			return q.Where(func(q orm.Query) orm.Query { return q.Scopes(allowed) })
		}},
		{"Scopes inside OrWhere group", func(q orm.Query) orm.Query {
			return q.Where("1 = 0").OrWhere(func(q orm.Query) orm.Query {
				return q.Where("id > ?", 0).Scopes(allowed)
			})
		}},
		{"nested groups inside Scopes", func(q orm.Query) orm.Query {
			return q.Where("id > ?", 0).Scopes(func(q orm.Query) orm.Query {
				return q.Where(func(q orm.Query) orm.Query {
					return q.Where(func(q orm.Query) orm.Query {
						return q.Where("parent_id > ?", 0).Scopes(allowed)
					}).Scopes(func(q orm.Query) orm.Query { return q.Where("id > ?", 0) })
				})
			})
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, _, c, hc := makeScopeTestPanel(t)
			r := Define[scopedTestChild](c).Policy(Policy[scopedTestChild]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
				return test.scope(q)
			}})
			var rows []scopedTestChild
			if err := p.scope(r, c, hc, p.opts.Orm.Query()).Find(&rows); err != nil {
				t.Fatal(err)
			}
			assertScopedChildIDs(t, rows, []int{1, 2, 4})

			// Both policy OR branches must stay within a key supplied on either side
			// of scope, including when nested scopes produce those branches.
			for _, keyBefore := range []bool{true, false} {
				for _, key := range []int{1, 2, 3, 5} {
					q := p.opts.Orm.Query()
					if keyBefore {
						q = q.Where("id = ?", key)
					}
					q = p.scope(r, c, hc, q)
					if !keyBefore {
						q = q.Where("id = ?", key)
					}
					rows = nil
					if err := q.Find(&rows); err != nil {
						t.Fatal(err)
					}
					var want []int
					if key == 1 || key == 2 {
						want = []int{key}
					}
					assertScopedChildIDs(t, rows, want)
				}
			}

			parentConfig := Config{Name: "parents", Key: "id", Relations: map[string]string{"children": "Children"}}
			parent := Define[scopedTestParent](parentConfig)
			p.order = []string{"parents", "children"}
			p.resources = map[string]registered{
				"parents":  {res: parent, cfg: parentConfig},
				"children": {res: r, cfg: c},
			}
			q, resp := p.preloadIncludes(nil, p.opts.Orm.Query().Where("id = ?", 1), parent, parentConfig, []string{"children"}, hc)
			if resp != nil {
				t.Fatalf("preload children: unexpected HTTP response %v", resp)
			}
			var row scopedTestParent
			if err := q.FirstOrFail(&row); err != nil {
				t.Fatal(err)
			}
			assertScopedChildIDs(t, row.Children, []int{1, 2})
		})
	}
}

func TestPolicyScopePreservesCumulativeScopeRestrictions(t *testing.T) {
	p, _, c, hc := makeScopeTestPanel(t)
	r := Define[scopedTestChild](c).Policy(Policy[scopedTestChild]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
		return q.Scopes(
			func(q orm.Query) orm.Query {
				return q.Where("shared = ?", true).Scopes(func(q orm.Query) orm.Query {
					return q.Where("parent_id = ?", 1)
				})
			},
			func(q orm.Query) orm.Query { return q.Where("owner_id = ?", 8) },
		)
	}})
	var rows []scopedTestChild
	if err := p.scope(r, c, hc, p.opts.Orm.Query()).Find(&rows); err != nil {
		t.Fatal(err)
	}
	assertScopedChildIDs(t, rows, []int{2})
}

func TestPolicyScopeExpandsScopesAfterPredicateFamilies(t *testing.T) {
	for _, test := range []struct {
		name      string
		predicate func(orm.Query) orm.Query
	}{
		{"WhereIn", func(q orm.Query) orm.Query { return q.WhereIn("id", []any{1, 2, 3, 4, 5}) }},
		{"OrWhereIn", func(q orm.Query) orm.Query { return q.OrWhereIn("id", []any{1, 2, 3, 4, 5}) }},
		{"WhereNotIn", func(q orm.Query) orm.Query { return q.WhereNotIn("id", []any{6}) }},
		{"OrWhereNotIn", func(q orm.Query) orm.Query { return q.OrWhereNotIn("id", []any{6}) }},
		{"WhereBetween", func(q orm.Query) orm.Query { return q.WhereBetween("id", 1, 5) }},
		{"OrWhereBetween", func(q orm.Query) orm.Query { return q.OrWhereBetween("id", 1, 5) }},
		{"WhereNotBetween", func(q orm.Query) orm.Query { return q.WhereNotBetween("id", 6, 10) }},
		{"OrWhereNotBetween", func(q orm.Query) orm.Query { return q.OrWhereNotBetween("id", 6, 10) }},
		{"WhereAll", func(q orm.Query) orm.Query { return q.WhereAll([]string{"id", "parent_id"}, ">", 0) }},
		{"WhereAny", func(q orm.Query) orm.Query { return q.WhereAny([]string{"id", "parent_id"}, ">", 0) }},
		{"WhereNone", func(q orm.Query) orm.Query { return q.WhereNone([]string{"id", "parent_id"}, 0) }},
		{"WhereNotNull", func(q orm.Query) orm.Query { return q.WhereNotNull("id") }},
		{"WhereNull", func(q orm.Query) orm.Query { return q.WhereNull("NULLIF(id, id)") }},
		{"OrWhereNull", func(q orm.Query) orm.Query { return q.OrWhereNull("NULLIF(id, id)") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			p, _, c, hc := makeScopeTestPanel(t)
			r := Define[scopedTestChild](c).Policy(Policy[scopedTestChild]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
				return test.predicate(q).Scopes(func(q orm.Query) orm.Query { return q.Where("owner_id = ?", 7) })
			}})
			var rows []scopedTestChild
			if err := p.scope(r, c, hc, p.opts.Orm.Query()).Find(&rows); err != nil {
				t.Fatal(err)
			}
			assertScopedChildIDs(t, rows, []int{1})
		})
	}
}

func TestPolicyScopeRejectsUnsupportedOperations(t *testing.T) {
	for _, test := range []struct {
		name  string
		scope func(orm.Query) orm.Query
	}{
		{"Join", func(q orm.Query) orm.Query {
			return q.Join("JOIN scoped_test_parents ON parent_id = scoped_test_parents.id")
		}},
		{"Having", func(q orm.Query) orm.Query { return q.Having("owner_id = ?", 7) }},
		{"Limit", func(q orm.Query) orm.Query { return q.Limit(0) }},
		{"Model", func(q orm.Query) orm.Query { return q.Model(&scopedTestChild{}) }},
		{"Table", func(q orm.Query) orm.Query { return q.Table("scoped_test_children") }},
		{"Raw", func(q orm.Query) orm.Query { return q.Raw("SELECT * FROM scoped_test_children WHERE owner_id = ?", 7) }},
		{"WithoutGlobalScopes", func(q orm.Query) orm.Query { return q.WithoutGlobalScopes() }},
		{"WithTrashed", func(q orm.Query) orm.Query { return q.WithTrashed() }},
		{"Select", func(q orm.Query) orm.Query { return q.Select("id") }},
		{"Delete", func(q orm.Query) orm.Query { _, _ = q.Delete(&scopedTestChild{}); return q }},
		{"Exec", func(q orm.Query) orm.Query { _, _ = q.Exec("DELETE FROM scoped_test_children"); return q }},
	} {
		for _, context := range []string{"direct", "after predicate in Scopes", "nested group"} {
			t.Run(test.name+"/"+context, func(t *testing.T) {
				p, _, c, hc := makeScopeTestPanel(t)
				r := Define[scopedTestChild](c).Policy(Policy[scopedTestChild]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
					switch context {
					case "after predicate in Scopes":
						return q.Where("1 = 1").Scopes(test.scope)
					case "nested group":
						return q.OrWhere(func(q orm.Query) orm.Query { return q.Scopes(test.scope) })
					default:
						return test.scope(q)
					}
				}})
				func() {
					defer func() {
						got := fmt.Sprint(recover())
						if !strings.Contains(got, "Policy.Scope") || !strings.Contains(got, test.name+" is unsupported") {
							t.Errorf("unsupported operation panic = %q", got)
						}
					}()
					var rows []scopedTestChild
					_ = p.scope(r, c, hc, p.opts.Orm.Query()).Find(&rows)
				}()
				// In particular, unsupported terminal operations must never execute.
				n, err := p.opts.Orm.Query().Model(&scopedTestChild{}).Count()
				if err != nil || n != 5 {
					t.Fatalf("rows after rejected operation = %d, error %v; want 5", n, err)
				}
			})
		}
	}
}

func TestPolicyScopeRejectsReplacementQueries(t *testing.T) {
	for _, context := range []string{"direct", "Scopes", "group"} {
		for _, replacement := range []string{"nil", "unrelated query", "outer group"} {
			if context != "group" && replacement == "outer group" {
				continue
			}
			t.Run(context+"/"+replacement, func(t *testing.T) {
				p, _, c, hc := makeScopeTestPanel(t)
				r := Define[scopedTestChild](c).Policy(Policy[scopedTestChild]{Scope: func(_ *HookContext, outer orm.Query) orm.Query {
					callback := func(orm.Query) orm.Query {
						switch replacement {
						case "unrelated query":
							return p.opts.Orm.Query().Where("1 = 1")
						case "outer group":
							return outer
						default:
							return nil
						}
					}
					switch context {
					case "Scopes":
						return outer.Scopes(callback)
					case "group":
						return outer.Where(callback)
					default:
						return callback(outer)
					}
				}})
				defer func() {
					if got := fmt.Sprint(recover()); !strings.Contains(got, "must return the supplied predicate query") {
						t.Errorf("replacement query panic = %q", got)
					}
				}()
				var rows []scopedTestChild
				_ = p.scope(r, c, hc, p.opts.Orm.Query()).Find(&rows)
			})
		}
	}
}

func TestPolicyScopeKeepsEagerLoadConstraints(t *testing.T) {
	p, child, childConfig, hc := makeScopeTestPanel(t)
	parentConfig := Config{Name: "parents", Key: "id", Relations: map[string]string{"children": "Children"}}
	parent := Define[scopedTestParent](parentConfig)
	p.order = []string{"parents", "children"}
	p.resources = map[string]registered{
		"parents":  {res: parent, cfg: parentConfig},
		"children": {res: child, cfg: childConfig},
	}

	q, resp := p.preloadIncludes(nil, p.opts.Orm.Query().Where("id = ?", 1), parent, parentConfig, []string{"children"}, hc)
	if resp != nil {
		t.Fatalf("preload children: unexpected HTTP response %v", resp)
	}
	var row scopedTestParent
	if err := q.FirstOrFail(&row); err != nil {
		t.Fatal(err)
	}
	assertScopedChildIDs(t, row.Children, []int{1, 2})
}

func TestPolicyScopeAllowsNoOp(t *testing.T) {
	p, _, c, hc := makeScopeTestPanel(t)
	r := Define[scopedTestChild](c).Policy(Policy[scopedTestChild]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
		return q
	}})
	var rows []scopedTestChild
	if err := p.scope(r, c, hc, p.opts.Orm.Query()).Find(&rows); err != nil {
		t.Fatal(err)
	}
	assertScopedChildIDs(t, rows, []int{1, 2, 4, 5})
}

func TestPolicyScopeDeniesWithoutCurrentTenant(t *testing.T) {
	p, r, c, _ := makeScopeTestPanel(t)
	var rows []scopedTestChild
	if err := p.scope(r, c, &HookContext{}, p.opts.Orm.Query()).Find(&rows); err != nil {
		t.Fatal(err)
	}
	assertScopedChildIDs(t, rows, nil)
}

func assertScopedChildIDs(t *testing.T, rows []scopedTestChild, want []int) {
	t.Helper()
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	slices.Sort(ids)
	if !slices.Equal(ids, want) {
		t.Fatalf("child IDs = %v, want %v", ids, want)
	}
}
