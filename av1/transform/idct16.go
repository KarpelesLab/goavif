package transform

// IDCT16 performs an in-place 16-point inverse DCT (spec §7.7.2.1).
//
// The implementation follows the spec's 7-stage butterfly decomposition:
//
//   stage 1 — bit-reverse permutation
//   stage 2 — outer odd-lane butterflies (8 ops with cos_pi[60,4,28,36,44,20,12,52])
//   stage 3 — inner even-lane butterflies (IDCT8 structure)
//   stage 4 — cross butterflies on the odd lane
//   stage 5 — even-lane final adds / odd-lane pair combinations
//   stage 6 — odd-lane cos_pi[32] crosses
//   stage 7 — final 16-wide add/subtract
func IDCT16(x []int32) {
	if len(x) != 16 {
		panic("transform: IDCT16 requires exactly 16 coefficients")
	}

	// stage 1 — permutation
	s := [16]int32{
		x[0], x[8], x[4], x[12], x[2], x[10], x[6], x[14],
		x[1], x[9], x[5], x[13], x[3], x[11], x[7], x[15],
	}

	// stage 2 — odd-lane outer butterflies
	t8 := halfBtf(cosPi[60], s[8], -cosPi[4], s[15])
	t15 := halfBtf(cosPi[4], s[8], cosPi[60], s[15])
	t9 := halfBtf(cosPi[28], s[9], -cosPi[36], s[14])
	t14 := halfBtf(cosPi[36], s[9], cosPi[28], s[14])
	t10 := halfBtf(cosPi[44], s[10], -cosPi[20], s[13])
	t13 := halfBtf(cosPi[20], s[10], cosPi[44], s[13])
	t11 := halfBtf(cosPi[12], s[11], -cosPi[52], s[12])
	t12 := halfBtf(cosPi[52], s[11], cosPi[12], s[12])

	// stage 3 — even-lane half-butterflies (IDCT4 + IDCT8 inner pair) +
	// odd-lane pair combinations.
	u0 := halfBtf(cosPi[32], s[0], cosPi[32], s[1])
	u1 := halfBtf(cosPi[32], s[0], -cosPi[32], s[1])
	u2 := halfBtf(cosPi[48], s[2], -cosPi[16], s[3])
	u3 := halfBtf(cosPi[16], s[2], cosPi[48], s[3])
	u4 := halfBtf(cosPi[56], s[4], -cosPi[8], s[7])
	u5 := halfBtf(cosPi[24], s[5], -cosPi[40], s[6])
	u6 := halfBtf(cosPi[40], s[5], cosPi[24], s[6])
	u7 := halfBtf(cosPi[8], s[4], cosPi[56], s[7])

	u8 := t8 + t9
	u9 := t8 - t9
	u10 := -t10 + t11
	u11 := t10 + t11
	u12 := t12 + t13
	u13 := t12 - t13
	u14 := -t14 + t15
	u15 := t14 + t15

	// stage 4 — finish IDCT8 structure on even lane + cross on odd lane.
	v0 := u0 + u3
	v1 := u1 + u2
	v2 := u1 - u2
	v3 := u0 - u3
	v4 := u4 + u5
	v5 := u4 - u5
	v6 := -u6 + u7
	v7 := u6 + u7
	v8 := u8
	v9 := halfBtf(-cosPi[16], u9, cosPi[48], u14)
	v10 := halfBtf(-cosPi[48], u10, -cosPi[16], u13)
	v11 := u11
	v12 := u12
	v13 := halfBtf(-cosPi[16], u10, cosPi[48], u13)
	v14 := halfBtf(cosPi[48], u9, cosPi[16], u14)
	v15 := u15

	// stage 5 — complete even-lane IDCT8 sums + combine odd-lane pairs.
	w0 := v0 + v7
	w1 := v1 + v6
	w2 := v2 + v5
	w3 := v3 + v4
	w4 := v3 - v4
	w5 := v2 - v5
	w6 := v1 - v6
	w7 := v0 - v7
	w8 := v8 + v11
	w9 := v9 + v10
	w10 := v9 - v10
	w11 := v8 - v11
	w12 := -v12 + v15
	w13 := -v13 + v14
	w14 := v13 + v14
	w15 := v12 + v15

	// stage 6 — cos_pi[32] cross on odd-lane inners.
	x8 := w8
	x9 := w9
	x10 := halfBtf(-cosPi[32], w10, cosPi[32], w13)
	x11 := halfBtf(-cosPi[32], w11, cosPi[32], w12)
	x12 := halfBtf(cosPi[32], w11, cosPi[32], w12)
	x13 := halfBtf(cosPi[32], w10, cosPi[32], w13)
	x14 := w14
	x15 := w15

	// stage 7 — final 16-wide butterfly.
	x[0] = w0 + x15
	x[1] = w1 + x14
	x[2] = w2 + x13
	x[3] = w3 + x12
	x[4] = w4 + x11
	x[5] = w5 + x10
	x[6] = w6 + x9
	x[7] = w7 + x8
	x[8] = w7 - x8
	x[9] = w6 - x9
	x[10] = w5 - x10
	x[11] = w4 - x11
	x[12] = w3 - x12
	x[13] = w2 - x13
	x[14] = w1 - x14
	x[15] = w0 - x15
}
