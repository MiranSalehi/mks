package cms

import (
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"
	ormmodel "github.com/goravel/framework/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// MenuLocationsTable holds the menu locations of each site and the menu shown at each
// (MKSine's menu_locations with its assignments): a menu may be at several locations,
// a location shows one menu.
const MenuLocationsTable = "cms_menu_locations"

// MenuLocation is a place on a site that shows a menu. The places come from code
// (Plugin.Locations and the themes); the rows keep each site's label and menu.
type MenuLocation struct {
	ormmodel.Model
	SiteID uint `json:"site_id"`
	// Location is the key of the place, e.g. "header".
	Location string `json:"location"`
	Label    string `json:"label"`
	MenuID   *uint  `json:"menu_id"`
	Menu     *Menu  `gorm:"foreignKey:MenuID" json:"menu,omitempty"`
}

func (MenuLocation) TableName() string { return MenuLocationsTable }

// CreateMenuLocationsTable creates the table, moves the location of every menu into it
// and drops menus.location; call it from a migration.
func CreateMenuLocationsTable(s schema.Schema, q orm.Query) error {
	if !s.HasTable(MenuLocationsTable) {
		if err := s.Create(MenuLocationsTable, func(t schema.Blueprint) {
			t.ID()
			t.UnsignedBigInteger("site_id")
			t.String("location", 50)
			t.String("label")
			t.UnsignedBigInteger("menu_id").Nullable()
			t.Timestamps()
			t.Unique("site_id", "location")
		}); err != nil {
			return err
		}
	}
	if !s.HasColumn(MenusTable, "location") {
		return nil
	}
	var menus []struct {
		ID       uint
		SiteID   uint
		Location *string
	}
	if err := q.Table(MenusTable).Select("id", "site_id", "location").Where("location IS NOT NULL").Scan(&menus); err != nil {
		return err
	}
	now := time.Now()
	for _, m := range menus {
		if m.Location == nil || *m.Location == "" {
			continue
		}
		// Safe to run again after a failure: rows already moved stay as they are.
		n, err := q.Table(MenuLocationsTable).Where("site_id = ? AND location = ?", m.SiteID, *m.Location).Count()
		if err != nil {
			return err
		}
		if n > 0 {
			continue
		}
		if _, err := q.Exec("INSERT INTO "+MenuLocationsTable+" (site_id, location, label, menu_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			m.SiteID, *m.Location, locationLabel(*m.Location), m.ID, now, now); err != nil {
			return err
		}
	}
	// MySQL keeps the unique index for the site_id foreign key: give it its own first.
	if err := s.Table(MenusTable, func(t schema.Blueprint) { t.Index("site_id") }); err != nil && !strings.Contains(err.Error(), "Duplicate key name") {
		return err
	}
	return s.Table(MenusTable, func(t schema.Blueprint) {
		t.DropUnique("site_id", "location")
		t.DropColumn("location")
	})
}

// locationLabel is the default label of a known location key.
func locationLabel(key string) string {
	for _, l := range DefaultLocations {
		if l.Key == key {
			return l.Label
		}
	}
	return key
}

// siteLocations are the locations a site has: the plugin's and its theme's.
func (pl *Plugin) siteLocations(site *Site) []Location {
	list := append([]Location{}, pl.locations()...)
	for _, l := range pl.pluginLocations() {
		if !hasLocation(list, l.Key) {
			list = append(list, l)
		}
	}
	if site != nil {
		for _, l := range pl.themeByID(pl.settings(site)["theme"]).Locations {
			if !hasLocation(list, l.Key) {
				list = append(list, l)
			}
		}
	}
	return list
}

func hasLocation(list []Location, key string) bool {
	for _, l := range list {
		if l.Key == key {
			return true
		}
	}
	return false
}

// syncLocations adds the rows of the site's locations that are missing (MKSine's
// syncToDatabase). Rows of locations no longer defined stay, so a theme switch back
// keeps their menus.
func (pl *Plugin) syncLocations(siteID any) {
	site, err := pl.Sites.ByID(siteID)
	if err != nil || site == nil {
		return
	}
	var rows []MenuLocation
	if err := pl.query().Model(&MenuLocation{}).Where("site_id = ?", site.ID).Find(&rows); err != nil {
		pl.logf("cms: menu locations: %v", err)
		return
	}
	have := map[string]bool{}
	for _, r := range rows {
		have[r.Location] = true
	}
	for _, l := range pl.siteLocations(site) {
		if have[l.Key] {
			continue
		}
		id, _ := toUint(site.ID)
		row := MenuLocation{SiteID: id, Location: l.Key, Label: l.Label}
		if err := pl.query().Create(&row); err != nil {
			pl.logf("cms: add menu location %s: %v", l.Key, err)
		}
	}
}

// menuLocationsResource lists a site's menu locations; editing one renames it or picks
// its menu. Locations come from code, so they are neither created nor deleted here.
func (pl *Plugin) menuLocationsResource() mks.ModelResource[MenuLocation] {
	return mks.Define[MenuLocation](mks.Config{
		Name:        "menu_locations",
		Tenant:      "site_id",
		Searchable:  []string{"location", "label"},
		Sortable:    []string{"location", "label", "updated_at"},
		Fillable:    []string{"label", "menu_id"},
		TitleColumn: "label",
		DefaultSort: "label",
		Relations:   map[string]string{"menu": "Menu"},
		Rules:       map[string]string{"label": "required|max_len:255"},
	}).
		BeforeUpdate(func(ctx *mks.HookContext, loc *MenuLocation, data map[string]any) error {
			if label, ok := data["label"].(string); ok {
				data["label"] = strings.TrimSpace(label)
			}
			raw, sent := data["menu_id"]
			if !sent || raw == nil || raw == "" {
				if sent {
					data["menu_id"] = nil
				}
				return nil
			}
			id, ok := toUint(raw)
			if !ok || id == 0 {
				return mks.ValidationErrors{"menu_id": "منوی معتبری انتخاب کنید."}
			}
			// Before hooks run outside the save's transaction (ctx.Query is nil).
			n, err := pl.query().Model(&Menu{}).Where("id = ? AND site_id = ?", id, loc.SiteID).Count()
			if err != nil {
				return err
			}
			if n == 0 {
				return mks.ValidationErrors{"menu_id": "این منو در این سایت نیست."}
			}
			data["menu_id"] = id
			return nil
		}).
		Policy(mks.Policy[MenuLocation]{
			Create: func(*mks.HookContext) bool { return false },
			Update: func(ctx *mks.HookContext, _ *MenuLocation) bool { return pl.can(ctx, ManageMenus) },
			Delete: func(*mks.HookContext, *MenuLocation) bool { return false },
			Scope: func(ctx *mks.HookContext, q orm.Query) orm.Query {
				if ctx != nil && ctx.TenantID() != nil {
					pl.syncLocations(ctx.TenantID())
				}
				return q
			},
		})
}

// siteMenus are the menus of a site by location key, as its locations assign them.
func (pl *Plugin) siteMenus(site *Site) (map[string]Menu, error) {
	var rows []MenuLocation
	if err := pl.query().Model(&MenuLocation{}).Where("site_id = ? AND menu_id IS NOT NULL", site.ID).Find(&rows); err != nil {
		return nil, err
	}
	ids := map[uint]bool{}
	for _, r := range rows {
		ids[*r.MenuID] = true
	}
	if len(ids) == 0 {
		return map[string]Menu{}, nil
	}
	list := make([]any, 0, len(ids))
	for id := range ids {
		list = append(list, id)
	}
	var menus []Menu
	if err := pl.query().Model(&Menu{}).Where("site_id = ?", site.ID).WhereIn("id", list).Find(&menus); err != nil {
		return nil, err
	}
	byID := map[uint]Menu{}
	for _, m := range menus {
		byID[m.ID] = m
	}
	out := map[string]Menu{}
	for _, r := range rows {
		if m, ok := byID[*r.MenuID]; ok {
			out[r.Location] = m
		}
	}
	return out, nil
}
