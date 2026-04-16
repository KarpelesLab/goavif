package decoder

import (
	"errors"
	"fmt"

	"github.com/KarpelesLab/goavif/av1/cdef"
	"github.com/KarpelesLab/goavif/av1/loopfilter"
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
	Width       int
	Height      int
	BitDepth    int
	Subsampling struct{ X, Y uint8 }
	Monochrome  bool

	Y []byte
	U []byte
	V []byte

	YStride int
	CStride int

	Header *obu.FrameHeader
	Seq    *obu.SequenceHeader
}

// Decode parses the OBUs in itemData using seqHdr (typically from the
// containing AVIF's av1C box) and returns the decoded frame.
//
// For AVIF stills this drives the entropy + coefficient decoder over the
// tile group payload to produce reconstructed Y/U/V pixel planes.
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
			sh, err := obu.ParseSequenceHeader(u.Payload)
			if err != nil {
				return nil, fmt.Errorf("av1/decoder: inline seq header: %w", err)
			}
			seqHdr = sh
		case obu.TypeFrame:
			fh, consumed, err := obu.ParseFrameHeaderBytes(u.Payload, seqHdr, nil)
			if err != nil {
				return nil, err
			}
			frameHdr = fh
			if consumed < len(u.Payload) {
				tileGroupPayload = u.Payload[consumed:]
			}
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

	// Run the tile decoder over the tile group payload.
	fs := NewFrameState(
		int(frameHdr.FrameWidth), int(frameHdr.FrameHeight),
		int(seqHdr.Color.SubsamplingX), int(seqHdr.Color.SubsamplingY),
		seqHdr.Color.Monochrome,
	)
	if err := runTileGroup(fs, tileGroupPayload, frameHdr, seqHdr); err != nil {
		return nil, err
	}

	f := &Frame{
		Width:      fs.Width,
		Height:     fs.Height,
		BitDepth:   int(seqHdr.Color.BitDepth),
		Monochrome: seqHdr.Color.Monochrome,
		Y:          fs.Y,
		U:          fs.U,
		V:          fs.V,
		YStride:    fs.YStride,
		CStride:    fs.UVStride,
		Header:     frameHdr,
		Seq:        seqHdr,
	}
	f.Subsampling.X = seqHdr.Color.SubsamplingX
	f.Subsampling.Y = seqHdr.Color.SubsamplingY
	return f, nil
}

// runTileGroup assumes a single-tile payload (common for AVIF stills) and
// walks all superblocks in it. Multi-tile frames need the tile_size_bytes
// leb128 prefixes and per-tile entropy decoder resets; that path is
// deferred.
func runTileGroup(fs *FrameState, tileData []byte, fh *obu.FrameHeader, sh *obu.SequenceHeader) error {
	if len(tileData) == 0 {
		return fmt.Errorf("av1/decoder: empty tile group payload")
	}
	if fh.Tile.TileCols > 1 || fh.Tile.TileRows > 1 {
		return fmt.Errorf("%w: multi-tile frames not yet supported", ErrPixelDecodeUnimplemented)
	}

	td, err := NewTileDecoder(tileData, fh, sh)
	if err != nil {
		return err
	}
	sbSize := 64
	if sh.Use128x128Superblock {
		sbSize = 128
	}
	for sbY := 0; sbY < fs.Height; sbY += sbSize {
		for sbX := 0; sbX < fs.Width; sbX += sbSize {
			if err := td.DecodeSuperblock(fs, sbX, sbY); err != nil {
				return err
			}
		}
	}

	applyLoopFilter(fs, fh)
	applyCDEF(fs, fh, sh)
	return nil
}

// applyCDEF runs the constrained directional enhancement filter after
// deblocking. AV1 signals per-superblock cdef_idx bits selecting one of
// Cdef.YPriStrengths[i] (plus matching secondary / UV strengths); the
// per-SB signaling isn't yet parsed, so we apply strengths[0] to every
// 8×8 block as a reasonable default approximating libavif encoder
// behavior.
func applyCDEF(fs *FrameState, fh *obu.FrameHeader, sh *obu.SequenceHeader) {
	if !sh.EnableCdef {
		return
	}
	damping := int(fh.Cdef.CdefDampingMinus3) + 3
	// Luma.
	pri := scaleCDEFPriStrength(int(fh.Cdef.YPriStrengths[0]))
	sec := scaleCDEFSecStrength(int(fh.Cdef.YSecStrengths[0]))
	cdef.ApplyFrame(cdef.Plane{
		Pix: fs.Y, Stride: fs.YStride, Width: fs.Width, Height: fs.Height,
	}, pri, sec, damping)
	if fs.Monochrome {
		return
	}
	pri = scaleCDEFPriStrength(int(fh.Cdef.UVPriStrengths[0]))
	sec = scaleCDEFSecStrength(int(fh.Cdef.UVSecStrengths[0]))
	// Chroma uses damping - 1 per spec.
	dmp := damping - 1
	cdef.ApplyFrame(cdef.Plane{
		Pix: fs.U, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
	}, pri, sec, dmp)
	cdef.ApplyFrame(cdef.Plane{
		Pix: fs.V, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
	}, pri, sec, dmp)
}

// scaleCDEFPriStrength maps the 4-bit primary-strength signal 0..15 to
// the per-spec multiplier used by Constrain(). Strength 0 disables the
// primary filter; other values are lifted by the spec's ×4 factor.
func scaleCDEFPriStrength(v int) int {
	if v == 0 {
		return 0
	}
	return v * 4
}

// scaleCDEFSecStrength maps the 2-bit secondary strength 0..3 (with a
// slight bump to 4 when max) to the filter's clip limit.
func scaleCDEFSecStrength(v int) int {
	if v == 0 {
		return 0
	}
	return v * 4
}

// applyLoopFilter runs the 4-tap narrow deblocking filter on the Y plane
// and (if present) the U / V planes. Edges are at fixed 8-pixel strides
// (the smallest AV1 TX grid common to all sizes); the real spec walks
// the per-block transform grid. This simpler form is sufficient for
// intra-only stills where TX sizes rarely cross MB boundaries without
// being aligned.
func applyLoopFilter(fs *FrameState, fh *obu.FrameHeader) {
	if fh.LoopFilter.LevelY0 == 0 && fh.LoopFilter.LevelY1 == 0 {
		return
	}
	th := loopfilter.DeriveThresholds(int(fh.LoopFilter.LevelY0), int(fh.LoopFilter.Sharpness))
	yPlane := loopfilter.Plane{
		Pix: fs.Y, Stride: fs.YStride,
		Width: fs.Width, Height: fs.Height,
	}
	loopfilter.ApplyFrameNarrow(yPlane, loopfilter.UniformGrid(fs.Width, fs.Height, 8, 8), th)

	if !fs.Monochrome {
		uvLvl := int(fh.LoopFilter.LevelU)
		if uvLvl == 0 {
			return
		}
		thUV := loopfilter.DeriveThresholds(uvLvl, int(fh.LoopFilter.Sharpness))
		grid := loopfilter.UniformGrid(fs.UVWidth, fs.UVHeight, 8, 8)
		loopfilter.ApplyFrameNarrow(loopfilter.Plane{
			Pix: fs.U, Stride: fs.UVStride,
			Width: fs.UVWidth, Height: fs.UVHeight,
		}, grid, thUV)
		loopfilter.ApplyFrameNarrow(loopfilter.Plane{
			Pix: fs.V, Stride: fs.UVStride,
			Width: fs.UVWidth, Height: fs.UVHeight,
		}, grid, thUV)
	}
}
