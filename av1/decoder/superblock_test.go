package decoder

import (
	"errors"
	"testing"
)

func TestDecodeSuperblockSmoke(t *testing.T) {
	// Synthetic tile data — the decoded modes will be whatever the entropy
	// decoder selects from the random-ish bytes, but the partition + mode
	// decode should not crash.
	tileData := make([]byte, 256)
	for i := range tileData {
		tileData[i] = byte(i * 53)
	}
	td, err := NewTileDecoder(tileData, minimalFrameHeader(), minimalSeqHeader())
	if err != nil {
		t.Fatalf("NewTileDecoder: %v", err)
	}
	fs := NewFrameState(64, 48, 1, 1, false)

	err = td.DecodeSuperblock(fs, 0, 0)
	// The most likely outcome is ErrCoeffDecodeUnimplemented — the
	// synthetic bitstream will almost certainly produce at least one
	// non-skip block. But it must not panic.
	if err != nil && !errors.Is(err, ErrCoeffDecodeUnimplemented) {
		t.Fatalf("DecodeSuperblock: %v", err)
	}
}

func TestNewFrameState(t *testing.T) {
	fs := NewFrameState(64, 48, 1, 1, false)
	if fs.MICols != 16 || fs.MIRows != 12 {
		t.Errorf("MI dims: %dx%d want 16x12", fs.MICols, fs.MIRows)
	}
	if len(fs.Y) != 64*48 {
		t.Errorf("Y plane length: %d want %d", len(fs.Y), 64*48)
	}
}
