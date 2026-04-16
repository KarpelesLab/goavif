package predict

// Directional intra prediction (AV1 spec §7.11.2.5).
//
// For each of 6 base angles (45, 67, 113, 135, 157, 203 degrees) plus an
// angle_delta in {-3..3} at 3° steps, a per-sample projection is formed
// along the chosen angle and sampled from the neighbor (above and/or
// left) via sub-pixel interpolation.

// dr_intra_derivative from libaom reconintra.h. Indexed by an offset
// derived from the angle; entries that aren't on 3° increments are 0.
// The (256 / tan(angle)) values are scaled so dx/dy are Q6 fixed-point.
var drIntraDerivative = [90]int16{
	0, 0, 0,
	1023, 0, 0, // 3
	547, 0, 0, // 6
	372, 0, 0, 0, 0, // 9
	273, 0, 0, // 14
	215, 0, 0, // 17
	178, 0, 0, // 20
	151, 0, 0, // 23
	132, 0, 0, // 26
	116, 0, 0, // 29
	102, 0, 0, 0, // 32
	90, 0, 0, // 36
	80, 0, 0, // 39
	71, 0, 0, // 42
	64, 0, 0, // 45
	57, 0, 0, // 48
	51, 0, 0, // 51
	45, 0, 0, 0, // 54
	40, 0, 0, // 58
	35, 0, 0, // 61
	31, 0, 0, // 64
	27, 0, 0, // 67
	23, 0, 0, // 70
	19, 0, 0, // 73
	15, 0, 0, 0, 0, // 76
	11, 0, 0, // 81
	7, 0, 0, // 84
	3, 0, 0, // 87
}

// ModeToAngleMap returns the base angle (degrees) for each intra mode
// index; non-directional modes return 0. Matches libaom's
// mode_to_angle_map[INTRA_MODES].
var ModeToAngleMap = [13]int{
	0,   // DC_PRED
	90,  // V_PRED
	180, // H_PRED
	45,  // D45_PRED
	135, // D135_PRED
	113, // D113_PRED
	157, // D157_PRED
	203, // D203_PRED
	67,  // D67_PRED
	0, 0, 0, 0, // SMOOTH*, PAETH
}

// getDx returns the horizontal per-row step in Q6 (256/t where t is tan
// of the angle). Only valid for 0 < angle < 180.
func getDx(angle int) int {
	if angle > 0 && angle < 90 {
		return int(drIntraDerivative[angle])
	}
	if angle > 90 && angle < 180 {
		return int(drIntraDerivative[180-angle])
	}
	return 1
}

// getDy returns the vertical per-column step in Q6. Valid for 90 < angle
// < 270.
func getDy(angle int) int {
	if angle > 90 && angle < 180 {
		return int(drIntraDerivative[angle-90])
	}
	if angle > 180 && angle < 270 {
		return int(drIntraDerivative[270-angle])
	}
	return 1
}

// DirectionalPred fills dst (w×h, row-major) with the AV1 directional
// prediction for the given angle in degrees. The above and left
// neighbor arrays must be extended to cover the full projection:
//
//	above: at least (w + h + max_ext) samples
//	left:  at least (h + w + max_ext) samples
//
// For now callers need to zero-extend or replicate edge samples; this
// implementation clamps out-of-range reads to the last available sample.
func DirectionalPred(dst []uint8, w, h int, above, left []uint8, angle int) {
	switch {
	case angle > 0 && angle < 90:
		drPredAboveZone(dst, w, h, above, getDx(angle))
	case angle > 180 && angle < 270:
		drPredLeftZone(dst, w, h, left, getDy(angle))
	case angle > 90 && angle < 180:
		drPredMixedZone(dst, w, h, above, left, getDx(angle), getDy(angle))
	case angle == 90:
		// Vertical — same as V_PRED on the above row.
		for r := 0; r < h; r++ {
			for c := 0; c < w; c++ {
				dst[r*w+c] = above[c]
			}
		}
	case angle == 180:
		// Horizontal — same as H_PRED on the left column.
		for r := 0; r < h; r++ {
			v := left[r]
			for c := 0; c < w; c++ {
				dst[r*w+c] = v
			}
		}
	}
}

// drPredAboveZone handles zone 1 angles (45..89°) — each sample reads
// from the above row at an offset that grows by dx per row.
func drPredAboveZone(dst []uint8, w, h int, above []uint8, dx int) {
	maxIdx := len(above) - 1
	for r := 0; r < h; r++ {
		offset := (r + 1) * dx
		for c := 0; c < w; c++ {
			base := c + (offset >> 6)
			shift := (offset >> 1) & 0x1F
			b1 := base
			b2 := base + 1
			if b1 > maxIdx {
				b1 = maxIdx
			}
			if b2 > maxIdx {
				b2 = maxIdx
			}
			v := (int(above[b1])*(32-shift) + int(above[b2])*shift + 16) >> 5
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			dst[r*w+c] = uint8(v)
		}
	}
}

// drPredLeftZone handles zone 3 angles (181..269°) — reads from the
// left column with per-column dy growth.
func drPredLeftZone(dst []uint8, w, h int, left []uint8, dy int) {
	maxIdx := len(left) - 1
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			offset := (c + 1) * dy
			base := r + (offset >> 6)
			shift := (offset >> 1) & 0x1F
			b1 := base
			b2 := base + 1
			if b1 > maxIdx {
				b1 = maxIdx
			}
			if b2 > maxIdx {
				b2 = maxIdx
			}
			v := (int(left[b1])*(32-shift) + int(left[b2])*shift + 16) >> 5
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			dst[r*w+c] = uint8(v)
		}
	}
}

// drPredMixedZone handles zone 2 angles (91..179°) — the projection
// line may cross either the top row or the left column depending on
// the sample position. Each position chooses the source that the
// backward ray hits first.
func drPredMixedZone(dst []uint8, w, h int, above, left []uint8, dx, dy int) {
	maxA := len(above) - 1
	maxL := len(left) - 1
	// For zone 2 the spec uses an "inverse" dy to project onto the left.
	invDy := dy
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			// Project the ray backwards from (r, c) at the given angle.
			// The sample source depends on which of the top or left
			// edges the ray hits first.
			xOff := -(r+1)*dx + (c+1)*64
			yOff := -(c+1)*invDy + (r+1)*64
			if xOff >= 0 {
				// Ray exits the block through the top — sample above.
				base := xOff >> 6
				shift := (xOff >> 1) & 0x1F
				b1 := base
				b2 := base + 1
				if b1 > maxA {
					b1 = maxA
				}
				if b2 > maxA {
					b2 = maxA
				}
				v := (int(above[b1])*(32-shift) + int(above[b2])*shift + 16) >> 5
				if v < 0 {
					v = 0
				} else if v > 255 {
					v = 255
				}
				dst[r*w+c] = uint8(v)
			} else {
				// Ray exits through the left — sample left.
				base := yOff >> 6
				shift := (yOff >> 1) & 0x1F
				b1 := base
				b2 := base + 1
				if b1 > maxL {
					b1 = maxL
				}
				if b2 > maxL {
					b2 = maxL
				}
				v := (int(left[b1])*(32-shift) + int(left[b2])*shift + 16) >> 5
				if v < 0 {
					v = 0
				} else if v > 255 {
					v = 255
				}
				dst[r*w+c] = uint8(v)
			}
		}
	}
}
