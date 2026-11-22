package mks

import (
	"fmt"
	"reflect"
	"regexp"
	"slices"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
)

// Plugin is a bundle of panel features installed with p.Use: resources, widgets, pages,
// event listeners and API routes. Its frontend half is a PanelPlugin (definePlugin) with
// the same name.
//
//	type ActivityLog struct{}
//	func (ActivityLog) Name() string            { return "activity-log" }
//	func (ActivityLog) Register(p *mks.Panel) { p.OnSaved(...); p.Register(...) }
type Plugin interface {
	Name() string
	// Register adds the plugin's features to the panel. Called once, by Use.
	Register(p *Panel)
}

// RoutesPlugin is a plugin with its own API endpoints. They live under
// {panel path}/api/plugins/{name}/ and run with the panel's session, CSRF check and
// login, like every panel endpoint. Use p.Context(ctx) for the user.
type RoutesPlugin interface {
	Plugin
	Routes(r route.Router)
}

var pluginName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// Use installs plugins. It panics on an invalid or repeated name.
func (p *Panel) Use(plugins ...Plugin) *Panel {
	for _, pl := range plugins {
		name := pl.Name()
		if !pluginName.MatchString(name) {
			panic(fmt.Sprintf("mks: invalid plugin name %q (lowercase letters, digits and dashes)", name))
		}
		if slices.ContainsFunc(p.plugins, func(x Plugin) bool { return x.Name() == name }) {
			panic(fmt.Sprintf("mks: plugin %q installed twice", name))
		}
		p.plugins = append(p.plugins, pl)
		pl.Register(p)
	}
	return p
}

// Context is the hook context of a request (its user and the panel), for plugin and
// app handlers.
func (p *Panel) Context(ctx http.Context) *HookContext { return p.hookContext(ctx) }

// Path is where the panel is mounted, e.g. "/admin" (for links into it).
func (p *Panel) Path() string { return p.opts.Path }

// Forbidden is the panel's 403 response.
func (p *Panel) Forbidden(ctx http.Context) http.Response { return p.forbidden(ctx) }

func (p *Panel) pluginRoutes(g route.Router) {
	for _, pl := range p.plugins {
		if rp, ok := pl.(RoutesPlugin); ok {
			g.Prefix("/plugins/" + pl.Name()).Group(rp.Routes)
		}
	}
}

// --- events -------------------------------------------------------------------------------

// SavedListener runs after a record is created or updated through the panel (forms,
// relation managers), inside the save's transaction: an error rolls the save back.
type SavedListener func(ctx *HookContext, resource string, record any, created bool) error

// DeletedListener runs after a record is deleted through the panel (single or bulk),
// inside the delete's transaction.
type DeletedListener func(ctx *HookContext, resource string, record any) error

// OnSaved listens to saves of every resource, e.g. for an activity log. Actions that write
// records themselves do not fire it.
func (p *Panel) OnSaved(fn SavedListener) *Panel {
	p.onSaved = append(p.onSaved, fn)
	return p
}

// OnDeleted listens to deletes of every resource.
func (p *Panel) OnDeleted(fn DeletedListener) *Panel {
	p.onDeleted = append(p.onDeleted, fn)
	return p
}

func (p *Panel) fireSaved(hc *HookContext, resource string, record any, created bool) error {
	for _, fn := range p.onSaved {
		if err := fn(hc, resource, record, created); err != nil {
			return err
		}
	}
	return nil
}

func (p *Panel) fireDeleted(hc *HookContext, resource string, record any) error {
	for _, fn := range p.onDeleted {
		if err := fn(hc, resource, record); err != nil {
			return err
		}
	}
	return nil
}

// Key is a record's primary key value as the resource sees it, e.g. for a log entry.
func (p *Panel) Key(resource string, record any) (any, error) {
	reg, ok := p.lookup(resource)
	if !ok {
		return nil, fmt.Errorf("mks: unknown resource %q", resource)
	}
	return keyValue(record, reg.cfg.Key)
}

// Title is a record's title (its resource's TitleColumn), or "" without one.
func (p *Panel) Title(resource string, record any) string {
	reg, ok := p.lookup(resource)
	if !ok || reg.cfg.TitleColumn == "" {
		return ""
	}
	encoded, _ := encode(reflect.ValueOf(record), nil).(map[string]any)
	if v, ok := encoded[reg.cfg.TitleColumn]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}
