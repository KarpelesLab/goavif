package goavif

import (
	"bytes"
	"image"
	"testing"

	"github.com/KarpelesLab/goavif/av1/obu"
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

// TestEncodeQualityAffectsBaseQIndex verifies that the Quality option
// changes the encoded frame's base_q_index. High quality gives a low
// base_q; low quality gives a high base_q.
func TestEncodeQualityAffectsBaseQIndex(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))

	extractBaseQ := func(opts *Options) uint8 {
		var buf bytes.Buffer
		if err := Encode(&buf, src, opts); err != nil {
			t.Fatalf("Encode: %v", err)
		}
		ct, err := isobmff.ParseContainer(buf.Bytes())
		if err != nil {
			t.Fatalf("ParseContainer: %v", err)
		}
		itemData, err := ct.ItemData(ct.PrimaryItemID())
		if err != nil {
			t.Fatalf("ItemData: %v", err)
		}
		obus, err := obu.Split(itemData)
		if err != nil {
			t.Fatalf("OBU.Split: %v", err)
		}
		for _, u := range obus {
			if u.Header.Type != obu.TypeFrame {
				continue
			}
			// Extract the sequence header from the container's av1C.
			var seqBytes []byte
			for _, c := range ct.Meta.Children {
				iprp, ok := c.(*isobmff.Iprp)
				if !ok {
					continue
				}
				for _, p := range iprp.Ipco.Properties {
					if a, ok := p.(*isobmff.Av1C); ok {
						seqBytes = a.ConfigOBUs
					}
				}
			}
			seqOBUs, err := obu.Split(seqBytes)
			if err != nil {
				t.Fatalf("seq split: %v", err)
			}
			sh, err := obu.ParseSequenceHeader(seqOBUs[0].Payload)
			if err != nil {
				t.Fatalf("ParseSequenceHeader: %v", err)
			}
			fh, _, err := obu.ParseFrameHeaderBytes(u.Payload, sh, nil)
			if err != nil {
				t.Fatalf("ParseFrameHeaderBytes: %v", err)
			}
			return fh.Quant.BaseQIndex
		}
		t.Fatal("no FRAME OBU found")
		return 0
	}

	hiQ := extractBaseQ(&Options{Quality: 90})
	loQ := extractBaseQ(&Options{Quality: 10})
	if hiQ >= loQ {
		t.Fatalf("high-quality base_q (%d) should be lower than low-quality (%d)", hiQ, loQ)
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
