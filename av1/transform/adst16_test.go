package transform

import "testing"

func TestIADST16Linearity(t *testing.T) {
	a := make([]int32, 16)
	b := make([]int32, 16)
	for i := 0; i < 16; i++ {
		a[i] = int32((i*37)%200 - 100)
		b[i] = int32((i*23)%150 - 75)
	}
	sum := make([]int32, 16)
	for i := 0; i < 16; i++ {
		sum[i] = a[i] + b[i]
	}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	IADST16(ac)
	IADST16(bc)
	IADST16(sum)
	for i := 0; i < 16; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 12 {
			t.Errorf("IADST16 linearity at %d: got %d want %d", i, sum[i], want)
		}
	}
}

func TestIFLIPADST16ReversesIADST16(t *testing.T) {
	a := make([]int32, 16)
	for i := range a {
		a[i] = int32(i*17 - 50)
	}
	b := append([]int32(nil), a...)
	IADST16(a)
	IFLIPADST16(b)
	for i := 0; i < 16; i++ {
		if a[i] != b[15-i] {
			t.Errorf("IFLIPADST16[%d]=%d want %d (IADST16[%d])", i, b[15-i], a[i], i)
		}
	}
}
