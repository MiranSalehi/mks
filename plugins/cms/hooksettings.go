package cms

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/goravel/framework/contracts/database/schema"

	mks "github.com/MiranSalehi/mks"
)

// The hooks page (MKSine's mks_hooks): every event listener, filter and template action
// registered in code is listed; a manager can turn one off or give it another priority.
// The settings apply to every site. Hooks whose id starts with "system." stay on.

// HooksTable keeps the managers' settings of hooks.
const HooksTable = "cms_hooks"

// ManageHooks: turn hooks off and change their order (they apply to every site: give it
// to full managers only).
const ManageHooks Ability = "cms.hooks.manage"

// The kinds of hooks.
const (
	HookEvent  = "event"
	HookFilter = "filter"
	HookAction = "action"
)

// CreateHooksTable creates the hook settings table; call it from a migration.
func CreateHooksTable(s schema.Schema) error {
	return s.Create(HooksTable, func(t schema.Blueprint) {
		t.ID()
		t.String("kind", 16)
		t.String("name", 120)
		t.String("hook_id", 120)
		t.Boolean("enabled").Default(true)
		t.Integer("priority").Nullable()
		t.Unique("kind", "name", "hook_id")
	})
}

// HookSetting is a manager's setting of one hook.
type HookSetting struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	HookID   string `json:"hook_id"`
	Enabled  bool   `json:"enabled"`
	Priority *int   `json:"priority"`
}

func (HookSetting) TableName() string { return HooksTable }

type hookKey struct{ kind, name, id string }

type hookSettingsCache struct {
	mu     sync.RWMutex
	loaded bool
	gen    int
	byKey  map[hookKey]HookSetting
}

// hookSettings are the saved settings (read once, again after a save). Without the
// table (an app that has not migrated it) there are none.
func (pl *Plugin) hookSettings() map[hookKey]HookSetting {
	c := &pl.hookCache
	c.mu.RLock()
	if c.loaded {
		m := c.byKey
		c.mu.RUnlock()
		return m
	}
	gen := c.gen
	c.mu.RUnlock()
	m := map[hookKey]HookSetting{}
	if pl.Orm != nil {
		var rows []HookSetting
		if err := pl.query().Model(&HookSetting{}).Find(&rows); err == nil {
			for _, r := range rows {
				m[hookKey{r.Kind, r.Name, r.HookID}] = r
			}
		}
	}
	c.mu.Lock()
	if c.gen == gen {
		c.byKey, c.loaded = m, true
	}
	c.mu.Unlock()
	return m
}

func (pl *Plugin) forgetHookSettings() {
	c := &pl.hookCache
	c.mu.Lock()
	c.loaded, c.byKey, c.gen = false, nil, c.gen+1
	c.mu.Unlock()
}

// applyHookSettings drops the turned-off hooks of a list and orders it by the
// effective priorities.
func applyHookSettings[T any](settings map[hookKey]HookSetting, kind, name string, list []T, info func(T) (id string, priority, order int)) []T {
	if len(settings) == 0 || len(list) == 0 {
		return list
	}
	type entry struct {
		item            T
		priority, order int
	}
	out := make([]entry, 0, len(list))
	changed := false
	for _, it := range list {
		id, p, o := info(it)
		if s, ok := settings[hookKey{kind, name, id}]; ok {
			if !s.Enabled && !strings.HasPrefix(id, "system.") {
				changed = true
				continue
			}
			if s.Priority != nil && *s.Priority != p {
				p, changed = *s.Priority, true
			}
		}
		out = append(out, entry{it, p, o})
	}
	if !changed {
		return list
	}
	sortBy(out, func(e entry) (int, int) { return e.priority, e.order })
	result := make([]T, len(out))
	for i, e := range out {
		result[i] = e.item
	}
	return result
}

// HookInfo is one registered hook as the hooks page lists it.
type HookInfo struct {
	Kind     string `json:"kind"`
	Name     string `json:"name"`
	ID       string `json:"id"`
	Priority int    `json:"priority"` // as registered
	Enabled  bool   `json:"enabled"`
	Override *int   `json:"override"` // the manager's priority, if any
	System   bool   `json:"system"`
}

// RegisteredHooks lists every hook with its settings.
func (pl *Plugin) RegisteredHooks() []HookInfo {
	settings := pl.hookSettings()
	var out []HookInfo
	add := func(kind, name, id string, priority int) {
		info := HookInfo{Kind: kind, Name: name, ID: id, Priority: priority, Enabled: true, System: strings.HasPrefix(id, "system.")}
		if s, ok := settings[hookKey{kind, name, id}]; ok {
			info.Enabled = s.Enabled || info.System
			info.Override = s.Priority
		}
		out = append(out, info)
	}
	pl.hooks.mu.RLock()
	for name, list := range pl.hooks.listeners {
		for _, l := range list {
			add(HookEvent, name, l.id, l.priority)
		}
	}
	for name, list := range pl.hooks.filters {
		for _, f := range list {
			add(HookFilter, name, f.id, f.priority)
		}
	}
	for name, list := range pl.hooks.actions {
		for _, a := range list {
			add(HookAction, name, a.id, a.priority)
		}
	}
	pl.hooks.mu.RUnlock()
	slices.SortFunc(out, func(a, b HookInfo) int {
		return strings.Compare(a.Kind+"\x00"+a.Name+"\x00"+a.ID, b.Kind+"\x00"+b.Name+"\x00"+b.ID)
	})
	return out
}

// hooksPage is the "hooks" page: Data lists the hooks; Handle saves {hooks: [{kind, name,
// id, enabled, priority}]}.
func (pl *Plugin) hooksPage() mks.Page {
	return mks.Page{
		Name:      "hooks",
		Authorize: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManageHooks) },
		Data: func(*mks.WidgetContext) (any, error) {
			return map[string]any{"hooks": pl.RegisteredHooks()}, nil
		},
		Handle: func(ctx *mks.PageContext) (*mks.ActionResult, error) {
			raw, _ := ctx.Data["hooks"].([]any)
			if len(raw) > 1000 {
				return nil, mks.ValidationErrors{"hooks": "بیش از اندازه."}
			}
			known := map[hookKey]bool{}
			for _, h := range pl.RegisteredHooks() {
				known[hookKey{h.Kind, h.Name, h.ID}] = true
			}
			for i, item := range raw {
				m, _ := item.(map[string]any)
				kind, _ := m["kind"].(string)
				name, _ := m["name"].(string)
				id, _ := m["id"].(string)
				key := hookKey{kind, name, id}
				if !known[key] {
					return nil, mks.ValidationErrors{"hooks": fmt.Sprintf("ردیف %d: این hook ثبت نشده است.", i+1)}
				}
				enabled, ok := m["enabled"].(bool)
				if !ok || strings.HasPrefix(id, "system.") {
					enabled = true
				}
				var priority *int
				if v, ok := m["priority"].(float64); ok {
					if v < -1000 || v > 1000 {
						return nil, mks.ValidationErrors{"hooks": fmt.Sprintf("ردیف %d: اولویت بین ۱۰۰۰- و ۱۰۰۰ است.", i+1)}
					}
					p := int(v)
					priority = &p
				}
				if _, err := ctx.Query.Exec("DELETE FROM "+HooksTable+" WHERE kind = ? AND name = ? AND hook_id = ?", kind, name, id); err != nil {
					return nil, err
				}
				if !enabled || priority != nil {
					if err := ctx.Query.Create(&HookSetting{Kind: kind, Name: name, HookID: id, Enabled: enabled, Priority: priority}); err != nil {
						return nil, err
					}
				}
			}
			ctx.AfterCommit(pl.forgetHookSettings)
			ctx.AfterCommit(func() {
				// Template actions change pages: retire every site's cached pages.
				for _, id := range pl.siteIDsForCache() {
					pl.bump(id)
				}
			})
			return mks.Success("تنظیم hookها ذخیره شد."), nil
		},
	}
}

// siteIDsForCache are the sites whose cached pages a global change retires.
func (pl *Plugin) siteIDsForCache() []any {
	var ids []uint
	if pl.Orm == nil {
		return nil
	}
	if err := pl.query().Model(&Page{}).Distinct("site_id").Pluck("site_id", &ids); err != nil {
		return nil
	}
	out := make([]any, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}
