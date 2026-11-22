package mks

import (
	"fmt"
	"reflect"

	"github.com/goravel/framework/contracts/database/orm"
)

// Filter is a filter the server defines in code, for conditions that are not one column
// compared to a value: "my posts", "has a tag", a subquery. The request names it like a
// column, filter[mine]=1 or filter[tag][in]=1,4, and Apply receives the values.
//
//	mks.Filter{Name: "mine", Apply: func(ctx *mks.HookContext, q orm.Query, _ []string) (orm.Query, error) {
//	    return q.Where("author_id = ?", ctx.UserID()), nil
//	}}
//
// Values come from the user: bind them as parameters, never build SQL from them. Return
// an error to reject a value (400).
type Filter struct {
	Name  string
	Apply func(ctx *HookContext, q orm.Query, values []string) (orm.Query, error)
}

// Filters adds custom filters to the resource.
func (r ModelResource[T]) Filters(filters ...Filter) ModelResource[T] {
	r.flts = append(append([]Filter{}, r.flts...), filters...)
	return r
}

func (r ModelResource[T]) filters() []Filter { return r.flts }

func reflectModel(r Resource) reflect.Type { return reflect.TypeOf(r.newModel()).Elem() }

// validateFilters checks filterable columns against the model and the custom filters
// against each other.
func validateFilters(r Resource, c Config) error {
	fields := fieldsOf(reflectModel(r))
	for col, ops := range c.Filterable {
		info, ok := fields[col]
		if !ok {
			return fmt.Errorf("mks: resource %q: filterable column %q is not a model field", c.Name, col)
		}
		fm, ok := describeType(info.typ)
		if !ok || fm.Type == "json" {
			return fmt.Errorf("mks: resource %q: column %q cannot be filtered", c.Name, col)
		}
		if contains(ops, "null") && !fm.Nullable {
			return fmt.Errorf("mks: resource %q: the null filter needs a nullable column, %q is not", c.Name, col)
		}
	}
	seen := map[string]bool{}
	for _, f := range r.filters() {
		switch {
		case !identifier.MatchString(f.Name):
			return fmt.Errorf("mks: resource %q: invalid filter name %q", c.Name, f.Name)
		case f.Apply == nil:
			return fmt.Errorf("mks: resource %q: filter %q has no Apply", c.Name, f.Name)
		case seen[f.Name]:
			return fmt.Errorf("mks: resource %q: filter %q defined twice", c.Name, f.Name)
		}
		if _, clash := c.Filterable[f.Name]; clash {
			return fmt.Errorf("mks: resource %q: filter %q is also a filterable column", c.Name, f.Name)
		}
		seen[f.Name] = true
	}
	return nil
}
