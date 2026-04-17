package goavif

import (
	"bytes"
	"image"
	"strings"
	"testing"
	"time"
)

func TestDecodeAllRejectsNonAvifContainer(t *testing.T) {
	// Not a valid ISOBMFF file — ParseContainer returns an error.
	_, _, err := DecodeAll(strings.NewReader("this is not an isobmff file, but close enough to reach ParseContainer"))
	if err == nil {
		t.Fatal("expected error on garbage input")
	}
}

func TestDecodeAllEmptyInputReturnsError(t *testing.T) {
	_, _, err := DecodeAll(bytes.NewReader(nil))
	if err == nil {
		t.Fatal("expected error on empty input")
	}
}

// TestEncodeAllDecodeAllRoundTrip verifies an AVIS sequence encoded
// by EncodeAll round-trips through DecodeAll with the expected number
// of frames and durations.
func TestEncodeAllDecodeAllRoundTrip(t *testing.T) {
	const dim = 64
	nFrames := 3
	frames := make([]image.Image, nFrames)
	for i := 0; i < nFrames; i++ {
		img := image.NewRGBA(image.Rect(0, 0, dim, dim))
		// Each frame has a distinct solid color so we can tell them
		// apart after round-trip.
		shade := uint8(50 + i*70) // 50, 120, 190
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				idx := (y*dim + x) * 4
				img.Pix[idx+0] = shade
				img.Pix[idx+1] = shade
				img.Pix[idx+2] = shade
				img.Pix[idx+3] = 255
			}
		}
		frames[i] = img
	}
	delays := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
	}

	var buf bytes.Buffer
	if err := EncodeAll(&buf, frames, delays, &Options{Quality: 90}); err != nil {
		t.Fatalf("EncodeAll: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("EncodeAll produced empty output")
	}

	decoded, gotDelays, err := DecodeAll(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(decoded) != nFrames {
		t.Fatalf("frame count: got %d, want %d", len(decoded), nFrames)
	}
	if len(gotDelays) != nFrames {
		t.Fatalf("delays count: got %d, want %d", len(gotDelays), nFrames)
	}
	for i, d := range gotDelays {
		want := delays[i]
		diff := d - want
		if diff < -10*time.Millisecond || diff > 10*time.Millisecond {
			t.Errorf("frame %d delay = %v, want %v", i, d, want)
		}
	}
	// Sanity-check per-frame luma: frame 0 should be darker than
	// frame 2.
	getCenterY := func(img image.Image) int {
		b := img.Bounds()
		cx := b.Min.X + b.Dx()/2
		cy := b.Min.Y + b.Dy()/2
		y, _, _, _ := img.At(cx, cy).RGBA()
		return int(y >> 8)
	}
	y0 := getCenterY(decoded[0])
	y2 := getCenterY(decoded[2])
	t.Logf("center Y: frame0=%d frame2=%d", y0, y2)
	if y2 <= y0 {
		t.Fatalf("expected frame 2 brighter than frame 0, got %d vs %d", y2, y0)
	}
}
