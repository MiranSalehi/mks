package cms

import (
	"testing"

	mks "github.com/MiranSalehi/mks"
)

// Custom JavaScript needs both the ability and Plugin.CustomJS: on the panel's origin it
// could call the panel API as whoever opens the site.
func TestCustomJSNeedsTheOption(t *testing.T) {
	pl := testPlugin()
	admin := &mks.HookContext{User: &testUser{ID: 1, Role: "admin"}}
	if pl.canCode(admin) {
		t.Fatal("custom JS is off unless the plugin enables it")
	}
	pl.CustomJS = true
	if !pl.canCode(admin) {
		t.Fatal("with CustomJS, holders of cms.themes.code may set it")
	}
	if pl.canCode(&mks.HookContext{User: &testUser{ID: 2, Role: "viewer"}}) {
		t.Fatal("others may not")
	}
}
