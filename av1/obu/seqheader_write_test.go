package obu

import "testing"

func TestWriteSequenceHeaderRoundTrip(t *testing.T) {
	payload := WriteSequenceHeader(320, 240)
	sh, err := ParseSequenceHeader(payload)
	if err != nil {
		t.Fatalf("ParseSequenceHeader: %v", err)
	}
	if sh.FrameWidthBitsMinusOne != 15 {
		t.Fatalf("FrameWidthBitsMinusOne = %d, want 15", sh.FrameWidthBitsMinusOne)
	}
	if sh.MaxFrameWidthMinusOne != 319 {
		t.Fatalf("MaxFrameWidthMinusOne = %d, want 319", sh.MaxFrameWidthMinusOne)
	}
	if sh.MaxFrameHeightMinusOne != 239 {
		t.Fatalf("MaxFrameHeightMinusOne = %d, want 239", sh.MaxFrameHeightMinusOne)
	}
	if sh.Color.BitDepth != 8 {
		t.Fatalf("BitDepth = %d, want 8", sh.Color.BitDepth)
	}
	if sh.Color.SubsamplingX != 1 || sh.Color.SubsamplingY != 1 {
		t.Fatalf("subsampling %d/%d want 1/1", sh.Color.SubsamplingX, sh.Color.SubsamplingY)
	}
	if !sh.ReducedStillPictureHeader {
		t.Fatal("ReducedStillPictureHeader should be true")
	}
}

func TestWrapOBUHasSizeField(t *testing.T) {
	payload := []byte{0x01, 0x02, 0x03}
	wrapped := WrapOBU(1 /* OBU_SEQUENCE_HEADER */, payload)
	// wrapped = [header_byte, leb128_size_bytes..., payload...]
	if len(wrapped) < len(payload)+2 {
		t.Fatalf("wrapped too short: %d", len(wrapped))
	}
	// First byte: obu_type in bits 6..3, size_field bit at position 1.
	headerByte := wrapped[0]
	obuType := (headerByte >> 3) & 0xF
	if obuType != 1 {
		t.Fatalf("obu_type = %d, want 1", obuType)
	}
	hasSize := (headerByte >> 1) & 1
	if hasSize != 1 {
		t.Fatalf("obu_has_size_field = %d, want 1", hasSize)
	}
}
