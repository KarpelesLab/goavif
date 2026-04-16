package transform

import "testing"

func TestIADST8Linearity(t *testing.T) {
	a := []int32{100, 50, -25, 10, 5, -40, 20, 15}
	b := []int32{-30, 40, 5, -12, 22, 7, -18, 3}
	sum := make([]int32, 8)
	for i := 0; i < 8; i++ {
		sum[i] = a[i] + b[i]
	}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	IADST8(ac)
	IADST8(bc)
	IADST8(sum)
	for i := 0; i < 8; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 4 {
			t.Errorf("IADST8 linearity at %d: got %d want %d", i, sum[i], want)
		}
	}
}

func TestIFLIPADST8ReversesIADST8(t *testing.T) {
	a := []int32{100, 50, -25, 10, 5, -40, 20, 15}
	b := append([]int32(nil), a...)
	IADST8(a)
	IFLIPADST8(b)
	for i := 0; i < 8; i++ {
		if a[i] != b[7-i] {
			t.Errorf("IFLIPADST8[%d]=%d want %d (IADST8[%d])", i, b[7-i], a[i], i)
		}
	}
}
