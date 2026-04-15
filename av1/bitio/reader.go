package bitio

import (
	"errors"
	"fmt"
)

// ErrEOF is returned when a Reader is asked to consume more bits than are
// available in its backing byte slice.
var ErrEOF = errors.New("bitio: end of stream")

// ErrOverflow is returned when a variable-length field exceeds the maximum
// width supported (32 bits for uvlc, 64 bits for leb128's unsigned result).
var ErrOverflow = errors.New("bitio: value out of range")

// Reader reads bits MSB-first from a byte slice. Readers are not safe for
// concurrent use.
type Reader struct {
	buf []byte
	// bit is the index of the next bit to consume, counted from MSB of buf[0].
	// So bit/8 gives the byte index and 7-(bit%8) gives the bit position
	// within that byte (MSB = 7).
	bit uint64
	// err latches the first read error so chained calls behave.
	err error
}

// NewReader returns a Reader that reads bits from buf starting at buf[0]'s MSB.
func NewReader(buf []byte) *Reader {
	return &Reader{buf: buf}
}

// Err returns the latched error, if any.
func (r *Reader) Err() error { return r.err }

// BitPos returns the number of bits consumed so far.
func (r *Reader) BitPos() uint64 { return r.bit }

// BytePos returns the byte index of the next bit (rounding down).
func (r *Reader) BytePos() uint64 { return r.bit / 8 }

// BitsRemaining reports how many bits are still unconsumed.
func (r *Reader) BitsRemaining() uint64 {
	total := uint64(len(r.buf)) * 8
	if r.bit >= total {
		return 0
	}
	return total - r.bit
}

// ReadBit reads a single bit. It returns 0 and latches ErrEOF on exhaustion.
func (r *Reader) ReadBit() uint32 {
	if r.err != nil {
		return 0
	}
	byteIdx := r.bit >> 3
	if byteIdx >= uint64(len(r.buf)) {
		r.err = ErrEOF
		return 0
	}
	bit := 7 - (r.bit & 7)
	r.bit++
	return uint32((r.buf[byteIdx] >> bit) & 1)
}

// F reads n bits as an unsigned big-endian integer. n must be in [0, 32].
// f(0) returns 0 without consuming any bits.
func (r *Reader) F(n uint) uint32 {
	if n == 0 {
		return 0
	}
	if n > 32 {
		r.err = fmt.Errorf("%w: f(%d) exceeds 32 bits", ErrOverflow, n)
		return 0
	}
	var v uint32
	for i := uint(0); i < n; i++ {
		v = (v << 1) | r.ReadBit()
	}
	return v
}

// F64 is like F but allows up to 64 bits, as needed by leb128-derived fields
// in the sequence header (frame_presentation_delay etc.).
func (r *Reader) F64(n uint) uint64 {
	if n == 0 {
		return 0
	}
	if n > 64 {
		r.err = fmt.Errorf("%w: f(%d) exceeds 64 bits", ErrOverflow, n)
		return 0
	}
	var v uint64
	for i := uint(0); i < n; i++ {
		v = (v << 1) | uint64(r.ReadBit())
	}
	return v
}

// Su reads a signed n-bit integer in sign-magnitude form as defined by the
// spec's su(n) primitive: if the MSB is set, the value is negative with
// magnitude f(n) - 2^n.
func (r *Reader) Su(n uint) int32 {
	if n == 0 {
		return 0
	}
	v := r.F(n)
	signMask := uint32(1) << (n - 1)
	if v&signMask != 0 {
		return int32(v) - int32(1<<n)
	}
	return int32(v)
}

// Uvlc decodes an unsigned variable-length code (spec §4.10.3).
// Leading zero bits give the magnitude width; a '1' bit follows; then the
// data bits. A run of >= 32 leading zeros returns 2^32 - 1 as defined.
func (r *Reader) Uvlc() uint32 {
	leadingZeros := uint(0)
	for {
		if r.err != nil {
			return 0
		}
		if r.ReadBit() == 1 {
			break
		}
		leadingZeros++
		if leadingZeros >= 32 {
			return (1 << 32) - 1
		}
	}
	if leadingZeros == 0 {
		return 0
	}
	value := r.F(leadingZeros)
	// value + (1 << leadingZeros) - 1 computed in 64-bit to avoid overflow.
	v64 := uint64(value) + (uint64(1) << leadingZeros) - 1
	if v64 >= 1<<32 {
		r.err = fmt.Errorf("%w: uvlc value %d exceeds 32 bits", ErrOverflow, v64)
		return 0
	}
	return uint32(v64)
}

// Leb128 decodes up to 8 bytes of unsigned LEB128 (spec §4.10.5). Each byte
// contributes 7 data bits; the high bit signals continuation. Returns the
// decoded value and the number of bytes consumed.
func (r *Reader) Leb128() (uint64, int) {
	var value uint64
	var bytesRead int
	for i := 0; i < 8; i++ {
		b := r.F(8)
		bytesRead++
		value |= uint64(b&0x7f) << (7 * uint(i))
		if b&0x80 == 0 {
			break
		}
		if i == 7 {
			// continuation bit set on the 8th byte is a format error
			r.err = fmt.Errorf("%w: leb128 continuation past 8 bytes", ErrOverflow)
			return 0, bytesRead
		}
	}
	return value, bytesRead
}

// Le reads n little-endian bytes as an unsigned integer. Used for frame
// length fields in large_scale_tile mode.
func (r *Reader) Le(n uint) uint64 {
	if n == 0 {
		return 0
	}
	if n > 8 {
		r.err = fmt.Errorf("%w: le(%d) exceeds 8 bytes", ErrOverflow, n)
		return 0
	}
	var v uint64
	for i := uint(0); i < n; i++ {
		b := uint64(r.F(8))
		v |= b << (8 * i)
	}
	return v
}

// Ns decodes a non-symmetric small-value field of max size n (spec §4.10.6).
// It uses ceil(log2(n)) bits in most cases and falls back to a longer form
// for high-range values to equalize code lengths.
func (r *Reader) Ns(n uint32) uint32 {
	if n <= 1 {
		return 0
	}
	w := uint(32 - leadingZeros32(n))
	m := (uint32(1) << w) - n
	v := r.F(w - 1)
	if v < m {
		return v
	}
	extra := r.ReadBit()
	return (v << 1) - m + extra
}

// ByteAlign consumes 0-7 zero bits to realign the reader to the next byte
// boundary. The consumed bits MUST be zero; non-zero padding is reported as
// an error but the reader still advances so that subsequent parses stay
// synchronized.
func (r *Reader) ByteAlign() {
	for r.bit&7 != 0 {
		if r.ReadBit() != 0 {
			r.err = fmt.Errorf("%w: non-zero byte-align padding", ErrOverflow)
		}
	}
}

// TrailingBits verifies and consumes the AV1 trailing bits pattern: a single
// '1' bit followed by zero bits to the next byte boundary (spec §4.10.4).
// Returns an error if the pattern does not match.
func (r *Reader) TrailingBits() error {
	if r.err != nil {
		return r.err
	}
	if r.ReadBit() != 1 {
		return fmt.Errorf("%w: missing trailing_one_bit", ErrOverflow)
	}
	for r.bit&7 != 0 {
		if r.ReadBit() != 0 {
			return fmt.Errorf("%w: non-zero trailing bit", ErrOverflow)
		}
	}
	return r.err
}

// leadingZeros32 returns the number of leading zero bits in x. For x == 0 it
// returns 32.
func leadingZeros32(x uint32) int {
	if x == 0 {
		return 32
	}
	n := 0
	if x&0xffff0000 == 0 {
		n += 16
		x <<= 16
	}
	if x&0xff000000 == 0 {
		n += 8
		x <<= 8
	}
	if x&0xf0000000 == 0 {
		n += 4
		x <<= 4
	}
	if x&0xc0000000 == 0 {
		n += 2
		x <<= 2
	}
	if x&0x80000000 == 0 {
		n += 1
	}
	return n
}
