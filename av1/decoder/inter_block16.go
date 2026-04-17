package decoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/predict"
	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// decodeInterLeafBlock16 is the HBD (10/12-bit) counterpart of
// decodeInterLeafBlock. It mirrors the 8-bit logic exactly but writes
// into fs.Y16 / fs.U16 / fs.V16 via the uint16 MC + uint16 residual
// reconstruction paths.
//
// Only pure inter blocks (is_inter=1, single-ref LAST, NEWMV) are
// supported in HBD today. Intra-within-inter blocks return
// ErrUnsupportedInterMode so callers see an explicit failure rather
// than silent corruption.
func (td *TileDecoder) decodeInterLeafBlock16(fs *FrameState, x, y int, bs BlockSize) error {
	w := bs.Width()
	h := bs.Height()
	if x+w > fs.Width {
		w = fs.Width - x
	}
	if y+h > fs.Height {
		h = fs.Height - y
	}
	if w <= 0 || h <= 0 {
		return nil
	}

	miCol := x >> 2
	miRow := y >> 2
	aboveIsInter := false
	leftIsInter := false
	if miRow > 0 {
		aboveIsInter = fs.GetMI(miCol, miRow-1).IsInter
	}
	if miCol > 0 {
		leftIsInter = fs.GetMI(miCol-1, miRow).IsInter
	}

	var segID uint8
	isInter := td.inter.ReadIsInter(aboveIsInter, leftIsInter)
	if !isInter {
		return fmt.Errorf("%w: HBD intra-within-inter block", ErrUnsupportedInterMode)
	}

	if _, err := td.inter.ReadSingleRefFrame(0, 0); err != nil {
		return fmt.Errorf("%w: multi-ref bit", ErrUnsupportedInterMode)
	}
	mode := td.inter.ReadInterMode(0, 0, 0)
	var mv MV
	switch mode {
	case InterModeNEWMV:
		mv = td.inter.ReadMV()
	case InterModeGLOBALMV, InterModeNEARESTMV, InterModeNEARMV:
		// MV = (0, 0) without a ref-MV list.
	}
	skip := td.inter.ReadSkip(0)

	miW := (w + 3) >> 2
	miH := (h + 3) >> 2
	for mr := 0; mr < miH && miRow+mr < fs.MIRows; mr++ {
		for mc := 0; mc < miW && miCol+mc < fs.MICols; mc++ {
			mi := fs.GetMI(miCol+mc, miRow+mr)
			mi.Skip = skip
			mi.IsInter = true
			mi.SegmentID = segID
		}
	}

	if td.refY16 == nil {
		return fmt.Errorf("%w: HBD inter block with no HBD reference", ErrUnsupportedInterMode)
	}
	pred := make([]uint16, w*h)
	MotionCompensate16(pred, w, h, td.refY16, td.refW, td.refH, td.refYSt,
		x, y, mv, predict.InterpRegular, fs.BitDepth)

	if skip {
		for r := 0; r < h; r++ {
			copy(fs.Y16[(y+r)*fs.YStride+x:(y+r)*fs.YStride+x+w], pred[r*w:r*w+w])
		}
	} else if err := td.reconstructInterResidual16(fs, pred, x, y, w, h, segID); err != nil {
		return err
	}

	if !fs.Monochrome {
		if err := td.decodeInterChromaBlock16(fs, x, y, w, h, skip, mv, segID); err != nil {
			return err
		}
	}
	return nil
}

// reconstructInterResidual16 is the HBD counterpart of
// reconstructInterResidual.
func (td *TileDecoder) reconstructInterResidual16(fs *FrameState, pred []uint16, x, y, w, h int, segID uint8) error {
	if td.coeff == nil {
		return fmt.Errorf("%w: coeff decoder not initialized", ErrCoeffDecodeUnimplemented)
	}
	txSizeIdx, nzMap, scan, txSize, err := selectTxParams(w, h)
	if err != nil {
		return err
	}
	numCoeffs := len(scan)
	coeffs, err := td.coeff.ReadCoefficients(txSizeIdx, 0, numCoeffs, scan, nzMap, w, h)
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
	if err := transform.Inverse2D(coeffs, transform.DctDct, txSize); err != nil {
		return err
	}
	maxV := int32((1 << uint(fs.BitDepth)) - 1)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			v := int32(pred[r*w+c]) + coeffs[r*w+c]
			if v < 0 {
				v = 0
			} else if v > maxV {
				v = maxV
			}
			fs.Y16[(y+r)*fs.YStride+x+c] = uint16(v)
		}
	}
	return nil
}

// decodeInterChromaBlock16 handles chroma for an HBD inter block.
func (td *TileDecoder) decodeInterChromaBlock16(fs *FrameState, x, y, w, h int, skip bool, mv MV, segID uint8) error {
	cx := x >> fs.SubX
	cy := y >> fs.SubY
	cw := w >> fs.SubX
	ch := h >> fs.SubY
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
	chromaMV := MV{Row: mv.Row >> uint(fs.SubY), Col: mv.Col >> uint(fs.SubX)}

	for plane := 0; plane < 2; plane++ {
		dst := fs.U16
		refPlane := td.refU16
		pl := quant.PlaneU
		if plane == 1 {
			dst = fs.V16
			refPlane = td.refV16
			pl = quant.PlaneV
		}
		if refPlane == nil {
			return fmt.Errorf("%w: HBD chroma inter with no reference", ErrUnsupportedInterMode)
		}
		pred := make([]uint16, cw*ch)
		MotionCompensate16(pred, cw, ch, refPlane, fs.UVWidth, fs.UVHeight, td.refCSt,
			cx, cy, chromaMV, predict.InterpRegular, fs.BitDepth)

		if skip {
			for r := 0; r < ch; r++ {
				copy(dst[(cy+r)*fs.UVStride+cx:(cy+r)*fs.UVStride+cx+cw], pred[r*cw:r*cw+cw])
			}
			continue
		}
		if err := td.reconstructInterChromaResidual16(fs, pred, dst, cx, cy, cw, ch, pl, plane, segID); err != nil {
			return err
		}
	}
	return nil
}

// reconstructInterChromaResidual16 reads + dequantizes + inverts a
// chroma residual and adds it to the MC prediction.
func (td *TileDecoder) reconstructInterChromaResidual16(
	fs *FrameState, pred []uint16, dst []uint16,
	cx, cy, cw, ch int, plane quant.Plane, planeIdx int, segID uint8,
) error {
	txSizeIdx, nzMap, scan, txSize, err := selectTxParams(cw, ch)
	if err != nil {
		return err
	}
	numCoeffs := len(scan)
	coeffs, err := td.coeff.ReadCoefficients(txSizeIdx, 1 /*chroma*/, numCoeffs, scan, nzMap, cw, ch)
	if err != nil {
		return err
	}
	qp := quant.Params{
		BaseQIndex: segmentedBaseQ(td, segID),
		BitDepth:   fs.BitDepth,
	}
	// DeltaQUDc/UAc / DeltaQVDc/VAc apply for U / V respectively.
	if plane == quant.PlaneU {
		qp.DeltaQUDc = int(td.fh.Quant.DeltaQUDc)
		qp.DeltaQUAc = int(td.fh.Quant.DeltaQUAc)
	} else {
		qp.DeltaQVDc = int(td.fh.Quant.DeltaQVDc)
		qp.DeltaQVAc = int(td.fh.Quant.DeltaQVAc)
	}
	qv := qp.Compute(plane)
	for i := range coeffs {
		coeffs[i] = DequantCoeff(coeffs[i], i, qv)
	}
	if err := transform.Inverse2D(coeffs, transform.DctDct, txSize); err != nil {
		return err
	}
	maxV := int32((1 << uint(fs.BitDepth)) - 1)
	for r := 0; r < ch; r++ {
		for c := 0; c < cw; c++ {
			v := int32(pred[r*cw+c]) + coeffs[r*cw+c]
			if v < 0 {
				v = 0
			} else if v > maxV {
				v = maxV
			}
			dst[(cy+r)*fs.UVStride+cx+c] = uint16(v)
		}
	}
	_ = planeIdx
	return nil
}
