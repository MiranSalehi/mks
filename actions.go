package mks

import (
	"fmt"
	"reflect"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

// maxActionRecords caps how many records one action call may load.
const maxActionRecords = 500

// Action is a server-side operation on a resource, run from a button in the UI.
//
//	mks.Action[models.Post]{
//	    Name: "publish",
//	    Handle: func(ctx *mks.ActionContext[models.Post]) (*mks.ActionResult, error) {
//	        for _, post := range ctx.Records { ... }
//	        return mks.Success("Published."), nil
//	    },
//	}
type Action[T any] struct {
	// Name is the URL segment and the key the frontend uses. "delete" is reserved.
	Name string
	// WithoutRecords makes it a header action that runs on no records. Otherwise the
	// request must name at least one record (a row or a bulk selection).
	WithoutRecords bool
	// Rules validate the data of the action's modal form, in Goravel syntax.
	Rules map[string]string
	// Handle runs inside a transaction (ctx.Query); returning an error rolls it back.
	Handle func(ctx *ActionContext[T]) (*ActionResult, error)
	// Authorize decides per record (nil for header actions). Without it, record actions
	// follow the policy's Update rule and header actions are open to the resource's users.
	Authorize func(ctx *HookContext, record *T) bool
}

// ActionContext is passed to an action's Handle.
type ActionContext[T any] struct {
	*HookContext
	// Records are the selected records, loaded from the database.
	Records []*T
	// Data is the validated data of the action's form.
	Data map[string]any
}

// ActionResult tells the UI what happened.
type ActionResult struct {
	Notification *Notification `json:"notification,omitempty"`
	// Redirect is a path inside the panel to open afterwards, e.g. "/posts".
	Redirect string `json:"redirect,omitempty"`
	// OpenURL navigates the browser to a public URL, for example a short-lived preview.
	OpenURL string `json:"open_url,omitempty"`
	// Invalidate lists resources whose data the UI should reload. Default: this resource.
	Invalidate []string `json:"invalidate"`
	// Data is a result for the page or action UI to show (e.g. an import's counts).
	Data any `json:"data,omitempty"`
}

type Notification struct {
	// Type is "success", "warning", "danger" or "info".
	Type  string `json:"type"`
	Title string `json:"title"`
}

func Success(title string) *ActionResult {
	return &ActionResult{Notification: &Notification{Type: "success", Title: title}}
}

func Warning(title string) *ActionResult {
	return &ActionResult{Notification: &Notification{Type: "warning", Title: title}}
}

type actionDef struct {
	name           string
	withoutRecords bool
	rules          map[string]string
	run            func(hc *HookContext, records any, data map[string]any) (*ActionResult, error)
	authorize      func(hc *HookContext, record any) bool
}

// Actions adds server actions to the resource.
func (r ModelResource[T]) Actions(actions ...Action[T]) ModelResource[T] {
	acts := append([]actionDef{}, r.acts...)
	for _, a := range actions {
		handle := a.Handle
		def := actionDef{name: a.Name, withoutRecords: a.WithoutRecords, rules: a.Rules}
		if authorize := a.Authorize; authorize != nil {
			def.authorize = func(hc *HookContext, record any) bool {
				if record == nil {
					return authorize(hc, nil)
				}
				return authorize(hc, record.(*T))
			}
		}
		if handle != nil { // left nil otherwise, so Register reports the missing Handle
			def.run = func(hc *HookContext, records any, data map[string]any) (*ActionResult, error) {
				var list []*T
				if records != nil {
					slice := reflect.ValueOf(records).Elem()
					list = make([]*T, slice.Len())
					for i := range list {
						list[i] = slice.Index(i).Addr().Interface().(*T)
					}
				}
				return handle(&ActionContext[T]{HookContext: hc, Records: list, Data: data})
			}
		}
		acts = append(acts, def)
	}
	r.acts = acts
	return r
}

func (r ModelResource[T]) actions() []actionDef { return r.acts }

func validateActions(r Resource, c Config) error {
	seen := map[string]bool{}
	for _, a := range r.actions() {
		if !identifier.MatchString(a.name) {
			return fmt.Errorf("mks: resource %q: invalid action name %q", c.Name, a.name)
		}
		if a.name == "delete" {
			return fmt.Errorf("mks: resource %q: action name \"delete\" is reserved; set Deletable instead", c.Name)
		}
		if seen[a.name] {
			return fmt.Errorf("mks: resource %q: action %q defined twice", c.Name, a.name)
		}
		if a.run == nil {
			return fmt.Errorf("mks: resource %q: action %q has no Handle", c.Name, a.name)
		}
		seen[a.name] = true
	}
	return nil
}

func findAction(r Resource, name string) (actionDef, bool) {
	for _, a := range r.actions() {
		if a.name == name {
			return a, true
		}
	}
	return actionDef{}, false
}

// --- handlers ---------------------------------------------------------------------------

type actionRequest struct {
	Records []any          `json:"records"`
	Data    map[string]any `json:"data"`
}

func (p *Panel) runAction(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	hc := p.hookContext(ctx)
	pol := r.policy()
	if !allows(pol.viewAny, hc) {
		return p.forbidden(ctx)
	}
	var body actionRequest
	if err := ctx.Request().Bind(&body); err != nil {
		return message(ctx, 400, `Body must be JSON: {"records": [...], "data": {...}}.`)
	}
	if body.Data == nil {
		body.Data = map[string]any{}
	}
	if len(body.Records) > maxActionRecords {
		return message(ctx, 400, fmt.Sprintf("At most %d records per action.", maxActionRecords))
	}

	name := ctx.Request().Route("action")
	if name == "delete" {
		if !c.Deletable {
			return message(ctx, 404, "Action not found.")
		}
		result, resp := p.deleteRecords(ctx, r, c, body.Records, hc)
		if resp != nil {
			return resp
		}
		return p.actionResponse(ctx, c, result)
	}
	def, ok := findAction(r, name)
	if !ok {
		return message(ctx, 404, "Action not found.")
	}
	if def.withoutRecords && len(body.Records) > 0 {
		return message(ctx, 400, "This action runs without records.")
	}
	if !def.withoutRecords && len(body.Records) == 0 {
		return message(ctx, 400, "This action needs at least one record.")
	}

	rules := make(map[string]any, len(def.rules))
	for k, v := range def.rules {
		rules[k] = v
	}
	if resp := p.validate(ctx, body.Data, rules); resp != nil {
		return resp
	}

	var records any
	if def.withoutRecords {
		if !canRunAction(def, pol, hc, nil) {
			return p.forbidden(ctx)
		}
	} else {
		var resp http.Response
		if records, resp = p.loadRecords(ctx, r, c, body.Records, p.scope(r, c, hc, p.opts.Orm.Query())); resp != nil {
			return resp
		}
		slice := reflect.ValueOf(records).Elem()
		for i := range slice.Len() {
			if !canRunAction(def, pol, hc, slice.Index(i).Addr().Interface()) {
				return p.forbidden(ctx)
			}
		}
	}

	var result *ActionResult
	err := p.transaction(hc, func(tx orm.Query) error {
		var err error
		result, err = def.run(hc, records, body.Data)
		return err
	})
	if err != nil {
		return p.hookError(ctx, err)
	}
	return p.actionResponse(ctx, c, result)
}

func (p *Panel) actionResponse(ctx http.Context, c Config, result *ActionResult) http.Response {
	if result == nil {
		result = &ActionResult{}
	}
	if result.Notification == nil {
		result.Notification = &Notification{Type: "success", Title: p.tr(ctx).actionDone}
	}
	if result.Invalidate == nil {
		result.Invalidate = []string{c.Name}
	}
	return ctx.Response().Json(200, result)
}

// loadRecords loads the records with the given keys. A missing key is a 404, so an action
// never silently runs on fewer records than the user selected.
func (p *Panel) loadRecords(ctx http.Context, r Resource, c Config, keys []any, q orm.Query) (any, http.Response) {
	unique := map[string]bool{}
	ids := make([]any, 0, len(keys))
	for _, k := range keys {
		s := fmt.Sprint(k)
		if !unique[s] {
			unique[s] = true
			ids = append(ids, k)
		}
	}
	slice := r.newSlice()
	if err := q.WhereIn(c.Key, ids).Find(slice); err != nil {
		return nil, p.serverError(ctx, err)
	}
	if reflect.ValueOf(slice).Elem().Len() != len(ids) {
		return nil, message(ctx, 404, p.tr(ctx).recordNotFound)
	}
	return slice, nil
}

// destroy deletes one record: DELETE /records/{id}.
func (p *Panel) destroy(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	if !c.Deletable {
		return message(ctx, 405, "This resource does not allow deleting.")
	}
	hc := p.hookContext(ctx)
	if !allows(r.policy().viewAny, hc) {
		return p.forbidden(ctx)
	}
	if _, resp := p.deleteRecords(ctx, r, c, []any{ctx.Request().Route("id")}, hc); resp != nil {
		return resp
	}
	return ctx.Response().NoContent()
}

// deleteRecords deletes the records in one transaction, running BeforeDelete for each.
// It returns the result, or the error response to send.
func (p *Panel) deleteRecords(ctx http.Context, r Resource, c Config, keys []any, hc *HookContext) (*ActionResult, http.Response) {
	if len(keys) == 0 {
		return nil, message(ctx, 400, "Select at least one record.")
	}
	pol := r.policy()
	records, resp := p.loadRecords(ctx, r, c, keys, p.scope(r, c, hc, p.opts.Orm.Query()))
	if resp != nil {
		return nil, resp
	}
	h := r.hooks()
	slice := reflect.ValueOf(records).Elem()
	for i := range slice.Len() {
		if !allowsRecord(pol.delete, hc, slice.Index(i).Addr().Interface()) {
			return nil, p.forbidden(ctx)
		}
	}
	var files []fileRef
	for i := range slice.Len() {
		files = append(files, managedFiles(c, slice.Index(i).Addr().Interface())...)
	}
	err := p.transaction(hc, func(tx orm.Query) error {
		for i := range slice.Len() {
			record := slice.Index(i).Addr().Interface()
			if h.beforeDelete != nil {
				if err := h.beforeDelete(hc, record); err != nil {
					return err
				}
			}
			if _, err := tx.Delete(record); err != nil {
				return err
			}
			if err := p.fireDeleted(hc, c.Name, record); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, p.hookError(ctx, err)
	}
	// The records are gone; their uploaded files go too.
	(&fileChanges{p: p, old: files}).commit()
	count := p.tr(ctx).localize(fmt.Sprint(slice.Len()))
	return &ActionResult{Notification: &Notification{Type: "success", Title: fmt.Sprintf(p.tr(ctx).deleted, count)}}, nil
}
