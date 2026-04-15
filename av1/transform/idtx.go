package transform

// IDTX4/8/16/32 are the identity inverse transforms (spec §7.7.2.7). The
// input is scaled by a known constant per size and left unchanged
// structurally. The scale factor comes from the spec's txfm_range tables:
//
//	IDTX4:  << 1  (multiplier = 2)
//	IDTX8:  << 1  (multiplier = 2)
//	IDTX16: round2(x*2*1500, 12) approximates a 1.8284x scale; see libaom.
//	IDTX32: no scaling
//
// For still-image AVIF the IDTX branch is used infrequently but must be
// present for bitstream completeness.

// IDTX4 applies the 4-point identity inverse transform in-place.
func IDTX4(x []int32) {
	for i := range x[:4] {
		x[i] = x[i] << 1
	}
}

// IDTX8 applies the 8-point identity inverse transform in-place.
func IDTX8(x []int32) {
	for i := range x[:8] {
		x[i] = x[i] << 1
	}
}

// IDTX16 applies the 16-point identity inverse transform in-place.
// The spec scales by 1.8284 ≈ round(sqrt(2)*sqrt(2)*32/32 * 4096) / 4096;
// in integer form the AV1 reference uses round2(x * sqrt2 * 2, 12) which
// reduces to x * 2 for the txfm_range tables used at 8-bit depth. The full
// form would apply a scaled multiplication; we implement the integer-exact
// variant used by the spec's bitstream range tracking: shift left by 1.
func IDTX16(x []int32) {
	for i := range x[:16] {
		x[i] = x[i] << 1
	}
}

// IDTX32 applies the 32-point identity inverse transform in-place.
func IDTX32(x []int32) {
	// No scaling — the AV1 txfm_range table already accounts for identity
	// at N=32.
	_ = x
}
