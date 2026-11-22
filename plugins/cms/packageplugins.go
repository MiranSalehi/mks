package cms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"regexp"
	"sort"
	"strconv"
	"strings"

	mks "github.com/MiranSalehi/mks"
)

// A plugin ZIP:
//
//	plugin.json  {"id","name","version","author","description","requires":[plugin ids],
//	  "blocks":[{"type","label","category","template":"blocks/x.html","fields":[field]}],
//	  "actions":[{"hook":"single.after_article","template":"actions/x.html","priority":10}],
//	  "shortcodes":[{"tag":"note","template":"shortcodes/note.html","label":"Note","description":"…","example":"[note]…[/note]"}],
//	  "settings":[field],
//	  "locations":[{"key","label"}],
//	  "assets":{"css":["assets/x.css"],"js":["assets/x.js"]}}
//	field        {"name","label","kind":"text|textarea|html|url|image|enum|bool|int|color",
//	              "options":{"value":"label"},"default","max","min","required","help"}
//
// A plugin's settings are site settings named "plugin:{id}:{name}".
//
// Templates are Go html/template: a block sees .D (its fields), .HTML (cleaned html
// fields), .Img and .URL (image and link fields) and .Settings; an action sees .View,
// .Settings and .Record; a shortcode sees .Attrs, .Content and .Settings.

type pkgField struct {
	Name     string            `json:"name"`
	Label    string            `json:"label"`
	Kind     string            `json:"kind"`
	Options  map[string]string `json:"options,omitempty"`
	Default  any               `json:"default,omitempty"`
	Max      int               `json:"max,omitempty"`
	Min      int               `json:"min,omitempty"`
	Required bool              `json:"required,omitempty"`
	Help     string            `json:"help,omitempty"`
}

type pluginManifest struct {
	manifestBase
	Blocks []struct {
		Type     string     `json:"type"`
		Label    string     `json:"label"`
		Category string     `json:"category"`
		Template string     `json:"template"`
		Fields   []pkgField `json:"fields"`
	} `json:"blocks"`
	Actions []struct {
		Hook     string `json:"hook"`
		Template string `json:"template"`
		Priority int    `json:"priority"`
	} `json:"actions"`
	Shortcodes []struct {
		Tag      string `json:"tag"`
		Template string `json:"template"`
		// For the editor's "Insert shortcode" list (optional).
		Label       string `json:"label"`
		Description string `json:"description"`
		Example     string `json:"example"`
	} `json:"shortcodes"`
	Settings     []pkgField `json:"settings"`
	ContentTypes []struct {
		Key        string `json:"key"`
		Singular   string `json:"singular"`
		Plural     string `json:"plural"`
		Public     bool   `json:"public"`
		HasArchive bool   `json:"has_archive"`
		ArchiveURI string `json:"archive_uri"`
		SingleURI  string `json:"single_uri"`
	} `json:"content_types"`
}

var (
	pkgNameRE  = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)
	pkgHookRE  = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,99}$`)
	pkgKinds   = map[string]fieldKind{"text": fText, "textarea": fTextarea, "html": fHTML, "url": fURL, "image": fMedia, "enum": fEnum, "bool": fBool, "int": fInt, "color": fColor}
	categories = map[string]bool{"content": true, "media": true, "layout": true, "interactive": true, "sections": true}
)

// PackageBlockType is the block type of a plugin's block: "pkg-{plugin}-{type}", so it
// never clashes with the built-in blocks or another plugin's.
func PackageBlockType(pluginID, blockType string) string { return "pkg-" + pluginID + "-" + blockType }

func checkFields(where string, fields []pkgField) error {
	seen := map[string]bool{}
	for _, f := range fields {
		if !pkgNameRE.MatchString(f.Name) || seen[f.Name] {
			return fmt.Errorf("%s: نام فیلد نامعتبر یا تکراری: %s", where, f.Name)
		}
		seen[f.Name] = true
		if _, ok := pkgKinds[f.Kind]; !ok {
			return fmt.Errorf("%s: نوع فیلد %s شناخته نیست: %s", where, f.Name, f.Kind)
		}
		if f.Kind == "enum" && len(f.Options) == 0 {
			return fmt.Errorf("%s: فیلد %s گزینه ندارد", where, f.Name)
		}
		if strings.TrimSpace(f.Label) == "" {
			return fmt.Errorf("%s: فیلد %s برچسب ندارد", where, f.Name)
		}
		for _, p := range presentationFields {
			if p.Name == f.Name {
				return fmt.Errorf("%s: نام %s برای سبک بلوک‌ها رزرو است", where, f.Name)
			}
		}
	}
	return nil
}

// checkPlugin validates a plugin's manifest and parses its templates.
func checkPlugin(raw []byte, files map[string][]byte) (*pluginManifest, error) {
	var m pluginManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("plugin.json معتبر نیست: %v", err)
	}
	if err := m.check(files); err != nil {
		return nil, err
	}
	tpl := func(where, file string) error {
		body, ok := files[file]
		if !ok {
			return fmt.Errorf("%s: فایل %s پیدا نشد", where, file)
		}
		if _, err := template.New(file).Parse(string(body)); err != nil {
			return fmt.Errorf("%s: %v", where, err)
		}
		return nil
	}
	seen := map[string]bool{}
	for _, b := range m.Blocks {
		if !pkgNameRE.MatchString(b.Type) || seen[b.Type] {
			return nil, fmt.Errorf("نوع بلوک نامعتبر یا تکراری: %s", b.Type)
		}
		seen[b.Type] = true
		if strings.TrimSpace(b.Label) == "" {
			return nil, fmt.Errorf("بلوک %s برچسب ندارد", b.Type)
		}
		if b.Category == "" {
			b.Category = "content"
		}
		if !categories[b.Category] {
			return nil, fmt.Errorf("دستهٔ بلوک %s نامعتبر است", b.Type)
		}
		if err := checkFields("بلوک "+b.Type, b.Fields); err != nil {
			return nil, err
		}
		if err := tpl("بلوک "+b.Type, b.Template); err != nil {
			return nil, err
		}
	}
	for _, a := range m.Actions {
		if !pkgHookRE.MatchString(a.Hook) {
			return nil, fmt.Errorf("نام hook نامعتبر: %s", a.Hook)
		}
		if err := tpl("hook "+a.Hook, a.Template); err != nil {
			return nil, err
		}
	}
	tags := map[string]bool{}
	for _, s := range m.Shortcodes {
		if !shortcodeName.MatchString(s.Tag) || tags[s.Tag] {
			return nil, fmt.Errorf("کد کوتاه نامعتبر یا تکراری: %s", s.Tag)
		}
		tags[s.Tag] = true
		if err := tpl("کد کوتاه "+s.Tag, s.Template); err != nil {
			return nil, err
		}
	}
	if err := checkFields("تنظیمات", m.Settings); err != nil {
		return nil, err
	}
	keys := map[string]bool{}
	for _, t := range m.ContentTypes {
		if !contentTypeKey.MatchString(t.Key) || len(t.Key) > 40 || reservedContentTypes[t.Key] || keys[t.Key] {
			return nil, fmt.Errorf("کلید نوع محتوا نامعتبر، رزروشده یا تکراری است: %s", t.Key)
		}
		keys[t.Key] = true
		if strings.TrimSpace(t.Singular) == "" || strings.TrimSpace(t.Plural) == "" {
			return nil, fmt.Errorf("نوع محتوای %s نام مفرد و جمع لازم دارد", t.Key)
		}
		for _, uri := range []string{t.ArchiveURI, t.SingleURI} {
			if uri != "" && !patternRE.MatchString(uri) {
				return nil, fmt.Errorf("نشانی نوع محتوای %s نامعتبر است: %s", t.Key, uri)
			}
		}
		if t.SingleURI != "" && !strings.Contains(t.SingleURI, "{slug}") {
			return nil, fmt.Errorf("نشانی تکی نوع محتوای %s باید {slug} داشته باشد", t.Key)
		}
	}
	return &m, nil
}

// loadedPlugin is an active plugin, ready to use.
type loadedPlugin struct {
	id, name, version string
	m                 *pluginManifest
	blocks            map[string]*pkgBlock // by full block type
	order             []string
	actions           []pkgAction
	shortcodes        map[string]*template.Template
	css               string
	contentTypes      []ContentType
}

type pkgBlock struct {
	spec   BlockSpec
	label  string
	fields []pkgField
	tpl    *template.Template
}

type pkgAction struct {
	hook     string
	priority int
	tpl      *template.Template
}

func (pl *Plugin) loadPlugin(r Package) (*loadedPlugin, error) {
	files, err := pl.readPackageFiles(KindPlugin, r.PackageID, r.Version)
	if err != nil {
		return nil, err
	}
	m, err := checkPlugin(files["plugin.json"], files)
	if err != nil {
		return nil, err
	}
	lp := &loadedPlugin{id: m.ID, name: m.Name, version: r.Version, m: m, blocks: map[string]*pkgBlock{}, shortcodes: map[string]*template.Template{}}
	parse := func(file string) *template.Template {
		return template.Must(template.New(file).Parse(string(files[file])))
	}
	for _, b := range m.Blocks {
		full := PackageBlockType(m.ID, b.Type)
		category := b.Category
		if category == "" {
			category = "content"
		}
		lp.blocks[full] = &pkgBlock{spec: BlockSpec{Type: full, Category: category, Fields: specFields(b.Fields)}, label: b.Label, fields: b.Fields, tpl: parse(b.Template)}
		lp.order = append(lp.order, full)
	}
	for _, a := range m.Actions {
		lp.actions = append(lp.actions, pkgAction{hook: a.Hook, priority: a.Priority, tpl: parse(a.Template)})
	}
	for _, s := range m.Shortcodes {
		lp.shortcodes[s.Tag] = parse(s.Template)
	}
	var css strings.Builder
	for _, f := range m.Assets.CSS {
		css.Write(files[f])
		css.WriteByte('\n')
	}
	lp.css = css.String()
	for _, t := range m.ContentTypes {
		lp.contentTypes = append(lp.contentTypes, withTypeDefaults(ContentType{Key: t.Key, Singular: t.Singular, Plural: t.Plural,
			Public: t.Public, HasArchive: t.HasArchive, ArchiveURI: t.ArchiveURI, SingleURI: t.SingleURI}))
	}
	return lp, nil
}

// specFields turns a plugin's fields into the builder's field specs.
func specFields(fields []pkgField) []fieldSpec {
	out := make([]fieldSpec, 0, len(fields))
	for _, f := range fields {
		s := fieldSpec{Name: f.Name, Kind: pkgKinds[f.Kind], Max: f.Max, Min: f.Min, Default: f.Default, Required: f.Required}
		switch s.Kind {
		case fText:
			if s.Max <= 0 || s.Max > 2000 {
				s.Max = 255
			}
		case fTextarea:
			if s.Max <= 0 || s.Max > 20000 {
				s.Max = 5000
			}
		case fHTML:
			if s.Max <= 0 || s.Max > 100000 {
				s.Max = 50000
			}
		case fEnum:
			for v := range f.Options {
				s.Options = append(s.Options, v)
			}
			sort.Strings(s.Options)
		case fInt:
			if s.Max == 0 && s.Min == 0 {
				s.Min, s.Max = -1000000, 1000000
			}
		}
		out = append(out, s)
	}
	return out
}

// applyPlugins swaps the template actions and shortcodes of the old set of plugins for
// the new set's. Their hook ids are "plugin.{id}.{n}", so the hooks page can turn them off.
func (pl *Plugin) applyPlugins(old, next []*loadedPlugin) {
	for _, lp := range old {
		for i, a := range lp.actions {
			pl.RemoveAction(a.hook, "plugin."+lp.id+"."+strconv.Itoa(i))
		}
		pl.hooks.mu.Lock()
		for tag := range lp.shortcodes {
			delete(pl.shortcodes, tag)
			delete(pl.pkgShortcodes, tag)
			delete(pl.shortcodeInfo, tag)
		}
		pl.hooks.mu.Unlock()
	}
	for _, lp := range next {
		lp := lp
		for i, a := range lp.actions {
			a := a
			pl.AddAction(a.hook, func(ctx ActionContext) template.HTML {
				var buf bytes.Buffer
				data := map[string]any{"View": ctx.View, "Record": ctx.Record, "Settings": pl.pluginSettings(ctx.Site, lp)}
				if err := a.tpl.Execute(&buf, data); err != nil {
					pl.logf("cms: plugin %s action %s: %v", lp.id, a.hook, err)
					return ""
				}
				return template.HTML(buf.String()) //nolint:gosec -- escaped by html/template
			}, "plugin."+lp.id+"."+strconv.Itoa(i), a.priority)
		}
		for tag, tpl := range lp.shortcodes {
			tag, tpl := tag, tpl
			pl.hooks.mu.Lock()
			_, taken := pl.shortcodes[tag]
			pl.hooks.mu.Unlock()
			if taken {
				pl.logf("cms: plugin %s: shortcode [%s] is taken", lp.id, tag)
				continue
			}
			pl.AddShortcode(tag, func(attrs map[string]string, content string, ctx ShortcodeContext) string {
				var buf bytes.Buffer
				data := map[string]any{"Attrs": attrs, "Content": content, "Settings": pl.pluginSettings(ctx.Site, lp)}
				if err := tpl.Execute(&buf, data); err != nil {
					pl.logf("cms: plugin %s shortcode %s: %v", lp.id, tag, err)
					return ""
				}
				return buf.String()
			})
			for _, s := range lp.m.Shortcodes {
				if s.Tag == tag {
					pl.DescribeShortcode(ShortcodeInfo{Tag: tag, Label: s.Label, Description: s.Description, Example: s.Example})
				}
			}
			pl.hooks.mu.Lock()
			if pl.pkgShortcodes == nil {
				pl.pkgShortcodes = map[string]bool{}
			}
			pl.pkgShortcodes[tag] = true
			pl.hooks.mu.Unlock()
		}
	}
}

// activePlugins are the loaded active plugins.
func (pl *Plugin) activePlugins() []*loadedPlugin {
	_, plugins := pl.loadedPackages()
	return plugins
}

// packageBlock is a plugin's block by its full type.
func (pl *Plugin) packageBlock(kind string) (*pkgBlock, *loadedPlugin) {
	if !strings.HasPrefix(kind, "pkg-") {
		return nil, nil
	}
	for _, lp := range pl.activePlugins() {
		if b, ok := lp.blocks[kind]; ok {
			return b, lp
		}
	}
	return nil, nil
}

// blockSpec is a built-in block's spec or an active plugin's.
func (pl *Plugin) blockSpec(kind string) (BlockSpec, bool) {
	if s, ok := specOf(kind); ok {
		return s, true
	}
	if pl == nil {
		return BlockSpec{}, false
	}
	if b, _ := pl.packageBlock(kind); b != nil {
		return b.spec, true
	}
	return BlockSpec{}, false
}

// pluginSettingKey is the site setting of a plugin's setting: "plugin:{id}:{name}" (no
// dots, which forms read as nested fields).
func pluginSettingKey(pluginID, name string) string { return "plugin:" + pluginID + ":" + name }

// pluginSettings are a plugin's settings of a site, by name (defaults when unset).
func (pl *Plugin) pluginSettings(site *Site, lp *loadedPlugin) map[string]any {
	out := map[string]any{}
	var stored map[string]string
	if site != nil {
		stored = pl.settings(site)
	}
	for _, f := range lp.m.Settings {
		v, ok := stored[pluginSettingKey(lp.id, f.Name)]
		switch {
		case !ok || v == "":
			out[f.Name] = f.Default
		case f.Kind == "bool":
			out[f.Name] = v == "1"
		case f.Kind == "int":
			n, _ := strconv.Atoi(v)
			out[f.Name] = n
		default:
			out[f.Name] = v
		}
	}
	return out
}

// pluginSettingRules are the rules of every active plugin's settings.
func (pl *Plugin) pluginSettingRules() map[string]SettingRule {
	out := map[string]SettingRule{}
	for _, lp := range pl.activePlugins() {
		for _, f := range lp.m.Settings {
			f := f
			out[pluginSettingKey(lp.id, f.Name)] = func(_ *Plugin, _ any, v string) (string, string) {
				return checkPluginSetting(f, v)
			}
		}
	}
	return out
}

func checkPluginSetting(f pkgField, v string) (string, string) {
	v = strings.TrimSpace(v)
	switch f.Kind {
	case "bool":
		if v != "" && v != "0" && v != "1" {
			return "", "مقدار نامعتبر است."
		}
	case "int":
		if v == "" {
			return "", ""
		}
		n, err := strconv.Atoi(v)
		if err != nil || (f.Max != 0 || f.Min != 0) && (n < f.Min || n > f.Max) {
			return "", "عدد معتبر نیست."
		}
	case "enum":
		if _, ok := f.Options[v]; v != "" && !ok {
			return "", "گزینهٔ نامعتبر."
		}
	case "url":
		if v != "" && !SafeLinkURL(v) {
			return "", "نشانی معتبر نیست."
		}
	case "color":
		if v != "" && !colorRE.MatchString(v) {
			return "", "رنگ معتبر نیست."
		}
	case "html":
		v = mks.SanitizeHTML(v)
	}
	if f.Required && v == "" {
		return "", "این فیلد لازم است."
	}
	if len(v) > 20000 {
		return "", "بیش از اندازه بلند است."
	}
	return v, ""
}

// pluginAssets are the CSS and JavaScript files of the active plugins for a site.
func (pl *Plugin) pluginAssets(site *Site) (string, []string) {
	var css strings.Builder
	var scripts []string
	for _, lp := range pl.activePlugins() {
		css.WriteString(lp.css)
		for _, f := range lp.m.Assets.JS {
			if u := pl.packageAssetURL(site, KindPlugin, lp.id, lp.version, f); u != "" {
				scripts = append(scripts, u)
			}
		}
	}
	return css.String(), scripts
}

// pluginLocations are the menu locations the active plugins add.
func (pl *Plugin) pluginLocations() []Location {
	var out []Location
	for _, lp := range pl.activePlugins() {
		for _, l := range lp.m.Locations {
			out = append(out, Location{Key: l.Key, Label: l.Label})
		}
	}
	return out
}

// packageBlockDefs describe the active plugins' blocks for the builder UI.
func (pl *Plugin) packageBlockDefs() []map[string]any {
	out := []map[string]any{}
	for _, lp := range pl.activePlugins() {
		for _, full := range lp.order {
			b := lp.blocks[full]
			out = append(out, map[string]any{"type": full, "label": b.label, "category": b.spec.Category, "plugin": lp.name, "fields": b.fields})
		}
	}
	return out
}

// renderPackageBlock runs a plugin block's template with the block's view.
func (r *renderer) renderPackageBlock(b *pkgBlock, lp *loadedPlugin, v *blockView) error {
	var buf bytes.Buffer
	data := map[string]any{"ID": v.ID, "D": v.D, "HTML": v.HTML, "Img": v.Img, "URL": v.URL, "Settings": r.pl.pluginSettings(r.siteOf(), lp)}
	if err := b.tpl.Execute(&buf, data); err != nil {
		return err
	}
	v.Inner = template.HTML(buf.String()) //nolint:gosec -- escaped by html/template
	return nil
}

func (r *renderer) siteOf() *Site {
	if r.req == nil {
		return nil
	}
	return r.req.site
}
