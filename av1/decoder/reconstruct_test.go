package decoder

import "testing"

func TestReconstructAddsResidual(t *testing.T) {
	pred := []uint8{100, 150, 200, 50}
	res := []int32{10, -20, 100, -100}
	dst := make([]uint8, 4)
	ReconstructBlock(dst, pred, res, 2, 2)
	want := []uint8{110, 130, 255, 0} // 200+100 clips to 255; 50-100 clips to 0
	for i, v := range dst {
		if v != want[i] {
			t.Errorf("dst[%d]=%d want %d", i, v, want[i])
		}
	}
}

func TestReconstructZeroResidualCopiesPred(t *testing.T) {
	pred := []uint8{42, 42, 42, 42}
	res := []int32{0, 0, 0, 0}
	dst := make([]uint8, 4)
	ReconstructBlock(dst, pred, res, 2, 2)
	for i, v := range dst {
		if v != 42 {
			t.Errorf("dst[%d]=%d want 42", i, v)
		}
	}
}
