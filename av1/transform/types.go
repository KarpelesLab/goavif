package transform

// TxType identifies one of AV1's separable transform type pairs (spec
// §6.8.21). The row transform and column transform can be chosen
// independently from {DCT, ADST, FLIPADST, IDTX}, giving 16 combinations
// plus the 16 "reduced set" variants for the smaller subset.
type TxType uint8

const (
	DctDct          TxType = 0
	AdstDct         TxType = 1
	DctAdst         TxType = 2
	AdstAdst        TxType = 3
	FlipadstDct     TxType = 4
	DctFlipadst     TxType = 5
	FlipadstFlipadst TxType = 6
	AdstFlipadst    TxType = 7
	FlipadstAdst    TxType = 8
	IdtxIdtx        TxType = 9
	VDct            TxType = 10
	HDct            TxType = 11
	VAdst           TxType = 12
	HAdst           TxType = 13
	VFlipadst       TxType = 14
	HFlipadst       TxType = 15
)

// TxSize identifies a transform block size. Values are encoded as log2 of
// width in the low 3 bits and log2 of height in the high 3 bits.
type TxSize uint8

const (
	Tx4x4 TxSize = iota
	Tx8x8
	Tx16x16
	Tx32x32
	Tx64x64
	Tx4x8
	Tx8x4
	Tx8x16
	Tx16x8
	Tx16x32
	Tx32x16
	Tx32x64
	Tx64x32
	Tx4x16
	Tx16x4
	Tx8x32
	Tx32x8
	Tx16x64
	Tx64x16
)

// Dim1D is the callable form of a 1D inverse transform.
type Dim1D func([]int32)

// RowOp returns the row-direction 1D inverse transform for a given type
// and transform size. Only the 4-point variants are currently implemented;
// callers receive nil for unsupported sizes.
func RowOp(ty TxType, sz TxSize) Dim1D {
	n := rowLen(sz)
	switch n {
	case 4:
		switch rowKind(ty) {
		case kindDCT:
			return IDCT4
		case kindADST:
			return IADST4
		case kindFLIPADST:
			return IFLIPADST4
		case kindIDTX:
			return IDTX4
		}
	case 8:
		switch rowKind(ty) {
		case kindDCT:
			return IDCT8
		case kindADST:
			return IADST8
		case kindFLIPADST:
			return IFLIPADST8
		case kindIDTX:
			return IDTX8
		}
	case 16:
		switch rowKind(ty) {
		case kindDCT:
			return IDCT16
		case kindIDTX:
			return IDTX16
		}
	case 32:
		switch rowKind(ty) {
		case kindDCT:
			return IDCT32
		case kindIDTX:
			return IDTX32
		}
	}
	return nil
}

// ColOp returns the column-direction 1D inverse transform.
func ColOp(ty TxType, sz TxSize) Dim1D {
	n := colLen(sz)
	switch n {
	case 4:
		switch colKind(ty) {
		case kindDCT:
			return IDCT4
		case kindADST:
			return IADST4
		case kindFLIPADST:
			return IFLIPADST4
		case kindIDTX:
			return IDTX4
		}
	case 8:
		switch colKind(ty) {
		case kindDCT:
			return IDCT8
		case kindIDTX:
			return IDTX8
		}
	case 16:
		switch colKind(ty) {
		case kindDCT:
			return IDCT16
		case kindIDTX:
			return IDTX16
		}
	case 32:
		switch colKind(ty) {
		case kindDCT:
			return IDCT32
		case kindIDTX:
			return IDTX32
		}
	}
	return nil
}

type kind uint8

const (
	kindDCT kind = iota
	kindADST
	kindFLIPADST
	kindIDTX
)

// rowKind returns the transform kind used for the row direction of ty.
// The spec names AdstDct as "row=DCT, col=ADST" (first word is column).
func rowKind(ty TxType) kind {
	switch ty {
	case DctDct, AdstDct, FlipadstDct, VDct:
		return kindDCT
	case DctAdst, AdstAdst, FlipadstAdst, VAdst:
		return kindADST
	case DctFlipadst, AdstFlipadst, FlipadstFlipadst, VFlipadst:
		return kindFLIPADST
	case IdtxIdtx, HDct, HAdst, HFlipadst:
		return kindIDTX
	}
	return kindDCT
}

func colKind(ty TxType) kind {
	switch ty {
	case DctDct, DctAdst, DctFlipadst, HDct:
		return kindDCT
	case AdstDct, AdstAdst, AdstFlipadst, HAdst:
		return kindADST
	case FlipadstDct, FlipadstAdst, FlipadstFlipadst, HFlipadst:
		return kindFLIPADST
	case IdtxIdtx, VDct, VAdst, VFlipadst:
		return kindIDTX
	}
	return kindDCT
}

// TxSizeWidth returns the width in samples for a transform size.
func TxSizeWidth(sz TxSize) int { return rowLen(sz) }

// TxSizeHeight returns the height in samples for a transform size.
func TxSizeHeight(sz TxSize) int { return colLen(sz) }

// rowLen and colLen return the transform's 1D dimensions in samples.
func rowLen(sz TxSize) int {
	switch sz {
	case Tx4x4, Tx4x8, Tx4x16:
		return 4
	case Tx8x8, Tx8x4, Tx8x16, Tx8x32:
		return 8
	case Tx16x16, Tx16x8, Tx16x32, Tx16x4, Tx16x64:
		return 16
	case Tx32x32, Tx32x16, Tx32x64, Tx32x8:
		return 32
	case Tx64x64, Tx64x32, Tx64x16:
		return 64
	}
	return 0
}

func colLen(sz TxSize) int {
	switch sz {
	case Tx4x4, Tx8x4, Tx16x4:
		return 4
	case Tx8x8, Tx4x8, Tx16x8, Tx32x8:
		return 8
	case Tx16x16, Tx8x16, Tx32x16, Tx4x16, Tx64x16:
		return 16
	case Tx32x32, Tx16x32, Tx64x32, Tx8x32:
		return 32
	case Tx64x64, Tx32x64, Tx16x64:
		return 64
	}
	return 0
}
