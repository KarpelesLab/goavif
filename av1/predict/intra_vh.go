package predict

// VPred fills dst with vertical prediction: each row is a copy of the
// above row (spec §7.11.2.2). haveAbove must be true — the spec guarantees
// V_PRED is only selected when the above row is available.
func VPred(dst []uint8, w, h int, above []uint8) {
	for r := 0; r < h; r++ {
		copy(dst[r*w:(r+1)*w], above[:w])
	}
}

// HPred fills dst with horizontal prediction: each column is a copy of the
// corresponding left sample (spec §7.11.2.2).
func HPred(dst []uint8, w, h int, left []uint8) {
	for r := 0; r < h; r++ {
		v := left[r]
		row := dst[r*w : (r+1)*w]
		for c := range row {
			row[c] = v
		}
	}
}
