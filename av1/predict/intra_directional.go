package predict

// D45Pred fills dst with the 45-degree directional predictor (spec
// §7.11.2.5). Each sample is read from the `above` row at a distance that
// increases diagonally; aboveExtended is the pre-filtered extension of the
// above row and must contain at least (w + h - 1) samples. Out-of-range
// samples are replicated from the last available one.
//
// The AV1 specification applies a recursive-filter intra-edge filter before
// sampling, controlled by intra_edge_filter and block size heuristics.
// Callers are expected to pre-apply that filter to aboveExtended when the
// decoder requests it; this helper does not modify the input.
func D45Pred(dst []uint8, w, h int, aboveExtended []uint8) {
	maxIdx := len(aboveExtended) - 1
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			idx := r + c + 1
			if idx > maxIdx {
				idx = maxIdx
			}
			dst[r*w+c] = aboveExtended[idx]
		}
	}
}

// D135Pred fills dst with the 135-degree directional predictor. Each
// sample is read along a diagonal running from top-right to bottom-left.
// The predictor consumes aboveLeft + one step of above[] or left[] per
// sample.
//
// aboveExtended must contain at least w samples; leftExtended must contain
// at least h samples. aboveLeft is the sample at (−1, −1).
func D135Pred(dst []uint8, w, h int, above, left []uint8, aboveLeft uint8) {
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			var v uint8
			switch {
			case r == c:
				v = aboveLeft
			case r > c:
				v = left[r-c-1]
			default:
				v = above[c-r-1]
			}
			dst[r*w+c] = v
		}
	}
}
