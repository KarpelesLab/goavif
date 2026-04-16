package filmgrain

// ApplyAR shapes a generated grain template in-place using an
// auto-regressive filter of the given order (spec §7.20.3.3). The
// filter taps form an L-shaped neighbourhood of order `lag` covering
// all positions (dy, dx) with:
//
//	dy in [-lag, 0], dx in [-lag, lag]
//	AND (dy, dx) precedes (0, 0) in raster order
//	AND NOT (dy == 0 && dx == 0)
//
// coeffs is laid out in the same order the spec transmits them
// (top-to-bottom, left-to-right within each row), taking
// (2*lag+1)*lag + lag entries total. Length must match; otherwise no
// shaping is applied. shift is ar_coeff_shift (6..9) — the final
// weighted sum is rounded-divided by 2^shift before being added to the
// grain sample.
//
// grain is row-major with `cols` samples per row; the caller provides
// initial LFSR noise in positions [0..cols-1] of every row (typically
// [-128, 127]). The filter modifies positions (lag..rows-1, lag..cols-1-lag).
func ApplyAR(grain []int16, cols, rows int, lag int, coeffs []int8, shift uint8) {
	if lag <= 0 {
		return
	}
	taps := (2*lag+1)*lag + lag
	if len(coeffs) != taps {
		return
	}
	if shift == 0 {
		shift = 7
	}
	round := int32(1) << uint(shift-1)

	for r := lag; r < rows; r++ {
		for c := lag; c < cols-lag; c++ {
			sum := int32(0)
			k := 0
			// Rows strictly above (r).
			for dy := -lag; dy < 0; dy++ {
				for dx := -lag; dx <= lag; dx++ {
					sum += int32(coeffs[k]) * int32(grain[(r+dy)*cols+(c+dx)])
					k++
				}
			}
			// Same row, strictly left of (c).
			for dx := -lag; dx < 0; dx++ {
				sum += int32(coeffs[k]) * int32(grain[r*cols+(c+dx)])
				k++
			}
			v := int32(grain[r*cols+c]) + ((sum + round) >> uint(shift))
			if v < -2048 {
				v = -2048
			}
			if v > 2047 {
				v = 2047
			}
			grain[r*cols+c] = int16(v)
		}
	}
}

// GenerateGrainTemplate fills a rows×cols buffer with signed grain
// samples drawn from a seeded LFSR. Output values are in the signed
// 8-bit range [-128, 127], stored in int16 so a subsequent AR pass
// (which runs in wider precision) can accumulate into them.
func GenerateGrainTemplate(cols, rows int, seed uint16) []int16 {
	out := make([]int16, cols*rows)
	if seed == 0 {
		seed = 1
	}
	rng := NewRNG(seed)
	for i := range out {
		out[i] = int16(rng.Byte())
	}
	return out
}
