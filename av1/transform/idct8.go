package transform

// IDCT8 performs an in-place 8-point inverse DCT (spec §7.7.2.1).
//
// The implementation follows the 5-stage butterfly layout defined by the
// spec: bit-reverse permutation, half-butterflies using cos_pi constants,
// recursive IDCT4 on the even lane, and the final add/subtract cross.
func IDCT8(x []int32) {
	if len(x) != 8 {
		panic("transform: IDCT8 requires exactly 8 coefficients")
	}
	// stage 1: reordered taps
	s0 := x[0]
	s1 := x[4]
	s2 := x[2]
	s3 := x[6]
	s4 := x[1]
	s5 := x[5]
	s6 := x[3]
	s7 := x[7]

	// stage 2
	t4 := halfBtf(cosPi[56], s4, -cosPi[8], s7)
	t5 := halfBtf(cosPi[24], s5, -cosPi[40], s6)
	t6 := halfBtf(cosPi[40], s5, cosPi[24], s6)
	t7 := halfBtf(cosPi[8], s4, cosPi[56], s7)

	// stage 3 — IDCT4 on the even lane.
	u0 := halfBtf(cosPi[32], s0, cosPi[32], s1)
	u1 := halfBtf(cosPi[32], s0, -cosPi[32], s1)
	u2 := halfBtf(cosPi[48], s2, -cosPi[16], s3)
	u3 := halfBtf(cosPi[16], s2, cosPi[48], s3)
	u4 := t4 + t5
	u5 := t4 - t5
	u6 := -t6 + t7
	u7 := t6 + t7

	// stage 4 — finish IDCT4 cross, mix u5/u6.
	v0 := u0 + u3
	v1 := u1 + u2
	v2 := u1 - u2
	v3 := u0 - u3
	v4 := u4
	v5 := halfBtf(-cosPi[32], u5, cosPi[32], u6)
	v6 := halfBtf(cosPi[32], u5, cosPi[32], u6)
	v7 := u7

	// stage 5 — final butterfly.
	x[0] = v0 + v7
	x[1] = v1 + v6
	x[2] = v2 + v5
	x[3] = v3 + v4
	x[4] = v3 - v4
	x[5] = v2 - v5
	x[6] = v1 - v6
	x[7] = v0 - v7
}
