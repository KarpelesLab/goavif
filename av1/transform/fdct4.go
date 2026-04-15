package transform

// FDCT4 performs an in-place forward 4-point DCT (encoder direction). The
// symmetric counterpart to [IDCT4]; running FDCT4 then IDCT4 reconstructs
// the input within rounding noise of a few ulps.
//
// This is the smallest forward transform in AV1's set and is needed for
// lossless / WHT signaling probes during encoding. The full encoder will
// also need FDCT8/16/32/64 + FADST variants — those land in the encoder
// phase.
func FDCT4(x []int32) {
	if len(x) != 4 {
		panic("transform: FDCT4 requires exactly 4 coefficients")
	}
	// stage 1 — symmetric butterfly
	s0 := x[0] + x[3]
	s3 := x[0] - x[3]
	s1 := x[1] + x[2]
	s2 := x[1] - x[2]

	// stage 2 — apply rotations
	x[0] = halfBtf(cosPi[32], s0, cosPi[32], s1)
	x[2] = halfBtf(cosPi[32], s0, -cosPi[32], s1)
	x[1] = halfBtf(cosPi[48], s3, cosPi[16], s2)
	x[3] = halfBtf(cosPi[16], s3, -cosPi[48], s2)
}
