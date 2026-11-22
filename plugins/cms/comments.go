package cms

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/orm"

	mks "github.com/MiranSalehi/mks"
)

// ModerateComments: approve, reject, edit, reply to and delete comments.
const ModerateComments Ability = "cms.comments.moderate"

var commentStatuses = []string{"pending", "approved", "spam", "trash"}

func (pl *Plugin) commentsResource() mks.ModelResource[Comment] {
	moderate := func(ctx *mks.HookContext, _ *Comment) bool { return pl.can(ctx, ModerateComments) }
	setStatus := func(name, status, done string) mks.Action[Comment] {
		return mks.Action[Comment]{
			Name:      name,
			Authorize: moderate,
			Handle: bumping(pl, func(ctx *mks.ActionContext[Comment]) (*mks.ActionResult, error) {
				ids := make([]any, 0, len(ctx.Records))
				for _, c := range ctx.Records {
					ids = append(ids, c.ID)
				}
				if len(ids) == 0 {
					return nil, mks.Reject("No comment selected.")
				}
				if _, err := ctx.Query.Model(&Comment{}).Where("site_id = ?", ctx.TenantID()).WhereIn("id", ids).Update("status", status); err != nil {
					return nil, err
				}
				r := mks.Success(fmt.Sprintf(done, faDigits(len(ids))))
				r.Invalidate = []string{"cms_comments"}
				return r, nil
			}),
		}
	}
	return mks.Define[Comment](mks.Config{
		Name:          "cms_comments",
		Tenant:        "site_id",
		Relations:     map[string]string{"post": "Post", "page": "Page", "parent": "Parent"},
		Searchable:    []string{"author_name", "author_email", "content"},
		Sortable:      []string{"id", "author_name", "rating", "status", "created_at"},
		Filterable:    map[string][]string{"status": {"eq", "in"}, "rating": {"eq", "gte"}, "post_id": {"eq"}, "page_id": {"eq"}, "parent_id": {"null"}},
		Fillable:      []string{"author_name", "author_email", "content", "rating", "status"},
		TitleColumn:   "author_name",
		DefaultSort:   "-created_at",
		Deletable:     true,
		GlobalSearch:  false,
		SearchDetails: []string{"status"},
		// MKSine's pending count next to Comments in the menu.
		NavigationBadge:      func(_ *mks.HookContext, q orm.Query) orm.Query { return q.Where("status = ?", "pending") },
		NavigationBadgeColor: "warning",
		Rules: map[string]string{
			"author_name":  "required|max_len:255",
			"author_email": "email|max_len:255",
			"content":      "required|max_len:5000",
			"rating":       "integer|min:1|max:5",
			"status":       "in:" + strings.Join(commentStatuses, ","),
		},
	}).
		Policy(mks.Policy[Comment]{
			// Comments come from readers; the panel only answers them (the reply action).
			Create: func(*mks.HookContext) bool { return false },
			Update: moderate,
			Delete: moderate,
		}).
		Actions(
			setStatus("approve", "approved", "%s comment(s) approved."),
			setStatus("spam", "spam", "%s comment(s) marked as spam."),
			setStatus("trash", "trash", "%s comment(s) moved to the trash."),
			setStatus("restore", "pending", "%s comment(s) moved back to pending."),
			mks.Action[Comment]{
				Name:      "reply",
				Authorize: moderate,
				Rules:     map[string]string{"content": "required|max_len:5000"},
				Handle:    bumping(pl, pl.replyToComment),
			},
		)
}

// replyToComment answers a comment as the panel user: an approved reply on the same
// target. Replies to replies attach to the root, since the site shows one level.
func (pl *Plugin) replyToComment(ctx *mks.ActionContext[Comment]) (*mks.ActionResult, error) {
	if len(ctx.Records) != 1 {
		return nil, mks.Reject("Select exactly one comment to reply to.")
	}
	parent := ctx.Records[0]
	content := strings.TrimSpace(stringValue(ctx.Data["content"]))
	if content == "" {
		return nil, mks.ValidationErrors{"content": "The reply text is required."}
	}
	rootID := parent.ID
	if parent.ParentID != nil {
		rootID = *parent.ParentID
	}
	name := ""
	if ctx.User != nil {
		name = ctx.User.PanelUser().Name
	}
	if name == "" {
		name = "Site admin"
	}
	user := fmt.Sprint(ctx.UserID())
	reply := Comment{SiteID: parent.SiteID, PostID: parent.PostID, PageID: parent.PageID, ParentID: &rootID,
		UserID: &user, AuthorName: name, Content: content, Status: "approved"}
	if ctx.HTTP != nil {
		ip, agent := ctx.HTTP.Request().Ip(), trimTo(ctx.HTTP.Request().Header("User-Agent", ""), 500)
		reply.IPAddress, reply.UserAgent = &ip, &agent
	}
	if err := ctx.Query.Create(&reply); err != nil {
		return nil, err
	}
	r := mks.Success("Reply published.")
	r.Invalidate = []string{"cms_comments"}
	return r, nil
}

func trimTo(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// publicComment is a comment as the site shows it.
type publicComment struct {
	ID      uint
	Author  string
	Date    string
	DateISO string
	Stars   string
	Content string
	Replies []publicComment
}

// approvedComments: approved top-level comments, oldest first, each with its approved
// replies (one level, as in MKSine). where selects the target.
func (pl *Plugin) approvedComments(q orm.Query, siteID any, column string, id uint, calendar string) ([]publicComment, float64, int, error) {
	var rows []Comment
	if err := q.Model(&Comment{}).Where("site_id = ? AND "+column+" = ? AND status = ?", siteID, id, "approved").
		OrderBy("created_at").OrderBy("id").Find(&rows); err != nil {
		return nil, 0, 0, err
	}
	byParent := map[uint][]Comment{}
	var roots []Comment
	var sum, count int
	for _, c := range rows {
		if c.ParentID == nil {
			roots = append(roots, c)
			if c.Rating != nil && *c.Rating > 0 {
				sum += int(*c.Rating)
				count++
			}
		} else {
			byParent[*c.ParentID] = append(byParent[*c.ParentID], c)
		}
	}
	view := func(c Comment) publicComment {
		var date, iso string
		if c.CreatedAt != nil {
			t := c.CreatedAt.StdTime()
			date, iso = formatDate(&t, calendar)
		}
		stars := ""
		if c.Rating != nil && *c.Rating >= 1 && *c.Rating <= 5 {
			stars = strings.Repeat("★", int(*c.Rating)) + strings.Repeat("☆", 5-int(*c.Rating))
		}
		return publicComment{ID: c.ID, Author: c.AuthorName, Date: date, DateISO: iso, Stars: stars, Content: c.Content}
	}
	out := make([]publicComment, 0, len(roots))
	for _, r := range roots {
		pc := view(r)
		for _, reply := range byParent[r.ID] {
			pc.Replies = append(pc.Replies, view(reply))
		}
		out = append(out, pc)
	}
	avg := 0.0
	if count > 0 {
		avg = float64(sum) / float64(count)
	}
	return out, avg, count, nil
}
