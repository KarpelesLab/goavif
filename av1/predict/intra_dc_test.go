package predict

import "testing"

func TestDCPredBothNeighbors(t *testing.T) {
	dst := make([]uint8, 4*4)
	above := []uint8{10, 20, 30, 40}
	left := []uint8{40, 50, 60, 70}
	DCPred(dst, 4, 4, above, left, true, true, 8)
	// average of (10+20+30+40+40+50+60+70) = 320 / 8 = 40
	for i, v := range dst {
		if v != 40 {
			t.Errorf("dst[%d]=%d, want 40", i, v)
		}
	}
}

func TestDCPredNoNeighbors(t *testing.T) {
	dst := make([]uint8, 4*4)
	DCPred(dst, 4, 4, nil, nil, false, false, 8)
	for i, v := range dst {
		if v != 128 {
			t.Errorf("dst[%d]=%d, want 128 (half-range)", i, v)
		}
	}
}

func TestDCPredLeftOnly(t *testing.T) {
	dst := make([]uint8, 4*4)
	left := []uint8{100, 100, 100, 100}
	DCPred(dst, 4, 4, nil, left, false, true, 8)
	for i, v := range dst {
		if v != 100 {
			t.Errorf("dst[%d]=%d, want 100", i, v)
		}
	}
}
