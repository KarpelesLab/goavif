package bitio

import "testing"

func TestWriterFReaderFRoundTrip(t *testing.T) {
	w := NewWriter()
	w.F(3, 5)   // 101
	w.F(5, 17)  // 10001
	w.F(8, 0xA5)
	w.F(1, 1)
	w.TrailingBits()
	r := NewReader(w.Bytes())
	if got := r.F(3); got != 5 {
		t.Fatalf("F(3) got %d want 5", got)
	}
	if got := r.F(5); got != 17 {
		t.Fatalf("F(5) got %d want 17", got)
	}
	if got := r.F(8); got != 0xA5 {
		t.Fatalf("F(8) got %x want a5", got)
	}
	if got := r.F(1); got != 1 {
		t.Fatalf("F(1) got %d want 1", got)
	}
}

func TestWriterUvlcRoundTrip(t *testing.T) {
	vals := []uint32{0, 1, 2, 3, 4, 100, 65535}
	w := NewWriter()
	for _, v := range vals {
		w.Uvlc(v)
	}
	w.TrailingBits()
	r := NewReader(w.Bytes())
	for i, v := range vals {
		got := r.Uvlc()
		if got != v {
			t.Fatalf("Uvlc[%d] got %d want %d", i, got, v)
		}
	}
}

func TestWriterLeb128RoundTrip(t *testing.T) {
	vals := []uint64{0, 1, 127, 128, 16384, 0xDEADBEEF}
	w := NewWriter()
	for _, v := range vals {
		w.Leb128(v)
	}
	r := NewReader(w.Bytes())
	for i, v := range vals {
		got, _ := r.Leb128()
		if got != v {
			t.Fatalf("Leb128[%d] got %d want %d", i, got, v)
		}
	}
}

func TestWriterSuRoundTrip(t *testing.T) {
	w := NewWriter()
	w.Su(8, -100)
	w.Su(12, 2000)
	w.Su(4, -1)
	w.TrailingBits()
	r := NewReader(w.Bytes())
	if got := r.Su(8); got != -100 {
		t.Fatalf("Su(8) got %d want -100", got)
	}
	if got := r.Su(12); got != 2000 {
		t.Fatalf("Su(12) got %d want 2000", got)
	}
	if got := r.Su(4); got != -1 {
		t.Fatalf("Su(4) got %d want -1", got)
	}
}

func TestWriterNsRoundTrip(t *testing.T) {
	vals := []struct {
		n, v uint32
	}{{4, 0}, {4, 1}, {4, 2}, {4, 3}, {5, 0}, {5, 4}, {7, 6}}
	w := NewWriter()
	for _, e := range vals {
		w.Ns(e.n, e.v)
	}
	w.TrailingBits()
	r := NewReader(w.Bytes())
	for i, e := range vals {
		got := r.Ns(e.n)
		if got != e.v {
			t.Fatalf("Ns[%d] got %d want %d (n=%d)", i, got, e.v, e.n)
		}
	}
}
