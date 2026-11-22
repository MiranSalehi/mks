package access

import (
	"fmt"
	"slices"

	"github.com/goravel/framework/contracts/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// Roles from the user form (MKSine's role checkboxes on the user): RolesOf reads them,
// SetUserRoles writes them with the rules of the roles page. A role can be given or taken
// away only by someone who could edit that role: a super role by a super user, any other
// by a user holding all of its permissions.

// RolesOf gives each user key their roles, for a resource that shows or derives from them
// (the access role ids are all a form needs; names and permissions come with this).
func (pl *Plugin) RolesGiven(keys []string) (map[string][]Role, error) {
	roles, err := pl.roles()
	if err != nil {
		return nil, err
	}
	out := map[string][]Role{}
	for _, k := range keys {
		out[k] = []Role{}
	}
	for _, r := range roles {
		for _, u := range r.Users.V {
			if _, want := out[u]; want {
				out[u] = append(out[u], r)
			}
		}
	}
	return out, nil
}

// RolesOf gives the role ids of each user key.
func (pl *Plugin) RolesOf(keys []string) (map[string][]uint, error) {
	roles, err := pl.roles()
	if err != nil {
		return nil, err
	}
	out := map[string][]uint{}
	for _, k := range keys {
		out[k] = []uint{}
	}
	for _, r := range roles {
		for _, u := range r.Users.V {
			if _, want := out[u]; want {
				out[u] = append(out[u], r.ID)
			}
		}
	}
	return out, nil
}

// mayAssign: may the acting user give or take away role r?
func (pl *Plugin) mayAssign(ctx *mks.HookContext, r Role) bool {
	if pl.isSuper(ctx) {
		return true
	}
	if r.Super {
		return false
	}
	for _, perm := range r.Permissions.V {
		if !pl.Allowed(ctx, perm) {
			return false
		}
	}
	return true
}

// SetUserRoles makes the user's roles exactly roleIDs, in q (the save's transaction).
func (pl *Plugin) SetUserRoles(ctx *mks.HookContext, q orm.Query, user any, roleIDs []uint) error {
	key := fmt.Sprint(user)
	if key == "" || key == "0" || key == "<nil>" {
		return mks.ValidationErrors{"roles": "کاربر معتبر نیست."}
	}
	var roles []Role
	if err := q.Model(&Role{}).Find(&roles); err != nil {
		return err
	}
	known := map[uint]bool{}
	for _, r := range roles {
		known[r.ID] = true
	}
	for _, id := range roleIDs {
		if !known[id] {
			return mks.ValidationErrors{"roles": "نقش انتخاب‌شده وجود ندارد."}
		}
	}
	for _, r := range roles {
		has := slices.Contains(r.Users.V, key)
		want := slices.Contains(roleIDs, r.ID)
		if has == want {
			continue
		}
		if !pl.mayAssign(ctx, r) {
			return mks.ValidationErrors{"roles": fmt.Sprintf("نقش «%s» مجوزهایی دارد که شما ندارید؛ نمی‌توانید آن را بدهید یا بگیرید.", r.Name)}
		}
		users := slices.DeleteFunc(slices.Clone(r.Users.V), func(u string) bool { return u == key })
		if want {
			users = append(users, key)
		}
		if _, err := q.Model(&Role{}).Where("id = ?", r.ID).Update("users", mks.NewJSON(users)); err != nil {
			return err
		}
	}
	return nil
}
