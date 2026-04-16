package predict

// DirectionalPred16 is the uint16 counterpart of [DirectionalPred].
// Results are clipped to the bit-depth range [0, (1<<bitDepth)-1].
func DirectionalPred16(dst []uint16, w, h int, above, left []uint16, angle, bitDepth int) {
	maxV := (1 << uint(bitDepth)) - 1
	switch {
	case angle > 0 && angle < 90:
		drPredAboveZone16(dst, w, h, above, getDx(angle), maxV)
	case angle > 180 && angle < 270:
		drPredLeftZone16(dst, w, h, left, getDy(angle), maxV)
	case angle > 90 && angle < 180:
		drPredMixedZone16(dst, w, h, above, left, getDx(angle), getDy(angle), maxV)
	case angle == 90:
		for r := 0; r < h; r++ {
			for c := 0; c < w; c++ {
				dst[r*w+c] = above[c]
			}
		}
	case angle == 180:
		for r := 0; r < h; r++ {
			v := left[r]
			for c := 0; c < w; c++ {
				dst[r*w+c] = v
			}
		}
	}
}

func drPredAboveZone16(dst []uint16, w, h int, above []uint16, dx, maxV int) {
	maxIdx := len(above) - 1
	for r := 0; r < h; r++ {
		offset := (r + 1) * dx
		for c := 0; c < w; c++ {
			base := c + (offset >> 6)
			shift := (offset >> 1) & 0x1F
			b1 := base
			b2 := base + 1
			if b1 > maxIdx {
				b1 = maxIdx
			}
			if b2 > maxIdx {
				b2 = maxIdx
			}
			v := (int(above[b1])*(32-shift) + int(above[b2])*shift + 16) >> 5
			if v < 0 {
				v = 0
			} else if v > maxV {
				v = maxV
			}
			dst[r*w+c] = uint16(v)
		}
	}
}

func drPredLeftZone16(dst []uint16, w, h int, left []uint16, dy, maxV int) {
	maxIdx := len(left) - 1
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			offset := (c + 1) * dy
			base := r + (offset >> 6)
			shift := (offset >> 1) & 0x1F
			b1 := base
			b2 := base + 1
			if b1 > maxIdx {
				b1 = maxIdx
			}
			if b2 > maxIdx {
				b2 = maxIdx
			}
			v := (int(left[b1])*(32-shift) + int(left[b2])*shift + 16) >> 5
			if v < 0 {
				v = 0
			} else if v > maxV {
				v = maxV
			}
			dst[r*w+c] = uint16(v)
		}
	}
}

func drPredMixedZone16(dst []uint16, w, h int, above, left []uint16, dx, dy, maxV int) {
	maxA := len(above) - 1
	maxL := len(left) - 1
	invDy := dy
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			xOff := -(r+1)*dx + (c+1)*64
			yOff := -(c+1)*invDy + (r+1)*64
			if xOff >= 0 {
				base := xOff >> 6
				shift := (xOff >> 1) & 0x1F
				b1 := base
				b2 := base + 1
				if b1 > maxA {
					b1 = maxA
				}
				if b2 > maxA {
					b2 = maxA
				}
				v := (int(above[b1])*(32-shift) + int(above[b2])*shift + 16) >> 5
				if v < 0 {
					v = 0
				} else if v > maxV {
					v = maxV
				}
				dst[r*w+c] = uint16(v)
			} else {
				base := yOff >> 6
				shift := (yOff >> 1) & 0x1F
				b1 := base
				b2 := base + 1
				if b1 > maxL {
					b1 = maxL
				}
				if b2 > maxL {
					b2 = maxL
				}
				v := (int(left[b1])*(32-shift) + int(left[b2])*shift + 16) >> 5
				if v < 0 {
					v = 0
				} else if v > maxV {
					v = maxV
				}
				dst[r*w+c] = uint16(v)
			}
		}
	}
}
