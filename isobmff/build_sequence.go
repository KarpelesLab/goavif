package isobmff

import (
	"bytes"
	"fmt"
)

// SequenceFrame is one frame in an AVIS image sequence: the AV1
// bitstream for that frame (sequence header OBU + frame OBU) plus
// its presentation duration in media timescale units.
type SequenceFrame struct {
	// AV1Bitstream is the complete self-contained AV1 byte stream for
	// this frame. A sync sample (IsSync=true) is a standalone
	// keyframe; a non-sync sample is an inter-predicted frame whose
	// decode depends on the preceding sample's reconstructed output.
	AV1Bitstream []byte
	// DurationTicks is the frame's presentation duration measured in
	// [Sequence.Timescale] ticks.
	DurationTicks uint32
	// IsSync reports whether this frame is a sync sample (keyframe).
	// When false, the frame is inter-predicted against the preceding
	// frame and cannot be decoded in isolation. Frame 0 must always
	// be a sync sample. For backward compatibility, a zero value
	// (false) is treated as true for frame 0.
	IsSync bool
}

// Sequence describes an AVIS image sequence encode: frame dimensions,
// color configuration, per-frame bitstreams, and timing.
type Sequence struct {
	Width, Height      uint32
	BitDepth           uint8
	Monochrome         bool
	ChromaSubsamplingX uint8
	ChromaSubsamplingY uint8
	// ConfigOBUs is the AV1 configuration OBU blob (typically the
	// sequence-header OBU wrapped with its size field) embedded in
	// av1C. Matches [StillImage.ConfigOBUs].
	ConfigOBUs []byte
	// Timescale is the media timescale (ticks per second). Common
	// choice: 1000 for millisecond precision.
	Timescale uint32
	// Frames is the ordered list of per-frame bitstreams.
	Frames []SequenceFrame
}

// BuildSequence assembles a complete AVIS container from s. The
// returned byte slice is ready to write to disk; the container's
// ftyp brand is "avis" with "miaf" + "msf1" compatibility.
//
// Every frame is a sync sample. The first frame also serves as the
// primary still item so viewers that don't understand sequences can
// render a single-frame poster.
func BuildSequence(s Sequence) ([]byte, error) {
	if len(s.Frames) == 0 {
		return nil, fmt.Errorf("%w: sequence has no frames", ErrInvalid)
	}
	if s.Timescale == 0 {
		s.Timescale = 1000
	}
	if s.BitDepth != 8 && s.BitDepth != 10 && s.BitDepth != 12 {
		return nil, fmt.Errorf("%w: unsupported bit depth %d", ErrInvalid, s.BitDepth)
	}

	// mdat payload = concatenation of all frame bitstreams.
	var mdat bytes.Buffer
	sampleOffsets := make([]uint32, len(s.Frames)) // mdat-relative offsets
	sampleSizes := make([]uint32, len(s.Frames))
	for i, f := range s.Frames {
		sampleOffsets[i] = uint32(mdat.Len())
		sampleSizes[i] = uint32(len(f.AV1Bitstream))
		mdat.Write(f.AV1Bitstream)
	}
	totalDuration := uint64(0)
	for _, f := range s.Frames {
		totalDuration += uint64(f.DurationTicks)
	}

	// ftyp — AVIS brand.
	ft := &Ftyp{
		MajorBrand: FourCCOf("avis"),
		CompatibleBrands: []FourCC{
			FourCCOf("avif"),
			FourCCOf("avis"),
			FourCCOf("msf1"),
			FourCCOf("miaf"),
			FourCCOf("mif1"),
			FourCCOf("iso8"),
		},
	}

	// meta — carries the still-image primary item so players that
	// don't walk moov can still render a frame.
	meta, primaryItemSize := buildSequenceMeta(s, sampleSizes[0])

	// moov — builds with stco placeholders. We'll compute final
	// offsets after measuring meta+ftyp+moov.
	moov := buildSequenceMoov(s, sampleOffsets, sampleSizes, totalDuration)

	// First pass: marshal everything to sizes.
	var ftypBuf bytes.Buffer
	if err := writeBox(&ftypBuf, ft); err != nil {
		return nil, err
	}
	var metaBuf bytes.Buffer
	if err := writeBox(&metaBuf, meta); err != nil {
		return nil, err
	}
	var moovBuf bytes.Buffer
	if err := writeBox(&moovBuf, moov); err != nil {
		return nil, err
	}

	// The mdat payload starts after ftyp+meta+moov+mdat_header.
	mdatHeaderLen := headerLen(uint64(mdat.Len()), TypeMdat)
	mdatPayloadOff := uint64(ftypBuf.Len() + metaBuf.Len() + moovBuf.Len()) + mdatHeaderLen

	// Patch stco offsets: add mdatPayloadOff to every sample offset.
	// Also patch the first item's iloc offset to point into mdat.
	patchSequenceOffsets(moov, mdatPayloadOff)
	patchIlocPrimaryItem(meta, mdatPayloadOff)

	// Second pass: remarshal moov and meta with patched offsets.
	moovBuf.Reset()
	if err := writeBox(&moovBuf, moov); err != nil {
		return nil, err
	}
	metaBuf.Reset()
	if err := writeBox(&metaBuf, meta); err != nil {
		return nil, err
	}

	// Assemble.
	out := make([]byte, 0, ftypBuf.Len()+metaBuf.Len()+moovBuf.Len()+int(mdatHeaderLen)+mdat.Len())
	out = append(out, ftypBuf.Bytes()...)
	out = append(out, metaBuf.Bytes()...)
	out = append(out, moovBuf.Bytes()...)

	// mdat header + payload.
	var mdatOut bytes.Buffer
	if err := writeBox(&mdatOut, &Mdat{Data: mdat.Bytes()}); err != nil {
		return nil, err
	}
	out = append(out, mdatOut.Bytes()...)

	// Size check: header layout is stable (we only changed 32-bit values).
	_ = primaryItemSize
	return out, nil
}

// buildSequenceMeta constructs the meta box for an AVIS container.
// The primary item is the first frame, so non-sequence-aware viewers
// see a still poster.
func buildSequenceMeta(s Sequence, primarySize uint32) (*Meta, uint32) {
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
	iloc := &Iloc{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Items: []IlocItem{{
			ItemID: 1,
			Extents: []IlocExtent{{
				Offset: 0, // mdat-relative; patched after layout
				Length: uint64(primarySize),
			}},
		}},
		OffsetSize:     8,
		LengthSize:     8,
		BaseOffsetSize: 0,
	}
	ispe := &Ispe{Width: s.Width, Height: s.Height}
	av1c := &Av1C{
		SeqProfile:           av1ProfileFor(s.BitDepth, s.Monochrome, s.ChromaSubsamplingX, s.ChromaSubsamplingY),
		SeqLevelIdx0:         1,
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
				{PropertyIndex: 1, Essential: false},
				{PropertyIndex: 2, Essential: true},
				{PropertyIndex: 3, Essential: false},
			},
		}},
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
	return meta, primarySize
}

// patchIlocPrimaryItem adds mdatPayloadOff to item 1's extent offset
// so it points at the first sample inside mdat. Mirrors the still
// image layout where iloc offsets are absolute file offsets.
func patchIlocPrimaryItem(meta *Meta, mdatPayloadOff uint64) {
	for _, ch := range meta.Children {
		if il, ok := ch.(*Iloc); ok {
			for i := range il.Items {
				for j := range il.Items[i].Extents {
					il.Items[i].Extents[j].Offset += mdatPayloadOff
				}
			}
			return
		}
	}
}

// buildSequenceMoov builds the movie box hierarchy for the sequence.
// stco offsets start as mdat-relative and are patched in a second
// pass once the absolute mdat position is known.
func buildSequenceMoov(s Sequence, sampleOffsets, sampleSizes []uint32, totalDurationTicks uint64) *Moov {
	// mvhd.
	mvhd := &Mvhd{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Timescale:     s.Timescale,
		Duration:      totalDurationTicks,
		Rate:          0x00010000, // 1.0
		Volume:        0x0100,     // 1.0
		NextTrackID:   2,
	}

	// tkhd — track header (version 0, flags=7: enabled|inMovie|inPreview).
	tkhd := buildTkhd(s.Width, s.Height, totalDurationTicks)

	// mdhd — media header (same timescale as mvhd for simplicity).
	mdhd := buildMdhd(s.Timescale, totalDurationTicks)

	// hdlr — handler_type=pict.
	hdlr := &Hdlr{
		HandlerType: FourCCOf("pict"),
		Name:        "GoAvif Image Handler",
	}

	// vmhd — video media header.
	vmhd := buildVmhd()

	// dinf + dref + url — data references point at self.
	dinf := buildDinf()

	// stsd — sample description box containing one av01 entry.
	stsd := buildStsd(s)

	// stts — one entry per frame (or compact when frames share delta).
	stts := buildStts(s.Frames)

	// stsc — one chunk per frame (simplest layout).
	stsc := &Stsc{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Entries: []StscEntry{
			{FirstChunk: 1, SamplesPerChunk: 1, DescriptionIdx: 1},
		},
	}

	// stsz — per-sample sizes.
	stsz := &Stsz{
		FullBoxHeader: FullBoxHeader{Version: 0},
		SampleSize:    0, // variable sizes
		SampleCount:   uint32(len(s.Frames)),
		Sizes:         append([]uint32(nil), sampleSizes...),
	}

	// stco — one offset per chunk (= per sample).
	stco := &Stco{
		FullBoxHeader: FullBoxHeader{Version: 0},
		Offsets:       append([]uint32(nil), sampleOffsets...),
	}

	// stss — lists only sync samples. Frame 0 is always a sync
	// sample; later frames can be inter-predicted if SequenceFrame
	// IsSync=false. Spec allows omitting stss to mean "every sample
	// is sync"; we emit an explicit listing so players that look
	// for it get unambiguous signaling.
	syncNums := make([]uint32, 0, len(s.Frames))
	for i, f := range s.Frames {
		if i == 0 || f.IsSync {
			syncNums = append(syncNums, uint32(i+1))
		}
	}
	stss := &Stss{
		FullBoxHeader: FullBoxHeader{Version: 0},
		SampleNumbers: syncNums,
	}

	stbl := &Stbl{Children: []Box{stsd, stts, stsc, stsz, stco, stss}}
	minf := &Minf{Children: []Box{vmhd, dinf, stbl}}
	mdia := &Mdia{Children: []Box{mdhd, hdlr, minf}}
	trak := &Trak{Children: []Box{tkhd, mdia}}

	return &Moov{Children: []Box{mvhd, trak}}
}

// patchSequenceOffsets walks moov.trak.mdia.minf.stbl.stco and adds
// mdatPayloadOff to every offset so they point at the right file
// positions. Matches the stco convention in the spec.
func patchSequenceOffsets(moov *Moov, mdatPayloadOff uint64) {
	stco := findStco(moov)
	if stco == nil {
		return
	}
	for i := range stco.Offsets {
		stco.Offsets[i] += uint32(mdatPayloadOff)
	}
}

func findStco(moov *Moov) *Stco {
	for _, ch := range moov.Children {
		trak, ok := ch.(*Trak)
		if !ok {
			continue
		}
		for _, tc := range trak.Children {
			mdia, ok := tc.(*Mdia)
			if !ok {
				continue
			}
			for _, mc := range mdia.Children {
				minf, ok := mc.(*Minf)
				if !ok {
					continue
				}
				for _, nc := range minf.Children {
					stbl, ok := nc.(*Stbl)
					if !ok {
						continue
					}
					for _, sc := range stbl.Children {
						if stco, ok := sc.(*Stco); ok {
							return stco
						}
					}
				}
			}
		}
	}
	return nil
}

// buildStts compacts per-frame durations into run-length (count, delta)
// entries.
func buildStts(frames []SequenceFrame) *Stts {
	s := &Stts{FullBoxHeader: FullBoxHeader{Version: 0}}
	for _, f := range frames {
		if n := len(s.Entries); n > 0 && s.Entries[n-1].Delta == f.DurationTicks {
			s.Entries[n-1].Count++
			continue
		}
		s.Entries = append(s.Entries, SttsEntry{Count: 1, Delta: f.DurationTicks})
	}
	return s
}

// buildTkhd, buildMdhd, buildVmhd, buildDinf, buildStsd return boxes as
// RawBox with hand-constructed payloads. These box types aren't parsed
// typed by this package (they're just pass-through RawBox today) so
// constructing them as raw bytes keeps the build surface small.

func buildTkhd(width, height uint32, duration uint64) Box {
	b := newBuilder()
	// full_box header: version=0, flags=0x000007 (enabled + in_movie + in_preview)
	b.writeU8(0)    // version
	b.writeU8(0)    // flags hi
	b.writeU8(0)    // flags mid
	b.writeU8(0x07) // flags lo
	b.writeU32(0)   // creation_time
	b.writeU32(0)   // modification_time
	b.writeU32(1)   // track_ID
	b.writeU32(0)   // reserved
	b.writeU32(uint32(duration))
	b.writeU32(0) // reserved[0]
	b.writeU32(0) // reserved[1]
	b.writeU16(0) // layer
	b.writeU16(0) // alternate_group
	b.writeU16(0) // volume
	b.writeU16(0) // reserved
	// Unity display matrix.
	matrix := [9]uint32{0x00010000, 0, 0, 0, 0x00010000, 0, 0, 0, 0x40000000}
	for _, v := range matrix {
		b.writeU32(v)
	}
	b.writeU32(width << 16)  // width as 16.16 fixed
	b.writeU32(height << 16) // height as 16.16 fixed
	return &RawBox{Type: TypeTkhd, Payload: b.bytes()}
}

func buildMdhd(timescale uint32, duration uint64) Box {
	b := newBuilder()
	b.writeU8(0) // version
	b.writeU8(0)
	b.writeU8(0)
	b.writeU8(0)
	b.writeU32(0) // creation_time
	b.writeU32(0) // modification_time
	b.writeU32(timescale)
	b.writeU32(uint32(duration))
	// language (ISO-639-2/T, 15 bits: 5-bit * 3 letters-0x60). Use
	// "und" = 0x55C4.
	b.writeU16(0x55C4)
	b.writeU16(0) // pre_defined
	return &RawBox{Type: TypeMdhd, Payload: b.bytes()}
}

func buildVmhd() Box {
	b := newBuilder()
	b.writeU8(0) // version
	b.writeU8(0)
	b.writeU8(0)
	b.writeU8(1) // flags = 1 (spec requires)
	b.writeU16(0) // graphicsmode
	b.writeU16(0) // opcolor[0]
	b.writeU16(0) // opcolor[1]
	b.writeU16(0) // opcolor[2]
	return &RawBox{Type: FourCCOf("vmhd"), Payload: b.bytes()}
}

// buildDinf builds a minimal dinf with a self-referencing url entry.
func buildDinf() Box {
	// url  box: version=0, flags=1 (self-contained), empty payload.
	urlPayload := []byte{0x00, 0x00, 0x00, 0x01}
	urlBox := &RawBox{Type: FourCCOf("url "), Payload: urlPayload}
	var urlBuf bytes.Buffer
	_ = writeBox(&urlBuf, urlBox)

	// dref full box: version=0, flags=0, entry_count=1, then url.
	var drefPayload bytes.Buffer
	drefPayload.Write([]byte{0, 0, 0, 0}) // version + flags
	drefPayload.Write([]byte{0, 0, 0, 1}) // entry_count
	drefPayload.Write(urlBuf.Bytes())
	drefBox := &RawBox{Type: FourCCOf("dref"), Payload: drefPayload.Bytes()}

	// dinf container.
	var drefBuf bytes.Buffer
	_ = writeBox(&drefBuf, drefBox)
	return &RawBox{Type: FourCCOf("dinf"), Payload: drefBuf.Bytes()}
}

// buildStsd builds the sample description box with one av01 entry.
func buildStsd(s Sequence) Box {
	av01 := buildAv01SampleEntry(s)
	var av01Buf bytes.Buffer
	_ = writeBox(&av01Buf, av01)

	var stsdPayload bytes.Buffer
	stsdPayload.Write([]byte{0, 0, 0, 0}) // version + flags
	stsdPayload.Write([]byte{0, 0, 0, 1}) // entry_count
	stsdPayload.Write(av01Buf.Bytes())
	return &RawBox{Type: TypeStsd, Payload: stsdPayload.Bytes()}
}

// buildAv01SampleEntry builds the av01 VisualSampleEntry (§12.1.3 +
// AV1-in-ISOBMFF §2.3). The payload is:
//   - 6 reserved bytes (0)
//   - data_reference_index (u16)
//   - VisualSampleEntry fields (pre_defined, reserved, etc., total 70 bytes)
//   - av1C box
func buildAv01SampleEntry(s Sequence) Box {
	b := newBuilder()
	// SampleEntry header (8 bytes).
	for i := 0; i < 6; i++ {
		b.writeU8(0)
	}
	b.writeU16(1) // data_reference_index = 1
	// VisualSampleEntry fields (70 bytes total).
	b.writeU16(0) // pre_defined
	b.writeU16(0) // reserved
	for i := 0; i < 3; i++ {
		b.writeU32(0) // pre_defined[3]
	}
	b.writeU16(uint16(s.Width))
	b.writeU16(uint16(s.Height))
	b.writeU32(0x00480000) // horiz resolution 72 dpi (16.16)
	b.writeU32(0x00480000) // vert resolution
	b.writeU32(0)          // reserved
	b.writeU16(1)          // frame_count
	// compressor_name: 32 bytes (pstring-like: length-prefixed,
	// padded with zeros).
	name := "GoAVIF"
	b.writeU8(uint8(len(name)))
	for _, c := range name {
		b.writeU8(uint8(c))
	}
	for i := 0; i < 31-len(name); i++ {
		b.writeU8(0)
	}
	b.writeU16(0x0018) // depth (0x18 = 24)
	b.writeU16(0xFFFF) // pre_defined = -1

	// av1C child box.
	av1c := &Av1C{
		SeqProfile:           av1ProfileFor(s.BitDepth, s.Monochrome, s.ChromaSubsamplingX, s.ChromaSubsamplingY),
		SeqLevelIdx0:         1,
		HighBitdepth:         boolBit(s.BitDepth >= 10),
		TwelveBit:            boolBit(s.BitDepth == 12),
		Monochrome:           boolBit(s.Monochrome),
		ChromaSubsamplingX:   s.ChromaSubsamplingX,
		ChromaSubsamplingY:   s.ChromaSubsamplingY,
		ChromaSamplePosition: 0,
		ConfigOBUs:           s.ConfigOBUs,
	}
	var av1cBuf bytes.Buffer
	_ = writeBox(&av1cBuf, av1c)
	b.writeBytes(av1cBuf.Bytes())

	return &RawBox{Type: FourCCOf("av01"), Payload: b.bytes()}
}
