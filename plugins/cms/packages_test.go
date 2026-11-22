package cms

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type zipEntry struct {
	name string
	body string
	link bool
}

func makeZip(t *testing.T, entries ...zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		h := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.link {
			h.SetMode(0o777 | 1<<27) // os.ModeSymlink
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func manifestJSON(t *testing.T, m map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestReadPackage(t *testing.T) {
	theme := manifestJSON(t, map[string]any{"id": "aurora", "name": "Aurora", "version": "1.0.0"})
	// The manifest may sit inside one top folder; macOS junk is skipped.
	files, err := readPackage(makeZip(t,
		zipEntry{name: "aurora/theme.json", body: theme},
		zipEntry{name: "aurora/assets/style.css", body: "body{}"},
		zipEntry{name: "__MACOSX/aurora/._theme.json", body: "x"},
		zipEntry{name: "aurora/.DS_Store", body: "x"},
	), "theme.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files["assets/style.css"] == nil || files["theme.json"] == nil {
		t.Fatalf("files = %v", files)
	}

	for name, z := range map[string][]byte{
		"traversal":     makeZip(t, zipEntry{name: "theme.json", body: theme}, zipEntry{name: "../../evil.html", body: "x"}),
		"absolute":      makeZip(t, zipEntry{name: "theme.json", body: theme}, zipEntry{name: "/etc/x.html", body: "x"}),
		"drive":         makeZip(t, zipEntry{name: "theme.json", body: theme}, zipEntry{name: "C:/x.html", body: "x"}),
		"backslash":     makeZip(t, zipEntry{name: "theme.json", body: theme}, zipEntry{name: `..\x.html`, body: "x"}),
		"server code":   makeZip(t, zipEntry{name: "theme.json", body: theme}, zipEntry{name: "run.php", body: "<?php"}),
		"binary":        makeZip(t, zipEntry{name: "theme.json", body: theme}, zipEntry{name: "tool.exe", body: "MZ"}),
		"hidden":        makeZip(t, zipEntry{name: "theme.json", body: theme}, zipEntry{name: ".env.json", body: "{}"}),
		"symlink":       makeZip(t, zipEntry{name: "theme.json", body: theme}, zipEntry{name: "assets/a.css", body: "/etc/passwd", link: true}),
		"no manifest":   makeZip(t, zipEntry{name: "assets/a.css", body: "x"}),
		"two manifests": makeZip(t, zipEntry{name: "a/theme.json", body: theme}, zipEntry{name: "b/theme.json", body: theme}),
		"not a zip":     []byte("hello"),
	} {
		if _, err := readPackage(z, "theme.json"); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	big := make([]byte, MaxPackageSize+1)
	if _, err := readPackage(big, "theme.json"); err == nil {
		t.Error("an oversized file was accepted")
	}
}

func TestCompareVersions(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0}, {"1.10.0", "1.9.9", 1}, {"1.0", "1.0.1", -1}, {"2", "1.99", 1},
		{"1.0.0-beta", "1.0.0", -1}, {"1.0.0", "1.0.0-rc1", 1}, {"1.0.0-alpha", "1.0.0-beta", -1},
	} {
		if got := compareVersions(c.a, c.b); got != c.want {
			t.Errorf("compareVersions(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestCheckTheme(t *testing.T) {
	ok := map[string][]byte{
		"theme.json":         []byte(manifestJSON(t, map[string]any{"id": "aurora", "name": "Aurora", "version": "1.2.0", "swatch": "#7c3aed", "assets": map[string]any{"css": []string{"assets/a.css"}}})),
		"assets/a.css":       []byte("body{color:red}"),
		"templates/x.html":   []byte(`{{define "terms"}}<h1>{{.Heading}}</h1>{{end}}`),
		"templates/y.tmpl":   []byte(`{{define "z"}}z{{end}}`),
		"templates/skip.css": []byte(`not a template`),
	}
	m, err := checkTheme(ok["theme.json"], ok)
	if err != nil || m.ID != "aurora" {
		t.Fatalf("valid theme: %v", err)
	}
	if text := themeTemplateText(ok); !strings.Contains(text, `define "terms"`) || strings.Contains(text, "not a template") {
		t.Fatalf("template text = %q", text)
	}
	bad := func(name string, change func(m map[string]any, files map[string][]byte)) {
		t.Helper()
		m := map[string]any{"id": "aurora", "name": "Aurora", "version": "1.0.0"}
		files := map[string][]byte{}
		change(m, files)
		raw := []byte(manifestJSON(t, m))
		files["theme.json"] = raw
		if _, err := checkTheme(raw, files); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	bad("id", func(m map[string]any, _ map[string][]byte) { m["id"] = "Aurora Theme" })
	bad("version", func(m map[string]any, _ map[string][]byte) { m["version"] = "latest" })
	bad("name", func(m map[string]any, _ map[string][]byte) { m["name"] = "" })
	bad("missing asset", func(m map[string]any, _ map[string][]byte) {
		m["assets"] = map[string]any{"css": []string{"assets/none.css"}}
	})
	bad("asset outside assets/", func(m map[string]any, f map[string][]byte) {
		f["x.css"] = []byte("a{}")
		m["assets"] = map[string]any{"css": []string{"x.css"}}
	})
	bad("css closing style", func(m map[string]any, f map[string][]byte) {
		f["assets/a.css"] = []byte("</STYLE><script>alert(1)</script>")
		m["assets"] = map[string]any{"css": []string{"assets/a.css"}}
	})
	bad("broken template", func(_ map[string]any, f map[string][]byte) { f["templates/x.html"] = []byte(`{{define "x"}}{{.A`) })
	bad("swatch", func(m map[string]any, _ map[string][]byte) { m["swatch"] = "red;background:url(x)" })
	bad("location", func(m map[string]any, _ map[string][]byte) {
		m["locations"] = []any{map[string]any{"key": "Bad Key", "label": "x"}}
	})
}

func TestCheckPlugin(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{"id": "notice", "name": "Notice", "version": "1.0.0",
			"blocks":     []any{map[string]any{"type": "box", "label": "Box", "template": "blocks/box.html", "fields": []any{map[string]any{"name": "title", "label": "Title", "kind": "text"}}}},
			"actions":    []any{map[string]any{"hook": "single.after_article", "template": "a.html"}},
			"shortcodes": []any{map[string]any{"tag": "note", "template": "s.html"}},
			"settings":   []any{map[string]any{"name": "accent", "label": "Accent", "kind": "color"}},
		}
	}
	files := map[string][]byte{"blocks/box.html": []byte(`<b>{{.D.title}}</b>`), "a.html": []byte(`<p>{{.Settings.accent}}</p>`), "s.html": []byte(`{{.Content}}`)}
	m, err := checkPlugin([]byte(manifestJSON(t, base())), files)
	if err != nil || len(m.Blocks) != 1 {
		t.Fatalf("valid plugin: %v", err)
	}
	if got := PackageBlockType("notice", "box"); got != "pkg-notice-box" {
		t.Fatalf("block type %s", got)
	}
	bad := func(name string, change func(m map[string]any)) {
		t.Helper()
		m := base()
		change(m)
		if _, err := checkPlugin([]byte(manifestJSON(t, m)), files); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	block := func(m map[string]any) map[string]any { return m["blocks"].([]any)[0].(map[string]any) }
	bad("block type", func(m map[string]any) { block(m)["type"] = "Box!" })
	bad("missing template", func(m map[string]any) { block(m)["template"] = "none.html" })
	bad("field kind", func(m map[string]any) {
		block(m)["fields"] = []any{map[string]any{"name": "x", "label": "X", "kind": "script"}}
	})
	bad("style name", func(m map[string]any) {
		block(m)["fields"] = []any{map[string]any{"name": "anchor", "label": "X", "kind": "text"}}
	})
	bad("enum without options", func(m map[string]any) {
		block(m)["fields"] = []any{map[string]any{"name": "x", "label": "X", "kind": "enum"}}
	})
	bad("category", func(m map[string]any) { block(m)["category"] = "secret" })
	bad("hook name", func(m map[string]any) { m["actions"].([]any)[0].(map[string]any)["hook"] = "<script>" })
	bad("shortcode tag", func(m map[string]any) { m["shortcodes"].([]any)[0].(map[string]any)["tag"] = "No Tag" })
	bad("requires", func(m map[string]any) { m["requires"] = []any{"../x"} })
	ct := func(key, singular, archive, single string) []any {
		return []any{map[string]any{"key": key, "singular": singular, "plural": "Xs", "archive_uri": archive, "single_uri": single}}
	}
	bad("reserved content type", func(m map[string]any) { m["content_types"] = ct("posts", "X", "", "") })
	bad("content type key", func(m map[string]any) { m["content_types"] = ct("Bad Key", "X", "", "") })
	bad("content type label", func(m map[string]any) { m["content_types"] = ct("book", "", "", "") })
	bad("content type address", func(m map[string]any) { m["content_types"] = ct("book", "X", "/Books!", "") })
	bad("single address without slug", func(m map[string]any) { m["content_types"] = ct("book", "X", "", "/books/all") })
	good := base()
	good["content_types"] = ct("book", "Book", "/books", "/books/{slug}")
	if m, err := checkPlugin([]byte(manifestJSON(t, good)), files); err != nil || len(m.ContentTypes) != 1 {
		t.Fatalf("valid content type: %v", err)
	}
}

func TestPluginSettings(t *testing.T) {
	if k := pluginSettingKey("notice", "accent"); strings.Contains(k, ".") {
		t.Fatalf("setting key %q has a dot (forms read it as nested)", k)
	}
	cases := []struct {
		f    pkgField
		in   string
		ok   bool
		want string
	}{
		{pkgField{Kind: "bool"}, "1", true, "1"},
		{pkgField{Kind: "bool"}, "yes", false, ""},
		{pkgField{Kind: "int", Min: 1, Max: 5}, "6", false, ""},
		{pkgField{Kind: "int"}, "42", true, "42"},
		{pkgField{Kind: "enum", Options: map[string]string{"a": "A"}}, "b", false, ""},
		{pkgField{Kind: "url"}, "javascript:alert(1)", false, ""},
		{pkgField{Kind: "color"}, "#abc", true, "#abc"},
		{pkgField{Kind: "color"}, "red", false, ""},
		{pkgField{Kind: "html"}, `<p onclick="x">hi</p><script>x</script>`, true, "<p>hi</p>"},
		{pkgField{Kind: "text", Required: true}, " ", false, ""},
	}
	for _, c := range cases {
		got, msg := checkPluginSetting(c.f, c.in)
		if (msg == "") != c.ok || (c.ok && got != c.want) {
			t.Errorf("%s %q: got %q, %q", c.f.Kind, c.in, got, msg)
		}
	}
}

func TestSpecFieldsLimits(t *testing.T) {
	specs := specFields([]pkgField{{Name: "a", Kind: "text"}, {Name: "b", Kind: "text", Max: 99999}, {Name: "c", Kind: "enum", Options: map[string]string{"z": "Z", "y": "Y"}}})
	if specs[0].Max != 255 || specs[1].Max != 255 || strings.Join(specs[2].Options, ",") != "y,z" {
		t.Fatalf("specs = %+v", specs)
	}
}
