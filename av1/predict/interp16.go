package predict

// InterpSubPel16 is the HBD (uint16) counterpart of [InterpSubPel].
// Same 8-tap filter, same rounding, but output clipped to (1<<bitDepth)-1.
func InterpSubPel16(dst []uint16, w, h int, src []uint16, srcStride, hp, vp int, filt InterpFilter, bitDepth int) {
	hFilter := filtTable(filt)[hp]
	vFilter := filtTable(filt)[vp]
	maxV := int32((1 << uint(bitDepth)) - 1)
	// Horizontal pass into a temp row-buffer. HBD samples need int32
	// accumulators since a single sample can reach ~4095 × 128 = ~500K
	// per tap, × 8 taps ≈ 4M — still fits in int32 comfortably.
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
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			sum := int32(0)
			for k := 0; k < 8; k++ {
				sum += int32(vFilter[k]) * tmp[(r+k)*w+c]
			}
			v := (sum + (1 << 13)) >> 14
			if v < 0 {
				v = 0
			} else if v > maxV {
				v = maxV
			}
			dst[r*w+c] = uint16(v)
		}
	}
}

// InterpInteger16 copies a w×h block from an HBD src at integer-pel
// offset.
func InterpInteger16(dst []uint16, w, h int, src []uint16, srcStride int) {
	for r := 0; r < h; r++ {
		copy(dst[r*w:r*w+w], src[r*srcStride:r*srcStride+w])
	}
}
