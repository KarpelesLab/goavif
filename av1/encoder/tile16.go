package encoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// WriteIntraOnlyTile16 is the uint16 / HBD counterpart of
// [WriteIntraOnlyTile]. lumaY / chromaU / chromaV carry samples in the
// 0..(1<<bitDepth)-1 range; the encoder uses the HBD quantizer tables
// and clips reconstructed samples to the same range.
func WriteIntraOnlyTile16(width, height int, fh *obu.FrameHeader, sh *obu.SequenceHeader, lumaY, chromaU, chromaV []uint16) ([]byte, error) {
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
	bitDepth := int(sh.Color.BitDepth)
	if bitDepth < 8 {
		bitDepth = 8
	}
	subX := int(sh.Color.SubsamplingX)
	subY := int(sh.Color.SubsamplingY)

	cw := width >> subX
	if cw < 1 {
		cw = 1
	}
	ch := height >> subY
	if ch < 1 {
		ch = 1
	}

	recY := make([]uint16, width*height)
	recU := make([]uint16, cw*ch)
	recV := make([]uint16, cw*ch)

	miCols := (width + 3) >> 2
	miRows := (height + 3) >> 2
	st := &encState{
		modes:  make([]decoder.IntraMode, miCols*miRows),
		miCols: miCols,
		miRows: miRows,
		qCtx:   qIndexToCtx(baseQ),
		subX:   subX,
		subY:   subY,
	}

	for y := 0; y < height; y += sbSize {
		for x := 0; x < width; x += sbSize {
			if err := writeSuperblock16(&enc, x, y, sbSize, width, height, cw, ch, sh,
				lumaY, chromaU, chromaV,
				recY, recU, recV, st,
				baseQ, bitDepth); err != nil {
				return nil, err
			}
		}
	}
	return enc.Finish(), nil
}

func writeSuperblock16(enc *entropy.Encoder, x, y, sbSize, frameW, frameH, cw, ch int, sh *obu.SequenceHeader,
	lumaY, chromaU, chromaV []uint16,
	recY, recU, recV []uint16,
	st *encState,
	baseQ, bitDepth int) error {
	if sbSize == 64 && x+sbSize <= frameW && y+sbSize <= frameH {
		writePartitionSymbol(enc, 3, 0, 3 /* PARTITION_SPLIT */)
		for _, off := range [4][2]int{{0, 0}, {32, 0}, {0, 32}, {32, 32}} {
			qx := x + off[0]
			qy := y + off[1]
			if lumaY != nil && highDetail32_16(lumaY, recY, qx, qy, frameW, frameH, bitDepth) {
				writePartitionSymbol(enc, 2, 0, 3 /* PARTITION_SPLIT */)
				for _, off2 := range [4][2]int{{0, 0}, {16, 0}, {0, 16}, {16, 16}} {
					qqx := qx + off2[0]
					qqy := qy + off2[1]
					writePartitionSymbol(enc, 1, 0, 0 /* PARTITION_NONE */)
					writeLeaf16(enc, sh, qqx, qqy, 16, 16, frameW, frameH, cw, ch,
						lumaY, chromaU, chromaV,
						recY, recU, recV, st,
						baseQ, bitDepth)
				}
				continue
			}
			writePartitionSymbol(enc, 2, 0, 0 /* PARTITION_NONE */)
			writeLeaf16(enc, sh, qx, qy, 32, 32, frameW, frameH, cw, ch,
				lumaY, chromaU, chromaV,
				recY, recU, recV, st,
				baseQ, bitDepth)
		}
		return nil
	}

	writePartitionSymbol(enc, partitionBsl(sbSize), 0, 0)
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
	writeLeaf16(enc, sh, x, y, bw, bh, frameW, frameH, cw, ch,
		lumaY, chromaU, chromaV,
		recY, recU, recV, st,
		baseQ, bitDepth)
	return nil
}

func writeLeaf16(enc *entropy.Encoder, sh *obu.SequenceHeader,
	bx, by, bw, bh, frameW, frameH, cStrideW, cStrideH int,
	lumaY, chromaU, chromaV []uint16,
	recY, recU, recV []uint16,
	st *encState,
	baseQ, bitDepth int) {
	miX := bx >> 2
	miY := by >> 2
	miW := bw >> 2
	miH := bh >> 2
	aboveBucket, leftBucket := st.modeCtx(miX, miY)

	chosenMode := decoder.DCPred
	var lumaPred []uint16
	if lumaY != nil && bw > 0 && bh > 0 {
		chosenMode, lumaPred = chooseIntraMode16(lumaY, recY, bx, by, bw, bh, frameW, frameH, bitDepth)
	}

	enc.EncodeSymbol(cdfs.DefaultKfYModeCDF[aboveBucket][leftBucket], int(chosenMode))
	st.setMode(miX, miY, miW, miH, chosenMode)

	var hasLumaResidual bool
	var lumaCoeffs []int32
	var lumaScan []int
	var lumaNzMap []int8
	var lumaTxSizeIdx int
	var lumaTxSize transform.TxSize
	var lumaTxW, lumaTxH int
	var lumaDequant []int32

	if lumaY != nil && bw > 0 && bh > 0 {
		lumaTxSizeIdx, lumaNzMap, lumaScan, lumaTxSize, lumaTxW, lumaTxH = selectEncTxParams(bw, bh)

		residual := make([]int32, lumaTxW*lumaTxH)
		for r := 0; r < bh; r++ {
			srcRow := (by + r) * frameW
			predRow := r * bw
			for c := 0; c < bw; c++ {
				residual[r*lumaTxW+c] = int32(lumaY[srcRow+bx+c]) - int32(lumaPred[predRow+c])
			}
		}

		if err := transform.Forward2D(residual, transform.DctDct, lumaTxSize); err != nil {
			residual = nil
		}

		if residual != nil {
			enforceClampedScan(residual, lumaTxSize, lumaTxW, lumaTxH)

			qp := quant.Params{BaseQIndex: baseQ, BitDepth: bitDepth}
			qv := qp.Compute(quant.PlaneY)
			lumaCoeffs = make([]int32, lumaTxW*lumaTxH)
			for i := range residual {
				lumaCoeffs[i] = quant.QuantizeCoeff(residual[i], i, qv)
			}

			for _, v := range lumaCoeffs {
				if v != 0 {
					hasLumaResidual = true
					break
				}
			}

			if hasLumaResidual {
				lumaDequant = make([]int32, lumaTxW*lumaTxH)
				for i, v := range lumaCoeffs {
					lumaDequant[i] = decoder.DequantCoeff(v, i, qv)
				}
			}
		}
	}

	skipCDF := cdfs.DefaultSkipCDF[0]
	if hasLumaResidual {
		enc.EncodeSymbol(skipCDF, 0)
	} else {
		enc.EncodeSymbol(skipCDF, 1)
	}

	if !sh.Color.Monochrome {
		enc.EncodeSymbol(cdfs.DefaultUVModeCDF[1][int(chosenMode)], 0 /* DC_PRED */)
	}

	if !hasLumaResidual {
		if lumaPred != nil {
			writeBack16(recY, lumaPred, bx, by, bw, bh, frameW)
		}
		if !sh.Color.Monochrome {
			writeChromaSkipReconstruction16(recU, recV, bx, by, bw, bh,
				cStrideW, cStrideH, bitDepth, st.subX, st.subY)
		}
		return
	}

	writeIntraTxTypeIfNeeded(enc, bw, bh, int(chosenMode))

	WriteCoefficients(enc, lumaCoeffs, lumaTxSizeIdx, 0, st.qCtx, lumaScan, lumaNzMap, lumaTxW, lumaTxH)

	reconstructAndWrite16(recY, lumaPred, lumaDequant,
		bx, by, bw, bh, lumaTxW, lumaTxH,
		transform.DctDct, lumaTxSize,
		frameW, bitDepth)

	if !sh.Color.Monochrome {
		writeChromaDCLeaf16(enc, bx, by, bw, bh, cStrideW, cStrideH,
			chromaU, chromaV, recU, recV, baseQ, bitDepth, st.qCtx, st.subX, st.subY)
	}
}

func writeChromaDCLeaf16(enc *entropy.Encoder,
	bx, by, bw, bh, cStrideW, cStrideH int,
	chromaU, chromaV []uint16,
	recU, recV []uint16,
	baseQ, bitDepth, qCtx, subX, subY int) {
	cx := bx >> subX
	cy := by >> subY
	cbw := bw >> subX
	cbh := bh >> subY
	if cbw < 1 {
		cbw = 1
	}
	if cbh < 1 {
		cbh = 1
	}
	txSizeIdx, nzMap, scan, txSize, txW, txH := selectEncTxParams(cbw, cbh)

	for plane := 0; plane < 2; plane++ {
		srcPlane := chromaU
		recPlane := recU
		pl := quant.PlaneU
		if plane == 1 {
			srcPlane = chromaV
			recPlane = recV
			pl = quant.PlaneV
		}

		pred := make([]uint16, cbw*cbh)
		dcPredBlock16(pred, recPlane, cx, cy, cbw, cbh, cStrideW, cStrideH, bitDepth)

		var hasChromaResidual bool
		var chromaCoeffs []int32
		var chromaDequant []int32

		if srcPlane != nil {
			residual := make([]int32, txW*txH)
			for r := 0; r < cbh && cy+r < cStrideH; r++ {
				srcRow := (cy + r) * cStrideW
				predRow := r * cbw
				for c := 0; c < cbw && cx+c < cStrideW; c++ {
					residual[r*txW+c] = int32(srcPlane[srcRow+cx+c]) - int32(pred[predRow+c])
				}
			}

			if err := transform.Forward2D(residual, transform.DctDct, txSize); err == nil {
				enforceClampedScan(residual, txSize, txW, txH)

				qp := quant.Params{BaseQIndex: baseQ, BitDepth: bitDepth}
				qv := qp.Compute(pl)
				chromaCoeffs = make([]int32, txW*txH)
				for i := range residual {
					chromaCoeffs[i] = quant.QuantizeCoeff(residual[i], i, qv)
				}
				for _, v := range chromaCoeffs {
					if v != 0 {
						hasChromaResidual = true
						break
					}
				}
				if hasChromaResidual {
					chromaDequant = make([]int32, txW*txH)
					for i, v := range chromaCoeffs {
						chromaDequant[i] = decoder.DequantCoeff(v, i, qv)
					}
				}
			}
		}

		if hasChromaResidual {
			WriteCoefficients(enc, chromaCoeffs, txSizeIdx, 1, qCtx, scan, nzMap, txW, txH)
			reconstructAndWrite16(recPlane, pred, chromaDequant,
				cx, cy, cbw, cbh, txW, txH,
				transform.DctDct, txSize,
				cStrideW, bitDepth)
		} else {
			enc.EncodeSymbol(cdfs.DefaultTxbSkipCDF[clamp(txSizeIdx, 0, 4)][0], 1)
			writeBack16(recPlane, pred, cx, cy, cbw, cbh, cStrideW)
		}
	}
}

// highDetail32_16 mirrors highDetail32 for uint16 samples.
func highDetail32_16(lumaY, recY []uint16, bx, by, frameW, frameH, bitDepth int) bool {
	if bx+32 > frameW || by+32 > frameH {
		return false
	}
	_, pred := chooseIntraMode16(lumaY, recY, bx, by, 32, 32, frameW, frameH, bitDepth)
	if pred == nil {
		return false
	}
	sad := 0
	for r := 0; r < 32; r++ {
		srcRow := (by + r) * frameW
		predRow := r * 32
		for c := 0; c < 32; c++ {
			d := int(lumaY[srcRow+bx+c]) - int(pred[predRow+c])
			if d < 0 {
				d = -d
			}
			sad += d
		}
	}
	// Scale the threshold with bit depth: MAD > 20 at 8-bit maps to
	// MAD > 20 * (1 << (bitDepth-8)) at higher depths.
	mad := sad / (32 * 32)
	threshold := 20 << uint(bitDepth-8)
	return mad > threshold
}

func chooseIntraMode16(lumaY, recY []uint16, bx, by, bw, bh, frameW, frameH, bitDepth int) (decoder.IntraMode, []uint16) {
	n := buildNeighbors16(recY, bx, by, bw, bh, frameW, frameH, bitDepth)

	candidates := []decoder.IntraMode{decoder.DCPred}
	if n.HaveAbove {
		candidates = append(candidates, decoder.VPred)
	}
	if n.HaveLeft {
		candidates = append(candidates, decoder.HPred)
	}
	if n.HaveAbove && n.HaveLeft {
		candidates = append(candidates,
			decoder.PaethPred, decoder.SmoothPred,
			decoder.SmoothVPred, decoder.SmoothHPred,
			decoder.D45Pred, decoder.D67Pred, decoder.D113Pred,
			decoder.D135Pred, decoder.D157Pred, decoder.D203Pred)
	}

	bestMode := decoder.DCPred
	var bestPred []uint16
	bestSAD := int(-1)
	for _, m := range candidates {
		pred := make([]uint16, bw*bh)
		if err := decoder.PredictIntra16(pred, bw, bh, m, n); err != nil {
			continue
		}
		sad := 0
		for r := 0; r < bh; r++ {
			srcRow := (by + r) * frameW
			predRow := r * bw
			for c := 0; c < bw; c++ {
				d := int(lumaY[srcRow+bx+c]) - int(pred[predRow+c])
				if d < 0 {
					d = -d
				}
				sad += d
			}
		}
		if bestSAD < 0 || sad < bestSAD {
			bestSAD = sad
			bestMode = m
			bestPred = pred
		}
	}
	return bestMode, bestPred
}

func buildNeighbors16(recY []uint16, bx, by, bw, bh, frameW, frameH, bitDepth int) *decoder.Neighbors16 {
	extLen := bw + bh
	n := &decoder.Neighbors16{
		HaveAbove:     by > 0,
		HaveLeft:      bx > 0,
		BitDepth:      bitDepth,
		Above:         make([]uint16, bw),
		Left:          make([]uint16, bh),
		AboveExtended: make([]uint16, extLen),
		LeftExtended:  make([]uint16, extLen),
	}
	if n.HaveAbove {
		row := (by - 1) * frameW
		for c := 0; c < extLen; c++ {
			sx := bx + c
			if sx >= frameW {
				sx = frameW - 1
			}
			n.AboveExtended[c] = recY[row+sx]
		}
		copy(n.Above, n.AboveExtended[:bw])
	}
	if n.HaveLeft {
		for r := 0; r < extLen; r++ {
			sy := by + r
			if sy >= frameH {
				sy = frameH - 1
			}
			n.LeftExtended[r] = recY[sy*frameW+(bx-1)]
		}
		copy(n.Left, n.LeftExtended[:bh])
	}
	if n.HaveAbove && n.HaveLeft {
		n.AboveLeft = recY[(by-1)*frameW+(bx-1)]
	}
	return n
}

func dcPredBlock16(dst []uint16, ref []uint16, bx, by, bw, bh, frameW, frameH, bitDepth int) {
	haveAbove := by > 0
	haveLeft := bx > 0
	sum := 0
	n := 0
	if haveAbove {
		row := (by - 1) * frameW
		for c := 0; c < bw && bx+c < frameW; c++ {
			sum += int(ref[row+bx+c])
			n++
		}
	}
	if haveLeft {
		for r := 0; r < bh && by+r < frameH; r++ {
			sum += int(ref[(by+r)*frameW+(bx-1)])
			n++
		}
	}
	var dc uint16 = uint16(1 << uint(bitDepth-1))
	if n > 0 {
		dc = uint16((sum + n/2) / n)
	}
	for i := range dst {
		dst[i] = dc
	}
}

func reconstructAndWrite16(ref []uint16, pred []uint16, dequant []int32,
	bx, by, bw, bh, txW, txH int,
	txType transform.TxType, txSize transform.TxSize,
	stride, bitDepth int) {
	resid := append([]int32(nil), dequant...)
	_ = transform.Inverse2D(resid, txType, txSize)
	maxV := int32((1 << uint(bitDepth)) - 1)
	for r := 0; r < bh; r++ {
		for c := 0; c < bw; c++ {
			v := int32(pred[r*bw+c]) + resid[r*txW+c]
			if v < 0 {
				v = 0
			} else if v > maxV {
				v = maxV
			}
			ref[(by+r)*stride+(bx+c)] = uint16(v)
		}
	}
}

func writeBack16(ref []uint16, src []uint16, bx, by, bw, bh, stride int) {
	for r := 0; r < bh; r++ {
		copy(ref[(by+r)*stride+bx:(by+r)*stride+bx+bw], src[r*bw:r*bw+bw])
	}
}

func writeChromaSkipReconstruction16(recU, recV []uint16,
	bx, by, bw, bh, cStrideW, cStrideH, bitDepth, subX, subY int) {
	cx := bx >> subX
	cy := by >> subY
	cbw := bw >> subX
	cbh := bh >> subY
	if cbw < 1 {
		cbw = 1
	}
	if cbh < 1 {
		cbh = 1
	}
	pred := make([]uint16, cbw*cbh)
	dcPredBlock16(pred, recU, cx, cy, cbw, cbh, cStrideW, cStrideH, bitDepth)
	writeBack16(recU, pred, cx, cy, cbw, cbh, cStrideW)
	dcPredBlock16(pred, recV, cx, cy, cbw, cbh, cStrideW, cStrideH, bitDepth)
	writeBack16(recV, pred, cx, cy, cbw, cbh, cStrideW)
}
