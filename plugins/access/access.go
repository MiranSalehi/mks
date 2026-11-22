// Package access is the panel's roles and permissions plugin (like Spatie permission and
// Filament Shield in MKSine). Roles hold permissions; users get roles; the plugin installs
// the panel's Gate, so every resource ability, action, widget and page needs a permission
// of one of the user's roles, on top of the resource's own policy. Its frontend half is
// @mks/mks/access.
//
//	p.Use(&access.Plugin{Orm: facades.Orm(), UsersResource: "users"})
//
// The table comes from a migration: access.CreateTable(facades.Schema()).
//
// A super role passes every permission check (policies still apply). Without any super
// user nobody could grant anything, so seed one (access.Grant) or set SuperUser.
package access

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	mks "github.com/MiranSalehi/mks"
)

// Table holds the roles.
const Table = "panel_roles"

// CreateTable creates the roles table; call it from a migration.
func CreateTable(s schema.Schema) error {
	return s.Create(Table, func(t schema.Blueprint) {
		t.ID()
		t.String("name", 100)
		t.Text("description").Nullable()
		t.Boolean("super").Default(false)
		t.Json("permissions").Nullable()
		// The users with this role, as user keys. Roles are few, so the gate reads all of
		// them at once and checks membership in Go (portable across databases).
		t.Json("users").Nullable()
		t.Timestamps()
		t.Unique("name")
	})
}

// Role is a named set of permissions given to users.
type Role struct {
	ID          uint               `gorm:"primaryKey" json:"id"`
	Name        string             `json:"name"`
	Description *string            `json:"description"`
	Super       bool               `json:"super"`
	Permissions mks.JSON[[]string] `json:"permissions"`
	Users       mks.JSON[[]string] `json:"users"`
	CreatedAt   *time.Time         `json:"created_at"`
	UpdatedAt   *time.Time         `json:"updated_at"`
}

func (Role) TableName() string { return Table }

// Plugin is the roles system. Orm is required.
type Plugin struct {
	Orm orm.Orm
	// UsersResource is the panel resource of the users (with a TitleColumn), so the roles
	// form can pick them. Default: "users".
	UsersResource string
	// SuperUser passes every check whatever the user's roles, e.g. an owner flag. Optional.
	SuperUser func(ctx *mks.HookContext) bool

	panel *mks.Panel
	// For tests: the roles and the permission catalog without a database or panel.
	loadRoles func() ([]Role, error)
	catalog   func() []mks.PermissionGroup
}

func (*Plugin) Name() string { return "access" }

// Register installs the gate and the roles resource.
func (pl *Plugin) Register(p *mks.Panel) {
	if pl.Orm == nil {
		panic("access: Orm is required")
	}
	pl.panel = p
	p.SetGate(pl.Allowed)
	p.Register(pl.rolesResource())
}

// Routes: GET /plugins/access/permissions, the permissions a role can hold.
func (pl *Plugin) Routes(r route.Router) {
	r.Get("/permissions", func(ctx http.Context) http.Response {
		hc := pl.panel.Context(ctx)
		if !pl.panel.Allowed(hc, "roles.view_any") {
			return pl.panel.Forbidden(ctx)
		}
		return ctx.Response().Json(200, http.Json{"data": pl.panel.Permissions()})
	})
}

type grantsKey struct{}

// grants is what a user may do: everything, or a set of permissions.
type grants struct {
	super       bool
	permissions map[string]bool
}

// Allowed is the gate: may the user of ctx use permission?
func (pl *Plugin) Allowed(ctx *mks.HookContext, permission string) bool {
	if ctx == nil || ctx.User == nil {
		return false
	}
	if pl.SuperUser != nil && pl.SuperUser(ctx) {
		return true
	}
	g, err := pl.grantsFor(ctx)
	if err != nil {
		// Fail closed: a role lookup error denies.
		return false
	}
	return g.super || g.permissions[permission]
}

// grantsFor reads the user's roles once per request.
func (pl *Plugin) grantsFor(ctx *mks.HookContext) (*grants, error) {
	if ctx.HTTP != nil {
		if g, ok := ctx.HTTP.Value(grantsKey{}).(*grants); ok {
			return g, nil
		}
	}
	roles, err := pl.roles()
	if err != nil {
		return nil, err
	}
	g := grantsOf(roles, fmt.Sprint(ctx.UserID()))
	if ctx.HTTP != nil {
		ctx.HTTP.WithValue(grantsKey{}, g)
	}
	return g, nil
}

func (pl *Plugin) roles() ([]Role, error) {
	if pl.loadRoles != nil {
		return pl.loadRoles()
	}
	var roles []Role
	err := pl.Orm.Query().Model(&Role{}).Find(&roles)
	return roles, err
}

// Outranks says whether the acting user holds everything the user with key target holds
// (a super target needs a super actor). Guard user editing with it: whoever can set a
// user's password or email gets that user's permissions. Lookup errors deny. SuperUser
// is not known for the target; protect such users in the app.
func (pl *Plugin) Outranks(ctx *mks.HookContext, target any) bool {
	if ctx == nil || ctx.User == nil {
		return false
	}
	if pl.SuperUser != nil && pl.SuperUser(ctx) {
		return true
	}
	actor, err := pl.grantsFor(ctx)
	if err != nil {
		return false
	}
	if actor.super {
		return true
	}
	roles, err := pl.roles()
	if err != nil {
		return false
	}
	t := grantsOf(roles, fmt.Sprint(target))
	if t.super {
		return false
	}
	for perm := range t.permissions {
		if !actor.permissions[perm] {
			return false
		}
	}
	return true
}

func grantsOf(roles []Role, user string) *grants {
	g := &grants{permissions: map[string]bool{}}
	if user == "" || user == "<nil>" {
		return g
	}
	for _, r := range roles {
		if !slices.Contains(r.Users.V, user) {
			continue
		}
		if r.Super {
			g.super = true
		}
		for _, perm := range r.Permissions.V {
			g.permissions[perm] = true
		}
	}
	return g
}

// Grant gives the user the role named name, creating the role if needed (super when
// created). For seeders and setup commands.
func Grant(q orm.Query, user any, name string) error {
	key := fmt.Sprint(user)
	var roles []Role
	if err := q.Model(&Role{}).Where("name = ?", name).Find(&roles); err != nil {
		return err
	}
	if len(roles) == 0 {
		now := time.Now()
		return q.Create(&Role{Name: name, Super: true, Permissions: mks.NewJSON([]string{}),
			Users: mks.NewJSON([]string{key}), CreatedAt: &now, UpdatedAt: &now})
	}
	role := roles[0]
	if slices.Contains(role.Users.V, key) {
		return nil
	}
	users := append(slices.Clone(role.Users.V), key)
	_, err := q.Model(&Role{}).Where("id = ?", role.ID).Update("users", mks.NewJSON(users))
	return err
}

// normalizeKeys turns submitted keys (numbers or strings) into unique strings.
func normalizeKeys(raw any, field string, known func(string) bool) ([]string, error) {
	list, ok := raw.([]any)
	if raw == nil {
		return []string{}, nil
	}
	if !ok {
		return nil, mks.ValidationErrors{field: "مقدار باید فهرست باشد."}
	}
	out := []string{}
	for _, v := range list {
		var s string
		switch x := v.(type) {
		case string:
			s = strings.TrimSpace(x)
		case float64:
			s = strings.TrimSuffix(fmt.Sprintf("%.0f", x), ".0")
			if float64(int64(x)) != x {
				return nil, mks.ValidationErrors{field: "مقدار نامعتبر است."}
			}
		default:
			return nil, mks.ValidationErrors{field: "مقدار نامعتبر است."}
		}
		if s == "" || (known != nil && !known(s)) {
			return nil, mks.ValidationErrors{field: fmt.Sprintf("«%s» شناخته‌شده نیست.", s)}
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out, nil
}
