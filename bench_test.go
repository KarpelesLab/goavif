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
