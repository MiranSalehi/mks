package mks

import (
	"io/fs"
	"mime"
	"path"
	"strings"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"
)

// uiRoutes serves the built frontend: real files as-is, every other path under the panel
// gets index.html so client-side routes (e.g. /admin/posts/create) survive a reload.
//
// Gin does not allow a catch-all next to /admin/api, so deep links are served from the
// router fallback. It only answers paths under the panel; everything else stays a 404.
//
// A router has one fallback, so an application that installs its own replaces this one.
// Such an app sets Options.NoFallback and calls ServeUI from its fallback; otherwise the
// panel's deep links 404.
func (p *Panel) uiRoutes(r route.Route) {
	r.Get(p.opts.Path, p.serveUI)
	r.Get(p.opts.Path+"/assets/*path", p.serveUI)
	if p.opts.NoFallback {
		return
	}
	r.Fallback(func(ctx http.Context) http.Response {
		if resp, ok := p.ServeUI(ctx); ok {
			return resp
		}
		return ctx.Response().Data(404, "text/plain; charset=utf-8", []byte("Not found"))
	})
}

// ServeUI answers the request if it is a GET for a page under the panel path.
func (p *Panel) ServeUI(ctx http.Context) (http.Response, bool) {
	reqPath := ctx.Request().Path()
	if p.opts.UI == nil || ctx.Request().Method() != http.MethodGet ||
		(reqPath != p.opts.Path && !strings.HasPrefix(reqPath, p.opts.Path+"/")) {
		return nil, false
	}
	return p.serveUI(ctx), true
}

func (p *Panel) serveUI(ctx http.Context) http.Response {
	rel := strings.TrimPrefix(strings.TrimPrefix(ctx.Request().Path(), p.opts.Path), "/")
	if rel == "api" || strings.HasPrefix(rel, "api/") {
		return message(ctx, 404, "Not found.")
	}
	setSecurityHeaders(ctx)

	if rel != "" {
		clean := path.Clean(rel)
		if fs.ValidPath(clean) {
			if data, err := fs.ReadFile(p.opts.UI, clean); err == nil {
				// Vite puts content-hashed files in assets/, so they can be cached forever.
				if strings.HasPrefix(clean, "assets/") {
					ctx.Response().Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				ctype := mime.TypeByExtension(path.Ext(clean))
				if ctype == "" {
					ctype = "application/octet-stream"
				}
				return ctx.Response().Data(200, ctype, data)
			}
		}
		// Missing asset files must 404, not return index.html.
		if path.Ext(clean) != "" {
			return ctx.Response().Data(404, "text/plain; charset=utf-8", []byte("Not found"))
		}
	}

	index, err := fs.ReadFile(p.opts.UI, "index.html")
	if err != nil {
		return ctx.Response().Data(503, "text/plain; charset=utf-8",
			[]byte("The panel UI is not built. Run the frontend build (see README)."))
	}
	ctx.Response().Header("Cache-Control", "no-cache")
	return ctx.Response().Data(200, "text/html; charset=utf-8", index)
}

func setSecurityHeaders(ctx http.Context) {
	res := ctx.Response()
	res.Header("X-Content-Type-Options", "nosniff")
	res.Header("X-Frame-Options", "DENY")
	res.Header("Referrer-Policy", "same-origin")
}
