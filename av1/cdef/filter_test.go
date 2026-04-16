package cdef

import "testing"

func TestConstrainZeroThreshold(t *testing.T) {
	// Zero threshold → always returns 0.
	if got := Constrain(42, 0, 5); got != 0 {
		t.Errorf("Constrain(42, 0, 5) = %d, want 0", got)
	}
}

func TestConstrainClampsToLimit(t *testing.T) {
	// Small diff within the threshold → returned unchanged (barring shift).
	if got := Constrain(3, 10, 6); got != 3 {
		t.Errorf("Constrain(3, 10, 6) = %d, want 3", got)
	}
	// Sign is preserved.
	if got := Constrain(-3, 10, 6); got != -3 {
		t.Errorf("Constrain(-3, 10, 6) = %d, want -3", got)
	}
}

func TestConstrainLargeDiffTaperedToZero(t *testing.T) {
	// |diff| far beyond threshold with small damping should taper to 0.
	got := Constrain(1000, 10, 3)
	if got > 0 || got < -10 {
		t.Errorf("Constrain(1000, 10, 3) = %d, expect in [-10, 0]", got)
	}
}

func TestFilterBlockConstantInputIsIdentity(t *testing.T) {
	// With a perfectly flat block, every neighbor difference is 0, so
	// the filter output equals the input regardless of strengths.
	stride := 16
	src := make([]uint8, 16*16)
	for i := range src {
		src[i] = 100
	}
	dst := make([]uint8, 16*16)
	copy(dst, src)
	FilterBlock(dst, src, stride, 4, 4, 2, 20, 10, 6)
	for r := 4; r < 12; r++ {
		for c := 4; c < 12; c++ {
			if dst[r*stride+c] != 100 {
				t.Errorf("flat block sample[%d,%d]=%d, want 100", r, c, dst[r*stride+c])
			}
		}
	}
}

func TestFilterBlockZeroStrengthIsIdentity(t *testing.T) {
	// Strength 0 everywhere → Constrain returns 0 → output = input.
	stride := 16
	src := make([]uint8, 16*16)
	for i := range src {
		src[i] = uint8((i * 7) & 0xFF)
	}
	dst := make([]uint8, 16*16)
	FilterBlock(dst, src, stride, 4, 4, 2, 0, 0, 3)
	for r := 4; r < 12; r++ {
		for c := 4; c < 12; c++ {
			if dst[r*stride+c] != src[r*stride+c] {
				t.Errorf("zero-strength changed sample[%d,%d]: %d → %d",
					r, c, src[r*stride+c], dst[r*stride+c])
			}
		}
	}
}
