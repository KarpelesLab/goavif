package decoder

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

func TestDecodeBlockSkip(t *testing.T) {
	// With Skip=true the result must equal the pure intra prediction.
	above := []uint8{50, 50, 50, 50}
	left := []uint8{50, 50, 50, 50}
	dst := make([]uint8, 16)
	in := &BlockInput{
		W: 4, H: 4,
		Mode: DCPred,
		Skip: true,
		Neighbors: &Neighbors{
			Above: above, Left: left,
			HaveAbove: true, HaveLeft: true,
			BitDepth: 8,
		},
	}
	if err := DecodeBlock(dst, in); err != nil {
		t.Fatalf("DecodeBlock: %v", err)
	}
	for i, v := range dst {
		if v != 50 {
			t.Errorf("skip path dst[%d]=%d want 50", i, v)
		}
	}
}

func TestDecodeBlockDCResidualLifts(t *testing.T) {
	// Predict 100 everywhere via DC, add a positive DC residual.
	above := []uint8{100, 100, 100, 100}
	left := []uint8{100, 100, 100, 100}
	coeffs := make([]int32, 16)
	coeffs[0] = 16384 // positive DC
	dst := make([]uint8, 16)
	in := &BlockInput{
		W: 4, H: 4,
		Mode: DCPred,
		TxType: transform.DctDct,
		TxSize: transform.Tx4x4,
		Coeffs: coeffs,
		Skip:   false,
		Neighbors: &Neighbors{
			Above: above, Left: left,
			HaveAbove: true, HaveLeft: true,
			BitDepth: 8,
		},
	}
	if err := DecodeBlock(dst, in); err != nil {
		t.Fatalf("DecodeBlock: %v", err)
	}
	// All samples should be > 100 because the DC residual is positive and
	// the inverse transform spreads it as a constant lift.
	for i, v := range dst {
		if v <= 100 {
			t.Errorf("dst[%d]=%d, want > 100 (predicted 100, residual lifted)", i, v)
		}
	}
}

func TestDequantCoeffSelectsDCorAC(t *testing.T) {
	vals := quant.Values{DC: 10, AC: 20}
	if got := DequantCoeff(3, 0, vals); got != 30 {
		t.Errorf("DC: got %d want 30", got)
	}
	if got := DequantCoeff(3, 5, vals); got != 60 {
		t.Errorf("AC: got %d want 60", got)
	}
}
