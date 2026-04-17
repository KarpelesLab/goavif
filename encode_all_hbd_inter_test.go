package goavif

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
)

// TestEncodeAllHBDInter verifies that a 10-bit AVIS sequence with
// InterEnabled=true round-trips through Encode+DecodeAll.
func TestEncodeAllHBDInter(t *testing.T) {
	const dim = 64
	mk := func(shade uint16) image.Image {
		m := image.NewRGBA64(image.Rect(0, 0, dim, dim))
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				// shade scaled to 10-bit range, then up-shifted to 16-bit.
				v := uint16(shade << 6)
				m.SetRGBA64(x, y, color.RGBA64{v, v, v, 0xFFFF})
			}
		}
		return m
	}
	shades := []uint16{100, 400, 700}
	frames := []image.Image{mk(shades[0]), mk(shades[1]), mk(shades[2])}
	delays := []time.Duration{100 * time.Millisecond, 100 * time.Millisecond, 100 * time.Millisecond}

	var buf bytes.Buffer
	if err := EncodeAll(&buf, frames, delays, &Options{
		Quality:          90,
		BitDepth:         10,
		InterEnabled:     true,
		KeyFrameInterval: 3,
	}); err != nil {
		t.Fatalf("EncodeAll HBD inter: %v", err)
	}
	t.Logf("encoded 10-bit AVIS (inter) size: %d bytes", buf.Len())

	decoded, _, err := DecodeAll(&buf)
	if err != nil {
		t.Fatalf("DecodeAll HBD inter: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("decoded %d frames, want 3", len(decoded))
	}
	// Sanity: each decoded frame should center-sample close to the
	// source shade (converted to 16-bit RGB via color.RGBA64).
	for i, img := range decoded {
		b := img.Bounds()
		r, _, _, _ := img.At(b.Min.X+b.Dx()/2, b.Min.Y+b.Dy()/2).RGBA()
		// Decoded value at 10-bit, promoted to 16-bit by decoder:
		// compare to (shade << 6). Allow a wide tolerance.
		want := int(shades[i]) << 6
		got := int(r)
		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		t.Logf("frame %d: decoded %d, source %d (diff %d)", i, got, want, diff)
		// Tolerance ~2500 in 16-bit (= ~40 in 10-bit) accounts for
		// BT.601 quantization + studio-range color conversion drift.
		if diff > 2500 {
			t.Errorf("frame %d HBD center too far from source", i)
		}
	}
}
