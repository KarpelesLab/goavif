package isobmff

import (
	"bytes"
	"fmt"
)

// GridTile is one tile in a grid-structured AVIF image: its coded
// width / height (all tiles share dimensions in the HEIF spec) plus
// its AV1 bitstream.
type GridTile struct {
	// AV1Bitstream is the complete AV1 frame stream for this tile
	// (sequence-header OBU + frame OBU). Each tile is a standalone
	// still AV1 item.
	AV1Bitstream []byte
}

// GridImage describes an AVIF "grid" container: rows × columns tiles
// composited into an image of OutputWidth × OutputHeight. TileWidth /
// TileHeight are the shared coded dimensions of every tile.
type GridImage struct {
	OutputWidth  uint32
	OutputHeight uint32
	TileWidth    uint32
	TileHeight   uint32
	Rows         int
	Columns      int
	BitDepth     uint8 // 8, 10 or 12
	// Chroma subsampling of every tile. Shared across the grid.
	ChromaSubsamplingX uint8
	ChromaSubsamplingY uint8
	// ConfigOBUs is the shared AV1 sequence-header OBU wrapper used
	// by every tile's av1C property.
	ConfigOBUs []byte
	Tiles      []GridTile
}

// BuildGrid assembles an AVIF container whose primary item is a
// "grid"-type derived image referencing g.Rows × g.Columns tile
// items via a dimg iref, per HEIF §6.6.2.
//
// The grid payload encodes output dimensions, rows/cols count, and
// flags (bit 0 = 1 when dims need 32 bits). Tiles are emitted as
// plain av01 items in row-major order; each tile shares the
// ConfigOBUs property and the TileWidth × TileHeight ispe.
func BuildGrid(g GridImage) (*Container, error) {
	if g.Rows < 1 || g.Columns < 1 {
		return nil, fmt.Errorf("%w: grid must have at least 1 row × 1 column", ErrInvalid)
	}
	expected := g.Rows * g.Columns
	if len(g.Tiles) != expected {
		return nil, fmt.Errorf("%w: grid expects %d tiles, got %d", ErrInvalid, expected, len(g.Tiles))
	}
	if g.BitDepth != 8 && g.BitDepth != 10 && g.BitDepth != 12 {
		return nil, fmt.Errorf("%w: unsupported bit depth %d", ErrInvalid, g.BitDepth)
	}

	ft := &Ftyp{
		MajorBrand: FourCCOf("avif"),
		CompatibleBrands: []FourCC{
			FourCCOf("avif"),
			FourCCOf("mif1"),
			FourCCOf("miaf"),
		},
	}

	hdlr := &Hdlr{HandlerType: FourCCOf("pict")}

	// Item IDs: grid = 1; tiles = 2..(1+N).
	gridID := uint32(1)
	pitm := &Pitm{
		FullBoxHeader: FullBoxHeader{Version: 0},
		ItemID:        gridID,
	}

	// iinf: one infe for the grid (ItemType "grid") + one per tile
	// (ItemType "av01"). Tile infe items carry flags=1 (hidden) so
	// players don't render them individually — only the grid.
	infeEntries := []*Infe{
		{
			FullBoxHeader: FullBoxHeader{Version: 2},
			ItemID:        gridID,
			ItemType:      FourCCOf("grid"),
		},
	}
	tileIDs := make([]uint32, len(g.Tiles))
	for i := range g.Tiles {
		tileID := uint32(2 + i)
		tileIDs[i] = tileID
		infeEntries = append(infeEntries, &Infe{
			FullBoxHeader: FullBoxHeader{Version: 2, Flags: 1}, // hidden
			ItemID:        tileID,
			ItemType:      FourCCOf("av01"),
		})
	}
	iinf := &Iinf{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Entries:       infeEntries,
	}

	// Grid item payload — 8-byte 16-bit form, matching our decoder.
	gridPayload := encodeGridPayload(g)

	// iloc: grid item (gridPayload) + one extent per tile.
	ilocItems := make([]IlocItem, 0, 1+len(g.Tiles))
	ilocItems = append(ilocItems, IlocItem{
		ItemID: gridID,
		Extents: []IlocExtent{{
			Offset: 0,
			Length: uint64(len(gridPayload)),
		}},
	})
	offset := uint64(len(gridPayload))
	for i, t := range g.Tiles {
		ilocItems = append(ilocItems, IlocItem{
			ItemID: tileIDs[i],
			Extents: []IlocExtent{{
				Offset: offset,
				Length: uint64(len(t.AV1Bitstream)),
			}},
		})
		offset += uint64(len(t.AV1Bitstream))
	}
	iloc := &Iloc{
		FullBoxHeader:  FullBoxHeader{Version: 0},
		Items:          ilocItems,
		OffsetSize:     8,
		LengthSize:     8,
		BaseOffsetSize: 0,
	}

	// iref dimg(grid → tiles) lists tile IDs in raster order.
	iref := &Iref{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Entries: []IrefEntry{{
			Type:   TypeDimg,
			FromID: gridID,
			ToIDs:  tileIDs,
		}},
	}

	// iprp properties:
	//   1. ispe for the grid (OutputWidth × OutputHeight)
	//   2. ispe for tiles (TileWidth × TileHeight)
	//   3. av1C shared by every tile
	//   4. pixi shared by every tile
	//
	// Grid item gets property 1 only; tiles get 2 + 3 + 4.
	gridIspe := &Ispe{Width: g.OutputWidth, Height: g.OutputHeight}
	tileIspe := &Ispe{Width: g.TileWidth, Height: g.TileHeight}
	av1c := &Av1C{
		SeqProfile:           av1ProfileFor(g.BitDepth, false, g.ChromaSubsamplingX, g.ChromaSubsamplingY),
		SeqLevelIdx0:         1,
		HighBitdepth:         boolBit(g.BitDepth >= 10),
		TwelveBit:            boolBit(g.BitDepth == 12),
		Monochrome:           0,
		ChromaSubsamplingX:   g.ChromaSubsamplingX,
		ChromaSubsamplingY:   g.ChromaSubsamplingY,
		ChromaSamplePosition: 0,
		ConfigOBUs:           g.ConfigOBUs,
	}
	pixiBits := []uint8{g.BitDepth, g.BitDepth, g.BitDepth}
	pixi := &Pixi{ChannelBits: pixiBits}
	ipcoProps := []Box{gridIspe, tileIspe, av1c, pixi}
	ipma := &Ipma{FullBoxHeader: FullBoxHeader{Version: 0}}
	ipma.Entries = append(ipma.Entries, IpmaEntry{
		ItemID: gridID,
		Associations: []IpmaAssoc{
			{PropertyIndex: 1, Essential: false}, // grid ispe
		},
	})
	for _, tid := range tileIDs {
		ipma.Entries = append(ipma.Entries, IpmaEntry{
			ItemID: tid,
			Associations: []IpmaAssoc{
				{PropertyIndex: 2, Essential: false}, // tile ispe
				{PropertyIndex: 3, Essential: true},  // av1C
				{PropertyIndex: 4, Essential: false}, // pixi
			},
		})
	}

	ipco := &Ipco{Properties: ipcoProps}
	iprp := &Iprp{Ipco: ipco, Ipma: []*Ipma{ipma}}

	meta := &Meta{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Children:      []Box{hdlr, pitm, iloc, iinf, iref, iprp},
	}

	// mdat = gridPayload || tile0 || tile1 || ...
	var mdatBuf bytes.Buffer
	mdatBuf.Write(gridPayload)
	for _, t := range g.Tiles {
		mdatBuf.Write(t.AV1Bitstream)
	}
	mdat := &Mdat{Data: mdatBuf.Bytes()}

	return &Container{
		Ftyp: ft,
		Meta: meta,
		Mdat: mdat,
	}, nil
}

// encodeGridPayload serializes the grid item body per HEIF §6.6.2.3.
// Uses the 16-bit form (flags bit 0 = 0) when dims fit in 16 bits;
// switches to the 32-bit form otherwise.
func encodeGridPayload(g GridImage) []byte {
	use32 := g.OutputWidth > 0xFFFF || g.OutputHeight > 0xFFFF
	flags := uint8(0)
	if use32 {
		flags = 1
	}
	out := []byte{0, flags, byte(g.Rows - 1), byte(g.Columns - 1)}
	if use32 {
		out = append(out,
			byte(g.OutputWidth>>24), byte(g.OutputWidth>>16),
			byte(g.OutputWidth>>8), byte(g.OutputWidth),
			byte(g.OutputHeight>>24), byte(g.OutputHeight>>16),
			byte(g.OutputHeight>>8), byte(g.OutputHeight),
		)
	} else {
		out = append(out,
			byte(g.OutputWidth>>8), byte(g.OutputWidth),
			byte(g.OutputHeight>>8), byte(g.OutputHeight),
		)
	}
	return out
}
