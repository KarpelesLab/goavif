package lr

// Self-guided restoration (SGR) — spec §7.17.4.
//
// SGR runs two box filters of different radii over the input, builds a
// per-pixel blending coefficient a from the local variance, and mixes a
// fraction of the original pixel with the local mean. The two
// sub-filters are then combined with learned weights xq[0] and xq[1]
// signaled per restoration unit.
//
// This file carries the box-mean / box-variance primitives and a
// placeholder ApplySGR that exposes the intended interface. The full
// spec path (with the exact parameter set tables from sgrproj_params)
// lands in a follow-up commit.

// BoxMean computes the mean of samples in a (2r+1)×(2r+1) window around
// each pixel. Out-of-range samples are clamp-replicated. Output is per-
// pixel mean in uint16 to preserve up to 12-bit sample ranges.
func BoxMean(src []uint8, w, h, stride, r int) []uint16 {
	out := make([]uint16, w*h)
	area := (2*r + 1) * (2*r + 1)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sum := 0
			for dy := -r; dy <= r; dy++ {
				yy := clamp(y+dy, 0, h-1)
				for dx := -r; dx <= r; dx++ {
					xx := clamp(x+dx, 0, w-1)
					sum += int(src[yy*stride+xx])
				}
			}
			out[y*w+x] = uint16(sum / area)
		}
	}
	return out
}

// BoxVar computes the sample variance (not unbiased — simple σ² form) in
// the same (2r+1)×(2r+1) window. Uses the identity
// var = E[x²] − (E[x])².
func BoxVar(src []uint8, w, h, stride, r int) []uint32 {
	out := make([]uint32, w*h)
	area := (2*r + 1) * (2*r + 1)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum, sumSq int
			for dy := -r; dy <= r; dy++ {
				yy := clamp(y+dy, 0, h-1)
				for dx := -r; dx <= r; dx++ {
					xx := clamp(x+dx, 0, w-1)
					s := int(src[yy*stride+xx])
					sum += s
					sumSq += s * s
				}
			}
			mean := sum / area
			variance := sumSq/area - mean*mean
			if variance < 0 {
				variance = 0
			}
			out[y*w+x] = uint32(variance)
		}
	}
	return out
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// SGRParams carries the per-unit SGR coefficients. xq[0] and xq[1] are
// signed weights blending the two sub-filters with the input. r0 / r1
// are the window radii per sub-filter (0 disables a sub-filter). eps0
// / eps1 are the edge-preservation thresholds.
type SGRParams struct {
	R0, R1   int
	Eps0, Eps1 int
	Xq       [2]int // signed, Q6
}

// ApplySGR is a placeholder for the spec's sgrproj_box_filter +
// sgrproj_dual path. Today it returns the input unchanged — callers
// can use the BoxMean / BoxVar primitives above to prototype the
// filter flow and the final per-unit blend.
func ApplySGR(dst, src []uint8, w, h, stride int, p SGRParams) {
	copy(dst, src)
}
