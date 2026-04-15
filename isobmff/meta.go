package isobmff

// Meta is the Meta Box (§8.11.1). It is a FullBox that contains a handler
// (hdlr) and a variable set of metadata child boxes.
type Meta struct {
	FullBoxHeader
	Children []Box
}

// BoxType implements [Box].
func (*Meta) BoxType() FourCC { return TypeMeta }

// MarshalPayload implements [Box].
func (m *Meta) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, m.FullBoxHeader)
	inner, err := EncodeChildren(m.Children)
	if err != nil {
		return nil, err
	}
	b.writeBytes(inner)
	return b.bytes(), nil
}

// ParseMeta decodes a meta payload. Child boxes are returned as [RawBox] by
// default; callers lift specific ones into typed boxes as needed.
func ParseMeta(payload []byte) (*Meta, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	children, err := ReadChildren(rest)
	if err != nil {
		return nil, err
	}
	return &Meta{FullBoxHeader: fbh, Children: children}, nil
}
