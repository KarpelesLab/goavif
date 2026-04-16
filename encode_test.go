package goavif

import (
	"bytes"
	"image"
	"testing"

	"github.com/KarpelesLab/goavif/isobmff"
)

func TestEncodeProducesValidContainer(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	var buf bytes.Buffer
	if err := Encode(&buf, src, nil); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("Encode produced empty output")
	}
	// Parse the container and confirm it has the expected AVIF brand.
	ct, err := isobmff.ParseContainer(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseContainer: %v", err)
	}
	if !ct.Ftyp.HasBrand("avif") {
		t.Fatalf("output lacks 'avif' brand; got %v", ct.Ftyp.CompatibleBrands)
	}
	if ct.PrimaryItemID() == 0 {
		t.Fatal("no primary item id in encoded output")
	}
}

func TestEncodeRejectsTinyImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	if err := Encode(&buf, src, nil); err == nil {
		t.Fatal("expected error for image < 4x4")
	}
}

func TestEncodeNilImage(t *testing.T) {
	var buf bytes.Buffer
	if err := Encode(&buf, nil, nil); err == nil {
		t.Fatal("expected error for nil image")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	var buf bytes.Buffer
	if err := Encode(&buf, src, nil); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	// Try to decode our own output. The current encoder emits an
	// all-skip PARTITION_NONE + DC_PRED frame, so pixel output is a
	// constant mid-grey — the goal is just that the decoder
	// consumes the bitstream without error.
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if img == nil {
		t.Fatal("Decode returned nil image")
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("decoded size %v, want 64x64", img.Bounds())
	}
}
