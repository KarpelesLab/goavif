package transform

import "testing"

func TestIDCT8DCConstantOutput(t *testing.T) {
	x := []int32{16384, 0, 0, 0, 0, 0, 0, 0}
	IDCT8(x)
	for i := 1; i < 8; i++ {
		if x[i] != x[0] {
			t.Errorf("DC reconstruction non-constant at %d: got %d, first = %d", i, x[i], x[0])
		}
	}
	if x[0] == 0 {
		t.Errorf("DC reconstruction zero")
	}
}

func TestIDCT8Linearity(t *testing.T) {
	a := []int32{100, 50, -25, 10, 5, -40, 20, 15}
	b := []int32{-30, 40, 5, -12, 22, 7, -18, 3}
	sum := make([]int32, 8)
	for i := 0; i < 8; i++ {
		sum[i] = a[i] + b[i]
	}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	IDCT8(ac)
	IDCT8(bc)
	IDCT8(sum)
	for i := 0; i < 8; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		// Integer IDCT with rounding at every butterfly stage is only
		// approximately linear; spec-conformant implementations can differ by
		// a few ulps across separate-vs-combined evaluations.
		if diff > 4 {
			t.Errorf("linearity at i=%d: got %d want %d", i, sum[i], want)
		}
	}
}

func TestIADST4Linearity(t *testing.T) {
	a := []int32{100, 50, -25, 10}
	b := []int32{-30, 40, 5, -12}
	sum := []int32{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	IADST4(ac)
	IADST4(bc)
	IADST4(sum)
	for i := 0; i < 4; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Errorf("linearity at i=%d: got %d want %d", i, sum[i], want)
		}
	}
}

func TestIDTXPreservesStructure(t *testing.T) {
	x := []int32{1, 2, 3, 4}
	IDTX4(x)
	want := []int32{2, 4, 6, 8}
	for i := 0; i < 4; i++ {
		if x[i] != want[i] {
			t.Errorf("IDTX4[%d]=%d want %d", i, x[i], want[i])
		}
	}
}
