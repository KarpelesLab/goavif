package encoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/av1/predict"
	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// WriteInterMETile16 is the HBD counterpart of [WriteInterMETile]:
// runs per-block motion estimation on uint16 src/ref planes and emits
// the inter tile-group payload.
func WriteInterMETile16(width, height int,
	fh *obu.FrameHeader, sh *obu.SequenceHeader,
	srcY, srcU, srcV []uint16,
	refY, refU, refV []uint16,
	refW, refH int,
	searchRange int,
) ([]byte, error) {
	if sh == nil || fh == nil {
		return nil, fmt.Errorf("encoder: nil sh / fh")
	}
	if width%64 != 0 || height%64 != 0 {
		return nil, fmt.Errorf("encoder: WriteInterMETile16 requires 64-aligned dims, got %dx%d", width, height)
	}
	var enc entropy.Encoder
	enc.Init(!fh.DisableCDFUpdate)

	sbSize := 64
	baseQ := int(fh.Quant.BaseQIndex)
	bitDepth := int(sh.Color.BitDepth)
	if bitDepth < 8 {
		bitDepth = 8
	}
	subX := int(sh.Color.SubsamplingX)
	subY := int(sh.Color.SubsamplingY)
	miCols := (width + 3) >> 2
	miRows := (height + 3) >> 2
	inter := make([]uint8, miCols*miRows)
	refYStride := refW
	refCStride := refW >> uint(subX)
	srcYStride := width

	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			writePartitionSymbol(&enc, 3, 0, 3)
			for _, off := range [4][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
				bx := x + off[0]
				by := y + off[1]
				encodeInter32_16(&enc, bx, by,
					srcY, srcU, srcV, srcYStride,
					refY, refU, refV, refW, refH, refYStride, refCStride,
					inter, miCols, miRows, baseQ, bitDepth, searchRange, subX, subY)
			}
		}
	}
	return enc.Finish(), nil
}

// encodeInter32_16 handles one 32×32 inter block at HBD. Mirrors
// encodeInter32 but for uint16 planes.
func encodeInter32_16(enc *entropy.Encoder, bx, by int,
	srcY, srcU, srcV []uint16, srcYStride int,
	refY, refU, refV []uint16,
	refW, refH, refYStride, refCStride int,
	inter []uint8, miCols, miRows, baseQ, bitDepth, searchRange int,
	subX, subY int,
) {
	mv := DiamondSearchMV16(srcY, srcYStride, bx, by, 32, 32,
		refY, refW, refH, refYStride, searchRange)
	mv = SubPelRefineMV16(srcY, srcYStride, bx, by, 32, 32,
		refY, refW, refH, refYStride, mv, bitDepth)
	sad32 := sadForMV16(srcY, srcYStride, bx, by, 32, 32,
		refY, refW, refH, refYStride, mv, bitDepth)
	// HBD samples are wider so the per-pixel threshold scales.
	splitThreshold := 32 * 32 * (12 << uint(bitDepth-8))
	if sad32 <= splitThreshold {
		writePartitionSymbol(enc, 2, 0, 0)
		writeInterResidualBlock16(enc, bx, by, 32, 32, mv,
			srcY, srcU, srcV,
			refY, refU, refV, refW, refH, refYStride, refCStride,
			inter, miCols, miRows, baseQ, bitDepth, subX, subY)
		return
	}
	writePartitionSymbol(enc, 2, 0, 3)
	for _, sub := range [4][2]int{{0, 0}, {16, 0}, {0, 16}, {16, 16}} {
		sx := bx + sub[0]
		sy := by + sub[1]
		mv16 := DiamondSearchMV16(srcY, srcYStride, sx, sy, 16, 16,
			refY, refW, refH, refYStride, searchRange)
		mv16 = SubPelRefineMV16(srcY, srcYStride, sx, sy, 16, 16,
			refY, refW, refH, refYStride, mv16, bitDepth)
		writePartitionSymbol(enc, 1, 0, 0)
		writeInterResidualBlock16(enc, sx, sy, 16, 16, mv16,
			srcY, srcU, srcV,
			refY, refU, refV, refW, refH, refYStride, refCStride,
			inter, miCols, miRows, baseQ, bitDepth, subX, subY)
	}
}

// writeInterResidualBlock16 is the HBD counterpart of
// writeInterResidualBlock: emits inter block symbols + residual using
// uint16 planes.
func writeInterResidualBlock16(enc *entropy.Encoder,
	bx, by, bw, bh int, mv decoder.MV,
	srcY, srcU, srcV []uint16,
	refY, refU, refV []uint16,
	refW, refH, refYStride, refCStride int,
	inter []uint8, miCols, miRows int,
	baseQ, bitDepth, subX, subY int,
) {
	miCol := bx >> 2
	miRow := by >> 2
	aboveIsInter := miRow > 0 && inter[(miRow-1)*miCols+miCol] != 0
	leftIsInter := miCol > 0 && inter[miRow*miCols+(miCol-1)] != 0
	ctx := 0
	if aboveIsInter && leftIsInter {
		ctx = 3
	} else if aboveIsInter || leftIsInter {
		ctx = 1
	}
	enc.EncodeSymbol(cdfs.DefaultIsInterCDF[ctx], 1)
	enc.EncodeSymbol(cdfs.DefaultSingleRefCDF[1][0], 0)
	enc.EncodeSymbol(cdfs.DefaultSingleRefCDF[1][1], 0)
	enc.EncodeSymbol(cdfs.DefaultNewMvCDF[0], 0)
	writeMV(enc, mv)

	pred := make([]uint16, bw*bh)
	decoder.MotionCompensate16(pred, bw, bh, refY, refW, refH, refYStride,
		bx, by, mv, predict.InterpRegular, bitDepth)

	txSizeIdx, nzMap, scan, txSize, txW, txH := selectEncTxParams(bw, bh)
	residual := make([]int32, txW*txH)
	for r := 0; r < bh; r++ {
		for c := 0; c < bw; c++ {
			residual[r*txW+c] = int32(srcY[(by+r)*refYStride+(bx+c)]) - int32(pred[r*bw+c])
		}
	}
	if err := transform.Forward2D(residual, transform.DctDct, txSize); err != nil {
		enc.EncodeSymbol(cdfs.DefaultSkipCDF[0], 1)
		markInter(inter, miCol, miRow, bw, bh, miCols, miRows)
		return
	}
	enforceClampedScan(residual, txSize, txW, txH)
	qp := quant.Params{BaseQIndex: baseQ, BitDepth: bitDepth}
	qv := qp.Compute(quant.PlaneY)
	coeffs := make([]int32, txW*txH)
	hasResidual := false
	for i := range residual {
		coeffs[i] = quant.QuantizeCoeff(residual[i], i, qv)
		if coeffs[i] != 0 {
			hasResidual = true
		}
	}

	if !hasResidual {
		enc.EncodeSymbol(cdfs.DefaultSkipCDF[0], 1)
		markInter(inter, miCol, miRow, bw, bh, miCols, miRows)
		return
	}
	enc.EncodeSymbol(cdfs.DefaultSkipCDF[0], 0)
	qCtx := qIndexToCtx(baseQ)
	WriteCoefficients(enc, coeffs, txSizeIdx, 0, qCtx, scan, nzMap, txW, txH)

	// Chroma (subsampling-aware).
	cbw := bw >> uint(subX)
	cbh := bh >> uint(subY)
	if cbw < 1 {
		cbw = 1
	}
	if cbh < 1 {
		cbh = 1
	}
	chromaMV := decoder.MV{Row: mv.Row >> uint(subY), Col: mv.Col >> uint(subX)}
	cbx := bx >> uint(subX)
	cby := by >> uint(subY)
	crefW := refW >> uint(subX)
	crefH := refH >> uint(subY)
	for plane := 0; plane < 2; plane++ {
		srcPlane := srcU
		refPlane := refU
		pl := quant.PlaneU
		if plane == 1 {
			srcPlane = srcV
			refPlane = refV
			pl = quant.PlaneV
		}
		cpred := make([]uint16, cbw*cbh)
		decoder.MotionCompensate16(cpred, cbw, cbh, refPlane,
			crefW, crefH, refCStride,
			cbx, cby, chromaMV, predict.InterpRegular, bitDepth)
		ctxSizeIdx, cnzMap, cscan, ctxSize, ctxW, ctxH := selectEncTxParams(cbw, cbh)
		cresid := make([]int32, ctxW*ctxH)
		for r := 0; r < cbh; r++ {
			for c := 0; c < cbw; c++ {
				cresid[r*ctxW+c] = int32(srcPlane[(cby+r)*refCStride+(cbx+c)]) - int32(cpred[r*cbw+c])
			}
		}
		if err := transform.Forward2D(cresid, transform.DctDct, ctxSize); err != nil {
			enc.EncodeSymbol(cdfs.DefaultTxbSkipCDF[clamp(ctxSizeIdx, 0, 4)][0], 1)
			continue
		}
		enforceClampedScan(cresid, ctxSize, ctxW, ctxH)
		cqp := quant.Params{BaseQIndex: baseQ, BitDepth: bitDepth}
		cqv := cqp.Compute(pl)
		ccoeffs := make([]int32, ctxW*ctxH)
		chasResid := false
		for i := range cresid {
			ccoeffs[i] = quant.QuantizeCoeff(cresid[i], i, cqv)
			if ccoeffs[i] != 0 {
				chasResid = true
			}
		}
		if !chasResid {
			enc.EncodeSymbol(cdfs.DefaultTxbSkipCDF[clamp(ctxSizeIdx, 0, 4)][0], 1)
			continue
		}
		WriteCoefficients(enc, ccoeffs, ctxSizeIdx, 1, qCtx, cscan, cnzMap, ctxW, ctxH)
	}
	markInter(inter, miCol, miRow, bw, bh, miCols, miRows)
}
