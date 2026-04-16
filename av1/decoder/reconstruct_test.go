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

func TestReconstruct16ClipsToBitDepth(t *testing.T) {
	pred := []uint16{1000, 100, 0, 4000}
	res := []int32{50, -200, -50, 200}
	dst := make([]uint16, 4)
	Reconstruct16Block(dst, pred, res, 2, 2, 12)
	// Expected: 1050, 0 (100-200 clips low), 0 (0-50 clips low), 4095 (4000+200 > 4095).
	want := []uint16{1050, 0, 0, 4095}
	for i, v := range dst {
		if v != want[i] {
			t.Errorf("dst[%d]=%d want %d", i, v, want[i])
		}
	}
}

func TestReconstruct16RespectsDifferentBitDepths(t *testing.T) {
	// 10-bit: max 1023.
	pred10 := []uint16{1000}
	res10 := []int32{100}
	dst10 := make([]uint16, 1)
	Reconstruct16Block(dst10, pred10, res10, 1, 1, 10)
	if dst10[0] != 1023 {
		t.Errorf("10-bit clip got %d want 1023", dst10[0])
	}
}
