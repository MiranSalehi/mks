package cms

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/orm"
	frameworkerrors "github.com/goravel/framework/errors"

	mks "github.com/MiranSalehi/mks"
)

func (pl *Plugin) pageTrashResource() mks.ModelResource[PageTrash] {
	return mks.Define[PageTrash](mks.Config{
		Name:        "page_trash",
		Tenant:      "site_id",
		Sortable:    []string{"id", "title", "slug", "deleted_at"},
		Searchable:  []string{"title", "slug"},
		TitleColumn: "title",
		DefaultSort: "-deleted_at",
	}).Policy(pl.pageTrashPolicy()).Actions(pl.pageTrashActions()...)
}

func (pl *Plugin) pageTrashPolicy() mks.Policy[PageTrash] {
	return mks.Policy[PageTrash]{
		ViewAny: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManagePages) },
		Create:  func(*mks.HookContext) bool { return false },
		Update:  func(*mks.HookContext, *PageTrash) bool { return false },
		Delete:  func(*mks.HookContext, *PageTrash) bool { return false },
		Scope: func(_ *mks.HookContext, q orm.Query) orm.Query {
			return q.Where("deleted_at IS NOT NULL")
		},
	}
}

func (pl *Plugin) pageTrashActions() []mks.Action[PageTrash] {
	return []mks.Action[PageTrash]{
		{
			Name:      "restore",
			Authorize: func(ctx *mks.HookContext, _ *PageTrash) bool { return pl.can(ctx, ManagePages) },
			Handle:    bumping(pl, pl.restoreTrashedPage),
		},
		{
			Name: "purge",
			Authorize: func(ctx *mks.HookContext, _ *PageTrash) bool {
				return pl.can(ctx, PurgePages)
			},
			Handle: bumping(pl, pl.purgeTrashedPage),
		},
	}
}

func (pl *Plugin) pageActions() []mks.Action[Page] {
	return []mks.Action[Page]{
		{
			Name: "duplicate",
			Authorize: func(ctx *mks.HookContext, _ *Page) bool {
				return pl.can(ctx, ManagePages)
			},
			Handle: bumping(pl, pl.duplicatePage),
		},
		{
			Name: "trash",
			Authorize: func(ctx *mks.HookContext, _ *Page) bool {
				return pl.can(ctx, ManagePages)
			},
			Handle: bumping(pl, pl.trashPage),
		},
		{
			Name: "apply_template",
			Authorize: func(ctx *mks.HookContext, _ *Page) bool {
				return pl.can(ctx, ManagePages)
			},
			Rules:  map[string]string{"template": "required"},
			Handle: bumping(pl, pl.applyTemplate),
		},
		{
			Name: "preview",
			Authorize: func(ctx *mks.HookContext, _ *Page) bool {
				return pl.can(ctx, ManagePages)
			},
			Handle: bumping(pl, pl.previewPage),
		},
	}
}

func (pl *Plugin) duplicatePage(ctx *mks.ActionContext[Page]) (*mks.ActionResult, error) {
	if ctx == nil || !pl.can(ctx.HookContext, ManagePages) {
		return nil, mks.Reject("You are not allowed to copy pages.")
	}
	if ctx.Query == nil || len(ctx.Records) != 1 || ctx.Records[0] == nil {
		return nil, mks.Reject("Select exactly one page to copy.")
	}
	original := ctx.Records[0]
	if original.ID == 0 || original.SiteID == 0 || original.DeletedAt != nil {
		return nil, mks.Reject("Invalid selected page.")
	}
	var locked Page
	err := ctx.Query.Model(&Page{}).
		Where("site_id = ? AND id = ? AND deleted_at IS NULL", original.SiteID, original.ID).
		LockForUpdate().First(&locked)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return nil, mks.Reject("Selected page not found.")
	}
	if err != nil {
		return nil, fmt.Errorf("lock page for duplicate: %w", err)
	}
	if locked.ID != original.ID || locked.SiteID != original.SiteID || locked.DeletedAt != nil {
		return nil, mks.Reject("Selected page not found.")
	}

	// Actions bypass normal form sanitization. Validate and sanitize the locked
	// content, including legacy HTML, without mutating the source or its history.
	fields, err := pageRevisionContent(pageSnapshot(&locked))
	if err != nil {
		return nil, err
	}
	blocks, errs := pl.NormalizeBuilder(fields.BuilderPayload, locked.SiteID, "builder_payload")
	if len(errs) > 0 {
		return nil, mks.Reject("Invalid page block content.")
	}
	fields.BuilderPayload = blocks
	slug, err := availablePageCopySlug(ctx, locked.SiteID, locked.Slug)
	if err != nil {
		return nil, err
	}
	const titleSuffix = " (copy)"
	title := []rune(fields.Title)
	if len(title) > 255-len(titleSuffix) {
		title = title[:255-len(titleSuffix)]
	}
	copy := Page{
		SiteID: locked.SiteID, Slug: slug, Status: "draft",
		CreatedBy: userID(ctx.UserID()), UpdatedBy: userID(ctx.UserID()),
	}
	fields.apply(&copy)
	copy.Title = string(title) + titleSuffix
	if err := ctx.Query.Create(&copy); err != nil {
		return nil, fmt.Errorf("create duplicate page: %w", err)
	}
	if err := savePageRevision(ctx.HookContext, &copy); err != nil {
		return nil, err
	}
	result := mks.Success("Copied draft created.")
	result.Redirect = fmt.Sprintf("/pages/%d", copy.ID)
	result.Invalidate = []string{"pages"}
	return result, nil
}

func availablePageCopySlug(ctx *mks.ActionContext[Page], siteID uint, originalSlug string) (string, error) {
	base := strings.TrimRight(originalSlug, "-")
	if base == "" {
		base = "page"
	}
	for number := 1; number < 100000; number++ {
		suffix := "-copy"
		if number > 1 {
			suffix = fmt.Sprintf("-copy-%d", number)
		}
		// Reserve room for the complete suffix before truncating the source slug.
		candidate := trimSlug(base, 255-len(suffix)) + suffix
		// Page.DeletedAt is a plain timestamp, so this tenant-local lookup also
		// reserves slugs belonging to trashed pages, matching the unique index.
		count, err := ctx.Query.Model(&Page{}).
			Where("site_id = ? AND slug = ?", siteID, candidate).
			Count()
		if err != nil {
			return "", fmt.Errorf("check duplicate page slug: %w", err)
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", mks.Reject("Could not build a unique slug for the page copy.")
}

func trimSlug(value string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(value) > max {
		// Slugs normally contain ASCII; keep legacy Unicode intact as well.
		for max > 0 && !utf8.RuneStart(value[max]) {
			max--
		}
		value = value[:max]
	}
	return strings.TrimRight(value, "-")
}

func (pl *Plugin) trashPage(ctx *mks.ActionContext[Page]) (*mks.ActionResult, error) {
	if ctx == nil || !pl.can(ctx.HookContext, ManagePages) {
		return nil, mks.Reject("You are not allowed to move pages to the trash.")
	}
	if ctx.Query == nil || len(ctx.Records) != 1 || ctx.Records[0] == nil {
		return nil, mks.Reject("Select exactly one page to move to the trash.")
	}
	page := ctx.Records[0]
	if page.ID == 0 || page.SiteID == 0 || page.DeletedAt != nil {
		return nil, mks.Reject("Invalid selected page.")
	}
	deletedAt := time.Now()
	updated, err := ctx.Query.Model(&Page{}).
		Where("site_id = ? AND id = ? AND deleted_at IS NULL", page.SiteID, page.ID).
		Update(map[string]any{"deleted_at": deletedAt, "updated_by": userID(ctx.UserID())})
	if err != nil {
		return nil, fmt.Errorf("trash page: %w", err)
	}
	if updated == nil || updated.RowsAffected != 1 {
		return nil, mks.Reject("The selected page was not found or it changed.")
	}
	result := mks.Success("Page moved to the trash.")
	result.Redirect = "/page_trash"
	result.Invalidate = []string{"pages", "page_trash"}
	return result, nil
}

func (pl *Plugin) purgeTrashedPage(ctx *mks.ActionContext[PageTrash]) (*mks.ActionResult, error) {
	if ctx == nil || !pl.can(ctx.HookContext, PurgePages) {
		return nil, mks.Reject("You are not allowed to permanently delete pages.")
	}
	if ctx.Query == nil || len(ctx.Records) != 1 || ctx.Records[0] == nil {
		return nil, mks.Reject("Select exactly one page to delete permanently.")
	}
	trashed := ctx.Records[0]
	if trashed.ID == 0 || trashed.SiteID == 0 || trashed.DeletedAt == nil {
		return nil, mks.Reject("The selected page is not in the trash.")
	}
	var current PageTrash
	err := ctx.Query.Model(&PageTrash{}).
		Where("site_id = ? AND id = ? AND deleted_at IS NOT NULL", trashed.SiteID, trashed.ID).
		LockForUpdate().First(&current)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return nil, mks.Reject("Page not found in the trash.")
	}
	if err != nil {
		return nil, fmt.Errorf("lock trashed page for purge: %w", err)
	}
	if current.ID != trashed.ID || current.DeletedAt == nil || current.SiteID != trashed.SiteID {
		return nil, mks.Reject("Page not found in the trash.")
	}
	deleted, err := ctx.Query.Model(&PageTrash{}).
		Where("site_id = ? AND id = ? AND deleted_at IS NOT NULL", current.SiteID, current.ID).
		ForceDelete(&current)
	if err != nil {
		return nil, fmt.Errorf("purge trashed page: %w", err)
	}
	if deleted == nil || deleted.RowsAffected != 1 {
		return nil, mks.Reject("The page was not found in the trash, or it changed.")
	}
	if err := dropAttachments(ctx.Query, OwnerPage, current.ID); err != nil {
		return nil, fmt.Errorf("purge page attachments: %w", err)
	}
	result := mks.Success("The page and its history were permanently deleted.")
	result.Redirect = "/page_trash"
	result.Invalidate = []string{"pages", "page_trash", "page_revisions"}
	return result, nil
}

func (pl *Plugin) restoreTrashedPage(ctx *mks.ActionContext[PageTrash]) (*mks.ActionResult, error) {
	if ctx == nil || !pl.can(ctx.HookContext, ManagePages) {
		return nil, mks.Reject("You are not allowed to restore pages.")
	}
	if ctx.Query == nil || len(ctx.Records) != 1 || ctx.Records[0] == nil {
		return nil, mks.Reject("Select exactly one page to restore.")
	}
	trashed := ctx.Records[0]
	if trashed.ID == 0 || trashed.SiteID == 0 || trashed.DeletedAt == nil {
		return nil, mks.Reject("The selected page is not in the trash.")
	}
	var current PageTrash
	err := ctx.Query.Model(&PageTrash{}).
		Where("site_id = ? AND id = ? AND deleted_at IS NOT NULL", trashed.SiteID, trashed.ID).
		LockForUpdate().First(&current)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) {
		return nil, mks.Reject("Page not found in the trash.")
	}
	if err != nil {
		return nil, fmt.Errorf("lock trashed page: %w", err)
	}
	if current.ID != trashed.ID || current.DeletedAt == nil || current.SiteID != trashed.SiteID {
		return nil, mks.Reject("Page not found in the trash.")
	}
	// Keep the slug, content and publication metadata: restoring resumes the
	// previous publication state, including a scheduled date that has now passed.
	updated, err := ctx.Query.Model(&Page{}).
		Where("site_id = ? AND id = ? AND deleted_at IS NOT NULL", current.SiteID, current.ID).
		Update(map[string]any{"deleted_at": nil, "updated_by": userID(ctx.UserID())})
	if err != nil {
		return nil, fmt.Errorf("restore trashed page: %w", err)
	}
	if updated == nil || updated.RowsAffected != 1 {
		return nil, mks.Reject("The page was not found in the trash, or it changed.")
	}
	result := mks.Success("Page restored.")
	result.Redirect = "/pages"
	result.Invalidate = []string{"pages", "page_trash"}
	return result, nil
}
