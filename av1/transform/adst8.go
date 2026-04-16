package transform

// IADST8 performs an in-place 8-point inverse ADST per AV1 spec §7.7.2.4.
// Structure mirrors libaom's av1_iadst8: 7-stage butterfly using cos_pi
// constants with specific bit-reverse permutation and final negations.
func IADST8(x []int32) {
	if len(x) != 8 {
		panic("transform: IADST8 requires exactly 8 coefficients")
	}

	// stage 1 — permutation.
	s := [8]int32{x[7], x[0], x[5], x[2], x[3], x[4], x[1], x[6]}

	// stage 2 — rotations.
	t := [8]int32{
		halfBtf(cosPi[4], s[0], cosPi[60], s[1]),
		halfBtf(cosPi[60], s[0], -cosPi[4], s[1]),
		halfBtf(cosPi[20], s[2], cosPi[44], s[3]),
		halfBtf(cosPi[44], s[2], -cosPi[20], s[3]),
		halfBtf(cosPi[36], s[4], cosPi[28], s[5]),
		halfBtf(cosPi[28], s[4], -cosPi[36], s[5]),
		halfBtf(cosPi[52], s[6], cosPi[12], s[7]),
		halfBtf(cosPi[12], s[6], -cosPi[52], s[7]),
	}

	// stage 3 — pair add/subtract.
	u := [8]int32{
		t[0] + t[4], t[1] + t[5], t[2] + t[6], t[3] + t[7],
		t[0] - t[4], t[1] - t[5], t[2] - t[6], t[3] - t[7],
	}

	// stage 4 — selective rotation on u[4..7].
	v := [8]int32{
		u[0], u[1], u[2], u[3],
		halfBtf(cosPi[16], u[4], cosPi[48], u[5]),
		halfBtf(cosPi[48], u[4], -cosPi[16], u[5]),
		halfBtf(-cosPi[48], u[6], cosPi[16], u[7]),
		halfBtf(cosPi[16], u[6], cosPi[48], u[7]),
	}

	// stage 5 — pair add/subtract.
	w := [8]int32{
		v[0] + v[2], v[1] + v[3], v[0] - v[2], v[1] - v[3],
		v[4] + v[6], v[5] + v[7], v[4] - v[6], v[5] - v[7],
	}

	// stage 6 — rotations on w[2..3] and w[6..7].
	y := [8]int32{
		w[0], w[1],
		halfBtf(cosPi[32], w[2], cosPi[32], w[3]),
		halfBtf(cosPi[32], w[2], -cosPi[32], w[3]),
		w[4], w[5],
		halfBtf(cosPi[32], w[6], cosPi[32], w[7]),
		halfBtf(cosPi[32], w[6], -cosPi[32], w[7]),
	}

	// stage 7 — permutation + alternating negations.
	x[0] = y[0]
	x[1] = -y[4]
	x[2] = y[6]
	x[3] = -y[2]
	x[4] = y[3]
	x[5] = -y[7]
	x[6] = y[5]
	x[7] = -y[1]
}

// IFLIPADST8 is IADST8 with output reversed.
func IFLIPADST8(x []int32) {
	IADST8(x)
	x[0], x[7] = x[7], x[0]
	x[1], x[6] = x[6], x[1]
	x[2], x[5] = x[5], x[2]
	x[3], x[4] = x[4], x[3]
}
