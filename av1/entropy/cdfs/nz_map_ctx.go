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

// NzMapCtxOffset32x32 is av1_nz_map_ctx_offset_32x32 — generated at
// init() time. The 2D square tables all share the structure:
//
//	(0,0)=0  (0,1)=1  (0,2)=6  (0,3)=6   others in row 0 = 21
//	(1,0)=1  (1,1)=6  (1,2)=6  others in row 1 = 21
//	(2,0)=6  (2,1)=6  others in row 2 = 21
//	(3,0)=6  others in row 3 = 21
//	rows 4..N-1: all 21
var NzMapCtxOffset32x32 [1024]int8

func init() {
	for i := range NzMapCtxOffset32x32 {
		NzMapCtxOffset32x32[i] = 21
	}
	setNzSquare(NzMapCtxOffset32x32[:], 32)
}

// setNzSquare writes the 4×4 top-left corner of a square nz_map offset
// table. Used for sizes where the reference tables follow the same
// structure (16×16 and 32×32 verified).
func setNzSquare(tbl []int8, n int) {
	tbl[0*n+0] = 0
	tbl[0*n+1] = 1
	tbl[0*n+2] = 6
	tbl[0*n+3] = 6
	tbl[1*n+0] = 1
	tbl[1*n+1] = 6
	tbl[1*n+2] = 6
	tbl[2*n+0] = 6
	tbl[2*n+1] = 6
	tbl[3*n+0] = 6
}

// NzMapCtxOffset16x16 is av1_nz_map_ctx_offset_16x16.
var NzMapCtxOffset16x16 = [256]int8{
	0, 1, 6, 6, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	1, 6, 6, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	6, 6, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	6, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
	21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21, 21,
}
