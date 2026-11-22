package mks

import (
	"fmt"
	"slices"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

// Multi-tenancy: records belong to a tenant (a team, a company) and users work inside one
// of the tenants they belong to.
//
//	mks.Options{Tenancy: &mks.TenancyOptions{Tenants: func(ctx *mks.HookContext) ([]mks.Tenant, error) { ... }}}
//	mks.Config{Name: "posts", Tenant: "team_id", ...}
//
// The UI picks the tenant per browser tab and sends it as the X-Mks-Tenant header; the
// server checks on every request that the user belongs to it. For a resource with a
// Tenant column, every query is limited to the current tenant, new records get its key,
// and references to records of tenant-scoped resources must stay inside it.

// TenantHeader carries the tenant the UI works in.
const TenantHeader = "X-Mks-Tenant"

// Tenant is one tenant a user can work in.
type Tenant struct {
	ID   any    `json:"id"`
	Name string `json:"name"`
}

type TenancyOptions struct {
	// Tenants lists the tenants the user belongs to, e.g. from a team_user table. The
	// first one is the default.
	Tenants func(ctx *HookContext) ([]Tenant, error)
}

type tenantState struct {
	tenants []Tenant
	current *Tenant
	err     error
	// refused: the request named a tenant the user does not belong to.
	refused bool
}

type tenantKey struct{}

// Tenant is the tenant the request works in, or nil (no tenancy, or none chosen).
func (c *HookContext) Tenant() *Tenant {
	if c.tenant == nil || c.tenant.current == nil {
		return nil
	}
	return c.tenant.current
}

// TenantID is the current tenant's key, or nil. Use it to scope your own queries in
// widgets, pages and actions.
func (c *HookContext) TenantID() any {
	if t := c.Tenant(); t != nil {
		return t.ID
	}
	return nil
}

// Tenants are the tenants the user belongs to (nil without tenancy or a user).
func (c *HookContext) Tenants() []Tenant {
	if c == nil || c.tenant == nil || c.tenant.err != nil {
		return nil
	}
	return c.tenant.tenants
}

// BelongsTo reports whether the user belongs to the tenant with key id.
func (c *HookContext) BelongsTo(id any) bool {
	return id != nil && slices.ContainsFunc(c.Tenants(), func(t Tenant) bool { return fmt.Sprint(t.ID) == fmt.Sprint(id) })
}

// resolveTenant works out the request's tenant once per request.
func (p *Panel) resolveTenant(ctx http.Context, hc *HookContext) *tenantState {
	if p.opts.Tenancy == nil || hc.User == nil {
		return nil
	}
	if cached, ok := ctx.Value(tenantKey{}).(*tenantState); ok {
		return cached
	}
	st := &tenantState{}
	st.tenants, st.err = p.opts.Tenancy.Tenants(hc)
	if st.err == nil {
		want := ctx.Request().Header(TenantHeader, "")
		switch {
		case want != "":
			i := slices.IndexFunc(st.tenants, func(t Tenant) bool { return fmt.Sprint(t.ID) == want })
			if i >= 0 {
				st.current = &st.tenants[i]
			} else {
				st.refused = true
			}
		case len(st.tenants) > 0:
			st.current = &st.tenants[0]
		}
	}
	ctx.WithValue(tenantKey{}, st)
	return st
}

func (p *Panel) tenantMeta(hc *HookContext) any {
	st := hc.tenant
	if st == nil || st.err != nil {
		return nil
	}
	out := http.Json{"tenants": st.tenants, "current": nil}
	if st.current != nil {
		out["current"] = st.current.ID
	}
	if out["tenants"] == nil {
		out["tenants"] = []Tenant{}
	}
	return out
}

// scope limits q to what the user may see of resource r: its tenant, then its policy.
// Every query for records goes through here.
func (p *Panel) scope(r Resource, c Config, hc *HookContext, q orm.Query) orm.Query {
	if c.Tenant != "" && p.opts.Tenancy != nil {
		if id := hc.TenantID(); id != nil {
			q = q.Where(c.Tenant+" = ?", id)
		} else {
			// No tenant, no records.
			return q.Where("1 = 0")
		}
	}
	pol := r.policy()
	if pol.scope == nil {
		return q
	}
	// Goravel builds callback-based Where conditions on an isolated builder.
	// AND the policy predicates as a group so OrWhere stays inside the policy,
	// outside the tenant, record key, and association conditions.
	return q.Where(func(group orm.Query) orm.Query {
		return policyWhereGroup(group, func(query orm.Query) orm.Query {
			return scoped(pol, hc, query)
		})
	})
}

// tenantFill is the column a new record of r gets, with the current tenant's key.
func (p *Panel) tenantFill(c Config, hc *HookContext) (map[string]any, bool) {
	if c.Tenant == "" || p.opts.Tenancy == nil {
		return nil, true
	}
	id := hc.TenantID()
	if id == nil {
		return nil, false
	}
	return map[string]any{c.Tenant: jsonValue(id)}, true
}

// checkTenantRefs rejects references (belongs-to keys and to-many lists in data) to
// records of a tenant-scoped resource outside the current tenant. Validation rules like
// exists: cannot see tenants, so this is what keeps one team from linking another's data.
func (p *Panel) checkTenantRefs(ctx http.Context, r Resource, c Config, hc *HookContext, data map[string]any) http.Response {
	if p.opts.Tenancy == nil {
		return nil
	}
	sch, err := p.schemaOf(r.newModel())
	if err != nil {
		return p.serverError(ctx, err)
	}
	errs := map[string][]string{}
	for name, field := range c.Relations {
		rel, ok := p.relation(r, c, name)
		if !ok || rel.target.cfg.Tenant == "" {
			continue
		}
		var column string
		var keys []any
		if rel.many {
			list, sent := data[name].([]any)
			if !sent || len(list) == 0 {
				continue
			}
			column, keys = name, uniqueKeys(list)
		} else {
			gr := sch.Relationships.Relations[field]
			if gr == nil || len(gr.References) == 0 || gr.References[0].OwnPrimaryKey {
				continue // not a belongs-to
			}
			column = gr.References[0].ForeignKey.DBName
			v, sent := data[column]
			if !sent || v == nil {
				continue
			}
			keys = []any{v}
		}
		tc := rel.target.cfg
		n, err := p.scope(rel.target.res, tc, hc, p.opts.Orm.Query().Model(rel.target.res.newModel())).WhereIn(tc.Key, keys).Count()
		if err != nil {
			return p.serverError(ctx, err)
		}
		if int(n) != len(keys) {
			errs[column] = []string{p.tr(ctx).notInTenant}
		}
	}
	if len(errs) > 0 {
		return p.validationError(ctx, errs)
	}
	return nil
}

// validateTenant checks a resource's Tenant column at registration.
func validateTenant(r Resource, c Config) error {
	if c.Tenant == "" {
		return nil
	}
	if !identifier.MatchString(c.Tenant) {
		return fmt.Errorf("mks: resource %q: invalid Tenant column %q", c.Name, c.Tenant)
	}
	if _, ok := fieldsOf(reflectModel(r))[c.Tenant]; !ok {
		return fmt.Errorf("mks: resource %q: Tenant column %q is not a model field", c.Name, c.Tenant)
	}
	if contains(c.Fillable, c.Tenant) {
		return fmt.Errorf("mks: resource %q: Tenant column %q must not be fillable (the server sets it)", c.Name, c.Tenant)
	}
	return nil
}
