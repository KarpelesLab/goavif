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
