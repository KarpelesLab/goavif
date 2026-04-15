package isobmff

import "fmt"

// StillImage describes the minimum inputs required to build a valid still
// AVIF container wrapping a single AV1-coded primary item.
type StillImage struct {
	Width   uint32
	Height  uint32
	BitDepth uint8 // 8, 10 or 12
	// Monochrome indicates no chroma planes.
	Monochrome bool
	// Chroma subsampling hints for av1C. Ignored when Monochrome.
	ChromaSubsamplingX uint8 // 0 or 1
	ChromaSubsamplingY uint8 // 0 or 1
	// AV1 Sequence Header OBU (and any metadata OBUs) to embed in av1C.
	ConfigOBUs []byte
	// AV1 bitstream for the primary item, as a sequence of OBUs.
	AV1Bitstream []byte
	// NCLX color info; leave Type zeroed to omit.
	NCLX *Colr
}

// BuildStillImage constructs a [Container] representing a valid still-image
// AVIF wrapping a single primary item coded with AV1.
//
// The returned container is ready to serialize via [Container.WriteTo]. The
// iloc inside already contains the correct byte length for the item and a
// placeholder extent offset (relative to mdat), which WriteTo patches into
// an absolute file offset during layout.
func BuildStillImage(s StillImage) (*Container, error) {
	if s.Width == 0 || s.Height == 0 {
		return nil, fmt.Errorf("%w: zero width or height", ErrInvalid)
	}
	if len(s.AV1Bitstream) == 0 {
		return nil, fmt.Errorf("%w: empty AV1 bitstream", ErrInvalid)
	}
	if s.BitDepth != 8 && s.BitDepth != 10 && s.BitDepth != 12 {
		return nil, fmt.Errorf("%w: unsupported bit depth %d", ErrInvalid, s.BitDepth)
	}
	ft := &Ftyp{
		MajorBrand: FourCCOf("avif"),
		CompatibleBrands: []FourCC{
			FourCCOf("avif"),
			FourCCOf("mif1"),
			FourCCOf("miaf"),
		},
	}

	hdlr := &Hdlr{
		HandlerType: FourCCOf("pict"),
	}
	pitm := &Pitm{
		FullBoxHeader: FullBoxHeader{Version: 0},
		ItemID:        1,
	}
	infe := &Infe{
		FullBoxHeader: FullBoxHeader{Version: 2},
		ItemID:        1,
		ItemType:      FourCCOf("av01"),
	}
	iinf := &Iinf{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Entries:       []*Infe{infe},
	}

	// iloc: one item, one extent; offset is mdat-relative (WriteTo rewrites).
	iloc := &Iloc{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Items: []IlocItem{{
			ItemID: 1,
			Extents: []IlocExtent{{
				Offset: 0,
				Length: uint64(len(s.AV1Bitstream)),
			}},
		}},
	}
	// Pick widths that accommodate both the item length and the eventual
	// absolute mdat offset that WriteTo will add. Using 8 byte widths on
	// the extent offset is safe and keeps iloc size stable under the
	// patching step.
	iloc.OffsetSize = 8
	iloc.LengthSize = 8
	iloc.BaseOffsetSize = 0

	// Property boxes.
	ispe := &Ispe{Width: s.Width, Height: s.Height}
	av1c := &Av1C{
		SeqProfile:           av1ProfileFor(s.BitDepth, s.Monochrome, s.ChromaSubsamplingX, s.ChromaSubsamplingY),
		SeqLevelIdx0:         1, // "level 2.0"; a conservative default
		HighBitdepth:         boolBit(s.BitDepth >= 10),
		TwelveBit:            boolBit(s.BitDepth == 12),
		Monochrome:           boolBit(s.Monochrome),
		ChromaSubsamplingX:   s.ChromaSubsamplingX,
		ChromaSubsamplingY:   s.ChromaSubsamplingY,
		ChromaSamplePosition: 0,
		ConfigOBUs:           s.ConfigOBUs,
	}
	channels := 3
	if s.Monochrome {
		channels = 1
	}
	pixiBits := make([]uint8, channels)
	for i := range pixiBits {
		pixiBits[i] = s.BitDepth
	}
	pixi := &Pixi{ChannelBits: pixiBits}

	ipcoProps := []Box{ispe, av1c, pixi}
	ipma := &Ipma{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Entries: []IpmaEntry{{
			ItemID: 1,
			Associations: []IpmaAssoc{
				{PropertyIndex: 1, Essential: false}, // ispe
				{PropertyIndex: 2, Essential: true},  // av1C — MIAF requires essential
				{PropertyIndex: 3, Essential: false}, // pixi
			},
		}},
	}
	if s.NCLX != nil && s.NCLX.Type == ColrTypeNCLX {
		ipcoProps = append(ipcoProps, s.NCLX)
		ipma.Entries[0].Associations = append(ipma.Entries[0].Associations,
			IpmaAssoc{PropertyIndex: uint16(len(ipcoProps)), Essential: false})
	}
	ipco := &Ipco{Properties: ipcoProps}
	iprp := &Iprp{Ipco: ipco, Ipma: []*Ipma{ipma}}

	meta := &Meta{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Children: []Box{
			hdlr,
			pitm,
			iloc,
			iinf,
			iprp,
		},
	}

	mdat := &Mdat{Data: append([]byte(nil), s.AV1Bitstream...)}

	return &Container{
		Ftyp: ft,
		Meta: meta,
		Mdat: mdat,
	}, nil
}

func boolBit(b bool) uint8 {
	if b {
		return 1
	}
	return 0
}

// av1ProfileFor returns the AV1 seq_profile given the bit depth / subsampling.
// See AV1 spec §6.4.1. Profile 0 = 4:2:0 8/10-bit, Profile 1 = 4:4:4 8/10-bit,
// Profile 2 = 4:2:2 or 12-bit. Monochrome always profile 0.
func av1ProfileFor(bitDepth uint8, mono bool, subX, subY uint8) uint8 {
	if mono || (subX == 1 && subY == 1) {
		if bitDepth == 12 {
			return 2
		}
		return 0
	}
	if subX == 0 && subY == 0 {
		if bitDepth == 12 {
			return 2
		}
		return 1
	}
	// 4:2:2 (subX=1, subY=0) is only profile 2.
	return 2
}
