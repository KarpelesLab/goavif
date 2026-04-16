package cdfs

// DefaultPartitionCDF is the default_partition_cdf from libaom
// av1/common/entropymode.c. It has 20 entries (PARTITION_CONTEXTS):
//
//	[0..3]   = BLOCK_8x8 contexts   — 4 symbols (NONE/HORZ/VERT/SPLIT)
//	[4..7]   = BLOCK_16x16 contexts — 10 symbols (+ HORZ_A/B, VERT_A/B, HORZ_4, VERT_4)
//	[8..11]  = BLOCK_32x32 contexts — 10 symbols
//	[12..15] = BLOCK_64x64 contexts — 10 symbols
//	[16..19] = BLOCK_128x128 ctxts  — 8 symbols (no HORZ_4/VERT_4)
//
// Within each group, the 4 contexts encode (left_has_split, above_has_split):
//
//	0 = neither, 1 = above only, 2 = left only, 3 = both
//
// Raw values are from the AOM_CDF* macros in libaom; AomCDF inverts them
// into the wire-format storage expected by the entropy decoder.
var DefaultPartitionCDF [20]CDF

func init() {
	// BLOCK_8x8: CDF4
	DefaultPartitionCDF[0] = AomCDF(19132, 25510, 30392)
	DefaultPartitionCDF[1] = AomCDF(13928, 19855, 28540)
	DefaultPartitionCDF[2] = AomCDF(12522, 23679, 28629)
	DefaultPartitionCDF[3] = AomCDF(9896, 18783, 25853)

	// BLOCK_16x16: CDF10
	DefaultPartitionCDF[4] = AomCDF(15597, 20929, 24571, 26706, 27664, 28821, 29601, 30571, 31902)
	DefaultPartitionCDF[5] = AomCDF(7925, 11043, 16785, 22470, 23971, 25043, 26651, 28701, 29834)
	DefaultPartitionCDF[6] = AomCDF(5414, 13269, 15111, 20488, 22360, 24500, 25537, 26336, 32117)
	DefaultPartitionCDF[7] = AomCDF(2662, 6362, 8614, 20860, 23053, 24778, 26436, 27829, 31171)

	// BLOCK_32x32: CDF10
	DefaultPartitionCDF[8] = AomCDF(18462, 20920, 23124, 27647, 28227, 29049, 29519, 30178, 31544)
	DefaultPartitionCDF[9] = AomCDF(7689, 9060, 12056, 24992, 25660, 26182, 26951, 28041, 29052)
	DefaultPartitionCDF[10] = AomCDF(6015, 9009, 10062, 24544, 25409, 26545, 27071, 27526, 32047)
	DefaultPartitionCDF[11] = AomCDF(1394, 2208, 2796, 28614, 29061, 29466, 29840, 30185, 31899)

	// BLOCK_64x64: CDF10
	DefaultPartitionCDF[12] = AomCDF(20137, 21547, 23078, 29566, 29837, 30261, 30524, 30892, 31724)
	DefaultPartitionCDF[13] = AomCDF(6732, 7490, 9497, 27944, 28250, 28515, 28969, 29630, 30104)
	DefaultPartitionCDF[14] = AomCDF(5945, 7663, 8348, 28683, 29117, 29749, 30064, 30298, 32238)
	DefaultPartitionCDF[15] = AomCDF(870, 1212, 1487, 31198, 31394, 31574, 31743, 31881, 32332)

	// BLOCK_128x128: CDF8
	DefaultPartitionCDF[16] = AomCDF(27899, 28219, 28529, 32484, 32539, 32619, 32639)
	DefaultPartitionCDF[17] = AomCDF(6607, 6990, 8268, 32060, 32219, 32338, 32371)
	DefaultPartitionCDF[18] = AomCDF(5429, 6676, 7122, 32027, 32227, 32531, 32582)
	DefaultPartitionCDF[19] = AomCDF(711, 966, 1172, 32448, 32538, 32617, 32664)
}
