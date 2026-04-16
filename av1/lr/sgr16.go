package lr

// SGRSubFilter16 is the uint16 counterpart of [SGRSubFilter]. Output is
// clipped to [0, (1<<bitDepth)-1]. The eps adjustment per bit depth is
// the caller's responsibility — pass an eps already scaled for the
// chosen bit depth.
func SGRSubFilter16(dst, src []uint16, w, h, stride, r, eps, bitDepth int) {
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
	tmp := make([]uint16, w*h)
	// For HBD, samples can be up to 12-bit so sumSq grows to
	// 4095²·49 ≈ 8.2e8 — still within int32, but push to int64 for
	// safety when a radius of 2 is used at 12-bit.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var sum int64
			var sumSq int64
			for dy := -r; dy <= r; dy++ {
				yy := clamp(y+dy, 0, h-1)
				for dx := -r; dx <= r; dx++ {
					xx := clamp(x+dx, 0, w-1)
					s := int64(src[yy*stride+xx])
					sum += s
					sumSq += s * s
				}
			}
			N := int64(n)
			p := N*sumSq - sum*sum
			if p < 0 {
				p = 0
			}
			// Scale p down by (bitDepth-8)*2 bits so the LUT index
			// matches the 8-bit-range expectation of sgrXByXPlus1.
			shift := uint(2 * (bitDepth - 8))
			p >>= shift
			z := (p*int64(eps) + (1 << 19)) >> 20
			if z < 0 {
				z = 0
			}
			if z > 255 {
				z = 255
			}
			a := int64(sgrXByXPlus1[z])
			mean := sum / N
			pix := int64(src[y*stride+x])
			out := (a*pix + (256-a)*mean + 128) >> 8
			tmp[y*w+x] = clipBD(int(out), bitDepth)
		}
	}
	for y := 0; y < h; y++ {
		copy(dst[y*stride:y*stride+w], tmp[y*w:y*w+w])
	}
}

// ApplySGR16 is the uint16 counterpart of [ApplySGR].
func ApplySGR16(dst, src []uint16, w, h, stride int, p SGRParams, bitDepth int) {
	if p.R0 == 0 && p.R1 == 0 {
		for y := 0; y < h; y++ {
			copy(dst[y*stride:y*stride+w], src[y*stride:y*stride+w])
		}
		return
	}
	flt0 := make([]uint16, w*h)
	flt1 := make([]uint16, w*h)
	SGRSubFilter16(flt0, src, w, h, w, p.R0, p.Eps0, bitDepth)
	SGRSubFilter16(flt1, src, w, h, w, p.R1, p.Eps1, bitDepth)
	maxV := (1 << uint(bitDepth)) - 1
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			pix := int(src[y*stride+x])
			d0 := int(flt0[y*w+x]) - pix
			d1 := int(flt1[y*w+x]) - pix
			v := pix + ((p.Xq[0]*d0 + p.Xq[1]*d1 + 32) >> 6)
			if v < 0 {
				v = 0
			} else if v > maxV {
				v = maxV
			}
			dst[y*stride+x] = uint16(v)
		}
	}
}
