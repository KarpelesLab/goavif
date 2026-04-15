package predict

import "testing"

func TestCFLSubsample420(t *testing.T) {
	// 4x4 luma block, constant value 100 → chroma 2x2 should also be ~800
	// (value * 8 for Q3), regardless of box size.
	luma := []uint8{
		100, 100, 100, 100,
		100, 100, 100, 100,
		100, 100, 100, 100,
		100, 100, 100, 100,
	}
	chroma := make([]int32, 2*2)
	CFLSubsample(chroma, luma, 4, 4, 1, 1)
	for i, v := range chroma {
		if v != 800 {
			t.Errorf("chroma[%d] = %d, want 800", i, v)
		}
	}
}

func TestCFLZeroAlphaReturnsDC(t *testing.T) {
	// alpha=0 → chroma = dcPred, independent of luma.
	luma := make([]int32, 4)
	for i := range luma {
		luma[i] = 500 + int32(i*100)
	}
	dc := []uint8{50, 60, 70, 80}
	dst := make([]uint8, 4)
	CFLPred(dst, 2, 2, luma, dc, 0)
	for i, v := range dst {
		if v != dc[i] {
			t.Errorf("alpha=0 CFL[%d]=%d want %d (dcPred)", i, v, dc[i])
		}
	}
}

func TestCFLPositiveAlphaFollowsLumaContrast(t *testing.T) {
	// Luma with one bright pixel in an otherwise-flat block + alpha > 0.
	// The same position should become brighter than its neighbors in the
	// chroma output.
	luma := []int32{100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100, 100}
	luma[5] = 2000 // single bright AC component
	dc := make([]uint8, 16)
	for i := range dc {
		dc[i] = 128
	}
	dst := make([]uint8, 16)
	CFLPred(dst, 4, 4, luma, dc, 8)
	if dst[5] <= dst[0] {
		t.Errorf("alpha>0 CFL should lift sample 5 above 0: got %d vs %d", dst[5], dst[0])
	}
}
