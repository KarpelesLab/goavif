package decoder

import "testing"

func TestPredictIntra16DCUsesHalfRangeOnIsolatedBlock(t *testing.T) {
	dst := make([]uint16, 4*4)
	n := &Neighbors16{BitDepth: 10}
	if err := PredictIntra16(dst, 4, 4, DCPred, n); err != nil {
		t.Fatalf("DC: %v", err)
	}
	want := uint16(1 << 9)
	for _, v := range dst {
		if v != want {
			t.Fatalf("DC fallback got %d want %d", v, want)
		}
	}
}

func TestPredictIntra16VMatchesAbove(t *testing.T) {
	above := []uint16{100, 200, 300, 400}
	n := &Neighbors16{
		Above:     above,
		HaveAbove: true,
		BitDepth:  10,
	}
	dst := make([]uint16, 4*4)
	if err := PredictIntra16(dst, 4, 4, VPred, n); err != nil {
		t.Fatalf("V: %v", err)
	}
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != above[c] {
				t.Fatalf("V (%d,%d) got %d want %d", r, c, dst[r*4+c], above[c])
			}
		}
	}
}

func TestPredictIntra16FallsBackToHalfRangeOnMissingEdge(t *testing.T) {
	n := &Neighbors16{HaveLeft: false, BitDepth: 12}
	dst := make([]uint16, 4*4)
	if err := PredictIntra16(dst, 4, 4, HPred, n); err != nil {
		t.Fatalf("H: %v", err)
	}
	want := uint16(1 << 11)
	for _, v := range dst {
		if v != want {
			t.Fatalf("H edge fallback got %d want %d", v, want)
		}
	}
}

func TestPredictIntra16SmoothConstantInput(t *testing.T) {
	above := []uint16{400, 400, 400, 400}
	left := []uint16{400, 400, 400, 400}
	n := &Neighbors16{Above: above, Left: left, HaveAbove: true, HaveLeft: true, BitDepth: 10}
	dst := make([]uint16, 4*4)
	if err := PredictIntra16(dst, 4, 4, SmoothPred, n); err != nil {
		t.Fatalf("SMOOTH: %v", err)
	}
	for _, v := range dst {
		if v != 400 {
			t.Fatalf("SMOOTH constant got %d want 400", v)
		}
	}
}

func TestPredictIntra16DirectionalRunsAllAngles(t *testing.T) {
	above := make([]uint16, 64)
	left := make([]uint16, 64)
	for i := range above {
		above[i] = 500
		left[i] = 600
	}
	n := &Neighbors16{
		Above:         above,
		Left:          left,
		AboveExtended: above,
		LeftExtended:  left,
		HaveAbove:     true,
		HaveLeft:      true,
		BitDepth:      10,
	}
	for _, m := range []IntraMode{D45Pred, D67Pred, D113Pred, D135Pred, D157Pred, D203Pred} {
		dst := make([]uint16, 4*4)
		if err := PredictIntra16(dst, 4, 4, m, n); err != nil {
			t.Fatalf("%s: %v", m, err)
		}
		// Output should lie in [500, 600] — convex combination of two refs.
		for _, v := range dst {
			if v < 500 || v > 600 {
				t.Fatalf("%s sample out of expected range: %d", m, v)
			}
		}
	}
}
