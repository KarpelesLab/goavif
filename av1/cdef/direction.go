package cdef

// Directions holds the (row_offset, col_offset) vectors that define the 8
// CDEF directions per spec §7.15.3.2. Each direction has two "distances":
// distance 1 (immediate neighbors along the line) and distance 2 (one
// sample further along).
//
// The outer index selects the direction (0..7), the next selects
// distance (0 = d1, 1 = d2), and the innermost is (row, col).
var Directions = [8][2][2]int{
	{{-1, 1}, {-2, 2}}, // 0 — shallow up-right / down-left
	{{0, 1}, {-1, 2}},  // 1
	{{0, 1}, {0, 2}},   // 2 — horizontal
	{{0, 1}, {1, 2}},   // 3
	{{1, 1}, {2, 2}},   // 4 — diagonal
	{{1, 0}, {2, 1}},   // 5
	{{1, 0}, {2, 0}},   // 6 — vertical
	{{1, 0}, {2, -1}},  // 7
}

// divTableFind is the division LUT used by FindDirection, matching
// libaom's div_table ({0, 840/1, 840/2, ..., 840/8}). The spec uses the
// quotient 840/n so every direction cost is comparable without
// re-dividing per line length.
var divTableFind = [9]int32{0, 840, 420, 280, 210, 168, 140, 120, 105}

// FindDirection runs the 8-way direction search on an 8×8 block per
// spec §7.15.3.2 (libaom cdef_find_dir_c). Returns the chosen direction
// (0..7) and the "variance difference" between the best and its
// orthogonal-direction cost, which callers use to decide whether to
// apply the filter at all.
//
// src is a stride-column plane view; (x, y) is the top-left corner of
// the 8×8 block. Samples are expected to be 8-bit (coeff_shift = 0).
func FindDirection(src []uint8, stride, x, y int) (dir int, variance int32) {
	var partial [8][15]int32
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			// Center around 128 to keep squared partial sums inside int32.
			xv := int32(src[(y+i)*stride+(x+j)]) - 128
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
	// Orthogonal direction: dir ^ 4 (180° apart in the 8-way ring).
	orthoCost := cost[dir^4]
	return dir, bestCost - orthoCost
}
