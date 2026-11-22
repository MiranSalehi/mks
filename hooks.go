package mks

import (
	"errors"
	"sort"
	"strings"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

// HookContext is passed to resource hooks.
type HookContext struct {
	HTTP http.Context
	// User is the logged-in panel user (the app's model, e.g. *models.User).
	User User
	// Query runs inside the save's transaction (nil in Before hooks, which run before it
	// starts). Use it for related writes that must succeed or fail together with the save.
	Query orm.Query

	panel  *Panel
	tenant *tenantState
	// Inside the panel's transaction: AfterCommit callbacks wait for the commit.
	inTx        bool
	afterCommit []func()
}

// AfterCommit runs fn once the save's transaction has committed (never if it rolls
// back), or at once outside a transaction. Use it for side effects others may observe,
// such as retiring caches: done earlier, a concurrent reader could cache the old data.
func (c *HookContext) AfterCommit(fn func()) {
	if c == nil || !c.inTx {
		fn()
		return
	}
	c.afterCommit = append(c.afterCommit, fn)
}

// transaction runs fn in a transaction with hc.Query set to it, then hc's AfterCommit
// callbacks if it committed.
func (p *Panel) transaction(hc *HookContext, fn func(tx orm.Query) error) error {
	hc.inTx, hc.afterCommit = true, nil
	err := p.opts.Orm.Transaction(func(tx orm.Query) error {
		hc.Query = tx
		return fn(tx)
	})
	pending := hc.afterCommit
	hc.inTx, hc.afterCommit = false, nil
	if err == nil {
		for _, f := range pending {
			f()
		}
	}
	return err
}

// Notify sends database notifications (see Panel.Notify). In an AfterSave hook or an
// action they are part of its transaction: kept only if it commits.
func (c *HookContext) Notify(n Notice, users ...any) error {
	if c.panel == nil {
		return errors.New("mks: Notify needs a hook context from the panel")
	}
	q := c.Query
	if q == nil {
		q = c.panel.opts.Orm.Query()
	}
	return c.panel.NotifyTx(q, n, users...)
}

// UserID is the logged-in user's id, as the panel reports it.
func (c *HookContext) UserID() any {
	if c.User == nil {
		return nil
	}
	return c.User.PanelUser().ID
}

// ValidationErrors rejects a save with field errors (422), e.g.
//
//	return mks.ValidationErrors{"published_at": "Pick a date in the future."}
type ValidationErrors map[string]string

func (e ValidationErrors) Error() string {
	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + ": " + e[k]
	}
	return "validation failed: " + strings.Join(parts, "; ")
}

// Reject stops a hook or an action with a message for the user (422, no field), e.g.
//
//	return mks.Reject("Published posts cannot be deleted.")
func Reject(message string) error { return rejection(message) }

type rejection string

func (r rejection) Error() string { return string(r) }

// RejectMessage is the message of an error made by Reject (for plugin routes that answer
// it themselves).
func RejectMessage(err error) (string, bool) {
	var r rejection
	if errors.As(err, &r) {
		return string(r), true
	}
	return "", false
}

type hooks struct {
	beforeCreate func(*HookContext, map[string]any) error
	beforeUpdate func(*HookContext, any, map[string]any) error
	afterSave    func(*HookContext, any, bool) error
	beforeDelete func(*HookContext, any) error
	afterRead    func(*HookContext, []any) error
}

// BeforeCreate runs after validation, before the insert. It may change data, including
// fields that are not fillable: the server decides those (e.g. the author).
func (r ModelResource[T]) BeforeCreate(fn func(ctx *HookContext, data map[string]any) error) ModelResource[T] {
	r.h.beforeCreate = fn
	return r
}

// BeforeUpdate runs after validation, before the update. record is the stored record;
// data holds only the changed fields and may be changed.
func (r ModelResource[T]) BeforeUpdate(fn func(ctx *HookContext, record *T, data map[string]any) error) ModelResource[T] {
	r.h.beforeUpdate = func(ctx *HookContext, record any, data map[string]any) error {
		return fn(ctx, record.(*T), data)
	}
	return r
}

// AfterSave runs inside the transaction after a create or update. Returning an error
// rolls the save back.
func (r ModelResource[T]) AfterSave(fn func(ctx *HookContext, record *T, created bool) error) ModelResource[T] {
	r.h.afterSave = func(ctx *HookContext, record any, created bool) error {
		return fn(ctx, record.(*T), created)
	}
	return r
}

// BeforeDelete runs inside the delete's transaction, for single and bulk deletes.
// Return ValidationErrors or another error to stop the delete (and roll back a bulk delete).
func (r ModelResource[T]) BeforeDelete(fn func(ctx *HookContext, record *T) error) ModelResource[T] {
	r.h.beforeDelete = func(ctx *HookContext, record any) error {
		return fn(ctx, record.(*T))
	}
	return r
}

// AfterRead runs on the records a response sends: each page of a list (also in relation
// managers), a single record, and the record returned by a create or update. Fill fields
// that are not columns (gorm:"-"), such as counts or attachments, with one query for the
// whole page instead of one per record.
func (r ModelResource[T]) AfterRead(fn func(ctx *HookContext, records []*T) error) ModelResource[T] {
	r.h.afterRead = func(ctx *HookContext, records []any) error {
		typed := make([]*T, len(records))
		for i, rec := range records {
			typed[i] = rec.(*T)
		}
		return fn(ctx, typed)
	}
	return r
}

// afterRead runs the resource's AfterRead on records (pointers to models).
func afterRead(r Resource, hc *HookContext, records ...any) error {
	if h := r.hooks(); h.afterRead != nil && len(records) > 0 {
		return h.afterRead(hc, records)
	}
	return nil
}

func (r ModelResource[T]) hooks() hooks { return r.h }

func (p *Panel) hookContext(ctx http.Context) *HookContext {
	hc := &HookContext{HTTP: ctx, User: p.currentUser(ctx), panel: p}
	hc.tenant = p.resolveTenant(ctx, hc)
	return hc
}

// hookError turns a hook's error into a response.
func (p *Panel) hookError(ctx http.Context, err error) http.Response {
	var rej rejection
	if errors.As(err, &rej) {
		return ctx.Response().Json(422, http.Json{"message": string(rej), "errors": map[string][]string{}})
	}
	var ve ValidationErrors
	if errors.As(err, &ve) {
		errs := make(map[string][]string, len(ve))
		for field, msg := range ve {
			errs[field] = []string{msg}
		}
		return p.validationError(ctx, errs)
	}
	return p.writeError(ctx, err)
}
