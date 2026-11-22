package mks

import (
	"fmt"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

// Policy decides what the logged-in user may do with a resource. Every rule is optional;
// a rule that is not set allows. The server checks these on every request; the abilities
// the UI receives only hide buttons.
//
//	mks.Define[models.Post](cfg).Policy(mks.Policy[models.Post]{
//	    Create: func(ctx *mks.HookContext) bool { return role(ctx) != "viewer" },
//	    Update: func(ctx *mks.HookContext, p *models.Post) bool { return isAdmin(ctx) || p.AuthorID == ctx.UserID() },
//	    Delete: func(ctx *mks.HookContext, _ *models.Post) bool { return isAdmin(ctx) },
//	})
type Policy[T any] struct {
	// ViewAny: may the user open the resource at all (list, menu, meta).
	ViewAny func(ctx *HookContext) bool
	// View: may the user see this record (edit page, API).
	View   func(ctx *HookContext, record *T) bool
	Create func(ctx *HookContext) bool
	Update func(ctx *HookContext, record *T) bool
	Delete func(ctx *HookContext, record *T) bool
	// Reorder: may the user change the manual order (Config.Reorderable)? Each moved record
	// also needs Update.
	Reorder func(ctx *HookContext) bool
	// Scope limits which records exist for the user, e.g. only their own. It applies to
	// lists, single records, updates, deletes, actions and relation options; records
	// outside it are a 404. Scope may use WHERE predicates and Scopes composed only of
	// WHERE predicates (Where, OrWhere, WhereIn, etc.). Other query operations panic before
	// SQL because the policy is grouped to keep OR branches inside tenant and record constraints.
	Scope func(ctx *HookContext, q orm.Query) orm.Query
}

type policyDef struct {
	viewAny func(*HookContext) bool
	view    func(*HookContext, any) bool
	create  func(*HookContext) bool
	update  func(*HookContext, any) bool
	delete  func(*HookContext, any) bool
	reorder func(*HookContext) bool
	scope   func(*HookContext, orm.Query) orm.Query
}

// Policy sets the resource's authorization rules.
func (r ModelResource[T]) Policy(p Policy[T]) ModelResource[T] {
	r.pol = policyDef{viewAny: p.ViewAny, create: p.Create, reorder: p.Reorder, scope: p.Scope}
	if p.View != nil {
		r.pol.view = func(ctx *HookContext, rec any) bool { return p.View(ctx, rec.(*T)) }
	}
	if p.Update != nil {
		r.pol.update = func(ctx *HookContext, rec any) bool { return p.Update(ctx, rec.(*T)) }
	}
	if p.Delete != nil {
		r.pol.delete = func(ctx *HookContext, rec any) bool { return p.Delete(ctx, rec.(*T)) }
	}
	return r
}

func (r ModelResource[T]) policy() policyDef { return r.pol }

func allows(fn func(*HookContext) bool, hc *HookContext) bool { return fn == nil || fn(hc) }

func allowsRecord(fn func(*HookContext, any) bool, hc *HookContext, record any) bool {
	return fn == nil || fn(hc, record)
}

// scoped applies the policy scope to a query.
func scoped(pol policyDef, hc *HookContext, q orm.Query) orm.Query {
	if pol.scope != nil {
		return pol.scope(hc, q)
	}
	return q
}

// canRunAction checks an action for one record (nil for header actions). An action
// without Authorize may run on records the user may update, and as a header action for
// anyone who can see the resource.
func canRunAction(a actionDef, pol policyDef, hc *HookContext, record any) bool {
	if a.authorize != nil {
		return a.authorize(hc, record)
	}
	if record == nil {
		return true
	}
	return allowsRecord(pol.update, hc, record)
}

// --- abilities sent to the UI -----------------------------------------------------------

// recordAbilities says what the user may do with one record. Buttons are hidden by it;
// the server checks again on every request.
type recordAbilities struct {
	Update  bool            `json:"update"`
	Delete  bool            `json:"delete"`
	Actions map[string]bool `json:"actions"`
}

func abilitiesFor(r Resource, c Config, hc *HookContext, record any) recordAbilities {
	pol := r.policy()
	a := recordAbilities{
		Update:  allowsRecord(pol.update, hc, record),
		Delete:  c.Deletable && allowsRecord(pol.delete, hc, record),
		Actions: map[string]bool{},
	}
	for _, act := range r.actions() {
		if !act.withoutRecords {
			a.Actions[act.name] = canRunAction(act, pol, hc, record)
		}
	}
	return a
}

type resourceAbilities struct {
	Create bool `json:"create"`
	// Delete: whether delete buttons can appear at all (each record is checked too).
	Delete bool `json:"delete"`
	// Reorder: whether the user may change the manual order (each record needs update).
	Reorder bool `json:"reorder"`
	// HeaderActions lists the header actions the user may run.
	HeaderActions map[string]bool `json:"header_actions"`
}

func resourceAbilitiesFor(r Resource, c Config, hc *HookContext) resourceAbilities {
	pol := r.policy()
	a := resourceAbilities{Create: allows(pol.create, hc), Delete: c.Deletable, Reorder: c.Reorderable != "" && allows(pol.reorder, hc), HeaderActions: map[string]bool{}}
	for _, act := range r.actions() {
		if act.withoutRecords {
			a.HeaderActions[act.name] = canRunAction(act, pol, hc, nil)
		}
	}
	return a
}

func (p *Panel) forbidden(ctx http.Context) http.Response {
	return message(ctx, 403, p.tr(ctx).forbidden)
}

// keyString is the record key as the abilities map uses it.
func keyString(record any, key string) string {
	v, err := keyValue(record, key)
	if err != nil {
		return ""
	}
	return fmt.Sprint(v)
}
