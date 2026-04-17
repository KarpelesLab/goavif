package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// FilmGrainWriteOpts controls the film grain emission.
type FilmGrainWriteOpts struct {
	// Seed drives the grain LFSR. 0 is valid; use a per-frame value
	// if you want the grain pattern to change.
	Seed uint16
	// LumaStrength in [0, 255]. 0 disables apply_grain. Typical
	// "subtle" grain is 8..32; "heavy" is 48..64.
	LumaStrength uint8
	// Monochrome skips chroma grain emission.
	Monochrome bool
}

// WriteFilmGrainParams emits film_grain_params() suitable for appending
// to a frame body when the sequence header sets
// film_grain_params_present=1. The encoded grain uses a single luma
// scaling point, no AR coefficients (ARCoeffLag=0), and inherits
// chroma scaling from luma to keep the syntax short.
//
// w is the outer frame-header writer; call after the frame body and
// before the final TrailingBits call of the frame header.
func WriteFilmGrainParams(w *bitio.Writer, isInter bool, opts FilmGrainWriteOpts) {
	if opts.LumaStrength == 0 {
		// apply_grain = 0
		w.F(1, 0)
		return
	}
	// apply_grain = 1
	w.F(1, 1)
	// grain_seed (16 bits)
	w.F(16, uint32(opts.Seed))
	// update_grain is coded only for inter frames; implicit true for intra.
	if isInter {
		w.F(1, 1)
	}
	// num_y_points = 1 → one scaling point.
	w.F(4, 1)
	// point_y_value[0] = 128 (mid-range)
	w.F(8, 128)
	// point_y_scaling[0] = LumaStrength
	w.F(8, uint32(opts.LumaStrength))

	if !opts.Monochrome {
		// chroma_scaling_from_luma = 1 → chroma follows luma curve.
		w.F(1, 1)
		// num_cb_points = 0 (no explicit Cb curve; we use from-luma)
		w.F(4, 0)
		// num_cr_points = 0
		w.F(4, 0)
	}

	// grain_scaling_minus_8 (2 bits) = 0 → shift 8.
	w.F(2, 0)
	// ar_coeff_lag (2 bits) = 0 → no AR coefficients.
	w.F(2, 0)
	// NumYPoints > 0 → numPosY = 0 coeffs (ar_lag=0), still nothing emitted.
	// The parser reads ar_coeffs_cb/cr only if chroma_scaling OR num_cb_points>0;
	// with ar_lag=0 and NumYPoints>0, numPosChroma = 0+1 = 1.
	if !opts.Monochrome {
		// ar_coeffs_cb[0] — zero offset (= 128 raw).
		w.F(8, 128)
		// ar_coeffs_cr[0]
		w.F(8, 128)
	}
	// ar_coeff_shift_minus_6 (2 bits) = 0
	w.F(2, 0)
	// grain_scale_shift (2 bits) = 0
	w.F(2, 0)
	// num_cb_points = 0 → no cb_mult/luma_mult/offset.
	// num_cr_points = 0 → no cr_mult/luma_mult/offset.
	// overlap_flag (1 bit) = 0
	w.F(1, 0)
	// clip_to_restricted_range (1 bit) = 0
	w.F(1, 0)
}
