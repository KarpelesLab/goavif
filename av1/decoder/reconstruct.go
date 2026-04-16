package decoder

// ReconstructBlock adds a dequantized residual to a predicted block, clips
// to the channel range, and writes the samples into dst as uint8 values.
//
// dst has w*h bytes (row-major). pred has w*h uint8 samples. residual
// has w*h int32 samples (the inverse-transform output, signed and scaled
// per spec §7.7.4's final round-and-shift).
//
// For bit depths > 8 use [Reconstruct16Block] so samples are clipped to
// the correct maximum.
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

// Reconstruct16Block is the uint16 counterpart used by 10/12-bit
// decode. It adds residual to pred and clips to [0, (1<<bitDepth)-1].
func Reconstruct16Block(dst []uint16, pred []uint16, residual []int32, w, h, bitDepth int) {
	maxV := int32((1 << uint(bitDepth)) - 1)
	for i := 0; i < w*h; i++ {
		v := int32(pred[i]) + residual[i]
		if v < 0 {
			v = 0
		} else if v > maxV {
			v = maxV
		}
		dst[i] = uint16(v)
	}
}
