package transform

import "testing"

func TestIDCT16DCConstantOutput(t *testing.T) {
	x := make([]int32, 16)
	x[0] = 32768
	IDCT16(x)
	// DC coefficient alone must reconstruct a constant block.
	first := x[0]
	for i := 1; i < 16; i++ {
		if x[i] != first {
			t.Errorf("DC reconstruction non-constant: sample[%d]=%d vs sample[0]=%d",
				i, x[i], first)
		}
	}
	if first == 0 {
		t.Errorf("DC reconstruction is zero")
	}
}

func TestIDCT16Linearity(t *testing.T) {
	a := []int32{100, 50, -25, 10, 5, -40, 20, 15, 0, -33, 7, 60, -11, 4, -8, 22}
	b := []int32{-30, 40, 5, -12, 22, 7, -18, 3, 10, 5, -2, -15, 33, -4, 9, -7}
	sum := make([]int32, 16)
	for i := 0; i < 16; i++ {
		sum[i] = a[i] + b[i]
	}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	IDCT16(ac)
	IDCT16(bc)
	IDCT16(sum)
	// Slightly wider ulp tolerance for the 16-point cascade (7 butterfly
	// stages vs 3 for IDCT4).
	for i := 0; i < 16; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 12 {
			t.Errorf("linearity at %d: got %d want %d", i, sum[i], want)
		}
	}
}
