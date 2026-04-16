package lr

import "testing"

func TestBoxMeanConstantInput(t *testing.T) {
	w, h := 8, 8
	src := make([]uint8, w*h)
	for i := range src {
		src[i] = 100
	}
	got := BoxMean(src, w, h, w, 2)
	for i, v := range got {
		if v != 100 {
			t.Errorf("BoxMean[%d]=%d, want 100", i, v)
		}
	}
}

func TestBoxVarConstantInput(t *testing.T) {
	w, h := 8, 8
	src := make([]uint8, w*h)
	for i := range src {
		src[i] = 73
	}
	got := BoxVar(src, w, h, w, 2)
	for i, v := range got {
		if v != 0 {
			t.Errorf("BoxVar[%d]=%d on flat input, want 0", i, v)
		}
	}
}

func TestBoxMeanSingleSpike(t *testing.T) {
	// A single hot pixel in a 5-radius window averages to 255/25 = 10.
	w, h := 10, 10
	src := make([]uint8, w*h)
	src[5*w+5] = 255
	got := BoxMean(src, w, h, w, 2)
	// The pixel at the center should see the 255 divided across 25 cells.
	if got[5*w+5] != 10 {
		t.Errorf("central mean = %d, want 10", got[5*w+5])
	}
}

func TestApplySGRPassthroughWhenRadiiZero(t *testing.T) {
	w, h := 8, 8
	src := make([]uint8, w*h)
	for i := range src {
		src[i] = uint8((i * 19) & 0xFF)
	}
	dst := make([]uint8, w*h)
	ApplySGR(dst, src, w, h, w, SGRParams{R0: 0, R1: 0})
	for i, v := range dst {
		if v != src[i] {
			t.Errorf("dst[%d]=%d, want %d (src)", i, v, src[i])
		}
	}
}

func TestSGRSubFilterFlatInput(t *testing.T) {
	w, h := 8, 8
	src := make([]uint8, w*h)
	for i := range src {
		src[i] = 77
	}
	dst := make([]uint8, w*h)
	SGRSubFilter(dst, src, w, h, w, 2, 12)
	for i, v := range dst {
		if v != 77 {
			t.Errorf("SGR on flat input: dst[%d]=%d, want 77", i, v)
		}
	}
}

func TestApplySGRSoftensNoise(t *testing.T) {
	// Checkerboard 40/160 → SGR should pull interior samples toward
	// the mean (100) with a visible softening effect.
	w, h := 8, 8
	src := make([]uint8, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if (r+c)%2 == 0 {
				src[r*w+c] = 40
			} else {
				src[r*w+c] = 160
			}
		}
	}
	dst := make([]uint8, w*h)
	ApplySGR(dst, src, w, h, w, SGRParams{R0: 1, R1: 0, Eps0: 40, Xq: [2]int{32, 0}})
	for r := 2; r < h-2; r++ {
		for c := 2; c < w-2; c++ {
			v := dst[r*w+c]
			if v < 30 || v > 170 {
				t.Errorf("dst[%d,%d]=%d out of expected softened band", r, c, v)
			}
		}
	}
}
