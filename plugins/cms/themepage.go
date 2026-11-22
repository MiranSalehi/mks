package cms

import (
	"strings"

	mks "github.com/MiranSalehi/mks"
)

// ManageThemes: activate a theme for the site and change its custom CSS (MKSine's theme
// manager page). Custom JavaScript needs ManageCode too.
const ManageThemes Ability = "cms.themes.manage"

// themesPage is the "themes" page: the site's themes, which one is active, and the
// site's custom CSS and JavaScript.
func (pl *Plugin) themesPage() mks.Page {
	return mks.Page{
		Name:      "themes",
		Authorize: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManageThemes) },
		Data: func(ctx *mks.WidgetContext) (any, error) {
			site, err := pl.Sites.ByID(ctx.TenantID())
			if err != nil || site == nil {
				return nil, err
			}
			current := pl.settings(site)
			active := pl.themeByID(current["theme"]).ID
			previous := map[string]string{}
			var rows []Package
			if err := pl.query().Model(&Package{}).Where("kind = ?", KindTheme).Find(&rows); err == nil {
				for _, r := range rows {
					if r.PreviousVersion != nil {
						previous[r.PackageID] = *r.PreviousVersion
					}
				}
			}
			list := []map[string]any{}
			for _, t := range pl.themes() {
				info := pl.themeInfo(pl.themeForSite(t, site), active)
				info["missing"] = pl.missingRequirements(t)
				info["previous_version"] = previous[t.ID]
				list = append(list, info)
			}
			return map[string]any{
				"themes":      list,
				"active":      active,
				"custom_css":  current["custom_css"],
				"custom_js":   current["custom_js"],
				"can_code":    pl.canCode(ctx.HookContext),
				"can_install": pl.canInstallThemes(ctx.HookContext),
			}, nil
		},
		Handle: func(ctx *mks.PageContext) (*mks.ActionResult, error) {
			action, _ := ctx.Data["action"].(string)
			switch action {
			case "scan":
				// Re-read the installed packages now and drop the site's page cache.
				pl.packagesChanged()
				pl.bumpAll()
				return mks.Success("قالب‌ها شناسایی شدند."), nil
			case "activate":
				id, _ := ctx.Data["id"].(string)
				if !pl.hasTheme(id) {
					return nil, mks.Reject("این قالب پیدا نشد.")
				}
				if missing := pl.missingRequirements(pl.themeByID(id)); len(missing) > 0 {
					return nil, mks.Reject("این قالب به افزونه‌های فعال نیاز دارد: " + strings.Join(missing, "، "))
				}
				ctx.Data = map[string]any{"theme": id}
				if _, err := pl.saveSettings(ctx); err != nil {
					return nil, err
				}
				return mks.Success("قالب فعال شد."), nil
			case "save_code":
				data := map[string]any{"custom_css": ctx.Data["custom_css"]}
				if v, sent := ctx.Data["custom_js"]; sent && pl.canCode(ctx.HookContext) {
					data["custom_js"] = v
				}
				ctx.Data = data
				if _, err := pl.saveSettings(ctx); err != nil {
					return nil, err
				}
				return mks.Success("CSS و JavaScript سفارشی ذخیره شد."), nil
			}
			return pl.handleThemeInstall(ctx, action)
		},
	}
}

func (pl *Plugin) hasTheme(id string) bool {
	for _, t := range pl.themes() {
		if t.ID == id {
			return true
		}
	}
	return false
}

// themeInfo is a theme as the themes page lists it.
func (pl *Plugin) themeInfo(t Theme, active string) map[string]any {
	return map[string]any{
		"id": t.ID, "name": t.Name, "description": t.Description, "swatch": t.Swatch,
		"version": t.Version, "author": t.Author, "screenshot": t.Screenshot, "requires": t.Requires,
		"uploaded": t.Uploaded, "active": t.ID == active,
	}
}

// InstallThemes: upload, update, roll back and delete theme ZIP files. Themes bring
// templates and JavaScript that run on the site, so give it to full administrators only.
const InstallThemes Ability = "cms.themes.install"

func (pl *Plugin) canInstallThemes(ctx *mks.HookContext) bool { return pl.can(ctx, InstallThemes) }
