package cms

import (
	"html"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	contentImgRE  = regexp.MustCompile(`(?is)(<picture\b.*?</picture>)|<img\b[^>]*>`)
	contentSrcRE  = regexp.MustCompile(`(?i)\ssrc="([^"]*)"`)
	contentAttrRE = regexp.MustCompile(`(?i)\s(srcset|width|height|loading)=`)
)

// contentSizes is the sizes attribute of images in a body (the text column).
const contentSizes = "(max-width: 48rem) 100vw, 48rem"

// responsiveContent gives the library images of a body their smaller copies: a srcset in
// the original's format, or a <picture> with the WebP copies; and their width, height and
// lazy loading when missing. Other images are left as they are. The HTML is the cleaned
// body (attributes in double quotes).
func (pl *Plugin) responsiveContent(req *request, body string) string {
	if req == nil || req.site == nil || pl.Orm == nil || !strings.Contains(body, "<img") {
		return body
	}
	var paths []*string
	for _, m := range contentImgRE.FindAllStringSubmatch(body, -1) {
		if m[1] != "" {
			continue // already a <picture>
		}
		if p := libraryPath(m[0]); p != "" {
			paths = append(paths, &p)
		}
	}
	if len(paths) == 0 {
		return body
	}
	set, err := pl.mediaImages(req, paths...)
	if err != nil {
		pl.logf("cms: content images: %v", err)
		return body
	}
	return rewriteContentImages(body, set)
}

// rewriteContentImages adds the copies of the images that are in set.
func rewriteContentImages(body string, set mediaSet) string {
	return contentImgRE.ReplaceAllStringFunc(body, func(tag string) string {
		if strings.HasPrefix(strings.ToLower(tag), "<picture") {
			return tag
		}
		p := libraryPath(tag)
		if _, ok := set.byKey[p]; !ok {
			return tag
		}
		img := set.get(&p, "")
		extra := ""
		has := map[string]bool{}
		for _, a := range contentAttrRE.FindAllStringSubmatch(tag, -1) {
			has[strings.ToLower(a[1])] = true
		}
		if img.Srcset != "" && !has["srcset"] {
			extra += ` srcset="` + html.EscapeString(img.Srcset) + `" sizes="` + contentSizes + `"`
		}
		if img.Width > 0 && !has["width"] && !has["height"] {
			extra += ` width="` + strconv.Itoa(img.Width) + `" height="` + strconv.Itoa(img.Height) + `"`
		}
		if !has["loading"] {
			extra += ` loading="lazy"`
		}
		end := strings.TrimSuffix(strings.TrimSuffix(tag, ">"), "/")
		out := end + extra + ">"
		if img.WebP != "" && !has["srcset"] {
			out = `<picture><source type="image/webp" srcset="` + html.EscapeString(img.WebP) + `" sizes="` + contentSizes + `">` + out + `</picture>`
		}
		return out
	})
}

// libraryPath is the media library path an <img> shows ("cms/media/..."), or "".
func libraryPath(tag string) string {
	m := contentSrcRE.FindStringSubmatch(tag)
	if m == nil {
		return ""
	}
	src := html.UnescapeString(m[1])
	i := strings.Index(src, "cms/media/")
	if i < 0 {
		return ""
	}
	p := src[i:]
	if j := strings.IndexAny(p, "?#"); j >= 0 {
		p = p[:j]
	}
	if u, err := url.PathUnescape(p); err == nil {
		p = u
	}
	if strings.Contains(p, "..") {
		return ""
	}
	return p
}
