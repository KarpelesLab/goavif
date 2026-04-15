package isobmff

import "fmt"

// IrefEntry is one typed reference: a source item and one or more target item
// IDs. The reference kind is given by the Type four-char code (e.g. "auxl"
// for alpha aux, "dimg" for derived images, "thmb" for thumbnails).
type IrefEntry struct {
	Type     FourCC
	FromID   uint32
	ToIDs    []uint32
}

// Iref is the Item Reference Box (ISO/IEC 14496-12 §8.11.12 as referenced by
// HEIF §6.5). The Version field on the outer FullBox controls the integer
// width of item IDs: version 0 uses 16-bit IDs, version 1 uses 32-bit.
type Iref struct {
	FullBoxHeader
	Entries []IrefEntry
}

// BoxType implements [Box].
func (*Iref) BoxType() FourCC { return TypeIref }

// MarshalPayload implements [Box].
func (r *Iref) MarshalPayload() ([]byte, error) {
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, r.FullBoxHeader)
	for _, e := range r.Entries {
		payload := newBuilder()
		switch r.Version {
		case 0:
			if e.FromID > 0xffff {
				return nil, fmt.Errorf("%w: iref v0 from_item_ID %d > 65535", ErrInvalid, e.FromID)
			}
			payload.writeU16(uint16(e.FromID))
		case 1:
			payload.writeU32(e.FromID)
		default:
			return nil, fmt.Errorf("%w: iref version %d", ErrInvalid, r.Version)
		}
		if len(e.ToIDs) > 0xffff {
			return nil, fmt.Errorf("%w: iref reference_count %d > 65535", ErrInvalid, len(e.ToIDs))
		}
		payload.writeU16(uint16(len(e.ToIDs)))
		for _, to := range e.ToIDs {
			switch r.Version {
			case 0:
				if to > 0xffff {
					return nil, fmt.Errorf("%w: iref v0 to_item_ID %d > 65535", ErrInvalid, to)
				}
				payload.writeU16(uint16(to))
			case 1:
				payload.writeU32(to)
			}
		}
		hl := headerLen(uint64(len(payload.bytes())), e.Type)
		size := hl + uint64(len(payload.bytes()))
		if size >= 1<<32 {
			b.writeU32(1)
			b.writeBytes(e.Type[:])
			b.writeU64(size)
		} else {
			b.writeU32(uint32(size))
			b.writeBytes(e.Type[:])
		}
		b.writeBytes(payload.bytes())
	}
	return b.bytes(), nil
}

// ParseIref decodes an iref payload.
func ParseIref(payload []byte) (*Iref, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	if fbh.Version != 0 && fbh.Version != 1 {
		return nil, fmt.Errorf("%w: iref version %d", ErrInvalid, fbh.Version)
	}
	r := &Iref{FullBoxHeader: fbh}
	children, err := ReadChildren(rest)
	if err != nil {
		return nil, err
	}
	for _, ch := range children {
		rb, ok := ch.(*RawBox)
		if !ok {
			return nil, fmt.Errorf("%w: iref child is typed, want raw", ErrInvalid)
		}
		c := newCursor(rb.Payload)
		var from uint32
		if fbh.Version == 0 {
			from = uint32(c.readU16())
		} else {
			from = c.readU32()
		}
		refCount := c.readU16()
		toIDs := make([]uint32, 0, refCount)
		for i := uint16(0); i < refCount; i++ {
			if fbh.Version == 0 {
				toIDs = append(toIDs, uint32(c.readU16()))
			} else {
				toIDs = append(toIDs, c.readU32())
			}
		}
		if c.err != nil {
			return nil, c.err
		}
		r.Entries = append(r.Entries, IrefEntry{
			Type:   rb.Type,
			FromID: from,
			ToIDs:  toIDs,
		})
	}
	return r, nil
}
