package mks

import (
	"fmt"
	"slices"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

// Page is the server side of a custom page: who may open it, the data it shows and what
// its form does. The page itself is a React component registered with the same name.
//
//	p.Pages(mks.Page{
//	    Name:      "authors",
//	    Authorize: func(ctx *mks.HookContext) bool { return isAdmin(ctx) },
//	    Data:      func(ctx *mks.WidgetContext) (any, error) { ... },
//	    Rules:     map[string]string{"title": "required|max_len:255"},
//	    Handle:    func(ctx *mks.PageContext) (*mks.ActionResult, error) { ... },
//	})
type Page struct {
	Name string
	// Authorize decides who sees the page; unset means every logged-in user.
	Authorize func(ctx *HookContext) bool
	// Data answers GET /pages/{name}. Optional.
	Data func(ctx *WidgetContext) (any, error)
	// Rules validate the data of POST /pages/{name}.
	Rules map[string]string
	// Handle answers POST /pages/{name} with {"data": {...}}, inside a transaction
	// (ctx.Query). Errors work as in actions: ValidationErrors and Reject are a 422.
	Handle func(ctx *PageContext) (*ActionResult, error)
	// NavigationBadge is a count next to the page in the sidebar (zero shows nothing),
	// e.g. the problems a health page found. It runs on every meta request: keep it cheap.
	NavigationBadge func(ctx *HookContext) int64
	// NavigationBadgeColor: "primary" (default), "success", "warning", "danger", "gray", "info".
	NavigationBadgeColor string
}

// PageContext is what a page's Handle receives.
type PageContext struct {
	*HookContext
	// Data is the validated form data.
	Data map[string]any
}

// Pages registers custom pages. It panics on invalid configuration.
func (p *Panel) Pages(pages ...Page) *Panel {
	for _, pg := range pages {
		switch {
		case !identifier.MatchString(pg.Name):
			panic(fmt.Sprintf("mks: invalid page name %q", pg.Name))
		case slices.ContainsFunc(p.pages, func(x Page) bool { return x.Name == pg.Name }):
			panic(fmt.Sprintf("mks: page %q registered twice", pg.Name))
		case p.hasResource(pg.Name):
			// Pages and resources share the first URL segment.
			panic(fmt.Sprintf("mks: page %q has the name of a resource", pg.Name))
		case len(pg.Rules) > 0 && pg.Handle == nil:
			panic(fmt.Sprintf("mks: page %q has Rules but no Handle", pg.Name))
		}
		p.pages = append(p.pages, pg)
	}
	return p
}

// pageBadges are the badges of the pages the user may open.
func (p *Panel) pageBadges(hc *HookContext) map[string]navBadge {
	out := map[string]navBadge{}
	for _, pg := range p.pages {
		if pg.NavigationBadge == nil || !allows(pg.Authorize, hc) || !p.Allowed(hc, "page."+pg.Name) {
			continue
		}
		out[pg.Name] = navBadge{Count: pg.NavigationBadge(hc), Color: pg.NavigationBadgeColor}
	}
	return out
}

func (p *Panel) visiblePages(hc *HookContext) []string {
	names := []string{}
	for _, pg := range p.pages {
		if allows(pg.Authorize, hc) && p.Allowed(hc, "page."+pg.Name) {
			names = append(names, pg.Name)
		}
	}
	return names
}

// page finds the requested page and checks access.
func (p *Panel) page(ctx http.Context) (Page, *HookContext, http.Response) {
	name := ctx.Request().Route("page")
	i := slices.IndexFunc(p.pages, func(pg Page) bool { return pg.Name == name })
	if i < 0 {
		return Page{}, nil, message(ctx, 404, "Page not found.")
	}
	hc := p.hookContext(ctx)
	if !allows(p.pages[i].Authorize, hc) || !p.Allowed(hc, "page."+p.pages[i].Name) {
		return Page{}, nil, p.forbidden(ctx)
	}
	return p.pages[i], hc, nil
}

func (p *Panel) pageData(ctx http.Context) http.Response {
	pg, hc, resp := p.page(ctx)
	if resp != nil {
		return resp
	}
	if pg.Data == nil {
		return ctx.Response().Json(200, http.Json{"data": nil})
	}
	data, err := pg.Data(&WidgetContext{HookContext: hc, Query: p.opts.Orm.Query(), Now: time.Now()})
	if err != nil {
		return p.hookError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{"data": data})
}

func (p *Panel) pageSubmit(ctx http.Context) http.Response {
	pg, hc, resp := p.page(ctx)
	if resp != nil {
		return resp
	}
	if pg.Handle == nil {
		return message(ctx, 405, "This page has no form.")
	}
	data, resp := readData(ctx)
	if resp != nil {
		return resp
	}
	rules := make(map[string]any, len(pg.Rules))
	for k, v := range pg.Rules {
		rules[k] = v
	}
	if resp := p.validate(ctx, data, rules); resp != nil {
		return resp
	}
	var result *ActionResult
	err := p.transaction(hc, func(tx orm.Query) error {
		var err error
		result, err = pg.Handle(&PageContext{HookContext: hc, Data: data})
		return err
	})
	if err != nil {
		return p.hookError(ctx, err)
	}
	if result == nil {
		result = &ActionResult{}
	}
	if result.Notification == nil {
		result.Notification = &Notification{Type: "success", Title: p.tr(ctx).actionDone}
	}
	if result.Invalidate == nil {
		result.Invalidate = []string{}
	}
	return ctx.Response().Json(200, result)
}
