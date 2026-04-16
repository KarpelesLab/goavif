package lr

// Self-guided restoration (SGR) — spec §7.17.4.
//
// SGR runs a variance-adaptive box filter whose output blend toward the
// local mean in flat regions and toward the input in high-variance
// regions. The single-pass filter is:
//
//	n = (2r+1)²                       // window area
//	A = sum over window of pixel²
//	B = sum over window of pixel
//	p = max(0, n·A − B²)              // n · σ² up to scaling
//	z = clip((p·eps + (1<<19)) >> 20, 0, 255)
//	a = sgrXByXPlus1[z]               // 256·z / (z+1)
//	mean = B / n
//	output = (a·pixel + (256−a)·mean + 128) >> 8
//
// AV1's full SGR is dual-pass with two sub-filters blended by per-unit
// xq[0,1] weights. ApplySGR implements the dual form; SGRSubFilter is
// exposed so callers / tests can exercise a single pass.

// sgrXByXPlus1 is the x/(x+1)·256 LUT used by the a-value step
// (spec §7.17.4 av1_x_by_xplus1). Entry 255 saturates.
var sgrXByXPlus1 = [256]uint8{
	1, 128, 171, 192, 205, 213, 219, 224, 228, 230, 233, 235, 236, 238, 239, 240,
	241, 242, 243, 243, 244, 244, 245, 245, 246, 246, 247, 247, 247, 247, 248, 248,
	248, 248, 249, 249, 249, 249, 249, 250, 250, 250, 250, 250, 250, 250, 251, 251,
	251, 251, 251, 251, 251, 251, 251, 251, 252, 252, 252, 252, 252, 252, 252, 252,
	252, 252, 252, 252, 252, 252, 252, 252, 253, 253, 253, 253, 253, 253, 253, 253,
	253, 253, 253, 253, 253, 253, 253, 253, 253, 253, 253, 253, 253, 253, 253, 253,
	253, 253, 253, 253, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254,
	254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 254, 255, 255,
}

// BoxMean computes the mean of samples in a (2r+1)×(2r+1) window around
// each pixel. Out-of-range samples are clamp-replicated.
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

// BoxVar computes sample variance σ² in the same (2r+1)² window.
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

// SGRParams carries the per-unit SGR coefficients. R0 / R1 are window
// radii (0 disables a sub-filter). Eps0 / Eps1 are the edge-
// preservation thresholds. Xq[0,1] are signed Q6 blending weights
// signaled per restoration unit.
type SGRParams struct {
	R0, R1     int
	Eps0, Eps1 int
	Xq         [2]int
}

// SGRSubFilter runs one SGR pass with radius r and eps. Passes with
// r == 0 are a no-op (copy). Operates on w×h pixels at the given
// stride; src and dst may alias.
func SGRSubFilter(dst, src []uint8, w, h, stride, r, eps int) {
	if r <= 0 {
		if len(dst) > 0 && len(src) > 0 && &dst[0] == &src[0] {
			return
		}
		for y := 0; y < h; y++ {
			copy(dst[y*stride:y*stride+w], src[y*stride:y*stride+w])
		}
		return
	}
	n := (2*r + 1) * (2*r + 1)
	tmp := make([]uint8, w*h)
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
			p := n*sumSq - sum*sum
			if p < 0 {
				p = 0
			}
			z := (p*eps + (1 << 19)) >> 20
			if z < 0 {
				z = 0
			}
			if z > 255 {
				z = 255
			}
			a := int(sgrXByXPlus1[z])
			mean := sum / n
			pix := int(src[y*stride+x])
			out := (a*pix + (256-a)*mean + 128) >> 8
			tmp[y*w+x] = clip8(out)
		}
	}
	for y := 0; y < h; y++ {
		copy(dst[y*stride:y*stride+w], tmp[y*w:y*w+w])
	}
}

// ApplySGR runs the dual-sub-filter SGR path and blends the two
// outputs with the input per spec §7.17.4. When both radii are zero
// it copies the input through.
func ApplySGR(dst, src []uint8, w, h, stride int, p SGRParams) {
	if p.R0 == 0 && p.R1 == 0 {
		for y := 0; y < h; y++ {
			copy(dst[y*stride:y*stride+w], src[y*stride:y*stride+w])
		}
		return
	}
	flt0 := make([]uint8, w*h)
	flt1 := make([]uint8, w*h)
	SGRSubFilter(flt0, src, w, h, w, p.R0, p.Eps0)
	SGRSubFilter(flt1, src, w, h, w, p.R1, p.Eps1)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pix := int(src[y*stride+x])
			d0 := int(flt0[y*w+x]) - pix
			d1 := int(flt1[y*w+x]) - pix
			v := pix + ((p.Xq[0]*d0 + p.Xq[1]*d1 + 32) >> 6)
			if v < 0 {
				v = 0
			} else if v > 255 {
				v = 255
			}
			dst[y*stride+x] = uint8(v)
		}
	}
}
