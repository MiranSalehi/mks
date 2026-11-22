package cms

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/goravel/framework/contracts/database/schema"

	mks "github.com/MiranSalehi/mks"
)

// ManageSettings: change a site's settings (name, logo, calendar, permalinks, ...).
const ManageSettings Ability = "cms.settings.manage"

// SettingsTable holds the settings of each site as key/value rows.
const SettingsTable = "cms_settings"

// Setting is one setting of a site.
type Setting struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	SiteID uint   `json:"site_id"`
	Name   string `json:"name"`
	Value  string `json:"value"`
}

func (Setting) TableName() string { return SettingsTable }

// CreateSettingsTable creates the settings table; call it from a migration.
func CreateSettingsTable(s schema.Schema, siteTable string) error {
	return s.Create(SettingsTable, func(t schema.Blueprint) {
		t.ID()
		t.UnsignedBigInteger("site_id")
		if siteTable != "" {
			t.Foreign("site_id").References("id").On(siteTable).CascadeOnDelete()
		}
		t.String("name", 100) // "key" is a reserved word in MySQL
		t.Text("value")
		t.Unique("site_id", "name")
	})
}

// DropSettingsTable removes the settings table.
func DropSettingsTable(s schema.Schema) error { return s.DropIfExists(SettingsTable) }

// SettingRule checks one setting: it returns the value to store or a message.
type SettingRule func(pl *Plugin, siteID any, value string) (string, string)

// Settings keys of the core tabs (MKSine's General, Permalinks and Performance).
var permalinkKeys = map[string]string{
	"permalink_posts": "", "permalink_post": "slug", "permalink_categories": "", "permalink_category": "path",
	"permalink_tags": "", "permalink_tag": "slug", "permalink_page": "slug", "permalink_author": "id",
}

var patternRE = regexp.MustCompile(`^(/[a-z0-9._~{}-]+)+/?$`)

func (pl *Plugin) coreSettings() map[string]SettingRule {
	text := func(max int) SettingRule {
		return func(_ *Plugin, _ any, v string) (string, string) {
			v = strings.TrimSpace(v)
			if utf8.RuneCountInString(v) > max {
				return "", fmt.Sprintf("At most %s characters.", faDigits(max))
			}
			return v, ""
		}
	}
	media := func(pl *Plugin, siteID any, v string) (string, string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return "", ""
		}
		n, err := pl.query().Model(&Media{}).Where("site_id = ? AND path = ?", siteID, v).Count()
		if err != nil || n == 0 {
			return "", "This file is not in the site's media library."
		}
		return v, ""
	}
	rules := map[string]SettingRule{
		"site_name":       text(255),
		"short_site_name": text(100),
		"tagline":         text(500),
		"logo":            media,
		"favicon":         media,
		"date_calendar": func(_ *Plugin, _ any, v string) (string, string) {
			if v == "" || v == "shamsi" || v == "gregorian" {
				return v, ""
			}
			return "", "The calendar must be Jalali or Gregorian."
		},
		"front_page_id": func(pl *Plugin, siteID any, v string) (string, string) {
			if strings.TrimSpace(v) == "" {
				return "", ""
			}
			id, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				return "", "Invalid page."
			}
			n, err := pl.query().Model(&Page{}).Where("site_id = ? AND id = ? AND deleted_at IS NULL", siteID, id).Count()
			if err != nil || n == 0 {
				return "", "This page was not found on this site."
			}
			return strconv.FormatUint(id, 10), ""
		},
		"page_cache_enabled": func(_ *Plugin, _ any, v string) (string, string) {
			switch v {
			case "", "0", "false":
				return "0", ""
			case "1", "true":
				return "1", ""
			}
			return "", "Invalid value."
		},
		"page_cache_ttl_minutes": func(_ *Plugin, _ any, v string) (string, string) {
			if v == "" {
				return "", ""
			}
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 || n > 10080 {
				return "", "A number between 1 and 10080 minutes."
			}
			return strconv.Itoa(n), ""
		},
	}
	for key, placeholder := range permalinkKeys {
		rules[key] = permalinkRule(placeholder)
	}
	rules["theme"] = func(pl *Plugin, _ any, v string) (string, string) {
		if v == "" {
			return "", ""
		}
		for _, t := range pl.themes() {
			if t.ID == v {
				return v, ""
			}
		}
		return "", "This skin does not exist."
	}
	rules["custom_css"] = func(_ *Plugin, _ any, v string) (string, string) { return customCSS(v) }
	rules["custom_js"] = func(_ *Plugin, _ any, v string) (string, string) { return customJS(v) }
	return rules
}

// permalinkRule: a path of lowercase segments with exactly the placeholder it needs.
func permalinkRule(placeholder string) SettingRule {
	return func(_ *Plugin, _ any, v string) (string, string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return "", ""
		}
		v = cleanPattern(v)
		if !patternRE.MatchString(v) {
			return "", "Only lowercase letters, digits, - and / are allowed; start with /."
		}
		open := strings.Count(v, "{")
		switch {
		case placeholder == "" && open > 0:
			return "", "This URL has no placeholder."
		case placeholder != "" && (open != 1 || strings.Count(v, "{"+placeholder+"}") != 1):
			return "", "The URL must contain exactly one {" + placeholder + "}."
		case placeholder != "" && strings.Contains(v, "{"+placeholder+"}") && !strings.Contains(v, "/{"+placeholder+"}"):
			return "", "{" + placeholder + "} must be a complete URL segment."
		}
		return v, ""
	}
}

// settingRules are the core rules plus those of plugins (RegisterSettings).
func (pl *Plugin) settingRules() map[string]SettingRule {
	rules := pl.coreSettings()
	for k, r := range pl.extraSettings {
		if _, core := rules[k]; !core {
			rules[k] = r
		}
	}
	// The settings of the active ZIP plugins.
	for k, r := range pl.pluginSettingRules() {
		if _, taken := rules[k]; !taken {
			rules[k] = r
		}
	}
	return rules
}

// RegisterSettings adds setting keys for an extension tab (MKSine's SettingsTabManager).
// The TSX half shows the fields: cms({ settingsTabs: [...] }). The first rule of a key wins.
func (pl *Plugin) RegisterSettings(rules map[string]SettingRule) {
	if pl.extraSettings == nil {
		pl.extraSettings = map[string]SettingRule{}
	}
	for k, r := range rules {
		if _, taken := pl.extraSettings[k]; !taken {
			pl.extraSettings[k] = r
		}
	}
}

// settingsCacheKey is the cache key of a site's settings. It carries a version that a save
// raises after its commit, so a reader that loaded the old rows while the save was
// running caches them under a key nobody reads any more.
func (pl *Plugin) settingsCacheKey(siteSlug string) string {
	return "cms:settings:" + siteSlug + ":" + strconv.Itoa(pl.Cache.GetInt(settingsVersionKey(siteSlug), 0))
}

func settingsVersionKey(siteSlug string) string { return "cms:setver:" + siteSlug }

// settings of a site, from the cache (filled from the database, cleared on save).
func (pl *Plugin) settings(site *Site) map[string]string {
	if site == nil {
		return map[string]string{}
	}
	var key string
	if pl.Cache != nil {
		key = pl.settingsCacheKey(site.Slug)
		if raw, ok := pl.Cache.Get(key).(string); ok && raw != "" {
			var m map[string]string
			if json.Unmarshal([]byte(raw), &m) == nil {
				return m
			}
		}
	}
	var rows []Setting
	if err := pl.query().Model(&Setting{}).Where("site_id = ?", site.ID).Find(&rows); err != nil {
		pl.logf("cms: load settings: %v", err)
		return map[string]string{}
	}
	m := make(map[string]string, len(rows))
	for _, r := range rows {
		m[r.Name] = r.Value
	}
	if pl.Cache != nil {
		if raw, err := json.Marshal(m); err == nil {
			_ = pl.Cache.Put(key, string(raw), time.Hour)
		}
	}
	return m
}

// settingsBySlug finds a site's settings by its slug (URL builders know only the slug).
func (pl *Plugin) settingsBySlug(slug string) map[string]string {
	if pl.Cache != nil {
		if raw, ok := pl.Cache.Get(pl.settingsCacheKey(slug)).(string); ok && raw != "" {
			var m map[string]string
			if json.Unmarshal([]byte(raw), &m) == nil {
				return m
			}
		}
	}
	if pl.Sites == nil || pl.Orm == nil {
		return map[string]string{}
	}
	site, err := pl.Sites.BySlug(slug)
	if err != nil || site == nil {
		return map[string]string{}
	}
	return pl.settings(site)
}

// Setting is one setting of a site by its slug, "" when unset.
func (pl *Plugin) Setting(siteSlug, key string) string { return pl.settingsBySlug(siteSlug)[key] }

// settingsPage is the "site_settings" custom page (definePage of the same name in TSX).
// settingsPage is one of the settings pages (site_settings, site_permalinks,
// site_performance): they share the data and the save, each form sends its own keys.
func (pl *Plugin) settingsPage(name string) mks.Page {
	return mks.Page{
		Name:      name,
		Authorize: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManageSettings) },
		Data: func(ctx *mks.WidgetContext) (any, error) {
			site, err := pl.Sites.ByID(ctx.TenantID())
			if err != nil || site == nil {
				return nil, err
			}
			current := pl.settings(site)
			core := pl.coreSettings()
			out := map[string]any{}
			for k := range pl.settingRules() {
				out[k] = current[k]
				// Extension settings may hold lists or objects (stored as JSON).
				if _, isCore := core[k]; !isCore && (strings.HasPrefix(current[k], "[") || strings.HasPrefix(current[k], "{")) {
					var v any
					if json.Unmarshal([]byte(current[k]), &v) == nil {
						out[k] = v
					}
				}
			}
			out["page_cache_enabled"] = current["page_cache_enabled"] == "1"
			if v, err := strconv.Atoi(current["front_page_id"]); err == nil {
				out["front_page_id"] = v
			}
			themes := []map[string]string{}
			for _, t := range pl.themes() {
				themes = append(themes, map[string]string{"id": t.ID, "name": t.Name, "description": t.Description, "swatch": t.Swatch})
			}
			// The settings of the active ZIP plugins, one tab each (Settings › Extensions).
			packageTabs := []map[string]any{}
			for _, lp := range pl.activePlugins() {
				if len(lp.m.Settings) == 0 {
					continue
				}
				fields := make([]pkgField, len(lp.m.Settings))
				for i, f := range lp.m.Settings {
					f.Name = pluginSettingKey(lp.id, f.Name)
					fields[i] = f
					switch raw, set := current[f.Name]; {
					case f.Kind == "bool":
						out[f.Name] = raw == "1" || (!set && f.Default == true)
					case f.Kind == "int" && raw != "":
						n, _ := strconv.Atoi(raw)
						out[f.Name] = n
					case !set && f.Default != nil:
						out[f.Name] = f.Default
					}
				}
				packageTabs = append(packageTabs, map[string]any{"id": lp.id, "label": lp.name, "fields": fields})
			}
			return map[string]any{"values": out, "themes": themes, "can_code": pl.canCode(ctx.HookContext),
				"package_tabs": packageTabs,
				"defaults":     map[string]any{"site_name": site.Name, "permalinks": pl.defaultPermalinkSettings()}}, nil
		},
		Handle: pl.saveSettings,
	}
}

func (pl *Plugin) defaultPermalinkSettings() map[string]string {
	d := DefaultPermalinks
	return map[string]string{
		"permalink_posts": d.Posts, "permalink_post": d.Post, "permalink_categories": d.Categories,
		"permalink_category": d.Category, "permalink_tags": d.Tags, "permalink_tag": d.Tag,
		"permalink_page": d.Page, "permalink_author": d.Author,
	}
}

// saveSettings validates every sent key and writes the changed ones.
func (pl *Plugin) saveSettings(ctx *mks.PageContext) (*mks.ActionResult, error) {
	siteID := ctx.TenantID()
	site, err := pl.Sites.ByID(siteID)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, mks.Reject("Active site not found.")
	}
	rules := pl.settingRules()
	clean := map[string]string{}
	errs := mks.ValidationErrors{}
	for key, raw := range ctx.Data {
		rule, ok := rules[key]
		if !ok {
			continue // unknown keys are ignored, never stored
		}
		var text string
		switch v := raw.(type) {
		case nil:
		case string:
			text = v
		case bool:
			text = map[bool]string{true: "1", false: "0"}[v]
		case float64:
			text = strconv.FormatFloat(v, 'f', -1, 64)
		case []any, map[string]any:
			// Lists and objects (extension settings) are stored as JSON for their rule.
			raw, err := json.Marshal(v)
			if err != nil || len(raw) > 65536 {
				errs[key] = "Invalid value."
				continue
			}
			text = string(raw)
		default:
			errs[key] = "Invalid value."
			continue
		}
		// Custom JavaScript runs for every visitor: it needs its own ability.
		if key == "custom_js" && !pl.canCode(ctx.HookContext) {
			errs[key] = "You are not allowed to change the site's JavaScript."
			continue
		}
		value, msg := rule(pl, siteID, text)
		if msg != "" {
			errs[key] = msg
			continue
		}
		clean[key] = value
	}
	// Two permalinks may not be the same pattern.
	seen := map[string]string{}
	keys := make([]string, 0, len(permalinkKeys))
	for k := range permalinkKeys {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	effective := pl.defaultPermalinkSettings()
	for _, k := range keys {
		if v, ok := clean[k]; ok && v != "" {
			effective[k] = v
		}
		if other, dup := seen[effective[k]]; dup {
			errs[k] = "The URL \"" + other + "\" is the same."
		}
		seen[effective[k]] = k
	}
	if len(errs) > 0 {
		return nil, errs
	}
	for key, value := range clean {
		var rows []Setting
		if err := ctx.Query.Model(&Setting{}).Where("site_id = ? AND name = ?", siteID, key).Find(&rows); err != nil {
			return nil, err
		}
		switch {
		case value == "" && len(rows) > 0:
			if _, err := ctx.Query.Model(&Setting{}).Where("id = ?", rows[0].ID).Delete(&Setting{}); err != nil {
				return nil, err
			}
		case value == "":
		case len(rows) > 0:
			if _, err := ctx.Query.Model(&Setting{}).Where("id = ?", rows[0].ID).Update("value", value); err != nil {
				return nil, err
			}
		default:
			sid, _ := toUint(fmt.Sprint(siteID))
			if err := ctx.Query.Create(&Setting{SiteID: sid, Name: key, Value: value}); err != nil {
				return nil, err
			}
		}
	}
	// Only once committed: earlier, a reader could cache the old settings or pages again.
	ctx.AfterCommit(func() {
		if pl.Cache != nil {
			key := settingsVersionKey(site.Slug)
			pl.Cache.Forever(key, pl.Cache.GetInt(key, 0)+1)
		}
		pl.bump(siteID)
		pl.afterSettingsSaved(site)
	})
	return mks.Success("Site settings saved."), nil
}

// afterSettingsSaved: hook for caches that depend on settings (the page cache).
func (pl *Plugin) afterSettingsSaved(site *Site) {
	if pl.onSettingsSaved != nil {
		pl.onSettingsSaved(site)
	}
}
