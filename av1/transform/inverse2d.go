package transform

import "fmt"

// Inverse2D runs the AV1 2D inverse transform: rows first, then columns
// (spec §7.7.4). coeffs is a W×H matrix in row-major layout with W = row
// length of sz and H = col length of sz. The result overwrites coeffs and
// represents the signed residual at the output of the inverse transform
// (before reconstruction adds it to the prediction).
//
// Returns an error if the (ty, sz) pair is not yet implemented.
func Inverse2D(coeffs []int32, ty TxType, sz TxSize) error {
	w := rowLen(sz)
	h := colLen(sz)
	if w == 0 || h == 0 {
		return fmt.Errorf("transform: unsupported tx size %d", sz)
	}
	if len(coeffs) != w*h {
		return fmt.Errorf("transform: coeffs size %d, want %d (%dx%d)", len(coeffs), w*h, w, h)
	}
	rowOp := RowOp(ty, sz)
	colOp := ColOp(ty, sz)
	if rowOp == nil || colOp == nil {
		return fmt.Errorf("transform: (ty=%d, sz=%d) not implemented", ty, sz)
	}

	// Row pass.
	row := make([]int32, w)
	for r := 0; r < h; r++ {
		copy(row, coeffs[r*w:(r+1)*w])
		rowOp(row)
		copy(coeffs[r*w:(r+1)*w], row)
	}

	// Column pass.
	col := make([]int32, h)
	for c := 0; c < w; c++ {
		for r := 0; r < h; r++ {
			col[r] = coeffs[r*w+c]
		}
		colOp(col)
		for r := 0; r < h; r++ {
			coeffs[r*w+c] = col[r]
		}
	}
	return nil
}
