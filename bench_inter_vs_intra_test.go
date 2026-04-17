package goavif

import (
	"bytes"
	"image"
	"image/color"
	"testing"
	"time"
)

// TestInterBeatsIntraOnStaticSequence verifies that an AVIS encoded
// with inter prediction is smaller than the same sequence encoded
// intra-only when frames are similar (the classic video case).
func TestInterBeatsIntraOnStaticSequence(t *testing.T) {
	const dim = 128
	makeFrame := func(shift int) image.Image {
		m := image.NewRGBA(image.Rect(0, 0, dim, dim))
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				// Two solid regions: top half bright, bottom half dark.
				// Shift the boundary by `shift` pixels per frame.
				boundary := dim/2 + shift
				var v uint8 = 50
				if y < boundary {
					v = 200
				}
				m.SetRGBA(x, y, color.RGBA{v, v, v, 255})
			}
		}
		return m
	}
	const nFrames = 5
	frames := make([]image.Image, nFrames)
	delays := make([]time.Duration, nFrames)
	for i := 0; i < nFrames; i++ {
		frames[i] = makeFrame(i * 2) // shift 2 pixels per frame
		delays[i] = 100 * time.Millisecond
	}

	var bufIntra bytes.Buffer
	if err := EncodeAll(&bufIntra, frames, delays, &Options{Quality: 80}); err != nil {
		t.Fatalf("EncodeAll intra: %v", err)
	}

	var bufInter bytes.Buffer
	if err := EncodeAll(&bufInter, frames, delays, &Options{
		Quality:          80,
		InterEnabled:     true,
		KeyFrameInterval: 10,
	}); err != nil {
		t.Fatalf("EncodeAll inter: %v", err)
	}

	t.Logf("intra-only AVIS: %d bytes, inter AVIS: %d bytes (%.1f%% of intra)",
		bufIntra.Len(), bufInter.Len(),
		100*float64(bufInter.Len())/float64(bufIntra.Len()))

	if bufInter.Len() >= bufIntra.Len() {
		t.Errorf("inter path did not beat intra: %d bytes vs %d bytes",
			bufInter.Len(), bufIntra.Len())
	}

	// Sanity: both must round-trip without error.
	if _, _, err := DecodeAll(bytes.NewReader(bufIntra.Bytes())); err != nil {
		t.Errorf("DecodeAll intra: %v", err)
	}
	if _, _, err := DecodeAll(bytes.NewReader(bufInter.Bytes())); err != nil {
		t.Errorf("DecodeAll inter: %v", err)
	}
}
