package mks

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// Relation managers: the related records of one record, e.g. the posts of a category
// (has-many) or the posts of a tag (many-to-many).
//
//	GET  /resources/{resource}/records/{id}/relations/{relation}/records   list (the related resource's list parameters)
//	POST /resources/{resource}/records/{id}/relations/{relation}/records   create a related record
//	POST /resources/{resource}/records/{id}/relations/{relation}/attach    {"records": [keys]}  many-to-many only
//	POST /resources/{resource}/records/{id}/relations/{relation}/detach    {"records": [keys]}  many-to-many only
//
// Editing and deleting a related record use its own resource's endpoints.

const (
	hasMany    = "has_many"
	manyToMany = "many_to_many"
)

// relationLink is how two tables are joined, read from the ORM's schema (so gorm tags
// like foreignKey and many2many are respected). All names come from code, not requests.
type relationLink struct {
	kind string
	// has-many: the related table's column holding the parent's key
	foreignKey string
	// many-to-many: the join table and its columns
	joinTable, joinParent, joinRelated string
	// the parent column the link refers to, and the related column for many-to-many
	parentKey, relatedKey string
}

func (p *Panel) namer() schema.Namer {
	if p.opts.Orm != nil {
		if g, ok := p.opts.Orm.Query().(interface{ Instance() *gorm.DB }); ok {
			if db := g.Instance(); db != nil && db.NamingStrategy != nil {
				return db.NamingStrategy
			}
		}
	}
	return schema.NamingStrategy{}
}

func (p *Panel) schemaOf(model any) (*schema.Schema, error) {
	return schema.Parse(model, &p.schemas, p.namer())
}

func (p *Panel) relationLink(r Resource, rel relationInfo) (relationLink, *schema.Schema, error) {
	sch, err := p.schemaOf(r.newModel())
	if err != nil {
		return relationLink{}, nil, err
	}
	gr, ok := sch.Relationships.Relations[rel.field]
	if !ok || gr.Polymorphic != nil {
		return relationLink{}, nil, fmt.Errorf("mks: relation %s.%s is not supported", sch.Name, rel.field)
	}
	var l relationLink
	switch gr.Type {
	case schema.HasMany:
		l.kind = hasMany
		for _, ref := range gr.References {
			if ref.OwnPrimaryKey && ref.PrimaryKey != nil {
				l.foreignKey, l.parentKey = ref.ForeignKey.DBName, ref.PrimaryKey.DBName
			}
		}
	case schema.Many2Many:
		l.kind = manyToMany
		if gr.JoinTable == nil {
			return l, nil, fmt.Errorf("mks: relation %s.%s has no join table", sch.Name, rel.field)
		}
		l.joinTable = gr.JoinTable.Table
		for _, ref := range gr.References {
			if ref.PrimaryKey == nil {
				continue
			}
			if ref.OwnPrimaryKey {
				l.joinParent, l.parentKey = ref.ForeignKey.DBName, ref.PrimaryKey.DBName
			} else {
				l.joinRelated, l.relatedKey = ref.ForeignKey.DBName, ref.PrimaryKey.DBName
			}
		}
	default:
		return l, nil, fmt.Errorf("mks: relation %s.%s: only has-many and many-to-many have relation managers", sch.Name, rel.field)
	}
	if l.parentKey == "" || (l.kind == hasMany && l.foreignKey == "") || (l.kind == manyToMany && (l.joinParent == "" || l.joinRelated == "")) {
		return l, nil, fmt.Errorf("mks: relation %s.%s: cannot read its keys", sch.Name, rel.field)
	}
	return l, sch, nil
}

// --- join table writes ----------------------------------------------------------------
//
// Many-to-many links are written to the join table directly. GORM's Association API
// would also upsert the related records with all their fields, which breaks on columns
// with constraints (an enum does not accept "").

// keyText compares keys as text, so 3, 3.0 and "3" are the same key.
func keyText(v any) string {
	if f, ok := v.(float64); ok && f == float64(int64(f)) {
		return fmt.Sprint(int64(f))
	}
	return fmt.Sprint(v)
}

// attach links related keys to the parent; keys already linked are left alone.
func (l relationLink) attach(tx orm.Query, parent any, related []any) error {
	if len(related) == 0 {
		return nil
	}
	var linked []string
	if err := tx.Table(l.joinTable).Where(l.joinParent+" = ?", parent).WhereIn(l.joinRelated, related).Pluck(l.joinRelated, &linked); err != nil {
		return err
	}
	done := map[string]bool{}
	for _, k := range linked {
		done[k] = true
	}
	for _, k := range related {
		if done[keyText(k)] {
			continue
		}
		done[keyText(k)] = true
		if _, err := tx.Exec("INSERT INTO "+l.joinTable+" ("+l.joinParent+", "+l.joinRelated+") VALUES (?, ?)", parent, k); err != nil {
			return err
		}
	}
	return nil
}

// detach removes links; the related records stay.
func (l relationLink) detach(tx orm.Query, parent any, related []any) error {
	if len(related) == 0 {
		return nil
	}
	_, err := tx.Exec("DELETE FROM "+l.joinTable+" WHERE "+l.joinParent+" = ? AND "+l.joinRelated+" IN ?", parent, related)
	return err
}

// replace makes related the parent's exact set of links.
func (l relationLink) replace(tx orm.Query, parent any, related []any) error {
	var err error
	if len(related) == 0 {
		_, err = tx.Exec("DELETE FROM "+l.joinTable+" WHERE "+l.joinParent+" = ?", parent)
	} else {
		_, err = tx.Exec("DELETE FROM "+l.joinTable+" WHERE "+l.joinParent+" = ? AND "+l.joinRelated+" NOT IN ?", parent, related)
	}
	if err != nil {
		return err
	}
	return l.attach(tx, parent, related)
}

// columnValue reads a model's value for a column, via the ORM schema.
func (p *Panel) columnValue(model any, column string) (any, error) {
	sch, err := p.schemaOf(model)
	if err != nil {
		return nil, err
	}
	field := sch.LookUpField(column)
	if field == nil {
		return nil, fmt.Errorf("mks: %s has no column %q", sch.Name, column)
	}
	v, _ := field.ValueOf(context.Background(), reflect.ValueOf(model).Elem())
	return v, nil
}

// relatedRequest is what every relation manager request needs.
type relatedRequest struct {
	r       Resource
	c       Config
	hc      *HookContext
	parent  any
	value   any // the parent's key value the link refers to
	rel     relationInfo
	link    relationLink
	target  Resource
	tc      Config
	canEdit bool // the user may update the parent, which adding and removing related records is
}

// related checks access and loads the parent: the user must see the parent resource and
// record and the related resource. Records outside the parent's scope are a 404.
func (p *Panel) related(ctx http.Context) (*relatedRequest, http.Response) {
	r, c, ok := p.resource(ctx)
	if !ok {
		return nil, message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	hc := p.hookContext(ctx)
	pol := r.policy()
	if !allows(pol.viewAny, hc) {
		return nil, p.forbidden(ctx)
	}
	rel, ok := p.relation(r, c, ctx.Request().Route("relation"))
	if !ok || !rel.many {
		return nil, message(ctx, 404, "Relation not found.")
	}
	target := rel.target.res
	if !allows(target.policy().viewAny, hc) {
		return nil, p.forbidden(ctx)
	}
	parent, resp := p.find(ctx, r, c, ctx.Request().Route("id"), nil, hc)
	if resp != nil {
		return nil, resp
	}
	if !allowsRecord(pol.view, hc, parent) {
		return nil, p.forbidden(ctx)
	}
	link, _, err := p.relationLink(r, rel)
	if err != nil {
		return nil, p.serverError(ctx, err)
	}
	value, err := p.columnValue(parent, link.parentKey)
	if err != nil {
		return nil, p.serverError(ctx, err)
	}
	return &relatedRequest{
		r: r, c: c, hc: hc, parent: parent, value: value, rel: rel, link: link,
		target: target, tc: rel.target.cfg, canEdit: allowsRecord(pol.update, hc, parent),
	}, nil
}

// query limits q (on the related table) to the parent's related records.
func (rr *relatedRequest) query(q orm.Query) orm.Query {
	if rr.link.kind == hasMany {
		return q.Where(rr.link.foreignKey+" = ?", rr.value)
	}
	return q.Where(rr.link.relatedKey+" IN (SELECT "+rr.link.joinRelated+" FROM "+rr.link.joinTable+" WHERE "+rr.link.joinParent+" = ?)", rr.value)
}

// relationMeta tells the UI what kind of relation it is and what the user may do.
type relationMeta struct {
	Type string `json:"type"`
	// ForeignKey (has-many) is set by the server on create; the form should not show it.
	ForeignKey string `json:"foreign_key,omitempty"`
	Create     bool   `json:"create"`
	Attach     bool   `json:"attach"`
	Detach     bool   `json:"detach"`
}

func (p *Panel) relatedList(ctx http.Context) http.Response {
	rr, resp := p.related(ctx)
	if resp != nil {
		return resp
	}
	tpol := rr.target.policy()
	meta := relationMeta{
		Type:   rr.link.kind,
		Create: rr.canEdit && allows(tpol.create, rr.hc),
		Attach: rr.canEdit && rr.link.kind == manyToMany,
		Detach: rr.canEdit && rr.link.kind == manyToMany,
	}
	if rr.link.kind == hasMany {
		meta.ForeignKey = rr.link.foreignKey
	}
	base := rr.query(p.scope(rr.target, rr.tc, rr.hc, p.opts.Orm.Query()))
	return p.listRecords(ctx, rr.target, rr.tc, rr.hc, base, http.Json{"relation": meta})
}

// relatedCreate creates a record of the related resource, with its validation and hooks,
// and links it to the parent in the same transaction.
func (p *Panel) relatedCreate(ctx http.Context) http.Response {
	rr, resp := p.related(ctx)
	if resp != nil {
		return resp
	}
	if !rr.canEdit || !allows(rr.target.policy().create, rr.hc) {
		return p.forbidden(ctx)
	}
	data, resp := readData(ctx)
	if resp != nil {
		return resp
	}
	var force map[string]any
	var then func(tx orm.Query, model any) error
	if rr.link.kind == hasMany {
		// The parent's key, whatever the client sent. A fillable key goes through
		// validation (so "required" and "exists" rules pass); otherwise it is set after.
		value := jsonValue(rr.value)
		if contains(rr.tc.Fillable, rr.link.foreignKey) {
			data[rr.link.foreignKey] = value
		} else {
			delete(data, rr.link.foreignKey)
			force = map[string]any{rr.link.foreignKey: value}
		}
	} else {
		then = func(tx orm.Query, model any) error {
			key, err := p.columnValue(model, rr.link.relatedKey)
			if err != nil {
				return err
			}
			return rr.link.attach(tx, rr.value, []any{key})
		}
	}
	stored, resp := p.createRecord(ctx, rr.target, rr.tc, rr.hc, data, force, then)
	if resp != nil {
		return resp
	}
	if err := afterRead(rr.target, rr.hc, stored); err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(201, http.Json{"data": encode(reflect.ValueOf(stored), nil), "abilities": abilitiesFor(rr.target, rr.tc, rr.hc, stored)})
}

func (p *Panel) relatedAttach(ctx http.Context) http.Response { return p.attachOrDetach(ctx, true) }
func (p *Panel) relatedDetach(ctx http.Context) http.Response { return p.attachOrDetach(ctx, false) }

func (p *Panel) attachOrDetach(ctx http.Context, attach bool) http.Response {
	rr, resp := p.related(ctx)
	if resp != nil {
		return resp
	}
	if rr.link.kind != manyToMany {
		return message(ctx, 400, "Only many-to-many relations can attach and detach.")
	}
	if !rr.canEdit {
		return p.forbidden(ctx)
	}
	var body struct {
		Records []any `json:"records"`
	}
	if err := ctx.Request().Bind(&body); err != nil || len(body.Records) == 0 {
		return message(ctx, 400, `Body must be JSON: {"records": [...]}.`)
	}
	if len(body.Records) > maxActionRecords {
		return message(ctx, 400, fmt.Sprintf("At most %d records at once.", maxActionRecords))
	}
	keys := uniqueKeys(body.Records)

	// The related records must exist for this user (their scope), and be visible.
	tpol := rr.target.policy()
	slice := rr.target.newSlice()
	q := p.scope(rr.target, rr.tc, rr.hc, p.opts.Orm.Query()).WhereIn(rr.tc.Key, keys)
	if !attach {
		q = rr.query(q)
	}
	if err := q.Find(slice); err != nil {
		return p.serverError(ctx, err)
	}
	rows := reflect.ValueOf(slice).Elem()
	if rows.Len() != len(keys) {
		return message(ctx, 404, p.tr(ctx).recordNotFound)
	}
	related := make([]any, rows.Len())
	for i := range related {
		record := rows.Index(i).Addr().Interface()
		if !allowsRecord(tpol.view, rr.hc, record) {
			return p.forbidden(ctx)
		}
		key, err := p.columnValue(record, rr.link.relatedKey)
		if err != nil {
			return p.serverError(ctx, err)
		}
		related[i] = key
	}

	err := p.opts.Orm.Transaction(func(tx orm.Query) error {
		if attach {
			return rr.link.attach(tx, rr.value, related)
		}
		return rr.link.detach(tx, rr.value, related)
	})
	if err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{"count": len(related)})
}

// uniqueKeys drops repeated keys, comparing them as text (1 and "1" are the same key).
func uniqueKeys(keys []any) []any {
	seen := map[string]bool{}
	out := make([]any, 0, len(keys))
	for _, k := range keys {
		s := keyText(k)
		if !seen[s] {
			seen[s] = true
			out = append(out, k)
		}
	}
	return out
}

// jsonValue turns a Go value into what JSON decoding would give (numbers become
// float64), so it can sit in request data next to client values.
func jsonValue(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}
