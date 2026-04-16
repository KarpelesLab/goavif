package decoder

import (
	"testing"
)

func TestPredictIntraDC(t *testing.T) {
	n := &Neighbors{
		Above:     []uint8{20, 20, 20, 20},
		Left:      []uint8{20, 20, 20, 20},
		HaveAbove: true,
		HaveLeft:  true,
		BitDepth:  8,
	}
	dst := make([]uint8, 16)
	if err := PredictIntra(dst, 4, 4, DCPred, n); err != nil {
		t.Fatalf("DC: %v", err)
	}
	for _, v := range dst {
		if v != 20 {
			t.Errorf("DC sample = %d, want 20", v)
		}
	}
}

func TestPredictIntraVFillsHalfWhenAboveMissing(t *testing.T) {
	// When above samples are not available (top-frame-edge block), the
	// spec fills a half-range default rather than erroring.
	n := &Neighbors{HaveAbove: false, BitDepth: 8}
	dst := make([]uint8, 16)
	if err := PredictIntra(dst, 4, 4, VPred, n); err != nil {
		t.Fatalf("V_PRED should succeed via half-range fallback: %v", err)
	}
	for i, v := range dst {
		if v != 128 {
			t.Errorf("dst[%d]=%d want 128", i, v)
		}
	}
}

func TestPredictIntraAllDirectionalModesRun(t *testing.T) {
	// Enough extended samples to keep the directional predictors in
	// bounds for a 4×4 block.
	ext := make([]uint8, 32)
	for i := range ext {
		ext[i] = 100
	}
	n := &Neighbors{
		Above:         ext,
		Left:          ext,
		AboveExtended: ext,
		LeftExtended:  ext,
		HaveAbove:     true,
		HaveLeft:      true,
		BitDepth:      8,
	}
	for _, m := range []IntraMode{D45Pred, D67Pred, D113Pred, D135Pred, D157Pred, D203Pred} {
		dst := make([]uint8, 16)
		if err := PredictIntra(dst, 4, 4, m, n); err != nil {
			t.Errorf("%s: %v", m, err)
		}
		// With constant inputs every output pixel should also be 100.
		for i, v := range dst {
			if v != 100 {
				t.Errorf("%s dst[%d]=%d want 100", m, i, v)
			}
		}
	}
}
