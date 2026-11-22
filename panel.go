// Package mks serves the admin panel protocol (see docs/PROTOCOL.md) on top of Goravel.
package mks

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/filesystem"
	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
	"github.com/goravel/framework/contracts/validation"
	frameworkerrors "github.com/goravel/framework/errors"
	httpmiddleware "github.com/goravel/framework/http/middleware"
	sessionmiddleware "github.com/goravel/framework/session/middleware"
	"github.com/goravel/framework/support/str"
	frameworkvalidation "github.com/goravel/framework/validation"
)

// ProtocolVersion is reported by /meta. Bumped on breaking protocol changes.
const ProtocolVersion = 0

type Options struct {
	// Path is where the panel is mounted. Defaults to "/admin".
	Path string
	// Brand is the panel name shown in the UI.
	Brand string
	// Locale and Direction are sent to the UI. Default: "fa" and "rtl".
	Locale    string
	Direction string

	Orm        orm.Orm
	Validation validation.Validation
	Auth       AuthOptions
	// Storage holds uploaded files (facades.Storage()). Needed when a resource has Uploads.
	Storage filesystem.Storage
	// TempDisk keeps uploads until the form is saved. Default: "local" (not public).
	TempDisk string
	// UI is the built frontend (index.html + assets), usually from go:embed. Optional:
	// in development the Vite dev server serves the UI instead.
	UI fs.FS

	// Tenancy turns on multi-tenancy (see TenancyOptions and Config.Tenant).
	Tenancy *TenancyOptions

	// Notifications turns on database notifications (the bell). The table must exist:
	// see CreateNotificationsTable.
	Notifications bool
	// NotificationPoll is how often the UI looks for new notifications. Default: 15s.
	NotificationPoll time.Duration

	// Debug adds internal error details to 500 responses. Never enable in production.
	Debug bool

	// NoFallback disables the catch-all that serves the panel's index.html for
	// client-side paths (e.g. /admin/posts/create) when no other route matched. Set it
	// when the application has its own fallback: that fallback must call Panel.ServeUI
	// for paths under the panel. Default false: the panel installs its fallback.
	NoFallback bool
}

type Panel struct {
	opts      Options
	resources map[string]registered
	order     []string
	// dynamic: resources added while running (SetResources); dynMu orders writers.
	dynamic   atomic.Pointer[dynamicSet]
	dynMu     sync.Mutex
	dummyHash string
	t         *texts
	schemas   sync.Map // ORM schemas, for relation managers
	widgets   []Widget
	// gate: see SetGate; customPermissions: see Permission.
	gate              Gate
	customPermissions []string
	metaValues        map[string]func(*HookContext) any
	pages             []Page
	plugins           []Plugin
	onSaved           []SavedListener
	onDeleted         []DeletedListener
}

type registered struct {
	res Resource
	cfg Config
}

// normalizePath turns the configured path into the panel's mount path, e.g. "/admin".
// The service provider uses it before the panel exists, to answer on the right path when
// the panel is not configured.
func normalizePath(path string) string {
	path = "/" + strings.Trim(path, "/")
	if path == "/" {
		return "/admin"
	}
	return path
}

// New creates a panel. It panics on invalid options, since that is a programming error.
func New(opts Options) *Panel {
	if opts.NotificationPoll <= 0 {
		opts.NotificationPoll = 15 * time.Second
	}
	opts.Path = normalizePath(opts.Path)
	if opts.Brand == "" {
		opts.Brand = "Admin"
	}
	if opts.Locale == "" {
		opts.Locale = "fa"
	}
	if opts.Direction == "" {
		opts.Direction = "rtl"
	}
	if opts.TempDisk == "" {
		opts.TempDisk = "local"
	}
	if opts.Orm == nil || opts.Validation == nil {
		panic("mks: Options.Orm and Options.Validation are required")
	}
	if err := opts.Auth.validate(); err != nil {
		panic(err)
	}
	dummy, err := opts.Auth.Hash.Make(str.Random(32))
	if err != nil {
		panic(fmt.Sprintf("mks: cannot create hash: %v", err))
	}
	return &Panel{opts: opts, resources: map[string]registered{}, dummyHash: dummy, t: textsFor(opts.Locale)}
}

// Register adds resources. It panics on invalid configuration, since that is a programming error.
func (p *Panel) Register(resources ...Resource) *Panel {
	for _, r := range resources {
		reg, err := p.prepare(r)
		if err != nil {
			panic(err)
		}
		if p.hasResource(reg.cfg.Name) {
			panic(fmt.Sprintf("mks: resource %q registered twice", reg.cfg.Name))
		}
		p.resources[reg.cfg.Name] = reg
		p.order = append(p.order, reg.cfg.Name)
	}
	return p
}

// prepare checks a resource's configuration and wraps it for the gate.
func (p *Panel) prepare(r Resource) (registered, error) {
	c := r.Config()
	for _, check := range []func() error{
		c.validate,
		func() error { return checkRelations(r.newModel(), c) },
		func() error { return checkIncludeTargets(r.newModel(), c) },
		func() error { return validateActions(r, c) },
		func() error { return validateFilters(r, c) },
		func() error { return validateTable(r, c) },
		func() error { return validateTenant(r, c) },
	} {
		if err := check(); err != nil {
			return registered{}, err
		}
	}
	if len(c.Uploads) > 0 && p.opts.Storage == nil {
		return registered{}, fmt.Errorf("mks: resource %q has Uploads but Options.Storage is not set", c.Name)
	}
	return registered{res: gatedResource{Resource: r, p: p, name: c.Name}, cfg: c}, nil
}

// dynamicSet holds resources added while the app runs (SetResources), by group.
type dynamicSet struct {
	byName map[string]registered
	order  []string
	groups map[string][]string
}

// SetResources replaces the resources of a group while the app runs, e.g. the content
// types a CMS plugin installed from a ZIP file brings: the group's previous resources
// go, the new ones come (after the resources from Register, in group order). Names may
// not clash with any other resource. Safe to call from any goroutine; requests that are
// running keep the set they started with.
func (p *Panel) SetResources(group string, resources ...Resource) error {
	p.dynMu.Lock()
	defer p.dynMu.Unlock()
	cur := p.dynamic.Load()
	next := &dynamicSet{byName: map[string]registered{}, groups: map[string][]string{}}
	if cur != nil {
		for g, names := range cur.groups {
			if g == group {
				continue
			}
			next.groups[g] = names
			for _, n := range names {
				next.byName[n] = cur.byName[n]
			}
		}
	}
	var names []string
	for _, r := range resources {
		reg, err := p.prepare(r)
		if err != nil {
			return err
		}
		name := reg.cfg.Name
		if _, static := p.resources[name]; static {
			return fmt.Errorf("mks: resource %q is already registered", name)
		}
		if _, taken := next.byName[name]; taken {
			return fmt.Errorf("mks: resource %q is already registered", name)
		}
		next.byName[name] = reg
		names = append(names, name)
	}
	if len(names) > 0 {
		next.groups[group] = names
	}
	groups := make([]string, 0, len(next.groups))
	for g := range next.groups {
		groups = append(groups, g)
	}
	slices.Sort(groups)
	for _, g := range groups {
		next.order = append(next.order, next.groups[g]...)
	}
	p.dynamic.Store(next)
	return nil
}

// lookup finds a resource by name: registered at start or added since.
func (p *Panel) lookup(name string) (registered, bool) {
	if reg, ok := p.resources[name]; ok {
		return reg, true
	}
	if d := p.dynamic.Load(); d != nil {
		reg, ok := d.byName[name]
		return reg, ok
	}
	return registered{}, false
}

func (p *Panel) mustLookup(name string) registered {
	reg, _ := p.lookup(name)
	return reg
}

func (p *Panel) hasResource(name string) bool {
	_, ok := p.lookup(name)
	return ok
}

// names are every resource name in order: those from Register, then those added since.
func (p *Panel) names() []string {
	d := p.dynamic.Load()
	if d == nil || len(d.order) == 0 {
		return p.order
	}
	out := make([]string, 0, len(p.order)+len(d.order))
	return append(append(out, p.order...), d.order...)
}

// Routes registers the API endpoints and, if Options.UI is set, the frontend.
//
// Every API route runs with a session and CSRF protection. Everything except the CSRF
// token and login requires an authenticated user.
func (p *Panel) Routes(r route.Route) {
	p.registerLoginLimiter()

	r.Prefix(p.opts.Path+"/api").Middleware(sessionmiddleware.StartSession(), csrfMiddleware{p}).Group(func(api route.Router) {
		api.Get("/auth/csrf", p.csrfToken)
		loginRoute := api
		if p.opts.Auth.RateLimiter != nil {
			loginRoute = api.Middleware(httpmiddleware.Throttle(loginLimiter))
		}
		loginRoute.Post("/auth/login", p.login)
		api.Get("/auth/me", p.me)

		api.Middleware(authMiddleware{p}).Group(func(g route.Router) {
			g.Post("/auth/logout", p.logout)
			g.Get("/meta", p.meta)
			g.Get("/resources/{resource}/records", p.list)
			g.Post("/resources/{resource}/records", p.create)
			g.Get("/resources/{resource}/records/{id}", p.show)
			g.Patch("/resources/{resource}/records/{id}", p.update)
			g.Delete("/resources/{resource}/records/{id}", p.destroy)
			g.Get("/resources/{resource}/options", p.resourceOptions)
			g.Get("/resources/{resource}/relations/{relation}/options", p.relationOptions)
			g.Get("/resources/{resource}/records/{id}/relations/{relation}/records", p.relatedList)
			g.Post("/resources/{resource}/records/{id}/relations/{relation}/records", p.relatedCreate)
			g.Post("/resources/{resource}/records/{id}/relations/{relation}/attach", p.relatedAttach)
			g.Post("/resources/{resource}/records/{id}/relations/{relation}/detach", p.relatedDetach)
			g.Post("/resources/{resource}/uploads/{field}", p.upload)
			g.Post("/resources/{resource}/actions/{action}", p.runAction)
			g.Post("/resources/{resource}/reorder", p.reorder)
			g.Get("/widgets/{widget}", p.widget)
			g.Get("/pages/{page}", p.pageData)
			g.Post("/pages/{page}", p.pageSubmit)
			g.Get("/search", p.search)
			p.pluginRoutes(g)
			if p.opts.Notifications {
				g.Get("/notifications", p.notifications)
				g.Post("/notifications/read", p.markRead(false))
				g.Post("/notifications/{id}/read", p.markRead(true))
				g.Delete("/notifications", p.deleteNotifications(false))
				g.Delete("/notifications/{id}", p.deleteNotifications(true))
			}
		})
	})

	if p.opts.UI != nil {
		p.uiRoutes(r)
	}
}

func (p *Panel) resource(ctx http.Context) (Resource, Config, bool) {
	reg, ok := p.lookup(ctx.Request().Route("resource"))
	return reg.res, reg.cfg, ok
}

// --- read -------------------------------------------------------------------------------

func (p *Panel) list(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	hc := p.hookContext(ctx)
	pol := r.policy()
	if !allows(pol.viewAny, hc) {
		return p.forbidden(ctx)
	}
	return p.listRecords(ctx, r, c, hc, p.scope(r, c, hc, p.opts.Orm.Query()), nil)
}

// listRecords answers a list request for r, starting from base (already scoped). extra
// adds keys to the response.
func (p *Panel) listRecords(ctx http.Context, r Resource, c Config, hc *HookContext, base orm.Query, extra http.Json) http.Response {
	params, err := parseList(c, ctx.Request().Origin().URL.Query(), listSchemaFor(r))
	if err != nil {
		return message(ctx, 400, err.Error())
	}
	filtered, err := params.filtered(c, base, hc)
	if err != nil {
		return message(ctx, 400, err.Error())
	}
	var sums map[string]map[string]*float64
	if len(params.Summaries) > 0 {
		if sums, err = summaries(r, filtered, params.Summaries); err != nil {
			return p.serverError(ctx, err)
		}
	}
	q := params.ordered(c, filtered)
	q, resp := p.preloadIncludes(ctx, q, r, c, params.Include, hc)
	if resp != nil {
		return resp
	}

	dest := r.newSlice()
	var total int64
	if err := q.Paginate(params.Page, params.PerPage, dest, &total); err != nil {
		return p.serverError(ctx, err)
	}
	if page := reflect.ValueOf(dest).Elem(); page.Len() > 0 {
		records := make([]any, page.Len())
		for i := range records {
			records[i] = page.Index(i).Addr().Interface()
		}
		if err := afterRead(r, hc, records...); err != nil {
			return p.serverError(ctx, err)
		}
	}

	force := forcedFields(c, params.Include)
	rows := reflect.ValueOf(dest).Elem()
	data := make([]any, rows.Len())
	abilities := make(map[string]recordAbilities, rows.Len())
	for i := range data {
		record := rows.Index(i).Addr().Interface()
		p.filterIncludedRecords(record, r, c, params.Include, hc)
		data[i] = p.encodeIncluded(rows.Index(i), force, r, c, params.Include)
		abilities[keyString(record, c.Key)] = abilitiesFor(r, c, hc, record)
	}

	out := http.Json{
		"data":      data,
		"abilities": abilities,
		"meta": http.Json{
			"page":      params.Page,
			"per_page":  params.PerPage,
			"total":     total,
			"last_page": int(math.Max(1, math.Ceil(float64(total)/float64(params.PerPage)))),
		},
	}
	if sums != nil {
		out["summary"] = sums
	}
	for k, v := range extra {
		out[k] = v
	}
	return ctx.Response().Json(200, out)
}

func (p *Panel) show(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	hc := p.hookContext(ctx)
	pol := r.policy()
	if !allows(pol.viewAny, hc) {
		return p.forbidden(ctx)
	}
	include, err := parseInclude(c, ctx)
	if err != nil {
		return message(ctx, 400, err.Error())
	}
	model, resp := p.find(ctx, r, c, ctx.Request().Route("id"), include, hc)
	if resp != nil {
		return resp
	}
	if !allowsRecord(pol.view, hc, model) {
		return p.forbidden(ctx)
	}
	if err := afterRead(r, hc, model); err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{
		"data":      p.encodeIncluded(reflect.ValueOf(model), forcedFields(c, include), r, c, include),
		"abilities": abilitiesFor(r, c, hc, model),
	})
}

func parseInclude(c Config, ctx http.Context) ([]string, error) {
	params, err := parseList(Config{Name: c.Name, Relations: c.Relations}, ctx.Request().Origin().URL.Query(), listSchema{})
	return params.Include, err
}

// find loads one record by key, or returns the error response to send. With a hook
// context, the policy scope applies: records outside it are a 404.
func (p *Panel) find(ctx http.Context, r Resource, c Config, key any, include []string, hc *HookContext) (any, http.Response) {
	q := p.opts.Orm.Query().Where(c.Key+" = ?", key)
	if hc != nil {
		q = p.scope(r, c, hc, q)
	}
	var resp http.Response
	q, resp = p.preloadIncludes(ctx, q, r, c, include, hc)
	if resp != nil {
		return nil, resp
	}
	model := r.newModel()
	if err := q.FirstOrFail(model); err != nil {
		if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
			return nil, message(ctx, 404, p.tr(ctx).recordNotFound)
		}
		return nil, p.serverError(ctx, err)
	}
	p.filterIncludedRecords(model, r, c, include, hc)
	return model, nil
}

// forcedFields turns included protocol relation names into Go field names whose key must
// always be present in the output.
func forcedFields(c Config, include []string) map[string]bool {
	force := make(map[string]bool, len(include))
	for _, rel := range include {
		force[c.Relations[rel]] = true
	}
	return force
}

// --- write ------------------------------------------------------------------------------

func (p *Panel) create(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	hc := p.hookContext(ctx)
	if pol := r.policy(); !allows(pol.viewAny, hc) || !allows(pol.create, hc) {
		return p.forbidden(ctx)
	}
	data, resp := readData(ctx)
	if resp != nil {
		return resp
	}
	stored, resp := p.createRecord(ctx, r, c, hc, data, nil, nil)
	if resp != nil {
		return resp
	}
	if err := afterRead(r, hc, stored); err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(201, http.Json{"data": encode(reflect.ValueOf(stored), nil), "abilities": abilitiesFor(r, c, hc, stored)})
}

// createRecord validates data and creates a record, returning it as stored. force sets
// fields after validation that the client may not set (e.g. the parent's key in a
// relation manager); then runs inside the transaction after the insert.
func (p *Panel) createRecord(ctx http.Context, r Resource, c Config, hc *HookContext, data, force map[string]any,
	then func(tx orm.Query, model any) error) (any, http.Response) {
	if resp := p.check(ctx, r, c, data, nil); resp != nil {
		return nil, resp
	}
	if resp := p.checkTenantRefs(ctx, r, c, hc, data); resp != nil {
		return nil, resp
	}
	sanitizeHTML(c, data)
	// The new record belongs to the current tenant; without one, nothing can be created.
	fill, ok := p.tenantFill(c, hc)
	if !ok {
		return nil, p.forbidden(ctx)
	}
	for k, v := range fill {
		data[k] = v
	}
	for k, v := range force {
		data[k] = v
	}

	h := r.hooks()
	if h.beforeCreate != nil {
		if err := h.beforeCreate(hc, data); err != nil {
			return nil, p.hookError(ctx, err)
		}
	}
	if err := p.hashFields(c, data); err != nil {
		return nil, p.serverError(ctx, err)
	}

	syncs, err := p.takeSyncs(r, c, data)
	if err != nil {
		return nil, p.decodeError(ctx, err)
	}
	files, err := p.takeUploads(ctx, c, data, nil)
	if err != nil {
		return nil, p.decodeError(ctx, err)
	}
	model := r.newModel()
	if err := decodeInto(model, data); err != nil {
		files.rollback()
		return nil, p.decodeError(ctx, err)
	}

	// Insert only the fields that were sent or set by hooks (plus timestamps), like
	// Eloquent does, so database defaults apply to everything else instead of Go zero values.
	columns := append(keys(data), timestampColumns(model, "created_at", "updated_at")...)
	err = p.transaction(hc, func(tx orm.Query) error {
		if err := tx.Select(columns...).Create(model); err != nil {
			return err
		}
		if err := p.applySyncs(tx, r, model, syncs, hc); err != nil {
			return err
		}
		if then != nil {
			if err := then(tx, model); err != nil {
				return err
			}
		}
		if h.afterSave != nil {
			if err := h.afterSave(hc, model, true); err != nil {
				return err
			}
		}
		return p.fireSaved(hc, c.Name, model, true)
	})
	if err != nil {
		files.rollback()
		return nil, p.hookError(ctx, err)
	}
	files.commit()

	// Re-read so the response reflects what the database actually stored.
	key, err := keyValue(model, c.Key)
	if err != nil {
		return nil, p.serverError(ctx, err)
	}
	return p.find(ctx, r, c, key, nil, nil)
}

// update is a partial update: only the fields in the body are validated and written.
func (p *Panel) update(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	hc := p.hookContext(ctx)
	pol := r.policy()
	if !allows(pol.viewAny, hc) {
		return p.forbidden(ctx)
	}
	id := ctx.Request().Route("id")
	model, resp := p.find(ctx, r, c, id, nil, hc)
	if resp != nil {
		return resp
	}
	if !allowsRecord(pol.update, hc, model) {
		return p.forbidden(ctx)
	}
	data, resp := readData(ctx)
	if resp != nil {
		return resp
	}
	if resp := p.check(ctx, r, c, data, id); resp != nil {
		return resp
	}
	if resp := p.checkTenantRefs(ctx, r, c, hc, data); resp != nil {
		return resp
	}
	sanitizeHTML(c, data)

	h := r.hooks()
	if h.beforeUpdate != nil {
		if err := h.beforeUpdate(hc, model, data); err != nil {
			return p.hookError(ctx, err)
		}
	}
	if err := p.hashFields(c, data); err != nil {
		return p.serverError(ctx, err)
	}

	syncs, err := p.takeSyncs(r, c, data)
	if err != nil {
		return p.decodeError(ctx, err)
	}
	files, err := p.takeUploads(ctx, c, data, model)
	if err != nil {
		return p.decodeError(ctx, err)
	}
	if err := decodeInto(model, data); err != nil {
		files.rollback()
		return p.decodeError(ctx, err)
	}
	if len(data) > 0 || len(syncs) > 0 {
		err := p.transaction(hc, func(tx orm.Query) error {
			if len(data) > 0 {
				columns := append(keys(data), timestampColumns(model, "updated_at")...)
				if _, err := tx.Model(model).Select(columns...).Update(model); err != nil {
					return err
				}
			}
			if err := p.applySyncs(tx, r, model, syncs, hc); err != nil {
				return err
			}
			if h.afterSave != nil {
				if err := h.afterSave(hc, model, false); err != nil {
					return err
				}
			}
			return p.fireSaved(hc, c.Name, model, false)
		})
		if err != nil {
			files.rollback()
			return p.hookError(ctx, err)
		}
	}
	files.commit()

	stored, resp := p.find(ctx, r, c, id, nil, nil)
	if resp != nil {
		return resp
	}
	if err := afterRead(r, hc, stored); err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{"data": encode(reflect.ValueOf(stored), nil), "abilities": abilitiesFor(r, c, hc, stored)})
}

func readData(ctx http.Context) (map[string]any, http.Response) {
	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := ctx.Request().Bind(&body); err != nil || body.Data == nil {
		return nil, message(ctx, 400, `Body must be JSON: {"data": {...}}.`)
	}
	return body.Data, nil
}

// check enforces the fillable whitelist and runs validation. For updates (updateKey != nil)
// only the rules of the sent fields run, and unique rules ignore the record being updated.
func (p *Panel) check(ctx http.Context, r Resource, c Config, data map[string]any, updateKey any) http.Response {
	// Mass-assignment protection: unknown fields are an error, not silently dropped.
	fieldErrors := map[string][]string{}
	for field := range data {
		if !contains(c.Fillable, field) {
			fieldErrors[field] = []string{p.tr(ctx).notFillable}
		}
	}
	if len(fieldErrors) > 0 {
		return p.validationError(ctx, fieldErrors)
	}

	return p.validate(ctx, data, rulesFor(c, data, updateKey, nullableFields(r)))
}

// nullableFields are the model fields that may hold null (pointers and nullable types).
func nullableFields(r Resource) map[string]bool {
	out := map[string]bool{}
	for name, info := range fieldsOf(reflectModel(r)) {
		if fm, ok := describeType(info.typ); ok && fm.Nullable {
			out[name] = true
		}
	}
	return out
}

// rulesFor picks the validation rules for a request.
//   - Nested rules ("faqs.*.question") run only when their list has items; otherwise
//     Goravel reports the wildcard key itself as missing, which no field can show.
//   - On update (updateKey set), only the rules of sent fields run, and unique rules
//     ignore the record being updated.
func rulesFor(c Config, data map[string]any, updateKey any, nullable map[string]bool) map[string]any {
	rules := make(map[string]any, len(c.Rules))
	for field, rule := range c.Rules {
		base, _, nested := strings.Cut(field, ".")
		value, sent := data[base]
		if nested && !hasItems(value) {
			continue
		}
		// Clearing a nullable field (null) skips its other rules, like Laravel's
		// "nullable": a format rule must not reject "no value". "required" still applies.
		if sent && value == nil && nullable[base] && !nested && !hasRule(rule, "required") {
			continue
		}
		if updateKey == nil {
			rules[field] = rule
		} else if sent {
			rules[field] = ignoreRecordInUnique(rule, c.Key, updateKey)
		}
	}
	return rules
}

// hasRule: whether a Goravel rule string ("required|max_len:5") contains a rule by name.
func hasRule(rules, name string) bool {
	for _, r := range strings.Split(rules, "|") {
		if r == name || strings.HasPrefix(r, name+":") {
			return true
		}
	}
	return false
}

func hasItems(v any) bool {
	switch x := v.(type) {
	case []any:
		return len(x) > 0
	case map[string]any:
		return len(x) > 0
	}
	return false
}

// validate runs Goravel validation with the panel's messages; nil when the data is valid.
func (p *Panel) validate(ctx http.Context, data map[string]any, rules map[string]any) http.Response {
	if len(rules) == 0 {
		return nil
	}
	var options []validation.Option
	if p.tr(ctx).rules != nil {
		options = append(options, frameworkvalidation.Messages(p.tr(ctx).rules))
	}
	v, err := p.opts.Validation.Make(ctx, data, rules, options...)
	if err != nil {
		return p.serverError(ctx, err)
	}
	if !v.Fails() {
		return nil
	}
	fieldErrors := map[string][]string{}
	for field, msgs := range v.Errors().All() {
		for _, m := range msgs {
			fieldErrors[field] = append(fieldErrors[field], p.tr(ctx).localize(m))
		}
	}
	return p.validationError(ctx, fieldErrors)
}

// ignoreRecordInUnique rewrites "unique:table,column" to "unique:table,column,key,id" so an
// update does not collide with the record itself. Rules that already have more parameters
// are left alone. "regex:" consumes the rest of the string in Goravel, so it is not split.
func ignoreRecordInUnique(rule, keyColumn string, id any) string {
	var out []string
	rest := rule
	for rest != "" {
		if strings.HasPrefix(rest, "regex:") || strings.HasPrefix(rest, "not_regex:") {
			out = append(out, rest)
			break
		}
		seg, after, _ := strings.Cut(rest, "|")
		rest = after
		if params, ok := strings.CutPrefix(seg, "unique:"); ok {
			parts := strings.Split(params, ",")
			if len(parts) <= 2 {
				for len(parts) < 2 {
					parts = append(parts, "")
				}
				seg = "unique:" + strings.Join(append(parts, keyColumn, fmt.Sprint(id)), ",")
			}
		}
		out = append(out, seg)
	}
	return strings.Join(out, "|")
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// timestampColumns returns the given timestamp columns that the model has.
func timestampColumns(model any, candidates ...string) []string {
	fields := fieldsOf(reflect.TypeOf(model).Elem())
	var cols []string
	for _, col := range candidates {
		if _, ok := fields[col]; ok {
			cols = append(cols, col)
		}
	}
	return cols
}

// keyValue reads the primary key from a model.
func keyValue(model any, key string) (any, error) {
	rv := reflect.ValueOf(model).Elem()
	info, ok := fieldsOf(rv.Type())[key]
	if !ok {
		return nil, fmt.Errorf("mks: model has no %q field", key)
	}
	return rv.FieldByIndex(info.index).Interface(), nil
}

// checkRelations makes sure every configured relation is a field on the model.
func checkRelations(model any, c Config) error {
	t := reflect.TypeOf(model).Elem()
	for rel, field := range c.Relations {
		if _, ok := t.FieldByName(field); !ok {
			return fmt.Errorf("mks: resource %q: relation %q points to missing field %s.%s", c.Name, rel, t.Name(), field)
		}
	}
	return nil
}

// --- errors -----------------------------------------------------------------------------

func message(ctx http.Context, status int, msg string) http.Response {
	return ctx.Response().Json(status, http.Json{"message": msg})
}

func (p *Panel) validationError(ctx http.Context, errs map[string][]string) http.Response {
	return ctx.Response().Json(422, http.Json{"message": p.tr(ctx).invalidData, "errors": errs})
}

func (p *Panel) decodeError(ctx http.Context, err error) http.Response {
	var fe fieldError
	if errors.As(err, &fe) {
		msg := p.tr(ctx).invalidType
		switch fe.kind {
		case invalidDateError:
			msg = p.tr(ctx).invalidDate
		case invalidUploadError:
			msg = p.tr(ctx).uploadInvalid
		case tooManyFilesError:
			msg = p.tr(ctx).tooManyFiles
		}
		return p.validationError(ctx, map[string][]string{fe.field: {msg}})
	}
	return message(ctx, 400, "Invalid data.")
}

// writeError maps database constraint errors that slipped past validation (for example two
// requests racing for the same unique value) to 409 instead of a generic 500.
func (p *Panel) writeError(ctx http.Context, err error) http.Response {
	if isDuplicateKey(err) {
		return message(ctx, 409, p.tr(ctx).duplicate)
	}
	if isForeignKeyViolation(err) {
		return message(ctx, 409, p.tr(ctx).inUse)
	}
	return p.serverError(ctx, err)
}

// isForeignKeyViolation reports a delete (or write) blocked by a foreign key, e.g. deleting
// a category that posts still use.
func isForeignKeyViolation(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Error 1451") || strings.Contains(msg, "Error 1452") || // MySQL / MariaDB
		strings.Contains(msg, "SQLSTATE 23503") || // PostgreSQL
		strings.Contains(msg, "FOREIGN KEY constraint failed") // SQLite
}

func isDuplicateKey(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "Error 1062") || // MySQL / MariaDB
		strings.Contains(msg, "SQLSTATE 23505") || // PostgreSQL
		strings.Contains(msg, "UNIQUE constraint failed") // SQLite
}

func (p *Panel) serverError(ctx http.Context, err error) http.Response {
	body := http.Json{"message": p.tr(ctx).serverError}
	if p.opts.Debug {
		body["error"] = err.Error()
	}
	return ctx.Response().Json(500, body)
}
