package mks

import (
	"testing"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	ormmock "github.com/goravel/framework/mocks/database/orm"
	"github.com/stretchr/testify/mock"
)

func TestPreloadIncludesAppliesTargetTenantAndPolicyScope(t *testing.T) {
	q := &ormmock.Query{}
	group := &ormmock.Query{}
	target := Define[tag](Config{Name: "tags", Tenant: "team_id"}).Policy(Policy[tag]{
		Scope: func(_ *HookContext, query contractsorm.Query) contractsorm.Query {
			return query.Where("is_visible = ?", true)
		},
	})
	source := Define[post](Config{Name: "posts"})
	p := &Panel{
		opts:  Options{Tenancy: &TenancyOptions{}},
		order: []string{"posts", "tags"},
		resources: map[string]registered{
			"posts": {res: source, cfg: Config{Name: "posts"}},
			"tags":  {res: target, cfg: Config{Name: "tags", Tenant: "team_id"}},
		},
	}
	hc := &HookContext{tenant: &tenantState{current: &Tenant{ID: 7}}}
	q.On("With", "Tags", mock.Anything).Return(q).Run(func(args mock.Arguments) {
		preload := args.Get(1).(func(contractsorm.Query) contractsorm.Query)
		preload(q)
	})
	q.On("Where", "team_id = ?", 7).Return(q)
	q.On("Where", mock.Anything).Return(q).Run(func(args mock.Arguments) {
		groupFn := args.Get(0).(func(contractsorm.Query) contractsorm.Query)
		groupFn(group)
	})
	group.On("Where", "is_visible = ?", true).Return(group)

	got, resp := p.preloadIncludes(nil, q, source, Config{Name: "posts", Relations: map[string]string{"tags": "Tags"}}, []string{"tags"}, hc)

	if resp != nil {
		t.Fatalf("preloadIncludes response = %v; want nil", resp)
	}
	if got != q {
		t.Fatal("preloadIncludes returned a different query")
	}
	q.AssertExpectations(t)
	group.AssertExpectations(t)
}

func TestFilterIncludedRecordsAppliesTargetViewPolicy(t *testing.T) {
	tags := Define[tag](Config{Name: "tags"}).Policy(Policy[tag]{
		View: func(_ *HookContext, record *tag) bool { return record.Name != "private" },
	})
	posts := Define[post](Config{Name: "posts"}).Policy(Policy[post]{
		View: func(_ *HookContext, record *post) bool { return record.Title != "private" },
	})
	p := &Panel{
		order: []string{"posts", "tags"},
		resources: map[string]registered{
			"posts": {res: posts, cfg: Config{Name: "posts"}},
			"tags":  {res: tags, cfg: Config{Name: "tags"}},
		},
	}
	c := Config{Name: "posts", Relations: map[string]string{"tags": "Tags", "parent": "Parent"}}
	record := &post{
		Tags:   []*tag{{Name: "public"}, {Name: "private"}, nil},
		Parent: &post{Title: "private"},
	}

	p.filterIncludedRecords(record, posts, c, []string{"tags", "parent"}, nil)

	if len(record.Tags) != 1 || record.Tags[0].Name != "public" {
		t.Fatalf("included tags = %#v; want only public tag", record.Tags)
	}
	if record.Parent != nil {
		t.Fatalf("included parent = %#v; want hidden record removed", record.Parent)
	}
}

func TestFilterIncludedRecordsRetainsVisibleRelations(t *testing.T) {
	tags := Define[tag](Config{Name: "tags"}).Policy(Policy[tag]{
		View: func(_ *HookContext, record *tag) bool { return record.Name == "public" },
	})
	posts := Define[post](Config{Name: "posts"})
	p := &Panel{
		order: []string{"posts", "tags"},
		resources: map[string]registered{
			"posts": {res: posts, cfg: Config{Name: "posts"}},
			"tags":  {res: tags, cfg: Config{Name: "tags"}},
		},
	}
	c := Config{Name: "posts", Relations: map[string]string{"tags": "Tags"}}
	record := &post{Tags: []*tag{{Name: "public"}}}

	p.filterIncludedRecords(record, posts, c, []string{"tags"}, nil)

	if len(record.Tags) != 1 || record.Tags[0].Name != "public" {
		t.Fatalf("visible included tags = %#v; want public tag retained", record.Tags)
	}
}
