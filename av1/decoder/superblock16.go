package decoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/predict"
	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// decodeLumaBlock16 is the uint16 counterpart of the luma predict +
// reconstruct step inside decodeLeafBlock. Neighbor gathering reads
// from fs.Y16, intra prediction runs in uint16, and the result
// (predicted for skip blocks, or reconstructed residual otherwise)
// writes back into fs.Y16.
func (td *TileDecoder) decodeLumaBlock16(fs *FrameState, yMode IntraMode, x, y, bw, bh int, skip bool, segID uint8) error {
	haveAbove := y > 0
	haveLeft := x > 0
	extLen := bw + bh
	above := make([]uint16, bw)
	left := make([]uint16, bh)
	aboveExt := make([]uint16, extLen)
	leftExt := make([]uint16, extLen)
	if haveAbove {
		for c := 0; c < extLen; c++ {
			sx := x + c
			if sx >= fs.Width {
				sx = fs.Width - 1
			}
			aboveExt[c] = fs.Y16[(y-1)*fs.YStride+sx]
		}
		copy(above, aboveExt[:bw])
	}
	if haveLeft {
		for r := 0; r < extLen; r++ {
			sy := y + r
			if sy >= fs.Height {
				sy = fs.Height - 1
			}
			leftExt[r] = fs.Y16[sy*fs.YStride+(x-1)]
		}
		copy(left, leftExt[:bh])
	}
	pred := make([]uint16, bw*bh)
	n := &Neighbors16{
		Above:         above,
		Left:          left,
		AboveExtended: aboveExt,
		LeftExtended:  leftExt,
		HaveAbove:     haveAbove,
		HaveLeft:      haveLeft,
		BitDepth:      fs.BitDepth,
	}
	if haveAbove && haveLeft {
		n.AboveLeft = fs.Y16[(y-1)*fs.YStride+(x-1)]
	}
	if err := PredictIntra16(pred, bw, bh, yMode, n); err != nil {
		return err
	}

	if skip {
		for r := 0; r < bh; r++ {
			for c := 0; c < bw; c++ {
				fs.Y16[(y+r)*fs.YStride+(x+c)] = pred[r*bw+c]
			}
		}
		return nil
	}
	return td.reconstructResidual16(fs, pred, x, y, bw, bh, yMode, segID)
}

// reconstructResidual16 mirrors reconstructResidual for the 10/12-bit
// path: read coefficients, dequantize, inverse-transform, then
// Reconstruct16Block into fs.Y16.
func (td *TileDecoder) reconstructResidual16(fs *FrameState, pred []uint16, x, y, bw, bh int, yMode IntraMode, segID uint8) error {
	if td.coeff == nil {
		return fmt.Errorf("%w: coeff decoder not initialized", ErrCoeffDecodeUnimplemented)
	}
	txSizeIdx, nzMap, scan, txSize, err := selectTxParams(bw, bh)
	if err != nil {
		return err
	}
	txSet := ExtTxSetForIntra(bw, bh)
	txType := transform.DctDct
	if txSet > 0 {
		raw := td.coeff.ReadIntraTxType(txSet, ExtTxSizeCtx(bw, bh), int(yMode))
		txType = IntraTxTypeFor(txSet, raw)
	}
	numCoeffs := len(scan)
	coeffs, err := td.coeff.ReadCoefficients(txSizeIdx, 0 /*luma*/, numCoeffs, scan, nzMap, bw, bh)
	if err != nil {
		return err
	}
	qParams := quant.Params{
		BaseQIndex: segmentedBaseQ(td, segID),
		DeltaQYDc:  int(td.fh.Quant.DeltaQYDc),
		BitDepth:   fs.BitDepth,
	}
	qv := qParams.Compute(quant.PlaneY)
	for i := range coeffs {
		coeffs[i] = DequantCoeff(coeffs[i], i, qv)
	}
	if err := transform.Inverse2D(coeffs, txType, txSize); err != nil {
		if err2 := transform.Inverse2D(coeffs, transform.DctDct, txSize); err2 != nil {
			return err2
		}
	}
	out := make([]uint16, bw*bh)
	Reconstruct16Block(out, pred, coeffs, bw, bh, fs.BitDepth)
	for r := 0; r < bh; r++ {
		for c := 0; c < bw; c++ {
			fs.Y16[(y+r)*fs.YStride+(x+c)] = out[r*bw+c]
		}
	}
	return nil
}

// decodeChromaBlock16 mirrors decodeChromaBlock for the uint16 path.
func (td *TileDecoder) decodeChromaBlock16(fs *FrameState, uvMode IntraMode, x, y, bw, bh int, skip bool, cflAlphaU, cflAlphaV int, segID uint8) error {
	cx := x >> fs.SubX
	cy := y >> fs.SubY
	cw := bw >> fs.SubX
	ch := bh >> fs.SubY
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	if cx+cw > fs.UVWidth {
		cw = fs.UVWidth - cx
	}
	if cy+ch > fs.UVHeight {
		ch = fs.UVHeight - cy
	}
	if cw <= 0 || ch <= 0 {
		return nil
	}

	for plane := 0; plane < 2; plane++ {
		dst := fs.U16
		if plane == 1 {
			dst = fs.V16
		}
		haveAbove := cy > 0
		haveLeft := cx > 0

		above := make([]uint16, cw)
		left := make([]uint16, ch)
		if haveAbove {
			for c := 0; c < cw; c++ {
				above[c] = dst[(cy-1)*fs.UVStride+(cx+c)]
			}
		}
		if haveLeft {
			for r := 0; r < ch; r++ {
				left[r] = dst[(cy+r)*fs.UVStride+(cx-1)]
			}
		}
		pred := make([]uint16, cw*ch)
		n := &Neighbors16{
			Above: above, Left: left,
			HaveAbove: haveAbove, HaveLeft: haveLeft,
			BitDepth: fs.BitDepth,
		}
		if haveAbove && haveLeft {
			n.AboveLeft = dst[(cy-1)*fs.UVStride+(cx-1)]
		}
		if uvMode == CFLPred {
			predict.DCPred16(pred, cw, ch, above, left, haveAbove, haveLeft, fs.BitDepth)
			if bw > 0 && bh > 0 {
				lumaBlock := make([]uint16, bw*bh)
				for r := 0; r < bh && y+r < fs.Height; r++ {
					for c := 0; c < bw && x+c < fs.Width; c++ {
						lumaBlock[r*bw+c] = fs.Y16[(y+r)*fs.YStride+(x+c)]
					}
				}
				lumaQ3 := make([]int32, cw*ch)
				predict.CFLSubsample16(lumaQ3, lumaBlock, bw, bh, fs.SubX, fs.SubY)
				alpha := cflAlphaU
				if plane == 1 {
					alpha = cflAlphaV
				}
				predict.CFLPred16(pred, cw, ch, lumaQ3, pred, alpha, fs.BitDepth)
			}
		} else if err := PredictIntra16(pred, cw, ch, uvMode, n); err != nil {
			return err
		}

		if skip {
			for r := 0; r < ch; r++ {
				for c := 0; c < cw; c++ {
					dst[(cy+r)*fs.UVStride+(cx+c)] = pred[r*cw+c]
				}
			}
			continue
		}
		if err := td.reconstructChromaResidual16(fs, dst, pred, cx, cy, cw, ch, plane,
			segmentedBaseQ(td, segID),
			int(td.fh.Quant.DeltaQUDc), int(td.fh.Quant.DeltaQUAc),
			int(td.fh.Quant.DeltaQVDc), int(td.fh.Quant.DeltaQVAc),
			fs.UVStride); err != nil {
			return err
		}
	}
	return nil
}

// reconstructChromaResidual16 mirrors reconstructChromaResidual for the
// uint16 path.
func (td *TileDecoder) reconstructChromaResidual16(
	fs *FrameState,
	dst []uint16, pred []uint16,
	cx, cy, cw, ch int,
	plane int,
	baseQ int,
	duDC, duAC, dvDC, dvAC int,
	stride int,
) error {
	if td.coeff == nil {
		return fmt.Errorf("%w: coeff decoder not initialized", ErrCoeffDecodeUnimplemented)
	}
	txSizeIdx, nzMap, scan, txSize, err := selectTxParams(cw, ch)
	if err != nil {
		return err
	}
	numCoeffs := len(scan)
	coeffs, err := td.coeff.ReadCoefficients(txSizeIdx, 1 /*chroma*/, numCoeffs, scan, nzMap, cw, ch)
	if err != nil {
		return err
	}
	qParams := quant.Params{
		BaseQIndex: baseQ,
		DeltaQUDc:  duDC,
		DeltaQUAc:  duAC,
		DeltaQVDc:  dvDC,
		DeltaQVAc:  dvAC,
		BitDepth:   fs.BitDepth,
	}
	pl := quant.PlaneU
	if plane == 1 {
		pl = quant.PlaneV
	}
	qv := qParams.Compute(pl)
	for i := range coeffs {
		coeffs[i] = DequantCoeff(coeffs[i], i, qv)
	}
	if err := transform.Inverse2D(coeffs, transform.DctDct, txSize); err != nil {
		return err
	}
	out := make([]uint16, cw*ch)
	Reconstruct16Block(out, pred, coeffs, cw, ch, fs.BitDepth)
	for r := 0; r < ch; r++ {
		for c := 0; c < cw; c++ {
			dst[(cy+r)*stride+(cx+c)] = out[r*cw+c]
		}
	}
	return nil
}
