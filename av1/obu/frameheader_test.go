package obu

import (
	"testing"
)

// buildReducedSeqHeader constructs a minimal reduced_still_picture_header
// for a 64x48 8-bit 4:2:0 image, all optional tools disabled. Returns the
// OBU payload (without OBU header / size).
func buildReducedSeqHeader(t *testing.T) []byte {
	t.Helper()
	w := &bw{}
	w.write(0, 3)  // seq_profile
	w.writeBit(1)  // still_picture
	w.writeBit(1)  // reduced_still_picture_header
	w.write(0, 5)  // seq_level_idx[0]
	w.write(5, 4)  // frame_width_bits_minus_1
	w.write(5, 4)  // frame_height_bits_minus_1
	w.write(63, 6) // max_frame_width_minus_1
	w.write(47, 6) // max_frame_height_minus_1
	w.writeBit(0)  // use_128x128_superblock
	w.writeBit(0)  // enable_filter_intra
	w.writeBit(0)  // enable_intra_edge_filter
	w.writeBit(0)  // enable_superres
	w.writeBit(0)  // enable_cdef
	w.writeBit(0)  // enable_restoration
	// color_config
	w.writeBit(0) // high_bitdepth
	w.writeBit(0) // monochrome
	w.writeBit(0) // color_description_present_flag
	w.writeBit(0) // color_range
	w.write(0, 2) // chroma_sample_position
	w.writeBit(0) // separate_uv_deltas_q
	w.writeBit(0) // film_grain_params_present
	w.trailing()
	return w.buf
}

// buildMinimalFrameHeader constructs a KEY_FRAME uncompressed header with
// base_q=0 (triggers all the lossless/disabled shortcuts), single tile,
// default color. Returns the OBU payload.
func buildMinimalFrameHeader(t *testing.T) []byte {
	t.Helper()
	w := &bw{}
	w.writeBit(0)   // disable_cdf_update
	w.writeBit(0)   // allow_screen_content_tools (seq was SELECT)
	// frame_size: frame_size_override=false per reduced, no bits here.
	// superres: enable_superres=false, no bit.
	w.writeBit(0)   // render_and_frame_size_different
	// allow_intrabc: AllowScreenContent=false, skipped.
	// disable_frame_end_update_cdf: reduced → implicit, no bit.
	// tile_info: one-tile image, only uniform_tile_spacing bit.
	w.writeBit(1)   // uniform_tile_spacing_flag
	// quantization_params:
	w.write(0, 8)   // base_q_index = 0
	w.writeBit(0)   // delta_q_y_dc present = 0
	w.writeBit(0)   // delta_q_u_dc present = 0
	w.writeBit(0)   // delta_q_u_ac present = 0
	w.writeBit(0)   // using_qmatrix = 0
	// segmentation_params:
	w.writeBit(0)   // seg_enabled = 0
	// delta_q_params, delta_lf_params: BaseQIndex==0 skips both.
	// loop_filter_params: CodedLosslessHint → early return.
	// cdef_params, lr_params: EnableCdef/Restoration = false → early return.
	// read_tx_mode: allLosslessHint → TxMode=Only4x4, no bit.
	// frame_reference_mode: FrameIsIntra → no bit.
	// skip_mode_params: always absent for intra.
	w.writeBit(0)   // reduced_tx_set
	// global_motion_params: FrameIsIntra → skipped.
	// film_grain_params: not present.
	w.trailing()
	return w.buf
}

func TestFrameHeaderMinimalKeyFrame(t *testing.T) {
	shBytes := buildReducedSeqHeader(t)
	sh, err := ParseSequenceHeader(shBytes)
	if err != nil {
		t.Fatalf("ParseSequenceHeader: %v", err)
	}
	if sh.SeqForceScreenContentTools != SelectScreenContentTools {
		t.Fatalf("expected reduced header to force SELECT_SCREEN_CONTENT_TOOLS, got %d", sh.SeqForceScreenContentTools)
	}

	fhBytes := buildMinimalFrameHeader(t)
	fh, err := ParseFrameHeader(fhBytes, sh, nil)
	if err != nil {
		t.Fatalf("ParseFrameHeader: %v", err)
	}
	if fh.FrameType != KeyFrame {
		t.Errorf("FrameType=%s, want KEY_FRAME", fh.FrameType)
	}
	if !fh.FrameIsIntra || !fh.ShowFrame {
		t.Errorf("FrameIsIntra=%v ShowFrame=%v", fh.FrameIsIntra, fh.ShowFrame)
	}
	if !fh.ErrorResilientMode {
		t.Errorf("ErrorResilientMode should be true for KEY_FRAME+ShowFrame")
	}
	if fh.FrameWidth != 64 || fh.FrameHeight != 48 {
		t.Errorf("dims=%dx%d, want 64x48", fh.FrameWidth, fh.FrameHeight)
	}
	if fh.Quant.BaseQIndex != 0 {
		t.Errorf("base_q_index=%d", fh.Quant.BaseQIndex)
	}
	if fh.Tile.TileCols != 1 || fh.Tile.TileRows != 1 {
		t.Errorf("tiles=%dx%d, want 1x1", fh.Tile.TileCols, fh.Tile.TileRows)
	}
	if fh.TxMode != TxModeOnly4x4 {
		t.Errorf("TxMode=%d, want Only4x4", fh.TxMode)
	}
	if fh.RefreshFrameFlags != 0xFF {
		t.Errorf("RefreshFrameFlags=%08b, want all-ones", fh.RefreshFrameFlags)
	}
}
