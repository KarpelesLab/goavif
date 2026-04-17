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

// WriteInterCopyTile emits the tile-group payload for an inter frame
// that is a bit-for-bit copy of the reference frame: every block is
// is_inter=1, single_ref=LAST, inter_mode=NEWMV with MV=(0,0),
// skip_txfm=1. The decoder runs motion compensation against the
// reference at zero offset and skips the residual, so the output
// matches the reference exactly.
//
// Intended for roundtrip testing of the inter decode path — it
// produces minimal but spec-structured inter bitstreams.
func WriteInterCopyTile(width, height int, fh *obu.FrameHeader, sh *obu.SequenceHeader) ([]byte, error) {
	return WriteInterUniformMVTile(width, height, fh, sh, decoder.MV{Row: 0, Col: 0})
}

// WriteInterUniformMVTile generalizes [WriteInterCopyTile] to any
// uniform motion vector: every block uses the same MV, single-ref
// LAST, NEWMV, skip_txfm=1. The MV uses eighth-pel precision.
func WriteInterUniformMVTile(width, height int, fh *obu.FrameHeader, sh *obu.SequenceHeader, mv decoder.MV) ([]byte, error) {
	if sh == nil || fh == nil {
		return nil, fmt.Errorf("encoder: nil sh / fh")
	}
	var enc entropy.Encoder
	enc.Init(!fh.DisableCDFUpdate)

	sbSize := 64
	if sh.Use128x128Superblock {
		sbSize = 128
	}

	// Track per-MI is_inter state for is_inter CDF context. 4-pixel
	// MI grid: inter[miRow*miCols + miCol] = 1 once the block there
	// has been written as inter.
	miCols := (width + 3) >> 2
	miRows := (height + 3) >> 2
	inter := make([]uint8, miCols*miRows)

	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			if x+sbSize <= width && y+sbSize <= height {
				// Full-SB split into four 32×32 leaves — same
				// structure as the intra encoder.
				writePartitionSymbol(&enc, 3, 0, 3 /* SPLIT */)
				for _, off := range [4][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
					bx := x + off[0]
					by := y + off[1]
					writePartitionSymbol(&enc, 2, 0, 0 /* NONE */)
					writeInterSkipBlock(&enc, sh, bx, by, 32, 32, mv, inter, miCols, miRows)
				}
				continue
			}
			// Fallback PARTITION_NONE at the SB size (last row/column).
			bw := sbSize
			bh := sbSize
			if x+bw > width {
				bw = width - x
			}
			if y+bh > height {
				bh = height - y
			}
			writePartitionSymbol(&enc, partitionBsl(sbSize), 0, 0)
			writeInterSkipBlock(&enc, sh, x, y, bw, bh, mv, inter, miCols, miRows)
		}
	}
	return enc.Finish(), nil
}

// writeInterSkipBlock emits the symbols for a single inter block
// using the supplied MV (eighth-pel units), single-ref LAST, NEWMV,
// skip_txfm=1. Matches decoder.decodeInterLeafBlock's read order.
func writeInterSkipBlock(enc *entropy.Encoder, sh *obu.SequenceHeader,
	bx, by, bw, bh int, mv decoder.MV, inter []uint8, miCols, miRows int) {
	// is_inter context matches decoder.ReadIsInter: above / left
	// neighbor's inter flag determines the CDF.
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

	// single_ref tree — LAST path. Decoder reads ctx=1.
	enc.EncodeSymbol(cdfs.DefaultSingleRefCDF[1][0], 0)
	enc.EncodeSymbol(cdfs.DefaultSingleRefCDF[1][1], 0)

	// Inter mode: NEWMV — first bit of the newmv tree is 0.
	enc.EncodeSymbol(cdfs.DefaultNewMvCDF[0], 0)

	// MV components.
	writeMV(enc, mv)

	// skip_txfm = 1.
	enc.EncodeSymbol(cdfs.DefaultSkipCDF[0], 1)

	// Mark every MI cell the block occupies as inter for the
	// neighbors of following blocks.
	miW := (bw + 3) >> 2
	miH := (bh + 3) >> 2
	for r := 0; r < miH && miRow+r < miRows; r++ {
		for c := 0; c < miW && miCol+c < miCols; c++ {
			inter[(miRow+r)*miCols+(miCol+c)] = 1
		}
	}

	_ = sh
}

// WriteInterResidualTile emits an inter tile-group payload where
// every 32×32 block uses the supplied MV, single-ref LAST, NEWMV,
// and carries a coded Y residual against the motion-compensated
// prediction. The residual uses the same coefficient path as the
// intra encoder (Forward2D + QuantizeCoeff + WriteCoefficients).
//
// srcY / srcU / srcV are the source planes at frame resolution
// (chroma planes at width/2 × height/2 for 4:2:0). refY / refU /
// refV are the decoded reference planes used for MC. Only
// single-reference 4:2:0 is supported today.
func WriteInterResidualTile(width, height int,
	fh *obu.FrameHeader, sh *obu.SequenceHeader,
	mv decoder.MV,
	srcY, srcU, srcV []uint8,
	refY, refU, refV []uint8,
	refW, refH int,
) ([]byte, error) {
	if sh == nil || fh == nil {
		return nil, fmt.Errorf("encoder: nil sh / fh")
	}
	if width%64 != 0 || height%64 != 0 {
		return nil, fmt.Errorf("encoder: WriteInterResidualTile requires 64-aligned dims, got %dx%d", width, height)
	}
	var enc entropy.Encoder
	enc.Init(!fh.DisableCDFUpdate)

	sbSize := 64
	baseQ := int(fh.Quant.BaseQIndex)
	subX := int(sh.Color.SubsamplingX)
	subY := int(sh.Color.SubsamplingY)
	miCols := (width + 3) >> 2
	miRows := (height + 3) >> 2
	inter := make([]uint8, miCols*miRows)
	refYStride := refW
	refCStride := refW >> uint(subX)

	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			if x+sbSize <= width && y+sbSize <= height {
				writePartitionSymbol(&enc, 3, 0, 3 /* SPLIT */)
				for _, off := range [4][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
					bx := x + off[0]
					by := y + off[1]
					writePartitionSymbol(&enc, 2, 0, 0 /* NONE */)
					writeInterResidualBlock(&enc, bx, by, 32, 32, mv,
						srcY, srcU, srcV,
						refY, refU, refV, refW, refH, refYStride, refCStride,
						inter, miCols, miRows, baseQ, subX, subY)
				}
				continue
			}
			// Fallback — shouldn't hit in 64-aligned path.
			writePartitionSymbol(&enc, partitionBsl(sbSize), 0, 0)
			bw := sbSize
			if x+bw > width {
				bw = width - x
			}
			bh := sbSize
			if y+bh > height {
				bh = height - y
			}
			writeInterResidualBlock(&enc, x, y, bw, bh, mv,
				srcY, srcU, srcV,
				refY, refU, refV, refW, refH, refYStride, refCStride,
				inter, miCols, miRows, baseQ, subX, subY)
		}
	}
	return enc.Finish(), nil
}

// writeInterResidualBlock emits one inter block with a real residual:
// is_inter, ref, NEWMV, MV, skip_txfm=0, Y coefficients against the
// motion-compensated prediction. Chroma is emitted as skipped for
// simplicity (the encoder clones the reference chroma into place).
func writeInterResidualBlock(enc *entropy.Encoder,
	bx, by, bw, bh int, mv decoder.MV,
	srcY, srcU, srcV []uint8,
	refY, refU, refV []uint8,
	refW, refH, refYStride, refCStride int,
	inter []uint8, miCols, miRows int,
	baseQ, subX, subY int,
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

	// Compute MC prediction for this block.
	pred := make([]uint8, bw*bh)
	decoder.MotionCompensate(pred, bw, bh, refY, refW, refH, refYStride,
		bx, by, mv, predict.InterpRegular)

	// Residual = src - pred.
	txSizeIdx, nzMap, scan, txSize, txW, txH := selectEncTxParams(bw, bh)
	residual := make([]int32, txW*txH)
	for r := 0; r < bh; r++ {
		for c := 0; c < bw; c++ {
			residual[r*txW+c] = int32(srcY[(by+r)*refYStride+(bx+c)]) - int32(pred[r*bw+c])
		}
	}
	if err := transform.Forward2D(residual, transform.DctDct, txSize); err != nil {
		// Fall back to skip.
		enc.EncodeSymbol(cdfs.DefaultSkipCDF[0], 1)
		markInter(inter, miCol, miRow, bw, bh, miCols, miRows)
		return
	}
	enforceClampedScan(residual, txSize, txW, txH)
	qp := quant.Params{BaseQIndex: baseQ, BitDepth: 8}
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

	// skip_txfm = 0.
	enc.EncodeSymbol(cdfs.DefaultSkipCDF[0], 0)
	// NOTE: tx_type is NOT emitted for inter blocks in our decoder —
	// it hardcodes DctDct in reconstructInterResidual. To stay in
	// sync we skip writeIntraTxTypeIfNeeded here. (Full AV1 would
	// emit via the inter-frame ext_tx CDF family, which is out of
	// scope for now.)
	qCtx := qIndexToCtx(baseQ)
	WriteCoefficients(enc, coeffs, txSizeIdx, 0, qCtx, scan, nzMap, txW, txH)

	// Monochrome: no chroma planes to code.
	if srcU == nil || srcV == nil || refU == nil || refV == nil {
		markInter(inter, miCol, miRow, bw, bh, miCols, miRows)
		return
	}

	// Chroma residuals (subsampling-aware).
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
		// MC prediction for chroma.
		cpred := make([]uint8, cbw*cbh)
		decoder.MotionCompensate(cpred, cbw, cbh, refPlane,
			crefW, crefH, refCStride,
			cbx, cby, chromaMV, predict.InterpRegular)
		// Residual.
		ctxSizeIdx, cnzMap, cscan, ctxSize, ctxW, ctxH := selectEncTxParams(cbw, cbh)
		cresid := make([]int32, ctxW*ctxH)
		for r := 0; r < cbh; r++ {
			for c := 0; c < cbw; c++ {
				cresid[r*ctxW+c] = int32(srcPlane[(cby+r)*refCStride+(cbx+c)]) - int32(cpred[r*cbw+c])
			}
		}
		if err := transform.Forward2D(cresid, transform.DctDct, ctxSize); err != nil {
			// Emit skip for this chroma plane.
			enc.EncodeSymbol(cdfs.DefaultTxbSkipCDF[clamp(ctxSizeIdx, 0, 4)][0], 1)
			continue
		}
		enforceClampedScan(cresid, ctxSize, ctxW, ctxH)
		qp := quant.Params{BaseQIndex: baseQ, BitDepth: 8}
		cqv := qp.Compute(pl)
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
		WriteCoefficients(enc, ccoeffs, ctxSizeIdx, 1 /*chroma*/, qCtx, cscan, cnzMap, ctxW, ctxH)
	}

	markInter(inter, miCol, miRow, bw, bh, miCols, miRows)
}

func markInter(inter []uint8, miCol, miRow, bw, bh, miCols, miRows int) {
	miW := (bw + 3) >> 2
	miH := (bh + 3) >> 2
	for r := 0; r < miH && miRow+r < miRows; r++ {
		for c := 0; c < miW && miCol+c < miCols; c++ {
			inter[(miRow+r)*miCols+(miCol+c)] = 1
		}
	}
}

// writeMV emits an MV through the inter-frame MV CDFs so
// decoder.MVDecoder.ReadMV reads the same value back. Assumes
// allow_high_precision_mv=false (the AVIS writer's default).
func writeMV(enc *entropy.Encoder, mv decoder.MV) {
	// mv_joint based on which components are non-zero.
	var joint decoder.MVJoint
	switch {
	case mv.Row == 0 && mv.Col == 0:
		joint = decoder.MVJointZero
	case mv.Row == 0:
		joint = decoder.MVJointHNZVZ
	case mv.Col == 0:
		joint = decoder.MVJointHZVNZ
	default:
		joint = decoder.MVJointHNZVNZ
	}
	enc.EncodeSymbol(cdfs.DefaultMvJointCDF, int(joint))
	if joint == decoder.MVJointHNZVZ || joint == decoder.MVJointHNZVNZ {
		writeMVComponent(enc, mv.Col, 0 /* horizontal comp */)
	}
	if joint == decoder.MVJointHZVNZ || joint == decoder.MVJointHNZVNZ {
		writeMVComponent(enc, mv.Row, 1 /* vertical comp */)
	}
}

// writeMVComponent mirrors decoder.MVDecoder.readComponent but in
// reverse: given a signed eighth-pel magnitude, emit sign / class /
// class0-bit / class0-fr / fr / bits symbols so the decoder
// reconstructs the same value. No high-precision MV bits are emitted
// (we hold allowHighPrecMV=false — the decoder then forces hp=1 to
// round the final magnitude up by 1/8 pel).
func writeMVComponent(enc *entropy.Encoder, comp int32, idx int) {
	sign := 0
	mag := comp
	if mag < 0 {
		sign = 1
		mag = -mag
	}
	enc.EncodeSymbol(cdfs.DefaultMvSignCDF[idx], sign)

	// The decoder reconstructs the magnitude as mag = body + hp + 1
	// where hp = 1 (because allowHighPrecMV=false forces it). So the
	// body we need to emit is mag - 2 (subtract the +1 from hp and
	// the +1 at the end of readComponent).
	if mag < 2 {
		// Encoding MV magnitudes below 2 eighth-pel is not
		// representable without negative fr; clip to 2.
		mag = 2
	}
	body := int32(mag - 2)

	// Pick class 0 when body < 8 (integer pel == 0), else higher
	// classes. Body = magInt (class 0) or magInt (class ≥ 1)
	// where magInt starts at 8 for class 1, 16 for class 2, etc.
	if body < 8 {
		// Class 0: body = (class0_bit*8) + (fr*2)
		enc.EncodeSymbol(cdfs.DefaultMvClassCDF[idx], 0)
		b := int32(0)
		if body >= 8 {
			b = 1
		}
		enc.EncodeSymbol(cdfs.DefaultMvClass0BitCDF[idx], int(b))
		fr := (body - b*8) / 2
		enc.EncodeSymbol(cdfs.DefaultMvClass0FrCDF[idx][b], int(fr))
		return
	}
	// Class 1..10. magInt range: class c has magInt starting at
	// (1 << (c+2)) eighth-pel, i.e. class 1: 8, class 2: 16, etc.
	// body = magInt + fr*2. Pick the smallest c such that body < next
	// class start.
	cls := int32(1)
	for cls < 10 && body >= int32(1<<uint(cls+3)) {
		cls++
	}
	enc.EncodeSymbol(cdfs.DefaultMvClassCDF[idx], int(cls))
	// Compute bits for this class: body = (1<<(cls+2)) + (bits<<3) + fr*2.
	extra := body - int32(1<<uint(cls+2))
	bits := extra >> 3
	fr := (extra - (bits << 3)) / 2
	for i := int32(0); i < cls; i++ {
		b := (bits >> uint(i)) & 1
		enc.EncodeSymbol(cdfs.DefaultMvBitsCDF[idx][i], int(b))
	}
	enc.EncodeSymbol(cdfs.DefaultMvFrCDF[idx], int(fr))
}
