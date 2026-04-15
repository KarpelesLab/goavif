package colorspace

// MatrixCoefficients enumerates CICP MC values relevant to AVIF. Values
// match ITU-T H.273 Table 4.
type MatrixCoefficients uint8

const (
	MCIdentity MatrixCoefficients = 0
	MCBT709    MatrixCoefficients = 1
	MCUnspecified MatrixCoefficients = 2
	MCFCC      MatrixCoefficients = 4
	MCBT470BG  MatrixCoefficients = 5 // BT.601 625-line
	MCBT601    MatrixCoefficients = 6 // BT.601 525-line
	MCSMPTE240 MatrixCoefficients = 7
	MCYCoCg    MatrixCoefficients = 8
	MCBT2020NCL MatrixCoefficients = 9
	MCBT2020CL  MatrixCoefficients = 10
)

// Range encodes CICP video_full_range_flag.
type Range uint8

const (
	Studio Range = 0 // limited / legal range
	Full   Range = 1
)

// Kr and Kb are the luma weights for the non-constant-luma matrices. For
// other matrices they're derived or not applicable.
type matrix struct {
	Kr, Kb float64
}

func matrixFor(mc MatrixCoefficients) matrix {
	switch mc {
	case MCBT709:
		return matrix{Kr: 0.2126, Kb: 0.0722}
	case MCBT470BG, MCBT601, MCUnspecified:
		return matrix{Kr: 0.299, Kb: 0.114}
	case MCSMPTE240:
		return matrix{Kr: 0.212, Kb: 0.087}
	case MCBT2020NCL:
		return matrix{Kr: 0.2627, Kb: 0.0593}
	}
	// MC_IDENTITY / MC_YCoCg use dedicated code paths elsewhere.
	return matrix{Kr: 0.299, Kb: 0.114}
}

// YUVToRGB8 converts a single 8-bit YUV triple to 8-bit RGB under the
// given CICP matrix and range. Used for tests and low-throughput paths;
// plane-level conversion should use [ConvertPlanar420].
func YUVToRGB8(y, u, v uint8, mc MatrixCoefficients, rng Range) (r, g, b uint8) {
	if mc == MCIdentity {
		// "Identity" matrix encodes G in Y, B in U, R in V (per CICP).
		return v, y, u
	}

	m := matrixFor(mc)
	kg := 1.0 - m.Kr - m.Kb
	var yy, cb, cr float64
	if rng == Full {
		yy = float64(y) / 255.0
		cb = (float64(u) - 128) / 255.0
		cr = (float64(v) - 128) / 255.0
	} else {
		yy = (float64(y) - 16) / 219.0
		cb = (float64(u) - 128) / 224.0
		cr = (float64(v) - 128) / 224.0
	}
	rr := yy + 2*(1-m.Kr)*cr
	gg := yy - 2*m.Kb*(1-m.Kb)/kg*cb - 2*m.Kr*(1-m.Kr)/kg*cr
	bb := yy + 2*(1-m.Kb)*cb
	return clamp8(rr), clamp8(gg), clamp8(bb)
}

// clamp8 scales a normalized [0,1] value to [0,255] with clipping.
func clamp8(v float64) uint8 {
	x := v*255 + 0.5
	switch {
	case x < 0:
		return 0
	case x > 255:
		return 255
	}
	return uint8(x)
}

// ConvertPlanar420 converts an 8-bit 4:2:0 planar YUV image to an
// interleaved RGBA buffer (one byte per R, G, B, A; A is set to 255).
//
// ySrc is w*h bytes, uSrc and vSrc are each (w/2)*(h/2) bytes. dst must
// be w*h*4 bytes long.
func ConvertPlanar420(dst, ySrc, uSrc, vSrc []uint8, w, h int, mc MatrixCoefficients, rng Range) {
	cw := w >> 1
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			y := ySrc[r*w+c]
			u := uSrc[(r>>1)*cw+(c>>1)]
			v := vSrc[(r>>1)*cw+(c>>1)]
			rr, gg, bb := YUVToRGB8(y, u, v, mc, rng)
			i := (r*w + c) * 4
			dst[i+0] = rr
			dst[i+1] = gg
			dst[i+2] = bb
			dst[i+3] = 255
		}
	}
}
