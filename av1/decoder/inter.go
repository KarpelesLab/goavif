package decoder

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/entropy"
	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
	"github.com/KarpelesLab/goavif/av1/predict"
)

// InterBlock carries the syntax decoded for one inter block plus the
// predicted samples after motion compensation. Callers still need to
// layer residual + clipping on top to produce the final pixels.
type InterBlock struct {
	IsInter     bool
	RefFrameIdx int // 0 = LAST (only ref we support)
	MV          MV
	SkipTxfm    bool
	InterpFilt  predict.InterpFilter
}

// InterDecoder wraps the entropy decoder with the CDFs used by the
// inter-frame block syntax reader. It's intentionally narrow —
// single-reference NEWMV / zero-MV blocks only. Compound prediction,
// global motion, warped motion, inter-intra, OBMC, and the full ref
// MV list machinery are not yet implemented and are flagged via
// ErrUnsupportedInterMode when their bits fire in the bitstream.
type InterDecoder struct {
	dec *entropy.Decoder
	mv  *MVDecoder

	isInterCDF    [4]cdfs.CDF
	skipModeCDF   [3]cdfs.CDF
	newMvCDF      [6]cdfs.CDF
	zeroMvCDF     [2]cdfs.CDF
	refMvCDF      [6]cdfs.CDF
	drlCDF        [3]cdfs.CDF
	singleRefCDF  [3][6]cdfs.CDF
	interpFiltCDF [16]cdfs.CDF
	yModeCDF      [4]cdfs.CDF
	skipCDF       [3]cdfs.CDF // reused intra skip CDF
}

// ErrUnsupportedInterMode is returned by [InterDecoder] when a syntax
// element outside the supported subset (single-ref, NEWMV / zero-MV,
// integer-pel, no compound / warp) is encountered.
var ErrUnsupportedInterMode = fmt.Errorf("av1/decoder: inter mode not yet supported")

// NewInterDecoder returns a fresh InterDecoder primed from libaom
// default CDFs. allowHighPrecMV forwards to the MV reader.
func NewInterDecoder(dec *entropy.Decoder, allowHighPrecMV bool) *InterDecoder {
	id := &InterDecoder{dec: dec, mv: InitMVDecoder(dec, allowHighPrecMV)}
	for i := 0; i < 4; i++ {
		id.isInterCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultIsInterCDF[i]...)
	}
	for i := 0; i < 3; i++ {
		id.skipModeCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultSkipModeCDF[i]...)
		id.drlCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultDrlCDF[i]...)
	}
	for i := 0; i < 6; i++ {
		id.newMvCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultNewMvCDF[i]...)
		id.refMvCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultRefMvCDF[i]...)
	}
	for i := 0; i < 2; i++ {
		id.zeroMvCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultZeroMvCDF[i]...)
	}
	for i := 0; i < 3; i++ {
		for j := 0; j < 6; j++ {
			id.singleRefCDF[i][j] = append(cdfs.CDF(nil), cdfs.DefaultSingleRefCDF[i][j]...)
		}
	}
	for i := 0; i < 16; i++ {
		id.interpFiltCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultInterpFilterCDF[i]...)
	}
	for i := 0; i < 4; i++ {
		id.yModeCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultYModeCDF[i]...)
	}
	for i := 0; i < 3; i++ {
		id.skipCDF[i] = append(cdfs.CDF(nil), cdfs.DefaultSkipCDF[i]...)
	}
	return id
}

// ReadIsInter returns true when the block is inter-predicted. The
// context is the sum of the above/left block's is_inter flags (0, 1,
// or 2 → mapped to 0, 1, 3 per libaom), clamped to 0..3.
func (id *InterDecoder) ReadIsInter(aboveIsInter, leftIsInter bool) bool {
	ctx := 0
	if aboveIsInter && leftIsInter {
		ctx = 3
	} else if aboveIsInter || leftIsInter {
		ctx = 1
	}
	return id.dec.DecodeSymbol(id.isInterCDF[ctx]) == 1
}

// ReadSingleRefFrame returns the reference frame index picked for a
// single-ref inter block. The AV1 single-ref binary tree has six
// decisions; we currently support only the "LAST" leaf (index 0).
// Other leaves return ErrUnsupportedInterMode so the outer decoder
// can bail cleanly.
func (id *InterDecoder) ReadSingleRefFrame(aboveCtx, leftCtx int) (int, error) {
	// Context buckets are 0..2 per spec; we simplify to 1 (neutral).
	ctx := 1
	// Bit 0: LAST vs others.
	if id.dec.DecodeSymbol(id.singleRefCDF[ctx][0]) == 0 {
		// Bit 1: LAST vs LAST2 — we only support LAST (index 0 == LAST).
		if id.dec.DecodeSymbol(id.singleRefCDF[ctx][1]) == 0 {
			return 0, nil // LAST
		}
		return 0, ErrUnsupportedInterMode
	}
	return 0, ErrUnsupportedInterMode
}

// ReadInterMode returns the four-way inter mode for a block: NEWMV,
// GLOBALMV, NEARESTMV, or NEARMV. We currently decode the bit-tree
// fully but the caller only supports NEWMV and zero-MV fallbacks.
func (id *InterDecoder) ReadInterMode(newMvCtx, zeroMvCtx, refMvCtx int) InterMode {
	if id.dec.DecodeSymbol(id.newMvCDF[newMvCtx]) == 0 {
		return InterModeNEWMV
	}
	if id.dec.DecodeSymbol(id.zeroMvCDF[zeroMvCtx]) == 0 {
		return InterModeGLOBALMV
	}
	if id.dec.DecodeSymbol(id.refMvCDF[refMvCtx]) == 0 {
		return InterModeNEARESTMV
	}
	return InterModeNEARMV
}

// ReadMV delegates to the MV decoder.
func (id *InterDecoder) ReadMV() MV { return id.mv.ReadMV() }

// ReadInterpFilter returns the per-block interpolation filter choice
// when the frame header sets InterpolationFilter=SWITCHABLE.
func (id *InterDecoder) ReadInterpFilter(ctx int) predict.InterpFilter {
	if ctx < 0 {
		ctx = 0
	}
	if ctx >= 16 {
		ctx = 15
	}
	return predict.InterpFilter(id.dec.DecodeSymbol(id.interpFiltCDF[ctx]))
}

// ReadYMode decodes the inter-frame intra Y-mode for blocks within an
// inter frame that are themselves intra-coded.
func (id *InterDecoder) ReadYMode(blockSizeGroup int) IntraMode {
	if blockSizeGroup < 0 {
		blockSizeGroup = 0
	}
	if blockSizeGroup >= 4 {
		blockSizeGroup = 3
	}
	return IntraMode(id.dec.DecodeSymbol(id.yModeCDF[blockSizeGroup]))
}

// ReadSkip decodes the skip_txfm flag for an inter block.
func (id *InterDecoder) ReadSkip(ctx int) bool {
	if ctx < 0 {
		ctx = 0
	}
	if ctx >= 3 {
		ctx = 2
	}
	return id.dec.DecodeSymbol(id.skipCDF[ctx]) == 1
}

// InterMode enumerates the four inter prediction modes after compound
// has been ruled out. AV1 also defines NEW_NEWMV / NEAR_NEW etc. for
// compound blocks — those are rejected in our narrow path.
type InterMode uint8

const (
	InterModeNEWMV     InterMode = 0
	InterModeGLOBALMV  InterMode = 1
	InterModeNEARESTMV InterMode = 2
	InterModeNEARMV    InterMode = 3
)
