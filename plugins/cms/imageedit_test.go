package cms

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

// A 3x2 image with a red pixel at its top-left corner.
func marked() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	return img
}

func redAt(img image.Image) (int, int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, _, _, _ := img.At(x, y).RGBA(); r > 0 {
				return x - b.Min.X, y - b.Min.Y
			}
		}
	}
	return -1, -1
}

func TestImageTransforms(t *testing.T) {
	r := rotate90(marked())
	if r.Bounds().Dx() != 2 || r.Bounds().Dy() != 3 {
		t.Fatalf("rotated size = %v", r.Bounds())
	}
	if x, y := redAt(r); x != 1 || y != 0 {
		t.Fatalf("a quarter turn clockwise moves the top-left corner to the top-right: %d,%d", x, y)
	}
	if x, y := redAt(flipH(marked())); x != 2 || y != 0 {
		t.Fatalf("mirror: %d,%d", x, y)
	}
	if x, y := redAt(flipV(marked())); x != 0 || y != 1 {
		t.Fatalf("vertical mirror: %d,%d", x, y)
	}
	// Orientation 6 (the camera turned right) shows the image a quarter turn clockwise.
	if x, y := redAt(upright(marked(), 6)); x != 1 || y != 0 {
		t.Fatalf("orientation 6: %d,%d", x, y)
	}
	// Orientation 5 is a transpose: the corner stays put, width and height swap.
	if u := upright(marked(), 5); u.Bounds().Dx() != 2 {
		t.Fatal("orientation 5 swaps the sides")
	} else if x, y := redAt(u); x != 0 || y != 0 {
		t.Fatalf("orientation 5: %d,%d", x, y)
	}
}

func TestApplyImageEdit(t *testing.T) {
	out, err := applyImageEdit(marked(), imageEdit{X: 0, Y: 0, Width: 2, Height: 1})
	if err != nil || out.Bounds().Dx() != 2 || out.Bounds().Dy() != 1 {
		t.Fatalf("crop = %v, %v", out.Bounds(), err)
	}
	if x, y := redAt(out); x != 0 || y != 0 {
		t.Fatal("the crop keeps the corner")
	}
	// Rotate, then crop in the rotated image's pixels (2 wide, 3 high).
	out, err = applyImageEdit(marked(), imageEdit{Rotate: 90, X: 1, Y: 0, Width: 1, Height: 1})
	if err != nil {
		t.Fatal(err)
	}
	if x, _ := redAt(out); x != 0 {
		t.Fatal("rotate then crop picks the turned corner")
	}
	// A crop past the edge is clipped; one fully outside is refused.
	if out, err := applyImageEdit(marked(), imageEdit{X: 2, Y: 1, Width: 50, Height: 50}); err != nil || out.Bounds().Dx() != 1 {
		t.Fatalf("clipped crop = %v, %v", out, err)
	}
	if _, err := applyImageEdit(marked(), imageEdit{X: 10, Y: 10, Width: 5, Height: 5}); err == nil {
		t.Fatal("a crop outside the image is refused")
	}
	if out, _ := applyImageEdit(marked(), imageEdit{}); out.Bounds().Dx() != 3 {
		t.Fatal("no crop keeps the whole image")
	}
}

func TestParseImageEdit(t *testing.T) {
	e, err := parseImageEdit(map[string]any{"rotate": float64(270), "flip": true, "crop": map[string]any{"x": 1.4, "y": float64(2), "width": float64(30), "height": float64(40)}})
	if err != nil || e != (imageEdit{Rotate: 270, Flip: true, X: 1, Y: 2, Width: 30, Height: 40}) {
		t.Fatalf("edit = %+v, %v", e, err)
	}
	for _, bad := range []map[string]any{
		{"rotate": float64(45)},
		{"rotate": float64(360)},
		{"crop": map[string]any{"x": float64(-1), "y": float64(0), "width": float64(1), "height": float64(1)}},
		{"crop": map[string]any{"x": "1", "y": float64(0), "width": float64(1), "height": float64(1)}},
	} {
		if _, err := parseImageEdit(bad); err == nil {
			t.Errorf("%v should be refused", bad)
		}
	}
}

// jpegWithOrientation is a JPEG start with an EXIF orientation tag (enough for the parser).
func jpegWithOrientation(o uint16, order binary.ByteOrder, mark string) []byte {
	tiff := make([]byte, 8+2+12+4)
	copy(tiff, mark)
	order.PutUint16(tiff[2:], 42)
	order.PutUint32(tiff[4:], 8)
	order.PutUint16(tiff[8:], 1)
	order.PutUint16(tiff[10:], 0x0112)
	order.PutUint16(tiff[12:], 3)
	order.PutUint32(tiff[14:], 1)
	order.PutUint16(tiff[18:], o)
	seg := append([]byte("Exif\x00\x00"), tiff...)
	out := []byte{0xFF, 0xD8, 0xFF, 0xE1, 0, 0}
	binary.BigEndian.PutUint16(out[4:], uint16(len(seg)+2))
	return append(append(out, seg...), 0xFF, 0xDA, 0, 2)
}

func TestExifOrientation(t *testing.T) {
	if o := exifOrientation(jpegWithOrientation(6, binary.LittleEndian, "II")); o != 6 {
		t.Fatalf("little endian = %d", o)
	}
	if o := exifOrientation(jpegWithOrientation(8, binary.BigEndian, "MM")); o != 8 {
		t.Fatalf("big endian = %d", o)
	}
	for _, raw := range [][]byte{nil, []byte("not a jpeg"), {0xFF, 0xD8, 0xFF, 0xDA, 0, 2}, jpegWithOrientation(99, binary.BigEndian, "MM")} {
		if o := exifOrientation(raw); o != 1 {
			t.Errorf("%x: orientation %d, want 1", raw, o)
		}
	}
}
