package isobmff

// Hdlr is the Handler Reference box (§8.4.3). Under a meta box in AVIF, the
// handler type is "pict".
type Hdlr struct {
	FullBoxHeader
	// PreDefined is zero in ISO/IEC 14496-12 but preserved for round-trip.
	PreDefined  uint32
	HandlerType FourCC
	// Name is a UTF-8 NUL-terminated string.
	Name string
}

// BoxType implements [Box].
func (*Hdlr) BoxType() FourCC { return TypeHdlr }

// MarshalPayload implements [Box].
func (h *Hdlr) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, h.FullBoxHeader)
	b.writeU32(h.PreDefined)
	b.writeBytes(h.HandlerType[:])
	// reserved[3]
	b.writeU32(0)
	b.writeU32(0)
	b.writeU32(0)
	b.writeCString(h.Name)
	return b.bytes(), nil
}

// ParseHdlr decodes an hdlr payload.
func ParseHdlr(payload []byte) (*Hdlr, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	h := &Hdlr{FullBoxHeader: fbh}
	h.PreDefined = c.readU32()
	copy(h.HandlerType[:], c.readBytes(4))
	_ = c.readU32() // reserved
	_ = c.readU32()
	_ = c.readU32()
	h.Name = c.readCString()
	return h, c.err
}
