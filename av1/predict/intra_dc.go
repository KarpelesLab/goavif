package predict

// DCPred fills an w×h destination block with DC intra prediction per AV1
// spec §7.11.2.3.
//
// above and left are the reconstructed neighbor samples (w samples above,
// h samples left). When haveAbove is false the above row is absent; when
// haveLeft is false the left column is absent. Either or both may be
// missing at frame / tile boundaries, in which case the AV1 spec substitutes
// a half-range value.
//
// dst is written as a flat slice in row-major order with stride equal to w.
func DCPred(dst []uint8, w, h int, above, left []uint8, haveAbove, haveLeft bool, bitDepth int) {
	sum := 0
	n := 0
	if haveAbove {
		for _, v := range above[:w] {
			sum += int(v)
		}
		n += w
	}
	if haveLeft {
		for _, v := range left[:h] {
			sum += int(v)
		}
		n += h
	}
	var dc uint8
	if n == 0 {
		dc = uint8(1 << (bitDepth - 1))
	} else {
		dc = uint8((sum + n/2) / n)
	}
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			dst[row*w+col] = dc
		}
	}
}
