package transform

// FDCT4 performs an in-place forward 4-point DCT (encoder direction). The
// symmetric counterpart to [IDCT4]; running FDCT4 then IDCT4 reconstructs
// the input within rounding noise of a few ulps.
func FDCT4(x []int32) {
	if len(x) != 4 {
		panic("transform: FDCT4 requires exactly 4 coefficients")
	}
	fdctMatrixInverse(x, IDCT4, 4)
}
