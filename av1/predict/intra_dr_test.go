package predict

import "testing"

func TestModeToAngleBaseValues(t *testing.T) {
	cases := map[int]int{
		3: 45, 4: 135, 5: 113, 6: 157, 7: 203, 8: 67,
	}
	for mode, want := range cases {
		if got := ModeToAngleMap[mode]; got != want {
			t.Errorf("mode %d: got %d, want %d", mode, got, want)
		}
	}
}

func TestDrDerivativeAt45(t *testing.T) {
	// tan(45°) = 1, so dx = 256/1 = 256 → Q6 value = 64.
	if drIntraDerivative[45] != 64 {
		t.Errorf("dr_intra_derivative[45] = %d, want 64", drIntraDerivative[45])
	}
}

func TestDirectionalPredConstantInputs(t *testing.T) {
	// Constant above and left at 128 should yield a constant 128 block
	// for all 3 zones of directional prediction.
	above := make([]uint8, 16)
	left := make([]uint8, 16)
	for i := range above {
		above[i] = 128
	}
	for i := range left {
		left[i] = 128
	}
	dst := make([]uint8, 16)
	for _, angle := range []int{45, 67, 113, 135, 157, 203} {
		for i := range dst {
			dst[i] = 0
		}
		DirectionalPred(dst, 4, 4, above, left, angle)
		for i, v := range dst {
			if v != 128 {
				t.Errorf("angle=%d dst[%d]=%d want 128", angle, i, v)
			}
		}
	}
}

func TestDirectionalPred45MatchesDiagonal(t *testing.T) {
	// At 45° with above = [10, 20, 30, 40, 50, 60, 70, 80] the first
	// sample at (0,0) should interpolate near above[0..1], at (0,1) near
	// above[1..2], etc. — generally monotonically increasing across a
	// row.
	above := []uint8{10, 20, 30, 40, 50, 60, 70, 80}
	left := []uint8{10, 20, 30, 40}
	dst := make([]uint8, 16)
	DirectionalPred(dst, 4, 4, above, left, 45)
	for r := 0; r < 4; r++ {
		prev := uint8(0)
		for c := 0; c < 4; c++ {
			v := dst[r*4+c]
			if c > 0 && v < prev {
				t.Errorf("row %d: dst[%d]=%d not monotonic after %d", r, c, v, prev)
			}
			prev = v
		}
	}
}
