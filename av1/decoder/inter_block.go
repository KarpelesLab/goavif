package decoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/predict"
	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// decodeInterLeafBlock is the inter-frame counterpart of
// decodeLeafBlock (see superblock.go). It reads the block-level
// inter syntax per spec §6.10.23, runs motion compensation against
// the reference frame stored on the tile decoder, adds any residual,
// and writes the samples into fs.Y / fs.U / fs.V.
//
// The narrow supported set is single-reference translational inter
// with explicit NEWMV (no ref MV list prediction), no compound, no
// warp, no interpolation-filter switching. Intra blocks within an
// inter frame are routed back through the existing intra path with
// the inter-frame Y mode CDF.
func (td *TileDecoder) decodeInterLeafBlock(fs *FrameState, x, y int, bs BlockSize) error {
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
	miHaveAbove := miRow > 0
	miHaveLeft := miCol > 0
	aboveIsInter := false
	leftIsInter := false
	if miHaveAbove {
		above := fs.GetMI(miCol, miRow-1)
		aboveIsInter = above.IsInter
	}
	if miHaveLeft {
		left := fs.GetMI(miCol-1, miRow)
		leftIsInter = left.IsInter
	}

	// Segment id isn't coded for frames without segmentation/update.
	var segID uint8

	// is_inter — context from above+left.
	isInter := td.inter.ReadIsInter(aboveIsInter, leftIsInter)

	// For intra-within-inter blocks we use the inter-frame
	// Y-mode / UV-mode CDFs. For pure inter blocks we still need
	// to read the inter-mode / ref / MV symbols.
	var yMode IntraMode
	var uvMode IntraMode = DCPred
	var mv MV
	var skip bool

	if isInter {
		// Single-reference LAST only in our narrow path.
		if _, err := td.inter.ReadSingleRefFrame(0, 0); err != nil {
			return fmt.Errorf("%w: multi-ref bit", ErrUnsupportedInterMode)
		}
		mode := td.inter.ReadInterMode(0, 0, 0)
		switch mode {
		case InterModeNEWMV:
			mv = td.inter.ReadMV()
		case InterModeGLOBALMV, InterModeNEARESTMV, InterModeNEARMV:
			// All three reduce to MV = (0, 0) in the absence of a
			// proper ref-MV list. Accept them but note the approx.
		}
		// Interpolation filter: AV1 emits the symbol only when the
		// sequence / frame header sets "switchable". Our simplified
		// path always uses REGULAR and skips the symbol — matches
		// encoders that pin InterpolationFilter=REGULAR.
		skip = td.inter.ReadSkip(0)
		yMode = DCPred // unused for inter blocks
	} else {
		// Intra block within inter frame: read y_mode from the
		// inter-frame CDF.
		yMode = td.inter.ReadYMode(blockSizeGroup(w, h))
		skip = td.inter.ReadSkip(0)
		// UV mode: reuse intra UV CDF. Our simplified encoder-style
		// path picks DC.
		if !fs.Monochrome {
			uvMode = td.DecodeUVMode(yMode, false)
		}
	}

	// Persist mode info for downstream neighbor context.
	miW := (w + 3) >> 2
	miH := (h + 3) >> 2
	for mr := 0; mr < miH && miRow+mr < fs.MIRows; mr++ {
		for mc := 0; mc < miW && miCol+mc < fs.MICols; mc++ {
			mi := fs.GetMI(miCol+mc, miRow+mr)
			mi.Mode = yMode
			mi.UVMode = uvMode
			mi.Skip = skip
			mi.IsInter = isInter
			mi.SegmentID = segID
		}
	}

	// Produce predicted samples — either from MC (inter) or intra.
	pred := make([]uint8, w*h)
	if isInter {
		if td.refY == nil {
			return fmt.Errorf("%w: inter block with no reference", ErrUnsupportedInterMode)
		}
		MotionCompensate(pred, w, h, td.refY, td.refW, td.refH, td.refYSt,
			x, y, mv, predict.InterpRegular)
	} else {
		n := td.buildLumaNeighbors(fs, x, y, w, h)
		if err := PredictIntra(pred, w, h, yMode, n); err != nil {
			return err
		}
	}

	if skip {
		for r := 0; r < h; r++ {
			copy(fs.Y[(y+r)*fs.YStride+x:(y+r)*fs.YStride+x+w], pred[r*w:r*w+w])
		}
	} else if err := td.reconstructInterResidual(fs, pred, x, y, w, h, segID); err != nil {
		return err
	}

	// Chroma.
	if !fs.Monochrome {
		if err := td.decodeInterChromaBlock(fs, uvMode, x, y, w, h, skip, isInter, mv, segID); err != nil {
			return err
		}
	}
	return nil
}

// blockSizeGroup maps a luma block (w, h) to the 4-bucket group used
// by the inter-frame y_mode CDF: 0 for ≤ 8×8 area, 1 for ≤ 16×16, 2
// for ≤ 32×32, 3 for larger.
func blockSizeGroup(w, h int) int {
	area := w * h
	switch {
	case area <= 8*8:
		return 0
	case area <= 16*16:
		return 1
	case area <= 32*32:
		return 2
	}
	return 3
}

// buildLumaNeighbors assembles the Neighbors struct expected by the
// intra predictor for a luma block inside an inter frame. It mirrors
// the shorter setup in decodeLeafBlock but is kept local so the
// inter path doesn't depend on the non-inter control flow.
func (td *TileDecoder) buildLumaNeighbors(fs *FrameState, x, y, w, h int) *Neighbors {
	haveAbove := y > 0
	haveLeft := x > 0
	ext := w + h
	above := make([]uint8, w)
	left := make([]uint8, h)
	aboveExt := make([]uint8, ext)
	leftExt := make([]uint8, ext)
	if haveAbove {
		for c := 0; c < ext; c++ {
			sx := x + c
			if sx >= fs.Width {
				sx = fs.Width - 1
			}
			aboveExt[c] = fs.Y[(y-1)*fs.YStride+sx]
		}
		copy(above, aboveExt[:w])
	}
	if haveLeft {
		for r := 0; r < ext; r++ {
			sy := y + r
			if sy >= fs.Height {
				sy = fs.Height - 1
			}
			leftExt[r] = fs.Y[sy*fs.YStride+(x-1)]
		}
		copy(left, leftExt[:h])
	}
	n := &Neighbors{
		Above:         above,
		Left:          left,
		AboveExtended: aboveExt,
		LeftExtended:  leftExt,
		HaveAbove:     haveAbove,
		HaveLeft:      haveLeft,
		BitDepth:      8,
	}
	if haveAbove && haveLeft {
		n.AboveLeft = fs.Y[(y-1)*fs.YStride+(x-1)]
	}
	return n
}

// reconstructInterResidual applies the coefficient/residual coding
// path to pred and writes the final samples into fs.Y. Reuses
// selectTxParams + the intra coefficient decoder; the inter frame's
// residual syntax is identical to the intra path.
func (td *TileDecoder) reconstructInterResidual(fs *FrameState, pred []uint8, x, y, w, h int, segID uint8) error {
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
		BitDepth:   int(td.sh.Color.BitDepth),
	}
	qv := qParams.Compute(quant.PlaneY)
	for i := range coeffs {
		coeffs[i] = DequantCoeff(coeffs[i], i, qv)
	}
	if err := transform.Inverse2D(coeffs, transform.DctDct, txSize); err != nil {
		return err
	}
	out := make([]uint8, w*h)
	ReconstructBlock(out, pred, coeffs, w, h)
	for r := 0; r < h; r++ {
		copy(fs.Y[(y+r)*fs.YStride+x:(y+r)*fs.YStride+x+w], out[r*w:r*w+w])
	}
	return nil
}

// decodeInterChromaBlock produces chroma samples for an inter-frame
// leaf block: MC for inter blocks, intra prediction + residual for
// intra blocks. Structure mirrors decodeChromaBlock in superblock.go
// but with MC replacing intra predict when isInter is true.
func (td *TileDecoder) decodeInterChromaBlock(fs *FrameState, uvMode IntraMode,
	x, y, w, h int, skip, isInter bool, mv MV, segID uint8) error {
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
	// Scale MV to chroma sample units when chroma is subsampled.
	chromaMV := MV{Row: mv.Row >> uint(fs.SubY), Col: mv.Col >> uint(fs.SubX)}

	for plane := 0; plane < 2; plane++ {
		dst := fs.U
		refPlane := td.refU
		if plane == 1 {
			dst = fs.V
			refPlane = td.refV
		}
		pred := make([]uint8, cw*ch)
		if isInter {
			if refPlane == nil {
				return fmt.Errorf("%w: inter chroma with no reference", ErrUnsupportedInterMode)
			}
			MotionCompensate(pred, cw, ch, refPlane, fs.UVWidth, fs.UVHeight, td.refCSt,
				cx, cy, chromaMV, predict.InterpRegular)
		} else {
			td.fillIntraChromaPred(pred, dst, cx, cy, cw, ch, uvMode)
		}

		if skip {
			for r := 0; r < ch; r++ {
				copy(dst[(cy+r)*fs.UVStride+cx:(cy+r)*fs.UVStride+cx+cw], pred[r*cw:r*cw+cw])
			}
			continue
		}
		// Non-skip chroma: read residual and add to prediction.
		if err := td.reconstructChromaResidual(dst, pred, cx, cy, cw, ch, plane,
			segmentedBaseQ(td, segID),
			int(td.fh.Quant.DeltaQUDc), int(td.fh.Quant.DeltaQUAc),
			int(td.fh.Quant.DeltaQVDc), int(td.fh.Quant.DeltaQVAc),
			fs.UVStride); err != nil {
			return err
		}
	}
	return nil
}

// fillIntraChromaPred produces intra-prediction samples for a chroma
// block within an inter frame. Uses the neighbor-derived DC predictor
// for uvMode DC_PRED (our narrow subset).
func (td *TileDecoder) fillIntraChromaPred(pred []uint8, dst []uint8, cx, cy, cw, ch int, uvMode IntraMode) {
	haveAbove := cy > 0
	haveLeft := cx > 0
	above := make([]uint8, cw)
	left := make([]uint8, ch)
	if haveAbove {
		for c := 0; c < cw; c++ {
			above[c] = dst[(cy-1)*cw+(cx+c)]
		}
	}
	if haveLeft {
		for r := 0; r < ch; r++ {
			left[r] = dst[(cy+r)*cw+(cx-1)]
		}
	}
	n := &Neighbors{
		Above:     above,
		Left:      left,
		HaveAbove: haveAbove,
		HaveLeft:  haveLeft,
		BitDepth:  8,
	}
	_ = PredictIntra(pred, cw, ch, uvMode, n)
}
