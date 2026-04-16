package predict

import "testing"

func TestDCPred16UsesHalfRangeWhenBothEdgesAbsent(t *testing.T) {
	dst := make([]uint16, 8*8)
	DCPred16(dst, 8, 8, nil, nil, false, false, 10)
	want := uint16(1 << 9)
	for i, v := range dst {
		if v != want {
			t.Fatalf("no-edge DC[%d] = %d, want %d", i, v, want)
		}
	}
}

func TestDCPred16AveragesBothEdges(t *testing.T) {
	above := []uint16{100, 200, 300, 400}
	left := []uint16{500, 600, 700, 800}
	dst := make([]uint16, 4*4)
	DCPred16(dst, 4, 4, above, left, true, true, 12)
	// Sum = 100+200+300+400+500+600+700+800 = 3600; 3600/8 = 450.
	if dst[0] != 450 {
		t.Fatalf("DC avg got %d want 450", dst[0])
	}
}

func TestVPred16CopiesAboveEveryRow(t *testing.T) {
	above := []uint16{1000, 2000, 3000, 4000}
	dst := make([]uint16, 4*4)
	VPred16(dst, 4, 4, above)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != above[c] {
				t.Fatalf("V[%d,%d]=%d want %d", r, c, dst[r*4+c], above[c])
			}
		}
	}
}

func TestHPred16CopiesLeftEveryColumn(t *testing.T) {
	left := []uint16{100, 200, 300, 400}
	dst := make([]uint16, 4*4)
	HPred16(dst, 4, 4, left)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != left[r] {
				t.Fatalf("H[%d,%d]=%d want %d", r, c, dst[r*4+c], left[r])
			}
		}
	}
}

func TestPaethPred16SamePredictionAsSamplesWhenConstant(t *testing.T) {
	above := []uint16{500, 500, 500, 500}
	left := []uint16{500, 500, 500, 500}
	dst := make([]uint16, 4*4)
	PaethPred16(dst, 4, 4, above, left, 500)
	for i, v := range dst {
		if v != 500 {
			t.Fatalf("constant paeth[%d]=%d", i, v)
		}
	}
}

func TestSmoothPred16InterpolatesConstantsToConstant(t *testing.T) {
	// If all four corner references are equal the output must be that
	// constant regardless of weights (assuming (256-wr)+(wr) = 256 and
	// same for wc).
	above := []uint16{400, 400, 400, 400}
	left := []uint16{400, 400, 400, 400}
	dst := make([]uint16, 4*4)
	SmoothPred16(dst, 4, 4, above, left)
	for i, v := range dst {
		if v != 400 {
			t.Fatalf("constant smooth[%d]=%d", i, v)
		}
	}
}

func TestSmoothVPred16AtTopEdgeMatchesAbove(t *testing.T) {
	above := []uint16{1000, 2000, 3000, 4000}
	left := []uint16{500, 600, 700, 800}
	dst := make([]uint16, 4*4)
	SmoothVPred16(dst, 4, 4, above, left)
	// At row 0 the weight wr=smWeights4[0]=255, so output ≈ above[c].
	for c := 0; c < 4; c++ {
		delta := int(dst[c]) - int(above[c])
		if delta < 0 {
			delta = -delta
		}
		if delta > 15 { // rounding + weight-1 leakage from belowPred
			t.Fatalf("row0 Smooth-V col=%d got %d above=%d", c, dst[c], above[c])
		}
	}
}
