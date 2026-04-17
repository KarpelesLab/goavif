package transform

import "testing"

// Forward2D then Inverse2D should reconstruct the input within a few ulps
// per coefficient (bounded by the per-1D rounding noise).
func TestForward2DRoundTripDCT(t *testing.T) {
	sizes := []struct {
		name string
		sz   TxSize
		w, h int
	}{
		{"4x4", Tx4x4, 4, 4},
		{"8x8", Tx8x8, 8, 8},
		{"16x16", Tx16x16, 16, 16},
		{"32x32", Tx32x32, 32, 32},
		{"4x8", Tx4x8, 4, 8},
		{"8x4", Tx8x4, 8, 4},
		{"8x16", Tx8x16, 8, 16},
		{"16x8", Tx16x8, 16, 8},
		{"16x32", Tx16x32, 16, 32},
		{"32x16", Tx32x16, 32, 16},
		{"64x64", Tx64x64, 64, 64},
	}
	for _, c := range sizes {
		t.Run(c.name, func(t *testing.T) {
			src := make([]int32, c.w*c.h)
			for i := range src {
				src[i] = int32((i*37 + 11) % 250 - 125)
			}
			cp := append([]int32(nil), src...)
			if err := Forward2D(cp, DctDct, c.sz); err != nil {
				t.Fatalf("Forward2D: %v", err)
			}
			if err := Inverse2D(cp, DctDct, c.sz); err != nil {
				t.Fatalf("Inverse2D: %v", err)
			}
			maxDim := c.w
			if c.h > maxDim {
				maxDim = c.h
			}
			tol := int32(maxDim)
			for i, v := range cp {
				diff := v - src[i]
				if diff < -tol || diff > tol {
					t.Fatalf("roundtrip diff at %d: got %d want %d (diff %d, tol %d)",
						i, v, src[i], diff, tol)
				}
			}
		})
	}
}

// Constant input: Forward2D should concentrate energy at DC only.
func TestForward2DConstantInputYieldsDCOnly(t *testing.T) {
	w, h := 4, 4
	coeffs := make([]int32, w*h)
	for i := range coeffs {
		coeffs[i] = 100
	}
	if err := Forward2D(coeffs, DctDct, Tx4x4); err != nil {
		t.Fatalf("Forward2D: %v", err)
	}
	if coeffs[0] == 0 {
		t.Fatalf("DC is zero after Forward2D of constant input")
	}
	for i := 1; i < w*h; i++ {
		if coeffs[i] < -1 || coeffs[i] > 1 {
			t.Fatalf("non-DC coefficient %d = %d should be ~0", i, coeffs[i])
		}
	}
}
