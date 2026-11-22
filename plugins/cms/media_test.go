package cms

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"reflect"
	"strings"
	"testing"

	mks "github.com/MiranSalehi/mks"
)

func TestCopyWidths(t *testing.T) {
	cases := map[int][]int{
		200:  nil,
		320:  nil,
		321:  {320},
		1000: {320, 640, 960},
		1920: {320, 640, 960, 1280},
		2500: {320, 640, 960, 1280, 1920},
	}
	for w, want := range cases {
		if got := copyWidths(w); !reflect.DeepEqual(got, want) {
			t.Errorf("copyWidths(%d) = %v, want %v", w, got, want)
		}
	}
}

func TestMakeCopiesKeepsAspectRatio(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 500))
	for x := 0; x < 1000; x++ {
		src.Set(x, 250, color.RGBA{255, 0, 0, 255})
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	copies, err := (&Plugin{}).makeCopies(buf.Bytes(), "image/jpeg", 1000, 500)
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 3 {
		t.Fatalf("copies = %d, want 3", len(copies))
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(copies[640].data))
	if err != nil || format != "jpeg" || cfg.Width != 640 || cfg.Height != 320 || copies[640].ext != "" {
		t.Fatalf("640 copy: %v %s %dx%d", err, format, cfg.Width, cfg.Height)
	}
}

func TestImagesRenderWithSrcsetAndAlt(t *testing.T) {
	pl := &Plugin{}
	w, h := 1000, 500
	alt := `A "red" line <b>`
	set := mediaSet{pl: pl, byKey: map[string]Media{"cms/media/a.jpg": {Path: "cms/media/a.jpg", Width: &w, Height: &h, Alt: &alt,
		Conversions: mks.NewJSON(map[string]string{"640": "cms/media/a-640w.jpg", "320": "cms/media/a-320w.jpg"})}}}
	p := "cms/media/a.jpg"
	img := set.get(&p, "fallback")
	if img.Srcset != "/storage/cms/media/a-320w.jpg 320w, /storage/cms/media/a-640w.jpg 640w, /storage/cms/media/a.jpg 1000w" || img.Alt != alt {
		t.Fatalf("image = %#v", img)
	}
	other := "cms/media/missing.jpg"
	if got := set.get(&other, "Title"); got.Src != "/storage/cms/media/missing.jpg" || got.Srcset != "" || got.Alt != "Title" {
		t.Fatalf("image outside the library = %#v", got)
	}

	body, err := pl.renderView(layoutView{List: &listView{Heading: "h", Items: []cardView{{Title: "T", URL: "/x", Image: img}}},
		Post: &postView{Title: "P", Image: img}})
	if err != nil {
		t.Fatal(err)
	}
	html := string(body)
	for _, want := range []string{
		`<img src="/storage/cms/media/a.jpg" srcset="/storage/cms/media/a-320w.jpg 320w, /storage/cms/media/a-640w.jpg 640w, /storage/cms/media/a.jpg 1000w" sizes="(max-width: 600px) 100vw, 22rem" alt="A &#34;red&#34; line &lt;b&gt;" width="1000" height="500" loading="lazy">`,
		`sizes="(max-width: 800px) 100vw, 48rem" alt="A &#34;red&#34; line &lt;b&gt;" class="hero" width="1000" height="500">`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s in\n%s", want, html)
		}
	}
}

// fakeWebP is a WebPEncoder that writes a RIFF/WEBP header and the image's width.
func fakeWebP(widths *[]int) WebPEncoder {
	return func(img image.Image, quality int) ([]byte, error) {
		*widths = append(*widths, img.Bounds().Dx())
		out := []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
		return binary.LittleEndian.AppendUint32(out, uint32(quality)), nil
	}
}

func testJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWebPCopies(t *testing.T) {
	var widths []int
	pl := &Plugin{WebP: fakeWebP(&widths), WebPQuality: 5}
	copies, err := pl.makeCopies(testJPEG(t, 1000, 500), "image/jpeg", 1000, 500)
	if err != nil {
		t.Fatal(err)
	}
	// Every smaller width plus the image's own (MKSine's rule for WebP).
	if !reflect.DeepEqual(widths, []int{320, 640, 960, 1000}) || len(copies) != 4 || copies[1000].ext != ".webp" {
		t.Fatalf("widths %v, copies %d", widths, len(copies))
	}
	if q := binary.LittleEndian.Uint32(copies[320].data[16:]); q != 30 {
		t.Fatalf("quality = %d, want the minimum 30", q)
	}
	// WebP uploads get copies only with an encoder.
	if (&Plugin{}).canCopy("image/webp") || !pl.canCopy("image/webp") || pl.canCopy("image/gif") {
		t.Fatal("canCopy")
	}

	// A failing encoder leaves copies in the original's format.
	pl.WebP = func(image.Image, int) ([]byte, error) { return nil, errors.New("broken") }
	copies, err = pl.makeCopies(testJPEG(t, 1000, 500), "image/jpeg", 1000, 500)
	if err != nil || len(copies) != 3 || copies[640].ext != "" {
		t.Fatalf("fallback: %v %d", err, len(copies))
	}
	// So does one that writes something else.
	pl.WebP = func(image.Image, int) ([]byte, error) { return []byte("<html>"), nil }
	if copies, err = pl.makeCopies(testJPEG(t, 700, 100), "image/jpeg", 700, 100); err != nil || len(copies) != 2 || copies[640].ext != "" {
		t.Fatalf("not webp: %v %d", err, len(copies))
	}
}

func TestCopiesAreUpright(t *testing.T) {
	// A 400×200 JPEG taken with the camera turned (orientation 6) shows 200 wide.
	head := jpegWithOrientation(6, binary.BigEndian, "MM")
	raw := append(head[:len(head)-4:len(head)-4], testJPEG(t, 400, 200)[2:]...)
	copies, err := (&Plugin{}).makeCopies(raw, "image/jpeg", 400, 200)
	if err != nil {
		t.Fatal(err)
	}
	if len(copies) != 0 {
		t.Fatalf("a 200-wide image needs no smaller copies, got %d", len(copies))
	}
	var widths []int
	copies, err = (&Plugin{WebP: fakeWebP(&widths)}).makeCopies(raw, "image/jpeg", 400, 200)
	if err != nil || !reflect.DeepEqual(widths, []int{200}) || len(copies) != 1 {
		t.Fatalf("webp widths %v (%v)", widths, err)
	}
}

func TestWebPCopiesRenderInPicture(t *testing.T) {
	pl := &Plugin{}
	w, h := 1000, 500
	set := mediaSet{pl: pl, byKey: map[string]Media{"cms/media/a.jpg": {Path: "cms/media/a.jpg", Width: &w, Height: &h,
		Conversions: mks.NewJSON(map[string]string{"1000": "cms/media/a-1000w.webp", "320": "cms/media/a-320w.webp"})}}}
	p := "cms/media/a.jpg"
	img := set.get(&p, "A")
	if img.Srcset != "" || img.WebP != "/storage/cms/media/a-320w.webp 320w, /storage/cms/media/a-1000w.webp 1000w" {
		t.Fatalf("image = %#v", img)
	}
	body, err := pl.renderView(layoutView{Post: &postView{Title: "P", Image: img}})
	if err != nil {
		t.Fatal(err)
	}
	want := `<picture><source type="image/webp" srcset="/storage/cms/media/a-320w.webp 320w, /storage/cms/media/a-1000w.webp 1000w" sizes="(max-width: 800px) 100vw, 48rem"><img src="/storage/cms/media/a.jpg" alt="A" class="hero" width="1000" height="500"></picture>`
	if !strings.Contains(string(body), want) {
		t.Fatalf("missing %s in\n%s", want, body)
	}

	// Builder blocks too, with the hero background's alt left empty.
	html := renderBlocks(t, pl, set, `[{"type":"hero","data":{"title":"T","background_image":"cms/media/a.jpg"}},{"type":"image","data":{"image":"cms/media/a.jpg","alt":"B"}}]`)
	for _, want := range []string{
		`<picture><source type="image/webp" srcset="/storage/cms/media/a-320w.webp 320w, /storage/cms/media/a-1000w.webp 1000w" sizes="100vw"><img src="/storage/cms/media/a.jpg" alt="" width="1000" height="500"></picture>`,
		`sizes="(max-width: 800px) 100vw, 48rem"><img src="/storage/cms/media/a.jpg" alt="B" width="1000" height="500" loading="lazy"></picture>`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %s in\n%s", want, html)
		}
	}
}

// renderBlocks renders a builder payload with these library images (no database).
func renderBlocks(t *testing.T, pl *Plugin, set mediaSet, payload string) string {
	t.Helper()
	var blocks []Block
	if err := json.Unmarshal([]byte(payload), &blocks); err != nil {
		t.Fatal(err)
	}
	r := &renderer{pl: pl, media: set, seen: map[uint]bool{}, sections: map[uint][]Block{}}
	h, err := r.blocks(blocks, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	return string(h)
}

func TestCWebP(t *testing.T) {
	if CWebP("no-such-cwebp-tool") != nil {
		t.Fatal("a missing tool should give no encoder")
	}
	enc := CWebP("")
	if enc == nil {
		t.Skip("cwebp is not installed")
	}
	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for x := range 64 {
		img.Set(x, 16, color.RGBA{255, 0, 0, 255})
	}
	data, err := enc(img, 80)
	if err != nil {
		t.Fatal(err)
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "webp" || cfg.Width != 64 || cfg.Height != 32 {
		t.Fatalf("decoded %v %s %dx%d", err, format, cfg.Width, cfg.Height)
	}
}

func TestContentImages(t *testing.T) {
	w, h := 1000, 500
	set := mediaSet{pl: &Plugin{}, byKey: map[string]Media{
		"cms/media/a.jpg": {Path: "cms/media/a.jpg", Width: &w, Height: &h, Conversions: mks.NewJSON(map[string]string{"640": "cms/media/a-640w.jpg"})},
		"cms/media/b.jpg": {Path: "cms/media/b.jpg", Width: &w, Height: &h, Conversions: mks.NewJSON(map[string]string{"640": "cms/media/b-640w.webp"})},
	}}
	if p := libraryPath(`<img src="https://x.test/storage/cms/media/a%20b.jpg?v=1" alt="">`); p != "cms/media/a b.jpg" {
		t.Fatalf("libraryPath = %q", p)
	}
	if p := libraryPath(`<img src="/storage/cms/media/../../etc/passwd">`); p != "" {
		t.Fatalf("a path with .. was taken: %q", p)
	}
	body := `<p><img src="/storage/cms/media/a.jpg" alt="A"></p>` +
		`<p><img src="/storage/cms/media/b.jpg" alt="B" width="10" height="5"/></p>` +
		`<p><img src="https://elsewhere.test/x.jpg" alt="C"></p>` +
		`<picture><img src="/storage/cms/media/a.jpg" alt="D"></picture>`
	out := rewriteContentImages(body, set)
	for _, want := range []string{
		`<img src="/storage/cms/media/a.jpg" alt="A" srcset="/storage/cms/media/a-640w.jpg 640w, /storage/cms/media/a.jpg 1000w" sizes="(max-width: 48rem) 100vw, 48rem" width="1000" height="500" loading="lazy">`,
		`<picture><source type="image/webp" srcset="/storage/cms/media/b-640w.webp 640w" sizes="(max-width: 48rem) 100vw, 48rem"><img src="/storage/cms/media/b.jpg" alt="B" width="10" height="5" loading="lazy"></picture>`,
		`<img src="https://elsewhere.test/x.jpg" alt="C">`,
		`<picture><img src="/storage/cms/media/a.jpg" alt="D"></picture>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in\n%s", want, out)
		}
	}
}
