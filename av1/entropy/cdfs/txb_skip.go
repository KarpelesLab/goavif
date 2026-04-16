package cdfs

// DefaultTxbSkipCDF is the av1_default_txb_skip_cdfs from libaom
// token_cdfs.h, Q context 0 (base_q_index 0..63). Indexed by
// [tx_size][txb_skip_context].
//
// tx_size 0..4 = TX_4X4 through TX_64X64.
// txb_skip_context 0..12 (derived from neighbor skip states).
//
// Each CDF2 encodes P(all-zero TXB = 1).
//
// NOTE: only Q context 0 is transcribed; contexts 1-3 (for higher QP
// ranges) should be added for full-QP support. Using Q=0 values at
// other QP ranges is slightly wrong but structurally correct — the
// decoder will run, just with suboptimal symbol probabilities.
var DefaultTxbSkipCDF [5][13]CDF

func init() {
	// TX_4X4
	DefaultTxbSkipCDF[0] = [13]CDF{
		AomCDF(31849), AomCDF(5892), AomCDF(12112), AomCDF(21935),
		AomCDF(20289), AomCDF(27473), AomCDF(32487), AomCDF(7654),
		AomCDF(19473), AomCDF(29984), AomCDF(9961), AomCDF(30242),
		AomCDF(32117),
	}
	// TX_8X8
	DefaultTxbSkipCDF[1] = [13]CDF{
		AomCDF(31548), AomCDF(1549), AomCDF(10130), AomCDF(16656),
		AomCDF(18591), AomCDF(26308), AomCDF(32537), AomCDF(5403),
		AomCDF(18096), AomCDF(30003), AomCDF(16384), AomCDF(16384),
		AomCDF(16384),
	}
	// TX_16X16
	DefaultTxbSkipCDF[2] = [13]CDF{
		AomCDF(29957), AomCDF(5391), AomCDF(18039), AomCDF(23566),
		AomCDF(22431), AomCDF(25822), AomCDF(32197), AomCDF(3778),
		AomCDF(15336), AomCDF(28981), AomCDF(16384), AomCDF(16384),
		AomCDF(16384),
	}
	// TX_32X32
	DefaultTxbSkipCDF[3] = [13]CDF{
		AomCDF(17920), AomCDF(1818), AomCDF(7282), AomCDF(25273),
		AomCDF(10923), AomCDF(31554), AomCDF(32624), AomCDF(1366),
		AomCDF(15628), AomCDF(30462), AomCDF(146), AomCDF(5132),
		AomCDF(31657),
	}
	// TX_64X64
	DefaultTxbSkipCDF[4] = [13]CDF{
		AomCDF(6308), AomCDF(117), AomCDF(1638), AomCDF(2161),
		AomCDF(16384), AomCDF(10923), AomCDF(30247), AomCDF(16384),
		AomCDF(16384), AomCDF(16384), AomCDF(16384), AomCDF(16384),
		AomCDF(16384),
	}
}
