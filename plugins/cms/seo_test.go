package cms

import (
	"strings"
	"testing"
)

func TestSocialTagsAndJSONLDAreEscaped(t *testing.T) {
	pl := &Plugin{BaseURL: "https://example.com/"}
	req := &request{site: &Site{Name: "Site", Slug: "s"}}
	view := layoutView{
		Title: `Hi </script><script>alert(1)</script>`, Canonical: "/sites/s/post/hi", Description: `d "q"`,
		Post: &postView{Title: `Hi </script><script>alert(1)</script>`, DateISO: "2026-11-21T00:00:00Z", Author: "Ali",
			Image: imageView{Src: "/storage/cms/media/a.jpg"}},
	}
	pl.social(req, &view)
	if view.Social.URL != "https://example.com/sites/s/post/hi" || view.Social.Image != "https://example.com/storage/cms/media/a.jpg" || view.Social.Type != "article" {
		t.Fatalf("social = %#v", view.Social)
	}
	body, err := pl.renderView(view)
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	if strings.Count(html, "<script") != 1 || strings.Contains(html, "<script>alert") {
		t.Fatalf("script injection through the title:\n%s", html)
	}
	for _, want := range []string{
		`<meta property="og:type" content="article">`,
		`<meta property="og:url" content="https://example.com/sites/s/post/hi">`,
		`<meta property="og:image" content="https://example.com/storage/cms/media/a.jpg">`,
		`<meta name="twitter:card" content="summary_large_image">`,
		`"@type":"BlogPosting"`,
		`<meta property="og:description" content="d &#34;q&#34;">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s", want)
		}
	}
	if absolute("", "/x") != "" || absolute("https://a.b", "https://c.d/x") != "https://c.d/x" {
		t.Error("absolute")
	}
}
