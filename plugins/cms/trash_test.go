package cms

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/db"
	frameworkerrors "github.com/goravel/framework/errors"
	ormmock "github.com/goravel/framework/mocks/database/orm"
	"github.com/stretchr/testify/mock"

	mks "github.com/MiranSalehi/mks"
)

func TestPageTrashResourceDeniesCRUDAndExplicitlyAuthorizesActions(t *testing.T) {
	c := testPlugin().pageTrashResource().Config()
	if c.Name != "page_trash" || c.Tenant != "site_id" || c.DefaultSort != "-deleted_at" || c.TitleColumn != "title" {
		t.Fatalf("unexpected trash configuration: %#v", c)
	}
	if c.Deletable || len(c.Fillable) != 0 || c.GlobalSearch {
		t.Fatalf("trash must expose only explicit lifecycle actions: %#v", c)
	}
	if !reflect.DeepEqual(c.Sortable, []string{"id", "title", "slug", "deleted_at"}) || !reflect.DeepEqual(c.Searchable, []string{"title", "slug"}) {
		t.Fatalf("unexpected trash query allowlists: %#v", c)
	}
	policy := testPlugin().pageTrashPolicy()
	if policy.Create == nil || policy.Update == nil || policy.Delete == nil || policy.ViewAny == nil || policy.Scope == nil {
		t.Fatal("trash CRUD and read scope must be explicit")
	}
	for _, actorRole := range []string{"admin", "editor", "viewer", "owner", "ADMIN", ""} {
		t.Run(actorRole, func(t *testing.T) {
			ctx := actionsTestContext[PageTrash](nil, actorRole).HookContext
			manage := actorRole == "admin" || actorRole == "editor"
			if policy.Create(ctx) || policy.Update(ctx, nil) || policy.Delete(ctx, nil) {
				t.Fatal("trash CRUD must be denied even to admins")
			}
			if policy.ViewAny(ctx) != manage {
				t.Fatalf("role %q has incorrect trash visibility", actorRole)
			}
			for _, action := range testPlugin().pageTrashActions() {
				want := manage
				if action.Name == "purge" {
					want = actorRole == "admin"
				}
				if action.Authorize == nil || action.Authorize(ctx, nil) != want {
					t.Fatalf("role %q action %q authorization differs from %v", actorRole, action.Name, want)
				}
				if !want {
					result, err := action.Handle(&mks.ActionContext[PageTrash]{HookContext: ctx})
					actionsRequireRejection(t, result, err)
				}
			}
			for _, action := range testPlugin().pageActions() {
				if action.Authorize == nil || action.Authorize(ctx, nil) != manage {
					t.Fatalf("role %q action %q authorization differs from %v", actorRole, action.Name, manage)
				}
				if !manage {
					result, err := action.Handle(&mks.ActionContext[Page]{HookContext: ctx})
					actionsRequireRejection(t, result, err)
				}
			}
		})
	}
	q := &ormmock.Query{}
	q.On("Where", "deleted_at IS NOT NULL").Return(q).Once()
	if policy.Scope(nil, q) != q {
		t.Fatal("trash scope discarded the caller query")
	}
	q.AssertExpectations(t)
}

func TestPageLifecycleRejectsMissingActors(t *testing.T) {
	for _, hook := range []*mks.HookContext{nil, {}, {User: (*testUser)(nil)}} {
		if testPlugin().can(hook, ManagePages) || testPlugin().pageTrashPolicy().ViewAny(hook) {
			t.Fatal("missing actor may not manage pages")
		}
		for _, action := range testPlugin().pageActions() {
			if action.Authorize(hook, nil) {
				t.Fatalf("missing actor may run %s", action.Name)
			}
			result, err := action.Handle(&mks.ActionContext[Page]{HookContext: hook})
			actionsRequireRejection(t, result, err)
			result, err = action.Handle(nil)
			actionsRequireRejection(t, result, err)
		}
		for _, action := range testPlugin().pageTrashActions() {
			if action.Authorize(hook, nil) {
				t.Fatalf("missing actor may run %s", action.Name)
			}
			result, err := action.Handle(&mks.ActionContext[PageTrash]{HookContext: hook})
			actionsRequireRejection(t, result, err)
			result, err = action.Handle(nil)
			actionsRequireRejection(t, result, err)
		}
	}
}

func TestPageActionsRequireOneSavedActivePageAndQuery(t *testing.T) {
	page := actionsTestPage()
	deleted := page
	now := time.Now()
	deleted.DeletedAt = &now
	noID, noTeam := page, page
	noID.ID, noTeam.SiteID = 0, 0
	for _, action := range testPlugin().pageActions() {
		if action.Name == "preview" {
			continue // Preview does not mutate pages and has separate handler tests.
		}
		for _, records := range [][]*Page{nil, {}, {nil}, {&page, &page}, {&noID}, {&noTeam}, {&deleted}} {
			q := &ormmock.Query{}
			result, err := action.Handle(actionsTestContext(q, "admin", records...))
			actionsRequireRejection(t, result, err)
			q.AssertNotCalled(t, "Model", mock.Anything)
		}
		result, err := action.Handle(actionsTestContext(nil, "admin", &page))
		actionsRequireRejection(t, result, err)
	}
}

func TestTrashActionsRequireOneSavedTrashedPageAndQuery(t *testing.T) {
	page := actionsTestTrash()
	active, noID, noTeam := page, page, page
	active.DeletedAt, noID.ID, noTeam.SiteID = nil, 0, 0
	for _, action := range testPlugin().pageTrashActions() {
		for _, records := range [][]*PageTrash{nil, {}, {nil}, {&page, &page}, {&noID}, {&noTeam}, {&active}} {
			q := &ormmock.Query{}
			result, err := action.Handle(actionsTestContext(q, "admin", records...))
			actionsRequireRejection(t, result, err)
			q.AssertNotCalled(t, "Model", mock.Anything)
		}
		result, err := action.Handle(actionsTestContext(nil, "admin", &page))
		actionsRequireRejection(t, result, err)
	}
}

func TestPageCopySlugsPreserveCompleteSuffixWithinLimit(t *testing.T) {
	cases := []struct {
		name       string
		source     string
		candidates []string
	}{
		{"ordinary", "about", []string{"about-copy", "about-copy-2", "about-copy-3"}},
		{"empty legacy slug", "", []string{"page-copy"}},
		{"hyphens only", "---", []string{"page-copy"}},
		{"trailing hyphens", "about---", []string{"about-copy", "about-copy-2"}},
		{"hyphen at truncation", strings.Repeat("a", 249) + "-extra", []string{strings.Repeat("a", 249) + "-copy"}},
		{"unicode boundary", strings.Repeat("é", 128), []string{strings.Repeat("é", 125) + "-copy", strings.Repeat("é", 124) + "-copy-2"}},
		{"maximum slug and two digit collisions", strings.Repeat("a", 255), nil},
	}
	for number := 1; number <= 12; number++ {
		suffix, prefixLength := "-copy", 250
		if number > 1 {
			suffix = fmt.Sprintf("-copy-%d", number)
			prefixLength = 255 - len(suffix)
		}
		cases[len(cases)-1].candidates = append(cases[len(cases)-1].candidates, strings.Repeat("a", prefixLength)+suffix)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			q := &ormmock.Query{}
			for i, candidate := range tc.candidates {
				if len(candidate) > 255 || !utf8.ValidString(candidate) {
					t.Fatalf("invalid expected candidate %q", candidate)
				}
				q.On("Model", mock.AnythingOfType("*cms.Page")).Return(q).Once()
				// No deleted_at filter: a trashed page reserves its slug too. The
				// exact team condition also prevents other tenants reserving it.
				q.On("Where", "site_id = ? AND slug = ?", uint(7), candidate).Return(q).Once()
				count := int64(1)
				if i == len(tc.candidates)-1 {
					count = 0
				}
				q.On("Count").Return(count, nil).Once()
			}
			slug, err := availablePageCopySlug(actionsTestContext[Page](q, "editor"), 7, tc.source)
			if err != nil || slug != tc.candidates[len(tc.candidates)-1] {
				t.Fatalf("copy slug = (%q, %v), want %q", slug, err, tc.candidates[len(tc.candidates)-1])
			}
			q.AssertExpectations(t)
		})
	}
}

func TestDuplicatePageUsesLockedContentAndResetsPublication(t *testing.T) {
	for _, pageType := range []string{"simple", "builder"} {
		t.Run(pageType, func(t *testing.T) {
			original := actionsTestPage()
			original.Title, original.Slug = "Stale title", "stale-url"
			locked := actionsTestPage()
			locked.Title, locked.Slug, locked.Type = "Current title", "current-url", pageType
			locked.Status, locked.ShowPageHeader, locked.BuilderContentWidth = "published", false, "wide"
			if pageType == "builder" {
				locked.Status = "scheduled"
			}
			date := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
			locked.PublishedAt = &date
			locked.Content = actionsString(`<p onclick="bad()">Current content</p><script>bad()</script>`)
			locked.BuilderPayload = mks.NewJSON([]Block{{Type: "heading", Data: map[string]any{"text": "Current heading"}}})
			locked.MetaTitle, locked.MetaDescription, locked.FocusKeyphrase = actionsString("Current SEO"), actionsString("Current description"), actionsString("Current phrase")
			locked.CreatedBy, locked.UpdatedBy = userID(uint(10)), userID(uint(11))
			before := historyJSONMap(t, locked)
			q := &ormmock.Query{}
			actionsExpectPageLock(q, &original, &locked, nil)
			actionsExpectSlug(q, locked.SiteID, "current-url-copy", 0, nil)
			var saved Page
			q.On("Create", mock.AnythingOfType("*cms.Page")).Return(nil).Once().Run(func(args mock.Arguments) {
				copy := args.Get(0).(*Page)
				if copy.ID != 0 || copy.SiteID != locked.SiteID || copy.Slug != "current-url-copy" || copy.Title != "Current title (copy)" {
					t.Fatalf("copy identity/title/slug = %#v", copy)
				}
				if copy.Status != "draft" || copy.PublishedAt != nil || copy.DeletedAt != nil || len(copy.Revisions) != 0 {
					t.Fatalf("copy retained lifecycle metadata: %#v", copy)
				}
				if copy.CreatedBy == nil || *copy.CreatedBy != 52 || copy.UpdatedBy == nil || *copy.UpdatedBy != 52 {
					t.Fatalf("copy retained original actors: %#v", copy)
				}
				if copy.Type != pageType || copy.ShowPageHeader || copy.BuilderContentWidth != "wide" || copy.Content == nil || *copy.Content != "<p>Current content</p>" {
					t.Fatalf("copy did not use sanitized locked content/layout: %#v", copy)
				}
				wantBlocks, _ := testPlugin().NormalizeBuilder(locked.BuilderPayload.V, locked.SiteID, "builder_payload")
				if !reflect.DeepEqual(copy.BuilderPayload.V, wantBlocks) || !reflect.DeepEqual(copy.MetaTitle, locked.MetaTitle) || !reflect.DeepEqual(copy.MetaDescription, locked.MetaDescription) || !reflect.DeepEqual(copy.FocusKeyphrase, locked.FocusKeyphrase) {
					t.Fatalf("copy did not use locked builder/SEO fields: %#v", copy)
				}
				copy.ID = 101
				saved = *copy
			})
			// Both creates must go through this exact action transaction query.
			q.On("Create", mock.AnythingOfType("*cms.PageRevision")).Return(nil).Once().Run(func(args mock.Arguments) {
				revision := args.Get(0).(*PageRevision)
				if saved.ID == 0 || revision.ID != 0 || revision.PageID != saved.ID || revision.SiteID != saved.SiteID || revision.UserID == nil || *revision.UserID != 52 {
					t.Fatalf("revision identity/transaction order = %#v", revision)
				}
				if !reflect.DeepEqual(historyJSONMap(t, revision.Snapshot.V), historyJSONMap(t, pageSnapshot(&saved))) {
					t.Fatalf("revision does not describe the saved draft: %#v", revision)
				}
			})
			ctx := actionsTestContext(q, "editor", &original)
			ctx.Data = map[string]any{"title": "Forged title", "site_id": 999, "status": "published", "slug": "forged-url"}
			result, err := testPlugin().duplicatePage(ctx)
			if err != nil || result == nil || result.Redirect != "/pages/101" || !reflect.DeepEqual(result.Invalidate, []string{"pages"}) {
				t.Fatalf("duplicate = (%#v, %v)", result, err)
			}
			if !reflect.DeepEqual(before, historyJSONMap(t, locked)) || original.Title != "Stale title" {
				t.Fatal("duplicate mutated its source records")
			}
			q.AssertExpectations(t)
		})
	}
}

func TestDuplicatePageTruncatesTitleWithoutLosingCopySuffix(t *testing.T) {
	page := actionsTestPage()
	page.Title = strings.Repeat("é", 255)
	q := &ormmock.Query{}
	actionsExpectPageLock(q, &page, &page, nil)
	actionsExpectSlug(q, page.SiteID, page.Slug+"-copy", 0, nil)
	q.On("Create", mock.AnythingOfType("*cms.Page")).Return(nil).Once().Run(func(args mock.Arguments) {
		copy := args.Get(0).(*Page)
		if copy.Title != strings.Repeat("é", 248)+" (copy)" || utf8.RuneCountInString(copy.Title) != 255 {
			t.Fatalf("incorrect title truncation: %q", copy.Title)
		}
		copy.ID = 101
	})
	q.On("Create", mock.AnythingOfType("*cms.PageRevision")).Return(nil).Once()
	if result, err := testPlugin().duplicatePage(actionsTestContext(q, "admin", &page)); err != nil || result == nil {
		t.Fatalf("duplicate = (%#v, %v)", result, err)
	}
	q.AssertExpectations(t)
}

func TestDuplicatePageRejectsMissingChangedOrDeletedLockedRow(t *testing.T) {
	for _, state := range []string{"not_found", "wrapped_not_found", "empty", "wrong_id", "wrong_team", "deleted", "database_error"} {
		t.Run(state, func(t *testing.T) {
			original, locked := actionsTestPage(), actionsTestPage()
			var lookupErr error
			switch state {
			case "not_found":
				lookupErr = frameworkerrors.OrmRecordNotFound
			case "wrapped_not_found":
				lookupErr = fmt.Errorf("wrapped: %w", frameworkerrors.OrmRecordNotFound)
			case "empty":
				locked = Page{}
			case "wrong_id":
				locked.ID++
			case "wrong_team":
				locked.SiteID++
			case "deleted":
				now := time.Now()
				locked.DeletedAt = &now
			case "database_error":
				lookupErr = errors.New("lock failed")
			}
			q := &ormmock.Query{}
			actionsExpectPageLock(q, &original, &locked, lookupErr)
			result, err := testPlugin().duplicatePage(actionsTestContext(q, "editor", &original))
			actionsRequireRejection(t, result, err)
			actionsCheckLookupError(t, lookupErr, err)
			q.AssertNotCalled(t, "Count")
			q.AssertNotCalled(t, "Create", mock.Anything)
			q.AssertExpectations(t)
		})
	}
}

func TestDuplicatePageRejectsInvalidLegacyContent(t *testing.T) {
	for _, state := range []string{"blank_title", "invalid_type", "invalid_width", "invalid_builder"} {
		t.Run(state, func(t *testing.T) {
			original, locked := actionsTestPage(), actionsTestPage()
			switch state {
			case "blank_title":
				locked.Title = " "
			case "invalid_type":
				locked.Type = "script"
			case "invalid_width":
				locked.BuilderContentWidth = "infinite"
			case "invalid_builder":
				// A block whose required text is missing cannot be cleaned up.
				locked.BuilderPayload = mks.NewJSON([]Block{{Type: "heading", Data: map[string]any{"text": ""}}})
			}
			q := &ormmock.Query{}
			actionsExpectPageLock(q, &original, &locked, nil)
			result, err := testPlugin().duplicatePage(actionsTestContext(q, "editor", &original))
			actionsRequireRejection(t, result, err)
			q.AssertNotCalled(t, "Count")
			q.AssertNotCalled(t, "Create", mock.Anything)
			q.AssertExpectations(t)
		})
	}
}

func TestDuplicatePagePropagatesSlugAndTransactionalWriteFailures(t *testing.T) {
	for _, state := range []string{"slug_query", "unique_constraint", "revision", "create_without_id"} {
		t.Run(state, func(t *testing.T) {
			page := actionsTestPage()
			q := &ormmock.Query{}
			actionsExpectPageLock(q, &page, &page, nil)
			failure := errors.New("transaction failed: " + state)
			if state == "slug_query" {
				actionsExpectSlug(q, page.SiteID, page.Slug+"-copy", 0, failure)
			} else {
				actionsExpectSlug(q, page.SiteID, page.Slug+"-copy", 0, nil)
				if state == "unique_constraint" {
					// A concurrent insert may win after Count. Propagate the DB
					// error; retrying must not duplicate a title or page content.
					q.On("Create", mock.AnythingOfType("*cms.Page")).Return(failure).Once()
				} else {
					q.On("Create", mock.AnythingOfType("*cms.Page")).Return(nil).Once().Run(func(args mock.Arguments) {
						if state != "create_without_id" {
							args.Get(0).(*Page).ID = 101
						}
					})
					if state == "revision" {
						q.On("Create", mock.AnythingOfType("*cms.PageRevision")).Return(failure).Once()
					}
				}
			}
			result, err := testPlugin().duplicatePage(actionsTestContext(q, "editor", &page))
			actionsRequireRejection(t, result, err)
			if state != "create_without_id" && !errors.Is(err, failure) {
				t.Fatalf("failure not preserved for transaction rollback: %v", err)
			}
			q.AssertExpectations(t)
		})
	}
}

func TestTrashPageScopesMutationAndChecksAffectedRows(t *testing.T) {
	for _, state := range []string{"success", "no_rows", "nil_result", "multiple_rows", "database_error"} {
		t.Run(state, func(t *testing.T) {
			page := actionsTestPage()
			before := historyJSONMap(t, page)
			q := &ormmock.Query{}
			q.On("Model", mock.AnythingOfType("*cms.Page")).Return(q).Once()
			q.On("Where", "site_id = ? AND id = ? AND deleted_at IS NULL", page.SiteID, page.ID).Return(q).Once()
			outcome, failure := actionsWriteOutcome(state)
			start := time.Now()
			q.On("Update", mock.Anything).Return(outcome, failure).Once().Run(func(args mock.Arguments) {
				data := args.Get(0).(map[string]any)
				deletedAt, ok := data["deleted_at"].(time.Time)
				if len(data) != 2 || !ok || deletedAt.Before(start) || deletedAt.After(time.Now()) || !reflect.DeepEqual(data["updated_by"], userID(uint(52))) {
					t.Fatalf("trash changed protected content or incorrect actor/timestamp: %#v", data)
				}
			})
			result, err := testPlugin().trashPage(actionsTestContext(q, "editor", &page))
			if state == "success" {
				if err != nil || result == nil || result.Redirect != "/page_trash" || !reflect.DeepEqual(result.Invalidate, []string{"pages", "page_trash"}) {
					t.Fatalf("trash = (%#v, %v)", result, err)
				}
			} else {
				actionsRequireRejection(t, result, err)
				if failure != nil && !errors.Is(err, failure) {
					t.Fatalf("trash database failure not preserved: %v", err)
				}
			}
			if !reflect.DeepEqual(before, historyJSONMap(t, page)) {
				t.Fatal("trash mutated server-loaded record")
			}
			q.AssertExpectations(t)
		})
	}
}

func TestTrashLifecycleRejectsMissingOrChangedLockedRows(t *testing.T) {
	for _, action := range testPlugin().pageTrashActions() {
		for _, state := range []string{"not_found", "wrapped_not_found", "empty", "wrong_id", "wrong_team", "active", "database_error"} {
			t.Run(action.Name+"/"+state, func(t *testing.T) {
				requested, current := actionsTestTrash(), actionsTestTrash()
				var lookupErr error
				switch state {
				case "not_found":
					lookupErr = frameworkerrors.OrmRecordNotFound
				case "wrapped_not_found":
					lookupErr = fmt.Errorf("wrapped: %w", frameworkerrors.OrmRecordNotFound)
				case "empty":
					current = PageTrash{}
				case "wrong_id":
					current.ID++
				case "wrong_team":
					current.SiteID++
				case "active":
					current.DeletedAt = nil
				case "database_error":
					lookupErr = errors.New("lock failed")
				}
				q := &ormmock.Query{}
				actionsExpectTrashLock(q, &requested, &current, lookupErr)
				result, err := action.Handle(actionsTestContext(q, "admin", &requested))
				actionsRequireRejection(t, result, err)
				actionsCheckLookupError(t, lookupErr, err)
				q.AssertNotCalled(t, "Update", mock.Anything)
				q.AssertNotCalled(t, "ForceDelete", mock.Anything)
				q.AssertExpectations(t)
			})
		}
	}
}

func TestRestoreTrashedPagePreservesCurrentContentSlugAndPublication(t *testing.T) {
	for _, state := range []string{"draft", "published", "published_without_date", "scheduled_past", "scheduled_future"} {
		t.Run(state, func(t *testing.T) {
			requested, current := actionsTestTrash(), actionsTestTrash()
			current.Title, current.Slug, current.Status = "Latest title", "latest-url", "published"
			date := time.Now().Add(-time.Hour)
			current.PublishedAt = &date
			switch state {
			case "draft":
				current.Status, current.PublishedAt = "draft", nil
			case "published_without_date":
				current.PublishedAt = nil
			case "scheduled_past", "scheduled_future":
				current.Status = "scheduled"
				if state == "scheduled_future" {
					date = time.Now().Add(time.Hour)
				}
			}
			before := historyJSONMap(t, current)
			q := &ormmock.Query{}
			actionsExpectTrashLock(q, &requested, &current, nil)
			q.On("Model", mock.AnythingOfType("*cms.Page")).Return(q).Once()
			q.On("Where", "site_id = ? AND id = ? AND deleted_at IS NOT NULL", current.SiteID, current.ID).Return(q).Once()
			q.On("Update", mock.Anything).Return(&db.Result{RowsAffected: 1}, nil).Once().Run(func(args mock.Arguments) {
				data := args.Get(0).(map[string]any)
				if !reflect.DeepEqual(data, map[string]any{"deleted_at": nil, "updated_by": userID(uint(52))}) {
					t.Fatalf("restore changed slug, content, or publication: %#v", data)
				}
			})
			ctx := actionsTestContext(q, "editor", &requested)
			ctx.Data = map[string]any{"status": "draft", "published_at": nil, "title": "Forged", "slug": "forged", "site_id": 999}
			result, err := testPlugin().restoreTrashedPage(ctx)
			if err != nil || result == nil || result.Redirect != "/pages" || !reflect.DeepEqual(result.Invalidate, []string{"pages", "page_trash"}) {
				t.Fatalf("restore = (%#v, %v)", result, err)
			}
			if !reflect.DeepEqual(before, historyJSONMap(t, current)) {
				t.Fatal("restore modified the locked record in memory")
			}
			q.AssertNotCalled(t, "Create", mock.Anything)
			q.AssertExpectations(t)
		})
	}
}

func TestPurgeTrashedPageDeletesFreshLockedRow(t *testing.T) {
	requested, current := actionsTestTrash(), actionsTestTrash()
	current.Title = "Freshly locked title"
	q := &ormmock.Query{}
	actionsExpectTrashLock(q, &requested, &current, nil)
	q.On("Model", mock.AnythingOfType("*cms.PageTrash")).Return(q).Once()
	q.On("Where", "site_id = ? AND id = ? AND deleted_at IS NOT NULL", current.SiteID, current.ID).Return(q).Once()
	q.On("ForceDelete", mock.MatchedBy(func(p *PageTrash) bool { return reflect.DeepEqual(*p, current) })).Return(&db.Result{RowsAffected: 1}, nil).Once()
	q.On("Exec", "DELETE FROM cms_media_attachments WHERE owner_type = ? AND owner_id = ?", OwnerPage, current.ID).Return(&db.Result{RowsAffected: 0}, nil).Once()
	result, err := testPlugin().purgeTrashedPage(actionsTestContext(q, "admin", &requested))
	if err != nil || result == nil || result.Redirect != "/page_trash" || !reflect.DeepEqual(result.Invalidate, []string{"pages", "page_trash", "page_revisions"}) {
		t.Fatalf("purge = (%#v, %v)", result, err)
	}
	q.AssertNotCalled(t, "Update", mock.Anything)
	q.AssertNotCalled(t, "Create", mock.Anything)
	q.AssertExpectations(t)
}

func TestTrashLifecycleRejectsUnappliedMutationsAndDatabaseFailures(t *testing.T) {
	for _, action := range testPlugin().pageTrashActions() {
		for _, state := range []string{"no_rows", "nil_result", "multiple_rows", "database_error"} {
			t.Run(action.Name+"/"+state, func(t *testing.T) {
				page := actionsTestTrash()
				q := &ormmock.Query{}
				actionsExpectTrashLock(q, &page, &page, nil)
				modelType, method := "*cms.Page", "Update"
				if action.Name == "purge" {
					modelType, method = "*cms.PageTrash", "ForceDelete"
				}
				q.On("Model", mock.AnythingOfType(modelType)).Return(q).Once()
				q.On("Where", "site_id = ? AND id = ? AND deleted_at IS NOT NULL", page.SiteID, page.ID).Return(q).Once()
				outcome, failure := actionsWriteOutcome(state)
				q.On(method, mock.Anything).Return(outcome, failure).Once()
				result, err := action.Handle(actionsTestContext(q, "admin", &page))
				actionsRequireRejection(t, result, err)
				if failure != nil && !errors.Is(err, failure) {
					t.Fatalf("database failure not preserved: %v", err)
				}
				q.AssertExpectations(t)
			})
		}
	}
}

func actionsTestPage() Page {
	page := Page{
		SiteID: 7, Title: "Original title", Slug: "original-url", Type: "simple", Status: "published",
		Content: actionsString("<p>Original content</p>"), ShowPageHeader: true,
		BuilderPayload: mks.NewJSON([]Block{}), BuilderContentWidth: "contained",
		MetaTitle: actionsString("SEO title"), MetaDescription: actionsString("SEO description"), FocusKeyphrase: actionsString("keyword"),
	}
	page.ID = 9
	return page
}

func actionsTestTrash() PageTrash {
	page := PageTrash(actionsTestPage())
	deletedAt := time.Date(2026, 11, 19, 9, 0, 0, 0, time.UTC)
	page.DeletedAt = &deletedAt
	return page
}

func actionsTestContext[T any](q *ormmock.Query, actorRole string, records ...*T) *mks.ActionContext[T] {
	actor := &testUser{Role: actorRole}
	actor.ID = 52
	hook := &mks.HookContext{User: actor}
	if q != nil {
		hook.Query = q
	}
	return &mks.ActionContext[T]{HookContext: hook, Records: records}
}

func actionsExpectPageLock(q *ormmock.Query, requested, locked *Page, err error) {
	q.On("Model", mock.AnythingOfType("*cms.Page")).Return(q).Once()
	q.On("Where", "site_id = ? AND id = ? AND deleted_at IS NULL", requested.SiteID, requested.ID).Return(q).Once()
	q.On("LockForUpdate").Return(q).Once()
	q.On("First", mock.AnythingOfType("*cms.Page")).Return(err).Once().Run(func(args mock.Arguments) {
		if err == nil {
			*args.Get(0).(*Page) = *locked
		}
	})
}

func actionsExpectTrashLock(q *ormmock.Query, requested, locked *PageTrash, err error) {
	q.On("Model", mock.AnythingOfType("*cms.PageTrash")).Return(q).Once()
	q.On("Where", "site_id = ? AND id = ? AND deleted_at IS NOT NULL", requested.SiteID, requested.ID).Return(q).Once()
	q.On("LockForUpdate").Return(q).Once()
	q.On("First", mock.AnythingOfType("*cms.PageTrash")).Return(err).Once().Run(func(args mock.Arguments) {
		if err == nil {
			*args.Get(0).(*PageTrash) = *locked
		}
	})
}

func actionsExpectSlug(q *ormmock.Query, siteID uint, slug string, count int64, err error) {
	q.On("Model", mock.AnythingOfType("*cms.Page")).Return(q).Once()
	q.On("Where", "site_id = ? AND slug = ?", siteID, slug).Return(q).Once()
	q.On("Count").Return(count, err).Once()
}

func actionsWriteOutcome(state string) (*db.Result, error) {
	switch state {
	case "nil_result":
		return nil, nil
	case "no_rows":
		return &db.Result{}, nil
	case "multiple_rows":
		return &db.Result{RowsAffected: 2}, nil
	case "database_error":
		return nil, errors.New("database write failed")
	default:
		return &db.Result{RowsAffected: 1}, nil
	}
}

func actionsRequireRejection(t *testing.T, result *mks.ActionResult, err error) {
	t.Helper()
	if result != nil || err == nil {
		t.Fatalf("action = (%#v, %v), want rejection without success", result, err)
	}
}

func actionsCheckLookupError(t *testing.T, original, result error) {
	t.Helper()
	if errors.Is(original, frameworkerrors.OrmRecordNotFound) {
		if reflect.TypeOf(result) != reflect.TypeOf(mks.Reject("")) {
			t.Fatalf("missing row should produce an actionable rejection: %v", result)
		}
	} else if original != nil && !errors.Is(result, original) {
		t.Fatalf("lookup error was not preserved: %v", result)
	}
}

func actionsString(value string) *string { return &value }
