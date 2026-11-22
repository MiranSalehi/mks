package access

import (
	"errors"
	"reflect"
	"testing"

	mks "github.com/MiranSalehi/mks"
)

type user struct{ id uint }

func (u *user) PanelPassword() string   { return "" }
func (u *user) PanelUser() mks.UserInfo { return mks.UserInfo{ID: u.id} }

func ctxFor(id uint) *mks.HookContext { return &mks.HookContext{User: &user{id: id}} }

func testPlugin(roles []Role) *Plugin {
	return &Plugin{
		loadRoles: func() ([]Role, error) { return roles, nil },
		catalog: func() []mks.PermissionGroup {
			return []mks.PermissionGroup{
				{Kind: "resource", Name: "posts", Permissions: []string{"posts.view_any", "posts.create", "posts.delete"}},
				{Kind: "custom", Permissions: []string{"cms.pages.manage"}},
			}
		},
	}
}

var defaultRoles = []Role{
	{ID: 1, Name: "admin", Super: true, Users: mks.NewJSON([]string{"1"})},
	{ID: 2, Name: "editor", Permissions: mks.NewJSON([]string{"posts.view_any", "posts.create"}), Users: mks.NewJSON([]string{"2", "4"})},
	{ID: 3, Name: "publisher", Permissions: mks.NewJSON([]string{"cms.pages.manage"}), Users: mks.NewJSON([]string{"4"})},
}

func TestGateUnionOfRolesAndSuper(t *testing.T) {
	pl := testPlugin(defaultRoles)
	cases := []struct {
		user uint
		perm string
		want bool
	}{
		{1, "anything.at.all", true},
		{2, "posts.create", true},
		{2, "posts.delete", false},
		{2, "cms.pages.manage", false},
		{4, "posts.create", true},
		{4, "cms.pages.manage", true},
		{9, "posts.view_any", false},
	}
	for _, c := range cases {
		if got := pl.Allowed(ctxFor(c.user), c.perm); got != c.want {
			t.Errorf("user %d %s = %v, want %v", c.user, c.perm, got, c.want)
		}
	}
	if pl.Allowed(nil, "posts.view_any") || pl.Allowed(&mks.HookContext{}, "posts.view_any") {
		t.Fatal("no user, no permission")
	}
	failing := &Plugin{loadRoles: func() ([]Role, error) { return nil, errors.New("db down") }}
	if failing.Allowed(ctxFor(1), "posts.view_any") {
		t.Fatal("a lookup error must deny")
	}
	owner := testPlugin(nil)
	owner.SuperUser = func(ctx *mks.HookContext) bool { return ctx.UserID() == uint(7) }
	if !owner.Allowed(ctxFor(7), "x") || owner.Allowed(ctxFor(8), "x") {
		t.Fatal("SuperUser must pass every check, only for its users")
	}
}

func TestCheckRoleStopsPrivilegeEscalation(t *testing.T) {
	pl := testPlugin(defaultRoles)
	editor, admin := ctxFor(2), ctxFor(1)
	ve := func(err error, field string) bool {
		var v mks.ValidationErrors
		return errors.As(err, &v) && v[field] != ""
	}

	// Only super users make super roles or edit them.
	if err := pl.checkRole(editor, map[string]any{"super": true}, nil); !ve(err, "super") {
		t.Fatalf("editor made a super role: %v", err)
	}
	if err := pl.checkRole(editor, map[string]any{"users": []any{"2"}}, &defaultRoles[0]); err == nil {
		t.Fatal("editor changed the super role")
	}
	if err := pl.checkRole(admin, map[string]any{"super": true}, nil); err != nil {
		t.Fatalf("admin: %v", err)
	}

	// Only permissions you hold yourself (or the role already had).
	if err := pl.checkRole(editor, map[string]any{"permissions": []any{"posts.delete"}}, nil); !ve(err, "permissions") {
		t.Fatalf("editor granted posts.delete: %v", err)
	}
	// A role holding more than you is out of reach, even just to add yourself to it.
	stronger := Role{ID: 5, Permissions: mks.NewJSON([]string{"posts.delete"})}
	if err := pl.checkRole(editor, map[string]any{"users": []any{"2"}}, &stronger); err == nil {
		t.Fatal("editor joined a role with permissions they lack")
	}
	weaker := Role{ID: 6, Permissions: mks.NewJSON([]string{"posts.view_any"})}
	data := map[string]any{"permissions": []any{"posts.view_any", "posts.create", "posts.create"}}
	if err := pl.checkRole(editor, data, &weaker); err != nil {
		t.Fatalf("editing a role within your own permissions: %v", err)
	}
	if got := data["permissions"].(mks.JSON[[]string]).V; !reflect.DeepEqual(got, []string{"posts.view_any", "posts.create"}) {
		t.Fatalf("permissions not normalized: %v", got)
	}

	// Unknown permissions and malformed lists are refused, even for admins.
	if err := pl.checkRole(admin, map[string]any{"permissions": []any{"posts.hack"}}, nil); !ve(err, "permissions") {
		t.Fatalf("unknown permission accepted: %v", err)
	}
	if err := pl.checkRole(admin, map[string]any{"permissions": "posts.create"}, nil); !ve(err, "permissions") {
		t.Fatalf("non-list accepted: %v", err)
	}

	// Users are kept as unique string keys.
	data = map[string]any{"users": []any{float64(3), "3", " 4 "}}
	if err := pl.checkRole(admin, data, nil); err != nil {
		t.Fatal(err)
	}
	if got := data["users"].(mks.JSON[[]string]).V; !reflect.DeepEqual(got, []string{"3", "4"}) {
		t.Fatalf("users = %v", got)
	}
	if err := pl.checkRole(admin, map[string]any{"users": []any{1.5}}, nil); !ve(err, "users") {
		t.Fatalf("fractional key accepted: %v", err)
	}
}

func TestOutranks(t *testing.T) {
	pl := testPlugin(defaultRoles)
	cases := []struct {
		actor  uint
		target any
		want   bool
	}{
		{1, 2, true},   // super over anyone
		{1, "1", true}, // super over super
		{2, 1, false},  // nobody but super over super
		{2, 4, false},  // 4 also holds cms.pages.manage
		{4, 2, true},   // 4 holds all of 2's permissions
		{2, 9, true},   // a user without roles
		{2, "2", true}, // oneself
		{9, 2, false},  // no permissions, the target has some
	}
	for _, c := range cases {
		if got := pl.Outranks(ctxFor(c.actor), c.target); got != c.want {
			t.Errorf("Outranks(%d, %v) = %v, want %v", c.actor, c.target, got, c.want)
		}
	}
	failing := &Plugin{loadRoles: func() ([]Role, error) { return nil, errors.New("db down") }}
	if failing.Outranks(ctxFor(1), 2) || pl.Outranks(nil, 2) {
		t.Fatal("errors and missing users deny")
	}
}

func TestRolesOfAndMayAssign(t *testing.T) {
	pl := testPlugin(defaultRoles)
	got, err := pl.RolesOf([]string{"2", "4", "9"})
	if err != nil || !reflect.DeepEqual(got["4"], []uint{2, 3}) || !reflect.DeepEqual(got["2"], []uint{2}) || len(got["9"]) != 0 {
		t.Fatalf("roles = %v, %v", got, err)
	}
	editor := ctxFor(2)
	if pl.mayAssign(editor, defaultRoles[0]) {
		t.Error("only super users give the super role")
	}
	if !pl.mayAssign(editor, defaultRoles[1]) {
		t.Error("a role whose permissions you hold can be given")
	}
	if pl.mayAssign(editor, defaultRoles[2]) {
		t.Error("a role with permissions you lack cannot be given")
	}
	if !pl.mayAssign(ctxFor(1), defaultRoles[0]) {
		t.Error("super users give anything")
	}
}
