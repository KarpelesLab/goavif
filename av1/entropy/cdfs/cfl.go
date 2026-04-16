package cdfs

// DefaultCFLSignCDF is default_cfl_sign_cdf from libaom entropymode.c.
// It encodes the joint sign of the U and V alpha components into 8
// symbols (CFL_JOINT_SIGNS).
var DefaultCFLSignCDF = AomCDF(1418, 2123, 13340, 18405, 26972, 28343, 32294)

// DefaultCFLAlphaCDF is default_cfl_alpha_cdf from libaom. Indexed by
// one of 6 joint-sign contexts (CFL_ALPHA_CONTEXTS), each encoding the
// magnitude of the U or V alpha component in 16 symbols.
var DefaultCFLAlphaCDF [6]CDF

func init() {
	DefaultCFLAlphaCDF[0] = AomCDF(7637, 20719, 31401, 32481, 32657, 32688, 32692,
		32696, 32700, 32704, 32708, 32712, 32716, 32720, 32724)
	DefaultCFLAlphaCDF[1] = AomCDF(14365, 23603, 28135, 31168, 32167, 32395, 32487,
		32573, 32620, 32647, 32668, 32672, 32676, 32680, 32684)
	DefaultCFLAlphaCDF[2] = AomCDF(11532, 22380, 28445, 31360, 32349, 32523, 32584,
		32649, 32673, 32677, 32681, 32685, 32689, 32693, 32697)
	DefaultCFLAlphaCDF[3] = AomCDF(26990, 31402, 32282, 32571, 32692, 32696, 32700,
		32704, 32708, 32712, 32716, 32720, 32724, 32728, 32732)
	DefaultCFLAlphaCDF[4] = AomCDF(17248, 26058, 28904, 30608, 31305, 31877, 32126,
		32321, 32394, 32464, 32516, 32560, 32576, 32593, 32622)
	DefaultCFLAlphaCDF[5] = AomCDF(14738, 21678, 25779, 27901, 29024, 30302, 30980,
		31843, 32144, 32413, 32520, 32594, 32622, 32656, 32660)
}
