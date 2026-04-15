package isobmff

import (
	"bytes"
	"testing"
)

func TestBuildStillAndRoundtrip(t *testing.T) {
	av1 := bytes.Repeat([]byte{0xDE, 0xAD, 0xBE, 0xEF}, 100) // 400 bytes of fake AV1 payload
	cfg := []byte{0x0A, 0x0B}                                 // fake sequence header OBU bytes
	ct, err := BuildStillImage(StillImage{
		Width:              320,
		Height:             240,
		BitDepth:           8,
		ChromaSubsamplingX: 1,
		ChromaSubsamplingY: 1,
		ConfigOBUs:         cfg,
		AV1Bitstream:       av1,
		NCLX: &Colr{
			Type:                    ColrTypeNCLX,
			ColourPrimaries:         1,
			TransferCharacteristics: 13,
			MatrixCoefficients:      1,
			FullRange:               true,
		},
	})
	if err != nil {
		t.Fatalf("BuildStillImage: %v", err)
	}

	encoded, err := ct.Encode()
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	parsed, err := ParseContainer(encoded)
	if err != nil {
		t.Fatalf("ParseContainer: %v", err)
	}

	if !parsed.Ftyp.HasBrand("avif") {
		t.Errorf("expected avif compat brand, got %+v", parsed.Ftyp)
	}

	primary := parsed.PrimaryItemID()
	if primary != 1 {
		t.Fatalf("primary id = %d, want 1", primary)
	}

	data, err := parsed.ItemData(primary)
	if err != nil {
		t.Fatalf("ItemData: %v", err)
	}
	if !bytes.Equal(data, av1) {
		t.Fatalf("item bytes differ: got %d, want %d (first mismatch at %d)", len(data), len(av1), firstMismatch(data, av1))
	}

	// Verify property associations resolved correctly.
	iprp := parsed.findIprp()
	if iprp == nil {
		t.Fatal("no iprp")
	}
	var sawIspe, sawAv1c, sawPixi, sawColr bool
	for _, prop := range iprp.Ipco.Properties {
		switch p := prop.(type) {
		case *Ispe:
			sawIspe = true
			if p.Width != 320 || p.Height != 240 {
				t.Errorf("ispe = %dx%d, want 320x240", p.Width, p.Height)
			}
		case *Av1C:
			sawAv1c = true
			if !bytes.Equal(p.ConfigOBUs, cfg) {
				t.Errorf("av1C config obus = %x, want %x", p.ConfigOBUs, cfg)
			}
		case *Pixi:
			sawPixi = true
			if len(p.ChannelBits) != 3 {
				t.Errorf("pixi channels = %d, want 3", len(p.ChannelBits))
			}
		case *Colr:
			sawColr = true
		}
	}
	if !sawIspe || !sawAv1c || !sawPixi || !sawColr {
		t.Errorf("missing props: ispe=%v av1C=%v pixi=%v colr=%v", sawIspe, sawAv1c, sawPixi, sawColr)
	}
}

func TestEncodeDeterministic(t *testing.T) {
	av1 := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	cfg := []byte{0xAA, 0xBB}
	s := StillImage{
		Width: 64, Height: 48, BitDepth: 8,
		ChromaSubsamplingX: 1, ChromaSubsamplingY: 1,
		ConfigOBUs: cfg, AV1Bitstream: av1,
	}
	ct1, _ := BuildStillImage(s)
	b1, err := ct1.Encode()
	if err != nil {
		t.Fatalf("Encode 1: %v", err)
	}
	ct2, _ := BuildStillImage(s)
	b2, err := ct2.Encode()
	if err != nil {
		t.Fatalf("Encode 2: %v", err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatalf("Encode is non-deterministic: lengths %d vs %d", len(b1), len(b2))
	}
}

func firstMismatch(a, b []byte) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	if len(a) != len(b) {
		return n
	}
	return -1
}
