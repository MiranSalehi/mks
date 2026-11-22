package cms

import (
	"testing"

	"github.com/goravel/framework/contracts/auth"
	"github.com/goravel/framework/contracts/cache"
	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/filesystem"
	"github.com/goravel/framework/contracts/hash"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/validation"

	mks "github.com/MiranSalehi/mks"
)

type stubHash struct{ hash.Hash }

func (stubHash) Make(string) (string, error) { return "x", nil }

type stubStorage struct{ filesystem.Storage }

type stubOrm struct{ orm.Orm }

type stubValidation struct{ validation.Validation }

type stubSites struct{}

func (stubSites) BySlug(string) (*Site, error) { return nil, nil }
func (stubSites) ByID(any) (*Site, error)      { return nil, nil }

// Registering the plugin validates every resource, action and page it adds: a bad name
// or config panics here instead of at app start.
func TestPluginRegistersOnAPanel(t *testing.T) {
	p := mks.New(mks.Options{
		Orm:        stubOrm{},
		Validation: stubValidation{},
		Auth: mks.AuthOptions{
			Auth:    func(http.Context) auth.Auth { return nil },
			Guard:   "admin",
			Hash:    stubHash{},
			NewUser: func() mks.User { return &testUser{} },
		},
		Storage: stubStorage{},
		Tenancy: &mks.TenancyOptions{Tenants: func(*mks.HookContext) ([]mks.Tenant, error) { return nil, nil }},
	})
	pl := &Plugin{Orm: stubOrm{}, Cache: &mapCache{values: map[string]int{}}, Sites: stubSites{}}
	var _ cache.Cache = pl.Cache
	p.Use(pl)

	names := map[string]bool{}
	for _, g := range p.Permissions() {
		if g.Kind == "resource" {
			names[g.Name] = true
		}
	}
	for _, want := range []string{"pages", "page_revisions", "page_trash", "cms_posts", "cms_categories", "cms_tags", "cms_comments", "cms_media", "menus", "menu_locations"} {
		if !names[want] {
			t.Errorf("resource %q not registered", want)
		}
		// Every CMS resource shows on the site: its changes retire cached pages.
		if !pl.watched[want] {
			t.Errorf("changes of %q do not clear the page cache", want)
		}
	}
	custom := map[string]bool{}
	for _, g := range p.Permissions() {
		if g.Kind == "custom" || g.Kind == "pages" {
			for _, perm := range g.Permissions {
				custom[perm] = true
			}
		}
	}
	for _, want := range []string{"page.site_settings", "page.site_permalinks", "page.site_performance", "page.themes", "page.plugins",
		"cms.pages.manage", "cms.settings.manage", "cms.media.manage", "cms.comments.moderate", "cms.themes.manage", "cms.themes.install", "cms.plugins.manage"} {
		if !custom[want] {
			t.Errorf("permission %q missing", want)
		}
	}
}
