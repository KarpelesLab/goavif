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
	w := bitio.NewWriter()

	// reduced_still_picture_header path:
	// show_existing_frame not coded
	// frame_type = KEY_FRAME (implicit)
	// show_frame = true (implicit)
	// error_resilient_mode = true (implicit, derived from KeyFrame+ShowFrame)

	// disable_cdf_update = 0
	w.F(1, 0)
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
	writeQuantParams(w, baseQIdx)

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
		writeLoopFilterParams(w)
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

	// film_grain_params: film_grain_params_present was 0 in seq header,
	// reads nothing.

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

// writeQuantParams writes quantization_params().
func writeQuantParams(w *bitio.Writer, baseQIdx uint8) {
	// base_q_idx (8 bits)
	w.F(8, uint32(baseQIdx))
	// diff_uv_delta = 0 in reduced mode — wait no, this depends on color
	// config. With SeparateUVDeltaQ=0 (which is our default), no extra
	// bits are coded here.
	// DeltaQYDc: 1-bit flag + su(6) if set. Set 0.
	w.F(1, 0) // delta_q_y_dc_flag = 0
	// DeltaQUDc: 1-bit flag. Set 0.
	w.F(1, 0)
	// DeltaQUAc: 1-bit flag. Set 0.
	w.F(1, 0)
	// Note: DeltaQVDc/DeltaQVAc coded only when SeparateUVDeltaQ=1.
	// using_qmatrix = 0
	w.F(1, 0)
}

// writeLoopFilterParams writes loop_filter_params(). For our minimal
// encoder all filter levels are 0 (filter disabled).
func writeLoopFilterParams(w *bitio.Writer) {
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
