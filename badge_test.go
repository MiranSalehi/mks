package mks

import (
	"testing"

	"github.com/goravel/framework/contracts/database/orm"
)

type badgeTestComment struct {
	ID     int    `json:"id"`
	TeamID int    `json:"team_id"`
	Status string `json:"status"`
	Spam   bool   `json:"spam"`
}

// The sidebar badge counts only records the user can see: the current tenant and the
// policy scope apply before the badge's own condition.
func TestNavigationBadgeCountsVisibleRecords(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, sql := range []string{
		`CREATE TABLE badge_test_comments (id INTEGER PRIMARY KEY, team_id INTEGER, status TEXT, spam BOOLEAN)`,
		`INSERT INTO badge_test_comments (id, team_id, status, spam) VALUES
			(1, 10, 'pending', FALSE), (2, 10, 'pending', FALSE), (3, 10, 'approved', FALSE),
			(4, 10, 'pending', TRUE), (5, 20, 'pending', FALSE)`,
	} {
		if _, err := o.Query().Exec(sql); err != nil {
			t.Fatal(err)
		}
	}
	p := &Panel{opts: Options{Orm: o, Tenancy: &TenancyOptions{}}}
	c := Config{Name: "comments", Tenant: "team_id", NavigationBadgeColor: "warning",
		NavigationBadge: func(_ *HookContext, q orm.Query) orm.Query { return q.Where("status = ?", "pending") }}
	r := Define[badgeTestComment](c).Policy(Policy[badgeTestComment]{Scope: func(_ *HookContext, q orm.Query) orm.Query {
		return q.Where("spam = ?", false)
	}})
	reg := registered{res: r, cfg: c}
	inTeam := func(id uint) *HookContext { return &HookContext{tenant: &tenantState{current: &Tenant{ID: id}}} }

	if b := p.navigationBadge(reg, inTeam(10)); b == nil || b.Count != 2 || b.Color != "warning" {
		t.Fatalf("team 10 badge = %#v, want 2 pending (spam and other teams excluded)", b)
	}
	if b := p.navigationBadge(reg, inTeam(30)); b == nil || b.Count != 0 {
		t.Fatalf("a zero count is sent (the UI hides it), got %#v", b)
	}
	if b := p.navigationBadge(reg, &HookContext{tenant: &tenantState{}}); b == nil || b.Count != 0 {
		t.Fatalf("without a tenant there are no records, got %#v", b)
	}
	reg.cfg.NavigationBadge = func(_ *HookContext, q orm.Query) orm.Query { return q.Where("missing_column = ?", 1) }
	if b := p.navigationBadge(reg, inTeam(10)); b != nil {
		t.Fatal("a failing count shows no badge instead of failing the menu")
	}
}

func TestNavigationBadgeColorIsValidated(t *testing.T) {
	c := Config{Name: "comments", NavigationBadgeColor: "red"}
	if err := c.validate(); err == nil {
		t.Fatal("an unknown badge color is a configuration error")
	}
}
