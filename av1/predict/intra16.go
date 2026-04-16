package predict

// This file defines uint16 variants of the core intra prediction
// functions used by 10- and 12-bit AV1 decoding. The algorithms are
// structurally identical to the uint8 versions; only the sample type
// and the bit-depth-sensitive DC fallback differ.
//
// Sample values must fit in bitDepth bits — callers are responsible
// for clamping. These functions never emit values beyond 2^bitDepth-1.

// DCPred16 is the uint16 counterpart of [DCPred].
func DCPred16(dst []uint16, w, h int, above, left []uint16, haveAbove, haveLeft bool, bitDepth int) {
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
	var dc uint16
	if n == 0 {
		dc = uint16(1) << uint(bitDepth-1)
	} else {
		dc = uint16((sum + n/2) / n)
	}
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			dst[row*w+col] = dc
		}
	}
}

// VPred16 is the uint16 counterpart of [VPred]. Each row of dst copies
// the above array.
func VPred16(dst []uint16, w, h int, above []uint16) {
	for row := 0; row < h; row++ {
		copy(dst[row*w:(row+1)*w], above[:w])
	}
}

// HPred16 is the uint16 counterpart of [HPred]. Each column of dst
// replicates the corresponding left sample across the row.
func HPred16(dst []uint16, w, h int, left []uint16) {
	for row := 0; row < h; row++ {
		v := left[row]
		for col := 0; col < w; col++ {
			dst[row*w+col] = v
		}
	}
}

// PaethPred16 is the uint16 counterpart of [PaethPred].
func PaethPred16(dst []uint16, w, h int, above, left []uint16, aboveLeft uint16) {
	for row := 0; row < h; row++ {
		for col := 0; col < w; col++ {
			base := int(above[col]) + int(left[row]) - int(aboveLeft)
			pA := abs16(base - int(above[col]))
			pL := abs16(base - int(left[row]))
			pAL := abs16(base - int(aboveLeft))
			var pred uint16
			switch {
			case pA <= pL && pA <= pAL:
				pred = above[col]
			case pL <= pAL:
				pred = left[row]
			default:
				pred = aboveLeft
			}
			dst[row*w+col] = pred
		}
	}
}

func abs16(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// SmoothPred16 is the uint16 counterpart of [SmoothPred]. It blends
// above and left toward the bottom-left and top-right corner samples.
func SmoothPred16(dst []uint16, w, h int, above, left []uint16) {
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
			dst[r*w+c] = uint16((pred + 256) >> 9)
		}
	}
}

// SmoothVPred16 blends only vertically (above → bottom-left).
func SmoothVPred16(dst []uint16, w, h int, above, left []uint16) {
	wh := smWeightTable(h)
	belowPred := uint32(left[h-1])
	for r := 0; r < h; r++ {
		wr := uint32(wh[r])
		for c := 0; c < w; c++ {
			pred := wr*uint32(above[c]) + (256-wr)*belowPred
			dst[r*w+c] = uint16((pred + 128) >> 8)
		}
	}
}

// SmoothHPred16 blends only horizontally (left → top-right).
func SmoothHPred16(dst []uint16, w, h int, above, left []uint16) {
	ww := smWeightTable(w)
	rightPred := uint32(above[w-1])
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			wc := uint32(ww[c])
			pred := wc*uint32(left[r]) + (256-wc)*rightPred
			dst[r*w+c] = uint16((pred + 128) >> 8)
		}
	}
}
