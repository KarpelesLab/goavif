package goavif

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
)

// TestRealisticAnimationRoundTrip encodes and decodes a 10-frame
// panning gradient animation (simulating a common "slow pan" use
// case) and validates that every frame is decoded with modest error.
// Exercises the full inter pipeline at scale.
func TestRealisticAnimationRoundTrip(t *testing.T) {
	const (
		dim     = 128
		nFrames = 10
	)
	// Build a "panning" sequence: a smooth color landscape that
	// shifts by 3 pixels per frame.
	frames := make([]image.Image, nFrames)
	delays := make([]time.Duration, nFrames)
	landscape := func(x, y int) color.RGBA {
		// Two-channel diagonal wave + color gradient.
		r := uint8((x + 2*y) & 0xFF)
		g := uint8((y * 3) & 0xFF)
		b := uint8((x + y) & 0xFF)
		return color.RGBA{r, g, b, 255}
	}
	for i := 0; i < nFrames; i++ {
		m := image.NewRGBA(image.Rect(0, 0, dim, dim))
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				m.SetRGBA(x, y, landscape(x+3*i, y))
			}
		}
		frames[i] = m
		delays[i] = 100 * time.Millisecond
	}

	var bufInter bytes.Buffer
	if err := EncodeAll(&bufInter, frames, delays, &Options{
		Quality:          75,
		InterEnabled:     true,
		KeyFrameInterval: 10,
	}); err != nil {
		t.Fatalf("EncodeAll: %v", err)
	}

	var bufIntra bytes.Buffer
	if err := EncodeAll(&bufIntra, frames, delays, &Options{Quality: 75}); err != nil {
		t.Fatalf("EncodeAll intra: %v", err)
	}

	t.Logf("10-frame pan sequence: intra=%d bytes, inter=%d bytes (%.1f%% of intra)",
		bufIntra.Len(), bufInter.Len(),
		100*float64(bufInter.Len())/float64(bufIntra.Len()))

	decoded, _, err := DecodeAll(&bufInter)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	if len(decoded) != nFrames {
		t.Fatalf("decoded %d frames, want %d", len(decoded), nFrames)
	}

	// Verify each decoded frame is close to its source on a coarse
	// MAD metric sampled at center row.
	for i, img := range decoded {
		b := img.Bounds()
		src := frames[i]
		row := b.Min.Y + b.Dy()/2
		sad := 0
		n := 0
		for x := 0; x < dim; x++ {
			sr, sg, sb, _ := src.At(x, row).RGBA()
			dr, dg, db, _ := img.At(x, row).RGBA()
			sad += absDiff(int(sr>>8), int(dr>>8))
			sad += absDiff(int(sg>>8), int(dg>>8))
			sad += absDiff(int(sb>>8), int(db>>8))
			n += 3
		}
		mad := float64(sad) / float64(n)
		t.Logf("frame %d: RGB MAD at mid-row = %.2f", i, mad)
		if mad > 40 {
			t.Errorf("frame %d MAD %.2f exceeds 40", i, mad)
		}
	}
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}
