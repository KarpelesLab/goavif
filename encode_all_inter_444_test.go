package goavif

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
)

// TestEncodeAllInter444 verifies inter prediction works at 4:4:4
// chroma (no subsampling). Exercises the subsampling-aware inter
// chroma path.
func TestEncodeAllInter444(t *testing.T) {
	const dim = 64
	mk := func(shade uint8) image.Image {
		m := image.NewRGBA(image.Rect(0, 0, dim, dim))
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				m.SetRGBA(x, y, color.RGBA{shade, shade, shade, 255})
			}
		}
		return m
	}
	frames := []image.Image{mk(60), mk(120), mk(180)}
	delays := []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond}

	var buf bytes.Buffer
	if err := EncodeAll(&buf, frames, delays, &Options{
		Quality:           90,
		ChromaSubsampling: Chroma444,
		InterEnabled:      true,
		KeyFrameInterval:  3,
	}); err != nil {
		t.Fatalf("EncodeAll 4:4:4 inter: %v", err)
	}
	t.Logf("encoded 4:4:4 AVIS (inter) size: %d bytes", buf.Len())

	decoded, _, err := DecodeAll(&buf)
	if err != nil {
		t.Fatalf("DecodeAll 4:4:4 inter: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("decoded %d frames, want 3", len(decoded))
	}
	// Each frame should center-sample near its source shade.
	shades := []uint8{60, 120, 180}
	for i, img := range decoded {
		b := img.Bounds()
		cx := b.Min.X + b.Dx()/2
		cy := b.Min.Y + b.Dy()/2
		r, _, _, _ := img.At(cx, cy).RGBA()
		v := int(r >> 8)
		want := int(shades[i])
		diff := v - want
		if diff < 0 {
			diff = -diff
		}
		t.Logf("frame %d: decoded center = %d, source = %d (diff %d)", i, v, want, diff)
		if diff > 30 {
			t.Errorf("frame %d: decoded %d too far from source %d", i, v, want)
		}
	}
}
