package lr

// WienerTaps holds the 4 unique coefficients used along one axis of the
// 7-tap symmetric Wiener filter. The full 7-element kernel is:
//
//	{Taps[0], Taps[1], Taps[2], Taps[3], Taps[2], Taps[1], Taps[0]}
//
// The center coefficient (Taps[3]) is the strongest; outer ones are
// typically negative. Each coefficient is in a Q7 fixed-point form
// (the spec uses values in [-127, 128) representing filter weights
// divided by 128).
type WienerTaps [4]int

// clip8 saturates the result of a Wiener convolution back to 0..255.
func clip8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// convolveRow applies a symmetric 7-tap filter across a single row,
// writing output bytes to dst. src is read with edge clamping.
func convolveRow(dst, src []uint8, w int, taps WienerTaps) {
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
		dst[i] = clip8((acc + 64) >> 7)
	}
}

// ApplyWiener runs the 7×7 separable Wiener filter on the given plane.
// The horizontal 1D pass is applied first (per row), then the vertical
// 1D pass (per column). horiz/vert are the filter taps.
//
// src and dst may alias. When they do we use an internal intermediate
// buffer to preserve the two-pass semantics.
func ApplyWiener(dst, src []uint8, w, h, stride int, horiz, vert WienerTaps) {
	// First pass: horizontal convolution into a scratch buffer so the
	// column pass reads the filtered rows.
	tmp := make([]uint8, w*h)
	for r := 0; r < h; r++ {
		srcRow := src[r*stride : r*stride+w]
		convolveRow(tmp[r*w:r*w+w], srcRow, w, horiz)
	}
	// Second pass: vertical 7-tap convolution down each column.
	col := make([]uint8, h)
	out := make([]uint8, h)
	for c := 0; c < w; c++ {
		for r := 0; r < h; r++ {
			col[r] = tmp[r*w+c]
		}
		// Treat the column as a 1D sequence; reuse convolveRow.
		convolveRow(out, col, h, vert)
		for r := 0; r < h; r++ {
			dst[r*stride+c] = out[r]
		}
	}
}
