package isobmff

import "fmt"

// ConstructionMethod selects where an iloc extent's bytes are stored.
// Values follow ISO/IEC 14496-12 §8.11.3.
type ConstructionMethod uint8

const (
	// ConstructionFileOffset: extent bytes are at extent_offset inside the
	// enclosing file (mdat). This is the only form AVIF typically uses.
	ConstructionFileOffset ConstructionMethod = 0
	// ConstructionIdat: extent bytes are at extent_offset inside the meta
	// idat box.
	ConstructionIdat ConstructionMethod = 1
	// ConstructionItem: extent bytes are located inside another item.
	ConstructionItem ConstructionMethod = 2
)

// IlocExtent is one chunk of an item's data.
type IlocExtent struct {
	Index  uint64 // only used when Iloc.IndexSize != 0
	Offset uint64
	Length uint64
}

// IlocItem locates one item (typically the coded image or its alpha).
type IlocItem struct {
	ItemID             uint32
	ConstructionMethod ConstructionMethod // version>=1 only
	DataReferenceIndex uint16
	BaseOffset         uint64
	Extents            []IlocExtent
}

// Iloc is the Item Location Box (§8.11.3). The *Size fields are the widths in
// bytes (0, 1, 2, 4 or 8) used when serializing the variable-width members.
// On parse they are taken from the wire format; on marshal the caller may set
// them explicitly or call [Iloc.AutoSize] to pick minimum widths.
type Iloc struct {
	FullBoxHeader
	OffsetSize     uint8
	LengthSize     uint8
	BaseOffsetSize uint8
	IndexSize      uint8 // only used for version 1/2
	Items          []IlocItem
}

// BoxType implements [Box].
func (*Iloc) BoxType() FourCC { return TypeIloc }

// AutoSize picks minimum widths for OffsetSize/LengthSize/BaseOffsetSize/
// IndexSize that can represent every value currently in Items. It chooses
// from {0, 4, 8} for simplicity — widely compatible with real-world parsers.
func (i *Iloc) AutoSize() {
	pick := func(max uint64) uint8 {
		switch {
		case max == 0:
			return 0
		case max <= 0xffffffff:
			return 4
		default:
			return 8
		}
	}
	var maxOffset, maxLength, maxBase, maxIndex uint64
	for _, it := range i.Items {
		if it.BaseOffset > maxBase {
			maxBase = it.BaseOffset
		}
		for _, ex := range it.Extents {
			if ex.Offset > maxOffset {
				maxOffset = ex.Offset
			}
			if ex.Length > maxLength {
				maxLength = ex.Length
			}
			if ex.Index > maxIndex {
				maxIndex = ex.Index
			}
		}
	}
	i.OffsetSize = pick(maxOffset)
	i.LengthSize = pick(maxLength)
	i.BaseOffsetSize = pick(maxBase)
	i.IndexSize = pick(maxIndex)
}

// MarshalPayload implements [Box].
func (i *Iloc) MarshalPayload() ([]byte, error) {
	if err := validIlocWidth(i.OffsetSize, "offset"); err != nil {
		return nil, err
	}
	if err := validIlocWidth(i.LengthSize, "length"); err != nil {
		return nil, err
	}
	if err := validIlocWidth(i.BaseOffsetSize, "base offset"); err != nil {
		return nil, err
	}
	if i.Version == 1 || i.Version == 2 {
		if err := validIlocWidth(i.IndexSize, "index"); err != nil {
			return nil, err
		}
	}
	b := newBuilder()
	b.buf = appendFullBoxHeader(b.buf, i.FullBoxHeader)

	b.writeU8((i.OffsetSize << 4) | (i.LengthSize & 0x0f))
	var second uint8 = i.BaseOffsetSize << 4
	if i.Version == 1 || i.Version == 2 {
		second |= i.IndexSize & 0x0f
	}
	b.writeU8(second)

	switch i.Version {
	case 0, 1:
		if len(i.Items) > 0xffff {
			return nil, fmt.Errorf("%w: iloc v%d item count %d > 65535", ErrInvalid, i.Version, len(i.Items))
		}
		b.writeU16(uint16(len(i.Items)))
	case 2:
		b.writeU32(uint32(len(i.Items)))
	default:
		return nil, fmt.Errorf("%w: iloc version %d", ErrInvalid, i.Version)
	}

	for _, it := range i.Items {
		switch i.Version {
		case 0, 1:
			if it.ItemID > 0xffff {
				return nil, fmt.Errorf("%w: iloc v%d item_ID %d > 65535", ErrInvalid, i.Version, it.ItemID)
			}
			b.writeU16(uint16(it.ItemID))
		case 2:
			b.writeU32(it.ItemID)
		}
		if i.Version == 1 || i.Version == 2 {
			// 12 reserved bits + 4 bits construction_method.
			b.writeU16(uint16(it.ConstructionMethod) & 0x000f)
		}
		b.writeU16(it.DataReferenceIndex)
		if err := b.writeUN(it.BaseOffset, int(i.BaseOffsetSize)); err != nil {
			return nil, err
		}
		if len(it.Extents) > 0xffff {
			return nil, fmt.Errorf("%w: iloc extent count %d > 65535", ErrInvalid, len(it.Extents))
		}
		b.writeU16(uint16(len(it.Extents)))
		for _, ex := range it.Extents {
			if (i.Version == 1 || i.Version == 2) && i.IndexSize > 0 {
				if err := b.writeUN(ex.Index, int(i.IndexSize)); err != nil {
					return nil, err
				}
			}
			if err := b.writeUN(ex.Offset, int(i.OffsetSize)); err != nil {
				return nil, err
			}
			if err := b.writeUN(ex.Length, int(i.LengthSize)); err != nil {
				return nil, err
			}
		}
	}
	return b.bytes(), nil
}

// ParseIloc decodes an iloc payload.
func ParseIloc(payload []byte) (*Iloc, error) {
	fbh, rest, err := readFullBoxHeader(payload)
	if err != nil {
		return nil, err
	}
	c := newCursor(rest)
	i := &Iloc{FullBoxHeader: fbh}

	first := c.readU8()
	second := c.readU8()
	i.OffsetSize = first >> 4
	i.LengthSize = first & 0x0f
	i.BaseOffsetSize = second >> 4
	if fbh.Version == 1 || fbh.Version == 2 {
		i.IndexSize = second & 0x0f
	}
	if err := validIlocWidth(i.OffsetSize, "offset"); err != nil {
		return nil, err
	}
	if err := validIlocWidth(i.LengthSize, "length"); err != nil {
		return nil, err
	}
	if err := validIlocWidth(i.BaseOffsetSize, "base offset"); err != nil {
		return nil, err
	}
	if fbh.Version == 1 || fbh.Version == 2 {
		if err := validIlocWidth(i.IndexSize, "index"); err != nil {
			return nil, err
		}
	}

	var itemCount uint32
	switch fbh.Version {
	case 0, 1:
		itemCount = uint32(c.readU16())
	case 2:
		itemCount = c.readU32()
	default:
		return nil, fmt.Errorf("%w: iloc version %d", ErrInvalid, fbh.Version)
	}
	i.Items = make([]IlocItem, 0, itemCount)

	for n := uint32(0); n < itemCount; n++ {
		var it IlocItem
		switch fbh.Version {
		case 0, 1:
			it.ItemID = uint32(c.readU16())
		case 2:
			it.ItemID = c.readU32()
		}
		if fbh.Version == 1 || fbh.Version == 2 {
			cm := c.readU16()
			it.ConstructionMethod = ConstructionMethod(cm & 0x000f)
		}
		it.DataReferenceIndex = c.readU16()
		it.BaseOffset = c.readUN(int(i.BaseOffsetSize))
		extentCount := c.readU16()
		it.Extents = make([]IlocExtent, 0, extentCount)
		for e := uint16(0); e < extentCount; e++ {
			var ex IlocExtent
			if (fbh.Version == 1 || fbh.Version == 2) && i.IndexSize > 0 {
				ex.Index = c.readUN(int(i.IndexSize))
			}
			ex.Offset = c.readUN(int(i.OffsetSize))
			ex.Length = c.readUN(int(i.LengthSize))
			it.Extents = append(it.Extents, ex)
		}
		if c.err != nil {
			return nil, c.err
		}
		i.Items = append(i.Items, it)
	}
	return i, c.err
}

// validIlocWidth checks that w is one of the permitted nibble widths.
func validIlocWidth(w uint8, name string) error {
	switch w {
	case 0, 1, 2, 4, 8:
		return nil
	}
	return fmt.Errorf("%w: iloc %s size %d (expected 0/1/2/4/8)", ErrInvalid, name, w)
}
