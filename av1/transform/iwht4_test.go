package transform

import "testing"

func TestIWHT4Reversible(t *testing.T) {
	// The WHT is reversible — applying it twice (using a matching forward
	// WHT that just reverses the operation) should recover the input.
	// We don't have a forward WHT, so we just check it runs and produces
	// finite output.
	x := []int32{16, 0, 0, 0}
	IWHT4(x)
	// With DC=16 and UNIT_QUANT_SHIFT=2, pre-shift a=4, b=c=d=0.
	// a+=c → 4; d-=b → 0; e=(4-0)>>1=2; b=e-b=2; c=e-c=2; a-=b=2; d+=c=2.
	want := []int32{2, 2, 2, 2}
	for i, v := range x {
		if v != want[i] {
			t.Errorf("x[%d]=%d want %d", i, v, want[i])
		}
	}
}

func TestIWHT4Linearity(t *testing.T) {
	// WHT is an integer linear operator with exact arithmetic — linearity
	// should hold precisely modulo the right-shift rounding.
	a := []int32{40, 16, -8, 20}
	b := []int32{-24, 4, 32, -12}
	sum := []int32{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	IWHT4(ac)
	IWHT4(bc)
	IWHT4(sum)
	for i := 0; i < 4; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 3 {
			t.Errorf("WHT linearity at %d: got %d want %d", i, sum[i], want)
		}
	}
}
