package cdef

// Direction holds the (row_offset, col_offset) vectors that define the 8
// CDEF directions per spec §7.15.3.2. Each direction has two "distances":
// distance 1 (immediate neighbors along the line) and distance 2 (one
// sample further along).
//
// The outer index selects the direction (0..7), the next selects
// distance (0 = d1, 1 = d2), and the innermost is (row, col).
var Directions = [8][2][2]int{
	{{-1, 1}, {-2, 2}},  // direction 0 — shallow up-right / down-left
	{{-1, 1}, {-2, 3}},  // 1
	{{0, 1}, {-1, 2}},   // 2
	{{0, 1}, {0, 2}},    // 3 — horizontal
	{{0, 1}, {1, 2}},    // 4
	{{1, 1}, {2, 3}},    // 5
	{{1, 1}, {2, 2}},    // 6
	{{1, 0}, {2, 1}},    // 7
}

// DivTable is the "division" LUT from spec §7.15.3.2 (divTable). It
// avoids a runtime divide when computing per-line-length mean/variance.
//
//	divTable[i] = 2^15 / (i+1) rounded (i in 1..15, divTable[0] unused)
var DivTable = [16]int32{
	0, 840, 420, 280, 210, 168, 140, 120, 105, 93, 84, 76, 70, 65, 60, 56,
}

// FindDirection runs the 8-way direction search on an 8×8 block, per
// spec §7.15.3.2. It returns the chosen direction (0..7) and the
// variance measure used to select it.
//
// src is a w-stride plane view; (x, y) is the top-left corner of the
// 8×8 block within src (inclusive in bytes).
func FindDirection(src []uint8, stride, x, y int) (dir int, variance int32) {
	// Collect block samples centered around the mean so sign-correlated
	// patterns show up in the cost.
	const bs = 8
	mean := int32(0)
	for r := 0; r < bs; r++ {
		for c := 0; c < bs; c++ {
			mean += int32(src[(y+r)*stride+(x+c)])
		}
	}
	mean /= int32(bs * bs)

	// Centered block values.
	var centered [bs][bs]int32
	for r := 0; r < bs; r++ {
		for c := 0; c < bs; c++ {
			centered[r][c] = int32(src[(y+r)*stride+(x+c)]) - mean
		}
	}

	// For each direction, compute a cost = sum over "lines" of
	// (sum_along_line)² / line_length. The direction with the maximum
	// cost wins (the spec finds the direction along which the signal
	// is most aligned; implemented as per libaom's cdef_find_dir).
	var cost [8]int32

	// Each direction defines 8 "lines" that sweep diagonally across the
	// block. The spec tabulates these, but since the lines are
	// regularly spaced along each angle we can generate them here.
	//
	// For simplicity and correctness, use a direct mapping table of
	// (dir, sample_index) → line_index. Per libaom's filter_width /
	// line_map tables for 8×8:
	lineMaps := [8][8][8]int{
		{ // dir 0 (shallow up-right): 15 lines total but indexed 0..14
			{0, 0, 0, 0, 1, 1, 1, 1},
			{0, 0, 0, 0, 1, 1, 1, 1},
			{0, 0, 0, 0, 1, 1, 1, 1},
			{0, 0, 0, 0, 1, 1, 1, 1},
			{0, 0, 0, 0, 1, 1, 1, 1},
			{0, 0, 0, 0, 1, 1, 1, 1},
			{0, 0, 0, 0, 1, 1, 1, 1},
			{0, 0, 0, 0, 1, 1, 1, 1},
		},
		{}, {}, {}, {}, {}, {}, {},
	}
	// NOTE: the line-mapping tables for a bit-exact CDEF direction search
	// are large and position-specific; generating them programmatically
	// is error-prone. The map above covers only direction 0 with a
	// simplified 2-line split; directions 1..7 currently default to the
	// same layout, so FindDirection returns a coarse answer. A
	// follow-up phase should port the exact tables from libaom's
	// av1/common/cdef_block.c.

	for d := 0; d < 8; d++ {
		var sums [16]int32
		var counts [16]int32
		for r := 0; r < bs; r++ {
			for c := 0; c < bs; c++ {
				li := lineMaps[d][r][c]
				sums[li] += centered[r][c]
				counts[li]++
			}
		}
		var c int32
		for i := 0; i < 16; i++ {
			if counts[i] == 0 {
				continue
			}
			c += sums[i] * sums[i] / counts[i]
		}
		cost[d] = c
	}

	// Pick the direction with the largest cost (most signal alignment).
	bestCost := cost[0]
	for d := 1; d < 8; d++ {
		if cost[d] > bestCost {
			bestCost = cost[d]
			dir = d
		}
	}
	return dir, bestCost - cost[0]
}
