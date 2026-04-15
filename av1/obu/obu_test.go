package obu

import (
	"errors"
	"testing"
)

// bw is a tiny MSB-first bit-appender used only by tests to hand-build OBUs.
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

func TestParseOneRejectsEmpty(t *testing.T) {
	_, _, err := ParseOne(nil)
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("err=%v want ErrMalformed", err)
	}
}

func TestOBUHeaderForbiddenBit(t *testing.T) {
	// First bit of header must be zero.
	_, _, err := ParseOne([]byte{0x80, 0x01})
	if !errors.Is(err, ErrMalformed) {
		t.Fatalf("err=%v want ErrMalformed", err)
	}
}

func TestReducedStillPictureSequenceHeader(t *testing.T) {
	// Build a minimal reduced_still_picture_header for 64x48, 8-bit 4:2:0,
	// all optional tools disabled.
	w := &bw{}
	w.write(0, 3)        // seq_profile
	w.writeBit(1)        // still_picture
	w.writeBit(1)        // reduced_still_picture_header
	w.write(0, 5)        // seq_level_idx[0]
	w.write(5, 4)        // frame_width_bits_minus_1 (6 bits -> max 63)
	w.write(5, 4)        // frame_height_bits_minus_1
	w.write(63, 6)       // max_frame_width_minus_1 -> width 64
	w.write(47, 6)       // max_frame_height_minus_1 -> height 48
	w.writeBit(0)        // use_128x128_superblock
	w.writeBit(0)        // enable_filter_intra
	w.writeBit(0)        // enable_intra_edge_filter
	w.writeBit(0)        // enable_superres
	w.writeBit(0)        // enable_cdef
	w.writeBit(0)        // enable_restoration
	// color_config:
	w.writeBit(0)        //   high_bitdepth
	w.writeBit(0)        //   monochrome
	w.writeBit(0)        //   color_description_present_flag
	w.writeBit(0)        //   color_range (profile 0 non-sRGB)
	w.write(0, 2)        //   chroma_sample_position (subX=subY=1 for profile 0)
	w.writeBit(0)        //   separate_uv_deltas_q
	w.writeBit(0)        // film_grain_params_present
	w.trailing()

	sh, err := ParseSequenceHeader(w.buf)
	if err != nil {
		t.Fatalf("ParseSequenceHeader: %v", err)
	}
	if !sh.StillPicture || !sh.ReducedStillPictureHeader {
		t.Errorf("still_picture=%v reduced=%v, want true,true", sh.StillPicture, sh.ReducedStillPictureHeader)
	}
	if sh.MaxFrameWidthMinusOne != 63 || sh.MaxFrameHeightMinusOne != 47 {
		t.Errorf("dims = %dx%d; want 63x47", sh.MaxFrameWidthMinusOne, sh.MaxFrameHeightMinusOne)
	}
	if sh.Color.BitDepth != 8 || sh.Color.NumPlanes != 3 {
		t.Errorf("BitDepth=%d NumPlanes=%d", sh.Color.BitDepth, sh.Color.NumPlanes)
	}
	if sh.Color.SubsamplingX != 1 || sh.Color.SubsamplingY != 1 {
		t.Errorf("subX=%d subY=%d", sh.Color.SubsamplingX, sh.Color.SubsamplingY)
	}
	if len(sh.OperatingPoints) != 1 {
		t.Errorf("operating points = %d", len(sh.OperatingPoints))
	}
	if sh.EnableCdef || sh.EnableRestoration {
		t.Errorf("cdef/restoration should be off")
	}
}

func TestSplitWithSizeField(t *testing.T) {
	// Construct an OBU with obu_has_size_field set wrapping the seq header.
	inner := &bw{}
	inner.write(0, 3)
	inner.writeBit(1)
	inner.writeBit(1)
	inner.write(0, 5)
	inner.write(5, 4)
	inner.write(5, 4)
	inner.write(63, 6)
	inner.write(47, 6)
	inner.write(0, 3) // use_128 / filter_intra / intra_edge
	inner.write(0, 3) // superres / cdef / restoration
	inner.write(0, 3) // high_bitdepth / monochrome / color_desc
	inner.writeBit(0) // color_range
	inner.write(0, 2) // chroma_sample_position
	inner.writeBit(0) // separate_uv_deltas_q
	inner.writeBit(0) // film_grain_params_present
	inner.trailing()

	// OBU header: obu_forbidden_bit=0, type=SeqHdr(1), ext=0, has_size=1, reserved=0
	obuHdr := byte((0 << 7) | (1 << 3) | (0 << 2) | (1 << 1) | 0)

	data := []byte{obuHdr}
	// size leb128 (single byte if < 128)
	data = append(data, byte(len(inner.buf)))
	data = append(data, inner.buf...)

	obus, err := Split(data)
	if err != nil {
		t.Fatalf("Split: %v", err)
	}
	if len(obus) != 1 {
		t.Fatalf("got %d OBUs", len(obus))
	}
	if obus[0].Header.Type != TypeSequenceHeader {
		t.Errorf("type = %s", obus[0].Header.Type)
	}
	sh, err := ParseSequenceHeader(obus[0].Payload)
	if err != nil {
		t.Fatalf("ParseSequenceHeader: %v", err)
	}
	if sh.MaxFrameWidthMinusOne != 63 {
		t.Errorf("dims: got max_w-1=%d, want 63", sh.MaxFrameWidthMinusOne)
	}
}
