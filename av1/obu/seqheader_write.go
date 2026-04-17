package obu

import (
	"bytes"

	"github.com/KarpelesLab/goavif/av1/bitio"
)

// SeqWriteOpts configures the sequence header writer. Zero values
// produce the default AVIF 8-bit 4:2:0 profile-0 still-image header
// matching [WriteSequenceHeader].
type SeqWriteOpts struct {
	BitDepth     int  // 8, 10, 12 (defaults to 8)
	Monochrome   bool // single-plane (alpha aux or grayscale)
	SubsamplingX int  // 0 = no subsampling, 1 = half-width. Ignored when Monochrome.
	SubsamplingY int  // 0 = no subsampling, 1 = half-height. Ignored when Monochrome.
	// FilmGrainPresent sets film_grain_params_present=1 in the seq
	// header. Each frame header must then carry a film_grain_params
	// block (see WriteFilmGrainParams).
	FilmGrainPresent bool
}

// WriteSequenceHeader serializes a minimal AVIF-oriented sequence
// header to a byte stream. The output uses reduced_still_picture_header
// mode (§5.5.1) which is the AVIF preferred form: one operating
// point at level 0, no decoder model, no timing info.
//
// Produces an 8-bit 4:2:0 profile-0 header. See [WriteSequenceHeaderHBD]
// for 10/12-bit and [WriteSequenceHeaderFull] for non-4:2:0 chroma.
func WriteSequenceHeader(width, height int) []byte {
	return WriteSequenceHeaderFull(width, height, SeqWriteOpts{
		BitDepth: 8, SubsamplingX: 1, SubsamplingY: 1,
	})
}

// WriteSequenceHeaderHBD serializes a sequence header for HBD still-
// image content at 4:2:0. 10-bit uses profile 0 (high_bitdepth=1);
// 12-bit uses profile 2 (high_bitdepth=1, twelve_bit=1). Any other
// bit depth is clamped to 10.
func WriteSequenceHeaderHBD(width, height, bitDepth int) []byte {
	if bitDepth != 10 && bitDepth != 12 {
		bitDepth = 10
	}
	return WriteSequenceHeaderFull(width, height, SeqWriteOpts{
		BitDepth: bitDepth, SubsamplingX: 1, SubsamplingY: 1,
	})
}

// WriteMonoSequenceHeader serializes a monochrome (single-plane)
// variant of the AVIF sequence header. Used by the alpha encoder path
// to build an auxiliary AV1 item carrying just the alpha plane.
func WriteMonoSequenceHeader(width, height int) []byte {
	return WriteSequenceHeaderFull(width, height, SeqWriteOpts{
		BitDepth: 8, Monochrome: true,
	})
}

// WriteMonoSequenceHeaderHBD serializes a monochrome HBD sequence
// header. Used when the alpha plane matches a 10/12-bit primary.
func WriteMonoSequenceHeaderHBD(width, height, bitDepth int) []byte {
	if bitDepth != 10 && bitDepth != 12 {
		bitDepth = 10
	}
	return WriteSequenceHeaderFull(width, height, SeqWriteOpts{
		BitDepth: bitDepth, Monochrome: true,
	})
}

// WriteSequenceHeaderAVIS emits a sequence header suited to an AVIS
// image sequence that may contain inter frames. Unlike
// [WriteSequenceHeader], this path turns off still_picture /
// reduced_still_picture_header so frame headers can carry
// FrameType=INTER. Chroma / bit-depth / monochrome are passed the
// same way as the still-image form.
func WriteSequenceHeaderAVIS(width, height int, opts SeqWriteOpts) []byte {
	return writeSequenceHeaderAVIS(width, height, opts)
}

// WriteSequenceHeaderFull emits a still-image sequence header with
// the given opts. Picks the correct seq_profile automatically:
//
//   - Profile 0: 4:2:0 at 8/10-bit (or monochrome).
//   - Profile 1: 4:4:4 at 8/10-bit (monochrome=false).
//   - Profile 2: 12-bit (any chroma) or 4:2:2 at any bit depth.
func WriteSequenceHeaderFull(width, height int, opts SeqWriteOpts) []byte {
	if opts.BitDepth != 8 && opts.BitDepth != 10 && opts.BitDepth != 12 {
		opts.BitDepth = 8
	}
	// Normalize subsampling for monochrome (inferred as 1/1).
	subX, subY := opts.SubsamplingX, opts.SubsamplingY
	if opts.Monochrome {
		subX, subY = 1, 1
	} else {
		if subX != 0 && subX != 1 {
			subX = 1
		}
		if subY != 0 && subY != 1 {
			subY = 1
		}
	}

	// Profile selection per spec §6.4.1.
	seqProfile := uint32(0)
	switch {
	case opts.Monochrome:
		// Monochrome at any bit depth: profile 0 for 8/10, profile 2 for 12.
		if opts.BitDepth == 12 {
			seqProfile = 2
		}
	case opts.BitDepth == 12:
		seqProfile = 2
	case subX == 1 && subY == 0:
		// 4:2:2 is only valid under profile 2.
		seqProfile = 2
	case subX == 0 && subY == 0:
		// 4:4:4 at 8/10 → profile 1; at 12 → profile 2 (already handled).
		seqProfile = 1
	}

	w := bitio.NewWriter()

	w.F(3, seqProfile)
	// still_picture = 1
	w.F(1, 1)
	// reduced_still_picture_header = 1
	w.F(1, 1)
	// seq_level_idx[0] = 0 (5 bits)
	w.F(5, 0)

	// frame_width_bits_minus_1, frame_height_bits_minus_1 (4 bits each).
	w.F(4, 15)
	w.F(4, 15)
	w.F(16, uint32(width-1))
	w.F(16, uint32(height-1))

	// use_128x128_superblock = 0
	w.F(1, 0)
	// enable_filter_intra = 0
	w.F(1, 0)
	// enable_intra_edge_filter = 0
	w.F(1, 0)

	// enable_superres = 0
	w.F(1, 0)
	// enable_cdef = 0
	w.F(1, 0)
	// enable_restoration = 0
	w.F(1, 0)

	// color_config (spec §5.5.2):
	// high_bitdepth: 1 for 10/12-bit, 0 for 8-bit.
	if opts.BitDepth >= 10 {
		w.F(1, 1)
	} else {
		w.F(1, 0)
	}
	// twelve_bit: only emitted when seq_profile==2 && high_bitdepth.
	if seqProfile == 2 && opts.BitDepth >= 10 {
		if opts.BitDepth == 12 {
			w.F(1, 1)
		} else {
			w.F(1, 0)
		}
	}
	// monochrome (not coded when profile==1).
	if seqProfile != 1 {
		if opts.Monochrome {
			w.F(1, 1)
		} else {
			w.F(1, 0)
		}
	}
	// color_description_present_flag = 0
	w.F(1, 0)
	// color_range. Studio for color, full for monochrome.
	if opts.Monochrome {
		w.F(1, 1)
	} else {
		w.F(1, 0)
	}
	if !opts.Monochrome {
		// Per spec, subsampling bits are only coded for profile 2
		// (and with extra conditioning for 12-bit).
		switch seqProfile {
		case 0:
			// Inferred 1/1 — no bits coded.
		case 1:
			// Inferred 0/0 — no bits coded.
		case 2:
			if opts.BitDepth == 12 {
				w.F(1, uint32(subX))
				if subX == 1 {
					w.F(1, uint32(subY))
				}
			}
			// Profile 2 at 10-bit always means 4:2:2 (subX=1, subY=0) —
			// no bits coded, inferred.
		}
		// chroma_sample_position (2 bits) when subsampling is 1/1.
		if subX == 1 && subY == 1 {
			w.F(2, 0)
		}
	}
	// separate_uv_delta_q (1 bit) when !monochrome.
	if !opts.Monochrome {
		w.F(1, 0)
	}

	// film_grain_params_present
	if opts.FilmGrainPresent {
		w.F(1, 1)
	} else {
		w.F(1, 0)
	}

	w.TrailingBits()
	return append([]byte(nil), w.Bytes()...)
}

// writeSequenceHeaderAVIS emits the non-reduced-still-picture-header
// form for AVIS sequences. The extra fields (operating points,
// timing info) are set to their minimal defaults so frame_type can
// be KEY or INTER per-frame.
func writeSequenceHeaderAVIS(width, height int, opts SeqWriteOpts) []byte {
	if opts.BitDepth != 8 && opts.BitDepth != 10 && opts.BitDepth != 12 {
		opts.BitDepth = 8
	}
	subX, subY := opts.SubsamplingX, opts.SubsamplingY
	if opts.Monochrome {
		subX, subY = 1, 1
	}

	seqProfile := uint32(0)
	switch {
	case opts.Monochrome && opts.BitDepth == 12:
		seqProfile = 2
	case opts.BitDepth == 12:
		seqProfile = 2
	case !opts.Monochrome && subX == 1 && subY == 0:
		seqProfile = 2
	case !opts.Monochrome && subX == 0 && subY == 0:
		seqProfile = 1
	}

	w := bitio.NewWriter()
	w.F(3, seqProfile)
	// still_picture = 0
	w.F(1, 0)
	// reduced_still_picture_header = 0
	w.F(1, 0)

	// timing_info_present_flag = 0
	w.F(1, 0)
	// initial_display_delay_present_flag = 0
	w.F(1, 0)
	// operating_points_cnt_minus_1 = 0 (5 bits)
	w.F(5, 0)
	// operating_point_idc[0] (12 bits) = 0
	w.F(12, 0)
	// seq_level_idx[0] (5 bits) = 0
	w.F(5, 0)
	// seq_tier[0] not coded (level <= 7)
	// decoder_model_present_for_this_op[0] not coded
	// initial_display_delay_present_for_this_op[0] not coded

	// frame_width_bits_minus_1 / frame_height_bits_minus_1
	w.F(4, 15)
	w.F(4, 15)
	w.F(16, uint32(width-1))
	w.F(16, uint32(height-1))

	// frame_id_numbers_present_flag = 0
	w.F(1, 0)
	// use_128x128_superblock = 0
	w.F(1, 0)
	// enable_filter_intra = 0
	w.F(1, 0)
	// enable_intra_edge_filter = 0
	w.F(1, 0)

	// Non-reduced path: these flags are coded.
	// enable_interintra_compound = 0
	w.F(1, 0)
	// enable_masked_compound = 0
	w.F(1, 0)
	// enable_warped_motion = 0
	w.F(1, 0)
	// enable_dual_filter = 0
	w.F(1, 0)
	// enable_order_hint = 0
	w.F(1, 0)
	// enable_jnt_comp not coded (depends on enable_order_hint)
	// enable_ref_frame_mvs not coded (depends on enable_order_hint)
	// seq_force_screen_content_tools (2 bits) = 0 (NO screen content)
	w.F(1, 1) // seq_choose_screen_content_tools bit → 1 = SELECT
	// screen_content_tools now inferred as SELECT (2)
	// seq_force_integer_mv: only coded if SELECT screen content AND
	// screen content is allowed. With SELECT screen content, emit
	// seq_choose_integer_mv = 1 → inferred as SELECT (2).
	w.F(1, 1)
	// order_hint_bits_minus_1 not coded (order_hint disabled)

	// enable_superres = 0
	w.F(1, 0)
	// enable_cdef = 0
	w.F(1, 0)
	// enable_restoration = 0
	w.F(1, 0)

	// color_config — same logic as the still-image form.
	if opts.BitDepth >= 10 {
		w.F(1, 1)
	} else {
		w.F(1, 0)
	}
	if seqProfile == 2 && opts.BitDepth >= 10 {
		if opts.BitDepth == 12 {
			w.F(1, 1)
		} else {
			w.F(1, 0)
		}
	}
	if seqProfile != 1 {
		if opts.Monochrome {
			w.F(1, 1)
		} else {
			w.F(1, 0)
		}
	}
	w.F(1, 0) // color_description_present_flag
	if opts.Monochrome {
		w.F(1, 1) // color_range (full for mono)
	} else {
		w.F(1, 0) // color_range (studio)
	}
	if !opts.Monochrome {
		if seqProfile == 2 && opts.BitDepth == 12 {
			w.F(1, uint32(subX))
			if subX == 1 {
				w.F(1, uint32(subY))
			}
		}
		if subX == 1 && subY == 1 {
			w.F(2, 0) // chroma_sample_position
		}
	}
	if !opts.Monochrome {
		w.F(1, 0) // separate_uv_delta_q
	}
	// film_grain_params_present
	if opts.FilmGrainPresent {
		w.F(1, 1)
	} else {
		w.F(1, 0)
	}

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
