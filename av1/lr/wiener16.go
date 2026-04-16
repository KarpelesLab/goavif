package lr

// clipBD clips v to [0, (1<<bitDepth)-1] and narrows to uint16.
func clipBD(v, bitDepth int) uint16 {
	maxV := (1 << uint(bitDepth)) - 1
	if v < 0 {
		return 0
	}
	if v > maxV {
		return uint16(maxV)
	}
	return uint16(v)
}

// convolveRow16 is the uint16 counterpart of convolveRow.
func convolveRow16(dst, src []uint16, w int, taps WienerTaps, bitDepth int) {
	for i := 0; i < w; i++ {
		acc := 0
		for k := 0; k < 7; k++ {
			j := i + k - 3
			if j < 0 {
				j = 0
			}
			if j >= w {
				j = w - 1
			}
			var c int
			switch k {
			case 0, 6:
				c = taps[0]
			case 1, 5:
				c = taps[1]
			case 2, 4:
				c = taps[2]
			case 3:
				c = taps[3]
			}
			acc += c * int(src[j])
		}
		dst[i] = clipBD((acc+64)>>7, bitDepth)
	}
}

// ApplyWiener16 is the uint16 counterpart of [ApplyWiener].
func ApplyWiener16(dst, src []uint16, w, h, stride int, horiz, vert WienerTaps, bitDepth int) {
	tmp := make([]uint16, w*h)
	for r := 0; r < h; r++ {
		srcRow := src[r*stride : r*stride+w]
		convolveRow16(tmp[r*w:r*w+w], srcRow, w, horiz, bitDepth)
	}
	col := make([]uint16, h)
	out := make([]uint16, h)
	for c := 0; c < w; c++ {
		for r := 0; r < h; r++ {
			col[r] = tmp[r*w+c]
		}
		convolveRow16(out, col, h, vert, bitDepth)
		for r := 0; r < h; r++ {
			dst[r*stride+c] = out[r]
		}
	}
}
