package transform

import "testing"

func TestIDCT64DCConstantOutput(t *testing.T) {
	x := make([]int32, 64)
	x[0] = 65536
	IDCT64(x)
	first := x[0]
	for i := 1; i < 64; i++ {
		if x[i] != first {
			t.Errorf("DC reconstruction non-constant: x[%d]=%d vs x[0]=%d", i, x[i], first)
		}
	}
	if first == 0 {
		t.Errorf("DC reconstruction is zero")
	}
}

func TestIDCT64Linearity(t *testing.T) {
	a := make([]int32, 64)
	b := make([]int32, 64)
	for i := 0; i < 64; i++ {
		a[i] = int32((i*37)%200 - 100)
		b[i] = int32((i*23)%150 - 75)
	}
	sum := make([]int32, 64)
	for i := 0; i < 64; i++ {
		sum[i] = a[i] + b[i]
	}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	IDCT64(ac)
	IDCT64(bc)
	IDCT64(sum)
	for i := 0; i < 64; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		// 11-stage cascade accumulates more rounding error.
		if diff > 60 {
			t.Errorf("IDCT64 linearity at %d: got %d want %d", i, sum[i], want)
		}
	}
}
