package mks

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

const optionsPageSize = 25

type relationInfo struct {
	field  string       // Go field name, e.g. "Tags"
	target registered   // the resource for the related model
	many   bool         // a slice: has-many or many-to-many
	elem   reflect.Type // the related model type
}

// relation resolves a protocol relation name to its field and the registered resource of
// the related model. Relations to models that are not panel resources are not exposed.
func (p *Panel) relation(r Resource, c Config, name string) (relationInfo, bool) {
	field, ok := c.Relations[name]
	if !ok {
		return relationInfo{}, false
	}
	sf, ok := reflect.TypeOf(r.newModel()).Elem().FieldByName(field)
	if !ok {
		return relationInfo{}, false
	}
	t, many := sf.Type, false
	if t.Kind() == reflect.Slice {
		t, many = t.Elem(), true
	}
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for _, name := range p.names() {
		reg, _ := p.lookup(name)
		if reflect.TypeOf(reg.res.newModel()).Elem() == t {
			return relationInfo{field: field, target: reg, many: many, elem: t}, true
		}
	}
	return relationInfo{}, false
}

// --- options for relation selects ---------------------------------------------------------

type option struct {
	Value any    `json:"value"`
	Label string `json:"label"`
}

// relationOptions answers GET /resources/{resource}/relations/{relation}/options.
//
//	?search=go&page=2   search the related resource (its Searchable, or its TitleColumn)
//	?values=1,4         labels for the given keys, e.g. the current value of a select
func (p *Panel) relationOptions(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	rel, ok := p.relation(r, c, ctx.Request().Route("relation"))
	if !ok {
		return message(ctx, 404, "Relation not found.")
	}
	// Options show records of the related resource: both resources must be visible, and
	// the related resource's scope applies.
	hc := p.hookContext(ctx)
	if !allows(r.policy().viewAny, hc) || !allows(rel.target.res.policy().viewAny, hc) {
		return p.forbidden(ctx)
	}
	return p.options(ctx, hc, rel.target)
}

// resourceOptions answers GET /resources/{resource}/options: the records of a resource
// itself as select options (same parameters as relation options). It is for values that
// point at a resource without being an ORM relation, e.g. a page id inside a JSON menu.
func (p *Panel) resourceOptions(ctx http.Context) http.Response {
	reg, ok := p.lookup(ctx.Request().Route("resource"))
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	hc := p.hookContext(ctx)
	if !allows(reg.res.policy().viewAny, hc) {
		return p.forbidden(ctx)
	}
	return p.options(ctx, hc, reg)
}

// options answers an options request for the records of target, within its scope.
func (p *Panel) options(ctx http.Context, hc *HookContext, target registered) http.Response {
	tc := target.cfg
	if tc.TitleColumn == "" {
		return message(ctx, 400, fmt.Sprintf("Resource %q has no TitleColumn.", tc.Name))
	}
	values := splitList(ctx.Request().Query("values"))
	if len(values) > maxActionRecords {
		return message(ctx, 400, "Too many values.")
	}
	page := 1
	if v := ctx.Request().Query("page"); v != "" && values == nil {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return message(ctx, 400, "invalid page")
		}
		page = n
	}
	q := p.scope(target.res, tc, hc, p.opts.Orm.Query())
	out, hasMore, err := optionsFor(q, target, ctx.Request().Query("search"), values, page)
	if err != nil {
		if errors.Is(err, errOptionColumns) {
			return message(ctx, 400, "Key or title column is not a model field.")
		}
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{"data": out, "meta": http.Json{"page": page, "has_more": hasMore}})
}

var errOptionColumns = errors.New("key or title column is not a model field")

// optionsFor reads one page of options (or the given values) from q, a scoped query of
// target. Options are the key and the TitleColumn of each record, ordered by title.
func optionsFor(q orm.Query, target registered, search string, values []string, page int) ([]option, bool, error) {
	tc := target.cfg
	elem := reflect.TypeOf(target.res.newModel()).Elem()
	fields := fieldsOf(elem)
	keyInfo, titleInfo := fields[tc.Key], fields[tc.TitleColumn]
	if keyInfo.index == nil || titleInfo.index == nil {
		return nil, false, errOptionColumns
	}
	if values != nil {
		ids := make([]any, len(values))
		for i, v := range values {
			ids[i] = v
		}
		q = q.WhereIn(tc.Key, ids)
	} else {
		if s := strings.TrimSpace(search); s != "" {
			cols := tc.Searchable
			if len(cols) == 0 {
				cols = []string{tc.TitleColumn}
			}
			like := "%" + escapeLike(s) + "%"
			conds := make([]string, len(cols))
			args := make([]any, len(cols))
			for i, col := range cols {
				conds[i] = col + " LIKE ?"
				args[i] = like
			}
			q = q.Where("("+strings.Join(conds, " OR ")+")", args...)
		}
		// One extra row tells whether there is a next page.
		q = q.OrderBy(tc.TitleColumn).Offset((page - 1) * optionsPageSize).Limit(optionsPageSize + 1)
	}

	slice := target.res.newSlice()
	if err := q.Find(slice); err != nil {
		return nil, false, err
	}
	rows := reflect.ValueOf(slice).Elem()
	count := rows.Len()
	hasMore := values == nil && count > optionsPageSize
	if hasMore {
		count = optionsPageSize
	}
	out := make([]option, count)
	for i := range count {
		row := rows.Index(i)
		if row.Kind() == reflect.Pointer {
			row = row.Elem()
		}
		out[i] = option{
			Value: row.FieldByIndex(keyInfo.index).Interface(),
			Label: fmt.Sprint(row.FieldByIndex(titleInfo.index).Interface()),
		}
	}
	return out, hasMore, nil
}

// --- many-to-many writes ------------------------------------------------------------------

type relationSync struct {
	field  string
	target relationInfo
	keys   []any
}

// takeSyncs removes to-many relations (e.g. "tags": [1, 4]) from data, since they are not
// columns. They are written after the record is saved, in the same transaction.
func (p *Panel) takeSyncs(r Resource, c Config, data map[string]any) ([]relationSync, error) {
	var syncs []relationSync
	for name := range c.Relations {
		raw, sent := data[name]
		if !sent {
			continue
		}
		rel, ok := p.relation(r, c, name)
		if !ok || !rel.many {
			continue
		}
		delete(data, name)
		var list []any
		switch v := raw.(type) {
		case nil:
		case []any:
			list = v
		default:
			return nil, fieldError{field: name, kind: invalidTypeError}
		}
		for _, k := range list {
			switch k.(type) {
			case string, float64:
			default:
				return nil, fieldError{field: name, kind: invalidTypeError}
			}
		}
		syncs = append(syncs, relationSync{field: rel.field, target: rel, keys: list})
	}
	return syncs, nil
}

// applySyncs replaces the visible subset of each to-many relation with the given keys.
// Existing links to targets outside the current target scope are retained: a partial form
// payload must not turn an inaccessible link into an unlink. Submitted keys, however, must
// resolve to a scoped and viewable target before they can be attached.
func (p *Panel) applySyncs(tx orm.Query, r Resource, model any, syncs []relationSync, hc *HookContext) error {
	for _, s := range syncs {
		link, _, err := p.relationLink(r, s.target)
		if err != nil {
			return err
		}
		if link.kind != manyToMany {
			return fmt.Errorf("mks: relation field %s: only many-to-many relations can be written from a form", s.field)
		}
		parent, err := p.columnValue(model, link.parentKey)
		if err != nil {
			return err
		}
		keys := make([]any, 0, len(s.keys))
		seenKeys := map[string]bool{}
		for _, k := range s.keys {
			// JSON numbers arrive as float64; whole numbers are bound as integers.
			if f, ok := k.(float64); ok && f == float64(int64(f)) {
				k = int64(f)
			}
			if !seenKeys[keyText(k)] {
				seenKeys[keyText(k)] = true
				keys = append(keys, k)
			}
		}

		// Read current join rows first. Join rows use link.relatedKey, while submitted
		// values use the target resource's Config.Key; these domains can differ when
		// GORM's many-to-many references are customized.
		var linked []string
		if err := tx.Table(link.joinTable).Where(link.joinParent+" = ?", parent).Pluck(link.joinRelated, &linked); err != nil {
			return err
		}

		visibleLinked := map[string]bool{}
		submittedRelated := make([]any, 0, len(keys))
		canViewTargets := allows(s.target.target.res.policy().viewAny, hc)
		if !canViewTargets && len(keys) > 0 {
			return ValidationErrors{s.field: "related records are not viewable"}
		}
		if canViewTargets {
			// Resolve submitted protocol keys using Config.Key, then convert each
			// authorized record to the actual model column used by the join table.
			if len(keys) > 0 {
				q := p.scope(s.target.target.res, s.target.target.cfg, hc, tx.Model(s.target.target.res.newModel()))
				q = q.WhereIn(s.target.target.cfg.Key, keys)
				slice := s.target.target.res.newSlice()
				if err := q.Find(slice); err != nil {
					return err
				}
				rows := reflect.ValueOf(slice).Elem()
				keyInfo := fieldsOf(s.target.elem)[s.target.target.cfg.Key]
				if keyInfo.index == nil {
					return fmt.Errorf("mks: related resource key %q is not a model field", s.target.target.cfg.Key)
				}
				for i := 0; i < rows.Len(); i++ {
					record := rows.Index(i).Addr().Interface()
					if !allowsRecord(s.target.target.res.policy().view, hc, record) {
						continue
					}
					relatedKey, err := p.columnValue(record, link.relatedKey)
					if err != nil {
						return err
					}
					submittedRelated = append(submittedRelated, relatedKey)
				}
			}

			// Check existing links in the join-key domain so inaccessible links are
			// retained without confusing them with submitted protocol keys.
			if len(linked) > 0 {
				linkedKeys := make([]any, len(linked))
				for i, k := range linked {
					linkedKeys[i] = k
				}
				q := p.scope(s.target.target.res, s.target.target.cfg, hc, tx.Model(s.target.target.res.newModel()))
				q = q.WhereIn(link.relatedKey, linkedKeys)
				slice := s.target.target.res.newSlice()
				if err := q.Find(slice); err != nil {
					return err
				}
				rows := reflect.ValueOf(slice).Elem()
				for i := 0; i < rows.Len(); i++ {
					record := rows.Index(i).Addr().Interface()
					if !allowsRecord(s.target.target.res.policy().view, hc, record) {
						continue
					}
					relatedKey, err := p.columnValue(record, link.relatedKey)
					if err != nil {
						return err
					}
					visibleLinked[keyText(relatedKey)] = true
				}
			}
		}
		if len(submittedRelated) != len(keys) {
			return ValidationErrors{s.field: "related record is not accessible"}
		}

		// Keep every currently linked key outside the visible target set, then
		// replace only the visible portion with resolved join-key values.
		desired := make([]any, 0, len(submittedRelated)+len(linked))
		for _, k := range linked {
			if !visibleLinked[keyText(k)] {
				desired = append(desired, k)
			}
		}
		desired = append(desired, submittedRelated...)
		if err := link.replace(tx, parent, desired); err != nil {
			return err
		}
	}
	return nil
}
