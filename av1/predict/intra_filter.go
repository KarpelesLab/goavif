package predict

// Filter-intra prediction (spec §7.11.2.7).
//
// The block is processed in 4×2 cells; each cell predicts 8 new samples
// from a row of 5 above samples (including the top-left corner) and 2
// left samples, via an 8×7 coefficient matrix per mode.

// FilterIntraTaps are the 5 learned tap sets from libaom's
// av1_filter_intra_taps. Shape: [mode][output_idx][input_idx] where the
// 8 outputs are the 8 samples of a 4×2 cell and the 7 inputs are the
// corner/above/left reference samples.
var FilterIntraTaps = [5][8][8]int8{
	// Mode 0
	{
		{-6, 10, 0, 0, 0, 12, 0, 0},
		{-5, 2, 10, 0, 0, 9, 0, 0},
		{-3, 1, 1, 10, 0, 7, 0, 0},
		{-3, 1, 1, 2, 10, 5, 0, 0},
		{-4, 6, 0, 0, 0, 2, 12, 0},
		{-3, 2, 6, 0, 0, 2, 9, 0},
		{-3, 2, 2, 6, 0, 2, 7, 0},
		{-3, 1, 2, 2, 6, 3, 5, 0},
	},
	// Mode 1
	{
		{-10, 16, 0, 0, 0, 10, 0, 0},
		{-6, 0, 16, 0, 0, 6, 0, 0},
		{-4, 0, 0, 16, 0, 4, 0, 0},
		{-2, 0, 0, 0, 16, 2, 0, 0},
		{-10, 16, 0, 0, 0, 0, 10, 0},
		{-6, 0, 16, 0, 0, 0, 6, 0},
		{-4, 0, 0, 16, 0, 0, 4, 0},
		{-2, 0, 0, 0, 16, 0, 2, 0},
	},
	// Mode 2
	{
		{-8, 8, 0, 0, 0, 16, 0, 0},
		{-8, 0, 8, 0, 0, 16, 0, 0},
		{-8, 0, 0, 8, 0, 16, 0, 0},
		{-8, 0, 0, 0, 8, 16, 0, 0},
		{-4, 4, 0, 0, 0, 0, 16, 0},
		{-4, 0, 4, 0, 0, 0, 16, 0},
		{-4, 0, 0, 4, 0, 0, 16, 0},
		{-4, 0, 0, 0, 4, 0, 16, 0},
	},
	// Mode 3
	{
		{-2, 8, 0, 0, 0, 10, 0, 0},
		{-1, 3, 8, 0, 0, 6, 0, 0},
		{-1, 2, 3, 8, 0, 4, 0, 0},
		{0, 1, 2, 3, 8, 2, 0, 0},
		{-1, 4, 0, 0, 0, 3, 10, 0},
		{-1, 3, 4, 0, 0, 4, 6, 0},
		{-1, 2, 3, 4, 0, 4, 4, 0},
		{-1, 2, 2, 3, 4, 3, 3, 0},
	},
	// Mode 4
	{
		{-12, 14, 0, 0, 0, 14, 0, 0},
		{-10, 0, 14, 0, 0, 12, 0, 0},
		{-9, 0, 0, 14, 0, 11, 0, 0},
		{-8, 0, 0, 0, 14, 10, 0, 0},
		{-10, 12, 0, 0, 0, 0, 14, 0},
		{-9, 1, 12, 0, 0, 0, 12, 0},
		{-8, 0, 0, 12, 0, 1, 11, 0},
		{-7, 0, 0, 1, 12, 1, 9, 0},
	},
}

// FilterIntraPred runs filter_intra prediction for a block up to 32×32.
// above/left must be extended so the algorithm can read (bw+1) above
// samples starting from index 0 (including the corner) and bh left
// samples. aboveLeft is the (-1, -1) corner reference.
//
// dst is written with w*h samples in row-major layout.
func FilterIntraPred(dst []uint8, w, h int, above, left []uint8, aboveLeft uint8, mode int) {
	if mode < 0 || mode >= 5 {
		mode = 0
	}
	// Working buffer including the corner row/column reference.
	var buf [33][33]uint8
	buf[0][0] = aboveLeft
	for c := 0; c < w; c++ {
		buf[0][c+1] = above[c]
	}
	for r := 0; r < h; r++ {
		buf[r+1][0] = left[r]
	}

	for r := 1; r < h+1; r += 2 {
		for c := 1; c < w+1; c += 4 {
			p0 := int(buf[r-1][c-1])
			p1 := int(buf[r-1][c])
			p2 := int(buf[r-1][c+1])
			p3 := int(buf[r-1][c+2])
			p4 := int(buf[r-1][c+3])
			p5 := int(buf[r][c-1])
			p6 := int(buf[r+1][c-1])
			for k := 0; k < 8; k++ {
				t := &FilterIntraTaps[mode][k]
				pr := int(t[0])*p0 + int(t[1])*p1 + int(t[2])*p2 +
					int(t[3])*p3 + int(t[4])*p4 + int(t[5])*p5 +
					int(t[6])*p6
				pr = (pr + 8) >> 4
				if pr < 0 {
					pr = 0
				} else if pr > 255 {
					pr = 255
				}
				dr := r + k/4
				dc := c + k%4
				buf[dr][dc] = uint8(pr)
			}
		}
	}

	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			dst[r*w+c] = buf[r+1][c+1]
		}
	}
}
