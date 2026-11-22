package mks

import (
	"fmt"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/http"
)

const (
	searchPerResource = 5
	searchMinLength   = 2
)

type searchResult struct {
	Resource string         `json:"resource"`
	Key      any            `json:"key"`
	Title    string         `json:"title"`
	Details  map[string]any `json:"details,omitempty"`
}

// search answers GET /search?q=...: a few matches from every resource with GlobalSearch,
// each searched in its Searchable columns, within its policy (ViewAny and Scope).
func (p *Panel) search(ctx http.Context) http.Response {
	q := strings.TrimSpace(ctx.Request().Query("q"))
	if utf8.RuneCountInString(q) < searchMinLength {
		return ctx.Response().Json(200, http.Json{"data": []searchResult{}})
	}
	if utf8.RuneCountInString(q) > 100 {
		return message(ctx, 400, "Search text is too long.")
	}
	hc := p.hookContext(ctx)
	like := "%" + escapeLike(q) + "%"
	results := []searchResult{}
	for _, name := range p.names() {
		reg, _ := p.lookup(name)
		c := reg.cfg
		pol := reg.res.policy()
		if !c.GlobalSearch || !allows(pol.viewAny, hc) {
			continue
		}
		conds := make([]string, len(c.Searchable))
		args := make([]any, len(c.Searchable))
		for i, col := range c.Searchable {
			conds[i] = col + " LIKE ?"
			args[i] = like
		}
		slice := reg.res.newSlice()
		query := p.scope(reg.res, c, hc, p.opts.Orm.Query()).Where("("+strings.Join(conds, " OR ")+")", args...).Limit(searchPerResource)
		if err := query.Find(slice); err != nil {
			return p.serverError(ctx, err)
		}
		rows := reflect.ValueOf(slice).Elem()
		for i := range rows.Len() {
			record := rows.Index(i).Addr().Interface()
			if !allowsRecord(pol.view, hc, record) {
				continue
			}
			encoded, _ := encode(rows.Index(i), nil).(map[string]any)
			r := searchResult{Resource: name, Key: encoded[c.Key]}
			if t, ok := encoded[c.TitleColumn]; ok && t != nil {
				r.Title = toString(t)
			}
			if len(c.SearchDetails) > 0 {
				r.Details = map[string]any{}
				for _, col := range c.SearchDetails {
					r.Details[col] = encoded[col]
				}
			}
			results = append(results, r)
		}
	}
	return ctx.Response().Json(200, http.Json{"data": results})
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprint(v)
}

// hasGlobalSearch: whether the user can search anything, so the UI shows the search box.
func (p *Panel) hasGlobalSearch(hc *HookContext) bool {
	for _, name := range p.names() {
		reg, _ := p.lookup(name)
		if reg.cfg.GlobalSearch && allows(reg.res.policy().viewAny, hc) {
			return true
		}
	}
	return false
}
