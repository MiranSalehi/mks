package mks

import (
	"reflect"
	"slices"

	"github.com/goravel/framework/contracts/http"
)

type fieldMeta struct {
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

type resourceMeta struct {
	Key          string               `json:"key"`
	Fields       map[string]fieldMeta `json:"fields"`
	Relations    []string             `json:"relations"`
	Capabilities capabilities         `json:"capabilities"`
	Deletable    bool                 `json:"deletable"`
	Actions      []actionMeta         `json:"actions"`
	Abilities    resourceAbilities    `json:"abilities"`
	Badge        *navBadge            `json:"badge,omitempty"`
}

type navBadge struct {
	Count int64  `json:"count"`
	Color string `json:"color,omitempty"`
}

// navigationBadge counts the records of Config.NavigationBadge, or nil (none, or an error:
// the menu must not fail because of a badge). A zero count is sent too, so the UI knows to
// refresh it after writes; it shows nothing.
func (p *Panel) navigationBadge(reg registered, hc *HookContext) *navBadge {
	if reg.cfg.NavigationBadge == nil || p.opts.Orm == nil {
		return nil
	}
	q := p.scope(reg.res, reg.cfg, hc, p.opts.Orm.Query().Model(reg.res.newModel()))
	n, err := reg.cfg.NavigationBadge(hc, q).Count()
	if err != nil {
		return nil
	}
	return &navBadge{Count: n, Color: reg.cfg.NavigationBadgeColor}
}

type actionMeta struct {
	Name           string   `json:"name"`
	WithoutRecords bool     `json:"without_records"`
	Fields         []string `json:"fields"`
}

type capabilities struct {
	Searchable []string            `json:"searchable"`
	Sortable   []string            `json:"sortable"`
	Filterable map[string][]string `json:"filterable"`
	// Filters names the custom filters (mks.Filter).
	Filters  []string `json:"filters"`
	Fillable []string `json:"fillable"`
	// Summaries: column -> allowed aggregates ("sum", "avg", "min", "max").
	Summaries map[string][]string `json:"summaries"`
	// Reorderable is the manual order column, or "" when the resource has none.
	Reorderable string `json:"reorderable"`
}

// meta describes the panel for the current user. Labels and layout live in the frontend
// resource definitions; this is what the server allows.
func (p *Panel) meta(ctx http.Context) http.Response {
	user := p.currentUser(ctx)
	if user == nil {
		return message(ctx, 401, p.tr(ctx).unauthenticated)
	}

	// Only resources the user may open; the UI builds its menu from these.
	hc := p.hookContext(ctx)
	resources := make(map[string]resourceMeta, len(p.names()))
	order := make([]string, 0, len(p.names()))
	for _, name := range p.names() {
		reg, _ := p.lookup(name)
		if !allows(reg.res.policy().viewAny, hc) {
			continue
		}
		m := describeResource(reg.res, reg.cfg)
		m.Abilities = resourceAbilitiesFor(reg.res, reg.cfg, hc)
		m.Badge = p.navigationBadge(reg, hc)
		resources[name] = m
		order = append(order, name)
	}

	return ctx.Response().Json(200, http.Json{
		"protocol": ProtocolVersion,
		"panel": http.Json{
			"path":          p.opts.Path,
			"brand":         p.opts.Brand,
			"locale":        p.opts.Locale,
			"direction":     p.opts.Direction,
			"notifications": p.notificationsMeta(),
		},
		"user":      user.PanelUser(),
		"resources": resources,
		"order":     order,
		// Dashboard widgets and custom pages the user may see.
		"widgets": p.visibleWidgets(hc),
		"pages":   p.visiblePages(hc),
		// Sidebar counts of pages (Page.NavigationBadge), by page name.
		"page_badges": p.pageBadges(hc),
		// Whether any resource takes part in the global search.
		"search": p.hasGlobalSearch(hc),
		// The user's tenants and the current one, or null without tenancy.
		"tenancy": p.tenantMeta(hc),
		// Values plugins add (MetaValue), by key.
		"extra": p.metaExtra(hc),
	})
}

func describeResource(r Resource, c Config) resourceMeta {
	relationFields := map[string]bool{}
	relations := make([]string, 0, len(c.Relations))
	for name, field := range c.Relations {
		relations = append(relations, name)
		relationFields[field] = true
	}
	slices.Sort(relations)

	t := reflect.TypeOf(r.newModel()).Elem()
	fields := map[string]fieldMeta{}
	for name, info := range fieldsOf(t) {
		if relationFields[t.FieldByIndex(info.index).Name] {
			continue
		}
		if fm, ok := describeType(info.typ); ok {
			fields[name] = fm
		}
	}

	filterable := c.Filterable
	if filterable == nil {
		filterable = map[string][]string{}
	}
	filterNames := make([]string, 0, len(r.filters()))
	for _, f := range r.filters() {
		filterNames = append(filterNames, f.Name)
	}
	slices.Sort(filterNames)
	actions := make([]actionMeta, 0, len(r.actions()))
	for _, a := range r.actions() {
		fields := make([]string, 0, len(a.rules))
		for f := range a.rules {
			fields = append(fields, f)
		}
		slices.Sort(fields)
		actions = append(actions, actionMeta{Name: a.name, WithoutRecords: a.withoutRecords, Fields: fields})
	}

	return resourceMeta{
		Deletable: c.Deletable,
		Actions:   actions,
		Key:       c.Key,
		Fields:    fields,
		Relations: relations,
		Capabilities: capabilities{
			Searchable:  nonNil(c.Searchable),
			Sortable:    nonNil(c.Sortable),
			Filterable:  filterable,
			Filters:     filterNames,
			Summaries:   summariesOrEmpty(c.Summaries),
			Reorderable: c.Reorderable,
			Fillable:    nonNil(c.Fillable),
		},
	}
}

// describeType maps a Go field type to a protocol field type. Nested structs and slices that
// are not relations are not described.
func describeType(t reflect.Type) (fieldMeta, bool) {
	nullable := false
	if t.Kind() == reflect.Pointer {
		nullable = true
		if isTimeType(t) {
			return fieldMeta{Type: "datetime", Nullable: true}, true
		}
		t = t.Elem()
	}
	if isTimeType(t) {
		return fieldMeta{Type: "datetime", Nullable: nullable}, true
	}
	if t.Implements(jsonTypedType) {
		return fieldMeta{Type: "json", Nullable: nullable}, true
	}
	switch t.Kind() {
	case reflect.String:
		return fieldMeta{Type: "string", Nullable: nullable}, true
	case reflect.Bool:
		return fieldMeta{Type: "boolean", Nullable: nullable}, true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return fieldMeta{Type: "integer", Nullable: nullable}, true
	case reflect.Float32, reflect.Float64:
		return fieldMeta{Type: "number", Nullable: nullable}, true
	}
	return fieldMeta{}, false
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func summariesOrEmpty(m map[string][]string) map[string][]string {
	if m == nil {
		return map[string][]string{}
	}
	return m
}

// MetaValue adds a value to every meta response under extra[key], computed per request
// (e.g. a plugin's navigation settings). Plugins call it from Register.
func (p *Panel) MetaValue(key string, fn func(ctx *HookContext) any) *Panel {
	if p.metaValues == nil {
		p.metaValues = map[string]func(*HookContext) any{}
	}
	p.metaValues[key] = fn
	return p
}

func (p *Panel) metaExtra(hc *HookContext) map[string]any {
	out := map[string]any{}
	for k, fn := range p.metaValues {
		out[k] = fn(hc)
	}
	return out
}
