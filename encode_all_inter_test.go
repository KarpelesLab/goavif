package goavif

import (
	"bytes"
	"image"
	"testing"
	"time"
)

// TestEncodeAllInterRoundTrip verifies that an AVIS sequence encoded
// with InterEnabled=true decodes end-to-end through DecodeAll without
// errors, and that every decoded frame is close to the source frame.
// Uses solid-color frames so we can compare decoded luma directly.
func TestEncodeAllInterRoundTrip(t *testing.T) {
	const dim = 64
	makeFrame := func(shade uint8) image.Image {
		m := image.NewRGBA(image.Rect(0, 0, dim, dim))
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				idx := (y*dim + x) * 4
				m.Pix[idx+0] = shade
				m.Pix[idx+1] = shade
				m.Pix[idx+2] = shade
				m.Pix[idx+3] = 255
			}
		}
		return m
	}

	srcShades := []uint8{60, 120, 180}
	frames := []image.Image{
		makeFrame(srcShades[0]),
		makeFrame(srcShades[1]),
		makeFrame(srcShades[2]),
	}
	delays := []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond}

	var buf bytes.Buffer
	if err := EncodeAll(&buf, frames, delays, &Options{
		Quality:          90,
		InterEnabled:     true,
		KeyFrameInterval: 3, // frame 0 is key; 1 and 2 are inter.
	}); err != nil {
		t.Fatalf("EncodeAll: %v", err)
	}
	t.Logf("encoded AVIS (inter) size: %d bytes", buf.Len())

	decoded, durations, err := DecodeAll(&buf)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(decoded) != len(frames) {
		t.Fatalf("decoded %d frames, want %d", len(decoded), len(frames))
	}
	if len(durations) != len(frames) {
		t.Fatalf("got %d durations, want %d", len(durations), len(frames))
	}

	// Center-sample each frame and verify it's close to the source shade.
	for i, img := range decoded {
		b := img.Bounds()
		cx := b.Min.X + b.Dx()/2
		cy := b.Min.Y + b.Dy()/2
		r, _, _, _ := img.At(cx, cy).RGBA()
		v := int(r >> 8)
		want := int(srcShades[i])
		diff := v - want
		if diff < 0 {
			diff = -diff
		}
		t.Logf("frame %d: decoded center = %d, source = %d (diff %d)", i, v, want, diff)
		if diff > 25 {
			t.Errorf("frame %d: decoded %d too far from source %d", i, v, want)
		}
	}
}

// TestEncodeAllInterDisabled asserts the default (intra-only) path
// still works when InterEnabled is false.
func TestEncodeAllInterDisabled(t *testing.T) {
	const dim = 64
	frames := make([]image.Image, 2)
	for i := range frames {
		m := image.NewRGBA(image.Rect(0, 0, dim, dim))
		shade := uint8(80 + 40*i)
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				idx := (y*dim + x) * 4
				m.Pix[idx+0] = shade
				m.Pix[idx+1] = shade
				m.Pix[idx+2] = shade
				m.Pix[idx+3] = 255
			}
		}
		frames[i] = m
	}
	var buf bytes.Buffer
	// Same options as the inter test minus InterEnabled; this is the
	// backwards-compatible path.
	err := EncodeAll(&buf, frames, []time.Duration{100 * time.Millisecond, 100 * time.Millisecond}, &Options{Quality: 90})
	if err != nil {
		t.Fatalf("EncodeAll intra: %v", err)
	}
	decoded, _, err := DecodeAll(&buf)
	if err != nil {
		t.Fatalf("DecodeAll intra: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("intra frames = %d, want 2", len(decoded))
	}
}
