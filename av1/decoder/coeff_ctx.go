package decoder

// AV1 coefficient context derivation per spec §6.10.6.
//
// Coefficients are decoded in reverse scan order: first the coefficient at
// position scan[eob-1], then scan[eob-2], and so on down to the DC at
// scan[0]. The sig_coef_ctx used to select a CDF for each coefficient's
// base level is derived from:
//
//  1. A "neighbor sum template" over 5 positions adjacent to the current
//     one (toward higher scan indices), clamped and added up.
//  2. A position-dependent offset from nz_map_ctx_offset[tx_size].
//
// This file implements the 2D-scan template used by all square blocks
// and the 4×4/8×8 sizes that intra-only AVIF stills typically use.

// template2DOffsets lists the 5 neighbor positions sampled for a 2D scan's
// coefficient context: (dr, dc) offsets from the current (r, c).
var template2DOffsets = [5][2]int{
	{0, 1}, {1, 0}, {1, 1}, {0, 2}, {2, 0},
}

// SigCoefCtx2D returns the sig_coef_ctx for a coefficient at block
// position (r, c) in a block of width w and height h, given the
// partial absolute-level grid decoded so far (absLevels[r*w+c], clamped
// to 0..3) and the position-specific offset from nzMapOffset at
// scanIdx.
func SigCoefCtx2D(r, c, w, h int, absLevels []int8, nzMapOffset []int8, scanIdx int) int {
	if scanIdx == 0 {
		return 0
	}
	stats := 0
	for _, d := range template2DOffsets {
		rr := r + d[0]
		cc := c + d[1]
		if rr < h && cc < w {
			v := int(absLevels[rr*w+cc])
			if v > 3 {
				v = 3
			}
			stats += v
		}
	}
	ctxBase := (stats + 1) >> 1
	if ctxBase > 4 {
		ctxBase = 4
	}
	return ctxBase + int(nzMapOffset[scanIdx])
}

// LevelCtx returns the coeff_br_ctx for a coefficient at position (r, c)
// given the partial decoded levels. Used when reading additional base-
// range levels beyond NUM_BASE_LEVELS (2).
//
// Spec §6.10.7 defines this as a template of 3 neighbor positions
// clamped to levels beyond base, then a scan-position offset.
func LevelCtx(r, c, w, h int, absLevels []int8) int {
	stats := 0
	for _, d := range [3][2]int{{0, 1}, {1, 0}, {1, 1}} {
		rr := r + d[0]
		cc := c + d[1]
		if rr < h && cc < w {
			v := int(absLevels[rr*w+cc])
			if v > 3 {
				v -= 3
				if v > 3 {
					v = 3
				}
				stats += v
			}
		}
	}
	ctx := (stats + 1) >> 1
	if ctx > 3 {
		ctx = 3
	}
	// Position bias: DC and first row/column share small contexts.
	// Simplified: return ctx alone; full spec adds a position-based
	// remap via another offset table (deferred).
	return ctx
}
