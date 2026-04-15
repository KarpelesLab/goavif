package goavif

import (
	"bytes"
	"errors"
	"image"
	"testing"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/isobmff"
)

// bw is a small MSB-first bit writer used to hand-build AV1 OBUs for tests.
type bw struct {
	buf []byte
	bit uint
}

func (w *bw) writeBit(b uint32) {
	if w.bit%8 == 0 {
		w.buf = append(w.buf, 0)
	}
	if b != 0 {
		w.buf[len(w.buf)-1] |= 1 << (7 - (w.bit % 8))
	}
	w.bit++
}

func (w *bw) write(v uint32, n uint) {
	for i := n; i > 0; i-- {
		w.writeBit((v >> (i - 1)) & 1)
	}
}

func (w *bw) trailing() {
	w.writeBit(1)
	for w.bit%8 != 0 {
		w.writeBit(0)
	}
}

// minimalFrameOBU returns an OBU_FRAME containing only a minimal uncompressed
// frame header for a 64x48 KEY_FRAME. The tile group payload is empty — the
// caller is expected to hit ErrPixelDecodeUnimplemented or ErrMalformed.
func minimalFrameOBU() []byte {
	inner := &bw{}
	inner.writeBit(0) // disable_cdf_update
	inner.writeBit(0) // allow_screen_content_tools (seq forces SELECT)
	inner.writeBit(0) // render_and_frame_size_different
	inner.writeBit(1) // uniform_tile_spacing_flag
	inner.write(0, 8) // base_q_index = 0
	inner.writeBit(0) // delta_q_y_dc
	inner.writeBit(0) // delta_q_u_dc
	inner.writeBit(0) // delta_q_u_ac
	inner.writeBit(0) // using_qmatrix
	inner.writeBit(0) // seg_enabled
	inner.writeBit(0) // reduced_tx_set
	inner.trailing()
	// OBU header: type = FRAME (6), has_size = 1
	obuHdr := byte((6 << 3) | (1 << 1))
	return append([]byte{obuHdr, byte(len(inner.buf))}, inner.buf...)
}

// minimalSeqHeaderOBUs returns a valid OBU_SEQUENCE_HEADER (with has_size
// field) wrapping a reduced_still_picture_header for 64x48 8-bit 4:2:0.
// The output is suitable for dropping into av1C.ConfigOBUs.
func minimalSeqHeaderOBUs() []byte {
	inner := &bw{}
	inner.write(0, 3)  // seq_profile
	inner.writeBit(1)  // still_picture
	inner.writeBit(1)  // reduced_still_picture_header
	inner.write(0, 5)  // seq_level_idx[0]
	inner.write(5, 4)  // frame_width_bits_minus_1
	inner.write(5, 4)  // frame_height_bits_minus_1
	inner.write(63, 6) // max_frame_width_minus_1
	inner.write(47, 6) // max_frame_height_minus_1
	inner.write(0, 6)  // use_128 / enable_filter_intra / enable_intra_edge / enable_superres / cdef / restoration
	inner.writeBit(0)  // high_bitdepth
	inner.writeBit(0)  // monochrome
	inner.writeBit(0)  // color_description_present_flag
	inner.writeBit(0)  // color_range
	inner.write(0, 2)  // chroma_sample_position
	inner.writeBit(0)  // separate_uv_deltas_q
	inner.writeBit(0)  // film_grain_params_present
	inner.trailing()

	// OBU header byte: obu_forbidden_bit(0) | type=1 (SEQ_HDR) | ext=0 | has_size=1 | reserved=0
	// Layout: bit7=forbidden, bits6..3=type, bit2=ext, bit1=has_size, bit0=reserved
	obuHdr := byte((1 << 3) | (1 << 1))
	return append([]byte{obuHdr, byte(len(inner.buf))}, inner.buf...)
}

func TestDecodeConfigRoundtrip(t *testing.T) {
	av1 := bytes.Repeat([]byte{0x00}, 128)
	ct, err := isobmff.BuildStillImage(isobmff.StillImage{
		Width:              1024,
		Height:             768,
		BitDepth:           8,
		ChromaSubsamplingX: 1,
		ChromaSubsamplingY: 1,
		ConfigOBUs:         minimalSeqHeaderOBUs(),
		AV1Bitstream:       av1,
	})
	if err != nil {
		t.Fatalf("BuildStillImage: %v", err)
	}
	encoded, err := ct.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	cfg, err := DecodeConfig(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("DecodeConfig: %v", err)
	}
	if cfg.Width != 1024 || cfg.Height != 768 {
		t.Errorf("dims = %dx%d, want 1024x768", cfg.Width, cfg.Height)
	}
}

func TestDecodeReturnsUnsupportedAfterFullHeaderParse(t *testing.T) {
	// Using a well-formed FRAME OBU so that Decode walks container -> seq
	// header -> primary item -> OBU split -> frame header -> unimplemented
	// pixel path and surfaces ErrPixelDecodeUnimplemented (wrapped by
	// ErrUnsupported).
	av1 := minimalFrameOBU()
	ct, _ := isobmff.BuildStillImage(isobmff.StillImage{
		Width: 64, Height: 48, BitDepth: 8, ChromaSubsamplingX: 1, ChromaSubsamplingY: 1,
		ConfigOBUs:   minimalSeqHeaderOBUs(),
		AV1Bitstream: av1,
	})
	encoded, _ := ct.Encode()
	_, err := Decode(bytes.NewReader(encoded))
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Decode err = %v; want ErrUnsupported wrap", err)
	}
	// The underlying cause should be decoder.ErrPixelDecodeUnimplemented,
	// confirming the pipeline traversed OBU + frame header successfully.
	if !errors.Is(err, decoder.ErrPixelDecodeUnimplemented) {
		t.Fatalf("Decode err chain should contain ErrPixelDecodeUnimplemented, got: %v", err)
	}
}

func TestImageRegisterFormat(t *testing.T) {
	av1 := []byte{0x00, 0x00, 0x00, 0x00}
	ct, _ := isobmff.BuildStillImage(isobmff.StillImage{
		Width: 32, Height: 24, BitDepth: 8, ChromaSubsamplingX: 1, ChromaSubsamplingY: 1,
		ConfigOBUs:   minimalSeqHeaderOBUs(),
		AV1Bitstream: av1,
	})
	encoded, _ := ct.Encode()

	// image.DecodeConfig should find and use our registered format.
	cfg, format, err := image.DecodeConfig(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("image.DecodeConfig: %v", err)
	}
	if format != "avif" {
		t.Errorf("format = %q, want avif", format)
	}
	if cfg.Width != 32 || cfg.Height != 24 {
		t.Errorf("dims = %dx%d, want 32x24", cfg.Width, cfg.Height)
	}
}
