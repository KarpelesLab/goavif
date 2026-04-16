package loopfilter

// DeriveThresholds computes the deblocking filter Thresholds from the
// uncompressed frame header's filter_level + sharpness per spec §7.14.3.
//
//	filterLevel is the per-plane filter_level (Y_V, Y_H, U, V all 0..63).
//	sharpness is 0..7.
//
// The resulting Thresholds govern the narrow filter's mask tests:
//
//	limit  = filter_level >> (sharpness >= 7 ? 2 : 1), clamped low bound
//	blimit = 2 * (filter_level + 2) + limit
//	thresh = filter_level >> 4
//
// A filter_level of 0 disables the filter; [ApplyFrameNarrow] skips
// every edge if NarrowMask rejects it, which naturally happens when
// limit is very small.
func DeriveThresholds(filterLevel, sharpness int) Thresholds {
	if filterLevel < 0 {
		filterLevel = 0
	}
	if filterLevel > 63 {
		filterLevel = 63
	}
	if sharpness < 0 {
		sharpness = 0
	}
	if sharpness > 7 {
		sharpness = 7
	}

	shift := 1
	if sharpness >= 7 {
		shift = 2
	}
	limit := filterLevel >> shift

	// When sharpness > 0, limit can't exceed MAX_LOOP_FILTER / (sharpness+1).
	if sharpness > 0 {
		cap := 63 / (sharpness + 1)
		if limit > cap {
			limit = cap
		}
	}
	if limit < 1 {
		limit = 1
	}

	blimit := 2*(filterLevel+2) + limit
	if blimit > 255 {
		blimit = 255
	}
	thresh := filterLevel >> 4

	return Thresholds{
		Limit:  uint8(limit),
		Blimit: uint8(blimit),
		Thresh: uint8(thresh),
	}
}
