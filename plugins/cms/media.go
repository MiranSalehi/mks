package cms

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	nethttp "net/http"
	"path"
	"sort"
	"strconv"
	"strings"

	_ "golang.org/x/image/webp"
	_ "image/gif" // dimensions of GIFs

	"github.com/goravel/framework/contracts/filesystem"
	"golang.org/x/image/draw"

	mks "github.com/MiranSalehi/mks"
)

// ManageMedia: upload, edit and delete files of the media library.
const ManageMedia Ability = "cms.media.manage"

// Image copies, as in MKSine: every width below the original, plus the original width
// capped at the largest. They are WebP when the plugin has a WebP encoder (Plugin.WebP);
// otherwise they keep the original's format (JPEG or PNG).
var (
	ResponsiveWidths   = []int{320, 640, 960, 1280, 1920}
	maxConvertPixels   = 40_000_000
	conversionJPEGRate = 82
)

// MediaAccept are the file types the library takes (detected from the content).
var MediaAccept = []string{"image/jpeg", "image/png", "image/webp", "image/gif", "application/pdf",
	"video/mp4", "video/webm", "audio/mpeg", "audio/wave", "application/ogg"}

func (pl *Plugin) disk() string {
	if pl.Disk != "" {
		return pl.Disk
	}
	return "public"
}

func (pl *Plugin) driver() filesystem.Driver {
	if pl.Storage == nil {
		return nil
	}
	return pl.Storage.Disk(pl.disk())
}

type mediaPathKey struct{}

func (pl *Plugin) mediaResource() mks.ModelResource[Media] {
	manage := func(ctx *mks.HookContext, _ *Media) bool { return pl.can(ctx, ManageMedia) }
	return mks.Define[Media](mks.Config{
		Name:        "cms_media",
		Tenant:      "site_id",
		Searchable:  []string{"file_name", "title", "alt"},
		Sortable:    []string{"id", "file_name", "size", "created_at"},
		Filterable:  map[string][]string{"mime_type": {"eq", "in"}, "id": {"in"}, "path": {"eq"}},
		Fillable:    []string{"path", "alt", "title", "caption"},
		TitleColumn: "file_name",
		DefaultSort: "-created_at",
		Deletable:   true,
		Uploads: map[string]mks.Upload{"path": {Disk: pl.disk(), Directory: "cms/media", Accept: MediaAccept,
			MaxSize: pl.mediaMaxSize(), NameField: "file_name"}},
		Rules: map[string]string{
			"path":    "required",
			"alt":     "max_len:255",
			"title":   "max_len:255",
			"caption": "max_len:2000",
		},
	}).
		BeforeCreate(func(ctx *mks.HookContext, data map[string]any) error {
			data["uploaded_by"] = fmt.Sprint(ctx.UserID())
			return nil
		}).
		BeforeUpdate(func(ctx *mks.HookContext, m *Media, data map[string]any) error {
			// A new file: forget the old copies after the save.
			if v, sent := data["path"]; sent && fmt.Sprint(v) != m.Path && ctx.HTTP != nil {
				ctx.HTTP.WithValue(mediaPathKey{}, m.Conversions.V)
			}
			return nil
		}).
		AfterSave(func(ctx *mks.HookContext, m *Media, created bool) error {
			old, replaced := map[string]string(nil), false
			if ctx.HTTP != nil {
				old, replaced = ctx.HTTP.Value(mediaPathKey{}).(map[string]string)
			}
			if !created && !replaced {
				return nil
			}
			info := pl.analyzeMedia(m.Path)
			if replaced {
				pl.deleteFiles(old)
			}
			name := m.FileName
			if name == "" {
				name = info.fileName
			}
			_, err := ctx.Query.Model(&Media{}).Where("id = ?", m.ID).Update(map[string]any{
				"file_name": trimTo(name, 255), "mime_type": info.mime, "size": info.size,
				"width": info.width, "height": info.height, "conversions": mks.NewJSON(info.conversions),
			})
			return err
		}).
		BeforeDelete(func(_ *mks.HookContext, m *Media) error {
			// The panel removes the file itself; the copies are ours.
			pl.deleteFiles(m.Conversions.V)
			return nil
		}).
		Actions(mks.Action[Media]{
			Name: "edit_image",
			Authorize: func(ctx *mks.HookContext, m *Media) bool {
				return m != nil && pl.canEdit(m.MimeType) && pl.can(ctx, ManageMedia)
			},
			Handle: bumping(pl, pl.editImage),
		}).
		Policy(mks.Policy[Media]{
			Create: func(ctx *mks.HookContext) bool { return pl.can(ctx, ManageMedia) },
			Update: manage,
			Delete: manage,
		})
}

func (pl *Plugin) mediaMaxSize() int64 {
	if pl.MediaMaxSize > 0 {
		return pl.MediaMaxSize
	}
	return 20 << 20
}

func (pl *Plugin) deleteFiles(files map[string]string) {
	d := pl.driver()
	if d == nil {
		return
	}
	for _, f := range files {
		if strings.HasPrefix(f, "cms/media/") && !strings.Contains(f, "..") {
			_ = d.Delete(f)
		}
	}
}

type mediaInfo struct {
	fileName    string
	mime        string
	size        int64
	width       *int
	height      *int
	conversions map[string]string
}

// analyzeMedia reads a stored file: its type, size and, for images, dimensions and the
// smaller copies. Failures leave the file as it is (a library entry without copies).
func (pl *Plugin) analyzeMedia(stored string) mediaInfo {
	info := mediaInfo{fileName: path.Base(stored), conversions: map[string]string{}}
	d := pl.driver()
	if d == nil {
		return info
	}
	content, err := d.Get(stored)
	if err != nil {
		pl.logf("cms: read media %s: %v", stored, err)
		return info
	}
	raw := []byte(content)
	info.size = int64(len(raw))
	info.mime = strings.SplitN(nethttp.DetectContentType(raw), ";", 2)[0]
	if !strings.HasPrefix(info.mime, "image/") {
		return info
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return info
	}
	w, h := cfg.Width, cfg.Height
	if info.mime == "image/jpeg" && exifOrientation(raw) >= 5 {
		// Turned a quarter: browsers show it h wide.
		w, h = h, w
	}
	info.width, info.height = &w, &h
	if !pl.canCopy(info.mime) {
		return info
	}
	copies, err := pl.makeCopies(raw, info.mime, cfg.Width, cfg.Height)
	if err != nil {
		pl.logf("cms: resize media %s: %v", stored, err)
		return info
	}
	base := strings.TrimSuffix(stored, path.Ext(stored))
	for cw, c := range copies {
		ext := c.ext
		if ext == "" {
			ext = path.Ext(stored)
		}
		target := base + "-" + strconv.Itoa(cw) + "w" + ext
		if target == stored {
			continue
		}
		if err := d.Put(target, string(c.data)); err != nil {
			pl.logf("cms: write media copy %s: %v", target, err)
			continue
		}
		info.conversions[strconv.Itoa(cw)] = target
	}
	return info
}

// canCopy says whether images of this type get smaller copies: JPEG and PNG always, WebP
// when there is an encoder for it.
func (pl *Plugin) canCopy(mime string) bool {
	return mime == "image/jpeg" || mime == "image/png" || (mime == "image/webp" && pl.WebP != nil)
}

// canEdit says whether the image editor can save images of this type (as canCopy).
func (pl *Plugin) canEdit(mime string) bool { return pl.canCopy(mime) }

// copyWidths are the widths to make for an image of width w (MKSine's rule).
func copyWidths(w int) []int {
	var out []int
	for _, cw := range ResponsiveWidths {
		if cw < w {
			out = append(out, cw)
		}
	}
	largest := ResponsiveWidths[len(ResponsiveWidths)-1]
	top := min(w, largest)
	if len(out) == 0 || out[len(out)-1] != top {
		// The original width itself needs no copy when it is the full image.
		if top < w {
			out = append(out, top)
		}
	}
	return out
}

// webpWidths are copyWidths plus one at the image's own width (capped at the largest):
// a WebP of the full image is smaller than the original, and the WebP srcset cannot fall
// back to the original's format.
func webpWidths(w int) []int {
	out := copyWidths(w)
	if top := min(w, ResponsiveWidths[len(ResponsiveWidths)-1]); len(out) == 0 || out[len(out)-1] != top {
		out = append(out, top)
	}
	return out
}

type imageCopy struct {
	data []byte
	ext  string // ".webp"; empty keeps the original's extension
}

// makeCopies are the smaller copies of an image w×h pixels (as stored, before any EXIF
// orientation), by width. JPEGs are turned upright first, since the copies carry no EXIF.
func (pl *Plugin) makeCopies(raw []byte, mime string, w, h int) (map[int]imageCopy, error) {
	out := map[int]imageCopy{}
	if w*h > maxConvertPixels {
		return out, nil
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	if mime == "image/jpeg" {
		src = upright(src, exifOrientation(raw))
	}
	w, h = src.Bounds().Dx(), src.Bounds().Dy()
	scaled := func(cw int) image.Image {
		if cw == w {
			return src
		}
		dst := image.NewRGBA(image.Rect(0, 0, cw, max(1, h*cw/w)))
		draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Over, nil)
		return dst
	}
	if pl.WebP != nil {
		q := pl.webpQuality()
		for _, cw := range webpWidths(w) {
			data, err := pl.WebP(scaled(cw), q)
			if err != nil || !strings.HasPrefix(nethttp.DetectContentType(data), "image/webp") {
				if err == nil {
					err = fmt.Errorf("not a WebP image")
				}
				if mime == "image/webp" {
					return nil, err
				}
				// One format for all copies: fall back to the original's.
				pl.logf("cms: webp copy: %v; keeping %s copies", err, mime)
				clear(out)
				break
			}
			out[cw] = imageCopy{data, ".webp"}
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	for _, cw := range copyWidths(w) {
		var buf bytes.Buffer
		if mime == "image/png" {
			err = png.Encode(&buf, scaled(cw))
		} else {
			err = jpeg.Encode(&buf, scaled(cw), &jpeg.Options{Quality: conversionJPEGRate})
		}
		if err != nil {
			return nil, err
		}
		out[cw] = imageCopy{data: buf.Bytes()}
	}
	return out, nil
}

// splitCopies sorts an image's copies into WebP ones and ones in the original's format.
func splitCopies(conversions map[string]string) (webp, same map[string]string) {
	webp, same = map[string]string{}, map[string]string{}
	for w, p := range conversions {
		if strings.HasSuffix(strings.ToLower(p), ".webp") {
			webp[w] = p
		} else {
			same[w] = p
		}
	}
	return webp, same
}

// srcset is the srcset attribute value of an image's copies, smallest first.
func srcset(conversions map[string]string, url func(string) string) string {
	type entry struct {
		w int
		p string
	}
	var list []entry
	for k, p := range conversions {
		if w, err := strconv.Atoi(k); err == nil {
			list = append(list, entry{w, p})
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].w < list[j].w })
	parts := make([]string, 0, len(list))
	for _, e := range list {
		u := url(e.p)
		if u == "" {
			continue
		}
		parts = append(parts, u+" "+strconv.Itoa(e.w)+"w")
	}
	return strings.Join(parts, ", ")
}
