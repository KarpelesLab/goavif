package transform

import "fmt"

// Forward2D runs the AV1 2D forward transform (encoder direction): row
// pass first, then column pass. coeffs initially contains the w*h spatial
// residual samples in row-major layout; on return it contains the
// transform coefficients in the same layout.
//
// Only the DCT_DCT row/column pair is implemented today — that is the
// only tx_type the current encoder emits. Other pairs return an error.
//
// Round-trip contract: Forward2D followed by Inverse2D reproduces the
// input within a few ulps per coefficient (inherited from the per-1D
// FDCT/IDCT pairs in fdct_test.go).
func Forward2D(coeffs []int32, ty TxType, sz TxSize) error {
	w := rowLen(sz)
	h := colLen(sz)
	if w == 0 || h == 0 {
		return fmt.Errorf("transform: unsupported tx size %d", sz)
	}
	if len(coeffs) != w*h {
		return fmt.Errorf("transform: coeffs size %d, want %d (%dx%d)", len(coeffs), w*h, w, h)
	}
	rowOp := forwardRowOp(ty, sz)
	colOp := forwardColOp(ty, sz)
	if rowOp == nil || colOp == nil {
		return fmt.Errorf("transform: Forward2D (ty=%d, sz=%d) not implemented", ty, sz)
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

// forwardRowOp returns the forward 1D transform for the row direction.
// Only DCT is wired up — other kinds return nil.
func forwardRowOp(ty TxType, sz TxSize) Dim1D {
	n := rowLen(sz)
	kind := rowKind(ty)
	switch kind {
	case kindDCT:
		switch n {
		case 4:
			return FDCT4
		case 8:
			return FDCT8
		case 16:
			return FDCT16
		case 32:
			return FDCT32
		case 64:
			return FDCT64
		}
	case kindIDTX:
		switch n {
		case 4:
			return FIdentity4
		case 8:
			return FIdentity8
		case 16:
			return FIdentity16
		case 32:
			return FIdentity32
		}
	}
	return nil
}

// forwardColOp returns the forward 1D transform for the column direction.
func forwardColOp(ty TxType, sz TxSize) Dim1D {
	n := colLen(sz)
	kind := colKind(ty)
	switch kind {
	case kindDCT:
		switch n {
		case 4:
			return FDCT4
		case 8:
			return FDCT8
		case 16:
			return FDCT16
		case 32:
			return FDCT32
		case 64:
			return FDCT64
		}
	case kindIDTX:
		switch n {
		case 4:
			return FIdentity4
		case 8:
			return FIdentity8
		case 16:
			return FIdentity16
		case 32:
			return FIdentity32
		}
	}
	return nil
}
