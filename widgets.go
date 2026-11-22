package mks

import (
	"fmt"
	"slices"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/http"
)

// Widget supplies the numbers for a dashboard widget: GET /widgets/{name}. How they are
// shown (stats cards, a chart) is decided by the frontend definition with the same name.
//
//	mks.Widget{Name: "overview", Data: func(ctx *mks.WidgetContext) (any, error) {
//	    total, err := ctx.Query.Model(&models.Post{}).Count()
//	    return mks.Stats{"posts": {Value: total}}, err
//	}}
type Widget struct {
	Name string
	// Authorize decides who sees the widget; unset means every logged-in user.
	Authorize func(ctx *HookContext) bool
	Data      func(ctx *WidgetContext) (any, error)
}

// WidgetContext is the hook context plus a query to read with.
type WidgetContext struct {
	*HookContext
	Query orm.Query
	// Now is the time the request started; use it for "last 30 days" and the like.
	Now time.Time
}

// Stats is the data of a stats widget: one entry per stat key of the frontend definition.
type Stats map[string]Stat

type Stat struct {
	Value any `json:"value"`
	// Change compared with an earlier period, as a fraction: 0.12 is "12% more".
	Change *float64 `json:"change,omitempty"`
	// Trend is a small series for a sparkline, oldest first.
	Trend []float64 `json:"trend,omitempty"`
}

// Chart is the data of a chart widget. Dataset keys match the frontend definition.
type Chart struct {
	Labels   []string  `json:"labels"`
	Datasets []Dataset `json:"datasets"`
}

type Dataset struct {
	Key  string    `json:"key"`
	Data []float64 `json:"data"`
}

// Widgets registers dashboard widgets. It panics on invalid configuration.
func (p *Panel) Widgets(widgets ...Widget) *Panel {
	for _, w := range widgets {
		switch {
		case !identifier.MatchString(w.Name):
			panic(fmt.Sprintf("mks: invalid widget name %q", w.Name))
		case w.Data == nil:
			panic(fmt.Sprintf("mks: widget %q has no Data", w.Name))
		case slices.ContainsFunc(p.widgets, func(x Widget) bool { return x.Name == w.Name }):
			panic(fmt.Sprintf("mks: widget %q registered twice", w.Name))
		}
		p.widgets = append(p.widgets, w)
	}
	return p
}

// visibleWidgets are the names of the widgets the user may see, in registration order.
func (p *Panel) visibleWidgets(hc *HookContext) []string {
	names := []string{}
	for _, w := range p.widgets {
		if allows(w.Authorize, hc) && p.Allowed(hc, "widget."+w.Name) {
			names = append(names, w.Name)
		}
	}
	return names
}

func (p *Panel) widget(ctx http.Context) http.Response {
	name := ctx.Request().Route("widget")
	i := slices.IndexFunc(p.widgets, func(w Widget) bool { return w.Name == name })
	if i < 0 {
		return message(ctx, 404, "Widget not found.")
	}
	w := p.widgets[i]
	hc := p.hookContext(ctx)
	if !allows(w.Authorize, hc) || !p.Allowed(hc, "widget."+w.Name) {
		return p.forbidden(ctx)
	}
	data, err := w.Data(&WidgetContext{HookContext: hc, Query: p.opts.Orm.Query(), Now: time.Now()})
	if err != nil {
		return p.serverError(ctx, err)
	}
	return ctx.Response().Json(200, http.Json{"data": data})
}

// --- helpers for time series --------------------------------------------------------------

// Period is the size of one point of a time series.
type Period string

const (
	Day   Period = "day"
	Month Period = "month"
)

// CountPer counts the rows of q (on model) per day or month of a date column, for the
// last n periods up to now, oldest first; empty periods are 0. Labels are ISO dates
// ("2026-11-19" or "2026-11"), in UTC; the frontend formats them.
//
// Rows are grouped by day in SQL (DATE() works in MySQL, PostgreSQL and SQLite); months
// are summed from the days, which keeps the query portable.
func CountPer(q orm.Query, model any, column string, period Period, n int, now time.Time) ([]string, []float64, error) {
	if !identifier.MatchString(column) {
		return nil, nil, fmt.Errorf("mks: invalid column %q", column)
	}
	if n < 1 {
		return nil, nil, fmt.Errorf("mks: CountPer needs at least one period")
	}
	now = now.UTC()
	labels, start := periodLabels(period, n, now)

	var rows []struct {
		Day   string
		Count float64
	}
	err := q.Model(model).
		Select("DATE("+column+") AS day", "COUNT(*) AS count").
		Where(column+" >= ?", start).
		Where(column+" <= ?", now).
		Group("day").
		Scan(&rows)
	if err != nil {
		return nil, nil, err
	}

	index := make(map[string]int, len(labels))
	for i, l := range labels {
		index[l] = i
	}
	values := make([]float64, len(labels))
	for _, r := range rows {
		// Drivers return DATE as "2026-11-19" or as a time ("2026-11-19T00:00:00Z").
		if len(r.Day) < 10 {
			continue
		}
		key := r.Day[:10]
		if period == Month {
			key = key[:7]
		}
		if i, ok := index[key]; ok {
			values[i] += r.Count
		}
	}
	return labels, values, nil
}

// periodLabels are the labels of the last n periods ending with the one holding now,
// and the start of the first.
func periodLabels(period Period, n int, now time.Time) ([]string, time.Time) {
	labels := make([]string, n)
	var start time.Time
	switch period {
	case Month:
		first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
		start = first.AddDate(0, -(n - 1), 0)
		for i := range n {
			labels[i] = start.AddDate(0, i, 0).Format("2006-01")
		}
	default:
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		start = today.AddDate(0, 0, -(n - 1))
		for i := range n {
			labels[i] = start.AddDate(0, 0, i).Format("2006-01-02")
		}
	}
	return labels, start
}

// Change is (current - previous) / previous, or nil when there is nothing to compare with.
func Change(current, previous float64) *float64 {
	if previous == 0 {
		return nil
	}
	c := (current - previous) / previous
	return &c
}
