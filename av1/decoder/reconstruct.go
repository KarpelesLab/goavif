package decoder

// ReconstructBlock adds a dequantized residual to a predicted block, clips
// to the channel range, and writes the samples into dst as uint8 values.
//
// dst has w*h bytes (row-major). pred has w*h uint8 samples. residual
// has w*h int32 samples (the inverse-transform output, signed and scaled
// per spec §7.7.4's final round-and-shift).
//
// For bit depths > 8 use [Reconstruct16Block] (not yet implemented) so
// samples are clipped to the correct maximum.
func ReconstructBlock(dst []uint8, pred []uint8, residual []int32, w, h int) {
	for i := 0; i < w*h; i++ {
		v := int32(pred[i]) + residual[i]
		if v < 0 {
			v = 0
		} else if v > 255 {
			v = 255
		}
		dst[i] = uint8(v)
	}
}
