package cdfs

// AV1 inter-prediction default CDFs sourced from libaom
// (entropymode.c / entropymv.c). Used by the decoder's inter-frame
// block syntax path.
//
// Values are the original libaom P(X<=i)·32768 form; AomCDF inverts
// them to the decoder's P(X>i) storage.

// DefaultIsInterCDF picks intra vs inter for a block (libaom
// `default_intra_inter_cdf`). Semantic is P(intra); decoder negates
// if it wants P(inter).
var DefaultIsInterCDF = [4]CDF{
	AomCDF(806), AomCDF(16662), AomCDF(20186), AomCDF(26538),
}

// DefaultSkipModeCDF — 3 contexts.
var DefaultSkipModeCDF = [3]CDF{
	AomCDF(32621), AomCDF(20708), AomCDF(8127),
}

// DefaultSingleRefCDF — [REF_CONTEXTS=3][SINGLE_REFS-1=6]. Each
// inner value is a binary decision for one level of the reference
// tree (LAST_FRAME / LAST2 / LAST3 / GOLDEN / BWDREF / ALTREF2).
var DefaultSingleRefCDF = [3][6]CDF{
	{AomCDF(4897), AomCDF(1555), AomCDF(4236), AomCDF(8650), AomCDF(904), AomCDF(1444)},
	{AomCDF(16973), AomCDF(16751), AomCDF(19647), AomCDF(24773), AomCDF(11014), AomCDF(15087)},
	{AomCDF(29744), AomCDF(30279), AomCDF(31194), AomCDF(31895), AomCDF(26875), AomCDF(30304)},
}

// DefaultNewMvCDF — 6 NEWMV_MODE_CONTEXTS. Binary decision:
// NEWMV vs NOT-NEWMV.
var DefaultNewMvCDF = [6]CDF{
	AomCDF(24035), AomCDF(16630), AomCDF(15339),
	AomCDF(8386), AomCDF(12222), AomCDF(4676),
}

// DefaultZeroMvCDF — 2 GLOBALMV_MODE_CONTEXTS, binary (GLOBAL vs not).
var DefaultZeroMvCDF = [2]CDF{AomCDF(2175), AomCDF(1054)}

// DefaultRefMvCDF — 6 REFMV_MODE_CONTEXTS, binary (NEAREST vs NEAR).
var DefaultRefMvCDF = [6]CDF{
	AomCDF(23974), AomCDF(24188), AomCDF(17848),
	AomCDF(28622), AomCDF(24312), AomCDF(19923),
}

// DefaultDrlCDF — 3 DRL_MODE_CONTEXTS, binary.
var DefaultDrlCDF = [3]CDF{AomCDF(13104), AomCDF(24560), AomCDF(18945)}

// DefaultMvJointCDF — 4 symbols (MV_JOINT_ZERO / HNZVZ / HZVNZ / HNZVNZ).
var DefaultMvJointCDF = AomCDF(4096, 11264, 19328)

// DefaultMvSignCDF — [2 comps] binary.
var DefaultMvSignCDF = [2]CDF{AomCDF(128 * 128), AomCDF(128 * 128)}

// DefaultMvClassCDF — [2 comps] 11 symbols (MV_CLASS_0..10).
var DefaultMvClassCDF = [2]CDF{
	AomCDF(28672, 30976, 31858, 32320, 32551, 32656, 32740, 32757, 32762, 32767),
	AomCDF(28672, 30976, 31858, 32320, 32551, 32656, 32740, 32757, 32762, 32767),
}

// DefaultMvClass0BitCDF — [2 comps] binary.
var DefaultMvClass0BitCDF = [2]CDF{AomCDF(216 * 128), AomCDF(216 * 128)}

// DefaultMvClass0FrCDF — [2 comps][2 class0 bits] 4 symbols.
var DefaultMvClass0FrCDF = [2][2]CDF{
	{AomCDF(16384, 24576, 26624), AomCDF(12288, 21248, 24128)},
	{AomCDF(16384, 24576, 26624), AomCDF(12288, 21248, 24128)},
}

// DefaultMvClass0HpCDF — [2 comps] binary.
var DefaultMvClass0HpCDF = [2]CDF{AomCDF(160 * 128), AomCDF(160 * 128)}

// DefaultMvFrCDF — [2 comps] 4 symbols.
var DefaultMvFrCDF = [2]CDF{
	AomCDF(8192, 17408, 21248),
	AomCDF(8192, 17408, 21248),
}

// DefaultMvHpCDF — [2 comps] binary.
var DefaultMvHpCDF = [2]CDF{AomCDF(128 * 128), AomCDF(128 * 128)}

// DefaultMvBitsCDF — [2 comps][10 bit positions], binary each.
// Used for the MV magnitude tail bits per MV class > 1.
var DefaultMvBitsCDF = [2][10]CDF{
	{
		AomCDF(128 * 136), AomCDF(128 * 140), AomCDF(128 * 148), AomCDF(128 * 160), AomCDF(128 * 176),
		AomCDF(128 * 192), AomCDF(128 * 224), AomCDF(128 * 234), AomCDF(128 * 234), AomCDF(128 * 240),
	},
	{
		AomCDF(128 * 136), AomCDF(128 * 140), AomCDF(128 * 148), AomCDF(128 * 160), AomCDF(128 * 176),
		AomCDF(128 * 192), AomCDF(128 * 224), AomCDF(128 * 234), AomCDF(128 * 234), AomCDF(128 * 240),
	},
}

// DefaultInterpFilterCDF — 16 contexts, 3 symbols (EIGHTTAP_REGULAR /
// EIGHTTAP_SMOOTH / EIGHTTAP_SHARP).
var DefaultInterpFilterCDF = [16]CDF{
	AomCDF(31935, 32720), AomCDF(5568, 32719), AomCDF(422, 2938), AomCDF(28244, 32608),
	AomCDF(31206, 31953), AomCDF(4862, 32121), AomCDF(770, 1152), AomCDF(20889, 25637),
	AomCDF(31910, 32724), AomCDF(4120, 32712), AomCDF(305, 2247), AomCDF(27403, 32636),
	AomCDF(31022, 32009), AomCDF(2963, 32093), AomCDF(601, 943), AomCDF(14969, 21398),
}

// DefaultYModeCDF — inter-frame Y-intra-mode CDF (different from
// DefaultKfYModeCDF). 4 block-size groups × 13 modes.
var DefaultYModeCDF = [4]CDF{
	AomCDF(22801, 23489, 24293, 24756, 25601, 26123, 26606, 27418, 27945, 29228, 29685, 30349),
	AomCDF(18673, 19845, 22631, 23318, 23950, 24649, 25527, 27364, 28152, 29701, 29984, 30852),
	AomCDF(19770, 20979, 23396, 23939, 24241, 24654, 25136, 27073, 27830, 29360, 29730, 30659),
	AomCDF(20155, 21301, 22838, 23178, 23261, 23533, 23703, 24804, 25352, 26575, 27016, 28049),
}
