package cdef

// FindDirection16 is the uint16 counterpart of [FindDirection]. It
// centers samples around the bit-depth midpoint so the squared partial
// sums stay inside int32 even at 10/12-bit.
func FindDirection16(src []uint16, stride, x, y, bitDepth int) (dir int, variance int32) {
	var partial [8][15]int32
	mid := int32(1) << uint(bitDepth-1)
	// Right-shift samples down to 8-bit range so the squared partial
	// sums match the uint8 path's dynamic range (spec §7.15.3.2 uses
	// coeff_shift = bitDepth - 8 for HBD).
	shift := uint(bitDepth - 8)
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			raw := int32(src[(y+i)*stride+(x+j)])
			xv := (raw - mid) >> shift
			partial[0][i+j] += xv
			partial[1][i+j/2] += xv
			partial[2][i] += xv
			partial[3][3+i-j/2] += xv
			partial[4][7+i-j] += xv
			partial[5][3-i/2+j] += xv
			partial[6][j] += xv
			partial[7][i/2+j] += xv
		}
	}

	var cost [8]int32
	for i := 0; i < 8; i++ {
		cost[2] += partial[2][i] * partial[2][i]
		cost[6] += partial[6][i] * partial[6][i]
	}
	cost[2] *= divTableFind[8]
	cost[6] *= divTableFind[8]

	for i := 0; i < 7; i++ {
		cost[0] += (partial[0][i]*partial[0][i] +
			partial[0][14-i]*partial[0][14-i]) * divTableFind[i+1]
		cost[4] += (partial[4][i]*partial[4][i] +
			partial[4][14-i]*partial[4][14-i]) * divTableFind[i+1]
	}
	cost[0] += partial[0][7] * partial[0][7] * divTableFind[8]
	cost[4] += partial[4][7] * partial[4][7] * divTableFind[8]

	for i := 1; i < 8; i += 2 {
		for j := 0; j < 5; j++ {
			cost[i] += partial[i][3+j] * partial[i][3+j]
		}
		cost[i] *= divTableFind[8]
		for j := 0; j < 3; j++ {
			cost[i] += (partial[i][j]*partial[i][j] +
				partial[i][10-j]*partial[i][10-j]) * divTableFind[2*j+2]
		}
	}

	bestCost := cost[0]
	for i := 1; i < 8; i++ {
		if cost[i] > bestCost {
			bestCost = cost[i]
			dir = i
		}
	}
	orthoCost := cost[dir^4]
	return dir, bestCost - orthoCost
}
