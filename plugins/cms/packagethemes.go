package cms

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"path"
	"sort"
	"strings"
)

// A theme ZIP:
//
//	theme.json          {"id","name","version","author","description","swatch","screenshot",
//	                     "requires":[plugin ids],"locations":[{"key","label"}],
//	                     "assets":{"css":["assets/style.css"],"js":["assets/site.js"]}}
//	templates/*.html    Go templates that replace views ({{define "post"}}...{{end}})
//	assets/...          CSS, JavaScript, images and fonts, served from the site
//
// Without CSS assets the default theme's CSS is used.

type themeManifest struct {
	manifestBase
	Swatch string `json:"swatch"`
}

var swatchRE = colorRE

// checkTheme validates a theme's manifest and templates before it is installed.
func checkTheme(raw []byte, files map[string][]byte) (*themeManifest, error) {
	var m themeManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("theme.json معتبر نیست: %v", err)
	}
	if err := m.check(files); err != nil {
		return nil, err
	}
	if m.Swatch != "" && !swatchRE.MatchString(m.Swatch) {
		return nil, errors.New("رنگ (swatch) باید مثل #0a7f72 باشد")
	}
	if _, err := parseThemeTemplates(themeTemplateText(files)); err != nil {
		return nil, fmt.Errorf("قالب‌های theme خطا دارند: %v", err)
	}
	return &m, nil
}

// themeTemplateText joins templates/*.html in name order.
func themeTemplateText(files map[string][]byte) string {
	var names []string
	for p := range files {
		if strings.HasPrefix(p, "templates/") && (path.Ext(p) == ".html" || path.Ext(p) == ".tmpl") {
			names = append(names, p)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		b.Write(files[n])
		b.WriteByte('\n')
	}
	return b.String()
}

func parseThemeTemplates(text string) (*template.Template, error) {
	tpl, err := template.New("site").Parse(themeLayout + pictureTemplate)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return tpl, nil
	}
	return tpl.Parse(text)
}

// loadTheme builds an installed theme from its files.
func (pl *Plugin) loadTheme(r Package) (Theme, error) {
	files, err := pl.readPackageFiles(KindTheme, r.PackageID, r.Version)
	if err != nil {
		return Theme{}, err
	}
	m, err := checkTheme(files["theme.json"], files)
	if err != nil {
		return Theme{}, err
	}
	t := Theme{
		ID: m.ID, Name: m.Name, Description: m.Description, Swatch: m.Swatch, Version: m.Version, Author: m.Author,
		Requires: m.Requires, Templates: themeTemplateText(files), Uploaded: true,
		pkgVersion: r.Version, pkgScreenshot: m.Screenshot, pkgJS: m.Assets.JS,
	}
	var css strings.Builder
	for _, f := range m.Assets.CSS {
		css.Write(files[f])
		css.WriteByte('\n')
	}
	t.CSS = css.String()
	for _, l := range m.Locations {
		t.Locations = append(t.Locations, Location{Key: l.Key, Label: l.Label})
	}
	return t, nil
}

// codeThemes are the themes in code: the built-in ones and Plugin.Themes.
func (pl *Plugin) codeThemes() []Theme {
	list := []Theme{DefaultTheme, MagazineTheme}
	for _, t := range pl.Themes {
		replaced := false
		for i := range list {
			if list[i].ID == t.ID {
				list[i], replaced = t, true
			}
		}
		if !replaced {
			list = append(list, t)
		}
	}
	return list
}

// forSite fills the addresses of an uploaded theme's files under a site.
func (pl *Plugin) themeForSite(t Theme, site *Site) Theme {
	if !t.Uploaded || site == nil {
		return t
	}
	if t.pkgScreenshot != "" {
		t.Screenshot = pl.packageAssetURL(site, KindTheme, t.ID, t.pkgVersion, t.pkgScreenshot)
	}
	t.Scripts = nil
	for _, f := range t.pkgJS {
		t.Scripts = append(t.Scripts, pl.packageAssetURL(site, KindTheme, t.ID, t.pkgVersion, f))
	}
	return t
}

// themeInUse is the name of a site that has the theme active, if any.
func (pl *Plugin) themeInUse(id string) (string, bool, error) {
	var rows []struct {
		SiteID uint
	}
	if err := pl.query().Table(SettingsTable).Select("site_id").Where("name = ? AND value = ?", "theme", id).Scan(&rows); err != nil {
		return "", false, err
	}
	for _, r := range rows {
		if site, err := pl.Sites.ByID(r.SiteID); err == nil && site != nil {
			return site.Name, true, nil
		}
	}
	return "", false, nil
}
