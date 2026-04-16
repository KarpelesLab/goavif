package encoder

import (
	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
)

// WriteCoefficients emits the range-coded coefficient syntax for one
// transform block. This is the encoder counterpart of
// [decoder.CoeffDecoder.ReadCoefficients].
//
// coeffs is the quantized coefficient array in row-major layout (w*h).
// scan maps scan-position indices to block positions. nzMapOffset is
// the per-position context offset table. txSizeIdx and planeType
// (0=luma, 1=chroma) select CDF families.
//
// If all coefficients are zero, txb_skip=1 is emitted and nothing
// else follows. Otherwise the full level + sign syntax is emitted.
func WriteCoefficients(
	enc *entropy.Encoder,
	coeffs []int32,
	txSizeIdx, planeType int,
	scan []int,
	nzMapOffset []int8,
	w, h int,
) {
	// Determine EOB: the 1-based index (in scan order) of the last
	// non-zero coefficient.
	eob := 0
	for i := len(scan) - 1; i >= 0; i-- {
		pos := scan[i]
		if pos < w*h && coeffs[pos] != 0 {
			eob = i + 1
			break
		}
	}

	// txb_skip CDF.
	txbSkipCDF := append(cdfs.CDF(nil), cdfs.DefaultTxbSkipCDF[clamp(txSizeIdx, 0, 4)][0]...)
	if eob == 0 {
		enc.EncodeSymbol(txbSkipCDF, 1) // skip = true
		return
	}
	enc.EncodeSymbol(txbSkipCDF, 0) // skip = false

	// EOB: encode eob_pt + optional eob_extra.
	writeEOB(enc, eob, len(scan), txSizeIdx, planeType)

	// Levels in REVERSE scan order (from eob-1 down to 0).
	absLevels := make([]int8, w*h)
	for i := eob - 1; i >= 0; i-- {
		pos := scan[i]
		level := abs32(coeffs[pos])
		r := pos / w
		c := pos % w

		if i == eob-1 {
			// EOB position: emit base level via coeff_base_eob_multi.
			eobBaseCtx := 0
			switch {
			case eob > 10:
				eobBaseCtx = 3
			case eob > 5:
				eobBaseCtx = 2
			case eob > 2:
				eobBaseCtx = 1
			}
			baseSymbol := int(level) - 1
			if baseSymbol > 2 {
				baseSymbol = 2
			}
			if baseSymbol < 0 {
				baseSymbol = 0
			}
			cdf := append(cdfs.CDF(nil),
				cdfs.DefaultCoeffBaseEOBMultiCDF[clamp(txSizeIdx, 0, 4)][planeType][clamp(eobBaseCtx, 0, 3)]...)
			enc.EncodeSymbol(cdf, baseSymbol)
		} else {
			// Non-EOB: emit base level via coeff_base_multi.
			sigCtx := decoder.SigCoefCtx2D(r, c, w, h, absLevels, nzMapOffset, i)
			baseSymbol := int(level)
			if baseSymbol > 3 {
				baseSymbol = 3
			}
			cdf := append(cdfs.CDF(nil),
				cdfs.DefaultCoeffBaseMultiCDF[0][clamp(txSizeIdx, 0, 4)][planeType][clamp(sigCtx, 0, 41)]...)
			enc.EncodeSymbol(cdf, baseSymbol)
		}

		// BR levels if base saturated at NUM_BASE_LEVELS+1 (= 3).
		effLevel := int(level)
		baseForBR := effLevel
		if i == eob-1 {
			baseForBR = int(level)
			if baseForBR > 3 {
				baseForBR = 3
			}
		} else {
			if baseForBR > 3 {
				baseForBR = 3
			}
		}
		if baseForBR == 3 {
			remaining := effLevel - 3
			for br := 0; br < 4; br++ {
				brCtx := decoder.LevelCtx(r, c, w, h, absLevels)
				inc := remaining
				if inc > 3 {
					inc = 3
				}
				cdf := append(cdfs.CDF(nil),
					cdfs.DefaultCoeffBrMultiCDF[0][clamp(txSizeIdx, 0, 4)][planeType][clamp(brCtx, 0, 20)]...)
				enc.EncodeSymbol(cdf, inc)
				remaining -= inc
				if inc < 3 {
					break
				}
			}
		}

		absLevels[pos] = int8(min32(int(level), 127))
	}

	// Signs — DC via dc_sign CDF, AC via uniform 50/50 bits.
	if coeffs[0] != 0 {
		sign := 0
		if coeffs[0] < 0 {
			sign = 1
		}
		dcCDF := append(cdfs.CDF(nil), cdfs.DefaultDCSignCDF[planeType][0]...)
		enc.EncodeSymbol(dcCDF, sign)
	}
	for i := 1; i < eob; i++ {
		pos := scan[i]
		if coeffs[pos] != 0 {
			sign := uint32(0)
			if coeffs[pos] < 0 {
				sign = 1
			}
			enc.EncodeBool(sign, 16384)
		}
	}
}

func abs32(x int32) int32 {
	if x < 0 {
		return -x
	}
	return x
}

func min32(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// writeEOB emits the eob_pt symbol + optional eob_extra bits for the
// given 1-based eob position. Mirrors ReadEOB in coeffs.go.
func writeEOB(enc *entropy.Encoder, eob, numCoeffs, txSizeIdx, planeType int) {
	pt, extra := eobToEOBPt(eob)
	// EOB multi CDF: select by numCoeffs bucket.
	var cdf cdfs.CDF
	switch {
	case numCoeffs <= 16:
		cdf = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti16CDF[0][planeType][0]...)
	case numCoeffs <= 32:
		cdf = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti32CDF[0][planeType][0]...)
	case numCoeffs <= 64:
		cdf = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti64CDF[0][planeType][0]...)
	case numCoeffs <= 128:
		cdf = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti128CDF[0][planeType][0]...)
	case numCoeffs <= 256:
		cdf = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti256CDF[0][planeType][0]...)
	case numCoeffs <= 512:
		cdf = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti512CDF[0][planeType][0]...)
	default:
		cdf = append(cdfs.CDF(nil), cdfs.DefaultEOBMulti1024CDF[0][planeType][0]...)
	}
	enc.EncodeSymbol(cdf, pt)

	if extra == 0 {
		return
	}
	// eob_extra: first bit from CDF, rest as bypass.
	_, binStart := decoder.EOBPtToEOB(pt)
	offset := eob - binStart
	highBit := (offset >> (extra - 1)) & 1
	eobCoefCtx := pt - 2
	if eobCoefCtx >= 9 {
		eobCoefCtx = 8
	}
	extraCDF := append(cdfs.CDF(nil),
		cdfs.DefaultEOBExtraCDF[0][clamp(txSizeIdx, 0, 4)][planeType][clamp(eobCoefCtx, 0, 8)]...)
	enc.EncodeSymbol(extraCDF, highBit)
	// Remaining extra bits: bypass.
	for b := extra - 2; b >= 0; b-- {
		enc.EncodeBool(uint32((offset>>uint(b))&1), 16384)
	}
}

// eobToEOBPt maps a 1-based eob position to the eob_pt symbol index +
// extra-bit count. Inverse of decoder.EOBPtToEOB.
func eobToEOBPt(eob int) (pt, extra int) {
	switch {
	case eob <= 1:
		return 0, 0
	case eob <= 2:
		return 1, 0
	case eob <= 4:
		return 2, 1
	case eob <= 8:
		return 3, 2
	case eob <= 16:
		return 4, 3
	case eob <= 32:
		return 5, 4
	case eob <= 64:
		return 6, 5
	case eob <= 128:
		return 7, 6
	case eob <= 256:
		return 8, 7
	case eob <= 512:
		return 9, 8
	default:
		return 10, 9
	}
}
