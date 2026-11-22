package cms

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"

	mks "github.com/MiranSalehi/mks"
)

const previewTTL = 10 * time.Minute

func previewCacheKey(token string) string { return "cms.page.preview." + token }

// previewPage makes a link that shows an unpublished page for ten minutes, to the user
// who asked and only while they belong to the page's site.
func (pl *Plugin) previewPage(ctx *mks.ActionContext[Page]) (*mks.ActionResult, error) {
	if ctx == nil || !pl.can(ctx.HookContext, ManagePages) {
		return nil, mks.Reject("You are not allowed to preview pages.")
	}
	if ctx.Query == nil || len(ctx.Records) != 1 || ctx.Records[0] == nil {
		return nil, mks.Reject("Select exactly one page to preview.")
	}
	selected := ctx.Records[0]
	if selected.ID == 0 || selected.SiteID == 0 || selected.DeletedAt != nil {
		return nil, mks.Reject("Invalid selected page.")
	}
	var page Page
	if err := ctx.Query.Model(&Page{}).
		Where("site_id = ? AND id = ? AND deleted_at IS NULL", selected.SiteID, selected.ID).
		First(&page); err != nil {
		return nil, fmt.Errorf("find page for preview: %w", err)
	}
	if page.ID != selected.ID || page.SiteID != selected.SiteID || page.DeletedAt != nil {
		return nil, mks.Reject("The selected page was not found on the active site.")
	}
	site, err := pl.Sites.ByID(page.SiteID)
	if err != nil {
		return nil, fmt.Errorf("find page site for preview: %w", err)
	}
	if site == nil || strings.TrimSpace(site.Slug) == "" {
		return nil, mks.Reject("Invalid public page site.")
	}
	user := ctx.UserID()
	if user == nil || fmt.Sprint(user) == "" {
		return nil, mks.Reject("Previewing needs a signed-in user.")
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate page preview token: %w", err)
	}
	token := hex.EncodeToString(raw)
	grant, err := json.Marshal(previewGrant{UserID: fmt.Sprint(user), SiteID: page.SiteID, PageID: page.ID})
	if err != nil {
		return nil, fmt.Errorf("encode page preview grant: %w", err)
	}
	if err := pl.Cache.Put(previewCacheKey(token), string(grant), previewTTL); err != nil {
		return nil, fmt.Errorf("store page preview grant: %w", err)
	}
	result := mks.Success("Page preview is ready.")
	result.OpenURL = pl.PageURL(site.Slug, page.Slug) + "?preview=" + url.QueryEscape(token)
	result.Invalidate = []string{"pages"}
	return result, nil
}

// checkPreview resolves a preview token for this request: the grant must exist, the
// logged-in panel user must be the one it was made for, may still manage pages, and must
// still belong to the page's site.
func (pl *Plugin) checkPreview(ctx http.Context, token string) (*previewGrant, bool, error) {
	decoded, err := hex.DecodeString(token)
	if err != nil || len(decoded) != 32 {
		return nil, false, nil
	}
	raw, ok := pl.Cache.Get(previewCacheKey(token)).(string)
	var grant previewGrant
	if !ok || raw == "" || json.Unmarshal([]byte(raw), &grant) != nil || grant.UserID == "" || grant.SiteID == 0 || grant.PageID == 0 {
		return nil, false, nil
	}
	if ctx == nil || pl.panel == nil || !ctx.Request().HasSession() {
		return nil, false, nil
	}
	hc := pl.panel.Context(ctx)
	if hc == nil || hc.User == nil || fmt.Sprint(hc.UserID()) != grant.UserID || !pl.can(hc, ManagePages) {
		return nil, false, nil
	}
	if !hc.BelongsTo(grant.SiteID) {
		return nil, false, nil
	}
	return &grant, true, nil
}
