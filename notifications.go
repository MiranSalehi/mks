package mks

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/orm"
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/contracts/http"
)

// Database notifications: messages for one user, shown under the bell in the panel.
//
//	p.Notify(mks.Notice{Type: "success", Title: "Post published", URL: "/posts/12"}, authorID)
//
// The UI checks for new ones every Options.NotificationPoll seconds while the tab is
// visible (Goravel's gin driver buffers every response in its global timeout middleware,
// so a long-lived push stream is not possible there).

// NotificationsTable is the table the notifications live in.
const NotificationsTable = "panel_notifications"

const notificationsPageSize = 20

// CreateNotificationsTable creates the notifications table; call it from a migration:
//
//	func (r *M...) Up() error   { return mks.CreateNotificationsTable(facades.Schema()) }
//	func (r *M...) Down() error { return facades.Schema().DropIfExists(mks.NotificationsTable) }
func CreateNotificationsTable(s schema.Schema) error {
	return s.Create(NotificationsTable, func(t schema.Blueprint) {
		t.BigIncrements("id")
		// Text, so integer and UUID user keys both fit.
		t.String("user_id", 64)
		t.String("type", 16)
		t.String("title", 255)
		t.Text("body").Nullable()
		t.String("url", 255).Nullable()
		t.Timestamp("read_at").Nullable()
		t.Timestamp("created_at").Nullable()
		t.Index("user_id", "read_at")
	})
}

// Notice is a notification to send.
type Notice struct {
	// Type is "success", "warning", "danger" or "info" (default).
	Type  string
	Title string
	Body  string
	// URL is a path inside the panel, e.g. "/posts/12" (opened when the user clicks).
	URL string
}

// notification is a stored row.
type notification struct {
	ID        uint64     `gorm:"primaryKey;column:id"`
	UserID    string     `gorm:"column:user_id"`
	Type      string     `gorm:"column:type"`
	Title     string     `gorm:"column:title"`
	Body      *string    `gorm:"column:body"`
	URL       *string    `gorm:"column:url"`
	ReadAt    *time.Time `gorm:"column:read_at"`
	CreatedAt *time.Time `gorm:"column:created_at"`
}

func (notification) TableName() string { return NotificationsTable }

func (n notification) json() http.Json {
	out := http.Json{"id": n.ID, "type": n.Type, "title": n.Title, "body": n.Body, "url": n.URL, "read_at": nil, "created_at": nil}
	if n.ReadAt != nil {
		out["read_at"] = n.ReadAt.UTC().Format(time.RFC3339)
	}
	if n.CreatedAt != nil {
		out["created_at"] = n.CreatedAt.UTC().Format(time.RFC3339)
	}
	return out
}

var noticeTypes = map[string]bool{"success": true, "warning": true, "danger": true, "info": true}

// Notify stores a notification for each user (by key, e.g. a user ID).
func (p *Panel) Notify(n Notice, users ...any) error {
	return p.NotifyTx(p.opts.Orm.Query(), n, users...)
}

// NotifyTx is Notify inside a transaction, e.g. an action's ctx.Query: the notifications
// are only kept if the transaction commits.
func (p *Panel) NotifyTx(q orm.Query, n Notice, users ...any) error {
	if n.Type == "" {
		n.Type = "info"
	}
	if !noticeTypes[n.Type] {
		return fmt.Errorf("mks: unknown notification type %q", n.Type)
	}
	if strings.TrimSpace(n.Title) == "" {
		return fmt.Errorf("mks: a notification needs a title")
	}
	// Only paths inside the panel: no other sites, no javascript: links.
	if n.URL != "" && (!strings.HasPrefix(n.URL, "/") || strings.HasPrefix(n.URL, "//")) {
		return fmt.Errorf("mks: notification URL must be a path inside the panel, got %q", n.URL)
	}
	now := time.Now().UTC()
	for _, u := range users {
		if u == nil {
			continue
		}
		row := notification{UserID: fmt.Sprint(u), Type: n.Type, Title: n.Title, CreatedAt: &now}
		if n.Body != "" {
			row.Body = &n.Body
		}
		if n.URL != "" {
			row.URL = &n.URL
		}
		if err := q.Create(&row); err != nil {
			return err
		}
	}
	return nil
}

// --- API ----------------------------------------------------------------------------------

func (p *Panel) userKey(ctx http.Context) (string, bool) {
	u := p.currentUser(ctx)
	if u == nil {
		return "", false
	}
	return fmt.Sprint(u.PanelUser().ID), true
}

// notifications answers GET /notifications?page=1: newest first, with the unread count.
func (p *Panel) notifications(ctx http.Context) http.Response {
	user, ok := p.userKey(ctx)
	if !ok {
		return message(ctx, 401, p.tr(ctx).unauthenticated)
	}
	page := 1
	if v := ctx.Request().Query("page"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return message(ctx, 400, "invalid page")
		}
		page = n
	}
	var rows []notification
	err := p.opts.Orm.Query().Where("user_id = ?", user).
		OrderByDesc("id").Offset((page - 1) * notificationsPageSize).Limit(notificationsPageSize + 1).
		Find(&rows)
	if err != nil {
		return p.serverError(ctx, err)
	}
	unread, err := p.opts.Orm.Query().Model(&notification{}).Where("user_id = ?", user).WhereNull("read_at").Count()
	if err != nil {
		return p.serverError(ctx, err)
	}
	hasMore := len(rows) > notificationsPageSize
	if hasMore {
		rows = rows[:notificationsPageSize]
	}
	data := make([]http.Json, len(rows))
	for i, r := range rows {
		data[i] = r.json()
	}
	return ctx.Response().Json(200, http.Json{"data": data, "meta": http.Json{"page": page, "has_more": hasMore, "unread": unread}})
}

// ownNotifications is the user's notifications, or one of them with {id}.
func (p *Panel) ownNotifications(ctx http.Context, one bool) (orm.Query, http.Response) {
	user, ok := p.userKey(ctx)
	if !ok {
		return nil, message(ctx, 401, p.tr(ctx).unauthenticated)
	}
	q := p.opts.Orm.Query().Model(&notification{}).Where("user_id = ?", user)
	if one {
		id, err := strconv.ParseUint(ctx.Request().Route("id"), 10, 64)
		if err != nil {
			return nil, message(ctx, 404, p.tr(ctx).recordNotFound)
		}
		q = q.Where("id = ?", id)
	}
	return q, nil
}

// markRead answers POST /notifications/{id}/read and POST /notifications/read (all).
func (p *Panel) markRead(one bool) func(ctx http.Context) http.Response {
	return func(ctx http.Context) http.Response {
		q, resp := p.ownNotifications(ctx, one)
		if resp != nil {
			return resp
		}
		res, err := q.WhereNull("read_at").Update("read_at", time.Now().UTC())
		if err != nil {
			return p.serverError(ctx, err)
		}
		return ctx.Response().Json(200, http.Json{"count": res.RowsAffected})
	}
}

// deleteNotifications answers DELETE /notifications/{id} and DELETE /notifications (all).
func (p *Panel) deleteNotifications(one bool) func(ctx http.Context) http.Response {
	return func(ctx http.Context) http.Response {
		q, resp := p.ownNotifications(ctx, one)
		if resp != nil {
			return resp
		}
		res, err := q.Delete(&notification{})
		if err != nil {
			return p.serverError(ctx, err)
		}
		if one && res.RowsAffected == 0 {
			return message(ctx, 404, p.tr(ctx).recordNotFound)
		}
		return ctx.Response().Json(200, http.Json{"count": res.RowsAffected})
	}
}

// notificationsMeta tells the UI whether to show the bell and how often to look.
func (p *Panel) notificationsMeta() any {
	if !p.opts.Notifications {
		return nil
	}
	return http.Json{"poll": int(p.opts.NotificationPoll / time.Second)}
}
