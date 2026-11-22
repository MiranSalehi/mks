package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	frameworkerrors "github.com/goravel/framework/errors"

	mks "github.com/MiranSalehi/mks"
)

// pageRevisionsResource exposes immutable history. Restoring is an explicitly
// authorized action, independent of the deliberately disabled CRUD writes.
func (pl *Plugin) pageRevisionsResource() mks.ModelResource[PageRevision] {
	return mks.Define[PageRevision](mks.Config{
		Name:        "page_revisions",
		Tenant:      "site_id",
		Sortable:    []string{"id", "created_at"},
		Filterable:  map[string][]string{"page_id": {"eq"}},
		DefaultSort: "-id",
		TitleColumn: "id",
	}).Policy(mks.Policy[PageRevision]{
		ViewAny: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManagePages) },
		Create:  func(*mks.HookContext) bool { return false },
		Update:  func(*mks.HookContext, *PageRevision) bool { return false },
		Delete:  func(*mks.HookContext, *PageRevision) bool { return false },
	}).Actions(mks.Action[PageRevision]{
		Name:      "restore",
		Authorize: func(ctx *mks.HookContext, _ *PageRevision) bool { return pl.can(ctx, ManagePages) },
		Handle:    bumping(pl, pl.restorePageRevision),
	})
}

// pageSnapshot retains publication metadata for historical inspection, but restore
// only applies the content, layout and SEO whitelist in pageRevisionContent.
func pageSnapshot(page *Page) map[string]any {
	return map[string]any{
		"title": page.Title, "slug": page.Slug, "type": page.Type, "status": page.Status,
		"content": page.Content, "builder_payload": page.BuilderPayload.V,
		"show_page_header": page.ShowPageHeader, "builder_content_width": page.BuilderContentWidth,
		"meta_title": page.MetaTitle, "meta_description": page.MetaDescription,
		"focus_keyphrase": page.FocusKeyphrase, "published_at": page.PublishedAt,
	}
}

// savePageRevision must use the caller's transaction so a failed history write
// rolls back the page save or restore as well.
func savePageRevision(ctx *mks.HookContext, page *Page) error {
	if ctx == nil || ctx.Query == nil {
		return errors.New("page revision requires the page save transaction")
	}
	if page == nil || page.ID == 0 || page.SiteID == 0 {
		return errors.New("page revision requires a saved page and team")
	}
	return ctx.Query.Create(&PageRevision{
		SiteID:   page.SiteID,
		PageID:   page.ID,
		UserID:   userID(ctx.UserID()),
		Snapshot: mks.NewJSON(pageSnapshot(page)),
	})
}

func (pl *Plugin) restorePageRevision(ctx *mks.ActionContext[PageRevision]) (*mks.ActionResult, error) {
	if ctx == nil || !pl.can(ctx.HookContext, ManagePages) {
		return nil, mks.Reject("You are not allowed to restore page revisions.")
	}
	if len(ctx.Records) != 1 || ctx.Records[0] == nil {
		return nil, mks.Reject("Select exactly one revision to restore.")
	}
	if ctx.Query == nil {
		return nil, errors.New("page restore requires an action transaction")
	}
	revision := ctx.Records[0] // Server-loaded records only; ctx.Data is never used.
	if revision.ID == 0 || revision.SiteID == 0 || revision.PageID == 0 {
		return nil, mks.Reject("Invalid selected revision.")
	}

	var page Page
	err := ctx.Query.Model(&Page{}).
		Where("site_id = ? AND id = ?", revision.SiteID, revision.PageID).
		Where("deleted_at IS NULL").LockForUpdate().First(&page)
	if errors.Is(err, frameworkerrors.OrmRecordNotFound) || (err == nil && page.ID == 0) {
		return nil, mks.Reject("The revision's page was not found on the selected site.")
	}
	if err != nil {
		return nil, fmt.Errorf("find page for revision restore: %w", err)
	}
	if page.ID != revision.PageID || page.SiteID != revision.SiteID {
		return nil, mks.Reject("The revision's page was not found on the selected site.")
	}

	restored, err := pageRevisionContent(revision.Snapshot.V)
	if err != nil {
		return nil, err
	}
	// Old revisions are cleaned like a new save: unknown blocks go, links and media are
	// checked again for this site.
	blocks, errs := pl.NormalizeBuilder(restored.BuilderPayload, revision.SiteID, "builder_payload")
	if len(errs) > 0 {
		return nil, mks.Reject("Invalid selected revision content.")
	}
	restored.BuilderPayload = blocks
	data := restored.changes()
	data["builder_payload"] = blocks
	// Include the current publication timestamp only for validation, so it does not
	// stamp a new date; the slug is not restored, so no slug check is needed.
	data["published_at"] = page.PublishedAt
	if err := pl.checkPagePublication(data, &page); err != nil {
		return nil, err
	}
	delete(data, "published_at")
	data["builder_payload"] = mks.NewJSON(restored.BuilderPayload)
	data["updated_by"] = ctx.UserID()
	updated, err := ctx.Query.Model(&page).
		Where("site_id = ? AND id = ?", revision.SiteID, revision.PageID).
		Where("deleted_at IS NULL").Update(data)
	if err != nil {
		return nil, fmt.Errorf("restore page revision: %w", err)
	}
	if updated == nil || updated.RowsAffected != 1 {
		return nil, mks.Reject("The revision's page changed or is no longer available.")
	}

	// Apply the same validated values in memory for the appended snapshot. Do not
	// Save the whole model: identity, publication and creation metadata stay untouched.
	restored.apply(&page)
	page.UpdatedBy = userID(ctx.UserID())
	if err := savePageRevision(ctx.HookContext, &page); err != nil {
		return nil, err
	}
	result := mks.Success("Page revision restored.")
	result.Invalidate = []string{"pages", "page_revisions"}
	return result, nil
}

// The typed whitelist rejects malformed values rather than silently zeroing
// them, and cannot carry identity, ownership or publication fields into UPDATE.
type pageRevisionFields struct {
	Title               string  `json:"title"`
	Type                string  `json:"type"`
	Content             *string `json:"content"`
	BuilderPayload      []Block `json:"builder_payload"`
	ShowPageHeader      bool    `json:"show_page_header"`
	BuilderContentWidth string  `json:"builder_content_width"`
	MetaTitle           *string `json:"meta_title"`
	MetaDescription     *string `json:"meta_description"`
	FocusKeyphrase      *string `json:"focus_keyphrase"`
}

func pageRevisionContent(snapshot map[string]any) (pageRevisionFields, error) {
	var restored pageRevisionFields
	invalid := func() (pageRevisionFields, error) {
		return pageRevisionFields{}, mks.Reject("Invalid selected revision content.")
	}
	selected := make(map[string]any, 9)
	for _, field := range []string{
		"title", "type", "content", "builder_payload", "show_page_header",
		"builder_content_width", "meta_title", "meta_description", "focus_keyphrase",
	} {
		value, exists := snapshot[field]
		if !exists {
			return invalid()
		}
		selected[field] = value
	}
	// JSON null must not turn required strings/booleans into their zero values.
	for _, field := range []string{"title", "type", "builder_content_width"} {
		if _, ok := selected[field].(string); !ok {
			return invalid()
		}
	}
	if _, ok := selected["show_page_header"].(bool); !ok {
		return invalid()
	}
	encoded, err := json.Marshal(selected)
	if err != nil || json.Unmarshal(encoded, &restored) != nil {
		return invalid()
	}
	if strings.TrimSpace(restored.Title) == "" || utf8.RuneCountInString(restored.Title) > 255 {
		return invalid()
	}
	if restored.Type != "simple" && restored.Type != "builder" {
		return invalid()
	}
	switch restored.BuilderContentWidth {
	case "contained", "wide", "full":
	default:
		return invalid()
	}
	for value, limit := range map[*string]int{restored.MetaTitle: 255, restored.MetaDescription: 500, restored.FocusKeyphrase: 255} {
		if value != nil && utf8.RuneCountInString(*value) > limit {
			return invalid()
		}
	}

	if restored.Content != nil {
		clean := mks.SanitizeHTML(*restored.Content)
		restored.Content = &clean
	}
	return restored, nil
}

func (r pageRevisionFields) changes() map[string]any {
	var content any
	if r.Content != nil {
		content = *r.Content // validatePageFields expects a string, not a pointer.
	}
	return map[string]any{
		"title": r.Title, "type": r.Type, "content": content,
		"builder_payload":  r.BuilderPayload,
		"show_page_header": r.ShowPageHeader, "builder_content_width": r.BuilderContentWidth,
		"meta_title": r.MetaTitle, "meta_description": r.MetaDescription,
		"focus_keyphrase": r.FocusKeyphrase,
	}
}

func (r pageRevisionFields) apply(page *Page) {
	page.Title, page.Type, page.Content = r.Title, r.Type, r.Content
	page.BuilderPayload = mks.NewJSON(r.BuilderPayload)
	page.ShowPageHeader, page.BuilderContentWidth = r.ShowPageHeader, r.BuilderContentWidth
	page.MetaTitle, page.MetaDescription, page.FocusKeyphrase = r.MetaTitle, r.MetaDescription, r.FocusKeyphrase
}
