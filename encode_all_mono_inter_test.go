package goavif

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
)

// TestEncodeAllMonoInter verifies monochrome inter-frame encoding
// round-trips through DecodeAll.
func TestEncodeAllMonoInter(t *testing.T) {
	const dim = 64
	mk := func(shade uint8) image.Image {
		m := image.NewGray(image.Rect(0, 0, dim, dim))
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				m.SetGray(x, y, color.Gray{Y: shade})
			}
		}
		return m
	}
	shades := []uint8{60, 120, 180}
	frames := []image.Image{mk(shades[0]), mk(shades[1]), mk(shades[2])}
	delays := []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond}

	var buf bytes.Buffer
	if err := EncodeAll(&buf, frames, delays, &Options{
		Quality:          90,
		InterEnabled:     true,
		KeyFrameInterval: 3,
	}); err != nil {
		t.Fatalf("EncodeAll mono inter: %v", err)
	}
	t.Logf("encoded mono AVIS (inter) size: %d bytes", buf.Len())

	decoded, _, err := DecodeAll(&buf)
	if err != nil {
		t.Fatalf("DecodeAll mono inter: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("decoded %d frames, want 3", len(decoded))
	}
	for i, img := range decoded {
		b := img.Bounds()
		cx := b.Min.X + b.Dx()/2
		cy := b.Min.Y + b.Dy()/2
		g, _, _, _ := img.At(cx, cy).RGBA()
		v := int(g >> 8)
		want := int(shades[i])
		diff := v - want
		if diff < 0 {
			diff = -diff
		}
		t.Logf("frame %d: decoded luma=%d, source=%d (diff %d)", i, v, want, diff)
		if diff > 25 {
			t.Errorf("frame %d: decoded %d too far from source %d", i, v, want)
		}
	}
}
