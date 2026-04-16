package cdfs

// DefaultAngleDeltaCDF is the default_angle_delta_cdf from libaom.
// Indexed by directional mode index (0..7, corresponding to D45..D67),
// each a 7-symbol CDF for angle_delta in {-3, -2, -1, 0, +1, +2, +3}.
//
// Raw AOM_CDF7 values from libaom, inverted by AomCDF.
var DefaultAngleDeltaCDF [8]CDF

func init() {
	DefaultAngleDeltaCDF[0] = AomCDF(2180, 5032, 7567, 22776, 26989, 30217)
	DefaultAngleDeltaCDF[1] = AomCDF(2301, 5608, 8801, 23487, 26974, 30330)
	DefaultAngleDeltaCDF[2] = AomCDF(3780, 11018, 13699, 19354, 23083, 31286)
	DefaultAngleDeltaCDF[3] = AomCDF(4581, 11226, 15147, 17138, 21834, 28397)
	DefaultAngleDeltaCDF[4] = AomCDF(1737, 10927, 14509, 19588, 22745, 28823)
	DefaultAngleDeltaCDF[5] = AomCDF(2664, 10176, 12485, 17650, 21600, 30495)
	DefaultAngleDeltaCDF[6] = AomCDF(2240, 11096, 15453, 20341, 22561, 28917)
	DefaultAngleDeltaCDF[7] = AomCDF(3605, 10428, 12459, 17676, 21244, 30655)
}
