package isobmff

import "fmt"

// TypeGridItem is the item_type for AVIF / HEIF grid items (§6.6.2).
var TypeGridItem = FourCCOf("grid")

// ImageGrid is the parsed payload of a grid-type item. Grid items
// reference `rows × columns` tile items via a dimg iref; the final
// image is the tile mosaic cropped to output_width × output_height.
type ImageGrid struct {
	Version      uint8
	Flags        uint8
	Rows         uint16 // rows_minus_one + 1
	Columns      uint16 // columns_minus_one + 1
	OutputWidth  uint32
	OutputHeight uint32
}

// ParseImageGrid decodes a grid item body. Spec §6.6.2.3.
func ParseImageGrid(data []byte) (*ImageGrid, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("%w: grid payload %d bytes < 8", ErrInvalid, len(data))
	}
	version := data[0]
	if version != 0 {
		return nil, fmt.Errorf("%w: grid version %d", ErrInvalid, version)
	}
	flags := data[1]
	fieldLength := (int(flags&1) + 1) * 16 // 16 or 32 bits per dimension
	rowsM1 := data[2]
	colsM1 := data[3]
	g := &ImageGrid{
		Version: version,
		Flags:   flags,
		Rows:    uint16(rowsM1) + 1,
		Columns: uint16(colsM1) + 1,
	}
	pos := 4
	if fieldLength == 16 {
		if len(data) < pos+4 {
			return nil, fmt.Errorf("%w: grid 16-bit dims truncated", ErrInvalid)
		}
		g.OutputWidth = uint32(data[pos])<<8 | uint32(data[pos+1])
		g.OutputHeight = uint32(data[pos+2])<<8 | uint32(data[pos+3])
	} else {
		if len(data) < pos+8 {
			return nil, fmt.Errorf("%w: grid 32-bit dims truncated", ErrInvalid)
		}
		g.OutputWidth = uint32(data[pos])<<24 | uint32(data[pos+1])<<16 | uint32(data[pos+2])<<8 | uint32(data[pos+3])
		g.OutputHeight = uint32(data[pos+4])<<24 | uint32(data[pos+5])<<16 | uint32(data[pos+6])<<8 | uint32(data[pos+7])
	}
	return g, nil
}

// FindDimgTargets returns the tile item IDs referenced via a dimg
// iref from sourceID, in association order. Returns nil when no
// such reference exists.
func (ct *Container) FindDimgTargets(sourceID uint32) []uint32 {
	if ct == nil || ct.Meta == nil {
		return nil
	}
	for _, ch := range ct.Meta.Children {
		iref, ok := ch.(*Iref)
		if !ok {
			continue
		}
		for _, e := range iref.Entries {
			if e.Type == TypeDimg && e.FromID == sourceID {
				out := make([]uint32, len(e.ToIDs))
				copy(out, e.ToIDs)
				return out
			}
		}
	}
	return nil
}

// ItemType returns the FourCC ItemType for the given itemID, looked
// up via iinf. Returns the zero FourCC when the item is not in iinf.
func (ct *Container) ItemType(itemID uint32) FourCC {
	if ct == nil || ct.Meta == nil {
		return FourCC{}
	}
	for _, ch := range ct.Meta.Children {
		iinf, ok := ch.(*Iinf)
		if !ok {
			continue
		}
		for _, e := range iinf.Entries {
			if e.ItemID == itemID {
				return e.ItemType
			}
		}
	}
	return FourCC{}
}
