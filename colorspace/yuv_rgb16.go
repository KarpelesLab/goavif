package colorspace

// YUVToRGB16 converts a single 10/12-bit YUV triple to 16-bit RGB
// under the given CICP matrix and range. Output samples are scaled
// to the full 16-bit range (x * 65535 / (2^bitDepth - 1)) so they
// fit into image.RGBA64 / image.NRGBA64 regardless of the input
// bit depth.
//
// Identity matrix (MC_IDENTITY) encodes G in Y, B in U, R in V, like
// the uint8 path.
func YUVToRGB16(y, u, v uint16, mc MatrixCoefficients, rng Range, bitDepth int) (r, g, b uint16) {
	if mc == MCIdentity {
		return scaleToU16(v, bitDepth),
			scaleToU16(y, bitDepth),
			scaleToU16(u, bitDepth)
	}

	m := matrixFor(mc)
	kg := 1.0 - m.Kr - m.Kb
	maxV := float64((uint(1) << uint(bitDepth)) - 1)
	mid := float64(uint(1) << uint(bitDepth-1))
	var yy, cb, cr float64
	if rng == Full {
		yy = float64(y) / maxV
		cb = (float64(u) - mid) / maxV
		cr = (float64(v) - mid) / maxV
	} else {
		// Studio / legal range at HBD: footroom and headroom both scale
		// by the same (bitDepth-8) factor per BT.709 §6.2.
		shift := float64(uint(1) << uint(bitDepth-8))
		yMin := 16.0 * shift
		yRange := 219.0 * shift
		cRange := 224.0 * shift
		yy = (float64(y) - yMin) / yRange
		cb = (float64(u) - mid) / cRange
		cr = (float64(v) - mid) / cRange
	}
	rr := yy + 2*(1-m.Kr)*cr
	gg := yy - 2*m.Kb*(1-m.Kb)/kg*cb - 2*m.Kr*(1-m.Kr)/kg*cr
	bb := yy + 2*(1-m.Kb)*cb
	return clamp16(rr), clamp16(gg), clamp16(bb)
}

// scaleToU16 maps an N-bit sample to the full 16-bit range.
func scaleToU16(v uint16, bitDepth int) uint16 {
	maxIn := uint32((1 << uint(bitDepth)) - 1)
	if maxIn == 0 {
		return 0
	}
	return uint16(uint32(v) * 65535 / maxIn)
}

// clamp16 scales a normalized [0,1] value to [0,65535] with clipping.
func clamp16(v float64) uint16 {
	x := v*65535 + 0.5
	switch {
	case x < 0:
		return 0
	case x > 65535:
		return 65535
	}
	return uint16(x)
}

// ConvertPlanar420_16 converts a 10/12-bit 4:2:0 planar YUV image into
// an interleaved 16-bit-per-channel RGBA buffer (big-endian for
// image.RGBA64 compatibility).
func ConvertPlanar420_16(dst []uint8, ySrc, uSrc, vSrc []uint16, w, h int, mc MatrixCoefficients, rng Range, bitDepth int) {
	ConvertPlanar16(dst, ySrc, uSrc, vSrc, w, h, 1, 1, mc, rng, bitDepth)
}

// ConvertPlanar16 is the generalized HBD planar-YUV → RGBA16 converter.
// subX / subY ∈ {0, 1} pick the chroma subsampling: 4:2:0 = (1, 1),
// 4:2:2 = (1, 0), 4:4:4 = (0, 0).
func ConvertPlanar16(dst []uint8, ySrc, uSrc, vSrc []uint16, w, h, subX, subY int, mc MatrixCoefficients, rng Range, bitDepth int) {
	cw := w >> subX
	if cw < 1 {
		cw = 1
	}
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			y := ySrc[r*w+c]
			u := uSrc[(r>>subY)*cw+(c>>subX)]
			v := vSrc[(r>>subY)*cw+(c>>subX)]
			rr, gg, bb := YUVToRGB16(y, u, v, mc, rng, bitDepth)
			i := (r*w + c) * 8
			dst[i+0] = uint8(rr >> 8)
			dst[i+1] = uint8(rr & 0xFF)
			dst[i+2] = uint8(gg >> 8)
			dst[i+3] = uint8(gg & 0xFF)
			dst[i+4] = uint8(bb >> 8)
			dst[i+5] = uint8(bb & 0xFF)
			dst[i+6] = 0xFF
			dst[i+7] = 0xFF
		}
	}
}
