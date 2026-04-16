// Package encoder assembles AV1 bitstreams for the goavif encoder. It
// pairs with [av1/decoder] at the syntax level: whatever encoder
// produces should be directly consumable by decoder.Decode.
//
// The encoder emits PARTITION_NONE + DC_PRED blocks. When a luma
// plane is provided, the encoder computes the DC residual (actual
// mean vs DC prediction) and emits quantized coefficients via
// WriteCoefficients. When no luma data is given, all blocks are
// skip=1 (constant mid-grey). Chroma is always skip=1 for now.
package encoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// WriteIntraOnlyTile emits a tile payload for an intra-only keyframe
// of dimension (width, height). Every superblock is encoded as a
// single 64×64 PARTITION_NONE block with DC_PRED y-mode and DC_PRED
// uv-mode.
//
// When lumaY is non-nil (length w*h, row-major, stride=width), the
// encoder computes the DC residual against DC_PRED for each block
// and emits quantized coefficients. When nil, all blocks are skip=1.
//
// Chroma is always skip=1.
func WriteIntraOnlyTile(width, height int, fh *obu.FrameHeader, sh *obu.SequenceHeader, lumaY []uint8) ([]byte, error) {
	if sh == nil || fh == nil {
		return nil, fmt.Errorf("encoder: nil sh / fh")
	}

	var enc entropy.Encoder
	enc.Init(!fh.DisableCDFUpdate)

	sbSize := 64
	if sh.Use128x128Superblock {
		sbSize = 128
	}
	baseQ := int(fh.Quant.BaseQIndex)
	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			if err := writeSuperblock(&enc, x, y, sbSize, width, height, sh, lumaY, baseQ); err != nil {
				return nil, err
			}
		}
	}
	return enc.Finish(), nil
}

// writeSuperblock emits the syntax for a single SB using PARTITION_NONE
// at the top with DC_PRED.
func writeSuperblock(enc *entropy.Encoder, x, y, sbSize, frameW, frameH int, sh *obu.SequenceHeader, lumaY []uint8, baseQ int) error {
	writePartitionNone(enc, sbSize)
	bw := sbSize
	bh := sbSize
	if x+bw > frameW {
		bw = frameW - x
	}
	if y+bh > frameH {
		bh = frameH - y
	}
	if bw <= 0 || bh <= 0 {
		return nil
	}
	writeDCLeaf(enc, sh, x, y, bw, bh, frameW, lumaY, baseQ)
	return nil
}

func writePartitionNone(enc *entropy.Encoder, bs int) {
	// bsl bucket per decoder's blockSizeLog: 3 = 64x64, 4 = 128x128.
	// decoder.decodePartitionNode computes cdfIdx = bsl*4 + ctx.
	bsl := 3 // 64x64
	if bs == 128 {
		bsl = 4
	}
	ctx := 0 // above/left = 0 at SB start (decoder uses same)
	cdfIdx := bsl*4 + ctx
	if cdfIdx >= len(cdfs.DefaultPartitionCDF) {
		return
	}
	// Use a local CDF copy so update behavior doesn't leak into the
	// (shared) default.
	cdf := append(cdfs.CDF(nil), cdfs.DefaultPartitionCDF[cdfIdx]...)
	// PARTITION_NONE = symbol 0.
	enc.EncodeSymbol(cdf, 0)
}

// writeDCLeaf emits the mode + coefficient syntax for a single leaf
// block with DC_PRED for both Y and UV. When lumaY is non-nil the
// luma block emits DC residual coefficients; otherwise it's all-skip.
// Chroma is always skip.
func writeDCLeaf(enc *entropy.Encoder, sh *obu.SequenceHeader, bx, by, bw, bh, frameW int, lumaY []uint8, baseQ int) {
	// Y intra mode = DC_PRED = 0.
	kfCDF := append(cdfs.CDF(nil), cdfs.DefaultKfYModeCDF[0][0]...)
	enc.EncodeSymbol(kfCDF, 0)

	// Compute luma residual if source data is available.
	var hasResidual bool
	var coeffs []int32
	var scan []int
	var nzMap []int8
	var txSizeIdx int

	if lumaY != nil && bw > 0 && bh > 0 {
		// Compute mean luma of the source block.
		var sum int
		for r := 0; r < bh; r++ {
			for c := 0; c < bw; c++ {
				sum += int(lumaY[(by+r)*frameW+(bx+c)])
			}
		}
		mean := sum / (bw * bh)
		pred := 128 // DC_PRED at frame corner is half-range
		residual := mean - pred

		// Forward-transform the residual. For a block where all
		// spatial samples have the same residual value, we just need
		// the DC coefficient of the FDCT. We simulate this by filling
		// a 1D row of length bw with the residual, applying FDCT,
		// and taking index 0.
		txW := bw
		if txW > 64 {
			txW = 64
		}
		txH := bh
		if txH > 64 {
			txH = 64
		}
		row := make([]int32, txW)
		for i := range row {
			row[i] = int32(residual)
		}
		switch txW {
		case 4:
			transform.FDCT4(row)
		case 8:
			transform.FDCT8(row)
		case 16:
			transform.FDCT16(row)
		case 32:
			transform.FDCT32(row)
		case 64:
			transform.FDCT64(row)
		}
		dc := row[0]

		// Quantize.
		qp := quant.Params{BaseQIndex: baseQ, BitDepth: 8}
		qv := qp.Compute(quant.PlaneY)
		qdc := quant.QuantizeCoeff(dc, 0, qv)

		if qdc != 0 {
			hasResidual = true
			numCoeffs := txW * txH
			coeffs = make([]int32, numCoeffs)
			coeffs[0] = qdc
			scan = transform.DefaultZigzagScan(txW, txH)
			nzMap = nzMapForSize(txW, txH)
			txSizeIdx = txSizeIdxFor(txW, txH)
		}
	}

	if hasResidual {
		// skip = 0
		skipCDF := append(cdfs.CDF(nil), cdfs.DefaultSkipCDF[0]...)
		enc.EncodeSymbol(skipCDF, 0)
	} else {
		// skip = 1
		skipCDF := append(cdfs.CDF(nil), cdfs.DefaultSkipCDF[0]...)
		enc.EncodeSymbol(skipCDF, 1)
	}

	// UV mode = DC_PRED = 0.
	if !sh.Color.Monochrome {
		uvCDF := append(cdfs.CDF(nil), cdfs.DefaultUVModeCDF[1][0]...)
		enc.EncodeSymbol(uvCDF, 0)
	}

	if hasResidual {
		// Luma coefficients.
		WriteCoefficients(enc, coeffs, txSizeIdx, 0 /*luma*/, scan, nzMap, len(scan)/len(scan)*txWidthOf(scan), txHeightOf(scan, len(scan)))
		// Chroma: skip both U and V.
		if !sh.Color.Monochrome {
			cw := len(scan) // placeholder — chroma uses smaller TX
			_ = cw
			// Emit txb_skip=1 for U and V.
			for plane := 0; plane < 2; plane++ {
				chromaTxIdx := txSizeIdx
				if chromaTxIdx > 4 {
					chromaTxIdx = 4
				}
				txbCDF := append(cdfs.CDF(nil), cdfs.DefaultTxbSkipCDF[clamp(chromaTxIdx, 0, 4)][0]...)
				enc.EncodeSymbol(txbCDF, 1) // skip
			}
		}
	}
}

func nzMapForSize(w, h int) []int8 {
	switch {
	case w == 4 && h == 4:
		return cdfs.NzMapCtxOffset4x4[:]
	case w == 8 && h == 8:
		return cdfs.NzMapCtxOffset8x8[:]
	case w == 16 && h == 16:
		return cdfs.NzMapCtxOffset16x16[:]
	case w == 32 && h == 32:
		return cdfs.NzMapCtxOffset32x32[:]
	default:
		return cdfs.NzMapCtxOffset32x32[:]
	}
}

func txSizeIdxFor(w, h int) int {
	switch {
	case w <= 4 && h <= 4:
		return 0
	case w <= 8 && h <= 8:
		return 1
	case w <= 16 && h <= 16:
		return 2
	case w <= 32 && h <= 32:
		return 3
	default:
		return 4
	}
}

func txWidthOf(scan []int) int {
	// Derive from scan length assuming square.
	n := len(scan)
	switch {
	case n <= 16:
		return 4
	case n <= 64:
		return 8
	case n <= 256:
		return 16
	case n <= 1024:
		return 32
	default:
		return 64
	}
}

func txHeightOf(scan []int, n int) int {
	return n / txWidthOf(scan)
}
