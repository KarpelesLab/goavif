package bitio

import "fmt"

// Writer accumulates bits MSB-first into a byte slice. The inverse
// of [Reader]: F() writes n bits, Su() writes an n-bit signed
// unsigned-reciprocal, Uvlc / Leb128 write the variable-length
// forms used by AV1's uncompressed headers.
//
// Writers are not safe for concurrent use.
type Writer struct {
	buf []byte
	// bit is the index of the next bit to write, counted from MSB of
	// buf[0]. Mirrors Reader.bit for round-trip symmetry.
	bit uint64
}

// NewWriter returns a fresh empty writer.
func NewWriter() *Writer {
	return &Writer{}
}

// Bytes returns the accumulated byte slice. Trailing bits in the
// current partial byte are padded with zeros.
func (w *Writer) Bytes() []byte {
	return w.buf
}

// BitPos returns the total number of bits written so far.
func (w *Writer) BitPos() uint64 {
	return w.bit
}

// F writes n bits of v, MSB first. Mirrors [Reader.F].
func (w *Writer) F(n uint, v uint32) {
	if n == 0 {
		return
	}
	for i := int(n) - 1; i >= 0; i-- {
		w.putBit((v >> uint(i)) & 1)
	}
}

// putBit appends a single bit to the output stream.
func (w *Writer) putBit(b uint32) {
	byteIdx := w.bit / 8
	bitPos := 7 - (w.bit % 8)
	for uint64(len(w.buf)) <= byteIdx {
		w.buf = append(w.buf, 0)
	}
	w.buf[byteIdx] |= byte((b & 1) << bitPos)
	w.bit++
}

// Su writes an n-bit two's-complement signed integer. Mirrors
// [Reader.Su]: the MSB is the sign bit.
func (w *Writer) Su(n uint, v int32) {
	if n == 0 {
		return
	}
	u := uint32(v) & ((uint32(1) << n) - 1)
	w.F(n, u)
}

// Uvlc writes an unsigned variable-length code (spec §4.10.3). v is
// stored as leadingZeros ones + one zero + leadingZeros binary bits
// of (v+1). Caller is responsible for staying under 2^32-1.
func (w *Writer) Uvlc(v uint32) {
	if v == 0xFFFFFFFF {
		panic("bitio: Uvlc: 2^32-1 not representable")
	}
	n := v + 1
	// Count bits of n (bit-length).
	bits := 0
	for tmp := n; tmp > 0; tmp >>= 1 {
		bits++
	}
	lz := bits - 1 // leadingZeros
	// Write lz zeros + one 1 bit + lz remaining bits of n (after the leading 1).
	for i := 0; i < lz; i++ {
		w.putBit(0)
	}
	w.putBit(1)
	if lz > 0 {
		w.F(uint(lz), n&((1<<uint(lz))-1))
	}
}

// Leb128 writes an unsigned value in LEB128 form (spec §4.10.5).
// Each byte contributes 7 bits of value plus a continuation bit.
func (w *Writer) Leb128(v uint64) {
	// Align to byte boundary so the output is byte-aligned LEB128.
	for w.bit%8 != 0 {
		w.putBit(0)
	}
	for {
		b := byte(v & 0x7F)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		w.buf = append(w.buf, b)
		w.bit += 8
		if v == 0 {
			return
		}
	}
}

// Ns writes a non-symmetric n-value code per spec §4.10.7: the value
// v is packed into ceil(log2(n)) or ceil(log2(n))-1 bits depending
// on whether v falls in the "short" or "long" range.
func (w *Writer) Ns(n uint32, v uint32) {
	if n == 0 {
		return
	}
	if v >= n {
		panic(fmt.Sprintf("bitio: Ns: v=%d >= n=%d", v, n))
	}
	w2 := 0
	for (1 << uint(w2)) < n {
		w2++
	}
	if w2 == 0 {
		return
	}
	k := uint32(1<<uint(w2)) - n
	if v < k {
		w.F(uint(w2-1), v)
	} else {
		val := v + k
		w.F(uint(w2-1), val>>1)
		w.putBit(val & 1)
	}
}

// TrailingBits emits a 1 bit followed by zeros until the next byte
// boundary (spec §5.3.4).
func (w *Writer) TrailingBits() {
	w.putBit(1)
	for w.bit%8 != 0 {
		w.putBit(0)
	}
}

// Reset clears the writer so it can be reused.
func (w *Writer) Reset() {
	w.buf = w.buf[:0]
	w.bit = 0
}

// ByteAlign pads with zeros to the next byte boundary.
func (w *Writer) ByteAlign() {
	for w.bit%8 != 0 {
		w.putBit(0)
	}
}
