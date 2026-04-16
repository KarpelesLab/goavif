package cdfs

// DefaultDCSignCDF is the av1_default_dc_sign_cdfs from libaom
// token_cdfs.h. Indexed by [plane_type][dc_sign_context]:
//
//	plane_type 0 = luma (Y), 1 = chroma (U/V)
//	dc_sign_context 0..2 (derived from above + left DC signs)
//
// All 4 TOKEN_CDF_Q_CTXS use the same values in the reference, so we
// store a single copy and let the decoder index it for any Q range.
var DefaultDCSignCDF [2][3]CDF

func init() {
	// Luma
	DefaultDCSignCDF[0][0] = AomCDF(128 * 125) // 16000
	DefaultDCSignCDF[0][1] = AomCDF(128 * 102) // 13056
	DefaultDCSignCDF[0][2] = AomCDF(128 * 147) // 18816

	// Chroma
	DefaultDCSignCDF[1][0] = AomCDF(128 * 119) // 15232
	DefaultDCSignCDF[1][1] = AomCDF(128 * 101) // 12928
	DefaultDCSignCDF[1][2] = AomCDF(128 * 135) // 17280
}
