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
		{"type": "hero", "data": {"title": "عنوان اصلی صفحه", "subtitle": "یک جملهٔ کوتاه دربارهٔ ارزشی که می‌دهید.", "height": "large", "overlay": "gradient", "button_text": "شروع کنید", "button_url": "#features", "show_secondary_button": true, "secondary_button_text": "بیشتر بدانید", "secondary_button_url": "#faq"}},
		{"type": "features", "data": {"anchor": "features", "heading": "چرا ما؟", "subheading": "سه دلیل مهم.", "columns": 3, "style": "shadowed", "features": [
			{"icon": "⚡", "title": "سریع", "description": "توضیح کوتاه این مزیت."},
			{"icon": "🔒", "title": "امن", "description": "توضیح کوتاه این مزیت."},
			{"icon": "💬", "title": "پشتیبانی", "description": "توضیح کوتاه این مزیت."}]}},
		{"type": "testimonial", "data": {"heading": "مشتریان ما می‌گویند", "columns": 2, "testimonials": [
			{"content": "تجربهٔ بسیار خوبی بود.", "author_name": "نام مشتری", "author_title": "سمت", "rating": 5},
			{"content": "پیشنهادش می‌کنم.", "author_name": "نام مشتری", "author_title": "سمت", "rating": 5}]}},
		{"type": "accordion", "data": {"anchor": "faq", "heading": "پرسش‌های پرتکرار", "items": [
			{"question": "پرسش اول؟", "answer": "<p>پاسخ کوتاه.</p>"},
			{"question": "پرسش دوم؟", "answer": "<p>پاسخ کوتاه.</p>"}]}},
		{"type": "cta", "data": {"title": "آماده‌اید؟", "description": "همین حالا شروع کنید.", "button_text": "تماس با ما", "button_url": "#"}}
	]`,
	"about": `[
		{"type": "heading", "data": {"text": "دربارهٔ ما", "level": "h1", "alignment": "center"}},
		{"type": "text", "data": {"content": "<p>داستان شروع کار، ارزش‌ها و هدف‌های ما را این‌جا بنویسید.</p>"}},
		{"type": "columns", "data": {"layout": "equal", "children": [
			{"blocks": [{"type": "heading", "data": {"text": "مأموریت", "level": "h3"}}, {"type": "text", "data": {"content": "<p>چه می‌کنیم و چرا.</p>"}}]},
			{"blocks": [{"type": "heading", "data": {"text": "چشم‌انداز", "level": "h3"}}, {"type": "text", "data": {"content": "<p>به کجا می‌رویم.</p>"}}]}]}},
		{"type": "cta", "data": {"title": "با ما همراه شوید", "style": "simple", "button_text": "تماس", "button_url": "#"}}
	]`,
	"contact": `[
		{"type": "heading", "data": {"text": "تماس با ما", "level": "h1"}},
		{"type": "columns", "data": {"layout": "equal", "children": [
			{"blocks": [{"type": "card", "data": {"children": [{"blocks": [{"type": "heading", "data": {"text": "نشانی", "level": "h4"}}]}, {"blocks": [{"type": "text", "data": {"content": "<p>نشانی دفتر را بنویسید.</p>"}}]}, {"blocks": []}]}}]},
			{"blocks": [{"type": "card", "data": {"children": [{"blocks": [{"type": "heading", "data": {"text": "تلفن و ایمیل", "level": "h4"}}]}, {"blocks": [{"type": "button", "data": {"text": "ارسال ایمیل", "url": "mailto:info@example.com", "style": "outline"}}]}, {"blocks": []}]}}]}]}}
	]`,
	"services": `[
		{"type": "hero", "data": {"title": "خدمات ما", "height": "small", "overlay": "dark"}},
		{"type": "features", "data": {"columns": 3, "style": "bordered", "icon_style": "square", "features": [
			{"icon": "🛠", "title": "خدمت اول", "description": "توضیح خدمت."},
			{"icon": "📈", "title": "خدمت دوم", "description": "توضیح خدمت."},
			{"icon": "🎯", "title": "خدمت سوم", "description": "توضیح خدمت."}]}},
		{"type": "tabs", "data": {"style": "pills", "tabs": [
			{"title": "روش کار", "content": "<p>گام‌های همکاری.</p>"},
			{"title": "هزینه‌ها", "content": "<p>پلن‌ها و قیمت‌ها.</p>"}]}},
		{"type": "cta", "data": {"title": "پروژه‌ای در ذهن دارید؟", "button_text": "درخواست مشاوره", "button_url": "#"}}
	]`,
	"blank": `[]`,
	// MKSine's default home (its theme blocks; the pricing block came from a plugin).
	"mksine_home": `[
		{"type": "mksine_hero_domain", "data": {"heading_line1": "دامنهٔ شما.", "heading_line2_prefix": "برند شما.", "heading_accent": "آنلاین در چند دقیقه.", "subheading": "یک دامنهٔ به‌یادماندنی، ایمیل امن و SSL؛ هرچه برای شروع مطمئن لازم دارید.", "cta_label": "جستجوی دامنه", "cta_url": "#domains", "illustration_position": "right", "background_color": "#efeaff"}},
		{"type": "mksine_finance_showcase", "data": {"heading": "حساب‌وکتاب کسب‌وکارتان، بی‌دردسر.", "heading_accent": "بی‌دردسر.", "paragraph_1": "فاکتورها، پرداخت‌ها و مالیات در یک داشبورد روشن، برای تیم‌های پرمشغله.", "paragraph_1_highlight": "داشبورد روشن", "paragraph_2": "یادآوری‌ها را خودکار کنید و با پیگیری هوشمند زودتر پول بگیرید.", "paragraph_2_highlight": "پیگیری هوشمند", "cta_primary_label": "شروع رایگان", "cta_primary_url": "#", "cta_secondary_label": "گفت‌وگو با فروش", "cta_secondary_url": "#", "trust_line": "برای دیدن نسخهٔ نمایشی کارت بانکی لازم نیست.", "social_caption": "مورد اعتماد کسب‌وکارهای امروزی", "social_count": "+۱۲٬۴۰۰ تیم", "mock_chart_caption": "درآمد ماهانه", "avatars": [{"letter": "ع"}, {"letter": "م"}, {"letter": "س"}, {"letter": "ر"}, {"letter": "ن"}]}},
		{"type": "mksine_services_trio", "data": {"section_title": "هرچه برای آنلاین‌شدن لازم دارید", "cards": [{"title": "میزبانی سریع", "body": "حافظهٔ SSD، پشتیبان روزانه و کنترل‌پنلی که سر راهتان نیست.", "cta_label": "دیدن پلن‌ها", "url": "#"}, {"title": "ایمیل حرفه‌ای", "body": "صندوق‌های ایمیل با نام برند شما، با محافظت در برابر هرزنامه.", "cta_label": "گرفتن ایمیل", "url": "#"}, {"title": "SSL رایگان", "body": "ترافیک بازدیدکننده را رمز کنید و اعتماد بسازید.", "cta_label": "بیشتر بدانید", "url": "#"}]}},
		{"type": "mksine_featured_domains", "data": {"anchor": "domains", "title": "پسوندهای ویژه", "view_all_label": "همهٔ دامنه‌ها", "view_all_url": "#", "items": [{"tld": ".ir", "category": "ایران", "price": "۱۸۰٬۰۰۰ تومان", "href": "#"}, {"tld": ".com", "category": "جهانی و کلاسیک", "price": "۱٬۲۰۰٬۰۰۰ تومان", "href": "#"}, {"tld": ".net", "category": "شبکه‌ها", "price": "۹۵۰٬۰۰۰ تومان", "original_price": "۱٬۴۰۰٬۰۰۰ تومان", "discount_percent": 32, "href": "#"}, {"tld": ".online", "category": "جسور و امروزی", "price": "۲۵۰٬۰۰۰ تومان", "original_price": "۲٬۵۰۰٬۰۰۰ تومان", "discount_percent": 90, "href": "#"}]}},
		{"type": "mksine_clinic_features_grid", "data": {"badge": "چرا ما؟", "heading_prefix": "ابزارهای قدرتمند برای ", "heading_accent": "کسب‌وکارهای امروزی", "subheading": "نوبت‌دهی، پرونده‌ها و ارتباط با مشتری در یک جا.", "cards": [{"gradient": "from-blue-500 to-blue-600", "icon": "users", "title": "پرونده‌های منظم", "body": "سابقه و پیوست‌ها در یک نگاه."}, {"gradient": "from-emerald-500 to-emerald-600", "icon": "calendar", "title": "نوبت‌دهی هوشمند", "body": "با یادآوری و رزرو آنلاین، غیبت‌ها کم می‌شود."}, {"gradient": "from-violet-500 to-violet-600", "icon": "chart-column", "title": "گزارش‌ها", "body": "دید روشن به درآمد و مراجعه‌ها."}, {"gradient": "from-yellow-500 to-yellow-600", "icon": "clipboard-list", "title": "انبار", "body": "پیگیری لوازم با هشدار کمبود."}, {"gradient": "from-purple-500 to-purple-600", "icon": "credit-card", "title": "صورت‌حساب", "body": "فاکتور، رسید و پیگیری پرداخت."}, {"gradient": "from-pink-500 to-pink-600", "icon": "settings", "title": "خودکارسازی", "body": "قاعده‌هایی که هر هفته ساعت‌ها صرفه‌جویی می‌کنند."}]}},
		{"type": "mksine_testimonials_grid", "data": {"badge": "تجربهٔ مشتریان", "title_prefix": "محبوب تیم‌ها ", "title_accent": "در همه‌جا", "subheading": "بازخورد واقعی از کسب‌وکارهایی که فصل پیش مهاجرت کردند.", "aside": "میانگین زمان پاسخ کمتر از ۲ ساعت؛ هر وقت لازم باشد کنارتان هستیم.", "items": [{"name": "سارا لطفی", "city": "تهران", "quote": "زمان کارهای اداری نصف شد؛ نوبت‌دهی و صورت‌حساب بالاخره به هم وصل‌اند.", "gradient": "from-violet-500 to-purple-600"}, {"name": "مهدی رضایی", "city": "اصفهان", "quote": "یادآوری‌ها روشن‌ترند و نوبت‌های بیشتری پر می‌شود.", "gradient": "from-indigo-500 to-blue-600"}, {"name": "الهام رستمی", "city": "شیراز", "quote": "گزارش‌گیری که یک آخر هفته کار داشت، حالا یک کلیک است.", "gradient": "from-fuchsia-500 to-pink-600"}]}}
	]`,
}

// applyTemplate replaces a page's blocks with a template (the page becomes block-based).
// The previous content stays in the revision history.
func (pl *Plugin) applyTemplate(ctx *mks.ActionContext[Page]) (*mks.ActionResult, error) {
	if ctx == nil || !pl.can(ctx.HookContext, ManagePages) {
		return nil, mks.Reject("اجازهٔ تغییر برگه را ندارید.")
	}
	if ctx.Query == nil || len(ctx.Records) != 1 || ctx.Records[0] == nil {
		return nil, mks.Reject("دقیقاً یک برگه انتخاب کنید.")
	}
	page := ctx.Records[0]
	name, _ := ctx.Data["template"].(string)
	raw, ok := PageTemplates[name]
	if !ok {
		return nil, mks.ValidationErrors{"template": "قالب انتخاب‌شده وجود ندارد."}
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
	r := mks.Success("قالب روی برگه گذاشته شد؛ محتوای قبلی در تاریخچه مانده است.")
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
		return ctx.Response().Json(200, http.Json{"message": "cache صفحه‌های سایت پاک شد."})
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
			return ctx.Response().Json(422, http.Json{"message": "بلوک‌ها خطا دارند؛ پیش‌نمایش پس از رفع آن‌ها به‌روز می‌شود.", "errors": errs})
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
