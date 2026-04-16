package decoder

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/obu"
)

func minimalSeqHeader() *obu.SequenceHeader {
	return &obu.SequenceHeader{
		SeqProfile:                0,
		StillPicture:             true,
		ReducedStillPictureHeader: true,
		MaxFrameWidthMinusOne:    63,
		MaxFrameHeightMinusOne:   47,
		Color: obu.ColorConfig{
			BitDepth:     8,
			NumPlanes:    3,
			SubsamplingX: 1,
			SubsamplingY: 1,
		},
		SeqForceScreenContentTools: obu.SelectScreenContentTools,
		SeqForceIntegerMV:          obu.SelectIntegerMV,
	}
}

func minimalFrameHeader() *obu.FrameHeader {
	return &obu.FrameHeader{
		FrameType:            obu.KeyFrame,
		FrameIsIntra:         true,
		ShowFrame:            true,
		ErrorResilientMode:   true,
		FrameWidth:           64,
		FrameHeight:          48,
		UpscaledWidth:        64,
		RenderWidth:          64,
		RenderHeight:         48,
		DisableCDFUpdate:     false,
		RefreshFrameFlags:    0xFF,
		PrimaryRefFrame:      obu.PrimaryRefNone,
		Tile: obu.TileInfo{
			TileCols: 1, TileRows: 1,
		},
	}
}

func TestTileDecoderInit(t *testing.T) {
	// 32 bytes of pseudo-random data as a synthetic tile bitstream.
	tileData := make([]byte, 32)
	for i := range tileData {
		tileData[i] = byte(i * 37)
	}
	td, err := NewTileDecoder(tileData, minimalFrameHeader(), minimalSeqHeader())
	if err != nil {
		t.Fatalf("NewTileDecoder: %v", err)
	}
	// Read one partition symbol — should not panic.
	pt := td.DecodePartition(0, 0)
	if pt < 0 || pt > 3 {
		t.Errorf("partition symbol out of range: %d", pt)
	}
	if td.Err() != nil {
		t.Errorf("err after partition read: %v", td.Err())
	}
}

func TestTileDecoderReadsModeSymbols(t *testing.T) {
	tileData := make([]byte, 64)
	for i := range tileData {
		tileData[i] = byte(i*13 + 7)
	}
	td, err := NewTileDecoder(tileData, minimalFrameHeader(), minimalSeqHeader())
	if err != nil {
		t.Fatalf("NewTileDecoder: %v", err)
	}
	// Read a sequence of symbols — verifies the CDF tables are wired and
	// the entropy decoder doesn't exhaust.
	_ = td.DecodePartition(0, 0) // partition at BLOCK_8x8, ctx 0
	yMode := td.DecodeIntraYMode(0, 0)
	if int(yMode) >= 13 {
		t.Errorf("Y mode out of range: %d", yMode)
	}
	uvMode := td.DecodeUVMode(yMode, true)
	if int(uvMode) >= 14 {
		t.Errorf("UV mode out of range: %d", uvMode)
	}
	skip := td.DecodeSkip(0)
	_ = skip
	if td.Err() != nil {
		t.Errorf("error after reading mode symbols: %v", td.Err())
	}
}
