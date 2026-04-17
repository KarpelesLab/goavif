package goavif

import (
	"bytes"
	"image"
	"testing"
)

// BenchmarkEncode64x64 measures the full encode path for a small
// frame. Dominated by forward transforms, range encoding, OBU
// writing, and container assembly.
func BenchmarkEncode64x64(b *testing.B) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := Encode(&buf, src, nil); err != nil {
			b.Fatalf("Encode: %v", err)
		}
	}
}

// BenchmarkEncode256x256 measures encoding at a more realistic size
// (16 superblocks).
func BenchmarkEncode256x256(b *testing.B) {
	src := image.NewRGBA(image.Rect(0, 0, 256, 256))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := Encode(&buf, src, nil); err != nil {
			b.Fatalf("Encode: %v", err)
		}
	}
}

// BenchmarkDecodeOwnEncodeOutput measures the decode path on our
// encoder's own output. This exercises the full tile decoder,
// including partition walk + symbol decode.
func BenchmarkDecodeOwnEncodeOutput(b *testing.B) {
	src := image.NewRGBA(image.Rect(0, 0, 256, 256))
	var buf bytes.Buffer
	if err := Encode(&buf, src, nil); err != nil {
		b.Fatalf("Encode: %v", err)
	}
	data := buf.Bytes()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Decode(bytes.NewReader(data))
		if err != nil {
			b.Fatalf("Decode: %v", err)
		}
	}
}

// BenchmarkInterEncode256x256 measures inter-frame encoding on a
// 5-frame sequence. Hot path covers ME, MC, and residual coding.
func BenchmarkInterEncode256x256(b *testing.B) {
	const dim = 256
	frames := make([]image.Image, 5)
	for i := range frames {
		m := image.NewRGBA(image.Rect(0, 0, dim, dim))
		// Gradient + per-frame shift for non-trivial motion.
		for y := 0; y < dim; y++ {
			for x := 0; x < dim; x++ {
				idx := (y*dim + x) * 4
				m.Pix[idx+0] = uint8((x + i) & 0xFF)
				m.Pix[idx+1] = uint8(y & 0xFF)
				m.Pix[idx+2] = uint8(x ^ y)
				m.Pix[idx+3] = 255
			}
		}
		frames[i] = m
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := EncodeAll(&buf, frames, nil, &Options{
			Quality:          80,
			InterEnabled:     true,
			KeyFrameInterval: 5,
		})
		if err != nil {
			b.Fatalf("EncodeAll: %v", err)
		}
	}
}
