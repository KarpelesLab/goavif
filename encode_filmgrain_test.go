package goavif

import (
	"bytes"
	"image"
	"image/color"
	"testing"
)

// TestEncodeFilmGrainAppliesNoise verifies that Encode with a
// non-zero FilmGrainStrength produces a bitstream whose decoded
// output differs from the grain-free version — evidence that the
// decoder applied the grain the encoder emitted.
func TestEncodeFilmGrainAppliesNoise(t *testing.T) {
	const dim = 128
	// Flat input: any difference in the decoded output between the
	// grain and no-grain versions is caused by grain synthesis.
	m := image.NewRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			m.SetRGBA(x, y, color.RGBA{128, 128, 128, 255})
		}
	}

	var bufPlain, bufGrain bytes.Buffer
	if err := Encode(&bufPlain, m, &Options{Quality: 95}); err != nil {
		t.Fatalf("Encode plain: %v", err)
	}
	if err := Encode(&bufGrain, m, &Options{
		Quality:           95,
		FilmGrainStrength: 48,
	}); err != nil {
		t.Fatalf("Encode grain: %v", err)
	}

	imgPlain, err := Decode(bytes.NewReader(bufPlain.Bytes()))
	if err != nil {
		t.Fatalf("Decode plain: %v", err)
	}
	imgGrain, err := Decode(bytes.NewReader(bufGrain.Bytes()))
	if err != nil {
		t.Fatalf("Decode grain: %v", err)
	}

	// Compare samples — the grain version should differ.
	diffs := 0
	for y := 0; y < dim; y += 4 {
		for x := 0; x < dim; x += 4 {
			rP, _, _, _ := imgPlain.At(x, y).RGBA()
			rG, _, _, _ := imgGrain.At(x, y).RGBA()
			if rP != rG {
				diffs++
			}
		}
	}
	total := (dim / 4) * (dim / 4)
	t.Logf("grain vs plain: %d/%d samples differ", diffs, total)
	if diffs < total/10 {
		t.Errorf("too few samples changed with grain enabled: %d/%d (expected >= %d)",
			diffs, total, total/10)
	}
}
