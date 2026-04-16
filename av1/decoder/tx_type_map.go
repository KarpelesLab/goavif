package decoder

import "github.com/KarpelesLab/goavif/av1/transform"

// IntraTxTypeFor maps a raw tx_type index decoded via ReadIntraTxType
// into the spec's TxType enum. The mapping is fixed by the two intra
// extended-tx sets (spec §6.10.15):
//
//	EXT_TX_SET_INTRA_1 (txSet=1, 7 types):
//	  0 → DCT_DCT, 1 → ADST_DCT, 2 → DCT_ADST, 3 → ADST_ADST,
//	  4 → IDTX, 5 → V_DCT, 6 → H_DCT
//
//	EXT_TX_SET_INTRA_2 (txSet=2, 5 types):
//	  0 → DCT_DCT, 1 → ADST_DCT, 2 → DCT_ADST, 3 → ADST_ADST, 4 → IDTX
//
// txSet=0 always implies DCT_DCT (no signaling).
func IntraTxTypeFor(txSet, raw int) transform.TxType {
	switch txSet {
	case 1:
		switch raw {
		case 0:
			return transform.DctDct
		case 1:
			return transform.AdstDct
		case 2:
			return transform.DctAdst
		case 3:
			return transform.AdstAdst
		case 4:
			return transform.IdtxIdtx
		case 5:
			return transform.VDct
		case 6:
			return transform.HDct
		}
	case 2:
		switch raw {
		case 0:
			return transform.DctDct
		case 1:
			return transform.AdstDct
		case 2:
			return transform.DctAdst
		case 3:
			return transform.AdstAdst
		case 4:
			return transform.IdtxIdtx
		}
	}
	return transform.DctDct
}

// ExtTxSetForIntra picks the intra-frame extended-tx set per spec
// §6.10.15. Returns 0 if tx_type is implicit DCT_DCT, 1 for the 7-type
// set (smaller blocks), 2 for the 5-type set (larger blocks).
func ExtTxSetForIntra(txW, txH int) int {
	area := txW * txH
	switch {
	case area <= 16*16:
		return 1
	case area <= 32*32:
		return 2
	}
	return 0 // implicit DCT_DCT for TX > 32×32
}

// ExtTxSizeCtx returns the 4-way size context used to index the intra
// ext_tx CDFs: TX_4X4=0, TX_8X8=1, TX_16X16=2, TX_32X32=3.
// Non-square sizes map to the square equivalent by area.
func ExtTxSizeCtx(txW, txH int) int {
	area := txW * txH
	switch {
	case area <= 4*4:
		return 0
	case area <= 8*8:
		return 1
	case area <= 16*16:
		return 2
	}
	return 3
}
