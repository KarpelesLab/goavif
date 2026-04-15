package isobmff

import "fmt"

// Pitm is the Primary Item Box (§8.11.4). In AVIF it designates the item id
// that is the primary coded image.
type Pitm struct {
	FullBoxHeader
	ItemID uint32
}

// BoxType implements [Box].
func (*Pitm) BoxType() FourCC { return TypePitm }

// MarshalPayload implements [Box].
func (p *Pitm) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, p.FullBoxHeader)
	switch p.Version {
	case 0:
		if p.ItemID > 0xffff {
			return nil, fmt.Errorf("%w: pitm v0 item_id %d > 65535", ErrInvalid, p.ItemID)
		}
		b.writeU16(uint16(p.ItemID))
	case 1:
		b.writeU32(p.ItemID)
	default:
		return nil, fmt.Errorf("%w: pitm version %d", ErrInvalid, p.Version)
	}
	return b.bytes(), nil
}

// ParsePitm decodes a pitm payload.
func ParsePitm(payload []byte) (*Pitm, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	p := &Pitm{FullBoxHeader: fbh}
	switch fbh.Version {
	case 0:
		p.ItemID = uint32(c.readU16())
	case 1:
		p.ItemID = c.readU32()
	default:
		return nil, fmt.Errorf("%w: pitm version %d", ErrInvalid, fbh.Version)
	}
	return p, c.err
}
