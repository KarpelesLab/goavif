package isobmff

import "fmt"

// Infe is a single Item Info Entry carried inside the Iinf container.
// ISO/IEC 14496-12 §8.11.6. Versions 2 and 3 are used by HEIF/AVIF.
type Infe struct {
	FullBoxHeader
	ItemID          uint32
	ItemProtection  uint16
	ItemType        FourCC
	ItemName        string
	ContentType     string // only when ItemType == "mime"
	ContentEncoding string // only when ItemType == "mime"
	// URI is present when ItemType == "uri ".
	ItemURI string
}

// BoxType implements [Box].
func (*Infe) BoxType() FourCC { return TypeInfe }

// MarshalPayload implements [Box].
func (e *Infe) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, e.FullBoxHeader)
	switch e.Version {
	case 2:
		if e.ItemID > 0xffff {
			return nil, fmt.Errorf("%w: infe v2 item_ID %d > 65535", ErrInvalid, e.ItemID)
		}
		b.writeU16(uint16(e.ItemID))
	case 3:
		b.writeU32(e.ItemID)
	default:
		return nil, fmt.Errorf("%w: infe version %d (need 2 or 3)", ErrInvalid, e.Version)
	}
	b.writeU16(e.ItemProtection)
	b.writeBytes(e.ItemType[:])
	b.writeCString(e.ItemName)
	switch {
	case e.ItemType.Equal("mime"):
		b.writeCString(e.ContentType)
		if e.ContentEncoding != "" {
			b.writeCString(e.ContentEncoding)
		}
	case e.ItemType.Equal("uri "):
		b.writeCString(e.ItemURI)
	}
	return b.bytes(), nil
}

// ParseInfe decodes an infe payload.
func ParseInfe(payload []byte) (*Infe, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	e := &Infe{FullBoxHeader: fbh}
	switch fbh.Version {
	case 2:
		e.ItemID = uint32(c.readU16())
	case 3:
		e.ItemID = c.readU32()
	default:
		return nil, fmt.Errorf("%w: infe version %d (need 2 or 3)", ErrInvalid, fbh.Version)
	}
	e.ItemProtection = c.readU16()
	copy(e.ItemType[:], c.readBytes(4))
	e.ItemName = c.readCString()
	switch {
	case e.ItemType.Equal("mime"):
		e.ContentType = c.readCString()
		if !c.eof() {
			e.ContentEncoding = c.readCString()
		}
	case e.ItemType.Equal("uri "):
		e.ItemURI = c.readCString()
	}
	return e, c.err
}

// Iinf is the Item Info Box (§8.11.6.2), a FullBox container for one or more
// [Infe] entries.
type Iinf struct {
	FullBoxHeader
	Entries []*Infe
}

// BoxType implements [Box].
func (*Iinf) BoxType() FourCC { return TypeIinf }

// MarshalPayload implements [Box].
func (i *Iinf) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, i.FullBoxHeader)
	switch i.Version {
	case 0:
		if len(i.Entries) > 0xffff {
			return nil, fmt.Errorf("%w: iinf v0 entry count %d > 65535", ErrInvalid, len(i.Entries))
		}
		b.writeU16(uint16(len(i.Entries)))
	case 1:
		b.writeU32(uint32(len(i.Entries)))
	default:
		return nil, fmt.Errorf("%w: iinf version %d", ErrInvalid, i.Version)
	}
	for _, e := range i.Entries {
		if err := writeBoxTo(b, e); err != nil {
			return nil, err
		}
	}
	return b.bytes(), nil
}

// ParseIinf decodes an iinf payload.
func ParseIinf(payload []byte) (*Iinf, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	i := &Iinf{FullBoxHeader: fbh}
	var count uint32
	switch fbh.Version {
	case 0:
		count = uint32(c.readU16())
	case 1:
		count = c.readU32()
	default:
		return nil, fmt.Errorf("%w: iinf version %d", ErrInvalid, fbh.Version)
	}
	if c.err != nil {
		return nil, c.err
	}

	children, err := ReadChildren(rest[c.pos:])
	if err != nil {
		return nil, err
	}
	if uint32(len(children)) != count {
		return nil, fmt.Errorf("%w: iinf declared %d entries, got %d", ErrInvalid, count, len(children))
	}
	for _, child := range children {
		if child.BoxType() != TypeInfe {
			return nil, fmt.Errorf("%w: iinf contains %q (want infe)", ErrInvalid, child.BoxType())
		}
		rb := child.(*RawBox)
		e, err := ParseInfe(rb.Payload)
		if err != nil {
			return nil, err
		}
		i.Entries = append(i.Entries, e)
	}
	return i, nil
}

// writeBoxTo writes a box into a builder by serializing its header+payload.
func writeBoxTo(b *builder, box Box) error {
	payload, err := box.MarshalPayload()
	if err != nil {
		return err
	}
	typ := box.BoxType()
	hl := headerLen(uint64(len(payload)), typ)
	size := hl + uint64(len(payload))
	if size >= 1<<32 {
		b.writeU32(1)
		b.writeBytes(typ[:])
		b.writeU64(size)
	} else {
		b.writeU32(uint32(size))
		b.writeBytes(typ[:])
	}
	b.writeBytes(payload)
	return nil
}
