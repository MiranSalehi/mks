package cms

import (
	"fmt"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/http"
)

// The public comment form: a POST to {site}/comments. Public comments are always guest
// comments (a name and an email), even for a logged-in panel user: a forged request can
// then not speak for anyone, so the form needs no CSRF token and pages stay cacheable.
// Panel users answer through the panel's reply action.

type commentsView struct {
	Count    string
	Average  string
	Items    []publicComment
	Action   string
	Target   string
	ReplyTo  *publicComment
	Notice   string
	Error    string
	Open     bool
	FormLink string
}

var commentNotices = map[string]string{
	"sent":     "دیدگاه شما ثبت شد و پس از تأیید نمایش داده می‌شود.",
	"name":     "نام لازم است (حداکثر ۱۰۰ نویسه).",
	"email":    "یک ایمیل معتبر وارد کنید.",
	"content":  "متن دیدگاه باید بین ۳ تا ۵۰۰۰ نویسه باشد.",
	"rating":   "امتیاز باید بین ۱ تا ۵ باشد.",
	"parent":   "دیدگاهی که به آن پاسخ می‌دهید پیدا نشد.",
	"closed":   "برای این مورد نمی‌توان دیدگاه گذاشت.",
	"too-many": "دیدگاه‌های زیادی از نشانی شما رسیده است؛ کمی بعد دوباره تلاش کنید.",
}

// commentsFor builds the comments section of a post or page.
func (pl *Plugin) commentsFor(req *request, kind string, id uint, base string) (*commentsView, error) {
	column := kind + "_id"
	items, avg, rated, err := pl.approvedComments(pl.query(), req.site.ID, column, id, req.calendar())
	if err != nil {
		return nil, err
	}
	total := 0
	for _, c := range items {
		total += 1 + len(c.Replies)
	}
	v := &commentsView{Count: persianDigits.Replace(strconv.Itoa(total)), Items: items, Open: true,
		Action: pl.SiteURL(req.site.Slug) + "/comments", Target: kind + ":" + strconv.FormatUint(uint64(id), 10), FormLink: base}
	if rated > 0 {
		v.Average = persianDigits.Replace(strconv.FormatFloat(avg, 'f', 1, 64)) + " از ۵ (" + persianDigits.Replace(strconv.Itoa(rated)) + " امتیاز)"
	}
	if replyTo, err := strconv.ParseUint(req.ctx.Request().Query("reply"), 10, 64); err == nil {
		for i := range items {
			if uint64(items[i].ID) == replyTo {
				v.ReplyTo = &items[i]
			}
		}
	}
	switch code := req.ctx.Request().Query("comment"); code {
	case "":
	case "sent":
		v.Notice = commentNotices[code]
	default:
		if msg, ok := commentNotices[code]; ok {
			v.Error = msg
		}
	}
	return v, nil
}

// SubmitComment takes a reader's comment: validated, rate-limited per IP address, with a
// honeypot field for bots, and saved as pending.
func (pl *Plugin) SubmitComment(ctx http.Context) http.Response {
	site, err := pl.Sites.BySlug(ctx.Request().Route("site"))
	if err != nil {
		pl.logf("cms: load site: %v", err)
		return plain(ctx, 500, "Unable to save comment")
	}
	if site == nil {
		return notFound(ctx)
	}
	// Refuse posts from other origins when the browser says where it came from.
	if origin := ctx.Request().Header("Origin", ""); origin != "" {
		if u, err := url.Parse(origin); err != nil || u.Host != ctx.Request().Host() {
			return plain(ctx, 403, "Forbidden")
		}
	}
	in := ctx.Request()
	kind, idText, _ := strings.Cut(in.Input("target"), ":")
	targetID, err := strconv.ParseUint(idText, 10, 64)
	if err != nil || (kind != "post" && !(kind == "page" && pl.CommentsOnPages)) {
		return plain(ctx, 400, "Bad request")
	}
	now := pl.now()
	base, open, err := pl.commentTarget(site, kind, uint(targetID), now)
	if err != nil {
		pl.logf("cms: comment target: %v", err)
		return plain(ctx, 500, "Unable to save comment")
	}
	back := func(code string) http.Response {
		anchor := "#comment-form"
		if code == "sent" {
			anchor = "#comments"
		}
		return ctx.Response().Redirect(303, base+"?comment="+code+anchor)
	}
	if !open {
		return notFound(ctx)
	}
	// Bots fill every field; people never see this one. Pretend it worked.
	if strings.TrimSpace(in.Input("website")) != "" {
		return back("sent")
	}
	name := strings.TrimSpace(in.Input("name"))
	email := strings.TrimSpace(in.Input("email"))
	content := strings.TrimSpace(in.Input("content"))
	switch {
	case name == "" || utf8.RuneCountInString(name) > 100:
		return back("name")
	case !validEmail(email):
		return back("email")
	case utf8.RuneCountInString(content) < 3 || utf8.RuneCountInString(content) > 5000:
		return back("content")
	}
	var rating *uint8
	if r := strings.TrimSpace(in.Input("rating")); r != "" {
		n, err := strconv.Atoi(r)
		if err != nil || n < 1 || n > 5 {
			return back("rating")
		}
		v := uint8(n)
		rating = &v
	}
	var parentID *uint
	if p := strings.TrimSpace(in.Input("parent_id")); p != "" {
		pid, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return back("parent")
		}
		// The parent must be an approved top-level comment of the same target.
		n, err := pl.query().Model(&Comment{}).
			Where("id = ? AND site_id = ? AND "+kind+"_id = ? AND status = ? AND parent_id IS NULL", pid, site.ID, targetID, "approved").Count()
		if err != nil {
			pl.logf("cms: comment parent: %v", err)
			return plain(ctx, 500, "Unable to save comment")
		}
		if n == 0 {
			return back("parent")
		}
		id := uint(pid)
		parentID = &id
		rating = nil // ratings rate the target, so only top-level comments carry them
	}
	if !pl.allowComment(in.Ip()) {
		return back("too-many")
	}
	ip, agent := in.Ip(), trimTo(in.Header("User-Agent", ""), 500)
	siteID, ok := toUint(fmt.Sprint(site.ID))
	if !ok {
		return plain(ctx, 500, "Unable to save comment")
	}
	c := Comment{SiteID: siteID, ParentID: parentID, AuthorName: name, AuthorEmail: &email, Content: content,
		Rating: rating, Status: "pending", IPAddress: &ip, UserAgent: &agent}
	tid := uint(targetID)
	if kind == "post" {
		c.PostID = &tid
	} else {
		c.PageID = &tid
	}
	if err := pl.query().Create(&c); err != nil {
		pl.logf("cms: save comment: %v", err)
		return plain(ctx, 500, "Unable to save comment")
	}
	return back("sent")
}

func validEmail(s string) bool {
	if s == "" || len(s) > 255 {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Address == s && strings.Contains(s[strings.LastIndex(s, "@")+1:], ".")
}

// allowComment counts comments per IP address in the cache: at most CommentsPerMinute in
// a minute (MKSine's comments.max_per_minute). The counter lives in a per-minute key, read
// and written with GetInt and Put so every cache driver works; a race can let a comment
// or two more through, which is fine for a spam brake.
func (pl *Plugin) allowComment(ip string) bool {
	limit := pl.CommentsPerMinute
	if limit <= 0 {
		limit = 5
	}
	key := "cms:comments:" + ip + ":" + strconv.FormatInt(pl.now().Unix()/60, 10)
	n := pl.Cache.GetInt(key, 0)
	if n >= limit {
		return false
	}
	return pl.Cache.Put(key, n+1, 2*time.Minute) == nil
}

// commentTarget finds the public post or page a comment is for: its address and whether
// it takes comments now.
func (pl *Plugin) commentTarget(site *Site, kind string, id uint, now time.Time) (string, bool, error) {
	if kind == "post" {
		var post Post
		found, err := first(pl.query().Model(&Post{}).Where("site_id = ? AND id = ? AND status = ? AND (published_at IS NULL OR published_at <= ?)", site.ID, id, "published", now), &post)
		if err != nil || !found {
			return pl.SiteURL(site.Slug), false, err
		}
		return pl.PostURL(site.Slug, post.Slug), true, nil
	}
	var page Page
	found, err := first(pl.query().Model(&Page{}).Where("site_id = ? AND id = ? AND deleted_at IS NULL", site.ID, id), &page)
	if err != nil || !found || !publicPageAvailable(page.Status, page.PublishedAt, now) {
		return pl.SiteURL(site.Slug), false, err
	}
	return pl.PageURL(site.Slug, page.Slug), true, nil
}
