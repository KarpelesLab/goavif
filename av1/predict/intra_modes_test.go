package predict

import "testing"

func TestVPred(t *testing.T) {
	dst := make([]uint8, 4*4)
	above := []uint8{10, 20, 30, 40}
	VPred(dst, 4, 4, above)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != above[c] {
				t.Errorf("V_PRED[%d,%d]=%d want %d", r, c, dst[r*4+c], above[c])
			}
		}
	}
}

func TestHPred(t *testing.T) {
	dst := make([]uint8, 4*4)
	left := []uint8{10, 20, 30, 40}
	HPred(dst, 4, 4, left)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != left[r] {
				t.Errorf("H_PRED[%d,%d]=%d want %d", r, c, dst[r*4+c], left[r])
			}
		}
	}
}

func TestPaethDegenerateConstant(t *testing.T) {
	// With above[] == left[] == aboveLeft == 50 the Paeth predictor must
	// return 50 for every sample (all three candidates equal).
	dst := make([]uint8, 4*4)
	row := []uint8{50, 50, 50, 50}
	PaethPred(dst, 4, 4, row, row, 50)
	for i, v := range dst {
		if v != 50 {
			t.Errorf("Paeth[%d]=%d want 50", i, v)
		}
	}
}

func TestPaethFollowsLeftWhenClose(t *testing.T) {
	// With left[r] exactly matching the predicted base, Paeth should pick
	// the left sample. Construct above=left=aboveLeft+0 trivially to
	// exercise the code path.
	above := []uint8{100, 100, 100, 100}
	left := []uint8{50, 50, 50, 50}
	dst := make([]uint8, 4*4)
	PaethPred(dst, 4, 4, above, left, 100)
	// base = above[c] + left[r] - aboveLeft = 100 + 50 - 100 = 50
	// p_a = |50-100| = 50, p_l = |50-50| = 0, p_al = |50-100| = 50
	// p_l is smallest → pred = left[r] = 50
	for i, v := range dst {
		if v != 50 {
			t.Errorf("Paeth[%d]=%d want 50", i, v)
		}
	}
}

func TestSmoothPredConstantInputs(t *testing.T) {
	// With all border samples equal to 100, smooth prediction must be 100.
	dst := make([]uint8, 4*4)
	border := []uint8{100, 100, 100, 100}
	SmoothPred(dst, 4, 4, border, border)
	for i, v := range dst {
		if v != 100 {
			t.Errorf("Smooth[%d]=%d want 100", i, v)
		}
	}
}

func TestSmoothVPredBoundsOfAbove(t *testing.T) {
	// With left[bh-1] == above[c] smooth-V must return above[c] exactly.
	dst := make([]uint8, 4*4)
	above := []uint8{30, 30, 30, 30}
	left := []uint8{0, 0, 0, 30}
	SmoothVPred(dst, 4, 4, above, left)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != 30 {
				t.Errorf("SmoothV[%d,%d]=%d want 30", r, c, dst[r*4+c])
			}
		}
	}
}
