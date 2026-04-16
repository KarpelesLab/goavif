package entropy

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/entropy/cdfs"
)

func TestEncodeBoolDecodeBoolRoundTrip(t *testing.T) {
	var enc Encoder
	enc.Init(false)
	probs := []uint32{1000, 8000, 16384, 24000, 30000}
	bits := []uint32{0, 1, 1, 0, 1, 0, 1, 1, 0, 0, 1}
	for i, b := range bits {
		enc.EncodeBool(b, probs[i%len(probs)])
	}
	buf := enc.Finish()

	var dec Decoder
	if err := dec.Init(buf, len(buf), false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for i, b := range bits {
		got := dec.DecodeBool(probs[i%len(probs)])
		if got != b {
			t.Fatalf("bit %d: got %d want %d", i, got, b)
		}
	}
}

func TestEncodeLiteralDecodeLiteralRoundTrip(t *testing.T) {
	var enc Encoder
	enc.Init(false)
	vals := []uint32{0, 1, 0xA5, 0xFF, 0x1234, 0}
	nbits := []int{1, 1, 8, 8, 16, 4}
	for i, v := range vals {
		enc.EncodeLiteral(v, nbits[i])
	}
	buf := enc.Finish()

	var dec Decoder
	if err := dec.Init(buf, len(buf), false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for i, v := range vals {
		got := dec.ReadLiteral(nbits[i])
		if got != v {
			t.Fatalf("literal %d: got %x want %x", i, got, v)
		}
	}
}

func TestEncodeSymbolDecodeSymbolRoundTrip(t *testing.T) {
	cdfCopy := func(src cdfs.CDF) cdfs.CDF {
		return append(cdfs.CDF(nil), src...)
	}
	var enc Encoder
	enc.Init(false)
	syms := []int{0, 1, 0, 1, 0, 1, 1, 0}
	for _, sym := range syms {
		cdfLocal := cdfCopy(cdfs.DefaultTxbSkipCDF[0][0])
		enc.EncodeSymbol(cdfLocal, sym)
	}
	buf := enc.Finish()

	var dec Decoder
	if err := dec.Init(buf, len(buf), false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for i, sym := range syms {
		decCDF := cdfCopy(cdfs.DefaultTxbSkipCDF[0][0])
		got := dec.DecodeSymbol(decCDF)
		if got != sym {
			t.Fatalf("symbol %d: got %d want %d", i, got, sym)
		}
	}
}

func TestEncodeLongBoolSequence(t *testing.T) {
	// Bursty pattern to exercise many renormalizations.
	var enc Encoder
	enc.Init(false)
	probs := []uint32{100, 20000, 500, 15000, 32000, 200}
	bits := []uint32{}
	for i := 0; i < 200; i++ {
		bits = append(bits, uint32((i*17+3)&1))
	}
	for i, b := range bits {
		enc.EncodeBool(b, probs[i%len(probs)])
	}
	buf := enc.Finish()

	var dec Decoder
	if err := dec.Init(buf, len(buf), false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	for i, b := range bits {
		got := dec.DecodeBool(probs[i%len(probs)])
		if got != b {
			t.Fatalf("bit %d: got %d want %d", i, got, b)
		}
	}
}
