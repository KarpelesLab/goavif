package predict

import "testing"

func TestCFLSubsample16FourToOneAveragesBox(t *testing.T) {
	luma := []uint16{
		100, 200,
		300, 400,
	}
	dst := make([]int32, 1)
	CFLSubsample16(dst, luma, 2, 2, 1, 1)
	// box avg = (100+200+300+400)/4 = 250; stored *8 = 2000.
	if dst[0] != 2000 {
		t.Fatalf("4:1 subsample: got %d want 2000", dst[0])
	}
}

func TestCFLSubsample16NoSubsample(t *testing.T) {
	luma := []uint16{500, 1500, 800, 2500}
	dst := make([]int32, 4)
	CFLSubsample16(dst, luma, 2, 2, 0, 0)
	// Each output is the *8 scaled of the single input sample.
	want := []int32{4000, 12000, 6400, 20000}
	for i, v := range dst {
		if v != want[i] {
			t.Errorf("dst[%d]=%d want %d", i, v, want[i])
		}
	}
}

func TestCFLPred16AlphaZeroIsDCPred(t *testing.T) {
	dc := []uint16{500, 500, 500, 500}
	lumaQ3 := []int32{100, 200, 300, 400}
	dst := make([]uint16, 4)
	CFLPred16(dst, 2, 2, lumaQ3, dc, 0, 10)
	for i, v := range dst {
		if v != 500 {
			t.Fatalf("alpha=0 should leave dc unchanged; dst[%d]=%d", i, v)
		}
	}
}

func TestCFLPred16PositiveAlphaAddsAC(t *testing.T) {
	dc := []uint16{500, 500, 500, 500}
	// lumaQ3 avg = 250; AC values: -150, -50, 50, 150.
	lumaQ3 := []int32{100, 200, 300, 400}
	dst := make([]uint16, 4)
	// alpha = 64 → scaling ≈ 1:1 after the /64 shift.
	CFLPred16(dst, 2, 2, lumaQ3, dc, 64, 10)
	// dst[0] ≈ 500 + round(-150) = 350; dst[3] ≈ 500 + 150 = 650.
	if dst[0] > 360 || dst[0] < 340 {
		t.Fatalf("dst[0] expected ~350 got %d", dst[0])
	}
	if dst[3] < 640 || dst[3] > 660 {
		t.Fatalf("dst[3] expected ~650 got %d", dst[3])
	}
}

func TestCFLPred16ClipsBitDepth(t *testing.T) {
	dc := []uint16{4000, 4000, 4000, 4000}
	lumaQ3 := []int32{100, 200, 1000, 2000}
	dst := make([]uint16, 4)
	CFLPred16(dst, 2, 2, lumaQ3, dc, 64, 12)
	maxV := uint16((1 << 12) - 1)
	for i, v := range dst {
		if v > maxV {
			t.Fatalf("dst[%d]=%d exceeds 12-bit max", i, v)
		}
	}
}
