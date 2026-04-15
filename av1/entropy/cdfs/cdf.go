package cdfs

import "fmt"

// CDF is a (N+1)-entry cumulative distribution function for an N-symbol
// alphabet, in the AV1 wire format. See the package doc for the layout.
type CDF []uint16

// Validate returns an error if the CDF is ill-formed: wrong length,
// non-monotonic decreasing probabilities, or a non-zero sentinel.
func (c CDF) Validate() error {
	if len(c) < 2 {
		return fmt.Errorf("cdfs: CDF length %d too short", len(c))
	}
	N := len(c) - 1
	if c[N-1] != 0 {
		return fmt.Errorf("cdfs: CDF sentinel cdf[%d] = %d, want 0", N-1, c[N-1])
	}
	for i := 0; i < N-1; i++ {
		if c[i] <= c[i+1] {
			return fmt.Errorf("cdfs: CDF not strictly decreasing at %d: %d <= %d", i, c[i], c[i+1])
		}
	}
	// cdf[0] must be < 32768 (a positive probability remains for the final symbol).
	if c[0] >= 32768 {
		return fmt.Errorf("cdfs: CDF starts at %d, must be < 32768", c[0])
	}
	return nil
}

// MakeCDF2 returns a 2-symbol CDF from P(X=0) expressed as Q15 (0..32768).
// Equivalent to libaom's AOM_CDF2(p0) macro followed by the zero count.
func MakeCDF2(p0 uint16) CDF {
	return CDF{32768 - p0, 0, 0}
}

// MakeCDF3 builds a 3-symbol CDF from per-symbol probabilities.
// p0+p1+p2 must equal 32768.
func MakeCDF3(p0, p1 uint16) CDF {
	return CDF{32768 - p0, 32768 - p0 - p1, 0, 0}
}

// MakeCDF4 builds a 4-symbol CDF; p0..p2 summing to <=32768 give the
// cumulative frontier; the final symbol gets the remaining mass.
func MakeCDF4(p0, p1, p2 uint16) CDF {
	return CDF{
		32768 - p0,
		32768 - p0 - p1,
		32768 - p0 - p1 - p2,
		0,
		0,
	}
}
