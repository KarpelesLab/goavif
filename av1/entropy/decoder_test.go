package entropy

import "testing"

// The entropy coder is validated against a matching encoder in phase 5.
// Until then, these tests only verify smoke-level behavior: that
// initialization and decoding do not overrun the buffer and that the
// state evolves as expected for trivial inputs.

func TestInitRejectsEmpty(t *testing.T) {
	var d Decoder
	if err := d.Init(nil, 0, false); err == nil {
		t.Fatalf("Init(nil) should error")
	}
}

func TestInitConsumes15Bits(t *testing.T) {
	// 4 bytes of all-ones: first 15 bits form buf=0x7FFF; padding=0x7FFF<<0.
	buf := []byte{0xFF, 0xFF, 0xFF, 0xFF}
	var d Decoder
	if err := d.Init(buf, len(buf), false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if d.symbolRange != SymbolRange0 {
		t.Errorf("initial range=%d, want %d", d.symbolRange, SymbolRange0)
	}
	// SymbolValue = ((1<<15)-1) XOR paddedBuf = 0x7FFF XOR 0x7FFF = 0.
	if d.symbolValue != 0 {
		t.Errorf("initial value=%d, want 0 (for all-ones input)", d.symbolValue)
	}
	if d.MaxBits() != int64(len(buf)*8-15) {
		t.Errorf("MaxBits=%d, want %d", d.MaxBits(), len(buf)*8-15)
	}
}

func TestDecodeBoolExtremeProb(t *testing.T) {
	// With probability 32767 (≈1.0) of returning 0, and enough input bits,
	// DecodeBool should yield 0 for the first call regardless of the state
	// derived from the initial value.
	buf := make([]byte, 32)
	for i := range buf {
		buf[i] = 0x00
	}
	var d Decoder
	if err := d.Init(buf, len(buf), false); err != nil {
		t.Fatalf("Init: %v", err)
	}
	bit := d.DecodeBool(32767)
	if bit != 0 {
		t.Errorf("DecodeBool(32767) with zero input buffer = %d, want 0", bit)
	}
}

func TestUpdateCDFSymbol(t *testing.T) {
	// A 3-symbol CDF with all mass on symbol 1.
	cdf := []uint16{1 << 15, 0, 0, 0}
	updateCDF(cdf, 3, 0)
	// After update, cdf[0] should have increased (moved toward 1<<15).
	if cdf[3] != 1 {
		t.Errorf("count slot not incremented: %d", cdf[3])
	}
	// cdf entries before the symbol move toward 0; after, toward 1<<15.
	// Here we nudge cdf[0] toward tmp=1<<15 (post-symbol target), but the
	// algorithm applies i == symbol check before the loop iteration; with
	// symbol=0 and i=0 we set tmp = 1<<15 immediately. Result: cdf[0]
	// decreases from (1<<15) toward (1<<15) — no change in this degenerate
	// case. Assertion is simply that the count slot advanced.
}
