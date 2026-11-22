package mks

import (
	"fmt"
	"reflect"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

// preloadIncludes loads relations backed by registered resources or explicit read-only
// IncludeTargets. The ORM callback applies the target's normal tenant and policy scopes.
func (p *Panel) preloadIncludes(ctx http.Context, q orm.Query, r Resource, c Config, names []string, hc *HookContext) (orm.Query, http.Response) {
	for _, name := range names {
		if target, ok := c.IncludeTargets[name]; ok {
			field := c.Relations[name]
			if !allows(target.Resource.policy().viewAny, hc) {
				return nil, p.forbidden(ctx)
			}
			res, tc := target.Resource, target.Resource.Config()
			q = q.With(field, func(related orm.Query) orm.Query {
				return p.scope(res, tc, hc, related)
			})
			continue
		}
		rel, ok := p.relation(r, c, name)
		if !ok {
			return nil, message(ctx, 400, fmt.Sprintf("Relation %q is not available.", name))
		}
		if !allows(rel.target.res.policy().viewAny, hc) {
			return nil, p.forbidden(ctx)
		}
		target, tc := rel.target.res, rel.target.cfg
		q = q.With(rel.field, func(related orm.Query) orm.Query {
			return p.scope(target, tc, hc, related)
		})
	}
	return q, nil
}

// includeRelation resolves a relation target for include reads, including explicitly
// configured read-only targets. It intentionally does not broaden p.relation, which powers
// options and relation managers.
func (p *Panel) includeRelation(r Resource, c Config, name string) (relationInfo, IncludeTarget, bool) {
	if target, ok := c.IncludeTargets[name]; ok {
		field := c.Relations[name]
		sf, exists := reflect.TypeOf(r.newModel()).Elem().FieldByName(field)
		if !exists {
			return relationInfo{}, IncludeTarget{}, false
		}
		t, many := sf.Type, false
		if t.Kind() == reflect.Slice {
			t, many = t.Elem(), true
		}
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		return relationInfo{field: field, target: registered{res: target.Resource, cfg: target.Resource.Config()}, many: many, elem: t}, target, true
	}
	rel, ok := p.relation(r, c, name)
	return rel, IncludeTarget{}, ok
}

// checkIncludeTargets validates read-only include mappings at registration time.
func checkIncludeTargets(model any, c Config) error {
	parentType := reflect.TypeOf(model).Elem()
	if len(c.IncludeTargets) > 0 && hasJSONMarshaler(parentType) {
		return fmt.Errorf("mks: resource %q: custom JSON marshaling is not supported with IncludeTargets", c.Name)
	}
	for name, target := range c.IncludeTargets {
		field, exists := c.Relations[name]
		if !exists {
			return fmt.Errorf("mks: resource %q: include target %q is not a configured relation", c.Name, name)
		}
		if target.Resource == nil || isNilValue(reflect.ValueOf(target.Resource)) {
			return fmt.Errorf("mks: resource %q: include target %q has no resource", c.Name, name)
		}
		sf, exists := parentType.FieldByName(field)
		if !exists {
			return fmt.Errorf("mks: resource %q: include target %q points to missing field %s", c.Name, name, field)
		}
		relatedType := sf.Type
		if relatedType.Kind() == reflect.Slice {
			relatedType = relatedType.Elem()
		}
		if relatedType.Kind() == reflect.Pointer {
			relatedType = relatedType.Elem()
		}
		targetType := reflect.TypeOf(target.Resource.newModel())
		if targetType.Kind() == reflect.Pointer {
			targetType = targetType.Elem()
		}
		if hasJSONMarshaler(targetType) {
			return fmt.Errorf("mks: resource %q: include target %q uses custom JSON marshaling", c.Name, name)
		}
		if relatedType != targetType {
			return fmt.Errorf("mks: resource %q: include target %q model does not match field %s", c.Name, name, field)
		}
		tc := target.Resource.Config()
		if err := tc.validate(); err != nil {
			return fmt.Errorf("mks: resource %q: include target %q: %w", c.Name, name, err)
		}
		if err := validateTenant(target.Resource, tc); err != nil {
			return fmt.Errorf("mks: resource %q: include target %q: %w", c.Name, name, err)
		}
		available := map[string]reflect.Type{}
		for _, f := range orderedFields(targetType) {
			available[f.name] = f.typ
		}
		if len(target.Fields) == 0 {
			return fmt.Errorf("mks: resource %q: include target %q must allow at least one field", c.Name, name)
		}
		seen := map[string]bool{}
		for _, exposed := range target.Fields {
			typ, found := available[exposed]
			if !found || seen[exposed] || !safeIncludeField(typ) {
				return fmt.Errorf("mks: resource %q: include target %q has invalid exposed field %q", c.Name, name, exposed)
			}
			seen[exposed] = true
		}
	}
	return nil
}

func hasJSONMarshaler(t reflect.Type) bool {
	if t == nil {
		return false
	}
	if t.Implements(marshalerType) {
		return true
	}
	if t.Kind() == reflect.Pointer {
		return t.Elem().Implements(marshalerType)
	}
	return reflect.PointerTo(t).Implements(marshalerType)
}

func safeIncludeField(t reflect.Type) bool {
	// The serializer normalizes recognized time types before considering custom
	// JSON marshalers, so they remain safe scalar fields.
	if isTimeType(t) {
		return true
	}
	if hasJSONMarshaler(t) {
		return false
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// encodeIncluded serializes the root record and projects included records to their
// explicitly allowed JSON fields.
func (p *Panel) encodeIncluded(v reflect.Value, force map[string]bool, r Resource, c Config, names []string) any {
	encoded, ok := encode(v, force).(map[string]any)
	if !ok {
		return encode(v, force)
	}
	root := v
	for root.IsValid() && (root.Kind() == reflect.Pointer || root.Kind() == reflect.Interface) {
		if root.IsNil() {
			return encoded
		}
		root = root.Elem()
	}
	for _, name := range names {
		rel, target, ok := p.includeRelation(r, c, name)
		if !ok || target.Resource == nil {
			continue
		}
		field, ok := root.Type().FieldByName(rel.field)
		if !ok {
			continue
		}
		jsonKey, _, skip := jsonName(field)
		if skip {
			continue
		}
		allow := make(map[string]bool, len(target.Fields))
		for _, key := range target.Fields {
			allow[key] = true
		}
		encoded[jsonKey] = projectIncluded(encoded[jsonKey], allow)
	}
	return encoded
}

func projectIncluded(value any, allowed map[string]bool) any {
	switch v := value.(type) {
	case map[string]any:
		for key := range v {
			if !allowed[key] {
				delete(v, key)
			}
		}
	case []any:
		for _, item := range v {
			projectIncluded(item, allowed)
		}
	}
	return value
}

// filterIncludedRecords applies record-level View policies after the scoped preload.
// Query scopes prevent hidden records from being fetched; this second check handles
// resource policies that make per-record decisions.
func (p *Panel) filterIncludedRecords(model any, r Resource, c Config, names []string, hc *HookContext) {
	parent := reflect.ValueOf(model)
	for parent.IsValid() && (parent.Kind() == reflect.Pointer || parent.Kind() == reflect.Interface) {
		if parent.IsNil() {
			return
		}
		parent = parent.Elem()
	}
	if !parent.IsValid() || parent.Kind() != reflect.Struct {
		return
	}
	for _, name := range names {
		rel, _, ok := p.includeRelation(r, c, name)
		if !ok {
			continue
		}
		field := parent.FieldByName(rel.field)
		if !field.IsValid() || !field.CanSet() {
			continue
		}
		switch field.Kind() {
		case reflect.Slice:
			filtered := reflect.MakeSlice(field.Type(), 0, field.Len())
			for i := 0; i < field.Len(); i++ {
				item := field.Index(i)
				if relatedRecordAllowed(rel.target.res.policy().view, hc, item) {
					filtered = reflect.Append(filtered, item)
				}
			}
			field.Set(filtered)
		case reflect.Pointer, reflect.Struct, reflect.Interface:
			if !isNilValue(field) && !relatedRecordAllowed(rel.target.res.policy().view, hc, field) {
				field.Set(reflect.Zero(field.Type()))
			}
		}
	}
}

func isNilValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Slice, reflect.Func:
		return v.IsNil()
	default:
		return false
	}
}

func relatedRecordAllowed(view func(*HookContext, any) bool, hc *HookContext, record reflect.Value) bool {
	if !record.IsValid() || isNilValue(record) {
		return false
	}
	if record.Kind() != reflect.Pointer && record.Kind() != reflect.Interface {
		if !record.CanAddr() {
			return false
		}
		record = record.Addr()
	}
	return allowsRecord(view, hc, record.Interface())
}
