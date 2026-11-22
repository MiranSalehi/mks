// Package navigation lets a panel manager rearrange the panel's sidebar (MKSine's Admin
// menu editor): hide, rename, move between groups and order items, and rename, order or
// hide groups. The settings travel to the UI in meta.extra.navigation; the sidebar
// applies them. Hiding only tidies the menu: permissions still decide what a user may
// open. Its frontend half is @mks/mks/navigation.
//
//	p.Use(&navigation.Plugin{Orm: facades.Orm()})
//
// The table comes from a migration: navigation.CreateTable(facades.Schema()).
package navigation

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"

	mks "github.com/MiranSalehi/mks"
)

// Table keeps the settings in one row.
const Table = "panel_navigation"

// Manage is the permission to change the menu (give it to full managers only).
const Manage = "navigation.manage"

// The editor page itself: it cannot be hidden, so the menu can always be repaired.
const pageName = "admin_menu"

// CreateTable creates the settings table; call it from a migration.
func CreateTable(s schema.Schema) error {
	return s.Create(Table, func(t schema.Blueprint) {
		t.ID()
		t.Json("config")
		t.Timestamp("updated_at").Nullable()
	})
}

type row struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	Config    string     `json:"config"`
	UpdatedAt *time.Time `json:"updated_at"`
}

func (row) TableName() string { return Table }

// Where an item sits after the manager edited it (MKSine's AdminMenuPlacement).
const (
	// PlacementDefault keeps the group the code gives the item.
	PlacementDefault = ""
	// PlacementHidden takes the item out of the sidebar; its URL and access rules stay.
	PlacementHidden = "hidden"
	// PlacementGroup moves the item into Item.Group.
	PlacementGroup = "group"
)

// Item is the override of one sidebar item ("r:posts", "p:authors", "dashboard").
type Item struct {
	Placement string `json:"placement,omitempty"`
	Label     string `json:"label,omitempty"`
	Group     string `json:"group,omitempty"`
	Sort      int    `json:"sort,omitempty"`
	// Hidden is the older shape of Placement; parse migrates it.
	Hidden bool `json:"hidden,omitempty"`
}

// Group is the override of a group, keyed by its original heading.
type Group struct {
	Hidden bool   `json:"hidden,omitempty"`
	Label  string `json:"label,omitempty"`
	Sort   int    `json:"sort,omitempty"`
}

// Config is the whole menu configuration.
type Config struct {
	Items  map[string]Item  `json:"items"`
	Groups map[string]Group `json:"groups"`
}

// Plugin is the menu editor. Orm is required.
type Plugin struct {
	Orm orm.Orm

	mu     sync.RWMutex
	gen    int // bumped on every save; a load that raced a save does not cache
	cached *Config
}

func (*Plugin) Name() string { return "navigation" }

func (pl *Plugin) Register(p *mks.Panel) {
	if pl.Orm == nil {
		panic("navigation: Orm is required")
	}
	p.Permission(Manage)
	p.MetaValue("navigation", func(*mks.HookContext) any {
		c, err := pl.load()
		if err != nil {
			return nil
		}
		return c
	})
	p.Pages(mks.Page{
		Name:      pageName,
		Authorize: func(ctx *mks.HookContext) bool { return p.Allowed(ctx, Manage) },
		Data: func(*mks.WidgetContext) (any, error) {
			c, err := pl.load()
			return map[string]any{"config": c}, err
		},
		Handle: func(ctx *mks.PageContext) (*mks.ActionResult, error) {
			c, err := parse(ctx.Data["config"])
			if err != nil {
				return nil, mks.ValidationErrors{"config": err.Error()}
			}
			// Outside the page's transaction, so the cache never holds a menu that was not
			// committed.
			if err := pl.save(pl.Orm.Query(), c); err != nil {
				return nil, err
			}
			message := "Admin menu saved"
			if action, _ := ctx.Data["action"].(string); action == "reset" {
				message = "The admin menu was reset to its default"
			}
			return mks.Success(message), nil
		},
	})
}

func (pl *Plugin) load() (*Config, error) {
	pl.mu.RLock()
	c, gen := pl.cached, pl.gen
	pl.mu.RUnlock()
	if c != nil {
		return c, nil
	}
	var rows []row
	if err := pl.Orm.Query().Model(&row{}).Limit(1).Find(&rows); err != nil {
		return nil, err
	}
	c = &Config{Items: map[string]Item{}, Groups: map[string]Group{}}
	if len(rows) > 0 && rows[0].Config != "" {
		if err := json.Unmarshal([]byte(rows[0].Config), c); err != nil {
			return nil, err
		}
	}
	pl.mu.Lock()
	if pl.gen == gen {
		pl.cached = c
	}
	pl.mu.Unlock()
	return c, nil
}

func (pl *Plugin) save(q orm.Query, c *Config) error {
	raw, err := json.Marshal(c)
	if err != nil {
		return err
	}
	now := time.Now()
	var rows []row
	if err := q.Model(&row{}).Limit(1).Find(&rows); err != nil {
		return err
	}
	if len(rows) == 0 {
		err = q.Create(&row{Config: string(raw), UpdatedAt: &now})
	} else {
		_, err = q.Model(&row{}).Where("id = ?", rows[0].ID).Update(map[string]any{"config": string(raw), "updated_at": now})
	}
	if err != nil {
		return err
	}
	pl.mu.Lock()
	pl.cached, pl.gen = c, pl.gen+1
	pl.mu.Unlock()
	return nil
}

// parse checks a submitted configuration: at most 8 KB (as in MKSine), labels up to 120
// characters, and the editor itself never hidden.
func parse(raw any) (*Config, error) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, errors.New("Invalid settings structure.")
	}
	if len(encoded) > 8192 {
		return nil, errors.New("Too many menu changes to save. Reset a few items to the default and try again.")
	}
	c := &Config{}
	if err := json.Unmarshal(encoded, c); err != nil {
		return nil, errors.New("Invalid settings structure.")
	}
	if c.Items == nil {
		c.Items = map[string]Item{}
	}
	if c.Groups == nil {
		c.Groups = map[string]Group{}
	}
	for key, it := range c.Items {
		it.Label, it.Group = strings.TrimSpace(it.Label), strings.TrimSpace(it.Group)
		if utf8.RuneCountInString(it.Label) > 120 || utf8.RuneCountInString(it.Group) > 120 || len(key) > 120 {
			return nil, fmt.Errorf("The name \"%s\" is longer than 120 characters.", key)
		}
		if it.Hidden {
			// The shape written before the editor had "placement".
			it.Placement, it.Hidden = PlacementHidden, false
		}
		switch it.Placement {
		case PlacementHidden:
		case PlacementGroup:
			if it.Group == "" {
				it.Placement = PlacementDefault
			}
		default:
			// Empty or unknown keeps the item in the group the code gives it.
			it.Placement = PlacementDefault
		}
		if key == "p:"+pageName {
			// The editor cannot hide itself, or there would be no way back to the menu.
			it.Placement = PlacementDefault
		}
		if it.Placement == PlacementDefault && it.Group == "" && it.Sort == 0 && it.Label == "" {
			delete(c.Items, key)
			continue
		}
		c.Items[key] = it
	}
	for key, g := range c.Groups {
		g.Label = strings.TrimSpace(g.Label)
		if utf8.RuneCountInString(g.Label) > 120 || len(key) > 120 {
			return nil, fmt.Errorf("The group name \"%s\" is longer than 120 characters.", key)
		}
		if g == (Group{}) {
			delete(c.Groups, key)
			continue
		}
		c.Groups[key] = g
	}
	return c, nil
}
