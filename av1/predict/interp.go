package predict

// AV1 sub-pel interpolation filters (spec §7.11.3.4). Each filter set
// is 16 phases × 8 taps. A block is resampled by applying the
// horizontal filter followed by the vertical filter. Motion vectors
// carry eighth-pel precision; the phase index is (mv & 15).

// InterpFilter enumerates the three filter sets actually used by
// AVIF content. The 4th libaom filter (BILINEAR) is not coded in
// the AVIF bitstream.
type InterpFilter uint8

const (
	InterpRegular InterpFilter = 0
	InterpSmooth  InterpFilter = 1
	InterpSharp   InterpFilter = 2
)

// Filter coefficients are stored as int16 because AV1's filter
// weights reach up to 128 (the all-tap at integer phase) and down
// to small negative values. int16 leaves plenty of headroom without
// sign-range surprises.

// EightTapRegular: libaom's `av1_sub_pel_filters_8` default.
var EightTapRegular = [16][8]int16{
	{0, 0, 0, 128, 0, 0, 0, 0},
	{0, 2, -6, 126, 8, -2, 0, 0},
	{0, 2, -10, 122, 18, -4, 0, 0},
	{0, 2, -12, 116, 28, -8, 2, 0},
	{0, 2, -14, 110, 38, -10, 2, 0},
	{0, 2, -14, 102, 48, -12, 2, 0},
	{0, 2, -16, 94, 58, -12, 2, 0},
	{0, 2, -14, 84, 66, -12, 2, 0},
	{0, 2, -14, 76, 76, -14, 2, 0},
	{0, 2, -12, 66, 84, -14, 2, 0},
	{0, 2, -12, 58, 94, -16, 2, 0},
	{0, 2, -12, 48, 102, -14, 2, 0},
	{0, 2, -10, 38, 110, -14, 2, 0},
	{0, 2, -8, 28, 116, -12, 2, 0},
	{0, 0, -4, 18, 122, -10, 2, 0},
	{0, 0, -2, 8, 126, -6, 2, 0},
}

// EightTapSmooth: libaom's `av1_sub_pel_filters_8smooth`.
var EightTapSmooth = [16][8]int16{
	{0, 0, 0, 128, 0, 0, 0, 0},
	{0, 2, 28, 62, 34, 2, 0, 0},
	{0, 0, 26, 62, 36, 4, 0, 0},
	{0, 0, 22, 62, 40, 4, 0, 0},
	{0, 0, 20, 60, 42, 6, 0, 0},
	{0, 0, 18, 58, 44, 8, 0, 0},
	{0, 0, 16, 56, 46, 10, 0, 0},
	{0, -2, 16, 54, 48, 12, 0, 0},
	{0, -2, 14, 52, 52, 14, -2, 0},
	{0, 0, 12, 48, 54, 16, -2, 0},
	{0, 0, 10, 46, 56, 16, 0, 0},
	{0, 0, 8, 44, 58, 18, 0, 0},
	{0, 0, 6, 42, 60, 20, 0, 0},
	{0, 0, 4, 40, 62, 22, 0, 0},
	{0, 0, 4, 36, 62, 26, 0, 0},
	{0, 0, 2, 34, 62, 28, 2, 0},
}

// EightTapSharp: libaom's `av1_sub_pel_filters_8sharp`.
var EightTapSharp = [16][8]int16{
	{0, 0, 0, 128, 0, 0, 0, 0},
	{-2, 2, -6, 126, 8, -2, 2, 0},
	{-2, 6, -12, 124, 16, -6, 4, -2},
	{-2, 8, -18, 120, 26, -10, 6, -2},
	{-4, 10, -22, 116, 38, -14, 6, -2},
	{-4, 10, -22, 108, 48, -18, 8, -2},
	{-4, 10, -24, 100, 60, -20, 8, -2},
	{-4, 10, -24, 90, 70, -22, 10, -2},
	{-4, 12, -24, 80, 80, -24, 12, -4},
	{-2, 10, -22, 70, 90, -24, 10, -4},
	{-2, 8, -20, 60, 100, -24, 10, -4},
	{-2, 8, -18, 48, 108, -22, 10, -4},
	{-2, 6, -14, 38, 116, -22, 10, -4},
	{-2, 6, -10, 26, 120, -18, 8, -2},
	{-2, 4, -6, 16, 124, -12, 6, -2},
	{0, 2, -2, 8, 126, -6, 2, -2},
}

// InterpSubPel applies an 8-tap horizontal + vertical interpolation
// to produce an w×h output block from a reference area of size
// (w+7)×(h+7) samples. filt picks the filter set; hp / vp are the
// horizontal / vertical 1/16-phase indices (mv & 15).
//
// src is a flat w+7 by h+7 byte region indexed as src[row*srcStride+col];
// dst is w*h bytes in row-major order. The caller is expected to have
// extracted src from the reference frame with the appropriate corner
// integer-pel offset.
func InterpSubPel(dst []uint8, w, h int, src []uint8, srcStride, hp, vp int, filt InterpFilter) {
	hFilter := filtTable(filt)[hp]
	vFilter := filtTable(filt)[vp]
	// Horizontal pass into a temp row-buffer of size w × (h+7) ints.
	tmp := make([]int32, w*(h+7))
	for r := 0; r < h+7; r++ {
		for c := 0; c < w; c++ {
			sum := int32(0)
			for k := 0; k < 8; k++ {
				sum += int32(hFilter[k]) * int32(src[r*srcStride+c+k])
			}
			tmp[r*w+c] = sum
		}
	}
	// Vertical pass: combine 8 tmp rows into 1 output pixel.
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			sum := int32(0)
			for k := 0; k < 8; k++ {
				sum += int32(vFilter[k]) * tmp[(r+k)*w+c]
			}
			// Total scale is 128×128 = 16384; round and clip.
			v := (sum + (1 << 13)) >> 14
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			dst[r*w+c] = uint8(v)
		}
	}
}

// filtTable returns the filter coefficient table for one set.
func filtTable(f InterpFilter) *[16][8]int16 {
	switch f {
	case InterpSmooth:
		return &EightTapSmooth
	case InterpSharp:
		return &EightTapSharp
	}
	return &EightTapRegular
}

// InterpInteger copies a w×h block from src at integer-pel offset.
// Use this when both mv sub-pel components are zero (hp == vp == 0).
func InterpInteger(dst []uint8, w, h int, src []uint8, srcStride int) {
	for r := 0; r < h; r++ {
		copy(dst[r*w:r*w+w], src[r*srcStride:r*srcStride+w])
	}
}
