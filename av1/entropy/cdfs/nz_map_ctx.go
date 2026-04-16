package cdfs

// NzMapCtxOffset4x4 is av1_nz_map_ctx_offset_4x4 from libaom
// av1/common/txb_common.c. Indexed by scan position, it gives the
// position-dependent offset added to the neighbor-template stat to
// produce the final sig_coef_ctx.
//
// The non-position-dependent "neighbor stat" is computed from the sum of
// absolute values of previously decoded coefficients at relative
// positions (c+1, r), (c, r+1), (c+1, r+1), (c+2, r), (c, r+2); see
// spec §6.10.6.
var NzMapCtxOffset4x4 = [16]int8{
	0, 1, 6, 6, 1, 6, 6, 21, 6, 6, 21, 21, 6, 21, 21, 21,
}

// NzMapCtxOffset8x8 is av1_nz_map_ctx_offset_8x8.
var NzMapCtxOffset8x8 = [64]int8{
	0, 1, 6, 6, 21, 21, 21, 21, 1, 6, 6, 21, 21, 21, 21, 21,
	6, 6, 21, 21, 21, 21, 21, 21, 6, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
}
