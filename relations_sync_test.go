package mks

import (
	"errors"
	"testing"

	"github.com/goravel/framework/contracts/database/orm"
)

type syncTestParent struct {
	ID   int           `json:"id"`
	Tags []syncTestTag `gorm:"many2many:sync_test_parent_tags"`
}

type syncTestTag struct {
	ID      int `json:"id"`
	OwnerID int `json:"owner_id"`
}

type syncTestCustomParent struct {
	ID   int                 `json:"id"`
	Tags []syncTestCustomTag `gorm:"many2many:sync_test_custom_parent_tags;foreignKey:ID;joinForeignKey:ParentID;references:ExternalID;joinReferences:TagExternalID"`
}

type syncTestCustomTag struct {
	ID         int `json:"id"`
	ExternalID int `json:"external_id"`
	OwnerID    int `json:"owner_id"`
}

func TestApplySyncsPreservesOutOfScopeLinks(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, ddl := range []string{
		`CREATE TABLE sync_test_parents (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE sync_test_tags (id INTEGER PRIMARY KEY, owner_id INTEGER)`,
		`CREATE TABLE sync_test_parent_tags (sync_test_parent_id INTEGER, sync_test_tag_id INTEGER)`,
		`INSERT INTO sync_test_parents (id) VALUES (1)`,
		`INSERT INTO sync_test_tags (id, owner_id) VALUES (1, 7), (2, 8)`,
		`INSERT INTO sync_test_parent_tags (sync_test_parent_id, sync_test_tag_id) VALUES (1, 1), (1, 2)`,
	} {
		if _, err := o.Query().Exec(ddl); err != nil {
			t.Fatalf("seed relation sync test: %v", err)
		}
	}

	parentCfg := Config{Name: "sync_test_parents", Key: "id", Relations: map[string]string{"tags": "Tags"}}
	targetCfg := Config{Name: "sync_test_tags", Key: "id"}
	parent := Define[syncTestParent](parentCfg)
	target := Define[syncTestTag](targetCfg).Policy(Policy[syncTestTag]{
		Scope: func(_ *HookContext, q orm.Query) orm.Query { return q.Where("owner_id = ?", 7) },
	})
	p := newSyncTestPanel(o, parent, parentCfg, target, targetCfg)
	rel, ok := p.relation(parent, parentCfg, "tags")
	if !ok {
		t.Fatal("tags relation was not resolved")
	}

	if err := p.applySyncs(o.Query(), parent, &syncTestParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: []any{float64(1)}}}, &HookContext{}); err != nil {
		t.Fatalf("apply visible subset: %v", err)
	}
	assertSyncLinks(t, o.Query(), []int{1, 2})

	if err := p.applySyncs(o.Query(), parent, &syncTestParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: nil}}, &HookContext{}); err != nil {
		t.Fatalf("apply empty visible subset: %v", err)
	}
	assertSyncLinks(t, o.Query(), []int{2})
}

func TestApplySyncsTranslatesConfiguredKeysAndPreservesHiddenLinks(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, ddl := range []string{
		`CREATE TABLE sync_test_custom_parents (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE sync_test_custom_tags (id INTEGER PRIMARY KEY, external_id INTEGER UNIQUE, owner_id INTEGER)`,
		`CREATE TABLE sync_test_custom_parent_tags (parent_id INTEGER, tag_external_id INTEGER)`,
		`INSERT INTO sync_test_custom_parents (id) VALUES (1)`,
		`INSERT INTO sync_test_custom_tags (id, external_id, owner_id) VALUES (10, 101, 7), (20, 10, 8)`,
		`INSERT INTO sync_test_custom_parent_tags (parent_id, tag_external_id) VALUES (1, 10)`,
	} {
		if _, err := o.Query().Exec(ddl); err != nil {
			t.Fatalf("seed custom related key test: %v", err)
		}
	}

	parentCfg := Config{Name: "sync_test_custom_parents", Key: "id", Relations: map[string]string{"tags": "Tags"}}
	targetCfg := Config{Name: "sync_test_custom_tags", Key: "id"}
	parent := Define[syncTestCustomParent](parentCfg)
	target := Define[syncTestCustomTag](targetCfg).Policy(Policy[syncTestCustomTag]{
		Scope: func(_ *HookContext, q orm.Query) orm.Query { return q.Where("owner_id = ?", 7) },
	})
	p := newSyncTestPanel(o, parent, parentCfg, target, targetCfg)
	rel, ok := p.relation(parent, parentCfg, "tags")
	if !ok {
		t.Fatal("tags relation was not resolved")
	}
	link, _, err := p.relationLink(parent, rel)
	if err != nil {
		t.Fatal(err)
	}
	if link.relatedKey != "external_id" || link.joinRelated != "tag_external_id" {
		t.Fatalf("custom relation keys = %#v, want external_id -> tag_external_id", link)
	}

	if err := p.applySyncs(o.Query(), parent, &syncTestCustomParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: []any{float64(10)}}}, &HookContext{}); err != nil {
		t.Fatalf("apply selected target by configured key: %v", err)
	}
	assertCustomSyncLinks(t, o.Query(), []int{10, 101})

	// The submitted key domain is Config.Key (ID), not the custom related column.
	// ID 20 is out of scope even though the existing hidden join key equals 10.
	err = p.applySyncs(o.Query(), parent, &syncTestCustomParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: []any{float64(20)}}}, &HookContext{})
	var validationErr ValidationErrors
	if !errors.As(err, &validationErr) {
		t.Fatalf("out-of-scope custom-key submission error = %T %v, want ValidationErrors", err, err)
	}
	assertCustomSyncLinks(t, o.Query(), []int{10, 101})

	// Clearing removes only the visible custom-key target and keeps the hidden one.
	if err := p.applySyncs(o.Query(), parent, &syncTestCustomParent{ID: 1}, []relationSync{{field: "Tags", target: rel}}, &HookContext{}); err != nil {
		t.Fatalf("clear visible custom-key selection: %v", err)
	}
	assertCustomSyncLinks(t, o.Query(), []int{10})
}

func TestApplySyncsRejectsOutOfScopeSubmittedTarget(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, ddl := range []string{
		`CREATE TABLE sync_test_parents (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE sync_test_tags (id INTEGER PRIMARY KEY, owner_id INTEGER)`,
		`CREATE TABLE sync_test_parent_tags (sync_test_parent_id INTEGER, sync_test_tag_id INTEGER)`,
		`INSERT INTO sync_test_parents (id) VALUES (1)`,
		`INSERT INTO sync_test_tags (id, owner_id) VALUES (2, 8)`,
	} {
		if _, err := o.Query().Exec(ddl); err != nil {
			t.Fatalf("seed relation access test: %v", err)
		}
	}

	parentCfg := Config{Name: "sync_test_parents", Key: "id", Relations: map[string]string{"tags": "Tags"}}
	targetCfg := Config{Name: "sync_test_tags", Key: "id"}
	parent := Define[syncTestParent](parentCfg)
	target := Define[syncTestTag](targetCfg).Policy(Policy[syncTestTag]{
		Scope: func(_ *HookContext, q orm.Query) orm.Query { return q.Where("owner_id = ?", 7) },
	})
	p := newSyncTestPanel(o, parent, parentCfg, target, targetCfg)
	rel, ok := p.relation(parent, parentCfg, "tags")
	if !ok {
		t.Fatal("tags relation was not resolved")
	}
	assertSyncRejectsTarget(t, p, o, parent, rel, 2)
}

func TestRejectedRelationSyncRollsBackParentUpdate(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, ddl := range []string{
		`CREATE TABLE sync_test_parents (id INTEGER PRIMARY KEY, title TEXT NOT NULL)`,
		`CREATE TABLE sync_test_tags (id INTEGER PRIMARY KEY, owner_id INTEGER)`,
		`CREATE TABLE sync_test_parent_tags (sync_test_parent_id INTEGER, sync_test_tag_id INTEGER)`,
		`INSERT INTO sync_test_parents (id, title) VALUES (1, 'before')`,
		`INSERT INTO sync_test_tags (id, owner_id) VALUES (2, 8)`,
	} {
		if _, err := o.Query().Exec(ddl); err != nil {
			t.Fatalf("seed rollback test: %v", err)
		}
	}

	parentCfg := Config{Name: "sync_test_parents", Key: "id", Relations: map[string]string{"tags": "Tags"}}
	targetCfg := Config{Name: "sync_test_tags", Key: "id"}
	parent := Define[syncTestParent](parentCfg)
	target := Define[syncTestTag](targetCfg).Policy(Policy[syncTestTag]{
		Scope: func(_ *HookContext, q orm.Query) orm.Query { return q.Where("owner_id = ?", 7) },
	})
	p := newSyncTestPanel(o, parent, parentCfg, target, targetCfg)
	rel, ok := p.relation(parent, parentCfg, "tags")
	if !ok {
		t.Fatal("tags relation was not resolved")
	}

	err := o.Transaction(func(tx orm.Query) error {
		if _, err := tx.Table("sync_test_parents").Where("id = ?", 1).Update("title", "changed"); err != nil {
			return err
		}
		return p.applySyncs(tx, parent, &syncTestParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: []any{float64(2)}}}, &HookContext{})
	})
	if err == nil {
		t.Fatal("out-of-scope sync should fail the encompassing transaction")
	}

	var title string
	if err := o.Query().Table("sync_test_parents").Where("id = ?", 1).Pluck("title", &title); err != nil {
		t.Fatal(err)
	}
	if title != "before" {
		t.Fatalf("parent title after rejected relation sync = %q, want rollback to %q", title, "before")
	}
	assertSyncLinks(t, o.Query(), nil)
}

func TestApplySyncsRejectsViewDeniedSubmittedTarget(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, ddl := range []string{
		`CREATE TABLE sync_test_parents (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE sync_test_tags (id INTEGER PRIMARY KEY, owner_id INTEGER)`,
		`CREATE TABLE sync_test_parent_tags (sync_test_parent_id INTEGER, sync_test_tag_id INTEGER)`,
		`INSERT INTO sync_test_parents (id) VALUES (1)`,
		`INSERT INTO sync_test_tags (id, owner_id) VALUES (2, 7)`,
	} {
		if _, err := o.Query().Exec(ddl); err != nil {
			t.Fatalf("seed relation View test: %v", err)
		}
	}

	parentCfg := Config{Name: "sync_test_parents", Key: "id", Relations: map[string]string{"tags": "Tags"}}
	targetCfg := Config{Name: "sync_test_tags", Key: "id"}
	parent := Define[syncTestParent](parentCfg)
	target := Define[syncTestTag](targetCfg).Policy(Policy[syncTestTag]{
		Scope: func(_ *HookContext, q orm.Query) orm.Query { return q.Where("owner_id = ?", 7) },
		View:  func(_ *HookContext, record *syncTestTag) bool { return record.ID != 2 },
	})
	p := newSyncTestPanel(o, parent, parentCfg, target, targetCfg)
	rel, ok := p.relation(parent, parentCfg, "tags")
	if !ok {
		t.Fatal("tags relation was not resolved")
	}
	assertSyncRejectsTarget(t, p, o, parent, rel, 2)
}

func TestApplySyncsRetainsViewDeniedLinkedTarget(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, ddl := range []string{
		`CREATE TABLE sync_test_parents (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE sync_test_tags (id INTEGER PRIMARY KEY, owner_id INTEGER)`,
		`CREATE TABLE sync_test_parent_tags (sync_test_parent_id INTEGER, sync_test_tag_id INTEGER)`,
		`INSERT INTO sync_test_parents (id) VALUES (1)`,
		`INSERT INTO sync_test_tags (id, owner_id) VALUES (1, 7), (2, 7)`,
		`INSERT INTO sync_test_parent_tags (sync_test_parent_id, sync_test_tag_id) VALUES (1, 2)`,
	} {
		if _, err := o.Query().Exec(ddl); err != nil {
			t.Fatalf("seed relation View preservation test: %v", err)
		}
	}
	parentCfg := Config{Name: "sync_test_parents", Key: "id", Relations: map[string]string{"tags": "Tags"}}
	targetCfg := Config{Name: "sync_test_tags", Key: "id"}
	parent := Define[syncTestParent](parentCfg)
	target := Define[syncTestTag](targetCfg).Policy(Policy[syncTestTag]{
		View: func(_ *HookContext, record *syncTestTag) bool { return record.ID != 2 },
	})
	p := newSyncTestPanel(o, parent, parentCfg, target, targetCfg)
	rel, ok := p.relation(parent, parentCfg, "tags")
	if !ok {
		t.Fatal("tags relation was not resolved")
	}
	if err := p.applySyncs(o.Query(), parent, &syncTestParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: []any{float64(1)}}}, &HookContext{}); err != nil {
		t.Fatalf("apply visible subset: %v", err)
	}
	assertSyncLinks(t, o.Query(), []int{1, 2})
}

func TestApplySyncsViewAnyFalsePreservesLinksAndRejectsChanges(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, ddl := range []string{
		`CREATE TABLE sync_test_parents (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE sync_test_tags (id INTEGER PRIMARY KEY, owner_id INTEGER)`,
		`CREATE TABLE sync_test_parent_tags (sync_test_parent_id INTEGER, sync_test_tag_id INTEGER)`,
		`INSERT INTO sync_test_parents (id) VALUES (1)`,
		`INSERT INTO sync_test_tags (id, owner_id) VALUES (1, 7)`,
		`INSERT INTO sync_test_parent_tags (sync_test_parent_id, sync_test_tag_id) VALUES (1, 1)`,
	} {
		if _, err := o.Query().Exec(ddl); err != nil {
			t.Fatalf("seed ViewAny relation test: %v", err)
		}
	}
	parentCfg := Config{Name: "sync_test_parents", Key: "id", Relations: map[string]string{"tags": "Tags"}}
	targetCfg := Config{Name: "sync_test_tags", Key: "id"}
	parent := Define[syncTestParent](parentCfg)
	target := Define[syncTestTag](targetCfg).Policy(Policy[syncTestTag]{ViewAny: func(*HookContext) bool { return false }})
	p := newSyncTestPanel(o, parent, parentCfg, target, targetCfg)
	rel, ok := p.relation(parent, parentCfg, "tags")
	if !ok {
		t.Fatal("tags relation was not resolved")
	}
	if err := p.applySyncs(o.Query(), parent, &syncTestParent{ID: 1}, []relationSync{{field: "Tags", target: rel}}, &HookContext{}); err != nil {
		t.Fatalf("empty submission should retain inaccessible links: %v", err)
	}
	assertSyncLinks(t, o.Query(), []int{1})
	if err := p.applySyncs(o.Query(), parent, &syncTestParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: []any{float64(1)}}}, &HookContext{}); err == nil {
		t.Fatal("non-empty submission must be rejected without ViewAny")
	}
	assertSyncLinks(t, o.Query(), []int{1})
}

func TestApplySyncsDeduplicatesSubmittedKeys(t *testing.T) {
	o := newSecurityTestORM(t)
	for _, ddl := range []string{
		`CREATE TABLE sync_test_parents (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE sync_test_tags (id INTEGER PRIMARY KEY, owner_id INTEGER)`,
		`CREATE TABLE sync_test_parent_tags (sync_test_parent_id INTEGER, sync_test_tag_id INTEGER)`,
		`INSERT INTO sync_test_parents (id) VALUES (1)`,
		`INSERT INTO sync_test_tags (id, owner_id) VALUES (1, 7)`,
	} {
		if _, err := o.Query().Exec(ddl); err != nil {
			t.Fatalf("seed duplicate relation test: %v", err)
		}
	}
	parentCfg := Config{Name: "sync_test_parents", Key: "id", Relations: map[string]string{"tags": "Tags"}}
	targetCfg := Config{Name: "sync_test_tags", Key: "id"}
	parent := Define[syncTestParent](parentCfg)
	target := Define[syncTestTag](targetCfg)
	p := newSyncTestPanel(o, parent, parentCfg, target, targetCfg)
	rel, ok := p.relation(parent, parentCfg, "tags")
	if !ok {
		t.Fatal("tags relation was not resolved")
	}
	if err := p.applySyncs(o.Query(), parent, &syncTestParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: []any{float64(1), float64(1)}}}, &HookContext{}); err != nil {
		t.Fatalf("apply duplicate keys: %v", err)
	}
	assertSyncLinks(t, o.Query(), []int{1})
}

func newSyncTestPanel(o orm.Orm, parent Resource, parentCfg Config, target Resource, targetCfg Config) *Panel {
	return &Panel{
		opts:      Options{Orm: o},
		resources: map[string]registered{"sync_test_parents": {res: parent, cfg: parentCfg}, "sync_test_tags": {res: target, cfg: targetCfg}},
		order:     []string{"sync_test_parents", "sync_test_tags"},
	}
}

func assertSyncRejectsTarget(t *testing.T, p *Panel, o orm.Orm, parent Resource, rel relationInfo, id int) {
	t.Helper()
	if err := p.applySyncs(o.Query(), parent, &syncTestParent{ID: 1}, []relationSync{{field: "Tags", target: rel, keys: []any{float64(id)}}}, &HookContext{}); err == nil {
		t.Fatal("inaccessible target was accepted")
	} else {
		var ve ValidationErrors
		if !errors.As(err, &ve) {
			t.Fatalf("error = %T %v, want ValidationErrors", err, err)
		}
	}
	assertSyncLinks(t, o.Query(), nil)
}

func assertCustomSyncLinks(t *testing.T, q orm.Query, want []int) {
	t.Helper()
	var got []int
	if err := q.Table("sync_test_custom_parent_tags").Where("parent_id = ?", 1).OrderBy("tag_external_id").Pluck("tag_external_id", &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("custom join related keys = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("custom join related keys = %v, want %v", got, want)
		}
	}
}

func assertSyncLinks(t *testing.T, q orm.Query, want []int) {
	t.Helper()
	var got []int
	if err := q.Table("sync_test_parent_tags").Where("sync_test_parent_id = ?", 1).OrderBy("sync_test_tag_id").Pluck("sync_test_tag_id", &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("linked tag IDs = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("linked tag IDs = %v, want %v", got, want)
		}
	}
}
