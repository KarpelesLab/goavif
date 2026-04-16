package quant

import "testing"

func TestQuantizeCoeffRoundsNearest(t *testing.T) {
	v := Values{DC: 100, AC: 200}
	// DC: raw 250 / 100 → 2.5 rounds to 3.
	if got := QuantizeCoeff(250, 0, v); got != 3 {
		t.Fatalf("QuantizeCoeff DC 250/100 = %d, want 3", got)
	}
	// DC: raw 249 / 100 → 2.49 rounds to 2.
	if got := QuantizeCoeff(249, 0, v); got != 2 {
		t.Fatalf("QuantizeCoeff DC 249/100 = %d, want 2", got)
	}
	// AC: raw -300 / 200 → -1.5 rounds to -2 (away from zero).
	if got := QuantizeCoeff(-300, 1, v); got != -2 {
		t.Fatalf("QuantizeCoeff AC -300/200 = %d, want -2", got)
	}
}

func TestQuantizeCoeffZeroQuantizerIsNoop(t *testing.T) {
	v := Values{DC: 0, AC: 0}
	for _, x := range []int32{-1000, -1, 0, 1, 1000} {
		if got := QuantizeCoeff(x, 0, v); got != 0 {
			t.Fatalf("QuantizeCoeff with Q=0 returned %d for %d", got, x)
		}
	}
}

func TestQuantizeBlockAppliesDCAndAC(t *testing.T) {
	block := []int32{500, 100, -200, 300}
	v := Values{DC: 100, AC: 50}
	QuantizeBlock(block, v)
	// Position 0 uses DC: 500/100 = 5.
	// Positions 1,2,3 use AC: 100/50=2, -200/50=-4, 300/50=6.
	want := []int32{5, 2, -4, 6}
	for i := range block {
		if block[i] != want[i] {
			t.Fatalf("block[%d] = %d, want %d", i, block[i], want[i])
		}
	}
}
