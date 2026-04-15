package isobmff

import "fmt"

// Ipco is the ItemPropertyContainerBox (ISO/IEC 23008-12 §9.3.1). It holds
// ordered item property boxes. It is *not* a FullBox.
type Ipco struct {
	Properties []Box
}

// BoxType implements [Box].
func (*Ipco) BoxType() FourCC { return TypeIpco }

// MarshalPayload implements [Box].
func (c *Ipco) MarshalPayload() ([]byte, error) { return EncodeChildren(c.Properties) }

// ParseIpco decodes an ipco payload into raw child boxes. The caller is
// expected to lift each child into its concrete typed box via package helpers.
func ParseIpco(payload []byte) (*Ipco, error) {
	children, err := ReadChildren(payload)
	if err != nil {
		return nil, err
	}
	return &Ipco{Properties: children}, nil
}

// IpmaAssoc is a single property association: a property index (1-based into
// the enclosing ipco) plus an essential flag.
type IpmaAssoc struct {
	PropertyIndex uint16 // 1-based; 0 means "no property"
	Essential     bool
}

// IpmaEntry associates a list of properties with one item.
type IpmaEntry struct {
	ItemID       uint32
	Associations []IpmaAssoc
}

// Ipma is the ItemPropertyAssociation box (§9.3.2). A given meta may contain
// multiple ipma boxes; typically AVIF uses a single one.
type Ipma struct {
	FullBoxHeader
	Entries []IpmaEntry
}

// BoxType implements [Box].
func (*Ipma) BoxType() FourCC { return TypeIpma }

// MarshalPayload implements [Box].
func (m *Ipma) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, m.FullBoxHeader)
	b.writeU32(uint32(len(m.Entries)))
	wideIndex := (m.Flags & 0x01) != 0
	for _, e := range m.Entries {
		if m.Version < 1 {
			if e.ItemID > 0xffff {
				return nil, fmt.Errorf("%w: ipma v0 item_ID %d > 65535", ErrInvalid, e.ItemID)
			}
			b.writeU16(uint16(e.ItemID))
		} else {
			b.writeU32(e.ItemID)
		}
		if len(e.Associations) > 0xff {
			return nil, fmt.Errorf("%w: ipma association count %d > 255", ErrInvalid, len(e.Associations))
		}
		b.writeU8(uint8(len(e.Associations)))
		for _, a := range e.Associations {
			if wideIndex {
				v := a.PropertyIndex & 0x7fff
				if a.Essential {
					v |= 0x8000
				}
				b.writeU16(v)
			} else {
				if a.PropertyIndex > 0x7f {
					return nil, fmt.Errorf("%w: ipma narrow property_index %d > 127", ErrInvalid, a.PropertyIndex)
				}
				v := uint8(a.PropertyIndex) & 0x7f
				if a.Essential {
					v |= 0x80
				}
				b.writeU8(v)
			}
		}
	}
	return b.bytes(), nil
}

// ParseIpma decodes an ipma payload.
func ParseIpma(payload []byte) (*Ipma, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	m := &Ipma{FullBoxHeader: fbh}
	count := c.readU32()
	wideIndex := (fbh.Flags & 0x01) != 0
	m.Entries = make([]IpmaEntry, 0, count)
	for n := uint32(0); n < count; n++ {
		var e IpmaEntry
		if fbh.Version < 1 {
			e.ItemID = uint32(c.readU16())
		} else {
			e.ItemID = c.readU32()
		}
		ac := c.readU8()
		e.Associations = make([]IpmaAssoc, 0, ac)
		for i := uint8(0); i < ac; i++ {
			if wideIndex {
				v := c.readU16()
				e.Associations = append(e.Associations, IpmaAssoc{
					PropertyIndex: v & 0x7fff,
					Essential:     v&0x8000 != 0,
				})
			} else {
				v := c.readU8()
				e.Associations = append(e.Associations, IpmaAssoc{
					PropertyIndex: uint16(v & 0x7f),
					Essential:     v&0x80 != 0,
				})
			}
		}
		if c.err != nil {
			return nil, c.err
		}
		m.Entries = append(m.Entries, e)
	}
	return m, c.err
}

// Iprp is the Item Properties Box (§9.3). It contains exactly one [Ipco] and
// one or more [Ipma] boxes.
type Iprp struct {
	Ipco       *Ipco
	Ipma       []*Ipma
	Extensions []Box // unknown siblings, preserved for round-trip
}

// BoxType implements [Box].
func (*Iprp) BoxType() FourCC { return TypeIprp }

// MarshalPayload implements [Box].
func (p *Iprp) MarshalPayload() ([]byte, error) {
	if p.Ipco == nil {
		return nil, fmt.Errorf("%w: iprp missing ipco", ErrInvalid)
	}
	var children []Box
	children = append(children, p.Ipco)
	for _, m := range p.Ipma {
		children = append(children, m)
	}
	children = append(children, p.Extensions...)
	return EncodeChildren(children)
}

// ParseIprp decodes an iprp payload.
func ParseIprp(payload []byte) (*Iprp, error) {
	children, err := ReadChildren(payload)
	if err != nil {
		return nil, err
	}
	p := &Iprp{}
	for _, ch := range children {
		rb, ok := ch.(*RawBox)
		if !ok {
			p.Extensions = append(p.Extensions, ch)
			continue
		}
		switch rb.Type {
		case TypeIpco:
			co, err := ParseIpco(rb.Payload)
			if err != nil {
				return nil, err
			}
			if p.Ipco != nil {
				return nil, fmt.Errorf("%w: iprp has multiple ipco", ErrInvalid)
			}
			p.Ipco = co
		case TypeIpma:
			ma, err := ParseIpma(rb.Payload)
			if err != nil {
				return nil, err
			}
			p.Ipma = append(p.Ipma, ma)
		default:
			p.Extensions = append(p.Extensions, ch)
		}
	}
	if p.Ipco == nil {
		return nil, fmt.Errorf("%w: iprp missing ipco", ErrInvalid)
	}
	return p, nil
}
