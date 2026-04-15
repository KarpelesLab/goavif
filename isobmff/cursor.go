package isobmff

import (
	"encoding/binary"
	"fmt"
)

// cursor is a tiny read helper over a byte slice. It tracks position and
// returns an error once exhausted. It is used for decoding box payloads with
// variable-width fields.
type cursor struct {
	data []byte
	pos  int
	err  error
}

func newCursor(data []byte) *cursor { return &cursor{data: data} }

func (c *cursor) remaining() int { return len(c.data) - c.pos }
func (c *cursor) eof() bool      { return c.pos >= len(c.data) }

func (c *cursor) need(n int) bool {
	if c.err != nil {
		return false
	}
	if c.pos+n > len(c.data) {
		c.err = fmt.Errorf("%w: need %d bytes, have %d", ErrTruncated, n, c.remaining())
		return false
	}
	return true
}

func (c *cursor) readU8() uint8 {
	if !c.need(1) {
		return 0
	}
	v := c.data[c.pos]
	c.pos++
	return v
}

func (c *cursor) readU16() uint16 {
	if !c.need(2) {
		return 0
	}
	v := binary.BigEndian.Uint16(c.data[c.pos:])
	c.pos += 2
	return v
}

func (c *cursor) readU24() uint32 {
	if !c.need(3) {
		return 0
	}
	v := uint32(c.data[c.pos])<<16 | uint32(c.data[c.pos+1])<<8 | uint32(c.data[c.pos+2])
	c.pos += 3
	return v
}

func (c *cursor) readU32() uint32 {
	if !c.need(4) {
		return 0
	}
	v := binary.BigEndian.Uint32(c.data[c.pos:])
	c.pos += 4
	return v
}

func (c *cursor) readU64() uint64 {
	if !c.need(8) {
		return 0
	}
	v := binary.BigEndian.Uint64(c.data[c.pos:])
	c.pos += 8
	return v
}

// readUN reads an unsigned big-endian integer of n bytes, where n must be
// 0, 1, 2, 4, or 8. A width of 0 yields 0 and consumes nothing.
func (c *cursor) readUN(n int) uint64 {
	switch n {
	case 0:
		return 0
	case 1:
		return uint64(c.readU8())
	case 2:
		return uint64(c.readU16())
	case 4:
		return uint64(c.readU32())
	case 8:
		return c.readU64()
	default:
		c.err = fmt.Errorf("%w: invalid integer width %d", ErrInvalid, n)
		return 0
	}
}

// readBytes returns a slice referencing n bytes of the underlying buffer.
// Callers must copy if they want to retain the slice beyond the cursor's
// lifetime.
func (c *cursor) readBytes(n int) []byte {
	if !c.need(n) {
		return nil
	}
	b := c.data[c.pos : c.pos+n]
	c.pos += n
	return b
}

// readCString reads a NUL-terminated UTF-8 string. The terminator is consumed
// but not included in the returned string.
func (c *cursor) readCString() string {
	if c.err != nil {
		return ""
	}
	for i := c.pos; i < len(c.data); i++ {
		if c.data[i] == 0 {
			s := string(c.data[c.pos:i])
			c.pos = i + 1
			return s
		}
	}
	c.err = fmt.Errorf("%w: unterminated C string", ErrTruncated)
	return ""
}

// builder is the write counterpart to cursor: append helpers returning the
// accumulated buffer.
type builder struct {
	buf []byte
}

func newBuilder() *builder { return &builder{} }

func (b *builder) bytes() []byte { return b.buf }

func (b *builder) writeU8(v uint8)   { b.buf = append(b.buf, v) }
func (b *builder) writeU16(v uint16) { b.buf = binary.BigEndian.AppendUint16(b.buf, v) }
func (b *builder) writeU24(v uint32) { b.buf = append(b.buf, byte(v>>16), byte(v>>8), byte(v)) }
func (b *builder) writeU32(v uint32) { b.buf = binary.BigEndian.AppendUint32(b.buf, v) }
func (b *builder) writeU64(v uint64) { b.buf = binary.BigEndian.AppendUint64(b.buf, v) }

func (b *builder) writeUN(v uint64, n int) error {
	switch n {
	case 0:
		if v != 0 {
			return fmt.Errorf("%w: cannot encode %d in 0 bytes", ErrInvalid, v)
		}
	case 1:
		if v > 0xff {
			return fmt.Errorf("%w: value %d exceeds 1 byte", ErrInvalid, v)
		}
		b.writeU8(uint8(v))
	case 2:
		if v > 0xffff {
			return fmt.Errorf("%w: value %d exceeds 2 bytes", ErrInvalid, v)
		}
		b.writeU16(uint16(v))
	case 4:
		if v > 0xffffffff {
			return fmt.Errorf("%w: value %d exceeds 4 bytes", ErrInvalid, v)
		}
		b.writeU32(uint32(v))
	case 8:
		b.writeU64(v)
	default:
		return fmt.Errorf("%w: invalid integer width %d", ErrInvalid, n)
	}
	return nil
}

func (b *builder) writeBytes(p []byte) { b.buf = append(b.buf, p...) }

func (b *builder) writeCString(s string) {
	b.buf = append(b.buf, s...)
	b.buf = append(b.buf, 0)
}
