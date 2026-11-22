package cms

import (
	"strings"
	"time"

	"github.com/goravel/framework/contracts/database/orm"

	mks "github.com/MiranSalehi/mks"
)

func (pl *Plugin) pagesResource() mks.ModelResource[Page] {
	return mks.Define[Page](mks.Config{
		Name:       "pages",
		Tenant:     "site_id",
		Relations:  map[string]string{"revisions": "Revisions", "tags": "Tags"},
		Searchable: []string{"title", "slug", "meta_title"},
		Sortable:   []string{"id", "title", "slug", "status", "type", "published_at", "created_at", "updated_at"},
		Filterable: map[string][]string{
			"status":       {"eq", "in"},
			"type":         {"eq"},
			"published_at": {"gte", "lt", "null"},
			"created_at":   {"gte", "lt"},
		},
		Fillable:      []string{"title", "slug", "type", "status", "content", "builder_payload", "show_page_header", "builder_content_width", "meta_title", "meta_description", "focus_keyphrase", "published_at", "tags", GalleryCollection},
		TitleColumn:   "title",
		GlobalSearch:  true,
		SearchDetails: []string{"status", "slug"},
		HTML:          []string{"content"},
		DefaultSort:   "-updated_at",
		Rules: map[string]string{
			"title":                  "required|max_len:255",
			"slug":                   "required|max_len:255|regex:^[a-z0-9-]+$",
			"type":                   "in:simple,builder",
			"status":                 "in:draft,published,scheduled",
			"builder_content_width":  "in:contained,wide,full",
			"meta_title":             "max_len:255",
			"meta_description":       "max_len:500",
			"focus_keyphrase":        "max_len:255",
			"builder_payload":        "array",
			"builder_payload.*.type": "required|in:" + strings.Join(BlockTypes, ","),
			"published_at":           "date",
		},
	}).
		BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
			for field, value := range map[string]any{"type": "simple", "status": "draft", "builder_content_width": "contained", "show_page_header": true} {
				if _, sent := data[field]; !sent {
					data[field] = value
				}
			}
			if err := pl.takeGallery(ctx, OwnerPage, data["site_id"], data); err != nil {
				return err
			}
			if err := pl.validatePageFields(data, nil); err != nil {
				return err
			}
			data["created_by"] = userID(ctx.UserID())
			data["updated_by"] = userID(ctx.UserID())
			return pl.beforeSave(ctx, "page", data["site_id"], nil, data, "", "")
		}).
		BeforeUpdate(func(ctx *mks.HookContext, page *Page, data map[string]any) error {
			if err := pl.takeGallery(ctx, OwnerPage, page.SiteID, data); err != nil {
				return err
			}
			if err := pl.validatePageFields(data, page); err != nil {
				return err
			}
			data["updated_by"] = userID(ctx.UserID())
			return pl.beforeSave(ctx, "page", page.SiteID, page, data, page.Slug, page.Status)
		}).
		AfterSave(func(ctx *mks.HookContext, page *Page, created bool) error {
			if err := savePageRevision(ctx, page); err != nil {
				return err
			}
			if err := pl.syncGallery(ctx, OwnerPage, page.SiteID, page.ID); err != nil {
				return err
			}
			return pl.afterSave(ctx, "page", page.SiteID, page, created, page.Slug, page.Status)
		}).
		AfterRead(func(_ *mks.HookContext, pages []*Page) error {
			ids := make([]uint, len(pages))
			for i, p := range pages {
				ids[i] = p.ID
			}
			galleries, err := pl.attachmentIDs(OwnerPage, ids, GalleryCollection)
			for _, p := range pages {
				p.Gallery = galleries[p.ID]
				if p.Gallery == nil {
					p.Gallery = []uint{}
				}
			}
			return err
		}).
		Policy(mks.Policy[Page]{
			Create: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManagePages) },
			Update: func(ctx *mks.HookContext, _ *Page) bool { return pl.can(ctx, ManagePages) },
			// Pages leave through the trash, never by a plain delete.
			Delete: func(*mks.HookContext, *Page) bool { return false },
			Scope: func(_ *mks.HookContext, q orm.Query) orm.Query {
				return q.Where("deleted_at IS NULL")
			},
		}).
		Actions(pl.pageActions()...)
}

// validatePageFields enforces site-local slugs and the publication contract. A scheduled
// page becomes public when its time comes, without a scheduler; drafts never do.
func (pl *Plugin) validatePageFields(data map[string]any, stored *Page) error {
	siteID := data["site_id"]
	if stored != nil {
		siteID = stored.SiteID
	}
	if slug, ok := data["slug"].(string); ok && slug != "" {
		if siteID == nil {
			return mks.Reject("برای برگه، سایت فعال لازم است.")
		}
		// A trashed page still holds its slug (the unique index), so it can be restored.
		q := pl.query().Model(&Page{}).Where("site_id = ? AND slug = ?", siteID, slug)
		if stored != nil {
			q = q.Where("id <> ?", stored.ID)
		}
		count, err := q.Count()
		if err != nil {
			return err
		}
		if count > 0 {
			return mks.ValidationErrors{"slug": "این نامک در سایت انتخاب‌شده قبلاً استفاده شده است."}
		}
	}
	if raw, sent := data["builder_payload"]; sent {
		blocks, errs := pl.NormalizeBuilder(raw, siteID, "builder_payload")
		if len(errs) > 0 {
			return errs
		}
		data["builder_payload"] = blocks
	}
	return pl.checkPagePublication(data, stored)
}

// checkPagePublication needs no database: content for publishing and a date for
// scheduling (the builder payload is already normalized). Publishing without a date
// stamps one.
func (pl *Plugin) checkPagePublication(data map[string]any, stored *Page) error {
	status, _ := data["status"].(string)
	if status == "" && stored != nil {
		status = stored.Status
	}
	pageType, _ := data["type"].(string)
	if pageType == "" && stored != nil {
		pageType = stored.Type
	}
	if status != "published" && status != "scheduled" {
		return nil
	}
	if pageType == "builder" {
		var blocks []Block
		switch v := data["builder_payload"].(type) {
		case []Block:
			blocks = v
		case nil:
			if _, sent := data["builder_payload"]; !sent && stored != nil {
				blocks = stored.BuilderPayload.V
			}
		}
		if !pl.hasContent(blocks) {
			return mks.ValidationErrors{"builder_payload": "برای انتشار، دست‌کم یک بلوک با محتوا لازم است."}
		}
	} else {
		var content string
		if v, sent := data["content"]; sent {
			content, _ = v.(string)
		} else if stored != nil && stored.Content != nil {
			content = *stored.Content
		}
		if strings.TrimSpace(content) == "" {
			return mks.ValidationErrors{"content": "برای انتشار، محتوای برگه لازم است."}
		}
	}

	publishedAt, sent := data["published_at"]
	if !sent && stored != nil {
		publishedAt = stored.PublishedAt
	}
	if status == "scheduled" {
		if emptyDate(publishedAt) {
			return mks.ValidationErrors{"published_at": "برای انتشار زمان‌بندی‌شده، تاریخ و ساعت لازم است."}
		}
		return nil
	}
	if !sent && (stored == nil || stored.PublishedAt == nil) {
		data["published_at"] = time.Now().Truncate(time.Second) // MySQL rounds fractions up: never stamp a moment that is still ahead
	}
	return nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func emptyDate(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case *time.Time:
		return v == nil
	case time.Time:
		return v.IsZero()
	default:
		return false
	}
}

// publicPageAvailable: what the public may see at now.
func publicPageAvailable(status string, publishedAt *time.Time, now time.Time) bool {
	switch status {
	case "published":
		return publishedAt == nil || !publishedAt.After(now)
	case "scheduled":
		return publishedAt != nil && !publishedAt.After(now)
	default:
		return false
	}
}
