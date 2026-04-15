package transform

import "testing"

func TestInverse2DDcBlock(t *testing.T) {
	// A 4x4 block with a single DC coefficient should reconstruct to a
	// constant block via Inverse2D = IDCT4 on rows then columns.
	coeffs := make([]int32, 16)
	coeffs[0] = 16384
	if err := Inverse2D(coeffs, DctDct, Tx4x4); err != nil {
		t.Fatalf("Inverse2D: %v", err)
	}
	first := coeffs[0]
	for i, v := range coeffs {
		if v != first {
			t.Errorf("output not constant at %d: %d != %d", i, v, first)
		}
	}
	if first == 0 {
		t.Errorf("constant is zero")
	}
}

func TestInverse2DUnsupported(t *testing.T) {
	coeffs := make([]int32, 256)
	if err := Inverse2D(coeffs, DctDct, Tx16x16); err == nil {
		t.Errorf("expected error for 16x16 (IDCT16 not implemented)")
	}
}

func TestInverse2D_DctAdst4x4(t *testing.T) {
	// Just check it runs; linearity is already tested per-1D-op.
	coeffs := []int32{
		100, 50, -25, 10,
		-30, 40, 5, -12,
		22, 7, -18, 3,
		11, -9, 14, 8,
	}
	if err := Inverse2D(coeffs, DctAdst, Tx4x4); err != nil {
		t.Fatalf("Inverse2D: %v", err)
	}
}
