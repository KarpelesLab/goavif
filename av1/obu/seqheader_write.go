package obu

import (
	"bytes"

	"github.com/KarpelesLab/goavif/av1/bitio"
)

// WriteSequenceHeader serializes a minimal AVIF-oriented sequence
// header to a byte stream. The output uses reduced_still_picture_header
// mode (§5.5.1) which is the AVIF preferred form: one operating
// point at level 0, no decoder model, no timing info.
//
// Only 8-bit + 4:2:0 + still-picture profile 0 is covered for now;
// HBD (10/12-bit) and non-4:2:0 variants land with the full encoder.
func WriteSequenceHeader(width, height int) []byte {
	w := bitio.NewWriter()

	// seq_profile = 0 (3 bits)
	w.F(3, 0)
	// still_picture = 1
	w.F(1, 1)
	// reduced_still_picture_header = 1
	w.F(1, 1)
	// seq_level_idx[0] = 0 (5 bits)
	w.F(5, 0)

	// frame_width_bits_minus_1, frame_height_bits_minus_1 (4 bits each).
	// Use 16-bit encoding for simplicity — covers up to 65535x65535.
	w.F(4, 15) // frame_width_bits_minus_1 = 15 → 16 bits below
	w.F(4, 15)
	// max_frame_width_minus_1, max_frame_height_minus_1 (16 bits each)
	w.F(16, uint32(width-1))
	w.F(16, uint32(height-1))

	// use_128x128_superblock = 0
	w.F(1, 0)
	// enable_filter_intra = 0
	w.F(1, 0)
	// enable_intra_edge_filter = 0
	w.F(1, 0)
	// Not reduced-still path params. In reduced_still mode: the following
	// are inferred and NOT coded:
	//   enable_interintra_compound, enable_masked_compound,
	//   enable_warped_motion, enable_dual_filter, enable_order_hint,
	//   enable_jnt_comp, enable_ref_frame_mvs, seq_force_screen_content_tools,
	//   seq_force_integer_mv, order_hint_bits_minus_1
	// So we skip directly to enable_superres and following flags.

	// enable_superres = 0
	w.F(1, 0)
	// enable_cdef = 0 (disabled for now — keeps the encoder simple)
	w.F(1, 0)
	// enable_restoration = 0
	w.F(1, 0)

	// color_config (seq_profile=0 path, spec §5.5.2):
	// high_bitdepth = 0 (8-bit)
	w.F(1, 0)
	// monochrome = 0
	w.F(1, 0)
	// color_description_present_flag = 0
	w.F(1, 0)
	// color_range = 0 (studio)
	w.F(1, 0)
	// profile=0 infers subsampling_x=1, subsampling_y=1 (not coded)
	// chroma_sample_position = 0 (coded because subsampling is 1/1)
	w.F(2, 0)
	// separate_uv_delta_q = 0
	w.F(1, 0)

	// film_grain_params_present = 0
	w.F(1, 0)

	// Trailing bits (one 1-bit + zero pad to byte boundary).
	w.TrailingBits()

	return append([]byte(nil), w.Bytes()...)
}

// WrapOBU wraps a payload in an OBU header (obu_type + extension +
// optional size field). kind is the OBU_TYPE enum value. For
// sequence header (kind = 1) and frame (kind = 6), the obu_has_size
// bit is always set since AV1 inside ISOBMFF uses sized OBUs.
func WrapOBU(kind uint8, payload []byte) []byte {
	w := bitio.NewWriter()
	// obu_forbidden_bit = 0
	w.F(1, 0)
	// obu_type (4 bits)
	w.F(4, uint32(kind&0xF))
	// obu_extension_flag = 0
	w.F(1, 0)
	// obu_has_size_field = 1
	w.F(1, 1)
	// obu_reserved_1bit = 0
	w.F(1, 0)

	var buf bytes.Buffer
	buf.Write(w.Bytes())
	// OBU size as leb128.
	sizeW := bitio.NewWriter()
	sizeW.Leb128(uint64(len(payload)))
	buf.Write(sizeW.Bytes())
	buf.Write(payload)
	return buf.Bytes()
}
