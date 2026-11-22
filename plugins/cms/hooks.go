package cms

import (
	"html/template"
	"sort"
	"strconv"
	"sync"
)

// Hooks, MKSine's two families in Go:
//
//   - Events (On / emit): something happened (post.published, content.slug_changed,
//     storefront.viewed, ...). Listeners of "-ing" events (post.creating, post.updating,
//     category.*, tag.*) can stop the save by returning an error.
//   - Filters (AddFilter / filter): a value passes through every filter of a name, in
//     priority order, e.g. "mksine.content.visible" or "mksine.shortcode.year".
//
// Plus template actions (AddAction): HTML a theme or plugin puts at named places of the
// public pages ("page.before_content", "single.after_article", ...).

// Event is what an event listener receives.
type Event struct {
	Name string
	// Site is the site's key (the tenant).
	Site any
	// Record is the post, page, category or tag (a pointer), or nil.
	Record any
	// Data carries the event's details, e.g. "old_slug" and "new_slug".
	Data map[string]any
}

type listener struct {
	fn       func(Event) error
	priority int
	id       string
	order    int
}

type filterFn struct {
	fn       func(value any, args ...any) any
	priority int
	id       string
	order    int
}

type action struct {
	fn       func(ActionContext) template.HTML
	priority int
	id       string
	order    int
}

// ActionContext is what a template action receives: the site and the page being shown.
type ActionContext struct {
	Site *Site
	// View is the kind of page: home, page, single, posts, category, tag, author,
	// categories, tags.
	View string
	// Record is the page or post shown, if any.
	Record any
}

type hookRegistry struct {
	mu        sync.RWMutex
	listeners map[string][]listener
	filters   map[string][]filterFn
	actions   map[string][]action
	n         int
}

func (h *hookRegistry) init() {
	if h.listeners == nil {
		h.listeners, h.filters, h.actions = map[string][]listener{}, map[string][]filterFn{}, map[string][]action{}
	}
}

// On listens to an event. Lower priority runs first (default 10). Its id on the hooks
// page is "{event}#{n}"; use Listen to name it.
func (pl *Plugin) On(event string, fn func(Event) error, priority ...int) {
	pl.Listen(event, "", fn, priority...)
}

// Listen listens to an event under an id, which the hooks page uses to turn it off or
// move it. An id starting with "system." cannot be turned off.
func (pl *Plugin) Listen(event, id string, fn func(Event) error, priority ...int) {
	h := &pl.hooks
	h.mu.Lock()
	defer h.mu.Unlock()
	h.init()
	h.n++
	if id == "" {
		id = event + "#" + strconv.Itoa(h.n)
	}
	h.listeners[event] = append(h.listeners[event], listener{fn: fn, priority: prio(priority), id: id, order: h.n})
	sortBy(h.listeners[event], func(l listener) (int, int) { return l.priority, l.order })
}

// emit runs the listeners of an event; the first error stops it.
func (pl *Plugin) emit(e Event) error {
	pl.hooks.mu.RLock()
	list := append([]listener(nil), pl.hooks.listeners[e.Name]...)
	pl.hooks.mu.RUnlock()
	list = applyHookSettings(pl.hookSettings(), HookEvent, e.Name, list, func(l listener) (string, int, int) { return l.id, l.priority, l.order })
	for _, l := range list {
		if err := l.fn(e); err != nil {
			return err
		}
	}
	return nil
}

// AddFilter adds a filter. id names it for RemoveFilter; priority defaults to 10.
func (pl *Plugin) AddFilter(name string, fn func(value any, args ...any) any, id string, priority ...int) {
	h := &pl.hooks
	h.mu.Lock()
	defer h.mu.Unlock()
	h.init()
	h.n++
	h.filters[name] = append(h.filters[name], filterFn{fn: fn, priority: prio(priority), id: id, order: h.n})
	sortBy(h.filters[name], func(f filterFn) (int, int) { return f.priority, f.order })
}

// RemoveFilter removes the filters of a name with the given id.
func (pl *Plugin) RemoveFilter(name, id string) {
	h := &pl.hooks
	h.mu.Lock()
	defer h.mu.Unlock()
	kept := h.filters[name][:0]
	for _, f := range h.filters[name] {
		if f.id != id {
			kept = append(kept, f)
		}
	}
	h.filters[name] = kept
}

// HasFilter reports whether a filter name has filters.
func (pl *Plugin) HasFilter(name string) bool {
	pl.hooks.mu.RLock()
	defer pl.hooks.mu.RUnlock()
	return len(pl.hooks.filters[name]) > 0
}

// filter passes value through the filters of name.
func (pl *Plugin) filter(name string, value any, args ...any) any {
	pl.hooks.mu.RLock()
	list := append([]filterFn(nil), pl.hooks.filters[name]...)
	pl.hooks.mu.RUnlock()
	list = applyHookSettings(pl.hookSettings(), HookFilter, name, list, func(f filterFn) (string, int, int) { return f.id, f.priority, f.order })
	for _, f := range list {
		value = f.fn(value, args...)
	}
	return value
}

// AddAction puts HTML at a named place of the public pages (MKSine's theme_add_action).
// The HTML is trusted: escape anything that comes from users.
func (pl *Plugin) AddAction(hook string, fn func(ActionContext) template.HTML, id string, priority ...int) {
	h := &pl.hooks
	h.mu.Lock()
	defer h.mu.Unlock()
	h.init()
	h.n++
	h.actions[hook] = append(h.actions[hook], action{fn: fn, priority: prio(priority), id: id, order: h.n})
	sortBy(h.actions[hook], func(a action) (int, int) { return a.priority, a.order })
}

// RemoveAction removes the actions of a hook with the given id.
func (pl *Plugin) RemoveAction(hook, id string) {
	h := &pl.hooks
	h.mu.Lock()
	defer h.mu.Unlock()
	kept := h.actions[hook][:0]
	for _, a := range h.actions[hook] {
		if a.id != id {
			kept = append(kept, a)
		}
	}
	h.actions[hook] = kept
}

// actionsFor runs every template action a view has, by hook name.
func (pl *Plugin) actionsFor(ctx ActionContext) map[string]template.HTML {
	pl.hooks.mu.RLock()
	defer pl.hooks.mu.RUnlock()
	if len(pl.hooks.actions) == 0 {
		return nil
	}
	out := map[string]template.HTML{}
	settings := pl.hookSettings()
	for hook, list := range pl.hooks.actions {
		list = applyHookSettings(settings, HookAction, hook, list, func(a action) (string, int, int) { return a.id, a.priority, a.order })
		var html template.HTML
		for _, a := range list {
			html += a.fn(ctx)
		}
		if html != "" {
			out[hook] = html
		}
	}
	return out
}

func prio(p []int) int {
	if len(p) > 0 {
		return p[0]
	}
	return 10
}

func sortBy[T any](list []T, key func(T) (int, int)) {
	sort.SliceStable(list, func(i, j int) bool {
		a1, a2 := key(list[i])
		b1, b2 := key(list[j])
		if a1 != b1 {
			return a1 < b1
		}
		return a2 < b2
	})
}
