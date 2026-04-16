package decoder

import "testing"

func TestSigCoefCtxDCIsZero(t *testing.T) {
	levels := make([]int8, 16)
	offset := []int8{0, 1, 6, 6, 1, 6, 6, 21, 6, 6, 21, 21, 6, 21, 21, 21}
	if ctx := SigCoefCtx2D(0, 0, 4, 4, levels, offset, 0); ctx != 0 {
		t.Errorf("DC ctx = %d, want 0", ctx)
	}
}

func TestSigCoefCtxZeroNeighborsUsesPositionOffset(t *testing.T) {
	// For a 4×4 block with all-zero neighbors, stats = 0, ctxBase = 0.
	// The context = 0 + nzMapOffset[scanIdx].
	levels := make([]int8, 16)
	offset := []int8{0, 1, 6, 6, 1, 6, 6, 21, 6, 6, 21, 21, 6, 21, 21, 21}
	// scan index 1 → position (0, 1) → offset 1 → ctx = 1
	if ctx := SigCoefCtx2D(0, 1, 4, 4, levels, offset, 1); ctx != 1 {
		t.Errorf("scanIdx=1 ctx = %d, want 1", ctx)
	}
	// scan index 7 → position (1, 3) probably → offset 21 → ctx = 21
	if ctx := SigCoefCtx2D(1, 3, 4, 4, levels, offset, 7); ctx != 21 {
		t.Errorf("scanIdx=7 ctx = %d, want 21", ctx)
	}
}

func TestSigCoefCtxClampsStats(t *testing.T) {
	// A neighbor with a huge absolute value should still clamp to 3 in
	// the stats sum.
	levels := make([]int8, 16)
	levels[0*4+1] = 100 // neighbor at (0,1) when we're at (0,0)... but we can't.
	// Test clamping with a more realistic setup: (1, 0) from (0, 0) is
	// within range, but scan idx 0 is DC so we return 0. Use a non-DC
	// position with a large neighbor.
	levels = make([]int8, 16)
	levels[1*4+1] = 100 // position (1, 1) is a neighbor of (0, 0)
	levels[0*4+1] = 100 // position (0, 1)
	levels[1*4+0] = 100 // position (1, 0)
	// With our position at (0, 0), scanIdx=1:
	// stats = min(3, 100) * 3 = 9; ctxBase = (9+1)/2 = 5 → clamp to 4
	offset := []int8{0, 1, 6, 6, 1, 6, 6, 21, 6, 6, 21, 21, 6, 21, 21, 21}
	if ctx := SigCoefCtx2D(0, 0, 4, 4, levels, offset, 1); ctx != 4+1 {
		t.Errorf("clamped ctx = %d, want 5", ctx)
	}
}
