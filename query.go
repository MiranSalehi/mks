package mks

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/goravel/framework/contracts/database/orm"
)

const (
	defaultPerPage = 25
	maxPerPage     = 100
)

var operators = map[string]string{
	"eq":   "=",
	"in":   "IN",
	"gt":   ">",
	"gte":  ">=",
	"lt":   "<",
	"lte":  "<=",
	"null": "IS NULL", // value true: IS NULL, false: IS NOT NULL
}

// maxFilterValues limits the values of one "in" filter.
const maxFilterValues = 100

// listSchema is what parseList needs to know beyond the Config: the protocol type of
// each filterable column, and the resource's custom filters.
type listSchema struct {
	types  map[string]string // column -> "string", "integer", "number", "boolean", "datetime"
	custom map[string]Filter
}

func listSchemaFor(r Resource) listSchema {
	s := listSchema{types: map[string]string{}, custom: map[string]Filter{}}
	for name, info := range fieldsOf(reflectModel(r)) {
		if fm, ok := describeType(info.typ); ok {
			s.types[name] = fm.Type
		}
	}
	for _, f := range r.filters() {
		s.custom[f.Name] = f
	}
	return s
}

type listParams struct {
	Page    int
	PerPage int
	Sort    []sortField
	Search  string
	Filters []filter
	Custom  []customFilter
	Include []string
	// Summaries are aggregates over all filtered records, e.g. views.sum.
	Summaries []summary
}

type summary struct {
	Column string
	Fn     string // "sum", "avg", "min", "max"
}

type customFilter struct {
	def    Filter
	values []string
}

type sortField struct {
	Column string
	Desc   bool
}

type filter struct {
	Column string
	Op     string
	Values []any // typed by the column: string, int64, float64, bool or time.Time
}

// badRequest is returned when the request asks for something outside the whitelist.
type badRequest struct{ msg string }

func (e badRequest) Error() string { return e.msg }

func badf(format string, args ...any) error { return badRequest{fmt.Sprintf(format, args...)} }

// parseList reads list parameters from the raw query string and checks them against the
// resource whitelist. Anything not allowed is an error, never silently ignored.
func parseList(c Config, q url.Values, s listSchema) (listParams, error) {
	p := listParams{Page: 1, PerPage: defaultPerPage}

	if v := q.Get("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return p, badf("invalid page %q", v)
		}
		p.Page = n
	}
	if v := q.Get("per_page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPerPage {
			return p, badf("per_page must be between 1 and %d", maxPerPage)
		}
		p.PerPage = n
	}

	sort := q.Get("sort")
	if sort == "" {
		sort = c.DefaultSort
	}
	for _, s := range splitList(sort) {
		f := sortField{Column: strings.TrimPrefix(s, "-"), Desc: strings.HasPrefix(s, "-")}
		if !contains(c.Sortable, f.Column) && (c.Reorderable == "" || f.Column != c.Reorderable) {
			return p, badf("sorting by %q is not allowed", f.Column)
		}
		p.Sort = append(p.Sort, f)
	}

	if s := strings.TrimSpace(q.Get("search")); s != "" {
		if len(c.Searchable) == 0 {
			return p, badf("resource %q is not searchable", c.Name)
		}
		p.Search = s
	}

	for key, vals := range q {
		col, op, ok := parseFilterKey(key)
		if !ok {
			continue
		}
		values := splitList(vals[len(vals)-1])
		if len(values) == 0 {
			return p, badf("filter %s[%s] has no value", col, op)
		}
		if len(values) > maxFilterValues {
			return p, badf("filter %s[%s] has too many values", col, op)
		}
		// A custom filter gets its values as they are; "in" and "eq" mean the same to it.
		if def, ok := s.custom[col]; ok {
			if op != "eq" && op != "in" {
				return p, badf("filter %s[%s] is not allowed", col, op)
			}
			p.Custom = append(p.Custom, customFilter{def: def, values: values})
			continue
		}
		allowed, exists := c.Filterable[col]
		if !exists || !contains(allowed, op) {
			return p, badf("filter %s[%s] is not allowed", col, op)
		}
		if op != "in" && len(values) != 1 {
			return p, badf("filter %s[%s] takes one value", col, op)
		}
		typed := make([]any, len(values))
		for i, v := range values {
			typ := s.types[col]
			if op == "null" {
				typ = "boolean"
			}
			t, err := filterValue(typ, v)
			if err != nil {
				return p, badf("filter %s[%s]: %v", col, op, err)
			}
			typed[i] = t
		}
		p.Filters = append(p.Filters, filter{Column: col, Op: op, Values: typed})
	}

	for _, item := range splitList(q.Get("summarize")) {
		col, fn, _ := strings.Cut(item, ".")
		if !contains(c.Summaries[col], fn) {
			return p, badf("summary %q is not allowed", item)
		}
		p.Summaries = append(p.Summaries, summary{Column: col, Fn: fn})
	}

	for _, rel := range splitList(q.Get("include")) {
		if _, ok := c.Relations[rel]; !ok {
			return p, badf("include %q is not allowed", rel)
		}
		p.Include = append(p.Include, rel)
	}

	return p, nil
}

// parseFilterKey turns "filter[status][in]" into ("status", "in"). "filter[status]" means "eq".
func parseFilterKey(key string) (col, op string, ok bool) {
	rest, found := strings.CutPrefix(key, "filter[")
	if !found {
		return "", "", false
	}
	col, rest, found = strings.Cut(rest, "]")
	if !found {
		return "", "", false
	}
	if rest == "" {
		return col, "eq", true
	}
	if !strings.HasPrefix(rest, "[") || !strings.HasSuffix(rest, "]") {
		return "", "", false
	}
	return col, rest[1 : len(rest)-1], true
}

// filterValue converts a filter value to the column's type, so that "true" is compared
// as a boolean and "2026-11-19T20:30:00Z" as a time. Unknown types stay strings.
func filterValue(typ, v string) (any, error) {
	switch typ {
	case "boolean":
		b, err := strconv.ParseBool(v)
		if err != nil {
			return nil, fmt.Errorf("%q is not a boolean", v)
		}
		return b, nil
	case "integer":
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not an integer", v)
		}
		return n, nil
	case "number":
		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return nil, fmt.Errorf("%q is not a number", v)
		}
		return f, nil
	case "datetime":
		return parseInputTime(v)
	}
	return v, nil
}

func splitList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// apply builds the ORM query: filtered, then ordered.
func (p listParams) apply(c Config, q orm.Query, hc *HookContext) (orm.Query, error) {
	q, err := p.filtered(c, q, hc)
	if err != nil {
		return nil, err
	}
	return p.ordered(c, q), nil
}

// filtered applies search and filters. Column names come only from the validated
// whitelist, values are always bound as parameters. An error comes from a custom filter
// rejecting its value.
func (p listParams) filtered(c Config, q orm.Query, hc *HookContext) (orm.Query, error) {
	if p.Search != "" {
		like := "%" + escapeLike(p.Search) + "%"
		conds := make([]string, len(c.Searchable))
		args := make([]any, len(c.Searchable))
		for i, col := range c.Searchable {
			conds[i] = col + " LIKE ?"
			args[i] = like
		}
		q = q.Where("("+strings.Join(conds, " OR ")+")", args...)
	}
	for _, f := range p.Filters {
		switch f.Op {
		case "in":
			q = q.WhereIn(f.Column, f.Values)
		case "null":
			if f.Values[0].(bool) {
				q = q.WhereNull(f.Column)
			} else {
				q = q.WhereNotNull(f.Column)
			}
		default:
			q = q.Where(f.Column+" "+operators[f.Op]+" ?", f.Values[0])
		}
	}
	for _, cf := range p.Custom {
		var err error
		if q, err = cf.def.Apply(hc, q, cf.values); err != nil {
			var bad badRequest
			if errors.As(err, &bad) {
				return nil, err
			}
			return nil, badRequest{fmt.Sprintf("filter %s: %v", cf.def.Name, err)}
		}
	}
	return q, nil
}

// ordered adds sorting. Includes are preloaded separately so their queries can apply
// the related resource's tenant and policy scopes.
func (p listParams) ordered(_ Config, q orm.Query) orm.Query {
	for _, s := range p.Sort {
		if s.Desc {
			q = q.OrderByDesc(s.Column)
		} else {
			q = q.OrderBy(s.Column)
		}
	}
	return q
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
