package cms

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseImportFormats(t *testing.T) {
	csvRows, err := ParseImport("csv", []byte("\xef\xbb\xbftitle,slug,status\n日本語,hello,publish\n\"With, comma\",two,draft\n"))
	if err != nil || len(csvRows) != 2 || csvRows[0]["title"] != "日本語" || csvRows[1]["title"] != "With, comma" {
		t.Fatalf("csv = %v, %v", csvRows, err)
	}
	jsonRows, err := ParseImport("json", []byte(`[{"title":"a","size":12,"ok":true,"nested":{"x":1}}, 5]`))
	if err != nil || len(jsonRows) != 1 || jsonRows[0]["size"] != "12" || jsonRows[0]["ok"] != "true" {
		t.Fatalf("json = %v, %v", jsonRows, err)
	}
	if _, has := jsonRows[0]["nested"]; has {
		t.Fatal("nested values are left out")
	}
	one, err := ParseImport("json", []byte(`{"title":"single"}`))
	if err != nil || len(one) != 1 {
		t.Fatalf("a single object is one row: %v %v", one, err)
	}
	xmlRows, err := ParseImport("xml", []byte(`<rows><row><title>x &amp; y</title><slug>xy</slug></row><row><title><![CDATA[<b>z</b>]]></title></row></rows>`))
	if err != nil || len(xmlRows) != 2 || xmlRows[0]["title"] != "x & y" || xmlRows[1]["title"] != "<b>z</b>" {
		t.Fatalf("xml = %v, %v", xmlRows, err)
	}
	for _, f := range []string{"csv", "json", "xml", "wxr"} {
		if f == "csv" {
			continue
		}
		if _, err := ParseImport(f, []byte("not valid <")); err == nil {
			t.Errorf("%s: broken input must fail", f)
		}
	}
	if _, err := ParseImport("yaml", []byte("a: 1")); err == nil {
		t.Error("unknown formats fail")
	}
	if _, err := ParseImport("json", make([]byte, ImportMaxBytes+1)); err == nil {
		t.Error("files over the limit fail")
	}
}

func TestParseWXR(t *testing.T) {
	wxr := `<?xml version="1.0"?>
<rss xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:excerpt="http://wordpress.org/export/1.2/excerpt/" xmlns:wp="http://wordpress.org/export/1.2/">
<channel>
<item><title>Hello</title><content:encoded><![CDATA[<p>Body</p>]]></content:encoded><excerpt:encoded><![CDATA[Short]]></excerpt:encoded>
<wp:post_name>hello-world</wp:post_name><wp:status>publish</wp:status><wp:post_type>post</wp:post_type><wp:post_date>2024-05-01 10:00:00</wp:post_date></item>
<item><title>About</title><wp:post_name>about</wp:post_name><wp:post_type>page</wp:post_type><wp:status>draft</wp:status></item>
<item><title>Logo</title><wp:post_type>attachment</wp:post_type><wp:attachment_url>https://example.com/logo.png</wp:attachment_url></item>
<item><title>Menu</title><wp:post_type>nav_menu_item</wp:post_type></item>
</channel></rss>`
	rows, err := ParseImport("wxr", []byte(wxr))
	if err != nil || len(rows) != 3 {
		t.Fatalf("wxr = %v, %v", rows, err)
	}
	want := ImportRow{"_entity": "post", "title": "Hello", "slug": "hello-world", "content": "<p>Body</p>", "excerpt": "Short", "status": "publish", "published_at": "2024-05-01 10:00:00"}
	if !reflect.DeepEqual(rows[0], want) {
		t.Fatalf("post row = %v", rows[0])
	}
	if rows[1]["_entity"] != "page" || rows[2]["_entity"] != "media" || rows[2]["url"] != "https://example.com/logo.png" {
		t.Fatalf("rows = %v", rows)
	}
}

func TestMapImportRow(t *testing.T) {
	row := ImportRow{"Headline": "T", "Body": "B", "slug": "s", "other": "x"}
	got := MapImportRow(row, map[string]string{"Headline": "title", "Body": "content"})
	if !reflect.DeepEqual(got, ImportRow{"title": "T", "content": "B", "slug": "s"}) {
		t.Fatalf("mapped = %v", got)
	}
	if !reflect.DeepEqual(MapImportRow(row, nil), row) {
		t.Fatal("no mapping keeps the row")
	}
}

func TestImportHelpers(t *testing.T) {
	for in, want := range map[string]string{"publish": "published", "future": "published", "trash": "archived", "private": "draft", "": "draft"} {
		if got := importStatus(ImportPost, in); got != want {
			t.Errorf("post status %q = %q, want %q", in, got, want)
		}
	}
	if importStatus(ImportPage, "future") != "scheduled" || importStatus(ImportPage, "archived") != "draft" {
		t.Error("page statuses")
	}
	if importSlug(ImportRow{"slug": "My Post"}) != "my-post" || importSlug(ImportRow{"title": "Go 1.25"}) != "go-1-25" {
		t.Error("slugs come from the slug, then the title")
	}
	if s := importSlug(ImportRow{"title": "日本語"}); !strings.HasPrefix(s, "item-") || len(s) != 13 {
		t.Errorf("a title without Latin letters gets a random slug: %q", s)
	}
	for _, in := range []string{"2024-05-01 10:00:00", "2024-05-01", "2024-05-01T10:00:00Z"} {
		if tm, ok := importTime(in); !ok || tm == nil || tm.Year() != 2024 {
			t.Errorf("time %q = %v, %v", in, tm, ok)
		}
	}
	if tm, ok := importTime("0000-00-00 00:00:00"); !ok || tm != nil {
		t.Error("WordPress's zero date is no date")
	}
	if _, ok := importTime("yesterday"); ok {
		t.Error("unknown dates are errors")
	}
}

func TestCSVFormulaGuard(t *testing.T) {
	for _, s := range []string{"=SUM(A1)", "+1", "-x", "@cmd"} {
		if got := csvSafe(s); got != "'"+s || unCSVSafe(got) != s {
			t.Errorf("%q: guarded %q, back %q", s, got, unCSVSafe(got))
		}
	}
	if csvSafe("plain") != "plain" || unCSVSafe("'quoted") != "'quoted" {
		t.Error("other text is untouched")
	}
}

func TestExportTimeRoundTrip(t *testing.T) {
	when, _ := importTime("2030-01-01T00:00:00+03:30")
	back, ok := importTime(exportText(when))
	if !ok || back == nil || !back.Equal(*when) {
		t.Fatalf("round trip %v -> %q -> %v", when, exportText(when), back)
	}
}

func TestImportSkipsRowsWithoutTitleOrSlug(t *testing.T) {
	pl := testPlugin()
	for _, row := range []ImportRow{{"title": "", "slug": ""}, {"content": "only text"}} {
		_, err := pl.importRow(nil, nil, 1, ImportPost, row, true)
		if _, ok := err.(errSkip); !ok {
			t.Errorf("%v: err = %v, want a skip", row, err)
		}
	}
}
