package cms

import (
	mks "github.com/MiranSalehi/mks"
)

// testUser is a panel user with a role, for the test plugin's Can.
type testUser struct {
	ID   uint
	Role string
}

func (u *testUser) PanelPassword() string { return "" }

func (u *testUser) PanelUser() mks.UserInfo {
	if u == nil {
		return mks.UserInfo{}
	}
	return mks.UserInfo{ID: u.ID}
}

// testPlugin grants by role: admins everything, editors pages and
// menus, everyone else nothing.
func testPlugin() *Plugin {
	return &Plugin{
		Can: func(ctx *mks.HookContext, ability Ability) bool {
			u, ok := ctx.User.(*testUser)
			if !ok || u == nil {
				return false
			}
			switch u.Role {
			case "admin":
				return true
			case "editor":
				return ability != PurgePages
			}
			return false
		},
	}
}
