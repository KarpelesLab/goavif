package bitio

import (
	"errors"
	"testing"
)

func TestF(t *testing.T) {
	// 0b10110100 11100000
	buf := []byte{0xB4, 0xE0}
	r := NewReader(buf)
	if v := r.F(1); v != 1 {
		t.Errorf("F(1)=%d want 1", v)
	}
	if v := r.F(3); v != 0b011 {
		t.Errorf("F(3)=%b want 011", v)
	}
	if v := r.F(4); v != 0b0100 {
		t.Errorf("F(4)=%b want 0100", v)
	}
	if v := r.F(8); v != 0xE0 {
		t.Errorf("F(8)=%x want E0", v)
	}
	if r.Err() != nil {
		t.Errorf("Err=%v", r.Err())
	}
}

func TestF_EOF(t *testing.T) {
	r := NewReader([]byte{0xFF})
	r.F(16)
	if !errors.Is(r.Err(), ErrEOF) {
		t.Errorf("err=%v want ErrEOF", r.Err())
	}
}

func TestSu(t *testing.T) {
	// 8-bit 2's complement values: 0x7F=+127, 0x80=-128, 0xFF=-1
	r := NewReader([]byte{0x7F, 0x80, 0xFF})
	if v := r.Su(8); v != 127 {
		t.Errorf("Su(8)=%d want 127", v)
	}
	if v := r.Su(8); v != -128 {
		t.Errorf("Su(8)=%d want -128", v)
	}
	if v := r.Su(8); v != -1 {
		t.Errorf("Su(8)=%d want -1", v)
	}
}

func TestUvlc(t *testing.T) {
	cases := []struct {
		// bits is the encoded bit string; value is the decoded uvlc.
		bits  string
		value uint32
	}{
		{"1", 0},           // leading_zeros=0
		{"010", 1},         // lz=1, extra=0 -> 0+(1<<1)-1=1
		{"011", 2},         // lz=1, extra=1 -> 1+1=2
		{"00100", 3},       // lz=2, extra=00 -> 0+3=3
		{"00111", 6},       // lz=2, extra=11 -> 3+3=6
		{"000010000", 15},  // lz=4, extra=0000 -> 0+15=15
	}
	for _, c := range cases {
		buf := packBits(c.bits)
		r := NewReader(buf)
		got := r.Uvlc()
		if got != c.value {
			t.Errorf("uvlc(%q) = %d, want %d", c.bits, got, c.value)
		}
	}
}

func TestLeb128(t *testing.T) {
	// 0xE5 0x8E 0x26 -> 624485 (wikipedia example)
	r := NewReader([]byte{0xE5, 0x8E, 0x26})
	v, n := r.Leb128()
	if v != 624485 || n != 3 {
		t.Errorf("Leb128 = (%d, %d), want (624485, 3)", v, n)
	}
	if r.Err() != nil {
		t.Errorf("err=%v", r.Err())
	}
}

func TestLeb128Single(t *testing.T) {
	r := NewReader([]byte{0x00})
	v, n := r.Leb128()
	if v != 0 || n != 1 {
		t.Errorf("Leb128(0x00) = (%d,%d), want (0,1)", v, n)
	}
}

func TestByteAlign(t *testing.T) {
	// After 3 bits of zeros we expect alignment to consume the remaining 5
	// zero bits cleanly.
	r := NewReader([]byte{0x00, 0xAB})
	_ = r.F(3)
	r.ByteAlign()
	if r.BitPos() != 8 {
		t.Errorf("BitPos after align = %d, want 8", r.BitPos())
	}
	if v := r.F(8); v != 0xAB {
		t.Errorf("next byte = %x, want AB", v)
	}
}

func TestTrailingBits(t *testing.T) {
	// Some payload bits then 1-bit end marker, zero-padded to byte boundary.
	// "1011" (4 payload bits) + "1" (trailing one) + "000" (pad) = 0b10111000
	r := NewReader([]byte{0b10111000})
	v := r.F(4)
	if v != 0b1011 {
		t.Errorf("F(4)=%b want 1011", v)
	}
	if err := r.TrailingBits(); err != nil {
		t.Errorf("TrailingBits: %v", err)
	}
}

func TestNs(t *testing.T) {
	// ns(5): w=3, m = 2^3 - 5 = 3. Values 0..2 encode in 2 bits directly;
	// values 3,4 use 3 bits.
	// ns(5,0) -> 00 -> 0
	// ns(5,2) -> 10 -> 2
	// ns(5,3) -> 110 -> 3
	// ns(5,4) -> 111 -> 4
	cases := []struct {
		bits string
		v    uint32
	}{
		{"00", 0},
		{"01", 1},
		{"10", 2},
		{"110", 3},
		{"111", 4},
	}
	for _, c := range cases {
		buf := packBits(c.bits)
		r := NewReader(buf)
		g := r.Ns(5)
		if g != c.v {
			t.Errorf("ns(5) from %q = %d, want %d", c.bits, g, c.v)
		}
	}
}

// packBits packs a bit string ("01011...") into a zero-padded byte slice.
func packBits(bits string) []byte {
	need := (len(bits) + 7) / 8
	out := make([]byte, need)
	for i, c := range bits {
		if c == '1' {
			out[i/8] |= 1 << (7 - (i % 8))
		}
	}
	return out
}
