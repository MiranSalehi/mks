package mks

import (
	"reflect"
	"testing"
	"time"

	contractsorm "github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/database/orm"
	ormmock "github.com/goravel/framework/mocks/database/orm"
	"github.com/stretchr/testify/mock"
)

type includeOnlyUser struct {
	orm.Model
	Name     string `json:"name"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	Password string `json:"-"`
}

type includeOnlyPost struct {
	orm.Model
	AuthorID uint             `json:"author_id"`
	Author   *includeOnlyUser `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
}

type includeMarshalingPost struct{ includeOnlyPost }

func (includeMarshalingPost) MarshalJSON() ([]byte, error) {
	return []byte(`{"author":{"id":17,"name":"Ada","email":"private@example.test"}}`), nil
}

type includeMarshalingUser struct {
	orm.Model
	Name string `json:"name"`
}

func (includeMarshalingUser) MarshalJSON() ([]byte, error) {
	return []byte(`{"id":17,"name":"Ada","email":"private@example.test"}`), nil
}

type includeMarshalingUserPost struct {
	orm.Model
	AuthorID uint                   `json:"author_id"`
	Author   *includeMarshalingUser `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
}

type includeMarshaledSecret string

func (includeMarshaledSecret) MarshalJSON() ([]byte, error) {
	return []byte(`{"secret":"sensitive"}`), nil
}

type includeMarshalingFieldUser struct {
	orm.Model
	Name   string                 `json:"name"`
	Secret includeMarshaledSecret `json:"secret"`
}

type includeMarshalingFieldPost struct {
	orm.Model
	AuthorID uint                        `json:"author_id"`
	Author   *includeMarshalingFieldUser `gorm:"foreignKey:AuthorID" json:"author,omitempty"`
}

func includeOnlyConfig() Config {
	return Config{
		Name:      "posts",
		Relations: map[string]string{"author": "Author"},
		IncludeTargets: map[string]IncludeTarget{
			"author": {
				Resource: Define[includeOnlyUser](Config{Name: "authors"}),
				Fields:   []string{"id", "name"},
			},
		},
	}
}

func TestCheckIncludeTargetsRequiresTypedFieldAllowlist(t *testing.T) {
	if !safeIncludeField(reflect.TypeFor[time.Time]()) {
		t.Fatal("time.Time should remain a safe scalar include field")
	}
	if err := checkIncludeTargets(&includeOnlyPost{}, includeOnlyConfig()); err != nil {
		t.Fatalf("valid read-only include config rejected: %v", err)
	}

	bad := includeOnlyConfig()
	delete(bad.Relations, "author")
	if err := checkIncludeTargets(&includeOnlyPost{}, bad); err == nil {
		t.Fatal("include target without a configured relation must be rejected")
	}

	bad = includeOnlyConfig()
	target := bad.IncludeTargets["author"]
	target.Fields = []string{"password"}
	bad.IncludeTargets["author"] = target
	if err := checkIncludeTargets(&includeOnlyPost{}, bad); err == nil {
		t.Fatal("json-hidden fields must not be available to include projection")
	}

	if err := checkIncludeTargets(&includeMarshalingPost{}, includeOnlyConfig()); err == nil {
		t.Fatal("custom-marshaled root models must not bypass include projection")
	}
	marshaledTarget := Config{
		Name:      "posts",
		Relations: map[string]string{"author": "Author"},
		IncludeTargets: map[string]IncludeTarget{
			"author": {Resource: Define[includeMarshalingUser](Config{Name: "authors"}), Fields: []string{"id", "name"}},
		},
	}
	if err := checkIncludeTargets(&includeMarshalingUserPost{}, marshaledTarget); err == nil {
		t.Fatal("custom-marshaled include targets must not bypass field projection")
	}
	marshaledFieldTarget := Config{
		Name:      "posts",
		Relations: map[string]string{"author": "Author"},
		IncludeTargets: map[string]IncludeTarget{
			"author": {Resource: Define[includeMarshalingFieldUser](Config{Name: "authors"}), Fields: []string{"name", "secret"}},
		},
	}
	if err := checkIncludeTargets(&includeMarshalingFieldPost{}, marshaledFieldTarget); err == nil {
		t.Fatal("custom-marshaled primitive fields must not bypass field projection")
	}
}

func TestIncludeTargetSerializesOnlyAllowlistedFields(t *testing.T) {
	resource := Define[includeOnlyPost](Config{Name: "posts"})
	cfg := includeOnlyConfig()
	p := &Panel{}
	record := &includeOnlyPost{Author: &includeOnlyUser{
		Model:    orm.Model{ID: 17},
		Name:     "Ada",
		Email:    "ada@example.test",
		Role:     "admin",
		Password: "must-not-appear",
	}}

	encoded, ok := p.encodeIncluded(reflect.ValueOf(record), forcedFields(cfg, []string{"author"}), resource, cfg, []string{"author"}).(map[string]any)
	if !ok {
		t.Fatalf("encoded result has type %T", encoded)
	}
	author, ok := encoded["author"].(map[string]any)
	if !ok {
		t.Fatalf("author result has type %T", encoded["author"])
	}
	if len(author) != 2 || author["id"] != uint(17) || author["name"] != "Ada" {
		t.Fatalf("projected author = %#v; want only id and name", author)
	}
}

func TestFilterIncludedRecordsAppliesIncludeTargetViewPolicy(t *testing.T) {
	cfg := includeOnlyConfig()
	target := cfg.IncludeTargets["author"]
	target.Resource = Define[includeOnlyUser](Config{Name: "authors"}).Policy(Policy[includeOnlyUser]{
		View: func(_ *HookContext, user *includeOnlyUser) bool { return user.Name != "private" },
	})
	cfg.IncludeTargets["author"] = target
	posts := Define[includeOnlyPost](Config{Name: "posts"})
	p := &Panel{}
	record := &includeOnlyPost{Author: &includeOnlyUser{Name: "private"}}

	p.filterIncludedRecords(record, posts, cfg, []string{"author"}, nil)
	if record.Author != nil {
		t.Fatalf("View-denied included author = %#v; want nil", record.Author)
	}
}

func TestIncludeTargetIsNotExposedAsResourceOrRelationManager(t *testing.T) {
	posts := Define[includeOnlyPost](Config{Name: "posts"})
	cfg := includeOnlyConfig()
	p := &Panel{resources: map[string]registered{}, order: []string{}}
	if _, ok := p.relation(posts, cfg, "author"); ok {
		t.Fatal("include-only target must not resolve as a public relation target")
	}
	if _, _, ok := p.includeRelation(posts, cfg, "author"); !ok {
		t.Fatal("explicit include target did not resolve for read-only includes")
	}
}

func TestIncludeTargetPreloadUsesItsPolicyScope(t *testing.T) {
	q := &ormmock.Query{}
	group := &ormmock.Query{}
	userResource := Define[includeOnlyUser](Config{Name: "authors"}).Policy(Policy[includeOnlyUser]{
		Scope: func(_ *HookContext, query contractsorm.Query) contractsorm.Query {
			return query.Where("active = ?", true)
		},
	})
	cfg := Config{
		Name:      "posts",
		Relations: map[string]string{"author": "Author"},
		IncludeTargets: map[string]IncludeTarget{
			"author": {Resource: userResource, Fields: []string{"id", "name"}},
		},
	}
	posts := Define[includeOnlyPost](Config{Name: "posts"})
	p := &Panel{resources: map[string]registered{}}
	q.On("With", "Author", mock.Anything).Return(q).Run(func(args mock.Arguments) {
		preload := args.Get(1).(func(contractsorm.Query) contractsorm.Query)
		preload(q)
	})
	q.On("Where", mock.Anything).Return(q).Run(func(args mock.Arguments) {
		groupFn := args.Get(0).(func(contractsorm.Query) contractsorm.Query)
		groupFn(group)
	})
	group.On("Where", "active = ?", true).Return(group)

	got, response := p.preloadIncludes(nil, q, posts, cfg, []string{"author"}, nil)
	if response != nil || got != q {
		t.Fatalf("preload include = (%v, %v); want original query and no response", got, response)
	}
	q.AssertExpectations(t)
	group.AssertExpectations(t)
}
