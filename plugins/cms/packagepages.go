package cms

import (
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"strings"
	"time"

	"github.com/goravel/framework/contracts/http"

	mks "github.com/MiranSalehi/mks"
)

// ManagePlugins: install, update, roll back, activate, deactivate and delete plugin ZIP
// files (MKSine's plugin manager, a super-admin page). Plugins bring templates and
// JavaScript that run on the site, so give it to full administrators only.
const ManagePlugins Ability = "cms.plugins.manage"

// uploadPackage answers POST /plugins/cms/packages: a theme or plugin ZIP (multipart
// "file", "kind" theme|plugin, "update" and "force" 1|0).
func (pl *Plugin) uploadPackage(ctx http.Context) http.Response {
	hc := pl.panel.Context(ctx)
	kind := ctx.Request().Input("kind")
	switch {
	case kind == KindTheme && pl.canInstallThemes(hc), kind == KindPlugin && pl.can(hc, ManagePlugins):
	default:
		return pl.panel.Forbidden(ctx)
	}
	file, err := ctx.Request().File("file")
	if err != nil {
		return ctx.Response().Json(422, http.Json{"message": "فایل ZIP را انتخاب کنید."})
	}
	if size, err := file.Size(); err != nil || size > MaxPackageSize {
		return ctx.Response().Json(422, http.Json{"message": fmt.Sprintf("فایل بزرگ‌تر از %d مگابایت است.", MaxPackageSize>>20)})
	}
	f, err := os.Open(file.File())
	if err != nil {
		return ctx.Response().Json(500, http.Json{"message": "فایل خوانده نشد."})
	}
	data, err := io.ReadAll(io.LimitReader(f, MaxPackageSize+1))
	f.Close()
	if err != nil {
		return ctx.Response().Json(500, http.Json{"message": "فایل خوانده نشد."})
	}
	update, force := ctx.Request().InputBool("update"), ctx.Request().InputBool("force")
	row, err := pl.installPackage(kind, data, update, force)
	if err != nil {
		if msg, ok := mks.RejectMessage(err); ok {
			return ctx.Response().Json(422, http.Json{"message": msg})
		}
		pl.logf("cms: install %s: %v", kind, err)
		return ctx.Response().Json(500, http.Json{"message": "نصب انجام نشد."})
	}
	what := map[string]string{KindTheme: "قالب", KindPlugin: "افزونه"}[kind]
	msg := fmt.Sprintf("%s «%s» نسخهٔ %s نصب شد.", what, row.PackageID, row.Version)
	if update {
		msg = fmt.Sprintf("%s «%s» به نسخهٔ %s به‌روزرسانی شد.", what, row.PackageID, row.Version)
	}
	if kind == KindPlugin && row.Status == StatusActive {
		pl.bumpAll()
	}
	if kind == KindTheme {
		pl.bumpAll()
	}
	return ctx.Response().Json(200, http.Json{"message": msg})
}

// handleThemeInstall: the theme actions that change installed files.
func (pl *Plugin) handleThemeInstall(ctx *mks.PageContext, action string) (*mks.ActionResult, error) {
	if !pl.canInstallThemes(ctx.HookContext) {
		return nil, mks.Reject("اجازهٔ این کار را ندارید.")
	}
	id, _ := ctx.Data["id"].(string)
	switch action {
	case "rollback":
		res, err := pl.rollbackPackage(KindTheme, id)
		if err == nil {
			pl.bumpAll()
		}
		return res, err
	case "delete":
		row, err := pl.packageRow(KindTheme, id)
		if err != nil {
			return nil, err
		}
		if name, used, err := pl.themeInUse(id); err != nil {
			return nil, err
		} else if used {
			return nil, mks.Reject(fmt.Sprintf("این قالب در سایت «%s» فعال است؛ اول قالب دیگری را فعال کنید.", name))
		}
		if err := pl.deletePackage(row); err != nil {
			return nil, err
		}
		return mks.Success("قالب حذف شد."), nil
	}
	return nil, mks.Reject("این کار شناخته نشد.")
}

// pluginsPage is the "plugins" page: the installed plugin ZIPs and what to do with them.
func (pl *Plugin) pluginsPage() mks.Page {
	return mks.Page{
		Name:      "plugins",
		Authorize: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManagePlugins) },
		Data: func(*mks.WidgetContext) (any, error) {
			var rows []Package
			if err := pl.query().Model(&Package{}).Where("kind = ?", KindPlugin).Order("package_id").Find(&rows); err != nil {
				return nil, err
			}
			list := []map[string]any{}
			counts := map[string]int{StatusInstalled: 0, StatusActive: 0, StatusInactive: 0}
			for _, r := range rows {
				counts[r.Status]++
				m := r.Manifest.V
				list = append(list, map[string]any{
					"id": r.PackageID, "name": m["name"], "description": m["description"], "author": m["author"],
					"version": r.Version, "previous_version": r.PreviousVersion, "status": r.Status, "requires": m["requires"],
					"boot_error": r.BootError, "log": r.Log.V, "installed_at": r.InstalledAt, "activated_at": r.ActivatedAt,
					"blocks": len(asList(m["blocks"])), "actions": len(asList(m["actions"])), "shortcodes": len(asList(m["shortcodes"])), "settings": len(asList(m["settings"])), "content_types": len(asList(m["content_types"])),
				})
			}
			return map[string]any{"plugins": list, "counts": counts}, nil
		},
		Handle: func(ctx *mks.PageContext) (*mks.ActionResult, error) {
			action, _ := ctx.Data["action"].(string)
			if action == "scan" {
				// Re-read the installed packages now (a ZIP may have been placed by hand).
				pl.packagesChanged()
				pl.bumpAll()
				return mks.Success("افزونه‌ها شناسایی شدند."), nil
			}
			id, _ := ctx.Data["id"].(string)
			row, err := pl.packageRow(KindPlugin, id)
			if err != nil {
				return nil, err
			}
			switch action {
			case "activate":
				return pl.activatePlugin(row)
			case "deactivate":
				return pl.deactivatePlugin(row)
			case "rollback":
				res, err := pl.rollbackPackage(KindPlugin, id)
				if err == nil {
					pl.bumpAll()
				}
				return res, err
			case "clear_log":
				if _, err := pl.query().Model(&Package{}).Where("id = ?", row.ID).Update("log", mks.NewJSON([]PackageLogEntry{})); err != nil {
					return nil, err
				}
				return mks.Success("گزارش پاک شد."), nil
			case "delete":
				if row.Status == StatusActive {
					return nil, mks.Reject("اول افزونه را غیرفعال کنید.")
				}
				if data, _ := ctx.Data["delete_data"].(bool); data {
					if _, err := pl.query().Exec("DELETE FROM "+SettingsTable+" WHERE name LIKE ?", "plugin:"+row.PackageID+":%"); err != nil {
						return nil, err
					}
					// Its content types' entries too (MKSine's uninstall drops a plugin's
					// tables), unless the type is in code or another plugin has it.
					for _, key := range pl.ownedContentTypes(row) {
						if _, err := pl.query().Exec("DELETE FROM "+EntriesTable+" WHERE content_type = ?", key); err != nil {
							return nil, err
						}
					}
				}
				if err := pl.deletePackage(row); err != nil {
					return nil, err
				}
				return mks.Success("افزونه حذف شد."), nil
			}
			return nil, mks.Reject("این کار شناخته نشد.")
		},
	}
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func (pl *Plugin) activatePlugin(row *Package) (*mks.ActionResult, error) {
	if row.Status == StatusActive {
		return mks.Success("افزونه فعال است."), nil
	}
	m := row.Manifest.V
	for _, req := range asList(m["requires"]) {
		dep, _ := req.(string)
		if !pl.pluginActive(dep) {
			return nil, mks.Reject(fmt.Sprintf("این افزونه به افزونهٔ «%s» نیاز دارد؛ اول آن را نصب و فعال کنید.", dep))
		}
	}
	// It must start before it is turned on.
	if _, err := pl.loadPlugin(*row); err != nil {
		pl.logPackage(row, "error", "فعال نشد: "+err.Error())
		return nil, mks.Reject("افزونه راه‌اندازی نشد: " + err.Error())
	}
	now := time.Now()
	if _, err := pl.query().Model(&Package{}).Where("id = ?", row.ID).Update(map[string]any{
		"status": StatusActive, "activated_at": now, "boot_error": nil, "boot_failed_at": nil,
		"log": mks.NewJSON(appendLog(row.Log.V, "info", "فعال شد.")),
	}); err != nil {
		return nil, err
	}
	pl.packagesChanged()
	pl.bumpAll()
	return mks.Success("افزونه فعال شد."), nil
}

func (pl *Plugin) deactivatePlugin(row *Package) (*mks.ActionResult, error) {
	if row.Status != StatusActive {
		return mks.Success("افزونه غیرفعال است."), nil
	}
	// Not while an active plugin or a theme in use needs it (MKSine blocks it too).
	var rows []Package
	if err := pl.query().Model(&Package{}).Find(&rows); err != nil {
		return nil, err
	}
	for _, other := range rows {
		if other.ID == row.ID || (other.Kind == KindPlugin && other.Status != StatusActive) {
			continue
		}
		for _, req := range asList(other.Manifest.V["requires"]) {
			if req != row.PackageID {
				continue
			}
			if other.Kind == KindPlugin {
				return nil, mks.Reject(fmt.Sprintf("افزونهٔ فعال «%s» به این افزونه نیاز دارد.", other.PackageID))
			}
			if name, used, err := pl.themeInUse(other.PackageID); err != nil {
				return nil, err
			} else if used {
				return nil, mks.Reject(fmt.Sprintf("قالب «%s» در سایت «%s» به این افزونه نیاز دارد.", other.PackageID, name))
			}
		}
	}
	now := time.Now()
	if _, err := pl.query().Model(&Package{}).Where("id = ?", row.ID).Update(map[string]any{
		"status": StatusInactive, "deactivated_at": now, "log": mks.NewJSON(appendLog(row.Log.V, "info", "غیرفعال شد.")),
	}); err != nil {
		return nil, err
	}
	pl.packagesChanged()
	pl.bumpAll()
	return mks.Success("افزونه غیرفعال شد."), nil
}

func (pl *Plugin) pluginActive(id string) bool {
	for _, lp := range pl.activePlugins() {
		if lp.id == id {
			return true
		}
	}
	return false
}

// missingRequirements are the plugins a theme needs that are not active.
func (pl *Plugin) missingRequirements(t Theme) []string {
	var out []string
	for _, r := range t.Requires {
		if !pl.pluginActive(r) {
			out = append(out, r)
		}
	}
	return out
}

// servePackageFile answers {site}/_packages/{kind}/{id}/{version}/{file}: a file of an
// installed package's assets folder (or its screenshot).
func (pl *Plugin) servePackageFile(ctx http.Context, rest string) http.Response {
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) != 4 || (parts[0] != KindTheme && parts[0] != KindPlugin) {
		return notFound(ctx)
	}
	kind, id, version, file := parts[0], parts[1], parts[2], parts[3]
	clean, ok := cleanPackagePath(file)
	if !ok || !packageIDRE.MatchString(id) || !packageVersionRE.MatchString(version) {
		return notFound(ctx)
	}
	row, err := pl.packageRow(kind, id)
	if err != nil || (row.Version != version && (row.PreviousVersion == nil || *row.PreviousVersion != version)) {
		return notFound(ctx)
	}
	screenshot, _ := row.Manifest.V["screenshot"].(string)
	if !strings.HasPrefix(clean, "assets/") && clean != screenshot {
		return notFound(ctx)
	}
	d := pl.packagesDriver()
	if d == nil {
		return notFound(ctx)
	}
	body, err := d.Get(packageDir(kind, id, version) + "/" + clean)
	if err != nil {
		return notFound(ctx)
	}
	ext := strings.ToLower(path.Ext(clean))
	ctype := mime.TypeByExtension(ext)
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	if ext == ".svg" {
		ctx.Response().Header("Content-Security-Policy", "script-src 'none'")
	}
	ctx.Response().Header("X-Content-Type-Options", "nosniff")
	ctx.Response().Header("Cache-Control", "public, max-age=31536000, immutable")
	return ctx.Response().Data(200, ctype, []byte(body))
}

// bumpAll empties the page cache of every site (a theme or plugin changed).
func (pl *Plugin) bumpAll() {
	seen := map[uint]bool{}
	for _, table := range []string{SettingsTable, PagesTable, PostsTable} {
		var rows []struct{ SiteID uint }
		if err := pl.query().Table(table).Select("DISTINCT site_id").Scan(&rows); err != nil {
			pl.logf("cms: sites for cache: %v", err)
			continue
		}
		for _, r := range rows {
			if !seen[r.SiteID] {
				seen[r.SiteID] = true
				pl.bump(r.SiteID)
			}
		}
	}
}

// ownedContentTypes are the content type keys only this plugin declares.
func (pl *Plugin) ownedContentTypes(row *Package) []string {
	others := map[string]bool{}
	for _, t := range pl.codeContentTypes() {
		others[t.Key] = true
	}
	var rows []Package
	if err := pl.query().Model(&Package{}).Where("kind = ? AND id <> ?", KindPlugin, row.ID).Find(&rows); err == nil {
		for _, r := range rows {
			for _, k := range manifestTypeKeys(r.Manifest.V) {
				others[k] = true
			}
		}
	}
	var out []string
	for _, k := range manifestTypeKeys(row.Manifest.V) {
		if !others[k] {
			out = append(out, k)
		}
	}
	return out
}

func manifestTypeKeys(m map[string]any) []string {
	var out []string
	for _, t := range asList(m["content_types"]) {
		if obj, ok := t.(map[string]any); ok {
			if k, ok := obj["key"].(string); ok && contentTypeKey.MatchString(k) {
				out = append(out, k)
			}
		}
	}
	return out
}
