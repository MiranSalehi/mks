package cms

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
)

// WebPEncoder writes an image as WebP at quality 1–100. Go has no WebP encoder of its own,
// so the plugin takes one: CWebP (the cwebp tool of libwebp) or any library.
type WebPEncoder func(img image.Image, quality int) ([]byte, error)

// CWebP is a WebPEncoder that runs libwebp's cwebp tool (without a shell, with a time
// limit). binary is its path; empty looks for "cwebp" on PATH. Nil when it is not found, so
// `WebP: cms.CWebP("")` turns WebP copies on only where the tool is installed.
func CWebP(binary string) WebPEncoder {
	if binary == "" {
		binary = "cwebp"
	}
	bin, err := exec.LookPath(binary)
	if err != nil {
		return nil
	}
	return func(img image.Image, quality int) ([]byte, error) {
		dir, err := os.MkdirTemp("", "cms-webp-")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(dir)
		in, out := filepath.Join(dir, "in.png"), filepath.Join(dir, "out.webp")
		var buf bytes.Buffer
		if err := (&png.Encoder{CompressionLevel: png.BestSpeed}).Encode(&buf, img); err != nil {
			return nil, err
		}
		if err := os.WriteFile(in, buf.Bytes(), 0o600); err != nil {
			return nil, err
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, bin, "-quiet", "-metadata", "none", "-q", strconv.Itoa(quality), in, "-o", out)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("cwebp: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
		}
		return os.ReadFile(out)
	}
}

func (pl *Plugin) webpQuality() int {
	if pl.WebPQuality <= 0 {
		return 80
	}
	return min(100, max(30, pl.WebPQuality))
}
