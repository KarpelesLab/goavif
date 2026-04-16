package cdfs

// DefaultCoeffBaseEOBMultiCDF holds coeff_base_eob_multi (the coefficient
// base level at the eob position only). Indexed by
// [tx_size][plane_type][sig_coef_contexts_eob=4].
//
// NUM_BASE_LEVELS = 2, so each CDF has 3 symbols (levels 0, 1, 2+).
//
// Only Q context 0 is transcribed.
var DefaultCoeffBaseEOBMultiCDF [5][2][4]CDF

func init() {
	// TX_4X4 luma
	DefaultCoeffBaseEOBMultiCDF[0][0][0] = AomCDF(17837, 29055)
	DefaultCoeffBaseEOBMultiCDF[0][0][1] = AomCDF(29600, 31446)
	DefaultCoeffBaseEOBMultiCDF[0][0][2] = AomCDF(30844, 31878)
	DefaultCoeffBaseEOBMultiCDF[0][0][3] = AomCDF(24926, 28948)
	// TX_4X4 chroma
	DefaultCoeffBaseEOBMultiCDF[0][1][0] = AomCDF(21365, 30026)
	DefaultCoeffBaseEOBMultiCDF[0][1][1] = AomCDF(30512, 32423)
	DefaultCoeffBaseEOBMultiCDF[0][1][2] = AomCDF(31658, 32621)
	DefaultCoeffBaseEOBMultiCDF[0][1][3] = AomCDF(29630, 31881)
	// TX_8X8 luma
	DefaultCoeffBaseEOBMultiCDF[1][0][0] = AomCDF(5717, 26477)
	DefaultCoeffBaseEOBMultiCDF[1][0][1] = AomCDF(30491, 31703)
	DefaultCoeffBaseEOBMultiCDF[1][0][2] = AomCDF(31550, 32158)
	DefaultCoeffBaseEOBMultiCDF[1][0][3] = AomCDF(29648, 31491)
	// TX_8X8 chroma
	DefaultCoeffBaseEOBMultiCDF[1][1][0] = AomCDF(12608, 27820)
	DefaultCoeffBaseEOBMultiCDF[1][1][1] = AomCDF(30680, 32225)
	DefaultCoeffBaseEOBMultiCDF[1][1][2] = AomCDF(30809, 32335)
	DefaultCoeffBaseEOBMultiCDF[1][1][3] = AomCDF(31299, 32423)
	// TX_16X16 luma
	DefaultCoeffBaseEOBMultiCDF[2][0][0] = AomCDF(1786, 12612)
	DefaultCoeffBaseEOBMultiCDF[2][0][1] = AomCDF(30663, 31625)
	DefaultCoeffBaseEOBMultiCDF[2][0][2] = AomCDF(32339, 32468)
	DefaultCoeffBaseEOBMultiCDF[2][0][3] = AomCDF(31148, 31833)
	// TX_16X16 chroma
	DefaultCoeffBaseEOBMultiCDF[2][1][0] = AomCDF(18857, 23865)
	DefaultCoeffBaseEOBMultiCDF[2][1][1] = AomCDF(31428, 32428)
	DefaultCoeffBaseEOBMultiCDF[2][1][2] = AomCDF(31744, 32373)
	DefaultCoeffBaseEOBMultiCDF[2][1][3] = AomCDF(31775, 32526)
	// TX_32X32 luma
	DefaultCoeffBaseEOBMultiCDF[3][0][0] = AomCDF(1787, 2532)
	DefaultCoeffBaseEOBMultiCDF[3][0][1] = AomCDF(30832, 31662)
	DefaultCoeffBaseEOBMultiCDF[3][0][2] = AomCDF(31824, 32682)
	DefaultCoeffBaseEOBMultiCDF[3][0][3] = AomCDF(32133, 32569)
	// TX_32X32 chroma
	DefaultCoeffBaseEOBMultiCDF[3][1][0] = AomCDF(13751, 22235)
	DefaultCoeffBaseEOBMultiCDF[3][1][1] = AomCDF(32089, 32409)
	DefaultCoeffBaseEOBMultiCDF[3][1][2] = AomCDF(27084, 27920)
	DefaultCoeffBaseEOBMultiCDF[3][1][3] = AomCDF(29291, 32594)
	// TX_64X64 luma
	DefaultCoeffBaseEOBMultiCDF[4][0][0] = AomCDF(1725, 3449)
	DefaultCoeffBaseEOBMultiCDF[4][0][1] = AomCDF(31102, 31935)
	DefaultCoeffBaseEOBMultiCDF[4][0][2] = AomCDF(32457, 32613)
	DefaultCoeffBaseEOBMultiCDF[4][0][3] = AomCDF(32412, 32649)
	// TX_64X64 chroma (uniform — spec fills with 10923/21845)
	DefaultCoeffBaseEOBMultiCDF[4][1][0] = AomCDF(10923, 21845)
	DefaultCoeffBaseEOBMultiCDF[4][1][1] = AomCDF(10923, 21845)
	DefaultCoeffBaseEOBMultiCDF[4][1][2] = AomCDF(10923, 21845)
	DefaultCoeffBaseEOBMultiCDF[4][1][3] = AomCDF(10923, 21845)
}
