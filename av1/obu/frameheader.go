package obu

import (
	"fmt"

	"github.com/KarpelesLab/goavif/av1/bitio"
)

// FrameHeader is the decoded uncompressed_header (spec §5.9).
//
// The struct currently covers the intra-only (KEY_FRAME / INTRA_ONLY_FRAME)
// decode path with and without reduced_still_picture_header. Inter-frame
// fields (motion mode, global motion refs, etc.) are present in the struct
// but only partially populated.
type FrameHeader struct {
	ShowExistingFrame   bool
	FrameToShowMapIdx   uint8
	DisplayFrameID      uint32
	FrameType           FrameType
	ShowFrame           bool
	ShowableFrame       bool
	FrameIsIntra        bool
	ErrorResilientMode  bool
	DisableCDFUpdate    bool
	AllowScreenContent  bool
	ForceIntegerMV      bool
	CurrentFrameID      uint32
	FrameSizeOverride   bool
	OrderHint           uint8
	PrimaryRefFrame     uint8
	RefreshFrameFlags   uint8
	RefOrderHint        [NumRefFrames]uint8

	FrameWidth   uint32
	FrameHeight  uint32
	UpscaledWidth uint32
	SuperresDenom uint8 // 8..16 (SUPERRES_NUM=8)
	RenderWidth  uint32
	RenderHeight uint32

	AllowIntrabc bool

	DisableFrameEndUpdateCDF bool

	Tile           TileInfo
	Quant          QuantizationParams
	Segmentation   SegmentationParams
	DeltaQPresent  bool
	DeltaQRes      uint8
	DeltaLFPresent bool
	DeltaLFRes     uint8
	DeltaLFMulti   bool
	LoopFilter     LoopFilterParams
	Cdef           CdefParams
	LR             LoopRestorationParams
	TxMode         TxMode
	ReferenceSelect bool
	SkipModePresent bool
	ReducedTxSet    bool
	AllowWarpedMotion bool
	GmType         [NumRefFrames]uint8 // global motion type per ref frame

	// Inter-frame fields.
	RefFrameIdx            [NumRefFramesPerFrame]uint8
	AllowHighPrecisionMV   bool
	InterpolationFilter    uint8 // 0..3 = filter index, or InterpolationFilterSwitchable
	IsMotionModeSwitchable bool
	UseRefFrameMVs         bool

	FilmGrain FilmGrainParams
}

// NumRefFramesPerFrame is the fixed AV1 count of references per
// inter frame: LAST, LAST2, LAST3, GOLDEN, BWDREF, ALTREF2, ALTREF.
const NumRefFramesPerFrame = 7

// InterpolationFilterSwitchable signals per-block interpolation filter
// selection via the switchable_interp CDF. Values 0..3 correspond to
// REGULAR, SMOOTH, SHARP, BILINEAR.
const InterpolationFilterSwitchable uint8 = 4

// Superres denominator constants (spec §3).
const (
	SuperresNum        = 8
	SuperresDenomMin   = 9
	SuperresDenomBits  = 3 // 3 bits encodes denom_minus_9 ∈ [0,7]
)

// TxMode per spec §6.8.21.
type TxMode uint8

const (
	TxModeOnly4x4 TxMode = 0
	TxModeLargest TxMode = 1
	TxModeSelect  TxMode = 2
)

// ParseFrameHeader decodes the OBU_FRAME_HEADER or the header portion of an
// OBU_FRAME. seqHdr must be the previously parsed sequence header.
//
// refInfo may be nil for the still-image case (no persistent reference frame
// state). When present, it provides the fields needed to fully process
// show_existing_frame and inter-frame paths.
func ParseFrameHeader(payload []byte, seqHdr *SequenceHeader, refInfo *RefFrameState) (*FrameHeader, error) {
	fh, _, err := ParseFrameHeaderBytes(payload, seqHdr, refInfo)
	return fh, err
}

// ParseFrameHeaderBytes is like [ParseFrameHeader] but also returns the
// number of bytes consumed from payload. This lets a caller split an
// OBU_FRAME into its uncompressed-header portion and the tile group that
// follows: tileGroup = payload[consumed:].
func ParseFrameHeaderBytes(payload []byte, seqHdr *SequenceHeader, refInfo *RefFrameState) (*FrameHeader, int, error) {
	r := bitio.NewReader(payload)
	fh := &FrameHeader{}
	if err := parseUncompressedHeader(r, fh, seqHdr, refInfo); err != nil {
		return nil, 0, err
	}
	if err := r.TrailingBits(); err != nil {
		return nil, 0, fmt.Errorf("%w: frame header trailing bits: %w", ErrMalformed, err)
	}
	return fh, int(r.BytePos()), nil
}

// RefFrameState carries inter-frame persistence needed to parse headers of
// frames that reference earlier ones. For a single-still-image decode it can
// be nil.
type RefFrameState struct {
	RefValid        [NumRefFrames]bool
	RefFrameType    [NumRefFrames]FrameType
	RefOrderHint    [NumRefFrames]uint8
	RefFrameID      [NumRefFrames]uint32
	RefUpscaledW    [NumRefFrames]uint32
	RefFrameHeight  [NumRefFrames]uint32
	RefRenderW      [NumRefFrames]uint32
	RefRenderH      [NumRefFrames]uint32
}

func parseUncompressedHeader(r *bitio.Reader, fh *FrameHeader, sh *SequenceHeader, refs *RefFrameState) error {
	idLen := uint(0)
	if sh.FrameIDNumbersPresentFlag {
		idLen = uint(sh.AdditionalFrameIDLengthMinusOne) + uint(sh.DeltaFrameIDLengthMinusTwo) + 3
	}
	allFrames := uint8((1 << NumRefFrames) - 1)

	if sh.ReducedStillPictureHeader {
		fh.ShowExistingFrame = false
		fh.FrameType = KeyFrame
		fh.FrameIsIntra = true
		fh.ShowFrame = true
		fh.ShowableFrame = false
	} else {
		fh.ShowExistingFrame = r.F(1) == 1
		if fh.ShowExistingFrame {
			fh.FrameToShowMapIdx = uint8(r.F(3))
			if sh.DecoderModelInfoPresentFlag && !sh.TimingInfo.EqualPictureInterval {
				// temporal_point_info — consume but do not store.
				_ = r.F64(uint(sh.DecoderModelInfo.FramePresentationTimeLengthMinusOne) + 1)
			}
			fh.RefreshFrameFlags = 0
			if sh.FrameIDNumbersPresentFlag {
				fh.DisplayFrameID = uint32(r.F(idLen))
			}
			if refs != nil {
				fh.FrameType = refs.RefFrameType[fh.FrameToShowMapIdx]
			}
			if fh.FrameType == KeyFrame {
				fh.RefreshFrameFlags = allFrames
			}
			if sh.FilmGrainParamsPresent {
				// Load grain params from the displayed ref — we skip for now.
			}
			return nil
		}
		fh.FrameType = FrameType(r.F(2))
		fh.FrameIsIntra = fh.FrameType.IsIntra()
		fh.ShowFrame = r.F(1) == 1
		if fh.ShowFrame && sh.DecoderModelInfoPresentFlag && !sh.TimingInfo.EqualPictureInterval {
			_ = r.F64(uint(sh.DecoderModelInfo.FramePresentationTimeLengthMinusOne) + 1)
		}
		if fh.ShowFrame {
			fh.ShowableFrame = fh.FrameType != KeyFrame
		} else {
			fh.ShowableFrame = r.F(1) == 1
		}
	}

	// error_resilient_mode derivation (spec §5.9.1).
	if fh.FrameType == SwitchFrame || (fh.FrameType == KeyFrame && fh.ShowFrame) {
		fh.ErrorResilientMode = true
	} else if !sh.ReducedStillPictureHeader {
		fh.ErrorResilientMode = r.F(1) == 1
	}

	if fh.FrameType == KeyFrame && fh.ShowFrame && refs != nil {
		for i := 0; i < NumRefFrames; i++ {
			refs.RefValid[i] = false
			refs.RefOrderHint[i] = 0
		}
	}

	fh.DisableCDFUpdate = r.F(1) == 1

	if sh.SeqForceScreenContentTools == SelectScreenContentTools {
		fh.AllowScreenContent = r.F(1) == 1
	} else {
		fh.AllowScreenContent = sh.SeqForceScreenContentTools == 1
	}
	if fh.AllowScreenContent {
		if sh.SeqForceIntegerMV == SelectIntegerMV {
			fh.ForceIntegerMV = r.F(1) == 1
		} else {
			fh.ForceIntegerMV = sh.SeqForceIntegerMV == 1
		}
	} else {
		fh.ForceIntegerMV = false
	}
	if fh.FrameIsIntra {
		fh.ForceIntegerMV = true
	}

	if sh.FrameIDNumbersPresentFlag {
		fh.CurrentFrameID = uint32(r.F(idLen))
	}

	switch fh.FrameType {
	case SwitchFrame:
		fh.FrameSizeOverride = true
	default:
		if sh.ReducedStillPictureHeader {
			fh.FrameSizeOverride = false
		} else {
			fh.FrameSizeOverride = r.F(1) == 1
		}
	}

	orderHintBits := uint(0)
	if sh.EnableOrderHint {
		orderHintBits = uint(sh.OrderHintBitsMinusOne) + 1
	}
	if orderHintBits > 0 {
		fh.OrderHint = uint8(r.F(orderHintBits))
	}

	if fh.FrameIsIntra || fh.ErrorResilientMode {
		fh.PrimaryRefFrame = PrimaryRefNone
	} else {
		fh.PrimaryRefFrame = uint8(r.F(3))
	}

	if sh.DecoderModelInfoPresentFlag {
		if r.F(1) == 1 { // buffer_removal_time_present_flag
			for i := range sh.OperatingPoints {
				op := &sh.OperatingPoints[i]
				if op.DecoderModelPresentForThisOP {
					opPtIdc := op.IDC
					// Inhibition check per spec; we consume the time regardless.
					if opPtIdc == 0 || (uint16(fh.temporalID())&opPtIdc) != 0 { //nolint:staticcheck
						_ = r.F(uint(sh.DecoderModelInfo.BufferRemovalTimeLengthMinusOne) + 1)
					}
				}
			}
		}
	}

	if fh.FrameType == SwitchFrame || (fh.FrameType == KeyFrame && fh.ShowFrame) {
		fh.RefreshFrameFlags = allFrames
	} else {
		fh.RefreshFrameFlags = uint8(r.F(NumRefFrames))
	}

	if !fh.FrameIsIntra || fh.RefreshFrameFlags != allFrames {
		if fh.ErrorResilientMode && sh.EnableOrderHint {
			for i := 0; i < NumRefFrames; i++ {
				fh.RefOrderHint[i] = uint8(r.F(orderHintBits))
			}
		}
	}

	if fh.FrameIsIntra {
		if err := parseFrameAndRenderSize(r, fh, sh); err != nil {
			return err
		}
		if fh.AllowScreenContent && fh.UpscaledWidth == fh.FrameWidth {
			fh.AllowIntrabc = r.F(1) == 1
		}
	} else {
		// Inter-frame branch per spec §5.9.9 — simplified for the
		// AVIF AVIS use case: no frame_refs_short_signaling, no
		// frame_id deltas, no warp / global motion bits.
		var frameRefsShortSignaling bool
		if sh.EnableOrderHint {
			frameRefsShortSignaling = r.F(1) == 1
			if frameRefsShortSignaling {
				// last_frame_idx / gold_frame_idx — 3 bits each; the
				// remaining ref indices are derived. We consume the
				// bits for syntax alignment but don't compute the
				// derived indices (unused in our narrow path).
				_ = r.F(3) // last_frame_idx
				_ = r.F(3) // gold_frame_idx
			}
		}
		for i := 0; i < NumRefFramesPerFrame; i++ {
			if !frameRefsShortSignaling {
				fh.RefFrameIdx[i] = uint8(r.F(3))
			}
			if sh.FrameIDNumbersPresentFlag {
				deltaLen := uint(sh.DeltaFrameIDLengthMinusTwo) + 2
				_ = r.F(deltaLen)
			}
		}
		if fh.FrameSizeOverride && !fh.ErrorResilientMode {
			// frame_size_with_refs — not commonly used; fall back to
			// explicit frame size.
			if err := parseFrameAndRenderSize(r, fh, sh); err != nil {
				return err
			}
		} else {
			if err := parseFrameAndRenderSize(r, fh, sh); err != nil {
				return err
			}
		}
		if !fh.ForceIntegerMV {
			fh.AllowHighPrecisionMV = r.F(1) == 1
		}
		// read_interpolation_filter.
		if r.F(1) == 1 {
			fh.InterpolationFilter = InterpolationFilterSwitchable
		} else {
			fh.InterpolationFilter = uint8(r.F(2))
		}
		// is_motion_mode_switchable (1 bit).
		fh.IsMotionModeSwitchable = r.F(1) == 1
		// use_ref_frame_mvs (1 bit, only if enable_order_hint).
		if sh.EnableOrderHint && !fh.ErrorResilientMode {
			fh.UseRefFrameMVs = r.F(1) == 1
		}
	}

	if sh.ReducedStillPictureHeader {
		fh.DisableFrameEndUpdateCDF = true
	} else {
		fh.DisableFrameEndUpdateCDF = r.F(1) == 1
	}

	// Tile info.
	if err := parseTileInfo(r, &fh.Tile, sh, fh); err != nil {
		return err
	}
	parseQuantizationParams(r, &fh.Quant, sh)
	parseSegmentationParams(r, &fh.Segmentation, fh)

	// delta_q_params
	fh.DeltaQRes = 0
	fh.DeltaQPresent = false
	if fh.Quant.BaseQIndex > 0 {
		fh.DeltaQPresent = r.F(1) == 1
	}
	if fh.DeltaQPresent {
		fh.DeltaQRes = uint8(r.F(2))
	}

	// delta_lf_params
	fh.DeltaLFPresent = false
	fh.DeltaLFRes = 0
	fh.DeltaLFMulti = false
	if fh.DeltaQPresent {
		if !fh.AllowIntrabc {
			fh.DeltaLFPresent = r.F(1) == 1
		}
		if fh.DeltaLFPresent {
			fh.DeltaLFRes = uint8(r.F(2))
			fh.DeltaLFMulti = r.F(1) == 1
		}
	}

	// CodedLossless / AllLossless are derived later in decode; we capture raw bits.

	parseLoopFilterParams(r, &fh.LoopFilter, fh, sh)
	parseCdefParams(r, &fh.Cdef, sh, fh)
	parseLoopRestorationParams(r, &fh.LR, sh, fh)

	// read_tx_mode
	if fh.allLosslessHint() {
		fh.TxMode = TxModeOnly4x4
	} else {
		if r.F(1) == 1 {
			fh.TxMode = TxModeSelect
		} else {
			fh.TxMode = TxModeLargest
		}
	}

	// frame_reference_mode
	if fh.FrameIsIntra {
		fh.ReferenceSelect = false
	} else {
		fh.ReferenceSelect = r.F(1) == 1
	}

	// skip_mode_params — intra-only frames always have skip_mode_frame not present.
	fh.SkipModePresent = false
	if !fh.FrameIsIntra && fh.ReferenceSelect && sh.EnableOrderHint {
		// Determined by order-hint logic; not exercised for intra-only.
	}

	// reduced_tx_set
	fh.ReducedTxSet = r.F(1) == 1

	// global_motion_params
	if !fh.FrameIsIntra {
		parseGlobalMotionParams(r, fh)
	}

	// film_grain_params
	parseFilmGrainParams(r, &fh.FilmGrain, sh, fh)

	if err := r.Err(); err != nil {
		return fmt.Errorf("%w: uncompressed header: %w", ErrMalformed, err)
	}
	return nil
}

// temporalID returns the temporal_id of the OBU enclosing this header. It is
// threaded in from the caller via a lightweight shim; for still-image AVIF
// this is always zero and buffer_removal paths never fire. We keep the
// method so the decoder_model_info path compiles.
func (fh *FrameHeader) temporalID() uint8 { return 0 }

// allLosslessHint reports whether every active segment has an effective
// lossless configuration. The spec defines this via a deeper walk, but for
// frame-header parsing purposes we treat base-q 0 with lossless-ready delta
// as a reliable lower bound. Full lossless tracking is handled by the
// decoder, not by this header parser.
func (fh *FrameHeader) allLosslessHint() bool {
	return fh.Quant.BaseQIndex == 0 &&
		fh.Quant.DeltaQYDc == 0 &&
		fh.Quant.DeltaQUDc == 0 &&
		fh.Quant.DeltaQUAc == 0 &&
		fh.Quant.DeltaQVDc == 0 &&
		fh.Quant.DeltaQVAc == 0
}

// parseFrameAndRenderSize handles frame_size() + superres_params() +
// render_size() per spec §5.9.5–7 for the intra-only path.
func parseFrameAndRenderSize(r *bitio.Reader, fh *FrameHeader, sh *SequenceHeader) error {
	var w, h uint32
	if fh.FrameSizeOverride {
		w = uint32(r.F(uint(sh.FrameWidthBitsMinusOne)+1)) + 1
		h = uint32(r.F(uint(sh.FrameHeightBitsMinusOne)+1)) + 1
	} else {
		w = sh.MaxFrameWidthMinusOne + 1
		h = sh.MaxFrameHeightMinusOne + 1
	}
	// superres_params()
	fh.SuperresDenom = SuperresNum
	if sh.EnableSuperres {
		if r.F(1) == 1 { // use_superres
			fh.SuperresDenom = uint8(r.F(SuperresDenomBits)) + SuperresDenomMin
		}
	}
	fh.UpscaledWidth = w
	fh.FrameWidth = (w*SuperresNum + uint32(fh.SuperresDenom)/2) / uint32(fh.SuperresDenom)
	fh.FrameHeight = h

	// render_size()
	if r.F(1) == 1 { // render_and_frame_size_different
		fh.RenderWidth = uint32(r.F(16)) + 1
		fh.RenderHeight = uint32(r.F(16)) + 1
	} else {
		fh.RenderWidth = fh.UpscaledWidth
		fh.RenderHeight = fh.FrameHeight
	}
	return nil
}
