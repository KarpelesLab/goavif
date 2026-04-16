package decoder

import "testing"

func TestNewFrameStateDefaultsEightBit(t *testing.T) {
	fs := NewFrameState(64, 64, 1, 1, false)
	if fs.BitDepth != 8 {
		t.Fatalf("BitDepth = %d want 8", fs.BitDepth)
	}
	if fs.Y == nil || fs.U == nil || fs.V == nil {
		t.Fatal("uint8 planes missing on 8-bit frame")
	}
	if fs.Y16 != nil || fs.U16 != nil || fs.V16 != nil {
		t.Fatal("uint16 planes should be nil on 8-bit frame")
	}
}

func TestNewFrameStateHBDTenBit(t *testing.T) {
	fs := NewFrameStateHBD(32, 32, 1, 1, false, 10)
	if fs.BitDepth != 10 {
		t.Fatalf("BitDepth = %d want 10", fs.BitDepth)
	}
	if fs.Y != nil || fs.U != nil || fs.V != nil {
		t.Fatal("uint8 planes should be nil on 10-bit frame")
	}
	if len(fs.Y16) != 32*32 {
		t.Fatalf("Y16 length = %d want 1024", len(fs.Y16))
	}
	if len(fs.U16) != 16*16 || len(fs.V16) != 16*16 {
		t.Fatalf("chroma uint16 dims wrong: U16=%d V16=%d", len(fs.U16), len(fs.V16))
	}
}

func TestNewFrameStateHBDTwelveBit(t *testing.T) {
	fs := NewFrameStateHBD(16, 16, 1, 1, false, 12)
	if fs.BitDepth != 12 {
		t.Fatalf("BitDepth = %d want 12", fs.BitDepth)
	}
	if len(fs.Y16) != 256 {
		t.Fatalf("Y16 size wrong: %d", len(fs.Y16))
	}
}

func TestNewFrameStateHBDInvalidFallsBackToEightBit(t *testing.T) {
	fs := NewFrameStateHBD(8, 8, 1, 1, false, 9)
	if fs.BitDepth != 8 {
		t.Fatalf("invalid bit depth should fall back to 8, got %d", fs.BitDepth)
	}
	if fs.Y == nil {
		t.Fatal("8-bit fallback should allocate uint8 Y plane")
	}
}

func TestNewFrameStateHBDMonochromeSkipsChroma(t *testing.T) {
	fs := NewFrameStateHBD(32, 32, 1, 1, true, 10)
	if fs.U16 != nil || fs.V16 != nil {
		t.Fatal("monochrome HBD shouldn't allocate chroma planes")
	}
	if len(fs.Y16) != 1024 {
		t.Fatalf("Y16 size wrong: %d", len(fs.Y16))
	}
}
