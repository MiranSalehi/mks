package cms

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"math"
	nethttp "net/http"
	"path"
	"strings"

	"golang.org/x/image/draw"

	mks "github.com/MiranSalehi/mks"
)

// The image editor (MKSine's media image editor: crop with free, 16:9, 4:3 and 1:1
// ratios, plus rotating and flipping). It works on JPEG and PNG files of the library,
// after their EXIF orientation (as browsers show them). The result replaces the file, or
// becomes a new library file with "as_copy". The copies (srcset) are made again.

// imageEdit is what the editor sends: a rotation (clockwise, in steps of 90), a mirror,
// then a crop rectangle in pixels of the rotated image (zero width: the whole image).
type imageEdit struct {
	Rotate int
	Flip   bool
	X, Y   int
	Width  int
	Height int
}

// parseImageEdit reads the action's data: the editor field "edit" ({rotate, flip, crop}),
// or the same keys at the top.
func parseImageEdit(data map[string]any) (imageEdit, error) {
	if nested, ok := data["edit"].(map[string]any); ok {
		data = nested
	}
	num := func(v any) (int, bool) {
		f, ok := v.(float64)
		if !ok || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > 1e6 {
			return 0, false
		}
		return int(math.Round(f)), true
	}
	var e imageEdit
	if v, sent := data["rotate"]; sent && v != nil {
		r, ok := num(v)
		if !ok || r%90 != 0 || r >= 360 {
			return e, mks.ValidationErrors{"edit": "Invalid rotation."}
		}
		e.Rotate = r
	}
	e.Flip, _ = data["flip"].(bool)
	if crop, ok := data["crop"].(map[string]any); ok {
		var good [4]bool
		e.X, good[0] = num(crop["x"])
		e.Y, good[1] = num(crop["y"])
		e.Width, good[2] = num(crop["width"])
		e.Height, good[3] = num(crop["height"])
		if !good[0] || !good[1] || !good[2] || !good[3] {
			return e, mks.ValidationErrors{"edit": "Invalid crop area."}
		}
	}
	return e, nil
}

// applyImageEdit rotates, mirrors and crops src (already upright).
func applyImageEdit(src image.Image, e imageEdit) (image.Image, error) {
	img := src
	for range (e.Rotate / 90) % 4 {
		img = rotate90(img)
	}
	if e.Flip {
		img = flipH(img)
	}
	b := img.Bounds()
	if e.Width == 0 || e.Height == 0 {
		return img, nil
	}
	r := image.Rect(e.X, e.Y, e.X+e.Width, e.Y+e.Height).Add(b.Min).Intersect(b)
	if r.Dx() < 1 || r.Dy() < 1 {
		return nil, mks.ValidationErrors{"edit": "The crop area is outside the image."}
	}
	out := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	draw.Draw(out, out.Bounds(), img, r.Min, draw.Src)
	return out, nil
}

// rotate90 turns an image a quarter clockwise.
func rotate90(src image.Image) image.Image {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dy(), b.Dx()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(b.Max.Y-1-y, x-b.Min.X, src.At(x, y))
		}
	}
	return out
}

// flipH mirrors an image left to right.
func flipH(src image.Image) image.Image {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(b.Max.X-1-x, y-b.Min.Y, src.At(x, y))
		}
	}
	return out
}

// flipV mirrors an image top to bottom.
func flipV(src image.Image) image.Image {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.Set(x-b.Min.X, b.Max.Y-1-y, src.At(x, y))
		}
	}
	return out
}

// upright applies a JPEG's EXIF orientation (1–8), as browsers do when showing it.
func upright(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return flipH(img)
	case 3:
		return rotate90(rotate90(img))
	case 4:
		return flipV(img)
	case 5:
		return flipH(rotate90(img))
	case 6:
		return rotate90(img)
	case 7:
		return flipH(rotate90(rotate90(rotate90(img))))
	case 8:
		return rotate90(rotate90(rotate90(img)))
	}
	return img
}

// exifOrientation reads the orientation tag of a JPEG (1 when there is none).
func exifOrientation(raw []byte) int {
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(raw) {
		if raw[i] != 0xFF {
			return 1
		}
		marker := raw[i+1]
		size := int(binary.BigEndian.Uint16(raw[i+2:]))
		if marker == 0xDA || size < 2 || i+2+size > len(raw) {
			return 1 // image data starts: no EXIF before it
		}
		seg := raw[i+4 : i+2+size]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return tiffOrientation(seg[6:])
		}
		i += 2 + size
	}
	return 1
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var order binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return 1
	}
	ifd := int(order.Uint32(t[4:]))
	if ifd+2 > len(t) {
		return 1
	}
	n := int(order.Uint16(t[ifd:]))
	for k := range n {
		e := ifd + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if order.Uint16(t[e:]) == 0x0112 {
			if v := int(order.Uint16(t[e+8:])); v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// editImage is the "edit_image" action of the media library.
func (pl *Plugin) editImage(ctx *mks.ActionContext[Media]) (*mks.ActionResult, error) {
	if len(ctx.Records) != 1 {
		return nil, mks.Reject("Select an image.")
	}
	m := ctx.Records[0]
	if !pl.canEdit(m.MimeType) {
		return nil, mks.Reject("Only JPEG, PNG and WebP images can be edited.")
	}
	edit, err := parseImageEdit(ctx.Data)
	if err != nil {
		return nil, err
	}
	d := pl.driver()
	if d == nil {
		return nil, mks.Reject("File storage is not configured.")
	}
	content, err := d.Get(m.Path)
	if err != nil {
		return nil, fmt.Errorf("read media: %w", err)
	}
	raw := []byte(content)
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width*cfg.Height > maxConvertPixels {
		return nil, mks.Reject("This image cannot be read, or it is too large.")
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, mks.Reject("This image cannot be read.")
	}
	if m.MimeType == "image/jpeg" {
		src = upright(src, exifOrientation(raw))
	}
	out, err := applyImageEdit(src, edit)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	switch m.MimeType {
	case "image/webp":
		var data []byte
		if data, err = pl.WebP(out, 90); err == nil {
			buf.Write(data)
		}
	case "image/png":
		err = png.Encode(&buf, out)
	default:
		err = jpeg.Encode(&buf, out, &jpeg.Options{Quality: 90})
	}
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(nethttp.DetectContentType(buf.Bytes()), m.MimeType) {
		return nil, fmt.Errorf("edited image has the wrong type")
	}

	asCopy, _ := ctx.Data["as_copy"].(bool)
	if asCopy {
		b := make([]byte, 20)
		_, _ = rand.Read(b)
		target := path.Join(path.Dir(m.Path), hex.EncodeToString(b)+path.Ext(m.Path))
		if err := d.Put(target, buf.String()); err != nil {
			return nil, err
		}
		info := pl.analyzeMedia(target)
		name := strings.TrimSuffix(m.FileName, path.Ext(m.FileName)) + "-edited" + path.Ext(m.FileName)
		uploader := fmt.Sprint(ctx.UserID())
		copyRow := Media{SiteID: m.SiteID, Path: target, FileName: trimTo(name, 255), MimeType: info.mime, Size: info.size,
			Width: info.width, Height: info.height, Alt: m.Alt, Title: m.Title, Caption: m.Caption, UploadedBy: &uploader,
			Conversions: mks.NewJSON(info.conversions)}
		if err := ctx.Query.Create(&copyRow); err != nil {
			pl.deleteFiles(info.conversions)
			pl.deleteFiles(map[string]string{"": target})
			return nil, err
		}
		res := mks.Success("The edited image was saved as a new file in the library.")
		res.Invalidate = []string{"cms_media"}
		return res, nil
	}

	if err := d.Put(m.Path, buf.String()); err != nil {
		return nil, err
	}
	old := m.Conversions.V
	info := pl.analyzeMedia(m.Path)
	stale := map[string]string{}
	for w, p := range old {
		if info.conversions[w] != p {
			stale[w] = p
		}
	}
	pl.deleteFiles(stale)
	if _, err := ctx.Query.Model(&Media{}).Where("id = ?", m.ID).Update(map[string]any{
		"size": info.size, "width": info.width, "height": info.height, "conversions": mks.NewJSON(info.conversions),
	}); err != nil {
		return nil, err
	}
	res := mks.Success("Image updated.")
	res.Invalidate = []string{"cms_media"}
	return res, nil
}
