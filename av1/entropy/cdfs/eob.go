package cdfs

// End-of-block CDFs from libaom token_cdfs.h. Indexed by
// [TOKEN_CDF_Q_CTXS][PLANE_TYPES][2 eob_multi_ctx].
//
// The "16/32/64" suffix is the number of coefficients in the TX block:
//   eob_multi16  → TX_4X4  (16 coeffs) — CDF5
//   eob_multi32  → TX_4X8/8X4 (32 coeffs) — CDF6
//   eob_multi64  → TX_8X8  (64 coeffs) — CDF7

// DefaultEOBMulti16CDF is the eob CDF for 16-coefficient blocks (TX_4X4).
// [q_ctx][plane_type][eob_multi_ctx] — CDF5 (5 symbols).
var DefaultEOBMulti16CDF [4][2][2]CDF

func init() {
	type e = [2][2]CDF
	DefaultEOBMulti16CDF = [4]e{
		{{{5: 0}, {5: 0}}, {{5: 0}, {5: 0}}}, // placeholder, filled below
		{{{5: 0}, {5: 0}}, {{5: 0}, {5: 0}}},
		{{{5: 0}, {5: 0}}, {{5: 0}, {5: 0}}},
		{{{5: 0}, {5: 0}}, {{5: 0}, {5: 0}}},
	}
	// Q context 0
	DefaultEOBMulti16CDF[0][0][0] = AomCDF(840, 1039, 1980, 4895)
	DefaultEOBMulti16CDF[0][0][1] = AomCDF(370, 671, 1883, 4471)
	DefaultEOBMulti16CDF[0][1][0] = AomCDF(3247, 4950, 9688, 14563)
	DefaultEOBMulti16CDF[0][1][1] = AomCDF(1904, 3354, 7763, 14647)
	// Q context 1
	DefaultEOBMulti16CDF[1][0][0] = AomCDF(2125, 2551, 5165, 8946)
	DefaultEOBMulti16CDF[1][0][1] = AomCDF(513, 765, 1859, 6339)
	DefaultEOBMulti16CDF[1][1][0] = AomCDF(7637, 9498, 14259, 19108)
	DefaultEOBMulti16CDF[1][1][1] = AomCDF(2497, 4096, 8866, 16993)
	// Q context 2
	DefaultEOBMulti16CDF[2][0][0] = AomCDF(4016, 4897, 8881, 14968)
	DefaultEOBMulti16CDF[2][0][1] = AomCDF(716, 1105, 2646, 10056)
	DefaultEOBMulti16CDF[2][1][0] = AomCDF(11139, 13270, 18241, 23566)
	DefaultEOBMulti16CDF[2][1][1] = AomCDF(3192, 5032, 10297, 19755)
	// Q context 3
	DefaultEOBMulti16CDF[3][0][0] = AomCDF(6708, 8958, 14746, 22133)
	DefaultEOBMulti16CDF[3][0][1] = AomCDF(1222, 2074, 4783, 15410)
	DefaultEOBMulti16CDF[3][1][0] = AomCDF(19575, 21766, 26044, 29709)
	DefaultEOBMulti16CDF[3][1][1] = AomCDF(7297, 10767, 19273, 28194)
}

// DefaultEOBMulti32CDF is the eob CDF for 32-coefficient blocks.
// CDF6 (6 symbols).
var DefaultEOBMulti32CDF [4][2][2]CDF

func init() {
	DefaultEOBMulti32CDF[0][0][0] = AomCDF(400, 520, 977, 2102, 6542)
	DefaultEOBMulti32CDF[0][0][1] = AomCDF(210, 405, 1315, 3326, 7537)
	DefaultEOBMulti32CDF[0][1][0] = AomCDF(2636, 4273, 7588, 11794, 20401)
	DefaultEOBMulti32CDF[0][1][1] = AomCDF(1786, 3179, 6902, 11357, 19054)
	DefaultEOBMulti32CDF[1][0][0] = AomCDF(989, 1249, 2019, 4151, 10785)
	DefaultEOBMulti32CDF[1][0][1] = AomCDF(313, 441, 1099, 2917, 8562)
	DefaultEOBMulti32CDF[1][1][0] = AomCDF(8394, 10352, 13932, 18855, 26014)
	DefaultEOBMulti32CDF[1][1][1] = AomCDF(2578, 4124, 8181, 13670, 24234)
	DefaultEOBMulti32CDF[2][0][0] = AomCDF(2515, 3003, 4452, 8162, 16041)
	DefaultEOBMulti32CDF[2][0][1] = AomCDF(574, 821, 1836, 5089, 13128)
	DefaultEOBMulti32CDF[2][1][0] = AomCDF(13468, 16303, 20361, 25105, 29281)
	DefaultEOBMulti32CDF[2][1][1] = AomCDF(3542, 5502, 10415, 16760, 25644)
	DefaultEOBMulti32CDF[3][0][0] = AomCDF(4617, 5709, 8446, 13584, 23135)
	DefaultEOBMulti32CDF[3][0][1] = AomCDF(1156, 1702, 3675, 9274, 20539)
	DefaultEOBMulti32CDF[3][1][0] = AomCDF(22086, 24282, 27010, 29770, 31743)
	DefaultEOBMulti32CDF[3][1][1] = AomCDF(7699, 10897, 20891, 26926, 31628)
}

// DefaultEOBMulti64CDF is the eob CDF for 64-coefficient blocks (TX_8X8).
// CDF7 (7 symbols).
var DefaultEOBMulti64CDF [4][2][2]CDF

func init() {
	DefaultEOBMulti64CDF[0][0][0] = AomCDF(329, 498, 1101, 1784, 3265, 7758)
	DefaultEOBMulti64CDF[0][0][1] = AomCDF(335, 730, 1459, 5494, 8755, 12997)
	DefaultEOBMulti64CDF[0][1][0] = AomCDF(3505, 5304, 10086, 13814, 17684, 23370)
	DefaultEOBMulti64CDF[0][1][1] = AomCDF(1563, 2700, 4876, 10911, 14706, 22480)
	DefaultEOBMulti64CDF[1][0][0] = AomCDF(1260, 1446, 2253, 3712, 6652, 13369)
	DefaultEOBMulti64CDF[1][0][1] = AomCDF(401, 605, 1029, 2563, 5845, 12626)
	DefaultEOBMulti64CDF[1][1][0] = AomCDF(8609, 10612, 14624, 18714, 22614, 29024)
	DefaultEOBMulti64CDF[1][1][1] = AomCDF(1923, 3127, 5867, 9703, 14277, 27100)
	DefaultEOBMulti64CDF[2][0][0] = AomCDF(2374, 2772, 4583, 7276, 12288, 19706)
	DefaultEOBMulti64CDF[2][0][1] = AomCDF(497, 810, 1315, 3000, 7004, 15641)
	DefaultEOBMulti64CDF[2][1][0] = AomCDF(15050, 17126, 21410, 24886, 28156, 30726)
	DefaultEOBMulti64CDF[2][1][1] = AomCDF(4034, 6290, 10235, 14982, 21214, 28491)
	DefaultEOBMulti64CDF[3][0][0] = AomCDF(6307, 7541, 12060, 16358, 22553, 27865)
	DefaultEOBMulti64CDF[3][0][1] = AomCDF(1289, 2320, 3971, 7926, 14153, 24291)
	DefaultEOBMulti64CDF[3][1][0] = AomCDF(24212, 25708, 28268, 30035, 31307, 32049)
	DefaultEOBMulti64CDF[3][1][1] = AomCDF(8726, 12378, 19409, 26450, 30038, 32462)
}
