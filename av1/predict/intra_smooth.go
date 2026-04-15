package predict

// Smooth-weight tables from spec §7.11.2.6. The weight value at index r (or
// c) is a linear-ish ramp from 255 toward the block's "scale tap" value.
//
// smWeights selects the table appropriate for block dimension N (4/8/16/32).

var smWeights4 = [4]uint16{255, 149, 85, 64}

var smWeights8 = [8]uint16{255, 197, 146, 105, 73, 50, 37, 32}

var smWeights16 = [16]uint16{255, 225, 196, 170, 145, 123, 102, 84,
	68, 54, 43, 33, 26, 20, 17, 16}

var smWeights32 = [32]uint16{255, 240, 225, 210, 196, 182, 169, 157,
	145, 133, 122, 111, 101, 92, 83, 74,
	66, 59, 52, 45, 39, 34, 29, 25,
	21, 17, 14, 12, 10, 9, 8, 8}

var smWeights64 = [64]uint16{
	255, 248, 242, 235, 228, 222, 215, 209, 202, 196, 190, 183, 177, 171, 165, 159,
	153, 148, 142, 137, 131, 126, 121, 116, 111, 106, 101, 96, 91, 87, 83, 78,
	74, 70, 66, 63, 59, 56, 53, 50, 47, 44, 41, 39, 36, 34, 32, 30,
	28, 26, 24, 23, 21, 20, 18, 17, 16, 15, 14, 13, 12, 11, 10, 10,
}

// smWeightTable returns the sm_weights entry for a given power-of-two
// block dimension N in {4, 8, 16, 32, 64}. It panics for unsupported sizes.
func smWeightTable(N int) []uint16 {
	switch N {
	case 4:
		return smWeights4[:]
	case 8:
		return smWeights8[:]
	case 16:
		return smWeights16[:]
	case 32:
		return smWeights32[:]
	case 64:
		return smWeights64[:]
	}
	panic("predict: sm_weights: unsupported block size")
}

// SmoothPred fills dst with SMOOTH_PRED (spec §7.11.2.6). All four border
// samples are used: above[0..w-1], left[0..h-1], bottomLeft = left[h-1],
// topRight = above[w-1].
func SmoothPred(dst []uint8, w, h int, above, left []uint8) {
	wh := smWeightTable(h)
	ww := smWeightTable(w)
	belowPred := uint32(left[h-1])
	rightPred := uint32(above[w-1])
	for r := 0; r < h; r++ {
		wr := uint32(wh[r])
		for c := 0; c < w; c++ {
			wc := uint32(ww[c])
			pred := wr*uint32(above[c]) + (256-wr)*belowPred +
				wc*uint32(left[r]) + (256-wc)*rightPred
			dst[r*w+c] = uint8((pred + 256) >> 9)
		}
	}
}

// SmoothVPred fills dst with SMOOTH_V_PRED (spec §7.11.2.6): vertical
// interpolation between above[c] and left[h-1].
func SmoothVPred(dst []uint8, w, h int, above, left []uint8) {
	wh := smWeightTable(h)
	belowPred := uint32(left[h-1])
	for r := 0; r < h; r++ {
		wr := uint32(wh[r])
		for c := 0; c < w; c++ {
			pred := wr*uint32(above[c]) + (256-wr)*belowPred
			dst[r*w+c] = uint8((pred + 128) >> 8)
		}
	}
}

// SmoothHPred fills dst with SMOOTH_H_PRED: horizontal interpolation
// between left[r] and above[w-1].
func SmoothHPred(dst []uint8, w, h int, above, left []uint8) {
	ww := smWeightTable(w)
	rightPred := uint32(above[w-1])
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			wc := uint32(ww[c])
			pred := wc*uint32(left[r]) + (256-wc)*rightPred
			dst[r*w+c] = uint8((pred + 128) >> 8)
		}
	}
}
