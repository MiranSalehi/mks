package access

import (
	"strings"

	mks "github.com/MiranSalehi/mks"
)

func (pl *Plugin) usersResource() string {
	if pl.UsersResource != "" {
		return pl.UsersResource
	}
	return "users"
}

func (pl *Plugin) rolesResource() mks.ModelResource[Role] {
	return mks.Define[Role](mks.Config{
		Name:        "roles",
		Searchable:  []string{"name"},
		Sortable:    []string{"id", "name", "updated_at"},
		Fillable:    []string{"name", "description", "super", "permissions", "users"},
		TitleColumn: "name",
		DefaultSort: "name",
		Deletable:   true,
		Rules: map[string]string{
			"name":        "required|max_len:100",
			"description": "max_len:500",
			"permissions": "array",
			"users":       "array",
		},
	}).
		BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
			for field, value := range map[string]any{"permissions": []any{}, "users": []any{}, "super": false} {
				if _, sent := data[field]; !sent {
					data[field] = value
				}
			}
			return pl.checkRole(ctx, data, nil)
		}).
		BeforeUpdate(func(ctx *mks.HookContext, role *Role, data map[string]any) error {
			return pl.checkRole(ctx, data, role)
		}).
		BeforeDelete(func(ctx *mks.HookContext, role *Role) error {
			if pl.isSuper(ctx) {
				return nil
			}
			if role.Super {
				return mks.Reject("فقط کاربر با نقش کامل می‌تواند نقش کامل را حذف کند.")
			}
			for _, perm := range role.Permissions.V {
				if !pl.Allowed(ctx, perm) {
					return mks.Reject("این نقش مجوزهایی دارد که شما ندارید.")
				}
			}
			return nil
		})
}

// isSuper: the acting user passes every check.
func (pl *Plugin) isSuper(ctx *mks.HookContext) bool {
	if ctx == nil || ctx.User == nil {
		return false
	}
	if pl.SuperUser != nil && pl.SuperUser(ctx) {
		return true
	}
	g, err := pl.grantsFor(ctx)
	return err == nil && g.super
}

// checkRole normalizes a role and stops privilege escalation: only a super user may make
// or change a super role, and a user can only hand out permissions they hold themselves.
func (pl *Plugin) checkRole(ctx *mks.HookContext, data map[string]any, stored *Role) error {
	if name, ok := data["name"].(string); ok {
		name = strings.TrimSpace(name)
		data["name"] = name
		q := pl.Orm.Query().Model(&Role{}).Where("name = ?", name)
		if stored != nil {
			q = q.Where("id <> ?", stored.ID)
		}
		n, err := q.Count()
		if err != nil {
			return err
		}
		if n > 0 {
			return mks.ValidationErrors{"name": "نقشی با این نام وجود دارد."}
		}
	}
	super := pl.isSuper(ctx)
	if stored != nil && stored.Super && !super {
		return mks.Reject("فقط کاربر با نقش کامل می‌تواند نقش کامل را تغییر دهد.")
	}
	// A role with permissions you lack is out of reach: changing it (even only its users,
	// e.g. adding yourself) would hand out what you do not hold.
	if stored != nil && !super {
		for _, perm := range stored.Permissions.V {
			if !pl.Allowed(ctx, perm) {
				return mks.Reject("این نقش مجوزهایی دارد که شما ندارید؛ فقط کاربری با این مجوزها می‌تواند آن را تغییر دهد.")
			}
		}
	}
	if v, sent := data["super"]; sent {
		b, ok := v.(bool)
		if !ok {
			return mks.ValidationErrors{"super": "مقدار نامعتبر است."}
		}
		if b && !super {
			return mks.ValidationErrors{"super": "فقط کاربر با نقش کامل می‌تواند نقش کامل بسازد."}
		}
	}
	if raw, sent := data["permissions"]; sent {
		catalog := map[string]bool{}
		groups := pl.catalog
		if groups == nil {
			groups = pl.panel.Permissions
		}
		for _, g := range groups() {
			for _, perm := range g.Permissions {
				catalog[perm] = true
			}
		}
		perms, err := normalizeKeys(raw, "permissions", func(s string) bool { return catalog[s] })
		if err != nil {
			return err
		}
		if !super {
			for _, perm := range perms {
				if !pl.Allowed(ctx, perm) {
					return mks.ValidationErrors{"permissions": "نمی‌توانید مجوزی را بدهید که خودتان ندارید: " + perm}
				}
			}
		}
		data["permissions"] = mks.NewJSON(perms)
	}
	if raw, sent := data["users"]; sent {
		users, err := normalizeKeys(raw, "users", func(s string) bool { return len(s) <= 64 })
		if err != nil {
			return err
		}
		data["users"] = mks.NewJSON(users)
	}
	return nil
}
