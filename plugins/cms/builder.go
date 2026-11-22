package cms

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	mks "github.com/MiranSalehi/mks"
)

// The page builder (MKSine's PageBuilder): a page is a list of typed blocks; container
// blocks (columns, grid, inset, card) hold child blocks in slots. Every block is described
// by a field spec, and one normalizer cleans every saved payload: unknown blocks and
// fields are dropped, text is cut to size, HTML is sanitized, links are checked (unsafe
// ones become "#", as in MKSine's BuilderUrl), and media must be in the site's library.
//
// Stored shape: [{type, data}], where container data has "children": [{blocks: [...]}].
// The shared style settings of MKSine's BlockPresentation are flat keys of data
// (anchor, spacing, padding, ...), so the panel's form engine can edit them.

type fieldKind int

const (
	fText fieldKind = iota
	fTextarea
	fHTML
	fURL
	fMedia
	fEnum
	fBool
	fInt
	fColor
	fList
	fSection
)

type fieldSpec struct {
	Name     string
	Kind     fieldKind
	Max      int      // characters for text, value for numbers, items for lists
	Min      int      // numbers and lists
	Options  []string // enums
	Default  any
	Required bool
	Item     []fieldSpec // list items
}

// BlockSpec describes a block type.
type BlockSpec struct {
	Type     string
	Category string
	Fields   []fieldSpec
	// Slots > 0 makes a container: children is a list of at most Slots slots, each with
	// blocks. MinSlots is the fewest it keeps.
	Slots    int
	MinSlots int
	// Required lists a field that must be filled for the block to count as content.
	Required string
}

var (
	sizes4    = []string{"none", "sm", "md", "lg"}
	alignLCR  = []string{"left", "center", "right"}
	btnStyles = []string{"primary", "secondary", "outline", "ghost"}
	// textDirections are the text_direction choices of the theme blocks.
	textDirections = []string{"auto", "rtl", "ltr"}
)

// presentationFields are the shared style settings of every block.
// Block fields must not reuse these names (a test checks).
var presentationFields = []fieldSpec{
	{Name: "anchor", Kind: fText, Max: 64},
	{Name: "spacing", Kind: fEnum, Options: sizes4, Default: "md"},
	{Name: "visibility", Kind: fEnum, Options: []string{"always", "hide_mobile", "hide_desktop"}, Default: "always"},
	{Name: "padding", Kind: fEnum, Options: sizes4, Default: "none"},
	{Name: "background", Kind: fEnum, Options: []string{"none", "muted", "brand"}, Default: "none"},
	{Name: "align", Kind: fEnum, Options: []string{"start", "left", "center", "right", "end"}, Default: "start"},
	{Name: "width", Kind: fEnum, Options: []string{"full", "contained"}, Default: "full"},
	{Name: "text_color", Kind: fColor},
	{Name: "background_color", Kind: fColor},
	{Name: "border_width", Kind: fEnum, Options: sizes4, Default: "none"},
	{Name: "border_color", Kind: fColor},
	{Name: "border_radius", Kind: fEnum, Options: sizes4, Default: "none"},
}

var anchorRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)
var colorRE = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func secondaryButton() []fieldSpec {
	return []fieldSpec{
		{Name: "show_secondary_button", Kind: fBool},
		{Name: "secondary_button_text", Kind: fText, Max: 100},
		{Name: "secondary_button_url", Kind: fURL},
	}
}

// BlockSpecs are the core blocks (MKSine's 20, its unregistered testimonial, and the
// quote and callout blocks of the first CMS slice).
var BlockSpecs = []BlockSpec{
	// Content
	{Type: "heading", Category: "content", Required: "text", Fields: []fieldSpec{
		{Name: "text", Kind: fText, Max: 300, Required: true},
		{Name: "level", Kind: fEnum, Options: []string{"h1", "h2", "h3", "h4", "h5", "h6"}, Default: "h2"},
		{Name: "alignment", Kind: fEnum, Options: alignLCR, Default: "right"},
	}},
	{Type: "text", Category: "content", Required: "content", Fields: []fieldSpec{
		{Name: "content", Kind: fHTML, Max: 100000, Required: true},
	}},
	{Type: "quote", Category: "content", Required: "text", Fields: []fieldSpec{
		{Name: "text", Kind: fTextarea, Max: 2000, Required: true},
		{Name: "author", Kind: fText, Max: 200},
	}},
	{Type: "callout", Category: "content", Required: "text", Fields: []fieldSpec{
		{Name: "tone", Kind: fEnum, Options: []string{"info", "warning"}, Default: "info"},
		{Name: "text", Kind: fTextarea, Max: 2000, Required: true},
	}},
	{Type: "features", Category: "content", Required: "features", Fields: []fieldSpec{
		{Name: "heading", Kind: fText, Max: 200},
		{Name: "subheading", Kind: fTextarea, Max: 500},
		{Name: "columns", Kind: fInt, Min: 1, Max: 6, Default: 3},
		{Name: "style", Kind: fEnum, Options: []string{"simple", "bordered", "shadowed", "filled"}, Default: "simple"},
		{Name: "icon_style", Kind: fEnum, Options: []string{"circle", "square", "none"}, Default: "circle"},
		{Name: "features", Kind: fList, Max: 24, Item: []fieldSpec{
			{Name: "icon", Kind: fText, Max: 8},
			{Name: "title", Kind: fText, Max: 200, Required: true},
			{Name: "description", Kind: fTextarea, Max: 1000},
			{Name: "link", Kind: fURL},
		}},
	}},
	{Type: "testimonial", Category: "content", Required: "testimonials", Fields: []fieldSpec{
		{Name: "heading", Kind: fText, Max: 200},
		{Name: "columns", Kind: fInt, Min: 1, Max: 4, Default: 3},
		{Name: "style", Kind: fEnum, Options: []string{"simple", "bordered", "shadowed", "quote"}, Default: "bordered"},
		{Name: "testimonials", Kind: fList, Max: 24, Item: []fieldSpec{
			{Name: "content", Kind: fTextarea, Max: 2000, Required: true},
			{Name: "author_name", Kind: fText, Max: 200},
			{Name: "author_title", Kind: fText, Max: 200},
			{Name: "author_image", Kind: fMedia},
			{Name: "rating", Kind: fInt, Min: 0, Max: 5, Default: 0},
		}},
	}},
	// Media
	{Type: "image", Category: "media", Required: "image", Fields: []fieldSpec{
		{Name: "image", Kind: fMedia, Required: true},
		{Name: "alt", Kind: fText, Max: 255},
		{Name: "caption", Kind: fText, Max: 500},
		{Name: "size", Kind: fEnum, Options: []string{"small", "medium", "large", "full"}, Default: "large"},
		{Name: "alignment", Kind: fEnum, Options: alignLCR, Default: "center"},
		{Name: "rounded", Kind: fBool},
		{Name: "shadow", Kind: fBool},
	}},
	{Type: "video", Category: "media", Required: "video", Fields: []fieldSpec{
		{Name: "video", Kind: fMedia, Required: true},
		{Name: "caption", Kind: fText, Max: 500},
		{Name: "controls", Kind: fBool, Default: true},
		{Name: "autoplay", Kind: fBool},
		{Name: "loop", Kind: fBool},
	}},
	{Type: "audio", Category: "media", Required: "audio", Fields: []fieldSpec{
		{Name: "audio", Kind: fMedia, Required: true},
		{Name: "caption", Kind: fText, Max: 500},
		{Name: "controls", Kind: fBool, Default: true},
		{Name: "autoplay", Kind: fBool},
		{Name: "loop", Kind: fBool},
	}},
	{Type: "slider", Category: "media", Required: "slides", Fields: []fieldSpec{
		{Name: "type", Kind: fEnum, Options: []string{"image", "content"}, Default: "image"},
		{Name: "height", Kind: fEnum, Options: []string{"small", "medium", "large", "auto"}, Default: "medium"},
		{Name: "autoplay", Kind: fBool, Default: true},
		{Name: "autoplay_speed", Kind: fInt, Min: 1000, Max: 30000, Default: 5000},
		{Name: "show_arrows", Kind: fBool, Default: true},
		{Name: "show_dots", Kind: fBool, Default: true},
		{Name: "loop", Kind: fBool, Default: true},
		{Name: "effect", Kind: fEnum, Options: []string{"slide", "fade"}, Default: "slide"},
		{Name: "slides", Kind: fList, Max: 20, Item: []fieldSpec{
			{Name: "image", Kind: fMedia},
			{Name: "title", Kind: fText, Max: 200},
			{Name: "subtitle", Kind: fTextarea, Max: 500},
			{Name: "button_text", Kind: fText, Max: 100},
			{Name: "button_url", Kind: fURL},
			{Name: "alt", Kind: fText, Max: 255},
		}},
	}},
	// Layout
	{Type: "spacer", Category: "layout", Fields: []fieldSpec{
		{Name: "size", Kind: fEnum, Options: []string{"xs", "sm", "md", "lg", "xl", "2xl"}, Default: "md"},
	}},
	{Type: "divider", Category: "layout", Fields: []fieldSpec{
		{Name: "style", Kind: fEnum, Options: []string{"solid", "dashed", "dotted"}, Default: "solid"},
		{Name: "line_width", Kind: fEnum, Options: []string{"25", "50", "75", "100"}, Default: "100"},
		{Name: "alignment", Kind: fEnum, Options: alignLCR, Default: "center"},
	}},
	{Type: "columns", Category: "layout", Slots: 12, MinSlots: 2, Fields: []fieldSpec{
		{Name: "layout", Kind: fEnum, Options: []string{"equal", "1-2", "2-1", "1-3", "3-1", "1-2-1"}, Default: "equal"},
		{Name: "gap", Kind: fEnum, Options: sizes4, Default: "md"},
		{Name: "vertical_alignment", Kind: fEnum, Options: []string{"start", "center", "end", "stretch"}, Default: "start"},
		{Name: "stack_on_mobile", Kind: fEnum, Options: []string{"always", "tablet", "mobile", "never"}, Default: "mobile"},
	}},
	{Type: "grid_layout", Category: "layout", Slots: 12, MinSlots: 1, Fields: []fieldSpec{
		{Name: "gap", Kind: fEnum, Options: sizes4, Default: "md"},
		{Name: "vertical_alignment", Kind: fEnum, Options: []string{"start", "center", "end", "stretch"}, Default: "start"},
		{Name: "stack_on_mobile", Kind: fEnum, Options: []string{"always", "tablet", "mobile", "never"}, Default: "mobile"},
		// MKSine's column_spans: the width of each column (of 12) on every screen and
		// from small (640px), medium (768px) and large (1024px) screens on; an empty
		// size keeps the smaller one's. No spans: the columns share the row evenly.
		{Name: "column_spans", Kind: fList, Max: 12, Item: []fieldSpec{
			{Name: "span", Kind: fInt, Min: 1, Max: 12, Default: 12},
			{Name: "span_sm", Kind: fInt, Min: 1, Max: 12},
			{Name: "span_md", Kind: fInt, Min: 1, Max: 12},
			{Name: "span_lg", Kind: fInt, Min: 1, Max: 12},
		}},
	}},
	{Type: "container_inset", Category: "layout", Slots: 1, MinSlots: 1, Fields: []fieldSpec{
		{Name: "max_width", Kind: fEnum, Options: []string{"full", "prose", "3xl", "5xl", "6xl", "7xl"}, Default: "5xl"},
		{Name: "background_full_bleed", Kind: fBool},
		{Name: "inset_background", Kind: fColor},
	}},
	{Type: "card", Category: "layout", Slots: 3, MinSlots: 1, Fields: []fieldSpec{
		{Name: "show_header", Kind: fBool, Default: true},
		{Name: "show_footer", Kind: fBool},
		{Name: "card_background", Kind: fColor},
		{Name: "card_text_color", Kind: fColor},
		{Name: "shadow", Kind: fEnum, Options: []string{"none", "sm", "md"}, Default: "sm"},
		{Name: "radius", Kind: fEnum, Options: sizes4, Default: "md"},
	}},
	{Type: "section_ref", Category: "layout", Required: "section_id", Fields: []fieldSpec{
		{Name: "section_id", Kind: fInt, Min: 1, Max: 1 << 31, Required: true},
	}},
	{Type: "hero", Category: "layout", Required: "title", Fields: append([]fieldSpec{
		{Name: "title", Kind: fText, Max: 200, Required: true},
		{Name: "subtitle", Kind: fTextarea, Max: 1000},
		{Name: "background_image", Kind: fMedia},
		{Name: "overlay", Kind: fEnum, Options: []string{"none", "light", "dark", "gradient"}, Default: "dark"},
		{Name: "height", Kind: fEnum, Options: []string{"small", "medium", "large", "full"}, Default: "medium"},
		{Name: "text_alignment", Kind: fEnum, Options: alignLCR, Default: "center"},
		{Name: "hero_text_color", Kind: fEnum, Options: []string{"white", "dark"}, Default: "white"},
		{Name: "button_text", Kind: fText, Max: 100},
		{Name: "button_url", Kind: fURL},
		{Name: "button_style", Kind: fEnum, Options: []string{"primary", "secondary", "outline"}, Default: "primary"},
	}, secondaryButton()...)},
	{Type: "tabs", Category: "layout", Required: "tabs", Fields: []fieldSpec{
		{Name: "style", Kind: fEnum, Options: []string{"underline", "pills", "boxed", "buttons"}, Default: "underline"},
		{Name: "tabs", Kind: fList, Max: 12, Item: []fieldSpec{
			{Name: "title", Kind: fText, Max: 100, Required: true},
			{Name: "content", Kind: fHTML, Max: 50000},
		}},
	}},
	// Interactive
	{Type: "button", Category: "interactive", Required: "text", Fields: []fieldSpec{
		{Name: "text", Kind: fText, Max: 100, Required: true},
		{Name: "url", Kind: fURL},
		{Name: "style", Kind: fEnum, Options: btnStyles, Default: "primary"},
		{Name: "size", Kind: fEnum, Options: []string{"sm", "md", "lg"}, Default: "md"},
		{Name: "alignment", Kind: fEnum, Options: alignLCR, Default: "right"},
		{Name: "new_tab", Kind: fBool},
		{Name: "full_width", Kind: fBool},
	}},
	{Type: "cta", Category: "interactive", Required: "title", Fields: append([]fieldSpec{
		{Name: "title", Kind: fText, Max: 200, Required: true},
		{Name: "description", Kind: fTextarea, Max: 1000},
		{Name: "style", Kind: fEnum, Options: []string{"simple", "boxed", "gradient", "dark"}, Default: "gradient"},
		{Name: "alignment", Kind: fEnum, Options: []string{"left", "center", "between"}, Default: "center"},
		{Name: "button_text", Kind: fText, Max: 100},
		{Name: "button_url", Kind: fURL},
	}, secondaryButton()...)},
	{Type: "accordion", Category: "interactive", Required: "items", Fields: []fieldSpec{
		{Name: "heading", Kind: fText, Max: 200},
		{Name: "style", Kind: fEnum, Options: []string{"simple", "bordered", "separated"}, Default: "bordered"},
		{Name: "allow_multiple", Kind: fBool},
		{Name: "first_open", Kind: fBool, Default: true},
		{Name: "items", Kind: fList, Max: 50, Item: []fieldSpec{
			{Name: "question", Kind: fText, Max: 300, Required: true},
			{Name: "answer", Kind: fHTML, Max: 20000},
		}},
	}},
	// Sections (MKSine's default-theme blocks; see themeblocks.go)
	{Type: "mksine_hero_domain", Category: "sections", Required: "heading_line1", Fields: []fieldSpec{
		{Name: "heading_line1", Kind: fText, Max: 255, Required: true},
		{Name: "heading_line2_prefix", Kind: fText, Max: 120},
		{Name: "heading_accent", Kind: fText, Max: 120},
		{Name: "heading_after", Kind: fText, Max: 120},
		{Name: "subheading", Kind: fTextarea, Max: 1000},
		{Name: "cta_label", Kind: fText, Max: 120},
		{Name: "cta_url", Kind: fURL},
		{Name: "illustration", Kind: fMedia},
		{Name: "illustration_alt", Kind: fText, Max: 255},
		{Name: "background_image", Kind: fMedia},
		{Name: "illustration_position", Kind: fEnum, Options: []string{"right", "left"}, Default: "right"},
	}},
	{Type: "mksine_finance_showcase", Category: "sections", Required: "heading", Fields: []fieldSpec{
		{Name: "heading", Kind: fText, Max: 255, Required: true},
		{Name: "heading_accent", Kind: fText, Max: 120},
		{Name: "paragraph_1", Kind: fTextarea, Max: 1000},
		{Name: "paragraph_1_highlight", Kind: fText, Max: 120},
		{Name: "paragraph_2", Kind: fTextarea, Max: 1000},
		{Name: "paragraph_2_highlight", Kind: fText, Max: 120},
		{Name: "cta_primary_label", Kind: fText, Max: 120},
		{Name: "cta_primary_url", Kind: fURL},
		{Name: "cta_secondary_label", Kind: fText, Max: 120},
		{Name: "cta_secondary_url", Kind: fURL},
		{Name: "trust_line", Kind: fTextarea, Max: 500},
		{Name: "social_caption", Kind: fText, Max: 200},
		{Name: "social_count", Kind: fText, Max: 60},
		{Name: "mock_chart_caption", Kind: fText, Max: 120},
		{Name: "avatars", Kind: fList, Max: 8, Item: []fieldSpec{
			{Name: "letter", Kind: fText, Max: 2, Required: true},
		}},
	}},
	{Type: "mksine_services_trio", Category: "sections", Required: "cards", Fields: []fieldSpec{
		{Name: "section_title", Kind: fText, Max: 255},
		{Name: "cards", Kind: fList, Max: 6, Item: []fieldSpec{
			{Name: "image", Kind: fMedia},
			{Name: "image_alt", Kind: fText, Max: 255},
			{Name: "title", Kind: fText, Max: 200, Required: true},
			{Name: "body", Kind: fTextarea, Max: 1000},
			{Name: "cta_label", Kind: fText, Max: 120},
			{Name: "url", Kind: fURL},
		}},
	}},
	{Type: "mksine_featured_domains", Category: "sections", Required: "items", Fields: []fieldSpec{
		{Name: "title", Kind: fText, Max: 255},
		{Name: "view_all_label", Kind: fText, Max: 120},
		{Name: "view_all_url", Kind: fURL},
		{Name: "items", Kind: fList, Max: 24, Item: []fieldSpec{
			{Name: "tld", Kind: fText, Max: 40, Required: true},
			{Name: "category", Kind: fText, Max: 120},
			{Name: "price", Kind: fText, Max: 40},
			{Name: "original_price", Kind: fText, Max: 40},
			{Name: "discount_percent", Kind: fInt, Min: 0, Max: 100},
			{Name: "href", Kind: fURL},
			{Name: "logo", Kind: fMedia},
		}},
	}},
	{Type: "mksine_clinic_features_grid", Category: "sections", Required: "cards", Fields: []fieldSpec{
		{Name: "text_direction", Kind: fEnum, Options: textDirections, Default: "auto"},
		{Name: "badge", Kind: fText, Max: 120},
		{Name: "heading_prefix", Kind: fText, Max: 200},
		{Name: "heading_accent", Kind: fText, Max: 200},
		{Name: "subheading", Kind: fTextarea, Max: 1000},
		{Name: "cards", Kind: fList, Max: 24, Item: []fieldSpec{
			{Name: "gradient", Kind: fEnum, Options: ThemeGradients, Default: ThemeGradients[0]},
			{Name: "icon", Kind: fEnum, Options: ThemeIcons, Default: ThemeIcons[0]},
			{Name: "title", Kind: fText, Max: 200, Required: true},
			{Name: "body", Kind: fTextarea, Max: 1000},
			{Name: "link_url", Kind: fURL},
			{Name: "link_new_tab", Kind: fBool},
		}},
	}},
	{Type: "mksine_testimonials_grid", Category: "sections", Required: "items", Fields: []fieldSpec{
		{Name: "text_direction", Kind: fEnum, Options: textDirections, Default: "auto"},
		{Name: "badge", Kind: fText, Max: 120},
		{Name: "title_prefix", Kind: fText, Max: 200},
		{Name: "title_accent", Kind: fText, Max: 200},
		{Name: "subheading", Kind: fTextarea, Max: 1000},
		{Name: "aside", Kind: fTextarea, Max: 500},
		{Name: "items", Kind: fList, Max: 12, Item: []fieldSpec{
			{Name: "name", Kind: fText, Max: 120, Required: true},
			{Name: "city", Kind: fText, Max: 120},
			{Name: "quote", Kind: fTextarea, Max: 1000},
			{Name: "gradient", Kind: fEnum, Options: ThemeGradients, Default: ThemeGradients[0]},
			{Name: "link_url", Kind: fURL},
			{Name: "link_new_tab", Kind: fBool},
		}},
	}},
	{Type: "mksine_post_comments_feed", Category: "sections", Fields: []fieldSpec{
		{Name: "post_id", Kind: fInt, Min: 0},
		{Name: "max_root_comments", Kind: fInt, Min: 1, Max: 50, Default: 12},
		{Name: "text_direction", Kind: fEnum, Options: textDirections, Default: "auto"},
		{Name: "badge", Kind: fText, Max: 120},
		{Name: "title_prefix", Kind: fText, Max: 200},
		{Name: "title_accent", Kind: fText, Max: 200},
		{Name: "subheading", Kind: fText, Max: 500},
		{Name: "aside", Kind: fTextarea, Max: 500},
		{Name: "quote_max_chars", Kind: fInt, Min: 80, Max: 500, Default: 220},
	}},
}

// BlockTypes are the type names of every block.
var BlockTypes = func() []string {
	out := make([]string, len(BlockSpecs))
	for i, s := range BlockSpecs {
		out[i] = s.Type
	}
	return out
}()

func specOf(kind string) (BlockSpec, bool) {
	for _, s := range BlockSpecs {
		if s.Type == kind {
			return s, true
		}
	}
	return BlockSpec{}, false
}

// maxBuilderDepth bounds nesting of containers (and the work of normalizing them).
const (
	maxBuilderDepth  = 4
	maxBuilderBlocks = 300
)

// builderNorm normalizes payloads of one site.
type builderNorm struct {
	pl     *Plugin
	siteID any
	errs   mks.ValidationErrors
	media  map[string]bool // checked library paths
	count  int
}

// NormalizeBuilder cleans a submitted payload (decoded JSON) for a site. Errors use the
// form paths ("builder_payload.2.data.title").
func (pl *Plugin) NormalizeBuilder(raw any, siteID any, field string) ([]Block, mks.ValidationErrors) {
	n := &builderNorm{pl: pl, siteID: siteID, errs: mks.ValidationErrors{}, media: map[string]bool{}}
	blocks := n.blocks(raw, field, 0)
	if len(n.errs) > 0 {
		return nil, n.errs
	}
	return blocks, nil
}

func (n *builderNorm) blocks(raw any, path string, depth int) []Block {
	var list []any
	switch v := raw.(type) {
	case nil:
		return []Block{}
	case []any:
		list = v
	case []Block:
		for _, b := range v {
			list = append(list, map[string]any{"type": b.Type, "data": b.Data})
		}
	default:
		n.errs[path] = "Invalid blocks structure."
		return nil
	}
	out := make([]Block, 0, len(list))
	for i, item := range list {
		bp := fmt.Sprintf("%s.%d", path, i)
		obj, ok := item.(map[string]any)
		if !ok {
			n.errs[bp] = "Invalid block structure."
			continue
		}
		kind, _ := obj["type"].(string)
		spec, ok := n.pl.blockSpec(kind)
		if !ok {
			// MKSine's PayloadNormalizer drops unknown blocks.
			continue
		}
		n.count++
		if n.count > maxBuilderBlocks {
			n.errs[path] = fmt.Sprintf("A page has at most %s blocks.", faDigits(maxBuilderBlocks))
			return out
		}
		data, _ := obj["data"].(map[string]any)
		if data == nil {
			data = map[string]any{}
		}
		clean := n.fields(spec.Fields, data, bp+".data")
		for k, v := range n.fields(presentationFields, data, bp+".data") {
			clean[k] = v
		}
		if a, _ := clean["anchor"].(string); a != "" && !anchorRE.MatchString(a) {
			n.errs[bp+".data.anchor"] = "Anchor: lowercase letters, digits, - and _; start with a letter."
		}
		if spec.Slots > 0 {
			clean["children"] = n.slots(data["children"], spec, bp+".data.children", depth)
		}
		out = append(out, Block{Type: kind, Data: clean})
	}
	return out
}

func (n *builderNorm) slots(raw any, spec BlockSpec, path string, depth int) []any {
	if depth+1 >= maxBuilderDepth {
		n.errs[path] = "Columns are nested too deeply."
		return nil
	}
	list, _ := raw.([]any)
	if len(list) > spec.Slots {
		list = list[:spec.Slots]
	}
	out := make([]any, 0, max(len(list), spec.MinSlots))
	for i, s := range list {
		obj, _ := s.(map[string]any)
		var blocks any
		if obj != nil {
			blocks = obj["blocks"]
		}
		out = append(out, map[string]any{"blocks": n.blocks(blocks, fmt.Sprintf("%s.%d.blocks", path, i), depth+1)})
	}
	for len(out) < spec.MinSlots {
		out = append(out, map[string]any{"blocks": []Block{}})
	}
	return out
}

func (n *builderNorm) fields(specs []fieldSpec, data map[string]any, path string) map[string]any {
	out := map[string]any{}
	for _, f := range specs {
		v, sent := data[f.Name]
		fp := path + "." + f.Name
		if !sent || v == nil || v == "" {
			if f.Default != nil {
				out[f.Name] = f.Default
			}
			if f.Required && f.Kind != fList {
				n.errs[fp] = "This field is required."
			}
			continue
		}
		switch f.Kind {
		case fText, fTextarea:
			s := strings.TrimSpace(fmt.Sprint(v))
			if f.Max > 0 && utf8.RuneCountInString(s) > f.Max {
				n.errs[fp] = fmt.Sprintf("At most %s characters.", faDigits(f.Max))
				continue
			}
			if s == "" && f.Required {
				n.errs[fp] = "This field is required."
				continue
			}
			out[f.Name] = s
		case fHTML:
			s, _ := v.(string)
			if f.Max > 0 && len(s) > f.Max*4 {
				n.errs[fp] = "The text is too long."
				continue
			}
			clean := mks.SanitizeHTML(s)
			if f.Required && strings.TrimSpace(plainText(clean, 10)) == "" {
				n.errs[fp] = "This field is required."
				continue
			}
			out[f.Name] = clean
		case fURL:
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" && !SafeLinkURL(s) {
				s = "#" // MKSine's BuilderUrl turns unsafe links into "#".
			}
			out[f.Name] = s
		case fMedia:
			s := strings.TrimSpace(fmt.Sprint(v))
			if !n.mediaOK(s) {
				n.errs[fp] = "This file is not in the site's media library."
				continue
			}
			out[f.Name] = s
		case fEnum:
			s := fmt.Sprint(v)
			ok := false
			for _, o := range f.Options {
				ok = ok || o == s
			}
			if !ok {
				if f.Default != nil {
					out[f.Name] = f.Default
				}
				continue
			}
			out[f.Name] = s
		case fBool:
			b, ok := v.(bool)
			if !ok {
				b = v == "1" || v == "true"
			}
			out[f.Name] = b
		case fInt:
			var num int
			switch x := v.(type) {
			case float64:
				num = int(x)
			case int:
				num = x
			case string:
				p, err := strconv.Atoi(strings.TrimSpace(x))
				if err != nil {
					n.errs[fp] = "Invalid number."
					continue
				}
				num = p
			default:
				n.errs[fp] = "Invalid number."
				continue
			}
			if num < f.Min || (f.Max > 0 && num > f.Max) {
				n.errs[fp] = fmt.Sprintf("A number between %s and %s.", faDigits(f.Min), faDigits(f.Max))
				continue
			}
			out[f.Name] = num
		case fColor:
			s := strings.TrimSpace(fmt.Sprint(v))
			if !colorRE.MatchString(s) {
				n.errs[fp] = "The color must be #rgb or #rrggbb."
				continue
			}
			out[f.Name] = s
		case fList:
			items, ok := v.([]any)
			if !ok {
				n.errs[fp] = "Invalid list."
				continue
			}
			if f.Max > 0 && len(items) > f.Max {
				n.errs[fp] = fmt.Sprintf("At most %s items.", faDigits(f.Max))
				continue
			}
			list := make([]any, 0, len(items))
			for i, it := range items {
				obj, _ := it.(map[string]any)
				if obj == nil {
					n.errs[fmt.Sprintf("%s.%d", fp, i)] = "Invalid item."
					continue
				}
				list = append(list, n.fields(f.Item, obj, fmt.Sprintf("%s.%d", fp, i)))
			}
			out[f.Name] = list
		}
	}
	return out
}

// mediaOK: a path of the site's media library (checked once per path).
func (n *builderNorm) mediaOK(p string) bool {
	if p == "" {
		return true
	}
	if ok, seen := n.media[p]; seen {
		return ok
	}
	ok := false
	if n.pl != nil && n.pl.Orm != nil {
		c, err := n.pl.query().Model(&Media{}).Where("site_id = ? AND path = ?", n.siteID, p).Count()
		ok = err == nil && c > 0
	}
	n.media[p] = ok
	return ok
}

// hasContent: at least one block with its main field filled (for publishing).
func hasContent(blocks []Block) bool { return (*Plugin)(nil).hasContent(blocks) }

// hasContent also counts the blocks of active plugins.
func (pl *Plugin) hasContent(blocks []Block) bool {
	for _, b := range blocks {
		spec, ok := pl.blockSpec(b.Type)
		if !ok {
			continue
		}
		if spec.Slots > 0 {
			slots, _ := b.Data["children"].([]any)
			for _, s := range slots {
				obj, _ := s.(map[string]any)
				switch inner := obj["blocks"].(type) {
				case []Block:
					if pl.hasContent(inner) {
						return true
					}
				case []any:
					var list []Block
					for _, x := range inner {
						if m, ok := x.(map[string]any); ok {
							t, _ := m["type"].(string)
							d, _ := m["data"].(map[string]any)
							list = append(list, Block{Type: t, Data: d})
						}
					}
					if pl.hasContent(list) {
						return true
					}
				}
			}
			continue
		}
		if spec.Required == "" {
			continue
		}
		switch v := b.Data[spec.Required].(type) {
		case string:
			if strings.TrimSpace(v) != "" {
				return true
			}
		case []any:
			if len(v) > 0 {
				return true
			}
		case int, float64:
			return true
		}
	}
	return false
}
