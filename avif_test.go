package goavif

import (
	"bytes"
	"errors"
	"image"
	"testing"

	"github.com/KarpelesLab/goavif/isobmff"
)

func TestDecodeConfigRoundtrip(t *testing.T) {
	av1 := bytes.Repeat([]byte{0x00}, 128)
	ct, err := isobmff.BuildStillImage(isobmff.StillImage{
		Width:              1024,
		Height:             768,
		BitDepth:           8,
		ChromaSubsamplingX: 1,
		ChromaSubsamplingY: 1,
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

func TestDecodeReturnsUnsupported(t *testing.T) {
	av1 := []byte{0x00, 0x00, 0x00, 0x00}
	ct, _ := isobmff.BuildStillImage(isobmff.StillImage{
		Width: 16, Height: 16, BitDepth: 8, ChromaSubsamplingX: 1, ChromaSubsamplingY: 1, AV1Bitstream: av1,
	})
	encoded, _ := ct.Encode()
	_, err := Decode(bytes.NewReader(encoded))
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("Decode err = %v; want ErrUnsupported wrap", err)
	}
}

func TestImageRegisterFormat(t *testing.T) {
	av1 := []byte{0x00, 0x00, 0x00, 0x00}
	ct, _ := isobmff.BuildStillImage(isobmff.StillImage{
		Width: 32, Height: 24, BitDepth: 8, ChromaSubsamplingX: 1, ChromaSubsamplingY: 1, AV1Bitstream: av1,
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
