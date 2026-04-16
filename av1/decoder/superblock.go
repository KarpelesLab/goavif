package decoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/quant"
	"github.com/KarpelesLab/goavif/av1/transform"
)

// ModeInfo stores per-MI-unit (4×4 block) decoded mode information.
type ModeInfo struct {
	Mode      IntraMode
	UVMode    IntraMode
	Skip      bool
	Predicted bool // true once intra prediction has been applied
}

// FrameState is the mutable per-frame state accumulated as the tile decoder
// processes each superblock. It holds the mode-info grid and the
// reconstructed pixel planes.
type FrameState struct {
	Width    int
	Height   int
	MICols   int
	MIRows   int
	MI       []ModeInfo // MICols × MIRows
	Y        []uint8    // luma plane, Width × Height
	YStride  int
}

// NewFrameState allocates a blank frame ready for decoding.
func NewFrameState(w, h int) *FrameState {
	miC := (w + 3) >> 2
	miR := (h + 3) >> 2
	return &FrameState{
		Width:   w,
		Height:  h,
		MICols:  miC,
		MIRows:  miR,
		MI:      make([]ModeInfo, miC*miR),
		Y:       make([]uint8, w*h),
		YStride: w,
	}
}

// GetMI returns a pointer to the ModeInfo at (miCol, miRow). Bounds are
// NOT checked; the caller must ensure validity.
func (fs *FrameState) GetMI(miCol, miRow int) *ModeInfo {
	return &fs.MI[miRow*fs.MICols+miCol]
}

// modeCtxBucket maps an IntraMode to a 5-bucket context index used by the
// kf_y_mode_cdf lookup: 0=DC, 1=V, 2=H, 3=D45..D67, 4=SMOOTH..PAETH.
func modeCtxBucket(m IntraMode) int {
	switch m {
	case DCPred:
		return 0
	case VPred:
		return 1
	case HPred:
		return 2
	case D45Pred, D135Pred, D113Pred, D157Pred, D203Pred, D67Pred:
		return 3
	default:
		return 4 // SMOOTH, SMOOTH_V, SMOOTH_H, PAETH
	}
}

// DecodeSuperblock reads and reconstructs one superblock at pixel
// position (sbX, sbY). It walks the partition tree, decodes modes for
// each leaf block, and for skip blocks applies the intra predictor to
// write pixels into fs.Y.
//
// Non-skip blocks are NOT yet handled (coefficient decode is pending);
// they return ErrCoeffDecodeUnimplemented on the first non-skip leaf
// encountered. For images encoded at very high QP (where every block is
// skip), the full frame will reconstruct.
func (td *TileDecoder) DecodeSuperblock(fs *FrameState, sbX, sbY int) error {
	sbBS := Block64x64
	if td.sbSize == 128 {
		sbBS = Block128x128
	}
	return td.decodePartitionNode(fs, sbX, sbY, sbBS)
}

// decodePartitionNode recursively walks the partition tree.
func (td *TileDecoder) decodePartitionNode(fs *FrameState, x, y int, bs BlockSize) error {
	w := bs.Width()
	h := bs.Height()

	// Skip blocks that fall entirely outside the frame.
	if x >= fs.Width || y >= fs.Height {
		return nil
	}

	// At minimum block size, decode directly.
	if bs == Block4x4 {
		return td.decodeLeafBlock(fs, x, y, bs)
	}

	if !bs.IsSquare() {
		return td.decodeLeafBlock(fs, x, y, bs)
	}

	// Determine BSL context (block-size-log for partition CDF index).
	bsl := blockSizeLog(bs)
	aboveCtx := 0
	leftCtx := 0

	pt := td.DecodePartition(bsl, aboveCtx*2+leftCtx)
	hw := w / 2
	hh := h / 2

	switch pt {
	case 0: // PARTITION_NONE
		return td.decodeLeafBlock(fs, x, y, bs)
	case 1: // PARTITION_HORZ
		top := halfBelowSize(bs, true)
		if err := td.decodeLeafBlock(fs, x, y, top); err != nil {
			return err
		}
		if y+hh < fs.Height {
			bot := halfBelowSize(bs, true)
			return td.decodeLeafBlock(fs, x, y+hh, bot)
		}
		return nil
	case 2: // PARTITION_VERT
		left := halfBelowSize(bs, false)
		if err := td.decodeLeafBlock(fs, x, y, left); err != nil {
			return err
		}
		if x+hw < fs.Width {
			right := halfBelowSize(bs, false)
			return td.decodeLeafBlock(fs, x+hw, y, right)
		}
		return nil
	case 3: // PARTITION_SPLIT
		sub := quarterSize(bs)
		if err := td.decodePartitionNode(fs, x, y, sub); err != nil {
			return err
		}
		if err := td.decodePartitionNode(fs, x+hw, y, sub); err != nil {
			return err
		}
		if err := td.decodePartitionNode(fs, x, y+hh, sub); err != nil {
			return err
		}
		return td.decodePartitionNode(fs, x+hw, y+hh, sub)
	default:
		// Extended partitions (HORZ_A/B, VERT_A/B, HORZ_4, VERT_4) — not
		// yet implemented but consumed from the bitstream. Treat as a
		// single leaf to stay synchronized.
		return td.decodeLeafBlock(fs, x, y, bs)
	}
}

// decodeLeafBlock decodes one coding block: reads mode symbols, and for
// skip blocks applies intra prediction directly.
func (td *TileDecoder) decodeLeafBlock(fs *FrameState, x, y int, bs BlockSize) error {
	w := bs.Width()
	h := bs.Height()

	// Clip to frame boundaries.
	if x+w > fs.Width {
		w = fs.Width - x
	}
	if y+h > fs.Height {
		h = fs.Height - y
	}
	if w <= 0 || h <= 0 {
		return nil
	}

	// Decode Y intra mode.
	miCol := x >> 2
	miRow := y >> 2
	aboveMode := DCPred
	leftMode := DCPred
	if miRow > 0 && miRow-1 < fs.MIRows && miCol < fs.MICols {
		aboveMode = fs.GetMI(miCol, miRow-1).Mode
	}
	if miCol > 0 && miCol-1 < fs.MICols && miRow < fs.MIRows {
		leftMode = fs.GetMI(miCol-1, miRow).Mode
	}
	yMode := td.DecodeIntraYMode(modeCtxBucket(aboveMode), modeCtxBucket(leftMode))
	skip := td.DecodeSkip(0)

	// Store mode info for every 4×4 MI cell covered by this block.
	miW := (w + 3) >> 2
	miH := (h + 3) >> 2
	for mr := 0; mr < miH && miRow+mr < fs.MIRows; mr++ {
		for mc := 0; mc < miW && miCol+mc < fs.MICols; mc++ {
			mi := fs.GetMI(miCol+mc, miRow+mr)
			mi.Mode = yMode
			mi.Skip = skip
		}
	}

	// Determine effective block dimensions (clipped to frame).
	bw := bs.Width()
	bh := bs.Height()
	if bw > fs.Width-x {
		bw = fs.Width - x
	}
	if bh > fs.Height-y {
		bh = fs.Height - y
	}

	// Build neighbor samples and run intra prediction.
	above := make([]uint8, bw)
	left := make([]uint8, bh)
	haveAbove := y > 0
	haveLeft := x > 0
	if haveAbove {
		for c := 0; c < bw; c++ {
			above[c] = fs.Y[(y-1)*fs.YStride+(x+c)]
		}
	}
	if haveLeft {
		for r := 0; r < bh; r++ {
			left[r] = fs.Y[(y+r)*fs.YStride+(x-1)]
		}
	}
	pred := make([]uint8, bw*bh)
	n := &Neighbors{
		Above:     above,
		Left:      left,
		HaveAbove: haveAbove,
		HaveLeft:  haveLeft,
		BitDepth:  8,
	}
	if haveAbove && haveLeft {
		n.AboveLeft = fs.Y[(y-1)*fs.YStride+(x-1)]
	}
	if err := PredictIntra(pred, bw, bh, yMode, n); err != nil {
		return err
	}

	if skip {
		// Zero residual — predicted samples are the final output.
		for r := 0; r < bh; r++ {
			for c := 0; c < bw; c++ {
				fs.Y[(y+r)*fs.YStride+(x+c)] = pred[r*bw+c]
			}
		}
		return nil
	}

	// Non-skip: read coefficients + dequant + inverse transform, then
	// add to the prediction. Currently limited to 4×4/8×8 TX blocks.
	if err := td.reconstructResidual(fs, pred, x, y, bw, bh); err != nil {
		return err
	}
	return nil
}

// reconstructResidual decodes the residual for a single-TX block (no TX
// splitting) and adds it to the prediction before writing to fs.Y. It
// supports TX_4X4 and TX_8X8 today; larger blocks return
// ErrCoeffDecodeUnimplemented.
func (td *TileDecoder) reconstructResidual(fs *FrameState, pred []uint8, x, y, bw, bh int) error {
	if td.coeff == nil {
		return fmt.Errorf("%w: coeff decoder not initialized", ErrCoeffDecodeUnimplemented)
	}
	var txSizeIdx int
	var nzMap []int8
	var scan []int
	switch {
	case bw == 4 && bh == 4:
		txSizeIdx = 0
		nzMap = cdfs.NzMapCtxOffset4x4[:]
		scan = transform.DefaultZigzagScan(4, 4)
	case bw == 8 && bh == 8:
		txSizeIdx = 1
		nzMap = cdfs.NzMapCtxOffset8x8[:]
		scan = transform.DefaultZigzagScan(8, 8)
	default:
		return fmt.Errorf("%w: residual decode only implemented for 4×4 and 8×8",
			ErrCoeffDecodeUnimplemented)
	}

	numCoeffs := bw * bh
	coeffs, err := td.coeff.ReadCoefficients(txSizeIdx, 0 /*luma*/, numCoeffs, scan, nzMap, bw, bh)
	if err != nil {
		return err
	}

	// Dequantize using base Y params; caller should apply the correct QP
	// once delta_q / segmentation are wired. For now, use BaseQIndex.
	qParams := quant.Params{
		BaseQIndex: int(td.fh.Quant.BaseQIndex),
		DeltaQYDc:  int(td.fh.Quant.DeltaQYDc),
		BitDepth:   int(td.sh.Color.BitDepth),
	}
	qv := qParams.Compute(quant.PlaneY)
	for i := range coeffs {
		coeffs[i] = DequantCoeff(coeffs[i], i, qv)
	}

	// Inverse 2D transform (DCT-DCT is the AVIF common case).
	var txSize transform.TxSize
	switch txSizeIdx {
	case 0:
		txSize = transform.Tx4x4
	case 1:
		txSize = transform.Tx8x8
	}
	if err := transform.Inverse2D(coeffs, transform.DctDct, txSize); err != nil {
		return err
	}

	// Reconstruct: pred + residual, clipped to 0..255.
	out := make([]uint8, bw*bh)
	ReconstructBlock(out, pred, coeffs, bw, bh)
	for r := 0; r < bh; r++ {
		for c := 0; c < bw; c++ {
			fs.Y[(y+r)*fs.YStride+(x+c)] = out[r*bw+c]
		}
	}
	return nil
}

// blockSizeLog returns the BSL (block-size-log) category for partition CDF
// indexing: 0 = 8×8, 1 = 16×16, 2 = 32×32, 3 = 64×64, 4 = 128×128.
func blockSizeLog(bs BlockSize) int {
	switch bs {
	case Block8x8:
		return 0
	case Block16x16:
		return 1
	case Block32x32:
		return 2
	case Block64x64:
		return 3
	case Block128x128:
		return 4
	}
	return 0
}
