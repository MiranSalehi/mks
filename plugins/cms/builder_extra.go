package cms

import (
	"encoding/json"
	"fmt"
	"html/template"
	"time"

	"github.com/goravel/framework/contracts/http"
	"github.com/goravel/framework/contracts/route"

	mks "github.com/MiranSalehi/mks"
)

// --- templates (MKSine's page templates) ---------------------------------------------------

// PageTemplates are ready-made block layouts a page can start from.
var PageTemplates = map[string]string{
	"landing": `[
		{"type": "hero", "data": {"title": "Page title", "subtitle": "One short line about the value you offer.", "height": "large", "overlay": "gradient", "button_text": "Get started", "button_url": "#features", "show_secondary_button": true, "secondary_button_text": "Learn more", "secondary_button_url": "#faq"}},
		{"type": "features", "data": {"anchor": "features", "heading": "Why us?", "subheading": "Three good reasons.", "columns": 3, "style": "shadowed", "features": [
			{"icon": "⚡", "title": "Fast", "description": "A short line about this advantage."},
			{"icon": "🔒", "title": "Secure", "description": "A short line about this advantage."},
			{"icon": "💬", "title": "Support", "description": "A short line about this advantage."}]}},
		{"type": "testimonial", "data": {"heading": "What our customers say", "columns": 2, "testimonials": [
			{"content": "It was a great experience.", "author_name": "Customer name", "author_title": "Role", "rating": 5},
			{"content": "I recommend it.", "author_name": "Customer name", "author_title": "Role", "rating": 5}]}},
		{"type": "accordion", "data": {"anchor": "faq", "heading": "Frequently asked questions", "items": [
			{"question": "First question?", "answer": "<p>A short answer.</p>"},
			{"question": "Second question?", "answer": "<p>A short answer.</p>"}]}},
		{"type": "cta", "data": {"title": "Ready?", "description": "Start right now.", "button_text": "Contact us", "button_url": "#"}}
	]`,
	"about": `[
		{"type": "heading", "data": {"text": "About us", "level": "h1", "alignment": "center"}},
		{"type": "text", "data": {"content": "<p>Write the story, the values and the goals of your work here.</p>"}},
		{"type": "columns", "data": {"layout": "equal", "children": [
			{"blocks": [{"type": "heading", "data": {"text": "Mission", "level": "h3"}}, {"type": "text", "data": {"content": "<p>What we do and why.</p>"}}]},
			{"blocks": [{"type": "heading", "data": {"text": "Vision", "level": "h3"}}, {"type": "text", "data": {"content": "<p>Where we are going.</p>"}}]}]}},
		{"type": "cta", "data": {"title": "Join us", "style": "simple", "button_text": "Contact", "button_url": "#"}}
	]`,
	"contact": `[
		{"type": "heading", "data": {"text": "Contact us", "level": "h1"}},
		{"type": "columns", "data": {"layout": "equal", "children": [
			{"blocks": [{"type": "card", "data": {"children": [{"blocks": [{"type": "heading", "data": {"text": "Address", "level": "h4"}}]}, {"blocks": [{"type": "text", "data": {"content": "<p>Write the office address.</p>"}}]}, {"blocks": []}]}}]},
			{"blocks": [{"type": "card", "data": {"children": [{"blocks": [{"type": "heading", "data": {"text": "Phone and email", "level": "h4"}}]}, {"blocks": [{"type": "button", "data": {"text": "Send email", "url": "mailto:info@example.com", "style": "outline"}}]}, {"blocks": []}]}}]}]}}
	]`,
	"services": `[
		{"type": "hero", "data": {"title": "Our services", "height": "small", "overlay": "dark"}},
		{"type": "features", "data": {"columns": 3, "style": "bordered", "icon_style": "square", "features": [
			{"icon": "🛠", "title": "First service", "description": "Service description."},
			{"icon": "📈", "title": "Second service", "description": "Service description."},
			{"icon": "🎯", "title": "Third service", "description": "Service description."}]}},
		{"type": "tabs", "data": {"style": "pills", "tabs": [
			{"title": "How we work", "content": "<p>Steps of the collaboration.</p>"},
			{"title": "Pricing", "content": "<p>Plans and prices.</p>"}]}},
		{"type": "cta", "data": {"title": "Have a project in mind?", "button_text": "Request a consultation", "button_url": "#"}}
	]`,
	"blank": `[]`,
	// MKSine's default home (its theme blocks; the pricing block came from a plugin).
	"mksine_home": `[
		{"type": "mksine_hero_domain", "data": {"heading_line1": "Your domain.", "heading_line2_prefix": "Your brand.", "heading_accent": "Online in minutes.", "subheading": "A memorable domain, secure email and SSL; everything for a confident start.", "cta_label": "Search domains", "cta_url": "#domains", "illustration_position": "right", "background_color": "#efeaff"}},
		{"type": "mksine_finance_showcase", "data": {"heading": "Business accounting, without the pain.", "heading_accent": "without the pain.", "paragraph_1": "A clear dashboard for everything", "paragraph_1_highlight": "clear dashboard", "paragraph_2": "Automate reminders and get paid sooner with smart follow-ups.", "paragraph_2_highlight": "smart follow-ups", "cta_primary_label": "Start for free", "cta_primary_url": "#", "cta_secondary_label": "Talk to sales", "cta_secondary_url": "#", "trust_line": "No bank card is needed to try the demo.", "social_caption": "Trusted by today's businesses", "social_count": "+12,400 teams", "mock_chart_caption": "Monthly revenue", "avatars": [{"letter": "A"}, {"letter": "B"}, {"letter": "C"}, {"letter": "D"}, {"letter": "E"}]}},
		{"type": "mksine_services_trio", "data": {"section_title": "Everything you need to get online", "cards": [{"title": "Fast hosting", "body": "SSD storage, daily backups and a control panel out of your way.", "cta_label": "See plans", "url": "#"}, {"title": "Professional email", "body": "Mailboxes under your brand, protected from spam.", "cta_label": "Get email", "url": "#"}, {"title": "Free SSL", "body": "Encrypt visitor traffic and build trust.", "cta_label": "Learn more", "url": "#"}]}},
		{"type": "mksine_featured_domains", "data": {"anchor": "domains", "title": "Featured extensions", "view_all_label": "All domains", "view_all_url": "#", "items": [{"tld": ".ir", "category": "Iran", "price": "180,000 tomans", "href": "#"}, {"tld": ".com", "category": "Global and classic", "price": "1,200,000 tomans", "href": "#"}, {"tld": ".net", "category": "Networks", "price": "950,000 tomans", "original_price": "1,400,000 tomans", "discount_percent": 32, "href": "#"}, {"tld": ".online", "category": "Bold and modern", "price": "250,000 tomans", "original_price": "2,500,000 tomans", "discount_percent": 90, "href": "#"}]}},
		{"type": "mksine_clinic_features_grid", "data": {"badge": "Why us?", "heading_prefix": "Powerful tools for ", "heading_accent": "today's businesses", "subheading": "Booking, records and customer communication in one place.", "cards": [{"gradient": "from-blue-500 to-blue-600", "icon": "users", "title": "Tidy records", "body": "History and attachments at a glance."}, {"gradient": "from-emerald-500 to-emerald-600", "icon": "calendar", "title": "Smart booking", "body": "Reminders and online booking cut no-shows."}, {"gradient": "from-violet-500 to-violet-600", "icon": "chart-column", "title": "Reports", "body": "A clear view of revenue and visits."}, {"gradient": "from-yellow-500 to-yellow-600", "icon": "clipboard-list", "title": "Inventory", "body": "Track supplies with low-stock alerts."}, {"gradient": "from-purple-500 to-purple-600", "icon": "credit-card", "title": "Invoices", "body": "Invoices, receipts and payment follow-up."}, {"gradient": "from-pink-500 to-pink-600", "icon": "settings", "title": "Automation", "body": "Rules that save hours every week."}]}},
		{"type": "mksine_testimonials_grid", "data": {"badge": "Customer experience", "title_prefix": "Loved by teams ", "title_accent": "everywhere", "subheading": "Real feedback from businesses that moved last season.", "aside": "Average response time under 2 hours; we are there whenever you need us.", "items": [{"name": "Sara Lotfi", "city": "Tehran", "quote": "Admin work is half what it was; booking and invoicing finally talk to each other.", "gradient": "from-violet-500 to-purple-600"}, {"name": "Mehdi Rezaei", "city": "Isfahan", "quote": "Reminders are clearer and more slots get filled.", "gradient": "from-indigo-500 to-blue-600"}, {"name": "Elham Rostami", "city": "Shiraz", "quote": "Reporting that used to take a weekend is now one click.", "gradient": "from-fuchsia-500 to-pink-600"}]}}
	]`,
}

// applyTemplate replaces a page's blocks with a template (the page becomes block-based).
// The previous content stays in the revision history.
func (pl *Plugin) applyTemplate(ctx *mks.ActionContext[Page]) (*mks.ActionResult, error) {
	if ctx == nil || !pl.can(ctx.HookContext, ManagePages) {
		return nil, mks.Reject("You are not allowed to change pages.")
	}
	if ctx.Query == nil || len(ctx.Records) != 1 || ctx.Records[0] == nil {
		return nil, mks.Reject("Select exactly one page.")
	}
	page := ctx.Records[0]
	name, _ := ctx.Data["template"].(string)
	raw, ok := PageTemplates[name]
	if !ok {
		return nil, mks.ValidationErrors{"template": "The selected theme does not exist."}
	}
	var payload any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("template %s: %w", name, err)
	}
	blocks, errs := pl.NormalizeBuilder(payload, page.SiteID, "builder_payload")
	if len(errs) > 0 {
		return nil, fmt.Errorf("template %s is invalid: %v", name, errs)
	}
	if _, err := ctx.Query.Model(&Page{}).Where("site_id = ? AND id = ? AND deleted_at IS NULL", page.SiteID, page.ID).
		Update(map[string]any{"type": "builder", "builder_payload": mks.NewJSON(blocks), "updated_by": userID(ctx.UserID())}); err != nil {
		return nil, err
	}
	page.Type, page.BuilderPayload = "builder", mks.NewJSON(blocks)
	if err := savePageRevision(ctx.HookContext, page); err != nil {
		return nil, err
	}
	r := mks.Success("The template was applied to the page; the previous content is kept in the history.")
	r.Invalidate = []string{"pages", "page_revisions"}
	return r, nil
}

// --- live preview --------------------------------------------------------------------------

// Routes: POST /plugins/cms/preview renders unsaved blocks of the current site, for the
// editor's device preview. Same checks as saving; the HTML goes into a sandboxed iframe.
func (pl *Plugin) Routes(r route.Router) {
	// POST /plugins/cms/cache/clear empties the current site's page cache.
	// GET /plugins/cms/export?kind=post|page|media&format=csv|json|xml: the current site's
	// content as a file's text ({filename, content}; the UI saves it).
	r.Get("/export", func(ctx http.Context) http.Response {
		hc := pl.panel.Context(ctx)
		if !pl.can(hc, ImportContent) {
			return pl.panel.Forbidden(ctx)
		}
		site, err := pl.Sites.ByID(hc.TenantID())
		if err != nil || site == nil {
			return pl.panel.Forbidden(ctx)
		}
		sid, _ := toUint(site.ID)
		kind, format := ctx.Request().Query("kind"), ctx.Request().Query("format")
		body, err := pl.Export(sid, kind, format)
		if err != nil {
			return ctx.Response().Json(422, http.Json{"message": err.Error()})
		}
		ctx.Response().Header("Cache-Control", "private, no-store")
		return ctx.Response().Json(200, http.Json{
			"filename": fmt.Sprintf("%s-%ss-%s.%s", site.Slug, kind, time.Now().Format("20060102-1504"), format),
			"content":  string(body),
		})
	})
	r.Post("/packages", pl.uploadPackage)
	// GET /plugins/cms/templates: the page templates' blocks by key, for the builder.
	r.Get("/templates", func(ctx http.Context) http.Response {
		if !pl.can(pl.panel.Context(ctx), ManagePages) {
			return pl.panel.Forbidden(ctx)
		}
		out := map[string]json.RawMessage{}
		for k, v := range PageTemplates {
			out[k] = json.RawMessage(v)
		}
		return ctx.Response().Json(200, http.Json{"templates": out})
	})
	r.Post("/cache/clear", func(ctx http.Context) http.Response {
		hc := pl.panel.Context(ctx)
		if !pl.can(hc, ManageSettings) {
			return pl.panel.Forbidden(ctx)
		}
		pl.bump(hc.TenantID())
		return ctx.Response().Json(200, http.Json{"message": "Site page cache cleared."})
	})
	r.Post("/preview", func(ctx http.Context) http.Response {
		hc := pl.panel.Context(ctx)
		if !pl.can(hc, ManagePages) {
			return pl.panel.Forbidden(ctx)
		}
		var body struct {
			Payload any    `json:"payload"`
			Width   string `json:"width"`
			// Canvas: the editor's canvas (blocks framed with their tools, named by Labels);
			// blocks with errors are drawn anyway, and the errors come back with the page.
			Canvas bool              `json:"canvas"`
			Labels map[string]string `json:"labels"`
		}
		if err := ctx.Request().Bind(&body); err != nil {
			return ctx.Response().Json(400, http.Json{"message": "Bad request"})
		}
		site, err := pl.Sites.ByID(hc.TenantID())
		if err != nil || site == nil {
			return ctx.Response().Json(400, http.Json{"message": "No site"})
		}
		blocks, errs := pl.NormalizeBuilder(body.Payload, site.ID, "builder_payload")
		if len(errs) > 0 && !body.Canvas {
			return ctx.Response().Json(422, http.Json{"message": "The blocks have errors; the preview updates after they are fixed.", "errors": errs})
		}
		if len(errs) > 0 {
			// The canvas shows the blocks as they are (templates escape them; html fields
			// are cleaned when drawn), so editing goes on while something is incomplete.
			blocks = rawBlocks(body.Payload)
		}
		req := &request{ctx: ctx, site: site, now: pl.now(), settings: pl.settings(site)}
		var html template.HTML
		var script bool
		if body.Canvas {
			if len(body.Labels) > 200 {
				body.Labels = nil
			}
			html, script, err = pl.renderCanvas(req, blocks, body.Labels)
		} else {
			html, script, err = pl.renderBuilder(req, blocks, true)
		}
		if err != nil {
			pl.logf("cms: preview: %v", err)
			return ctx.Response().Json(500, http.Json{"message": "Unable to render"})
		}
		width := map[string]string{"wide": "64rem", "full": "90rem"}[body.Width]
		name := site.Name
		if n := req.settings["site_name"]; n != "" {
			name = n
		}
		view := layoutView{Title: name, SiteName: name, Width: width, Page: &pageView{Builder: html}, Script: script, Bare: body.Canvas}
		if origin := pl.origin(ctx); origin != "" {
			view.Base = safeURL(origin + "/")
		}
		out, err := pl.renderView(view)
		if err != nil {
			return ctx.Response().Json(500, http.Json{"message": "Unable to render"})
		}
		ctx.Response().Header("Cache-Control", "private, no-store")
		if len(errs) > 0 {
			return ctx.Response().Json(200, http.Json{"html": string(out), "errors": errs})
		}
		return ctx.Response().Json(200, http.Json{"html": string(out)})
	})
}

// rawBlocks reads a payload without checking it (for the canvas while it has errors):
// unknown types are dropped when drawn, and nesting is bounded by the renderer.
func rawBlocks(payload any) []Block {
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) > 2<<20 {
		return nil
	}
	var blocks []Block
	if json.Unmarshal(raw, &blocks) != nil {
		return nil
	}
	return blocks
}
