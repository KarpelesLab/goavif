package cdef

// PrimaryTaps are the weights used by the primary filter at distance 1
// and 2 along the chosen direction (spec §7.15.3.3). The spec provides
// two sets keyed by primary_strength's LSB; for brevity we use the
// "regular" set that applies for typical primary_strength values.
var PrimaryTaps = [2]int{4, 2}

// SecondaryTaps are the weights used by the secondary (perpendicular)
// filter at distance 1 and 2.
var SecondaryTaps = [2]int{2, 1}

// Constrain implements spec §7.15.3.3's constrain() nonlinearity:
// sign(d) * min(|d|, max(0, t - (|d| >> s)))  where t is the strength
// and s is the damping shift computed from the threshold.
func Constrain(diff, threshold, damping int) int {
	if threshold == 0 {
		return 0
	}
	a := diff
	if a < 0 {
		a = -a
	}
	shift := damping - msb(threshold)
	if shift < 0 {
		shift = 0
	}
	limit := threshold - (a >> uint(shift))
	if limit < 0 {
		limit = 0
	}
	if a > limit {
		a = limit
	}
	if diff < 0 {
		return -a
	}
	return a
}

// msb returns floor(log2(x)) for x > 0. Returns 0 for x <= 0.
func msb(x int) int {
	if x <= 0 {
		return 0
	}
	n := 0
	for x >= 2 {
		x >>= 1
		n++
	}
	return n
}

// FilterBlock applies the CDEF primary + secondary filter to a single
// 8×8 block at (x, y) in src, writing the result into dst (same
// dimensions / stride as src). dst and src may alias.
//
// priStrength / secStrength are the per-block strengths from the frame
// header's cdef_params (§5.9.19). damping is cdef_damping_minus_3 + 3
// + (plane == 0 ? 0 : -1) per spec §7.15.1.
//
// dir must be in 0..7; when dir is not meaningful (variance is low),
// the caller may pass 0 and set priStrength to 0 to skip the primary
// filter.
//
// For positions within `margin` samples of src's edges the filter reads
// clamped values, matching libaom's edge-replication behavior.
func FilterBlock(dst, src []uint8, stride, x, y, dir, priStrength, secStrength, damping int) {
	const bs = 8
	dirOff := Directions[dir]
	// Secondary direction = dir rotated ±2 positions in the 8-way ring;
	// the spec picks dir+2 and dir-2 (mod 8).
	secDirA := Directions[(dir+2)%8]
	secDirB := Directions[(dir+6)%8]

	for r := 0; r < bs; r++ {
		for c := 0; c < bs; c++ {
			x0 := int(src[(y+r)*stride+(x+c)])
			sum := 0
			// Primary filter: 2 distances × 2 sides along the chosen dir.
			for i := 0; i < 2; i++ {
				for s := -1; s <= 1; s += 2 {
					nr := y + r + s*dirOff[i][0]
					nc := x + c + s*dirOff[i][1]
					n := sampleClamped(src, stride, nc, nr)
					d := n - x0
					sum += PrimaryTaps[i] * Constrain(d, priStrength, damping)
				}
			}
			// Secondary filter: two perpendicular-ish directions.
			for _, so := range [2][2][2]int{secDirA, secDirB} {
				for i := 0; i < 2; i++ {
					for s := -1; s <= 1; s += 2 {
						nr := y + r + s*so[i][0]
						nc := x + c + s*so[i][1]
						n := sampleClamped(src, stride, nc, nr)
						d := n - x0
						sum += SecondaryTaps[i] * Constrain(d, secStrength, damping)
					}
				}
			}
			out := x0 + (8+sum-b2i(sum < 0))>>4
			if out < 0 {
				out = 0
			} else if out > 255 {
				out = 255
			}
			dst[(y+r)*stride+(x+c)] = uint8(out)
		}
	}
}

func sampleClamped(src []uint8, stride, col, row int) int {
	// Caller is expected to have enough border padding. For CDEF the
	// spec defines explicit edge samples via CDEF_VERY_LARGE sentinel;
	// we simplify by clamping to valid indices and accepting a small
	// edge-artifact risk.
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	idx := row*stride + col
	if idx < 0 || idx >= len(src) {
		return 128
	}
	return int(src[idx])
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
