package predict

// CFL (Chroma from Luma) prediction, spec §7.11.5.
//
// CFL reconstructs chroma by combining a per-block DC prediction with a
// scaled, AC-only copy of the co-located reconstructed luma block.
//
// Inputs:
//   - reconLuma: the already-reconstructed luma block, dimensions
//     lumaW × lumaH.
//   - subX, subY: chroma subsampling in the {0,1} range.
//   - dcPred: the chroma DC prediction computed via one of the DC/V/H/Paeth
//     modes, dimensions chromaW × chromaH.
//   - alpha: the per-chroma-plane signed alpha value from the bitstream,
//     as decoded by the CFL signaling. Range is -16..16 and is typically
//     stored as (sign, magnitude) in the CDFs.
//
// Output: dst, chromaW × chromaH chroma samples.

// CFLSubsample computes the chroma-resolution luma average by block-averaging
// reconLuma per the given subsampling factors.
func CFLSubsample(dst []int32, reconLuma []uint8, lumaW, lumaH, subX, subY int) {
	chromaW := lumaW >> subX
	chromaH := lumaH >> subY
	stepX := 1 << subX
	stepY := 1 << subY
	boxArea := stepX * stepY
	for r := 0; r < chromaH; r++ {
		for c := 0; c < chromaW; c++ {
			sum := 0
			for dy := 0; dy < stepY; dy++ {
				for dx := 0; dx < stepX; dx++ {
					sum += int(reconLuma[(r*stepY+dy)*lumaW+(c*stepX+dx)])
				}
			}
			// Per spec, store the sum scaled as Q3 (×8) before AC normalization.
			dst[r*chromaW+c] = int32(sum * 8 / boxArea)
		}
	}
}

// CFLPred fills dst with the CFL chroma prediction per spec §7.11.5.
//
// lumaQ3 is the chroma-resolution luma average computed by [CFLSubsample]
// (scaled by 8 / Q3). dcPred is the chroma DC-style prediction. alpha is the
// signed alpha value; positive alpha gives same-sign AC, negative gives
// inverted.
func CFLPred(dst []uint8, w, h int, lumaQ3 []int32, dcPred []uint8, alpha int) {
	// Compute luma Q3 AC: subtract the mean of lumaQ3 (rounded).
	var sum int64
	for _, v := range lumaQ3[:w*h] {
		sum += int64(v)
	}
	half := int64(w * h / 2)
	avg := int32((sum + half) / int64(w*h))

	for i := 0; i < w*h; i++ {
		ac := lumaQ3[i] - avg
		// scaled = alpha * ac; alpha is in Q6 (signed magnitude × sign).
		// Per spec §7.11.5: signed_shift_right((alpha * ac + 32), 6).
		scaled := int32(alpha) * ac
		scaled = (scaled + (1 << 5)) >> 6
		v := int32(dcPred[i]) + scaled
		switch {
		case v < 0:
			v = 0
		case v > 255:
			v = 255
		}
		dst[i] = uint8(v)
	}
}
