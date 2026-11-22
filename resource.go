package mks

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/goravel/framework/contracts/database/orm"
)

// Config describes what the server allows for a resource. Everything the UI asks for
// (sorting, searching, filtering, writing, eager loading) is checked against this whitelist.
type Config struct {
	// Name is the URL segment and protocol key, e.g. "posts".
	Name string
	// Key is the primary key column. Defaults to "id".
	Key string

	Searchable []string
	Sortable   []string
	// Filterable maps a column to its allowed operators: "eq", "in", "gt", "gte", "lt",
	// "lte" and "null" (IS NULL / IS NOT NULL, nullable columns only). Values are converted
	// to the column's type. For other conditions, see Filter.
	Filterable map[string][]string
	// Fillable lists the fields a create or update request may set.
	Fillable []string
	// Relations maps a protocol relation name to the ORM field name, e.g. "category": "Category".
	Relations map[string]string
	// IncludeTargets supplies read-only policies and JSON field allowlists for included
	// relations whose models are not registered as panel resources. Keys must also exist in
	// Relations. These targets do not create API routes.
	IncludeTargets map[string]IncludeTarget

	// DefaultSort is used when the request has no sort, e.g. "-created_at".
	DefaultSort string
	// Rules are validation rules applied on create, in Goravel syntax.
	Rules map[string]string
	// TitleColumn names a record in relation selects of other resources, e.g. "name".
	TitleColumn string
	// Uploads makes fields file uploads (the field stores the file's path). They must be fillable.
	Uploads map[string]Upload
	// HTML lists rich-text fields; their HTML is sanitized on save (scripts, event handlers
	// and other unsafe markup removed).
	HTML []string
	// Hashed lists fields whose plain value is hashed with the application's hash service
	// before insert and update, e.g. "password". Empty values are skipped, so an edit can
	// omit the field. Do not hash the same field in a Before hook when using this.
	Hashed []string
	// Deletable allows deleting records: DELETE /records/{id} and the bulk "delete" action.
	Deletable bool
	// Summaries allows aggregates over all filtered records, shown under the table:
	// a numeric column and its functions ("sum", "avg", "min", "max").
	Summaries map[string][]string
	// Reorderable is an integer column holding a manual order, e.g. "sort_order". It can
	// be sorted by, and POST /reorder rewrites it (the user needs Update on each record).
	Reorderable string
	// GlobalSearch includes the resource in the panel-wide search (Ctrl+K), searching its
	// Searchable columns and showing its TitleColumn.
	GlobalSearch bool
	// SearchDetails are extra columns shown under a search result, e.g. "status".
	SearchDetails []string
	// Tenant is the column holding the record's tenant (e.g. "team_id") when the panel
	// has Tenancy: queries see only the current tenant and new records get its key.
	// It must not be fillable.
	Tenant string
	// NavigationBadge narrows the records the user can see (tenant and policy scope are
	// already applied); their count shows next to the resource in the sidebar, e.g.
	// pending comments: q.Where("status = ?", "pending"). Zero shows nothing. The UI
	// refreshes it after the user's writes; others' changes show on the next meta load.
	NavigationBadge func(ctx *HookContext, q orm.Query) orm.Query
	// NavigationBadgeColor is the badge's color: "primary" (default), "success",
	// "warning", "danger", "gray" or "info".
	NavigationBadgeColor string
}

// IncludeTarget exposes a related model only as a read-only, field-projected include. It does
// not register CRUD or relation-manager routes for that model.
type IncludeTarget struct {
	Resource Resource
	Fields   []string // JSON field names to expose, e.g. []string{"id", "name"}.
}

// Resource is implemented by embedding ModelResource[T].
type Resource interface {
	Config() Config
	newModel() any
	newSlice() any
	hooks() hooks
	actions() []actionDef
	policy() policyDef
	filters() []Filter
}

// ModelResource binds a Config to an ORM model type.
type ModelResource[T any] struct {
	C    Config
	h    hooks
	acts []actionDef
	pol  policyDef
	flts []Filter
}

func (r ModelResource[T]) Config() Config { return r.C }
func (r ModelResource[T]) newModel() any  { return new(T) }
func (r ModelResource[T]) newSlice() any  { return &[]T{} }

// Define creates a resource for model T.
func Define[T any](c Config) ModelResource[T] {
	return ModelResource[T]{C: c}
}

var identifier = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

// validate guards against misconfiguration: whitelisted names end up in SQL as identifiers,
// so they must be plain column names.
func (c *Config) validate() error {
	if !identifier.MatchString(c.Name) {
		return fmt.Errorf("mks: invalid resource name %q", c.Name)
	}
	if c.Key == "" {
		c.Key = "id"
	}
	cols := append([]string{c.Key}, c.Searchable...)
	cols = append(cols, c.HTML...)
	cols = append(cols, c.Hashed...)
	for field := range c.Uploads {
		cols = append(cols, field)
		if !contains(c.Fillable, field) {
			return fmt.Errorf("mks: resource %q: upload field %q must be fillable", c.Name, field)
		}
	}
	if c.TitleColumn != "" {
		cols = append(cols, c.TitleColumn)
	}
	cols = append(cols, c.Sortable...)
	for col := range c.Summaries {
		cols = append(cols, col)
	}
	if c.Reorderable != "" {
		cols = append(cols, c.Reorderable)
	}
	cols = append(cols, c.SearchDetails...)
	if c.NavigationBadgeColor != "" && !slices.Contains([]string{"primary", "success", "warning", "danger", "gray", "info"}, c.NavigationBadgeColor) {
		return fmt.Errorf("mks: resource %q: unknown NavigationBadgeColor %q", c.Name, c.NavigationBadgeColor)
	}
	if c.GlobalSearch && (len(c.Searchable) == 0 || c.TitleColumn == "") {
		return fmt.Errorf("mks: resource %q: GlobalSearch needs Searchable columns and a TitleColumn", c.Name)
	}
	cols = append(cols, c.Fillable...)
	for col := range c.Filterable {
		cols = append(cols, col)
	}
	for _, col := range cols {
		if !identifier.MatchString(col) {
			return fmt.Errorf("mks: resource %q: invalid column name %q", c.Name, col)
		}
	}
	for _, s := range splitList(c.DefaultSort) {
		if col := strings.TrimPrefix(s, "-"); !contains(c.Sortable, col) && col != c.Reorderable {
			return fmt.Errorf("mks: resource %q: default sort column %q must be sortable", c.Name, col)
		}
	}
	for col, ops := range c.Filterable {
		for _, op := range ops {
			if _, ok := operators[op]; !ok {
				return fmt.Errorf("mks: resource %q: unknown filter operator %q on %q", c.Name, op, col)
			}
		}
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
