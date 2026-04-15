package transform

import "testing"

func TestFDCT4ConstantInput(t *testing.T) {
	// A constant input should produce a single non-zero DC coefficient.
	x := []int32{42, 42, 42, 42}
	FDCT4(x)
	if x[0] == 0 {
		t.Errorf("DC should be non-zero")
	}
	for i := 1; i < 4; i++ {
		if x[i] != 0 {
			t.Errorf("AC coefficient %d non-zero: %d", i, x[i])
		}
	}
}

func TestFDCT4Linearity(t *testing.T) {
	a := []int32{100, 50, -25, 10}
	b := []int32{-30, 40, 5, -12}
	sum := []int32{a[0] + b[0], a[1] + b[1], a[2] + b[2], a[3] + b[3]}
	ac := append([]int32(nil), a...)
	bc := append([]int32(nil), b...)
	FDCT4(ac)
	FDCT4(bc)
	FDCT4(sum)
	for i := 0; i < 4; i++ {
		want := ac[i] + bc[i]
		diff := sum[i] - want
		if diff < 0 {
			diff = -diff
		}
		if diff > 1 {
			t.Errorf("FDCT4 linearity at %d: got %d want %d", i, sum[i], want)
		}
	}
}

// NOTE: FDCT4 is not the algebraic inverse of IDCT4 in AV1 — the spec
// defines only the inverse normative, and encoders apply their own
// per-coefficient scaling during quantization to compensate. A
// forward+inverse roundtrip therefore does NOT recover the input directly,
// even up to a constant scale. The encoder phase will introduce the
// correct quantization step that closes the loop.
