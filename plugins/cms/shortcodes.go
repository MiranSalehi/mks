package cms

import (
	stdhtml "html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Shortcodes (MKSine's Core/Shortcodes): [tag a="1" b='2' c=3], [tag/] or
// [tag]inner[/tag] in page and post content. Tag names are letters, digits and
// underscores, case-insensitive; an opening tag without a closing one is self-closing;
// unknown tags stay as they are. At most 5 levels and 2 passes.

// ShortcodeContext is where a shortcode is rendered.
type ShortcodeContext struct {
	Site     *Site
	Calendar string
	Now      time.Time
	// Record is the page or post whose content it is, if any.
	Record any
}

// ShortcodeFunc renders a shortcode. The returned HTML is trusted: escape attributes and
// content that come from editors (html.EscapeString).
type ShortcodeFunc func(attrs map[string]string, content string, ctx ShortcodeContext) string

const (
	shortcodeDepth  = 5
	shortcodePasses = 2
)

// AddShortcode registers a shortcode; the first registration of a tag wins.
func (pl *Plugin) AddShortcode(tag string, fn ShortcodeFunc) {
	tag = strings.ToLower(tag)
	if !shortcodeName.MatchString(tag) {
		panic("cms: invalid shortcode tag " + tag)
	}
	pl.hooks.mu.Lock()
	defer pl.hooks.mu.Unlock()
	if pl.shortcodes == nil {
		pl.shortcodes = map[string]ShortcodeFunc{}
	}
	if _, taken := pl.shortcodes[tag]; !taken {
		pl.shortcodes[tag] = fn
	}
}

// ShortcodeInfo describes a shortcode in the editor's "Insert shortcode" list. Example is
// what the list inserts ("[tag]" when empty).
type ShortcodeInfo struct {
	Tag         string `json:"tag"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Example     string `json:"example"`
}

// DescribeShortcode sets the label, description and example of a shortcode for the editor.
func (pl *Plugin) DescribeShortcode(info ShortcodeInfo) {
	info.Tag = strings.ToLower(info.Tag)
	pl.hooks.mu.Lock()
	defer pl.hooks.mu.Unlock()
	if pl.shortcodeInfo == nil {
		pl.shortcodeInfo = map[string]ShortcodeInfo{}
	}
	pl.shortcodeInfo[info.Tag] = info
}

// ShortcodeCatalog lists the registered shortcodes by tag, for the editor.
func (pl *Plugin) ShortcodeCatalog() []ShortcodeInfo {
	pl.hooks.mu.RLock()
	defer pl.hooks.mu.RUnlock()
	out := make([]ShortcodeInfo, 0, len(pl.shortcodes))
	for tag := range pl.shortcodes {
		info := pl.shortcodeInfo[tag]
		info.Tag = tag
		if info.Label == "" {
			info.Label = tag
		}
		if info.Example == "" {
			info.Example = "[" + tag + "]"
		}
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Tag < out[j].Tag })
	return out
}

var (
	shortcodeName = regexp.MustCompile(`^[a-z0-9_]+$`)
	shortcodeTag  = regexp.MustCompile(`\[/?[A-Za-z0-9_]+[^\[\]]*\]`)
	htmlTag       = regexp.MustCompile(`<[^>]*>`)
	placeholder   = regexp.MustCompile("\uE000[0-9]+\uE001")
	shortcodeOpen = regexp.MustCompile(`\[([A-Za-z0-9_]+)((?:\s+[A-Za-z0-9_-]+(?:\s*=\s*(?:"[^"]*"|'[^']*'|[^\s\]"']+))?)*)\s*(/?)\]`)
	shortcodeAttr = regexp.MustCompile(`([A-Za-z0-9_-]+)(?:\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s\]"']+)))?`)
)

func (pl *Plugin) coreShortcodes() {
	pl.AddShortcode("year", func(_ map[string]string, _ string, ctx ShortcodeContext) string {
		if ctx.Calendar == "gregorian" {
			return strconv.Itoa(ctx.Now.Year())
		}
		jy, _, _ := toJalali(ctx.Now.Year(), int(ctx.Now.Month()), ctx.Now.Day())
		return strconv.Itoa(jy)
	})
	pl.AddShortcode("site_name", func(_ map[string]string, _ string, ctx ShortcodeContext) string {
		if ctx.Site == nil {
			return ""
		}
		return stdhtml.EscapeString(ctx.Site.Name)
	})
	pl.DescribeShortcode(ShortcodeInfo{Tag: "year", Label: "Current year", Description: "Inserts the current year (Jalali when enabled)."})
	pl.DescribeShortcode(ShortcodeInfo{Tag: "site_name", Label: "Site name", Description: "Inserts the site name from settings, or the app name."})
}

// renderShortcodes expands the registered shortcodes of html.
func (pl *Plugin) renderShortcodes(content string, ctx ShortcodeContext) string {
	pl.hooks.mu.RLock()
	empty := len(pl.shortcodes) == 0
	pl.hooks.mu.RUnlock()
	if empty || !strings.Contains(content, "[") {
		return content
	}
	content, _ = pl.filter("mksine.content.before_shortcodes", content, ctx).(string)
	// Shortcodes live in text, never inside an HTML tag: there a quote decoded or HTML
	// produced by a shortcode could end an attribute. Tags are set aside meanwhile.
	var tags []string
	content = htmlTag.ReplaceAllStringFunc(content, func(tag string) string {
		tags = append(tags, tag)
		return "\uE000" + strconv.Itoa(len(tags)-1) + "\uE001"
	})
	// The HTML cleaner stores quotes in text as entities; tags need them back to parse.
	content = shortcodeTag.ReplaceAllStringFunc(content, func(tag string) string {
		return strings.NewReplacer("&quot;", `"`, "&#34;", `"`, "&#39;", "'", "&#x27;", "'", "&apos;", "'").Replace(tag)
	})
	for pass := 0; pass < shortcodePasses; pass++ {
		next := pl.expand(content, ctx, 0)
		if next == content {
			break
		}
		content = next
	}
	content = placeholder.ReplaceAllStringFunc(content, func(m string) string {
		i, err := strconv.Atoi(m[3 : len(m)-3])
		if err != nil || i < 0 || i >= len(tags) {
			return ""
		}
		return tags[i]
	})
	content, _ = pl.filter("mksine.content.after_shortcodes", content, ctx).(string)
	return content
}

func (pl *Plugin) expand(s string, ctx ShortcodeContext, depth int) string {
	if depth >= shortcodeDepth {
		return s
	}
	var out strings.Builder
	for {
		loc := shortcodeOpen.FindStringSubmatchIndex(s)
		if loc == nil {
			out.WriteString(s)
			return out.String()
		}
		tag := strings.ToLower(s[loc[2]:loc[3]])
		pl.hooks.mu.RLock()
		fn, known := pl.shortcodes[tag]
		pl.hooks.mu.RUnlock()
		if !known {
			out.WriteString(s[:loc[1]])
			s = s[loc[1]:]
			continue
		}
		out.WriteString(s[:loc[0]])
		attrs := parseAttrs(s[loc[4]:loc[5]])
		rest := s[loc[1]:]
		inner := ""
		if loc[6] == loc[7] { // not "[tag/]": look for a closing tag
			closing := regexp.MustCompile(`(?i)\[/` + regexp.QuoteMeta(tag) + `\]`)
			if c := closing.FindStringIndex(rest); c != nil {
				inner = pl.expand(rest[:c[0]], ctx, depth+1)
				rest = rest[c[1]:]
			}
		}
		html := fn(attrs, inner, ctx)
		if f, ok := pl.filter("mksine.shortcode."+tag, html, attrs, ctx).(string); ok {
			html = f
		}
		out.WriteString(html)
		s = rest
	}
}

func parseAttrs(raw string) map[string]string {
	attrs := map[string]string{}
	for _, m := range shortcodeAttr.FindAllStringSubmatch(raw, -1) {
		v := m[2] + m[3] + m[4]
		// The HTML cleaner stores quotes as entities; read them back.
		attrs[strings.ToLower(m[1])] = stdhtml.UnescapeString(v)
	}
	return attrs
}

// StripShortcodes removes shortcode tags, keeping enclosed text (MKSine's
// mks_strip_shortcodes), e.g. for excerpts.
func StripShortcodes(s string) string {
	s = regexp.MustCompile(`\[/[A-Za-z0-9_]+\]`).ReplaceAllString(s, "")
	return shortcodeOpen.ReplaceAllString(s, "")
}
