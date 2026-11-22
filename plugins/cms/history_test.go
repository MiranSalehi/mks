package cms

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/goravel/framework/contracts/database/db"
	frameworkerrors "github.com/goravel/framework/errors"
	ormmock "github.com/goravel/framework/mocks/database/orm"
	"github.com/stretchr/testify/mock"

	mks "github.com/MiranSalehi/mks"
)

func TestPageRevisionsResourceConfiguration(t *testing.T) {
	c := testPlugin().pageRevisionsResource().Config()
	if c.Name != "page_revisions" || c.Tenant != "site_id" || c.DefaultSort != "-id" || c.TitleColumn != "id" {
		t.Fatalf("unexpected revision configuration: %#v", c)
	}
	if c.Deletable || len(c.Fillable) != 0 || c.GlobalSearch {
		t.Fatalf("history must not expose mutable fields or global search: %#v", c)
	}
	if !reflect.DeepEqual(c.Sortable, []string{"id", "created_at"}) || !reflect.DeepEqual(c.Filterable, map[string][]string{"page_id": {"eq"}}) {
		t.Fatalf("unexpected history query allowlists: %#v", c)
	}
}

func TestPageRevisionRestoreAuthorizationIsExplicit(t *testing.T) {
	for _, role := range []string{"admin", "editor", "viewer", "", "owner", "ADMIN"} {
		t.Run(role, func(t *testing.T) {
			ctx := &mks.HookContext{User: &testUser{Role: role}}
			want := role == "admin" || role == "editor"
			if got := testPlugin().can(ctx, ManagePages); got != want {
				t.Fatalf("role %q restore permission = %v, want %v", role, got, want)
			}
			if !want {
				if result, err := testPlugin().restorePageRevision(&mks.ActionContext[PageRevision]{HookContext: ctx}); result != nil || err == nil {
					t.Fatalf("unauthorized direct handler = (%#v, %v), want rejection", result, err)
				}
			}
		})
	}
	if testPlugin().can(nil, ManagePages) || testPlugin().can(&mks.HookContext{}, ManagePages) {
		t.Fatal("missing actors must not restore revisions")
	}
	if result, err := testPlugin().restorePageRevision(nil); result != nil || err == nil {
		t.Fatalf("nil action context = (%#v, %v), want rejection", result, err)
	}
}

func TestPageRevisionRestoreRequiresOneSavedRevision(t *testing.T) {
	revision := historyTestRevision(historyTestPage())
	for _, records := range [][]*PageRevision{nil, {}, {nil}, {revision, revision}, {{}}, {{SiteID: 7, PageID: 9}}} {
		q := &ormmock.Query{}
		ctx := historyTestContext(q, records...)
		if result, err := testPlugin().restorePageRevision(ctx); result != nil || err == nil {
			t.Fatalf("restore records %#v = (%#v, %v), want rejection", records, result, err)
		}
		q.AssertNotCalled(t, "Model", mock.Anything)
		q.AssertNotCalled(t, "Create", mock.Anything)
	}
}

func TestPageSnapshotContainsOnlyVersionedFields(t *testing.T) {
	page := historyTestPage()
	snapshot := historyJSONMap(t, pageSnapshot(&page))
	wantKeys := []string{"title", "slug", "type", "status", "content", "builder_payload", "show_page_header", "builder_content_width", "meta_title", "meta_description", "focus_keyphrase", "published_at"}
	if len(snapshot) != len(wantKeys) {
		t.Fatalf("snapshot contains unexpected fields: %#v", snapshot)
	}
	for _, key := range wantKeys {
		if _, ok := snapshot[key]; !ok {
			t.Fatalf("snapshot is missing %q", key)
		}
	}
	if snapshot["title"] != page.Title || snapshot["slug"] != page.Slug || snapshot["content"] != *page.Content {
		t.Fatalf("snapshot did not retain page content: %#v", snapshot)
	}
}

func TestPageRevisionContentRejectsMalformedSnapshots(t *testing.T) {
	cases := []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing field", func(s map[string]any) { delete(s, "show_page_header") }},
		{"null title", func(s map[string]any) { s["title"] = nil }},
		{"blank title", func(s map[string]any) { s["title"] = " \t" }},
		{"long title", func(s map[string]any) { s["title"] = strings.Repeat("آ", 256) }},
		{"wrong title type", func(s map[string]any) { s["title"] = 12 }},
		{"unknown page type", func(s map[string]any) { s["type"] = "script" }},
		{"null header", func(s map[string]any) { s["show_page_header"] = nil }},
		{"string header", func(s map[string]any) { s["show_page_header"] = "true" }},
		{"invalid width", func(s map[string]any) { s["builder_content_width"] = "infinite" }},
		{"object content", func(s map[string]any) { s["content"] = map[string]any{"html": "x"} }},
		{"number content", func(s map[string]any) { s["content"] = 12 }},
		{"invalid meta title", func(s map[string]any) { s["meta_title"] = true }},
		{"long meta title", func(s map[string]any) { s["meta_title"] = strings.Repeat("x", 256) }},
		{"long description", func(s map[string]any) { s["meta_description"] = strings.Repeat("x", 501) }},
		{"long keyphrase", func(s map[string]any) { s["focus_keyphrase"] = strings.Repeat("x", 256) }},
		{"object builder", func(s map[string]any) { s["builder_payload"] = map[string]any{"type": "heading"} }},
		{"unserializable content", func(s map[string]any) { s["content"] = make(chan int) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page := historyTestPage()
			snapshot := pageSnapshot(&page)
			tc.change(snapshot)
			if _, err := pageRevisionContent(snapshot); err == nil {
				t.Fatalf("malformed snapshot accepted: %#v", snapshot)
			}
		})
	}
	if _, err := pageRevisionContent(nil); err == nil {
		t.Fatal("nil snapshot accepted")
	}
}

func TestPageRevisionContentSanitizesAndIgnoresUnversionedFields(t *testing.T) {
	page := historyTestPage()
	page.Content = historyString(`<p onclick="alert(1)">Safe</p><script>alert(2)</script><a href="javascript:alert(3)">link</a>`)
	snapshot := pageSnapshot(&page)
	snapshot["site_id"], snapshot["page_id"], snapshot["id"] = 999, 888, 777
	snapshot["created_by"], snapshot["updated_by"], snapshot["deleted_at"] = 1, 2, "2026-01-01"
	restored, err := pageRevisionContent(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Content == nil || !strings.Contains(*restored.Content, "Safe") || strings.Contains(*restored.Content, "alert") || strings.Contains(*restored.Content, "onclick") {
		t.Fatalf("unsafe HTML survived restore: %#v", restored.Content)
	}
	data := restored.changes()
	if len(data) != 9 {
		t.Fatalf("expected nine versioned fields, got %#v", data)
	}
	for _, key := range []string{"slug", "status", "published_at", "id", "site_id", "page_id", "created_by", "updated_by", "deleted_at"} {
		if _, exists := data[key]; exists {
			t.Fatalf("unversioned field %q entered restore changes", key)
		}
	}
	// Decoding and sanitization must not mutate the saved historical snapshot.
	if snapshot["content"] != page.Content || !strings.Contains(*page.Content, "onclick") {
		t.Fatal("decoding modified the original historical content")
	}
}

func TestPageRevisionRestoreUsesStoredSnapshotAndPreservesPublication(t *testing.T) {
	for _, state := range []string{"draft", "published", "published_without_date", "scheduled", "builder"} {
		t.Run(state, func(t *testing.T) {
			current := historyTestPage()
			current.Slug, current.Status = "current-url", "published"
			current.CreatedBy = userID(uint(44))
			date := time.Date(2026, 1, 2, 10, 20, 0, 0, time.UTC)
			current.PublishedAt = &date
			if state == "draft" || state == "published_without_date" {
				current.PublishedAt = nil
			}
			if state == "draft" || state == "scheduled" {
				current.Status = state
			}
			historical := historyTestPage()
			historical.Title, historical.Slug, historical.Status = "Restored title", "old-url", "draft"
			historical.Content = historyString(`<p onclick="bad()">Restored content</p><script>bad()</script>`)
			if state == "builder" {
				historical.Type = "builder"
				historical.BuilderPayload = mks.NewJSON([]Block{{Type: "heading", Data: map[string]any{"text": "Historical heading"}}})
			}
			revision := historyTestRevision(historical)
			// Exercise the actual JSON-decoded storage shape, including untrusted extra keys.
			revision.Snapshot.V = historyJSONMap(t, revision.Snapshot.V)
			revision.Snapshot.V["id"], revision.Snapshot.V["site_id"] = 500, 600
			revision.Snapshot.V["deleted_at"], revision.Snapshot.V["created_by"] = "2026-01-01", 700
			before := historyJSONMap(t, revision)

			q := &ormmock.Query{}
			historyExpectLookup(q, revision, &current, nil)
			q.On("Model", mock.MatchedBy(func(p *Page) bool { return p.ID == current.ID })).Return(q).Once()
			q.On("Where", "site_id = ? AND id = ?", revision.SiteID, revision.PageID).Return(q).Once()
			q.On("Where", "deleted_at IS NULL").Return(q).Once()
			q.On("Update", mock.Anything).Return(&db.Result{RowsAffected: 1}, nil).Once().Run(func(args mock.Arguments) {
				data := args.Get(0).(map[string]any)
				if data["title"] != historical.Title || data["content"] != "<p>Restored content</p>" || data["updated_by"] != uint(52) {
					t.Fatalf("incorrect restored fields or actor: %#v", data)
				}
				for _, key := range []string{"slug", "status", "published_at", "id", "site_id", "page_id", "created_by", "deleted_at"} {
					if _, exists := data[key]; exists {
						t.Fatalf("restore attempted to update protected field %q", key)
					}
				}
				// The stored payload, cleaned like a new save (defaults of the style settings).
				want, errs := testPlugin().NormalizeBuilder(historical.BuilderPayload.V, revision.SiteID, "builder_payload")
				blocks, ok := data["builder_payload"].(mks.JSON[[]Block])
				if len(errs) > 0 || !ok || !reflect.DeepEqual(blocks.V, want) {
					t.Fatalf("builder update = %#v, want normalized payload %#v", data["builder_payload"], want)
				}
			})
			q.On("Create", mock.AnythingOfType("*cms.PageRevision")).Return(nil).Once().Run(func(args mock.Arguments) {
				saved := args.Get(0).(*PageRevision)
				if saved.ID != 0 || saved.PageID != current.ID || saved.SiteID != current.SiteID || saved.UserID == nil || *saved.UserID != 52 {
					t.Fatalf("incorrect appended revision identity: %#v", saved)
				}
				snapshot := historyJSONMap(t, saved.Snapshot.V)
				if snapshot["slug"] != current.Slug || snapshot["status"] != current.Status || snapshot["title"] != historical.Title || snapshot["content"] != "<p>Restored content</p>" {
					t.Fatalf("appended revision does not describe the restored page: %#v", snapshot)
				}
				if !reflect.DeepEqual(snapshot["published_at"], historyJSONMap(t, pageSnapshot(&current))["published_at"]) {
					t.Fatalf("restore changed publication date: %#v", snapshot["published_at"])
				}
			})
			ctx := historyTestContext(q, revision)
			ctx.Data = map[string]any{
				"snapshot": map[string]any{"title": "Submitted forgery"}, "title": "Submitted title",
				"site_id": 999, "page_id": 999, "slug": "injected-url", "status": "draft",
			}
			result, err := testPlugin().restorePageRevision(ctx)
			if err != nil || result == nil || !reflect.DeepEqual(result.Invalidate, []string{"pages", "page_revisions"}) {
				t.Fatalf("restore = (%#v, %v)", result, err)
			}
			if !reflect.DeepEqual(before, historyJSONMap(t, revision)) {
				t.Fatal("restore mutated the original revision")
			}
			q.AssertExpectations(t)
		})
	}
}

func TestPageRevisionRestoreRejectsMissingOrMismatchedTarget(t *testing.T) {
	for _, state := range []string{"not_found", "empty_record", "different_team", "different_page", "database_error"} {
		t.Run(state, func(t *testing.T) {
			page := historyTestPage()
			revision := historyTestRevision(page)
			var lookupErr error
			switch state {
			case "not_found":
				lookupErr = frameworkerrors.OrmRecordNotFound
			case "empty_record":
				page.ID = 0
			case "different_team":
				page.SiteID++
			case "different_page":
				page.ID++
			case "database_error":
				lookupErr = errors.New("lookup failed")
			}
			q := &ormmock.Query{}
			historyExpectLookup(q, revision, &page, lookupErr)
			result, err := testPlugin().restorePageRevision(historyTestContext(q, revision))
			if result != nil || err == nil {
				t.Fatalf("missing/mismatched page restore = (%#v, %v)", result, err)
			}
			if lookupErr != nil && !errors.Is(lookupErr, frameworkerrors.OrmRecordNotFound) && !errors.Is(err, lookupErr) {
				t.Fatalf("lookup failure was not preserved: %v", err)
			}
			q.AssertNotCalled(t, "Update", mock.Anything)
			q.AssertNotCalled(t, "Create", mock.Anything)
			q.AssertExpectations(t)
		})
	}
}

func TestPageRevisionRestoreValidatesBeforeWriting(t *testing.T) {
	for _, state := range []string{"malformed_snapshot", "empty_published_content", "unsafe_only_published_content", "empty_published_builder", "scheduled_without_date"} {
		t.Run(state, func(t *testing.T) {
			page := historyTestPage()
			page.Status = "published"
			revision := historyTestRevision(page)
			switch state {
			case "malformed_snapshot":
				revision.Snapshot.V["title"] = map[string]any{"invalid": true}
			case "empty_published_content":
				revision.Snapshot.V["content"] = nil
			case "unsafe_only_published_content":
				revision.Snapshot.V["content"] = "<script>unsafe()</script>"
			case "empty_published_builder":
				revision.Snapshot.V["type"] = "builder"
				revision.Snapshot.V["builder_payload"] = []any{}
			case "scheduled_without_date":
				page.Status = "scheduled"
				page.PublishedAt = nil
			}
			q := &ormmock.Query{}
			historyExpectLookup(q, revision, &page, nil)
			if result, err := testPlugin().restorePageRevision(historyTestContext(q, revision)); result != nil || err == nil {
				t.Fatalf("invalid content restore = (%#v, %v), want rejection", result, err)
			}
			q.AssertNotCalled(t, "Update", mock.Anything)
			q.AssertNotCalled(t, "Create", mock.Anything)
			q.AssertExpectations(t)
		})
	}
}

func TestPageRevisionRestorePropagatesTransactionalWriteFailures(t *testing.T) {
	for _, state := range []string{"update", "revision"} {
		t.Run(state, func(t *testing.T) {
			page := historyTestPage()
			revision := historyTestRevision(page)
			q := &ormmock.Query{}
			historyExpectLookup(q, revision, &page, nil)
			q.On("Model", mock.MatchedBy(func(p *Page) bool { return p.ID == page.ID })).Return(q).Once()
			q.On("Where", "site_id = ? AND id = ?", revision.SiteID, revision.PageID).Return(q).Once()
			q.On("Where", "deleted_at IS NULL").Return(q).Once()
			failure := errors.New("transactional write failed")
			if state == "update" {
				q.On("Update", mock.Anything).Return((*db.Result)(nil), failure).Once()
			} else {
				q.On("Update", mock.Anything).Return(&db.Result{RowsAffected: 1}, nil).Once()
				q.On("Create", mock.AnythingOfType("*cms.PageRevision")).Return(failure).Once()
			}
			result, err := testPlugin().restorePageRevision(historyTestContext(q, revision))
			if result != nil || !errors.Is(err, failure) {
				t.Fatalf("restore write failure = (%#v, %v), want original error for action rollback", result, err)
			}
			if state == "update" {
				q.AssertNotCalled(t, "Create", mock.Anything)
			}
			q.AssertExpectations(t)
		})
	}
}

func TestSavePageRevisionUsesCallerTransactionAndActor(t *testing.T) {
	page := historyTestPage()
	q := &ormmock.Query{}
	q.On("Create", mock.AnythingOfType("*cms.PageRevision")).Return(nil).Once().Run(func(args mock.Arguments) {
		revision := args.Get(0).(*PageRevision)
		if revision.ID != 0 || revision.SiteID != page.SiteID || revision.PageID != page.ID || revision.UserID == nil || *revision.UserID != 52 {
			t.Fatalf("incorrect revision identity or actor: %#v", revision)
		}
		if !reflect.DeepEqual(historyJSONMap(t, revision.Snapshot.V), historyJSONMap(t, pageSnapshot(&page))) {
			t.Fatalf("revision does not match saved page: %#v", revision.Snapshot.V)
		}
	})
	if err := savePageRevision(historyTestContext(q).HookContext, &page); err != nil {
		t.Fatal(err)
	}
	q.AssertExpectations(t)
}

func TestSavePageRevisionRejectsMissingTransactionOrIdentity(t *testing.T) {
	page := historyTestPage()
	if err := savePageRevision(nil, &page); err == nil {
		t.Fatal("saving history without context must fail")
	}
	if err := savePageRevision(&mks.HookContext{}, &page); err == nil {
		t.Fatal("saving history without transaction must fail")
	}
	q := &ormmock.Query{}
	for _, page := range []*Page{nil, {}, {SiteID: 7}} {
		if err := savePageRevision(historyTestContext(q).HookContext, page); err == nil {
			t.Fatalf("saving invalid page %#v must fail", page)
		}
	}
	q.AssertNotCalled(t, "Create", mock.Anything)
}

func historyTestPage() Page {
	page := Page{
		SiteID: 7, Title: "Original title", Slug: "original-url", Type: "simple", Status: "draft",
		Content: historyString("<p>Original content</p>"), ShowPageHeader: true,
		BuilderPayload: mks.NewJSON([]Block{}), BuilderContentWidth: "contained",
		MetaTitle: historyString("SEO title"), MetaDescription: historyString("SEO description"), FocusKeyphrase: historyString("keyword"),
	}
	page.ID = 9
	return page
}

func historyTestRevision(page Page) *PageRevision {
	revision := &PageRevision{SiteID: page.SiteID, PageID: page.ID, Snapshot: mks.NewJSON(pageSnapshot(&page))}
	revision.ID = 123
	return revision
}

func historyTestContext(q *ormmock.Query, records ...*PageRevision) *mks.ActionContext[PageRevision] {
	actor := &testUser{Role: "editor"}
	actor.ID = 52
	return &mks.ActionContext[PageRevision]{
		HookContext: &mks.HookContext{Query: q, User: actor}, Records: records,
	}
}

func historyExpectLookup(q *ormmock.Query, revision *PageRevision, page *Page, err error) {
	q.On("Model", mock.MatchedBy(func(p *Page) bool { return p.ID == 0 })).Return(q).Once()
	q.On("Where", "site_id = ? AND id = ?", revision.SiteID, revision.PageID).Return(q).Once()
	q.On("Where", "deleted_at IS NULL").Return(q).Once()
	q.On("LockForUpdate").Return(q).Once()
	q.On("First", mock.AnythingOfType("*cms.Page")).Return(err).Once().Run(func(args mock.Arguments) {
		if err == nil {
			*args.Get(0).(*Page) = *page
		}
	})
}

func historyJSONMap(t *testing.T, value any) map[string]any {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func historyString(value string) *string { return &value }
