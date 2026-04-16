package transform

// IADST16 performs an in-place 16-point inverse ADST (spec §7.7.2.4).
// Structure mirrors libaom's av1_iadst16: a 9-stage butterfly with the
// spec's bit-reverse permutation, rotations at 6 cos_pi index sets, three
// pair add/sub stages, and final permutation with alternating negations.
func IADST16(x []int32) {
	if len(x) != 16 {
		panic("transform: IADST16 requires exactly 16 coefficients")
	}

	// stage 1 — permutation.
	in := [16]int32{}
	copy(in[:], x)
	out := [16]int32{
		in[15], in[0], in[13], in[2], in[11], in[4], in[9], in[6],
		in[7], in[8], in[5], in[10], in[3], in[12], in[1], in[14],
	}

	// stage 2 — rotations.
	var step [16]int32
	step[0] = halfBtf(cosPi[2], out[0], cosPi[62], out[1])
	step[1] = halfBtf(cosPi[62], out[0], -cosPi[2], out[1])
	step[2] = halfBtf(cosPi[10], out[2], cosPi[54], out[3])
	step[3] = halfBtf(cosPi[54], out[2], -cosPi[10], out[3])
	step[4] = halfBtf(cosPi[18], out[4], cosPi[46], out[5])
	step[5] = halfBtf(cosPi[46], out[4], -cosPi[18], out[5])
	step[6] = halfBtf(cosPi[26], out[6], cosPi[38], out[7])
	step[7] = halfBtf(cosPi[38], out[6], -cosPi[26], out[7])
	step[8] = halfBtf(cosPi[34], out[8], cosPi[30], out[9])
	step[9] = halfBtf(cosPi[30], out[8], -cosPi[34], out[9])
	step[10] = halfBtf(cosPi[42], out[10], cosPi[22], out[11])
	step[11] = halfBtf(cosPi[22], out[10], -cosPi[42], out[11])
	step[12] = halfBtf(cosPi[50], out[12], cosPi[14], out[13])
	step[13] = halfBtf(cosPi[14], out[12], -cosPi[50], out[13])
	step[14] = halfBtf(cosPi[58], out[14], cosPi[6], out[15])
	step[15] = halfBtf(cosPi[6], out[14], -cosPi[58], out[15])

	// stage 3 — pair add/sub on halves.
	for i := 0; i < 8; i++ {
		out[i] = step[i] + step[i+8]
		out[i+8] = step[i] - step[i+8]
	}

	// stage 4 — rotations on upper half.
	copy(step[:8], out[:8])
	step[8] = halfBtf(cosPi[8], out[8], cosPi[56], out[9])
	step[9] = halfBtf(cosPi[56], out[8], -cosPi[8], out[9])
	step[10] = halfBtf(cosPi[40], out[10], cosPi[24], out[11])
	step[11] = halfBtf(cosPi[24], out[10], -cosPi[40], out[11])
	step[12] = halfBtf(-cosPi[56], out[12], cosPi[8], out[13])
	step[13] = halfBtf(cosPi[8], out[12], cosPi[56], out[13])
	step[14] = halfBtf(-cosPi[24], out[14], cosPi[40], out[15])
	step[15] = halfBtf(cosPi[40], out[14], cosPi[24], out[15])

	// stage 5 — pair add/sub on quarters.
	for i := 0; i < 4; i++ {
		out[i] = step[i] + step[i+4]
		out[i+4] = step[i] - step[i+4]
		out[i+8] = step[i+8] + step[i+12]
		out[i+12] = step[i+8] - step[i+12]
	}

	// stage 6 — rotations.
	step[0] = out[0]
	step[1] = out[1]
	step[2] = out[2]
	step[3] = out[3]
	step[4] = halfBtf(cosPi[16], out[4], cosPi[48], out[5])
	step[5] = halfBtf(cosPi[48], out[4], -cosPi[16], out[5])
	step[6] = halfBtf(-cosPi[48], out[6], cosPi[16], out[7])
	step[7] = halfBtf(cosPi[16], out[6], cosPi[48], out[7])
	step[8] = out[8]
	step[9] = out[9]
	step[10] = out[10]
	step[11] = out[11]
	step[12] = halfBtf(cosPi[16], out[12], cosPi[48], out[13])
	step[13] = halfBtf(cosPi[48], out[12], -cosPi[16], out[13])
	step[14] = halfBtf(-cosPi[48], out[14], cosPi[16], out[15])
	step[15] = halfBtf(cosPi[16], out[14], cosPi[48], out[15])

	// stage 7 — pair add/sub on eighths.
	for i := 0; i < 2; i++ {
		out[i] = step[i] + step[i+2]
		out[i+2] = step[i] - step[i+2]
		out[i+4] = step[i+4] + step[i+6]
		out[i+6] = step[i+4] - step[i+6]
		out[i+8] = step[i+8] + step[i+10]
		out[i+10] = step[i+8] - step[i+10]
		out[i+12] = step[i+12] + step[i+14]
		out[i+14] = step[i+12] - step[i+14]
	}

	// stage 8 — cos_pi[32] crosses on odd pairs.
	step[0] = out[0]
	step[1] = out[1]
	step[2] = halfBtf(cosPi[32], out[2], cosPi[32], out[3])
	step[3] = halfBtf(cosPi[32], out[2], -cosPi[32], out[3])
	step[4] = out[4]
	step[5] = out[5]
	step[6] = halfBtf(cosPi[32], out[6], cosPi[32], out[7])
	step[7] = halfBtf(cosPi[32], out[6], -cosPi[32], out[7])
	step[8] = out[8]
	step[9] = out[9]
	step[10] = halfBtf(cosPi[32], out[10], cosPi[32], out[11])
	step[11] = halfBtf(cosPi[32], out[10], -cosPi[32], out[11])
	step[12] = out[12]
	step[13] = out[13]
	step[14] = halfBtf(cosPi[32], out[14], cosPi[32], out[15])
	step[15] = halfBtf(cosPi[32], out[14], -cosPi[32], out[15])

	// stage 9 — permutation + alternating negations.
	x[0] = step[0]
	x[1] = -step[8]
	x[2] = step[12]
	x[3] = -step[4]
	x[4] = step[6]
	x[5] = -step[14]
	x[6] = step[10]
	x[7] = -step[2]
	x[8] = step[3]
	x[9] = -step[11]
	x[10] = step[15]
	x[11] = -step[7]
	x[12] = step[5]
	x[13] = -step[13]
	x[14] = step[9]
	x[15] = -step[1]
}

// IFLIPADST16 is IADST16 with output reversed.
func IFLIPADST16(x []int32) {
	IADST16(x)
	for i := 0; i < 8; i++ {
		x[i], x[15-i] = x[15-i], x[i]
	}
}
