package decoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// CoeffDecoder reads transform coefficients from a tile's entropy-coded
// bitstream. It wraps the per-tile [entropy.Decoder] and holds mutable
// copies of the coefficient CDFs.
type CoeffDecoder struct {
	dec *entropy.Decoder

	// Mutable CDF copies per tile.
	txbSkipCDF          [5][13]cdfs.CDF
	eobMulti16CDF       [2][2]cdfs.CDF
	eobMulti32CDF       [2][2]cdfs.CDF
	eobMulti64CDF       [2][2]cdfs.CDF
	coeffBaseEOBMultiCDF [5][2][4]cdfs.CDF
	dcSignCDF            [2][3]cdfs.CDF
}

// InitCoeffDecoder populates a CoeffDecoder from the defaults, using Q
// context qCtx (0..3) where available.
func InitCoeffDecoder(dec *entropy.Decoder, qCtx int) *CoeffDecoder {
	if qCtx < 0 {
		qCtx = 0
	}
	if qCtx > 3 {
		qCtx = 3
	}
	cd := &CoeffDecoder{dec: dec}
	// txb_skip — only Q=0 available currently; degrade gracefully.
	for tx := range cdfs.DefaultTxbSkipCDF {
		for ctx := range cdfs.DefaultTxbSkipCDF[tx] {
			cd.txbSkipCDF[tx][ctx] = append(cdfs.CDF(nil), cdfs.DefaultTxbSkipCDF[tx][ctx]...)
		}
	}
	// eob_multi
	for p := 0; p < 2; p++ {
		for c := 0; c < 2; c++ {
			cd.eobMulti16CDF[p][c] = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti16CDF[qCtx][p][c]...)
			cd.eobMulti32CDF[p][c] = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti32CDF[qCtx][p][c]...)
			cd.eobMulti64CDF[p][c] = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti64CDF[qCtx][p][c]...)
		}
	}
	// coeff_base_eob — Q=0 only
	for tx := range cdfs.DefaultCoeffBaseEOBMultiCDF {
		for p := 0; p < 2; p++ {
			for ctx := 0; ctx < 4; ctx++ {
				cd.coeffBaseEOBMultiCDF[tx][p][ctx] = append(cdfs.CDF(nil), cdfs.DefaultCoeffBaseEOBMultiCDF[tx][p][ctx]...)
			}
		}
	}
	// dc_sign
	for p := 0; p < 2; p++ {
		for ctx := 0; ctx < 3; ctx++ {
			cd.dcSignCDF[p][ctx] = append(cdfs.CDF(nil), cdfs.DefaultDCSignCDF[p][ctx]...)
		}
	}
	return cd
}

// ReadTXBSkip reads the txb_skip flag for a transform block.
// txSizeIdx is 0..4 (TX_4X4..TX_64X64). ctx is 0..12.
func (cd *CoeffDecoder) ReadTXBSkip(txSizeIdx, ctx int) bool {
	return cd.dec.DecodeSymbol(cd.txbSkipCDF[txSizeIdx][ctx]) != 0
}

// ReadEOB reads the end-of-block position for a transform block of the
// given coefficient count (16, 32, or 64). Returns the 0-based EOB
// position (last non-zero coefficient index in scan order).
//
// Only 16/32/64-coefficient blocks are supported; larger blocks return 0.
func (cd *CoeffDecoder) ReadEOB(numCoeffs, planeType, eobCtx int) int {
	var eobPt int
	switch numCoeffs {
	case 16:
		eobPt = cd.dec.DecodeSymbol(cd.eobMulti16CDF[planeType][eobCtx])
	case 32:
		eobPt = cd.dec.DecodeSymbol(cd.eobMulti32CDF[planeType][eobCtx])
	case 64:
		eobPt = cd.dec.DecodeSymbol(cd.eobMulti64CDF[planeType][eobCtx])
	default:
		return 0
	}
	// Convert eob_pt to an actual coefficient count.
	// eob_pt encodes the position in a log-scale:
	//   0 → eob=1, 1 → eob=2, 2 → eob=3..4, 3 → eob=5..8, etc.
	eob := eobPtToEOB(eobPt)
	return eob
}

// eobPtToEOB converts the eob_pt symbol to the 1-based eob count. For
// eob_pt >= 2, extra bits would be read from the bitstream; this
// simplified version returns the midpoint of the bin as a placeholder.
func eobPtToEOB(pt int) int {
	switch pt {
	case 0:
		return 1
	case 1:
		return 2
	case 2:
		return 3 // 3..4 → midpoint
	case 3:
		return 6 // 5..8 → midpoint
	case 4:
		return 12 // 9..16 → midpoint
	case 5:
		return 24 // 17..32 → midpoint
	case 6:
		return 48 // 33..64 → midpoint
	}
	return 1
}

// ReadCoefficients decodes transform coefficients for a single block,
// returning the dequantized coefficients in scan order. Only the eob
// position's base level is decoded; positions before eob are NOT yet
// decoded (returns ErrCoeffDecodeUnimplemented).
//
// This is a partial implementation sufficient for single-coefficient-per-
// block images (DC-only); the full coefficient decoder needs the
// coeff_base_multi CDFs which haven't been transcribed yet.
func (cd *CoeffDecoder) ReadCoefficients(
	txSizeIdx, planeType int,
	numCoeffs int,
	scan []int,
) ([]int32, error) {
	coeffs := make([]int32, numCoeffs)
	txbSkipCtx := 0
	if cd.ReadTXBSkip(txSizeIdx, txbSkipCtx) {
		return coeffs, nil // all zero
	}
	eobCtx := 0
	eob := cd.ReadEOB(numCoeffs, planeType, eobCtx)
	if eob < 1 || eob > numCoeffs {
		eob = 1
	}

	// Read base level at eob position.
	eobBaseCtx := 0
	if eob > 2 {
		eobBaseCtx = 1
	}
	if eob > 5 {
		eobBaseCtx = 2
	}
	if eob > 10 {
		eobBaseCtx = 3
	}
	baseLevel := cd.dec.DecodeSymbol(cd.coeffBaseEOBMultiCDF[txSizeIdx][planeType][eobBaseCtx])
	level := baseLevel + 1 // eob position always has level >= 1

	// TODO: if baseLevel >= 2, read additional base_range levels
	// TODO: read sign for the eob coefficient

	if eob-1 < len(scan) {
		coeffs[scan[eob-1]] = int32(level)
	}

	// For positions before eob: would need coeff_base_multi CDFs.
	if eob > 1 {
		return coeffs, fmt.Errorf("%w: multi-coefficient blocks need coeff_base_multi CDFs",
			ErrCoeffDecodeUnimplemented)
	}
	return coeffs, nil
}

// txSizeToNumCoeffs returns the number of coefficients for a transform
// block.
func txSizeToNumCoeffs(sz transform.TxSize) int {
	w := transform.TxSizeWidth(sz)
	h := transform.TxSizeHeight(sz)
	if w == 0 || h == 0 {
		return 0
	}
	return w * h
}
