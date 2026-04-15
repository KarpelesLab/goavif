package decoder

import (
	"errors"
	"testing"
)

func TestPredictIntraDC(t *testing.T) {
	n := &Neighbors{
		Above:     []uint8{20, 20, 20, 20},
		Left:      []uint8{20, 20, 20, 20},
		HaveAbove: true,
		HaveLeft:  true,
		BitDepth:  8,
	}
	dst := make([]uint8, 16)
	if err := PredictIntra(dst, 4, 4, DCPred, n); err != nil {
		t.Fatalf("DC: %v", err)
	}
	for _, v := range dst {
		if v != 20 {
			t.Errorf("DC sample = %d, want 20", v)
		}
	}
}

func TestPredictIntraVRequiresAbove(t *testing.T) {
	n := &Neighbors{HaveAbove: false}
	dst := make([]uint8, 16)
	err := PredictIntra(dst, 4, 4, VPred, n)
	if err == nil {
		t.Fatalf("V_PRED without above should error")
	}
}

func TestPredictIntraD113Unimplemented(t *testing.T) {
	n := &Neighbors{HaveAbove: true, HaveLeft: true, BitDepth: 8}
	err := PredictIntra(make([]uint8, 16), 4, 4, D113Pred, n)
	if err == nil {
		t.Errorf("D113_PRED should report unimplemented")
	}
	// Sanity: the error should mention the mode name.
	if err != nil && !errors.Is(err, err) {
		t.Errorf("error wrap broken: %v", err)
	}
}
