package mks

import (
	"regexp"

	"github.com/microcosm-cc/bluemonday"
)

// richTextPolicy keeps formatting, links, lists, headings, quotes, code, images, tables,
// video and audio, and the editor's colors, sizes, fonts, alignment and indents; it drops
// scripts, other styles, event handlers, iframes and javascript: URLs.
var richTextPolicy = func() *bluemonday.Policy {
	p := bluemonday.UGCPolicy()
	p.RequireNoReferrerOnLinks(true)
	p.AddTargetBlankToFullyQualifiedLinks(true)

	// The editor's toolbar: text and background color, font size and family, alignment,
	// block indent and list numbering style.
	color := regexp.MustCompile(`(?i)^(#[0-9a-f]{3,8}|rgba?\(\s*[0-9.%\s,]+\)|hsla?\(\s*[0-9.%\s,deg]+\)|[a-z]{3,20})$`)
	p.AllowStyles("color", "background-color").Matching(color).Globally()
	p.AllowStyles("font-size").Matching(regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{1,2})?(px|pt|em|rem|%)$`)).Globally()
	p.AllowStyles("font-family").Matching(regexp.MustCompile(`^[\p{L}\p{N} ,'"_-]{1,120}$`)).Globally()
	p.AllowStyles("text-align").MatchingEnum("left", "right", "center", "justify", "start", "end").Globally()
	p.AllowStyles("margin-left", "margin-right", "margin-inline-start").Matching(regexp.MustCompile(`^[0-9]{1,3}px$`)).OnElements("p", "h1", "h2", "h3", "h4", "h5", "h6")
	p.AllowStyles("list-style-type").MatchingEnum("disc", "circle", "square", "decimal", "decimal-leading-zero", "lower-roman", "upper-roman", "lower-latin", "upper-latin").OnElements("ul", "ol")
	p.AllowStyles("min-width", "width").Matching(regexp.MustCompile(`^[0-9]{1,4}(px|%)$`)).OnElements("col", "td", "th", "table", "img")
	p.AllowElements("colgroup", "col")
	p.AllowAttrs("colwidth").Matching(regexp.MustCompile(`^[0-9,]{1,40}$`)).OnElements("td", "th")
	p.AllowAttrs("reversed").Matching(regexp.MustCompile(`^(|reversed)$`)).OnElements("ol")

	// The editor's named styles and markers (classes from a fixed list), code languages.
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^(category|document-title|document-subtitle|info-box|side-quote|marker|spoiler|fancy-code|fancy-code-dark|fancy-code-bright|media)( (fancy-code|fancy-code-dark|fancy-code-bright))*$`)).OnElements("h2", "h3", "p", "blockquote", "span", "pre", "figure")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^(marker-yellow|marker-green|marker-pink|marker-blue|pen-red|pen-green)$`)).OnElements("mark")
	p.AllowAttrs("class").Matching(regexp.MustCompile(`^language-[a-z0-9+#-]{1,20}$`)).OnElements("code")

	// To-do lists: the list and its checked items.
	p.AllowAttrs("data-type").Matching(regexp.MustCompile(`^(taskList|taskItem)$`)).OnElements("ul", "li")
	p.AllowAttrs("data-checked").Matching(regexp.MustCompile(`^(true|false)$`)).OnElements("li")
	p.AllowElements("label")
	p.AllowAttrs("type").Matching(regexp.MustCompile(`^checkbox$`)).OnElements("input")
	p.AllowAttrs("checked", "disabled").Matching(regexp.MustCompile(`^(|checked|disabled|true)$`)).OnElements("input")

	// Links the editor marks as downloads; media from the library.
	p.AllowAttrs("download").Matching(regexp.MustCompile(`^[\p{L}\p{N} ._-]{0,100}$`)).OnElements("a")
	p.AllowAttrs("src").OnElements("video", "audio")
	p.AllowAttrs("controls", "preload", "playsinline").Matching(regexp.MustCompile(`^(|controls|metadata|none|auto|playsinline)$`)).OnElements("video", "audio")
	return p
}()

// SanitizeHTML returns safe rich-text markup for use by first-party renderers. Callers
// should still escape ordinary text fields with html/template; only this returned value
// may be marked as trusted HTML.
func SanitizeHTML(content string) string { return richTextPolicy.Sanitize(content) }

// sanitizeHTML cleans the rich-text fields of the data in place.
func sanitizeHTML(c Config, data map[string]any) {
	for _, field := range c.HTML {
		if s, ok := data[field].(string); ok {
			data[field] = richTextPolicy.Sanitize(s)
		}
	}
}
