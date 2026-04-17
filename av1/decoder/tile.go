package decoder

import (
	"errors"
	"fmt"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// ErrCoeffDecodeUnimplemented is returned when the tile decoder successfully
// reads partition + mode symbols but cannot yet decode coefficient levels.
var ErrCoeffDecodeUnimplemented = errors.New("av1/decoder: coefficient decoding not yet implemented")

// DecodedBlock holds the mode-level information decoded from the bitstream
// for a single coding block. Coefficient data is not yet populated.
type DecodedBlock struct {
	X, Y   int
	W, H   int
	Mode   IntraMode
	UVMode IntraMode // or CFL (13)
	Skip   bool
}

// TileDecoder reads AV1 symbol-coded syntax from a single tile's byte
// span using the entropy decoder and the default CDF tables.
type TileDecoder struct {
	dec    entropy.Decoder
	fh     *obu.FrameHeader
	sh     *obu.SequenceHeader
	sbSize int
	coeff  *CoeffDecoder
	inter  *InterDecoder // non-nil only when decoding an inter frame
	refY   []uint8       // reference luma plane for MC; nil for intra frames
	refU   []uint8
	refV   []uint8
	refY16 []uint16 // HBD reference luma plane; nil for 8-bit frames
	refU16 []uint16
	refV16 []uint16
	refW   int
	refH   int
	refYSt int
	refCSt int

	// CDFs — mutable copies for per-tile adaptation.
	partitionCDF  [20]cdfs.CDF
	kfYModeCDF    [5][5]cdfs.CDF
	uvModeCDF     [2][13]cdfs.CDF
	angleDeltaCDF [8]cdfs.CDF
	skipCDF       [3]cdfs.CDF
	cflSignCDF    cdfs.CDF
	cflAlphaCDF   [6]cdfs.CDF
	segCDF        [3]cdfs.CDF
}

// NewTileDecoder initializes a tile decoder for the given tile data.
func NewTileDecoder(tileData []byte, fh *obu.FrameHeader, sh *obu.SequenceHeader) (*TileDecoder, error) {
	return NewTileDecoderWithRef(tileData, fh, sh, nil)
}

// NewTileDecoderWithRef is the inter-aware form of [NewTileDecoder].
// For inter frames, ref provides the previously decoded frame used as
// the motion-compensation source. Pass nil for intra-only frames.
func NewTileDecoderWithRef(tileData []byte, fh *obu.FrameHeader, sh *obu.SequenceHeader, ref *Frame) (*TileDecoder, error) {
	td := &TileDecoder{
		fh: fh,
		sh: sh,
	}
	if sh.Use128x128Superblock {
		td.sbSize = 128
	} else {
		td.sbSize = 64
	}
	allowUpdate := !fh.DisableCDFUpdate
	if err := td.dec.Init(tileData, len(tileData), allowUpdate); err != nil {
		return nil, fmt.Errorf("tile decoder init: %w", err)
	}
	td.initCDFs()
	// Derive Q context from the frame's base_q_index (spec §7.12.4).
	qCtx := qIndexToCtx(int(fh.Quant.BaseQIndex))
	td.coeff = InitCoeffDecoder(&td.dec, qCtx)
	// Inter-frame state: hook up MC refs and instantiate the inter
	// block-syntax reader. We only support integer-pel MVs today so
	// allow_high_precision_mv is held at false; sub-pel MVs fire
	// the 8-tap filter but not the 1/8-pel hp bits.
	if !fh.FrameIsIntra && ref != nil {
		td.inter = NewInterDecoder(&td.dec, false)
		td.refY = ref.Y
		td.refU = ref.U
		td.refV = ref.V
		td.refY16 = ref.Y16
		td.refU16 = ref.U16
		td.refV16 = ref.V16
		td.refW = ref.Width
		td.refH = ref.Height
		td.refYSt = ref.YStride
		td.refCSt = ref.CStride
	}
	return td, nil
}

// qIndexToCtx maps base_q_index to the 4-way TOKEN_CDF_Q_CTXS bucket.
// Spec boundaries: 0..63 → 0, 64..127 → 1, 128..191 → 2, 192..255 → 3.
func qIndexToCtx(q int) int {
	switch {
	case q < 64:
		return 0
	case q < 128:
		return 1
	case q < 192:
		return 2
	}
	return 3
}

// initCDFs copies the default CDFs into mutable per-tile state. The entropy
// decoder updates them in-place when allow_update_cdf is true.
func (td *TileDecoder) initCDFs() {
	for i := range cdfs.DefaultPartitionCDF {
		td.partitionCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultPartitionCDF[i]...)
	}
	for a := range cdfs.DefaultKfYModeCDF {
		for l := range cdfs.DefaultKfYModeCDF[a] {
			td.kfYModeCDF[a][l] = append(cdfs.CDF(nil), cdfs.DefaultKfYModeCDF[a][l]...)
		}
	}
	for c := range cdfs.DefaultUVModeCDF {
		for m := range cdfs.DefaultUVModeCDF[c] {
			td.uvModeCDF[c][m] = append(cdfs.CDF(nil), cdfs.DefaultUVModeCDF[c][m]...)
		}
	}
	for i := range cdfs.DefaultAngleDeltaCDF {
		td.angleDeltaCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultAngleDeltaCDF[i]...)
	}
	for i := range cdfs.DefaultSkipCDF {
		td.skipCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultSkipCDF[i]...)
	}
	td.cflSignCDF = append(cdfs.CDF(nil), cdfs.DefaultCFLSignCDF...)
	for i := range cdfs.DefaultCFLAlphaCDF {
		td.cflAlphaCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultCFLAlphaCDF[i]...)
	}
	for i := range cdfs.DefaultSpatialPredSegTreeCDF {
		td.segCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultSpatialPredSegTreeCDF[i]...)
	}
}

// DecodeSegmentID reads the per-block segment_id symbol using the
// spatial-prediction CDF. ctx is the 3-way neighbor context per spec:
//
//	0 = neither neighbor available / both zero
//	1 = one neighbor non-zero
//	2 = both neighbors non-zero
func (td *TileDecoder) DecodeSegmentID(ctx int) uint8 {
	if ctx < 0 {
		ctx = 0
	}
	if ctx >= 3 {
		ctx = 2
	}
	return uint8(td.dec.DecodeSymbol(td.segCDF[ctx]))
}

// SegmentIDCtx returns the 3-way neighbor context for spatial segment
// prediction, following the spec's max-based rule: the number of
// distinct non-zero segment_id values among the available above and
// left neighbors, clamped to [0, 2].
func SegmentIDCtx(aboveID, leftID uint8, haveAbove, haveLeft bool) int {
	var count int
	if haveAbove && aboveID != 0 {
		count++
	}
	if haveLeft && leftID != 0 {
		count++
	}
	return count
}

// CFLJointSign represents the 8 possible joint-sign values for the U and
// V alpha components (spec §6.10.14): each of U and V gets one of {-, 0,
// +}, minus the (0, 0) case.
type CFLJointSign uint8

// CFLSigns splits a joint sign value into (sign_u, sign_v) where each
// element is -1, 0, or +1.
func CFLSigns(joint int) (su, sv int) {
	// Mapping per libaom's CFL_SIGN_POS / CFL_SIGN_NEG / CFL_SIGN_ZERO
	// and the 8-state joint table.
	switch joint {
	case 0:
		return -1, -1
	case 1:
		return -1, 0
	case 2:
		return -1, +1
	case 3:
		return 0, -1
	case 4:
		return 0, +1
	case 5:
		return +1, -1
	case 6:
		return +1, 0
	case 7:
		return +1, +1
	}
	return 0, 0
}

// ReadCFLSign reads the joint-sign symbol (0..7) from the bitstream.
func (td *TileDecoder) ReadCFLSign() int {
	return td.dec.DecodeSymbol(td.cflSignCDF)
}

// ReadCFLAlpha reads a single plane's alpha magnitude symbol (0..15);
// the caller adds 1 to get the actual magnitude (1..16 in Q3). ctx is
// the 6-way context derived from the joint sign, 0..5.
func (td *TileDecoder) ReadCFLAlpha(ctx int) int {
	if ctx < 0 {
		ctx = 0
	}
	if ctx >= 6 {
		ctx = 5
	}
	return td.dec.DecodeSymbol(td.cflAlphaCDF[ctx])
}

// CFLAlphaCtx returns the cfl_alpha CDF context index for a given joint
// sign + plane. The spec uses a precomputed 6-way mapping per libaom.
func CFLAlphaCtx(joint, plane int) int {
	// plane 0 = U, 1 = V. The context is simply the joint_sign if the
	// plane's sign is non-zero (4 joint signs have U non-zero, 4 have V
	// non-zero); otherwise it's irrelevant.
	su, sv := CFLSigns(joint)
	if plane == 0 {
		if su == 0 {
			return 0
		}
		// Index packing: 6 distinct "alpha present" contexts for U.
		// Precise spec mapping; use a small table:
		switch joint {
		case 0:
			return 0
		case 1:
			return 1
		case 2:
			return 2
		case 5:
			return 3
		case 6:
			return 4
		case 7:
			return 5
		}
	} else {
		if sv == 0 {
			return 0
		}
		switch joint {
		case 0:
			return 0
		case 3:
			return 1
		case 5:
			return 2
		case 2:
			return 3
		case 4:
			return 4
		case 7:
			return 5
		}
	}
	return 0
}

// DecodePartition reads a partition symbol for a square block of the given
// size class and left/above context.
func (td *TileDecoder) DecodePartition(bslCtx int, ctx int) int {
	cdfIdx := bslCtx*4 + ctx
	if cdfIdx >= len(td.partitionCDF) {
		return 0
	}
	return td.dec.DecodeSymbol(td.partitionCDF[cdfIdx])
}

// DecodeIntraYMode reads the Y-plane intra mode for a KEY_FRAME block.
// aboveCtx and leftCtx are the 5-bucket mode contexts of the neighbors.
func (td *TileDecoder) DecodeIntraYMode(aboveCtx, leftCtx int) IntraMode {
	if aboveCtx >= 5 {
		aboveCtx = 4
	}
	if leftCtx >= 5 {
		leftCtx = 4
	}
	return IntraMode(td.dec.DecodeSymbol(td.kfYModeCDF[aboveCtx][leftCtx]))
}

// DecodeUVMode reads the UV-plane intra mode given the Y mode and whether
// CFL is allowed.
func (td *TileDecoder) DecodeUVMode(yMode IntraMode, cflAllowed bool) IntraMode {
	cflIdx := 0
	if cflAllowed {
		cflIdx = 1
	}
	return IntraMode(td.dec.DecodeSymbol(td.uvModeCDF[cflIdx][yMode]))
}

// DecodeAngleDelta reads the angle delta for a directional mode.
// dirIdx is the directional mode index (yMode - D45Pred, in 0..7).
func (td *TileDecoder) DecodeAngleDelta(dirIdx int) int {
	if dirIdx < 0 || dirIdx >= 8 {
		return 0
	}
	return td.dec.DecodeSymbol(td.angleDeltaCDF[dirIdx]) - 3
}

// DecodeSkip reads the skip flag given a context index (0..2).
func (td *TileDecoder) DecodeSkip(ctx int) bool {
	if ctx >= 3 {
		ctx = 2
	}
	return td.dec.DecodeSymbol(td.skipCDF[ctx]) != 0
}

// Err returns any latched error from the entropy decoder.
func (td *TileDecoder) Err() error { return td.dec.Err() }
