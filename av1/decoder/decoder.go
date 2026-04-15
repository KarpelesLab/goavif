package decoder

import (
	"errors"
	"fmt"

	"github.com/KarpelesLab/goavif/av1/obu"
)

// ErrPixelDecodeUnimplemented is returned by [Decode] when the header
// parsing succeeds but the tile-level residual / reconstruction path has
// not yet been implemented for the frame's profile.
var ErrPixelDecodeUnimplemented = errors.New("av1/decoder: pixel reconstruction not yet implemented")

// Frame is a decoded AV1 frame. Planes are in the layout described by the
// sequence header's color configuration: 8-bit samples occupy one byte per
// element, 10/12-bit samples occupy two bytes per element.
//
// Stride values are measured in samples (not bytes). Y/U/V widths and
// heights account for chroma subsampling.
type Frame struct {
	Width    int
	Height   int
	BitDepth int
	Subsampling struct{ X, Y uint8 }
	Monochrome bool

	Y []byte
	U []byte
	V []byte

	YStride int
	CStride int

	Header *obu.FrameHeader
	Seq    *obu.SequenceHeader
}

// Decode parses the OBUs in itemData using seqHdr (typically from the
// containing AVIF's av1C box) and returns the decoded frame metadata.
//
// The pixel planes (Y/U/V) are not yet populated; callers will receive
// [ErrPixelDecodeUnimplemented] once the header parsing succeeds, until
// the tile-group decoder lands.
func Decode(itemData []byte, seqHdr *obu.SequenceHeader) (*Frame, error) {
	if seqHdr == nil {
		return nil, fmt.Errorf("av1/decoder: seqHdr is required")
	}

	obus, err := obu.Split(itemData)
	if err != nil {
		return nil, fmt.Errorf("av1/decoder: OBU split: %w", err)
	}

	var frameHdr *obu.FrameHeader
	var tileGroupPayload []byte
	for _, u := range obus {
		switch u.Header.Type {
		case obu.TypeTemporalDelimiter, obu.TypePadding, obu.TypeMetadata:
			// ignored for still-image decode
		case obu.TypeSequenceHeader:
			// AVIF may duplicate the av1C seq header in the item bitstream;
			// use the later one to match the spec's "latest seq header wins".
			sh, err := obu.ParseSequenceHeader(u.Payload)
			if err != nil {
				return nil, fmt.Errorf("av1/decoder: inline seq header: %w", err)
			}
			seqHdr = sh
		case obu.TypeFrame:
			// FRAME OBU = FrameHeader + TileGroup concatenated. We split by
			// frame-header byte span; a helper in av1/obu would be cleaner
			// but the minimal path is to parse and let the frame header
			// consumer report how many bytes it used.
			fh, tail, err := parseFrameOBU(u.Payload, seqHdr)
			if err != nil {
				return nil, err
			}
			frameHdr = fh
			tileGroupPayload = tail
		case obu.TypeFrameHeader, obu.TypeRedundantFrameHeader:
			fh, err := obu.ParseFrameHeader(u.Payload, seqHdr, nil)
			if err != nil {
				return nil, err
			}
			frameHdr = fh
		case obu.TypeTileGroup:
			tileGroupPayload = u.Payload
		}
	}

	if frameHdr == nil {
		return nil, fmt.Errorf("av1/decoder: no FRAME or FRAME_HEADER OBU")
	}
	_ = tileGroupPayload // consumed by the pixel path once implemented

	f := &Frame{
		Width:      int(frameHdr.FrameWidth),
		Height:     int(frameHdr.FrameHeight),
		BitDepth:   int(seqHdr.Color.BitDepth),
		Monochrome: seqHdr.Color.Monochrome,
		Header:     frameHdr,
		Seq:        seqHdr,
	}
	f.Subsampling.X = seqHdr.Color.SubsamplingX
	f.Subsampling.Y = seqHdr.Color.SubsamplingY

	return f, ErrPixelDecodeUnimplemented
}

// parseFrameOBU splits a FRAME OBU into its frame_header and tile_group
// parts. The spec says the frame header is followed by a byte-aligned tile
// group starting at the next byte. We use the frame header's internal
// cursor position to determine the split.
//
// NOTE: obu.ParseFrameHeader does not currently expose the number of bytes
// consumed. As a pragmatic stand-in we re-parse via a helper that tracks
// position; for now, we return the tail as nil and let the caller treat the
// FRAME OBU as header-only when pixel decoding is unimplemented.
func parseFrameOBU(payload []byte, seq *obu.SequenceHeader) (*obu.FrameHeader, []byte, error) {
	fh, err := obu.ParseFrameHeader(payload, seq, nil)
	if err != nil {
		return nil, nil, err
	}
	return fh, nil, nil
}
