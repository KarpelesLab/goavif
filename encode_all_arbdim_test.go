package goavif

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
)

// TestEncodeAllArbitraryDim verifies AVIS encoding with frame sizes
// not aligned to 64 (the SB grid). The encoder auto-pads to the next
// 64-multiple; the container's tkhd declares the original dims.
func TestEncodeAllArbitraryDim(t *testing.T) {
	const (
		w = 100
		h = 100
	)
	mk := func(shade uint8) image.Image {
		m := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				m.SetRGBA(x, y, color.RGBA{shade, shade, shade, 255})
			}
		}
		return m
	}
	frames := []image.Image{mk(60), mk(120), mk(180)}
	delays := []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond}

	var buf bytes.Buffer
	if err := EncodeAll(&buf, frames, delays, &Options{
		Quality:          90,
		InterEnabled:     true,
		KeyFrameInterval: 3,
	}); err != nil {
		t.Fatalf("EncodeAll %dx%d inter: %v", w, h, err)
	}
	t.Logf("encoded %dx%d AVIS (inter) size: %d bytes", w, h, buf.Len())

	decoded, _, err := DecodeAll(&buf)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("decoded %d frames, want 3", len(decoded))
	}
	for i, img := range decoded {
		bounds := img.Bounds()
		t.Logf("frame %d: decoded %dx%d (coded)", i, bounds.Dx(), bounds.Dy())
		// Coded (padded) dims should be the next multiple of 64.
		// Caller-visible dims live in tkhd but aren't yet cropped back
		// by DecodeAll (todo).
		if bounds.Dx() < w || bounds.Dy() < h {
			t.Fatalf("frame %d: decoded %dx%d smaller than input %dx%d",
				i, bounds.Dx(), bounds.Dy(), w, h)
		}
	}
}
