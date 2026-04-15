package cdfs

// DefaultSkipCDF is default_skip_txfm_cdf from libaom
// av1/common/entropymode.c — the probability of skip for each of the
// three skip contexts (0 = neighbors non-skip, 1 = one neighbor skip,
// 2 = both neighbors skip).
//
// Values are the spec's Q15 probabilities of symbol 0 (skip=0).
var DefaultSkipCDF = [3]CDF{
	// context 0: neither neighbor is skip — skip is unusual
	{32768 - 31671, 0, 0},
	// context 1: one neighbor is skip
	{32768 - 13599, 0, 0},
	// context 2: both neighbors are skip — skip is common
	{32768 - 4576, 0, 0},
}
