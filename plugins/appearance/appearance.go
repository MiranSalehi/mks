// Package appearance lets users choose the panel's look (MKSine's admin appearance): a
// preset, the primary color, the color mode, the sidebar style, corner radius, font size,
// density, the form field style and where labels go. A manager sets the company look and saves named looks (at most 24); every
// user may set their own, which wins over the company's. The UI receives them in
// meta.extra.appearance. Its frontend half is @mksine/mks/appearance.
//
//	p.Use(&appearance.Plugin{Orm: facades.Orm()})
//
// The table comes from a migration: appearance.CreateTable(facades.Schema()).
package appearance

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"

	mks "github.com/MiranSalehi/mks"
)

// Table keeps the looks: the company's ("company"), each user's ("user:{id}") and the
// named ones ("saved:{name}").
const Table = "panel_appearance"

// Manage is the permission to set the company look and the named looks.
const Manage = "appearance.manage"

// MaxSaved caps the named looks (as in MKSine).
const MaxSaved = 24

// CreateTable creates the table; call it from a migration.
func CreateTable(s schema.Schema) error {
	return s.Create(Table, func(t schema.Blueprint) {
		t.ID()
		t.String("scope", 120)
		t.Json("settings")
		t.Timestamp("updated_at").Nullable()
		t.Unique("scope")
	})
}

type row struct {
	ID        uint       `gorm:"primaryKey"`
	Scope     string     `gorm:"column:scope"`
	Settings  string     `gorm:"column:settings"`
	UpdatedAt *time.Time `gorm:"column:updated_at"`
}

func (row) TableName() string { return Table }

// Look is one look; empty fields leave the preset's choice.
type Look struct {
	Preset  string `json:"preset,omitempty"`
	Primary string `json:"primary,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Rail    string `json:"rail,omitempty"`
	Radius  string `json:"radius,omitempty"`
	Font    string `json:"font,omitempty"`
	Density string `json:"density,omitempty"`
	Field   string `json:"field,omitempty"`
	Label   string `json:"label,omitempty"`
}

// The choices of each part.
var choices = map[string][]string{
	"preset":  {"modern", "classic", "minimal", "soft", "bold"},
	"primary": {"iris", "indigo", "blue", "cyan", "emerald", "violet", "orange", "fuchsia", "slate"},
	"mode":    {"light", "dark", "system"},
	"rail":    {"light", "dark", "brand"},
	"radius":  {"sharp", "medium", "round"},
	"font":    {"small", "normal", "large"},
	"density": {"compact", "comfortable"},
	"field":   {"outlined", "filled", "underline", "floating"},
	"label":   {"top", "side"},
}

// parseLook checks a submitted look: only known parts and values.
func parseLook(raw any) (*Look, error) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, mks.ValidationErrors{"look": "Invalid appearance."}
	}
	var l Look
	fields := map[string]*string{"preset": &l.Preset, "primary": &l.Primary, "mode": &l.Mode, "rail": &l.Rail,
		"radius": &l.Radius, "font": &l.Font, "density": &l.Density, "field": &l.Field, "label": &l.Label}
	for k, v := range m {
		dst, known := fields[k]
		if !known {
			continue
		}
		s, _ := v.(string)
		if s == "" {
			continue
		}
		if !slices.Contains(choices[k], s) {
			return nil, mks.ValidationErrors{"look": fmt.Sprintf("Invalid value \"%s\" for %s.", s, k)}
		}
		*dst = s
	}
	return &l, nil
}

// Named is a saved look.
type Named struct {
	Name string `json:"name"`
	Look Look   `json:"look"`
}

// Plugin is the appearance editor. Orm is required.
type Plugin struct {
	Orm orm.Orm

	panel  *mks.Panel
	mu     sync.RWMutex
	gen    int
	cached map[string]Look // by scope
}

func (*Plugin) Name() string { return "appearance" }

func (pl *Plugin) Register(p *mks.Panel) {
	if pl.Orm == nil {
		panic("appearance: Orm is required")
	}
	pl.panel = p
	p.Permission(Manage)
	p.MetaValue("appearance", func(ctx *mks.HookContext) any {
		looks, err := pl.load()
		if err != nil {
			return nil
		}
		out := map[string]any{"company": nil, "user": nil}
		if l, ok := looks["company"]; ok {
			out["company"] = l
		}
		if l, ok := looks["user:"+fmt.Sprint(ctx.UserID())]; ok && ctx.User != nil {
			out["user"] = l
		}
		return out
	})
	data := func(ctx *mks.WidgetContext) (any, error) {
		looks, err := pl.load()
		if err != nil {
			return nil, err
		}
		saved := []Named{}
		for scope, l := range looks {
			if name, ok := strings.CutPrefix(scope, "saved:"); ok {
				saved = append(saved, Named{Name: name, Look: l})
			}
		}
		slices.SortFunc(saved, func(a, b Named) int { return strings.Compare(a.Name, b.Name) })
		var company, user *Look
		if l, ok := looks["company"]; ok {
			company = &l
		}
		if l, ok := looks["user:"+fmt.Sprint(ctx.UserID())]; ok {
			user = &l
		}
		return map[string]any{"company": company, "user": user, "saved": saved,
			"can_manage": p.Allowed(ctx.HookContext, Manage)}, nil
	}
	handle := func(ctx *mks.PageContext) (*mks.ActionResult, error) {
		res, err := pl.handle(ctx)
		if err != nil {
			return nil, err
		}
		ctx.AfterCommit(pl.forget)
		return res, nil
	}
	// "appearance": every user's own look (MKSine's "My appearance" in the user menu);
	// "company_appearance": the company look and named looks (Settings › Appearance).
	p.Pages(
		mks.Page{Name: "appearance", Data: data, Handle: handle},
		mks.Page{Name: "company_appearance", Authorize: func(ctx *mks.HookContext) bool { return p.Allowed(ctx, Manage) }, Data: data, Handle: handle},
	)
}

func (pl *Plugin) handle(ctx *mks.PageContext) (*mks.ActionResult, error) {
	action, _ := ctx.Data["action"].(string)
	manage := pl.panel.Allowed(ctx.HookContext, Manage)
	user := "user:" + fmt.Sprint(ctx.UserID())
	switch action {
	case "save_user":
		l, err := parseLook(ctx.Data["look"])
		if err != nil {
			return nil, err
		}
		return mks.Success("Your appearance saved."), pl.put(ctx.Query, user, *l)
	case "reset_user":
		_, err := ctx.Query.Exec("DELETE FROM "+Table+" WHERE scope = ?", user)
		return mks.Success("Your appearance was reset to the organization's."), err
	case "save_company":
		if !manage {
			return nil, mks.Reject("You are not allowed to change the organization's appearance.")
		}
		l, err := parseLook(ctx.Data["look"])
		if err != nil {
			return nil, err
		}
		return mks.Success("Organization appearance saved."), pl.put(ctx.Query, "company", *l)
	case "save_named":
		if !manage {
			return nil, mks.Reject("You are not allowed to save named appearances.")
		}
		name, _ := ctx.Data["name"].(string)
		name = strings.TrimSpace(name)
		if name == "" || utf8.RuneCountInString(name) > 40 {
			return nil, mks.ValidationErrors{"name": "The name is required (at most 40 characters)."}
		}
		l, err := parseLook(ctx.Data["look"])
		if err != nil {
			return nil, err
		}
		n, err := ctx.Query.Model(&row{}).Where("scope LIKE ? AND scope <> ?", "saved:%", "saved:"+name).Count()
		if err != nil {
			return nil, err
		}
		if n >= MaxSaved {
			return nil, mks.ValidationErrors{"name": fmt.Sprintf("At most %d named appearances.", MaxSaved)}
		}
		return mks.Success(fmt.Sprintf("Appearance \"%s\" saved.", name)), pl.put(ctx.Query, "saved:"+name, *l)
	case "delete_named":
		if !manage {
			return nil, mks.Reject("You are not allowed to delete named appearances.")
		}
		name, _ := ctx.Data["name"].(string)
		_, err := ctx.Query.Exec("DELETE FROM "+Table+" WHERE scope = ?", "saved:"+name)
		return mks.Success("Appearance deleted."), err
	}
	return nil, mks.Reject("Invalid action.")
}

func (pl *Plugin) put(q orm.Query, scope string, l Look) error {
	raw, err := json.Marshal(l)
	if err != nil {
		return err
	}
	now := time.Now()
	if _, err := q.Exec("DELETE FROM "+Table+" WHERE scope = ?", scope); err != nil {
		return err
	}
	return q.Create(&row{Scope: scope, Settings: string(raw), UpdatedAt: &now})
}

func (pl *Plugin) forget() {
	pl.mu.Lock()
	pl.cached, pl.gen = nil, pl.gen+1
	pl.mu.Unlock()
}

func (pl *Plugin) load() (map[string]Look, error) {
	pl.mu.RLock()
	m, gen := pl.cached, pl.gen
	pl.mu.RUnlock()
	if m != nil {
		return m, nil
	}
	var rows []row
	if err := pl.Orm.Query().Model(&row{}).Find(&rows); err != nil {
		return nil, err
	}
	m = map[string]Look{}
	for _, r := range rows {
		var l Look
		if json.Unmarshal([]byte(r.Settings), &l) == nil {
			m[r.Scope] = l
		}
	}
	pl.mu.Lock()
	if pl.gen == gen {
		pl.cached = m
	}
	pl.mu.Unlock()
	return m, nil
}
