package cms

import (
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Location is a place on the public site that shows a menu.
type Location struct {
	Key   string
	Label string
}

// DefaultLocations are the menu locations of the built-in site theme.
var DefaultLocations = []Location{
	{Key: "header", Label: "منوی بالای سایت"},
	{Key: "footer", Label: "منوی پایین سایت"},
}

func (pl *Plugin) locations() []Location {
	if len(pl.Locations) > 0 {
		return pl.Locations
	}
	return DefaultLocations
}

func (pl *Plugin) isLocation(key string) bool {
	for _, l := range pl.locations() {
		if l.Key == key {
			return true
		}
	}
	return false
}

var persianDigits = strings.NewReplacer("0", "۰", "1", "۱", "2", "۲", "3", "۳", "4", "۴", "5", "۵", "6", "۶", "7", "۷", "8", "۸", "9", "۹")

// faDigits formats a number with Persian digits.
func faDigits(n int) string { return persianDigits.Replace(strconv.Itoa(n)) }

// MaxURLLength bounds a custom menu link.
const MaxURLLength = 2048

// SafeLinkURL reports whether a custom menu link may be stored and rendered: an absolute
// http(s) URL, a mailto: or tel: link, a path on this host ("/about") or a fragment
// ("#contact"). Everything else is refused, including javascript: and data: URLs,
// protocol-relative "//host" and "/\host" (browsers treat the backslash as a slash),
// whitespace and control characters.
func SafeLinkURL(raw string) bool {
	if raw == "" || len(raw) > MaxURLLength || !utf8.ValidString(raw) {
		return false
	}
	for _, r := range raw {
		if r <= ' ' || r == 0x7f || r == '\\' {
			return false
		}
	}
	switch raw[0] {
	case '/':
		return len(raw) == 1 || raw[1] != '/'
	case '#':
		return true
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != "" && u.User == nil
	case "mailto", "tel":
		return u.Opaque != ""
	}
	return false
}

// MenuRefs are the records a menu points at, by kind, without repeats.
type MenuRefs struct {
	Pages, Posts, Categories, Tags []uint
}

// Empty reports whether the menu points at no record.
func (r MenuRefs) Empty() bool {
	return len(r.Pages)+len(r.Posts)+len(r.Categories)+len(r.Tags) == 0
}

// refOf is the kind and id an entry points at ("" for links and groups).
func refOf(kind string, page, post, category, tag *uint) (string, *uint) {
	switch kind {
	case "page":
		return kind, page
	case "post":
		return kind, post
	case "category":
		return kind, category
	case "tag":
		return kind, tag
	}
	return "", nil
}

// Refs collects the records of a menu.
func Refs(items []MenuItem) MenuRefs {
	var r MenuRefs
	seen := map[string]bool{}
	add := func(kind string, id *uint) {
		if id == nil || *id == 0 {
			return
		}
		key := kind + ":" + strconv.FormatUint(uint64(*id), 10)
		if seen[key] {
			return
		}
		seen[key] = true
		switch kind {
		case "page":
			r.Pages = append(r.Pages, *id)
		case "post":
			r.Posts = append(r.Posts, *id)
		case "category":
			r.Categories = append(r.Categories, *id)
		case "tag":
			r.Tags = append(r.Tags, *id)
		}
	}
	var walk func(list []MenuItem)
	walk = func(list []MenuItem) {
		for _, item := range list {
			add(refOf(item.Type, item.PageID, item.PostID, item.CategoryID, item.TagID))
			walk(item.Children)
		}
	}
	walk(items)
	return r
}

// NavTarget is what a menu needs of a publicly visible record.
type NavTarget struct {
	URL   string
	Title string
}

// NavTargets are the records that are publicly visible right now, by kind and id. A menu
// entry for anything else (a draft, a trashed page, an inactive category) is left out.
type NavTargets map[string]map[uint]NavTarget

func (t NavTargets) get(kind string, id *uint) (NavTarget, bool) {
	if id == nil {
		return NavTarget{}, false
	}
	target, ok := t[kind][*id]
	return target, ok
}

// NavItem is a menu entry as the site shows it: a link, or (with Children) a submenu
// trigger that is not a link itself.
type NavItem struct {
	Label  string
	URL    string
	NewTab bool
	// Current is the entry of the page being viewed; on a trigger, one of its children.
	Current  bool
	Children []NavItem
}

// BuildNav turns stored menu items into what the site shows. Entries pointing at records
// that are not public right now, unsafe links, entries without a label and groups without
// visible children are left out. An entry with visible children opens its submenu instead
// of being a link (as in MKSine). current is the address of the page being viewed.
func BuildNav(items []MenuItem, targets NavTargets, current string) []NavItem {
	var out []NavItem
	for _, item := range items {
		children := BuildNav(item.Children, targets, current)
		if len(children) > 0 {
			label := strings.TrimSpace(item.Label)
			if label == "" {
				kind, id := refOf(item.Type, item.PageID, item.PostID, item.CategoryID, item.TagID)
				target, _ := targets.get(kind, id)
				label = target.Title
			}
			if label == "" {
				continue
			}
			entry := NavItem{Label: label, Children: children}
			for _, c := range children {
				entry.Current = entry.Current || c.Current
			}
			out = append(out, entry)
			continue
		}
		if link, ok := navLink(item.Type, item.Label, item.URL, item.NewTab, targets, current, item.PageID, item.PostID, item.CategoryID, item.TagID); ok {
			out = append(out, link)
		}
	}
	return out
}

func navLink(kind, label, link string, newTab bool, targets NavTargets, current string, page, post, category, tag *uint) (NavItem, bool) {
	label = strings.TrimSpace(label)
	switch kind {
	case "page", "post", "category", "tag":
		target, ok := targets.get(refOf(kind, page, post, category, tag))
		if !ok {
			return NavItem{}, false
		}
		if label == "" {
			label = target.Title
		}
		link = target.URL
	case "link":
		if !SafeLinkURL(link) {
			return NavItem{}, false
		}
	default:
		return NavItem{}, false
	}
	if label == "" {
		return NavItem{}, false
	}
	return NavItem{Label: label, URL: link, NewTab: newTab, Current: current != "" && link == current}, true
}
