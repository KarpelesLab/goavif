package obu

import "github.com/KarpelesLab/goavif/av1/bitio"

// WriteKeyFrameHeader produces a minimal uncompressed_header for a
// reduced_still_picture_header keyframe. Only the bit sequence that
// the parser will read back is emitted; fields with implicit values
// (frame_type = KEY_FRAME, show_frame = true, error_resilient_mode =
// true, etc.) are not coded.
//
// The emitted frame has:
//   - no screen-content tools
//   - no delta Q / delta LF
//   - loop filter levels 0 (filter disabled)
//   - CDEF / LR disabled (matches our sequence header)
//   - TxMode = TxModeLargest
//   - reduced_tx_set = 0
//   - film_grain_params = 0
//
// baseQIdx is the frame's base_q_index (0..255). A low value (e.g.,
// 32) gives mild quantization; higher values compress harder.
func WriteKeyFrameHeader(width, height int, baseQIdx uint8) []byte {
	return writeKeyFrameHeaderWith(width, height, baseQIdx, false, nil)
}

// WriteKeyFrameHeaderWithGrain is like [WriteKeyFrameHeader] but
// appends a film_grain_params block; callers must have set
// SeqWriteOpts.FilmGrainPresent=true when emitting the sequence
// header, otherwise the parser ignores the block.
func WriteKeyFrameHeaderWithGrain(width, height int, baseQIdx uint8, grain FilmGrainWriteOpts) []byte {
	return writeKeyFrameHeaderWith(width, height, baseQIdx, false, &grain)
}

// WriteMonoKeyFrameHeader emits a keyframe header for a monochrome
// sequence. Differs from [WriteKeyFrameHeader] only in the quantization
// param section (no chroma delta flags per spec §5.9.12).
func WriteMonoKeyFrameHeader(width, height int, baseQIdx uint8) []byte {
	return writeKeyFrameHeaderWith(width, height, baseQIdx, true, nil)
}

// WriteAVISKeyFrameHeader emits a keyframe header suitable for an
// AVIS sequence that pairs with [WriteSequenceHeaderAVIS]. Differs
// from [WriteKeyFrameHeader] in that it follows the non-reduced
// bit sequence: show_existing_frame is coded, the error_resilient
// bit is implicit rather than coded, screen-content + frame-size-
// override bits are present, and iref-less refresh_frame_flags are
// inferred rather than coded.
func WriteAVISKeyFrameHeader(width, height int, baseQIdx uint8) []byte {
	return writeAVISKeyFrameHeaderWith(width, height, baseQIdx, false)
}

// WriteMonoAVISKeyFrameHeader emits an AVIS keyframe header for a
// monochrome sequence. Differs from [WriteAVISKeyFrameHeader] only in
// that quantization_params skips the chroma delta-Q flags.
func WriteMonoAVISKeyFrameHeader(width, height int, baseQIdx uint8) []byte {
	return writeAVISKeyFrameHeaderWith(width, height, baseQIdx, true)
}

func writeAVISKeyFrameHeaderWith(width, height int, baseQIdx uint8, monochrome bool) []byte {
	w := bitio.NewWriter()

	// show_existing_frame = 0
	w.F(1, 0)
	// frame_type = 0 (KEY_FRAME)
	w.F(2, 0)
	// show_frame = 1
	w.F(1, 1)
	// error_resilient_mode: NOT coded for KeyFrame+ShowFrame (implicit true).

	// disable_cdf_update = 1
	w.F(1, 1)
	// allow_screen_content_tools (1) = 0 (seq SELECT)
	w.F(1, 0)
	// force_integer_mv not coded (allow_screen=0).

	// current_frame_id: FrameIDNumbersPresentFlag=0 → not coded.
	// frame_size_override (1) = 0
	w.F(1, 0)
	// order_hint bits not coded (disable in seq).
	// primary_ref_frame: KeyFrame → PRIMARY_REF_NONE, not coded.
	// refresh_frame_flags: KeyFrame+ShowFrame → all refs, not coded.
	// ref_order_hint loop: KeyFrame + refresh=allFrames → skipped.

	// Frame size + render size.
	// render_and_frame_size_different (1) = 0
	w.F(1, 0)
	// allow_intrabc: allow_screen=0 → not coded.

	// disable_frame_end_update_cdf (1) = 1
	w.F(1, 1)

	// Tile info.
	writeTileInfoSingle(w, width, height)
	// Quant / seg / delta_q / loop filter.
	writeQuantParams(w, baseQIdx, monochrome)
	// segmentation_enabled = 0
	w.F(1, 0)
	if baseQIdx > 0 {
		w.F(1, 0) // delta_q_present = 0
	}
	lossless := baseQIdx == 0
	if !lossless {
		writeLoopFilterParams(w, monochrome)
	}
	// cdef / lr skipped.
	if !lossless {
		w.F(1, 0) // tx_mode = LARGEST
	}
	// frame_reference_mode / skip_mode / global_motion NOT coded for
	// intra frames.
	// reduced_tx_set (1) = 0
	w.F(1, 0)
	// film_grain NOT coded (seq flag = 0).
	w.TrailingBits()
	return append([]byte(nil), w.Bytes()...)
}

// WriteInterFrameHeader emits a non-reduced uncompressed_header for
// an inter frame following an AVIS-form sequence header produced by
// [WriteSequenceHeaderAVIS]. The frame:
//
//   - is FrameType=INTER_FRAME, show_frame=true, error_resilient_mode=true
//   - refreshes every reference slot (refresh_frame_flags=0xFF)
//   - points every ref slot at reference buffer 0 (LAST = 0)
//   - picks InterpolationFilter=REGULAR (no switchable)
//   - has no global motion (IDENTITY for every ref)
//   - disables skip_mode / reference_select (single reference)
//
// baseQIdx sets base_q_index.
func WriteInterFrameHeader(width, height int, baseQIdx uint8) []byte {
	return writeInterFrameHeaderWith(width, height, baseQIdx, false)
}

// WriteMonoInterFrameHeader emits an AVIS inter frame header for a
// monochrome sequence. Differs from [WriteInterFrameHeader] only in
// that quantization_params skips the chroma delta-Q flags.
func WriteMonoInterFrameHeader(width, height int, baseQIdx uint8) []byte {
	return writeInterFrameHeaderWith(width, height, baseQIdx, true)
}

func writeInterFrameHeaderWith(width, height int, baseQIdx uint8, monochrome bool) []byte {
	w := bitio.NewWriter()

	// show_existing_frame = 0
	w.F(1, 0)
	// frame_type = 1 (INTER_FRAME)
	w.F(2, 1)
	// show_frame = 1
	w.F(1, 1)
	// error_resilient_mode = 1
	w.F(1, 1)

	// disable_cdf_update = 1
	w.F(1, 1)
	// seq_force_screen_content_tools = SELECT, so allow_screen_content (1 bit)
	w.F(1, 0) // no screen content
	// force_integer_mv not coded when allow_screen=0.

	// frame_size_override = 0
	w.F(1, 0)
	// order_hint bits not coded (enable_order_hint=0)
	// primary_ref_frame: not coded because error_resilient_mode=1
	// refresh_frame_flags (8) = 0xFF
	w.F(8, 0xFF)
	// ref_order_hint loop: only coded if ErrorResilientMode &&
	// EnableOrderHint. EnableOrderHint=0 → skip.

	// Inter branch.
	// frame_refs_short_signaling: only if enable_order_hint. Skip.
	for i := 0; i < 7; i++ {
		w.F(3, 0) // ref_frame_idx[i] = 0 (all point at buffer 0)
	}
	// frame_id deltas skipped (FrameIDNumbersPresentFlag=0)
	// frame_size_override=0 → render_and_frame_size_different (1) = 0
	w.F(1, 0)
	// allow_high_precision_mv (1) = 0 (ForceIntegerMV=false since no screen content)
	w.F(1, 0)
	// is_filter_switchable (1) = 0 → non-switchable, emit 2 bits for REGULAR.
	w.F(1, 0)
	w.F(2, 0) // InterpolationFilter = REGULAR
	// is_motion_mode_switchable (1) = 0
	w.F(1, 0)
	// use_ref_frame_mvs: not coded since enable_order_hint=0.

	// disable_frame_end_update_cdf (1) = 1
	w.F(1, 1)

	// Tile info.
	writeTileInfoSingle(w, width, height)

	// Quant params.
	writeQuantParams(w, baseQIdx, monochrome)

	// segmentation_enabled (1) = 0
	w.F(1, 0)

	// delta_q_params.
	if baseQIdx > 0 {
		w.F(1, 0) // delta_q_present = 0
	}
	// delta_lf_params skipped (delta_q_present=0).

	// Loop filter params — skipped when lossless; we set all zero.
	lossless := baseQIdx == 0
	if !lossless {
		writeLoopFilterParams(w, monochrome)
	}

	// CDEF / LR params: seq disables both → nothing coded.

	// tx_mode (1 bit unless lossless)
	if !lossless {
		w.F(1, 0) // tx_mode = TxModeLargest
	}

	// frame_reference_mode: reference_select (1) = 0 (single ref).
	w.F(1, 0)

	// skip_mode_params: skip_mode_allowed is false (no order_hint →
	// no forward/backward ref derivation), so nothing coded.

	// reduced_tx_set (1) = 0
	w.F(1, 0)

	// global_motion_params: for each of 7 refs, is_global (1) = 0 so
	// the ref stays IDENTITY; no further bits for IDENTITY.
	for i := 0; i < 7; i++ {
		w.F(1, 0)
	}

	// film_grain_params: seq.FilmGrainParamsPresent=0 → skipped.

	w.TrailingBits()
	return append([]byte(nil), w.Bytes()...)
}

func writeKeyFrameHeaderWith(width, height int, baseQIdx uint8, monochrome bool, grain *FilmGrainWriteOpts) []byte {
	w := bitio.NewWriter()

	// reduced_still_picture_header path:
	// show_existing_frame not coded
	// frame_type = KEY_FRAME (implicit)
	// show_frame = true (implicit)
	// error_resilient_mode = true (implicit, derived from KeyFrame+ShowFrame)

	// disable_cdf_update = 1 — the baseline encoder does not track
	// adaptive CDF state across symbols, so we explicitly turn off
	// the decoder's adaptation too.
	w.F(1, 1)
	// Screen content tools: SeqForceScreenContentTools is SELECT in our
	// sequence header's reduced mode (not coded there), so default to 0.
	// Actually, per spec in reduced mode, SeqForceScreenContentTools is
	// inferred as SELECT_SCREEN_CONTENT_TOOLS = 2, so allow_screen_content
	// IS coded here.
	w.F(1, 0) // allow_screen_content_tools = 0
	// ForceIntegerMV path: not coded since allow_screen_content_tools=0 and
	// the frame is intra (force_integer_mv derived as true).
	// FrameIDNumbersPresentFlag was 0 in the seq header, so current_frame_id
	// is not coded.
	// FrameSizeOverride: in reduced mode it's inferred as false, not coded.
	// OrderHint: enable_order_hint=0 in reduced mode, so order_hint bits=0.
	// PrimaryRefFrame: intra-only, not coded.
	// DecoderModelInfoPresent: 0, not coded.
	// RefreshFrameFlags: keyframe+showframe → allFrames (implicit).
	// Ref ordering: not coded.

	// Frame & render size. This path is entered for intra frames.
	// parseFrameAndRenderSize flow: since FrameSizeOverride=false and no
	// superres & no render_size_different flag, nothing is coded here
	// either. Actually let me be more careful — check the parser.
	// In reduced mode FrameSizeOverride=false → frame_size() reads nothing
	// (width/height come from sequence header max). Then superres_params
	// is skipped (seq.EnableSuperres=0). Then render_size reads 1 bit
	// (render_and_frame_size_different).
	w.F(1, 0) // render_and_frame_size_different = 0

	// allow_intrabc: requires AllowScreenContent && not-superres. We
	// allow_screen_content_tools=0 so not coded.

	// DisableFrameEndUpdateCDF implicit=true in reduced mode (not coded).

	// Tile info.
	writeTileInfoSingle(w, width, height)

	// Quantization.
	writeQuantParams(w, baseQIdx, monochrome)

	// Segmentation params: segmentation_enabled (1 bit) + depending on value
	// further fields. For a simple encoder set all to 0.
	w.F(1, 0) // segmentation_enabled = 0

	// delta_q_params: BaseQIndex > 0 → delta_q_present (1 bit). Set 0.
	if baseQIdx > 0 {
		w.F(1, 0) // delta_q_present = 0
	}
	// delta_lf_params: only if delta_q_present, we set 0 → skip.

	// LoopFilter params — skipped when CodedLossless is true (all Q = 0).
	lossless := baseQIdx == 0
	if !lossless {
		writeLoopFilterParams(w, monochrome)
	}

	// CDEF: enable_cdef was 0 in seq header, so this reads nothing.
	// LR: enable_restoration was 0, reads nothing.

	// TxMode: 1 bit unless allLossless (which is true iff CodedLossless
	// and TxSize > 4x4 constraint — simplified here to lossless).
	if !lossless {
		w.F(1, 0) // tx_mode = TxModeLargest
	}

	// frame_reference_mode: not coded for intra frames.

	// skip_mode_params: not coded for intra frames.

	// reduced_tx_set = 0
	w.F(1, 0)

	// global_motion_params: not coded for intra frames.

	// film_grain_params: emitted only when seq has
	// film_grain_params_present=1 (i.e. grain != nil).
	if grain != nil {
		WriteFilmGrainParams(w, false /* isInter */, *grain)
	}

	w.TrailingBits()
	return append([]byte(nil), w.Bytes()...)
}

// writeTileInfoSingle writes tile_info() for a single-tile frame. The
// parser loops TileColsLog2 from minLog2TileCols to maxLog2TileCols,
// reading one bit per iteration: 1 = increment, 0 = break. We emit
// one 0 bit to take minLog2TileCols as the final value; if min == max
// no bit is coded at all.
func writeTileInfoSingle(w *bitio.Writer, width, height int) {
	// uniform_tile_spacing_flag = 1 (single tile)
	w.F(1, 1)
	miCols := uint32((width + 7) >> 3)
	miRows := uint32((height + 7) >> 3)
	sbSize := uint32(64) // matches our seq header's use_128x128_superblock=0
	sbCols := (miCols + (sbSize>>3) - 1) / (sbSize >> 3)
	sbRows := (miRows + (sbSize>>3) - 1) / (sbSize >> 3)
	maxTileWidthSb := uint32(MaxTileWidth) / sbSize
	minLog2Cols := tileLog2(maxTileWidthSb, sbCols)
	maxLog2Cols := tileLog2(1, minU32(sbCols, MaxTileCols))
	if minLog2Cols < maxLog2Cols {
		// Emit a single 0 to break at minLog2Cols.
		w.F(1, 0)
	}
	// For tile rows, compute minLog2Rows. It depends on minLog2Tiles which
	// uses maxTileAreaSb. For small frames with 1-tile, minLog2Rows = 0.
	maxTileAreaSb := uint32(4096*2304) / (sbSize * sbSize)
	minLog2Tiles := maxU32(
		tileLog2(maxTileWidthSb, sbCols),
		tileLog2(maxTileAreaSb, sbRows*sbCols),
	)
	minLog2TileRows := uint32(0)
	if minLog2Tiles > minLog2Cols {
		minLog2TileRows = minLog2Tiles - minLog2Cols
	}
	maxLog2Rows := tileLog2(1, minU32(sbRows, MaxTileRows))
	if minLog2TileRows < maxLog2Rows {
		w.F(1, 0)
	}
	// TileColsLog2 = 0, TileRowsLog2 = 0 → no context_update_tile_id or
	// tile_size_bytes fields are coded.
}

// writeQuantParams writes quantization_params(). When monochrome is
// true, the chroma delta-Q flags are skipped per spec §5.9.12, matching
// the parser's NumPlanes>1 gate.
func writeQuantParams(w *bitio.Writer, baseQIdx uint8, monochrome bool) {
	// base_q_idx (8 bits)
	w.F(8, uint32(baseQIdx))
	// DeltaQYDc: 1-bit flag. Set 0.
	w.F(1, 0)
	if !monochrome {
		// DeltaQUDc: 1-bit flag. Set 0.
		w.F(1, 0)
		// DeltaQUAc: 1-bit flag. Set 0.
		w.F(1, 0)
		// Note: DeltaQVDc/DeltaQVAc coded only when SeparateUVDeltaQ=1.
	}
	// using_qmatrix = 0
	w.F(1, 0)
}

// writeLoopFilterParams writes loop_filter_params(). For our minimal
// encoder all filter levels are 0 (filter disabled). The monochrome
// argument is accepted for symmetry with the parser, but since filter
// levels are all zero, LevelU/V would be skipped in either branch.
func writeLoopFilterParams(w *bitio.Writer, monochrome bool) {
	_ = monochrome
	// filter_level[0], filter_level[1] (6 bits each)
	w.F(6, 0)
	w.F(6, 0)
	// Per spec §5.9.11: if filter_level[0] or filter_level[1] != 0,
	// then filter_level_u, filter_level_v are coded. They are 0 so skip.
	// sharpness (3 bits)
	w.F(3, 0)
	// mode_ref_delta_enabled (1 bit)
	w.F(1, 0)
	// mode_ref_delta_update not coded since enabled=0.
}
