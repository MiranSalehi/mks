package mks

import (
	"database/sql"
	"fmt"
	"reflect"
	"slices"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

var summaryFuncs = map[string]bool{"sum": true, "avg": true, "min": true, "max": true}

// validateTable checks Summaries and Reorderable against the model.
func validateTable(r Resource, c Config) error {
	fields := fieldsOf(reflectModel(r))
	typeOf := func(col string) string {
		info, ok := fields[col]
		if !ok {
			return ""
		}
		fm, _ := describeType(info.typ)
		if fm.Nullable {
			return "nullable " + fm.Type
		}
		return fm.Type
	}
	for col, fns := range c.Summaries {
		if t := typeOf(col); t != "integer" && t != "number" && t != "nullable integer" && t != "nullable number" {
			return fmt.Errorf("mks: resource %q: summary column %q must be a numeric model field", c.Name, col)
		}
		for _, fn := range fns {
			if !summaryFuncs[fn] {
				return fmt.Errorf("mks: resource %q: unknown summary %q on %q", c.Name, fn, col)
			}
		}
	}
	if c.Reorderable != "" && typeOf(c.Reorderable) != "integer" {
		return fmt.Errorf("mks: resource %q: Reorderable column %q must be a non-null integer model field", c.Name, c.Reorderable)
	}
	return nil
}

// summaries computes the requested aggregates over q (filtered, not ordered or paged).
// The result is column -> function -> value; a value is null when there are no records.
func summaries(r Resource, q orm.Query, list []summary) (map[string]map[string]*float64, error) {
	out := map[string]map[string]*float64{}
	for _, s := range list {
		var v sql.NullFloat64
		agg := q.Model(r.newModel())
		var err error
		switch s.Fn {
		case "sum":
			err = agg.Sum(s.Column, &v)
		case "avg":
			err = agg.Avg(s.Column, &v)
		case "min":
			err = agg.Min(s.Column, &v)
		case "max":
			err = agg.Max(s.Column, &v)
		}
		if err != nil {
			return nil, err
		}
		if out[s.Column] == nil {
			out[s.Column] = map[string]*float64{}
		}
		if v.Valid {
			f := v.Float64
			out[s.Column][s.Fn] = &f
		} else {
			out[s.Column][s.Fn] = nil
		}
	}
	return out, nil
}

// reorder answers POST /resources/{resource}/reorder with {"records": [keys in the new
// order]}. The records keep the order values they already have, handed out again in the
// new order, so records that were not sent (other pages, other filters) stay where they
// are. Sending just the moved part of a list is enough.
func (p *Panel) reorder(ctx http.Context) http.Response {
	r, c, ok := p.resource(ctx)
	if !ok {
		return message(ctx, 404, p.tr(ctx).resourceNotFound)
	}
	if c.Reorderable == "" {
		return message(ctx, 404, "This resource cannot be reordered.")
	}
	hc := p.hookContext(ctx)
	pol := r.policy()
	if !allows(pol.viewAny, hc) || !allows(pol.reorder, hc) {
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
	if len(keys) != len(body.Records) {
		return message(ctx, 400, "Each record may appear once.")
	}

	slice := r.newSlice()
	if err := p.scope(r, c, hc, p.opts.Orm.Query()).WhereIn(c.Key, keys).Find(slice); err != nil {
		return p.serverError(ctx, err)
	}
	rows := reflect.ValueOf(slice).Elem()
	if rows.Len() != len(keys) {
		return message(ctx, 404, p.tr(ctx).recordNotFound)
	}
	byKey := map[string]any{}
	var values []int64
	for i := range rows.Len() {
		record := rows.Index(i).Addr().Interface()
		if !allowsRecord(pol.update, hc, record) {
			return p.forbidden(ctx)
		}
		byKey[keyString(record, c.Key)] = record
		v, err := p.columnValue(record, c.Reorderable)
		if err != nil {
			return p.serverError(ctx, err)
		}
		values = append(values, reflect.ValueOf(v).Convert(reflect.TypeFor[int64]()).Int())
	}
	slices.Sort(values)
	// Equal values (e.g. all 0 before anything was ordered) cannot carry an order:
	// count up from the smallest instead.
	if len(slices.Compact(slices.Clone(values))) != len(values) {
		for i := range values {
			values[i] = values[0] + int64(i)
		}
	}

	sch, err := p.schemaOf(r.newModel())
	if err != nil {
		return p.serverError(ctx, err)
	}
	err = p.opts.Orm.Transaction(func(tx orm.Query) error {
		for i, k := range keys {
			record := byKey[keyText(k)]
			key, err := keyValue(record, c.Key)
			if err != nil {
				return err
			}
			// Only the order column: no timestamps, no hooks.
			if _, err := tx.Exec("UPDATE "+sch.Table+" SET "+c.Reorderable+" = ? WHERE "+c.Key+" = ?", values[i], key); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{"count": len(keys)})
}
