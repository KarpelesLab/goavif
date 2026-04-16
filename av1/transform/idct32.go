package transform

// IDCT32 performs an in-place 32-point inverse DCT (spec §7.7.2.1).
//
// This is a 9-stage butterfly, transcribed directly from libaom's
// av1_idct32. Our single-buffer implementation alternates between the
// input slice and a local step buffer to mirror the libaom ping-pong
// between bf0 and bf1.
func IDCT32(x []int32) {
	if len(x) != 32 {
		panic("transform: IDCT32 requires exactly 32 coefficients")
	}

	// stage 1 — permutation into the working buffer.
	in := [32]int32{}
	copy(in[:], x)
	out := [32]int32{
		in[0], in[16], in[8], in[24], in[4], in[20], in[12], in[28],
		in[2], in[18], in[10], in[26], in[6], in[22], in[14], in[30],
		in[1], in[17], in[9], in[25], in[5], in[21], in[13], in[29],
		in[3], in[19], in[11], in[27], in[7], in[23], in[15], in[31],
	}

	// stage 2 — rotations on indices 16..31.
	var step [32]int32
	copy(step[:16], out[:16])
	step[16] = halfBtf(cosPi[62], out[16], -cosPi[2], out[31])
	step[17] = halfBtf(cosPi[30], out[17], -cosPi[34], out[30])
	step[18] = halfBtf(cosPi[46], out[18], -cosPi[18], out[29])
	step[19] = halfBtf(cosPi[14], out[19], -cosPi[50], out[28])
	step[20] = halfBtf(cosPi[54], out[20], -cosPi[10], out[27])
	step[21] = halfBtf(cosPi[22], out[21], -cosPi[42], out[26])
	step[22] = halfBtf(cosPi[38], out[22], -cosPi[26], out[25])
	step[23] = halfBtf(cosPi[6], out[23], -cosPi[58], out[24])
	step[24] = halfBtf(cosPi[58], out[23], cosPi[6], out[24])
	step[25] = halfBtf(cosPi[26], out[22], cosPi[38], out[25])
	step[26] = halfBtf(cosPi[42], out[21], cosPi[22], out[26])
	step[27] = halfBtf(cosPi[10], out[20], cosPi[54], out[27])
	step[28] = halfBtf(cosPi[50], out[19], cosPi[14], out[28])
	step[29] = halfBtf(cosPi[18], out[18], cosPi[46], out[29])
	step[30] = halfBtf(cosPi[34], out[17], cosPi[30], out[30])
	step[31] = halfBtf(cosPi[2], out[16], cosPi[62], out[31])

	// stage 3 — rotations on 8..15, adds/subs on 16..31.
	copy(out[:8], step[:8])
	out[8] = halfBtf(cosPi[60], step[8], -cosPi[4], step[15])
	out[9] = halfBtf(cosPi[28], step[9], -cosPi[36], step[14])
	out[10] = halfBtf(cosPi[44], step[10], -cosPi[20], step[13])
	out[11] = halfBtf(cosPi[12], step[11], -cosPi[52], step[12])
	out[12] = halfBtf(cosPi[52], step[11], cosPi[12], step[12])
	out[13] = halfBtf(cosPi[20], step[10], cosPi[44], step[13])
	out[14] = halfBtf(cosPi[36], step[9], cosPi[28], step[14])
	out[15] = halfBtf(cosPi[4], step[8], cosPi[60], step[15])
	out[16] = step[16] + step[17]
	out[17] = step[16] - step[17]
	out[18] = -step[18] + step[19]
	out[19] = step[18] + step[19]
	out[20] = step[20] + step[21]
	out[21] = step[20] - step[21]
	out[22] = -step[22] + step[23]
	out[23] = step[22] + step[23]
	out[24] = step[24] + step[25]
	out[25] = step[24] - step[25]
	out[26] = -step[26] + step[27]
	out[27] = step[26] + step[27]
	out[28] = step[28] + step[29]
	out[29] = step[28] - step[29]
	out[30] = -step[30] + step[31]
	out[31] = step[30] + step[31]

	// stage 4
	copy(step[:4], out[:4])
	step[4] = halfBtf(cosPi[56], out[4], -cosPi[8], out[7])
	step[5] = halfBtf(cosPi[24], out[5], -cosPi[40], out[6])
	step[6] = halfBtf(cosPi[40], out[5], cosPi[24], out[6])
	step[7] = halfBtf(cosPi[8], out[4], cosPi[56], out[7])
	step[8] = out[8] + out[9]
	step[9] = out[8] - out[9]
	step[10] = -out[10] + out[11]
	step[11] = out[10] + out[11]
	step[12] = out[12] + out[13]
	step[13] = out[12] - out[13]
	step[14] = -out[14] + out[15]
	step[15] = out[14] + out[15]
	step[16] = out[16]
	step[17] = halfBtf(-cosPi[8], out[17], cosPi[56], out[30])
	step[18] = halfBtf(-cosPi[56], out[18], -cosPi[8], out[29])
	step[19] = out[19]
	step[20] = out[20]
	step[21] = halfBtf(-cosPi[40], out[21], cosPi[24], out[26])
	step[22] = halfBtf(-cosPi[24], out[22], -cosPi[40], out[25])
	step[23] = out[23]
	step[24] = out[24]
	step[25] = halfBtf(-cosPi[40], out[22], cosPi[24], out[25])
	step[26] = halfBtf(cosPi[24], out[21], cosPi[40], out[26])
	step[27] = out[27]
	step[28] = out[28]
	step[29] = halfBtf(-cosPi[8], out[18], cosPi[56], out[29])
	step[30] = halfBtf(cosPi[56], out[17], cosPi[8], out[30])
	step[31] = out[31]

	// stage 5
	out[0] = halfBtf(cosPi[32], step[0], cosPi[32], step[1])
	out[1] = halfBtf(cosPi[32], step[0], -cosPi[32], step[1])
	out[2] = halfBtf(cosPi[48], step[2], -cosPi[16], step[3])
	out[3] = halfBtf(cosPi[16], step[2], cosPi[48], step[3])
	out[4] = step[4] + step[5]
	out[5] = step[4] - step[5]
	out[6] = -step[6] + step[7]
	out[7] = step[6] + step[7]
	out[8] = step[8]
	out[9] = halfBtf(-cosPi[16], step[9], cosPi[48], step[14])
	out[10] = halfBtf(-cosPi[48], step[10], -cosPi[16], step[13])
	out[11] = step[11]
	out[12] = step[12]
	out[13] = halfBtf(-cosPi[16], step[10], cosPi[48], step[13])
	out[14] = halfBtf(cosPi[48], step[9], cosPi[16], step[14])
	out[15] = step[15]
	out[16] = step[16] + step[19]
	out[17] = step[17] + step[18]
	out[18] = step[17] - step[18]
	out[19] = step[16] - step[19]
	out[20] = -step[20] + step[23]
	out[21] = -step[21] + step[22]
	out[22] = step[21] + step[22]
	out[23] = step[20] + step[23]
	out[24] = step[24] + step[27]
	out[25] = step[25] + step[26]
	out[26] = step[25] - step[26]
	out[27] = step[24] - step[27]
	out[28] = -step[28] + step[31]
	out[29] = -step[29] + step[30]
	out[30] = step[29] + step[30]
	out[31] = step[28] + step[31]

	// stage 6
	step[0] = out[0] + out[3]
	step[1] = out[1] + out[2]
	step[2] = out[1] - out[2]
	step[3] = out[0] - out[3]
	step[4] = out[4]
	step[5] = halfBtf(-cosPi[32], out[5], cosPi[32], out[6])
	step[6] = halfBtf(cosPi[32], out[5], cosPi[32], out[6])
	step[7] = out[7]
	step[8] = out[8] + out[11]
	step[9] = out[9] + out[10]
	step[10] = out[9] - out[10]
	step[11] = out[8] - out[11]
	step[12] = -out[12] + out[15]
	step[13] = -out[13] + out[14]
	step[14] = out[13] + out[14]
	step[15] = out[12] + out[15]
	step[16] = out[16]
	step[17] = out[17]
	step[18] = halfBtf(-cosPi[16], out[18], cosPi[48], out[29])
	step[19] = halfBtf(-cosPi[16], out[19], cosPi[48], out[28])
	step[20] = halfBtf(-cosPi[48], out[20], -cosPi[16], out[27])
	step[21] = halfBtf(-cosPi[48], out[21], -cosPi[16], out[26])
	step[22] = out[22]
	step[23] = out[23]
	step[24] = out[24]
	step[25] = out[25]
	step[26] = halfBtf(-cosPi[16], out[21], cosPi[48], out[26])
	step[27] = halfBtf(-cosPi[16], out[20], cosPi[48], out[27])
	step[28] = halfBtf(cosPi[48], out[19], cosPi[16], out[28])
	step[29] = halfBtf(cosPi[48], out[18], cosPi[16], out[29])
	step[30] = out[30]
	step[31] = out[31]

	// stage 7
	out[0] = step[0] + step[7]
	out[1] = step[1] + step[6]
	out[2] = step[2] + step[5]
	out[3] = step[3] + step[4]
	out[4] = step[3] - step[4]
	out[5] = step[2] - step[5]
	out[6] = step[1] - step[6]
	out[7] = step[0] - step[7]
	out[8] = step[8]
	out[9] = step[9]
	out[10] = halfBtf(-cosPi[32], step[10], cosPi[32], step[13])
	out[11] = halfBtf(-cosPi[32], step[11], cosPi[32], step[12])
	out[12] = halfBtf(cosPi[32], step[11], cosPi[32], step[12])
	out[13] = halfBtf(cosPi[32], step[10], cosPi[32], step[13])
	out[14] = step[14]
	out[15] = step[15]
	out[16] = step[16] + step[23]
	out[17] = step[17] + step[22]
	out[18] = step[18] + step[21]
	out[19] = step[19] + step[20]
	out[20] = step[19] - step[20]
	out[21] = step[18] - step[21]
	out[22] = step[17] - step[22]
	out[23] = step[16] - step[23]
	out[24] = -step[24] + step[31]
	out[25] = -step[25] + step[30]
	out[26] = -step[26] + step[29]
	out[27] = -step[27] + step[28]
	out[28] = step[27] + step[28]
	out[29] = step[26] + step[29]
	out[30] = step[25] + step[30]
	out[31] = step[24] + step[31]

	// stage 8
	step[0] = out[0] + out[15]
	step[1] = out[1] + out[14]
	step[2] = out[2] + out[13]
	step[3] = out[3] + out[12]
	step[4] = out[4] + out[11]
	step[5] = out[5] + out[10]
	step[6] = out[6] + out[9]
	step[7] = out[7] + out[8]
	step[8] = out[7] - out[8]
	step[9] = out[6] - out[9]
	step[10] = out[5] - out[10]
	step[11] = out[4] - out[11]
	step[12] = out[3] - out[12]
	step[13] = out[2] - out[13]
	step[14] = out[1] - out[14]
	step[15] = out[0] - out[15]
	step[16] = out[16]
	step[17] = out[17]
	step[18] = out[18]
	step[19] = out[19]
	step[20] = halfBtf(-cosPi[32], out[20], cosPi[32], out[27])
	step[21] = halfBtf(-cosPi[32], out[21], cosPi[32], out[26])
	step[22] = halfBtf(-cosPi[32], out[22], cosPi[32], out[25])
	step[23] = halfBtf(-cosPi[32], out[23], cosPi[32], out[24])
	step[24] = halfBtf(cosPi[32], out[23], cosPi[32], out[24])
	step[25] = halfBtf(cosPi[32], out[22], cosPi[32], out[25])
	step[26] = halfBtf(cosPi[32], out[21], cosPi[32], out[26])
	step[27] = halfBtf(cosPi[32], out[20], cosPi[32], out[27])
	step[28] = out[28]
	step[29] = out[29]
	step[30] = out[30]
	step[31] = out[31]

	// stage 9 — final butterfly.
	x[0] = step[0] + step[31]
	x[1] = step[1] + step[30]
	x[2] = step[2] + step[29]
	x[3] = step[3] + step[28]
	x[4] = step[4] + step[27]
	x[5] = step[5] + step[26]
	x[6] = step[6] + step[25]
	x[7] = step[7] + step[24]
	x[8] = step[8] + step[23]
	x[9] = step[9] + step[22]
	x[10] = step[10] + step[21]
	x[11] = step[11] + step[20]
	x[12] = step[12] + step[19]
	x[13] = step[13] + step[18]
	x[14] = step[14] + step[17]
	x[15] = step[15] + step[16]
	x[16] = step[15] - step[16]
	x[17] = step[14] - step[17]
	x[18] = step[13] - step[18]
	x[19] = step[12] - step[19]
	x[20] = step[11] - step[20]
	x[21] = step[10] - step[21]
	x[22] = step[9] - step[22]
	x[23] = step[8] - step[23]
	x[24] = step[7] - step[24]
	x[25] = step[6] - step[25]
	x[26] = step[5] - step[26]
	x[27] = step[4] - step[27]
	x[28] = step[3] - step[28]
	x[29] = step[2] - step[29]
	x[30] = step[1] - step[30]
	x[31] = step[0] - step[31]
}
