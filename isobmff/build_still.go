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
	// Alpha fields — populated when the image has an alpha channel.
	// Creates a second AV1 item (monochrome) linked to the primary via
	// auxl iref + auxC alpha URN.
	AlphaBitstream []byte
	AlphaConfigOBUs []byte
	AlphaBitDepth  uint8 // 8, 10 or 12; defaults to primary BitDepth when zero
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
	infeEntries := []*Infe{infe}
	if len(s.AlphaBitstream) > 0 {
		infeEntries = append(infeEntries, &Infe{
			FullBoxHeader: FullBoxHeader{Version: 2},
			ItemID:        2,
			ItemType:      FourCCOf("av01"),
		})
	}
	iinf := &Iinf{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Entries:       infeEntries,
	}

	// iloc: primary item + optional alpha item. Offsets are mdat-relative
	// (WriteTo rewrites).
	ilocItems := []IlocItem{{
		ItemID: 1,
		Extents: []IlocExtent{{
			Offset: 0,
			Length: uint64(len(s.AV1Bitstream)),
		}},
	}}
	if len(s.AlphaBitstream) > 0 {
		ilocItems = append(ilocItems, IlocItem{
			ItemID: 2,
			Extents: []IlocExtent{{
				Offset: uint64(len(s.AV1Bitstream)),
				Length: uint64(len(s.AlphaBitstream)),
			}},
		})
	}
	iloc := &Iloc{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Items:         ilocItems,
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

	// Alpha auxiliary item. Adds:
	//  - ispe (same dimensions as primary) — reused via second association
	//  - av1C for alpha — monochrome AV1 profile
	//  - pixi for alpha — one channel
	//  - auxC carrying the alpha URN — marks the item as alpha aux
	//  - iref auxl(alpha → primary)
	var iref *Iref
	if len(s.AlphaBitstream) > 0 {
		alphaBitDepth := s.AlphaBitDepth
		if alphaBitDepth == 0 {
			alphaBitDepth = s.BitDepth
		}
		alphaAv1c := &Av1C{
			SeqProfile:           av1ProfileFor(alphaBitDepth, true /* mono */, 1, 1),
			SeqLevelIdx0:         1,
			HighBitdepth:         boolBit(alphaBitDepth >= 10),
			TwelveBit:            boolBit(alphaBitDepth == 12),
			Monochrome:           1,
			ChromaSubsamplingX:   1,
			ChromaSubsamplingY:   1,
			ChromaSamplePosition: 0,
			ConfigOBUs:           s.AlphaConfigOBUs,
		}
		alphaPixi := &Pixi{ChannelBits: []uint8{alphaBitDepth}}
		alphaAuxC := &AuxC{AuxType: "urn:mpeg:mpegB:cicp:systems:auxiliary:alpha"}

		ipcoProps = append(ipcoProps, alphaAv1c, alphaPixi, alphaAuxC)
		alphaAv1cIdx := uint16(len(ipcoProps) - 2)
		alphaPixiIdx := uint16(len(ipcoProps) - 1)
		alphaAuxCIdx := uint16(len(ipcoProps))

		ipma.Entries = append(ipma.Entries, IpmaEntry{
			ItemID: 2,
			Associations: []IpmaAssoc{
				{PropertyIndex: 1, Essential: false},              // ispe (shared)
				{PropertyIndex: alphaAv1cIdx, Essential: true},    // av1C
				{PropertyIndex: alphaPixiIdx, Essential: false},   // pixi
				{PropertyIndex: alphaAuxCIdx, Essential: true},    // auxC
			},
		})

		iref = &Iref{
			FullBoxHeader: FullBoxHeader{Version: 0},
			Entries: []IrefEntry{{
				Type:   TypeAuxl,
				FromID: 2,
				ToIDs:  []uint32{1},
			}},
		}
	}

	ipco := &Ipco{Properties: ipcoProps}
	iprp := &Iprp{Ipco: ipco, Ipma: []*Ipma{ipma}}

	metaChildren := []Box{
		hdlr,
		pitm,
		iloc,
		iinf,
		iprp,
	}
	if iref != nil {
		// iref is conventionally placed before iprp but after iinf per
		// HEIF; the ordering is not strictly required by the spec but
		// matches what libavif emits.
		metaChildren = []Box{
			hdlr,
			pitm,
			iloc,
			iinf,
			iref,
			iprp,
		}
	}

	meta := &Meta{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Children:      metaChildren,
	}

	mdatData := append([]byte(nil), s.AV1Bitstream...)
	if len(s.AlphaBitstream) > 0 {
		mdatData = append(mdatData, s.AlphaBitstream...)
	}
	mdat := &Mdat{Data: mdatData}

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
