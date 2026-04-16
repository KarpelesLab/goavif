package transform

import "testing"

func TestIDCT32DCConstantOutput(t *testing.T) {
	x := make([]int32, 32)
	x[0] = 32768
	IDCT32(x)
	first := x[0]
	for i := 1; i < 32; i++ {
		if x[i] != first {
			t.Errorf("DC reconstruction non-constant: x[%d]=%d vs x[0]=%d", i, x[i], first)
		}
	}
	if first == 0 {
		t.Errorf("DC reconstruction is zero")
	}
}

func TestIDCT32Linearity(t *testing.T) {
	a := make([]int32, 32)
	b := make([]int32, 32)
	for i := 0; i < 32; i++ {
		a[i] = int32((i*37)%200 - 100)
		b[i] = int32((i*23)%150 - 75)
	}
	sum := make([]int32, 32)
	for i := 0; i < 32; i++ {
		sum[i] = a[i] + b[i]
	}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	IDCT32(ac)
	IDCT32(bc)
	IDCT32(sum)
	for i := 0; i < 32; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		// 9-stage cascade; allow slightly wider tolerance.
		if diff > 30 {
			t.Errorf("IDCT32 linearity at %d: got %d want %d", i, sum[i], want)
		}
	}
}
