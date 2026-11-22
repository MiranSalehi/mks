// Package activity is a panel plugin that logs who created, changed or deleted what,
// and shows the log as a read-only resource ("activities"). Its frontend half is
// @mksine/mks/activity.
//
//	p.Use(activity.Plugin{Authorize: func(ctx *mks.HookContext) bool { return isAdmin(ctx) }})
//
// The table comes from a migration: activity.CreateTable(facades.Schema()).
package activity

import (
	"fmt"
	"time"

	"github.com/goravel/framework/contracts/database/schema"

	mks "github.com/MiranSalehi/mks"
)

// Table is where the log lives.
const Table = "panel_activities"

// CreateTable creates the log table; call it from a migration.
func CreateTable(s schema.Schema) error {
	return s.Create(Table, func(t schema.Blueprint) {
		t.BigIncrements("id")
		t.String("user_id", 64).Nullable()
		t.String("user_name", 255).Nullable()
		t.String("resource", 64)
		t.String("record_key", 64)
		t.String("title", 255).Nullable()
		t.String("action", 16)
		t.Timestamp("created_at").Nullable()
		t.Index("resource", "record_key")
	})
}

// Activity is one log entry.
type Activity struct {
	ID        uint64     `gorm:"primaryKey;column:id" json:"id"`
	UserID    *string    `gorm:"column:user_id" json:"user_id"`
	UserName  *string    `gorm:"column:user_name" json:"user_name"`
	Resource  string     `gorm:"column:resource" json:"resource"`
	RecordKey string     `gorm:"column:record_key" json:"record_key"`
	Title     *string    `gorm:"column:title" json:"title"`
	Action    string     `gorm:"column:action" json:"action"` // created, updated, deleted
	CreatedAt *time.Time `gorm:"column:created_at" json:"created_at"`
}

func (Activity) TableName() string { return Table }

// Plugin logs every save and delete made through the panel.
type Plugin struct {
	// Authorize decides who may read the log; unset means every panel user.
	Authorize func(ctx *mks.HookContext) bool
}

func (Plugin) Name() string { return "activity-log" }

func (pl Plugin) Register(p *mks.Panel) {
	// Same transaction as the change: no log entry for a change that was rolled back.
	p.OnSaved(func(ctx *mks.HookContext, resource string, record any, created bool) error {
		action := "updated"
		if created {
			action = "created"
		}
		return log(p, ctx, resource, record, action)
	})
	p.OnDeleted(func(ctx *mks.HookContext, resource string, record any) error {
		return log(p, ctx, resource, record, "deleted")
	})

	never := func(*mks.HookContext) bool { return false }
	p.Register(mks.Define[Activity](mks.Config{
		Name:        "activities",
		Searchable:  []string{"title", "user_name"},
		Sortable:    []string{"id", "created_at"},
		Filterable:  map[string][]string{"action": {"eq", "in"}, "resource": {"eq", "in"}},
		DefaultSort: "-id",
		TitleColumn: "title",
	}).Policy(mks.Policy[Activity]{
		ViewAny: pl.Authorize,
		// The log is read-only, even for admins.
		Create: never,
		Update: func(*mks.HookContext, *Activity) bool { return false },
		Delete: func(*mks.HookContext, *Activity) bool { return false },
	}))
}

func log(p *mks.Panel, ctx *mks.HookContext, resource string, record any, action string) error {
	if resource == "activities" {
		return nil
	}
	key, err := p.Key(resource, record)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	entry := Activity{Resource: resource, RecordKey: fmt.Sprint(key), Action: action, CreatedAt: &now}
	if t := p.Title(resource, record); t != "" {
		entry.Title = &t
	}
	if ctx.User != nil {
		info := ctx.User.PanelUser()
		id, name := fmt.Sprint(info.ID), info.Name
		entry.UserID, entry.UserName = &id, &name
	}
	return ctx.Query.Create(&entry)
}
