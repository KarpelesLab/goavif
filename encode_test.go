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
	for _, sz := range []int{64, 128, 256} {
		t.Run(dimName(sz), func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, sz, sz))
			var buf bytes.Buffer
			if err := Encode(&buf, src, nil); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			img, err := Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("Decode %dx%d: %v", sz, sz, err)
			}
			if img == nil {
				t.Fatal("Decode returned nil image")
			}
			if img.Bounds().Dx() != sz || img.Bounds().Dy() != sz {
				t.Fatalf("decoded size %v, want %dx%d", img.Bounds(), sz, sz)
			}
		})
	}
}

func dimName(n int) string {
	return "x" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
