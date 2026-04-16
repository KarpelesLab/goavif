package cdfs

// AomCDF creates a CDF in our wire-format storage from the raw libaom
// AOM_CDF* values. The input values are P(X<=i)*32768 in increasing order
// (as they appear in the C source). The output is the inverted, decreasing
// form P(X>i)*32768 that the spec's decode_symbol loop consumes:
//
//	stored[i] = 32768 - raw[i]     for i in 0..N-2
//	stored[N-1] = 0                (sentinel)
//	stored[N]   = 0                (CDF update counter)
//
// The returned slice has len(raw)+2 entries.
func AomCDF(raw ...uint16) CDF {
	n := len(raw)
	out := make(CDF, n+2)
	for i := 0; i < n; i++ {
		out[i] = 32768 - raw[i]
	}
	out[n] = 0   // sentinel
	out[n+1] = 0 // count
	return out
}
