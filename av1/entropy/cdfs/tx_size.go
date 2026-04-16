package cdfs

// DefaultTxSizeCDF is the default_tx_size_cdf from libaom. Indexed by
// [tx_category][tx_size_context]:
//
//	tx_category 0: max_tx = 8×8   — CDF2 (2 symbols: {current, split})
//	tx_category 1: max_tx = 16×16 — CDF3 (3 symbols: {current, one_split, full_split})
//	tx_category 2: max_tx = 32×32 — CDF3
//	tx_category 3: max_tx = 64×64 — CDF3
//
// tx_size_context is 0..2 (derived from neighbor tx sizes).
var DefaultTxSizeCDF [4][3]CDF

func init() {
	// cat 0: CDF2
	DefaultTxSizeCDF[0][0] = AomCDF(19968)
	DefaultTxSizeCDF[0][1] = AomCDF(19968)
	DefaultTxSizeCDF[0][2] = AomCDF(24320)

	// cat 1: CDF3
	DefaultTxSizeCDF[1][0] = AomCDF(12272, 30172)
	DefaultTxSizeCDF[1][1] = AomCDF(12272, 30172)
	DefaultTxSizeCDF[1][2] = AomCDF(18677, 30848)

	// cat 2: CDF3
	DefaultTxSizeCDF[2][0] = AomCDF(12986, 15180)
	DefaultTxSizeCDF[2][1] = AomCDF(12986, 15180)
	DefaultTxSizeCDF[2][2] = AomCDF(24302, 25602)

	// cat 3: CDF3
	DefaultTxSizeCDF[3][0] = AomCDF(5782, 11475)
	DefaultTxSizeCDF[3][1] = AomCDF(5782, 11475)
	DefaultTxSizeCDF[3][2] = AomCDF(16803, 22759)
}

// DefaultTxfmPartitionCDF is the default_txfm_partition_cdf from libaom.
// 21 entries (TXFM_PARTITION_CONTEXTS), each a 2-symbol CDF for the
// "should this transform block be split" decision.
var DefaultTxfmPartitionCDF [21]CDF

func init() {
	p := [21]uint16{
		28581, 23846, 20847,
		24315, 18196, 12133,
		18791, 10887, 11005,
		27179, 20004, 11281,
		26549, 19308, 14224,
		28015, 21546, 14400,
		28165, 22401, 16088,
	}
	for i, v := range p {
		DefaultTxfmPartitionCDF[i] = AomCDF(v)
	}
}
