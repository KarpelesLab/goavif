package decoder

import (
	"errors"
	"fmt"

	"github.com/KarpelesLab/goavif/av1/cdef"
	"github.com/KarpelesLab/goavif/av1/filmgrain"
	"github.com/KarpelesLab/goavif/av1/loopfilter"
	"github.com/KarpelesLab/goavif/av1/lr"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// ErrPixelDecodeUnimplemented is returned by [Decode] when the header
// parsing succeeds but the tile-level residual / reconstruction path has
// not yet been implemented for the frame's profile.
var ErrPixelDecodeUnimplemented = errors.New("av1/decoder: pixel reconstruction not yet implemented")

// ErrInterFrameUnsupported is returned by [Decode] when the frame
// header signals an inter-predicted frame. goavif's decoder is
// intra-only today — callers that want a best-effort playback can
// check errors.Is and repeat the previous frame themselves, which
// is exactly what [goavif.DecodeAll] does.
var ErrInterFrameUnsupported = errors.New("av1/decoder: inter frame (motion compensation not implemented)")

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

	// Y / U / V hold the reconstructed planes for 8-bit frames; the
	// slices are nil when BitDepth > 8.
	Y []byte
	U []byte
	V []byte

	// Y16 / U16 / V16 hold the reconstructed planes for 10/12-bit
	// frames. nil when BitDepth == 8.
	Y16 []uint16
	U16 []uint16
	V16 []uint16

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
	return DecodeWithRef(itemData, seqHdr, nil)
}

// DecodeWithRef is the inter-capable variant of [Decode]: it accepts
// the previously decoded frame as a motion-compensation reference.
// For intra-only content pass nil for ref and the function behaves
// identically to [Decode].
//
// Inter-frame block decode is still landing (see ROADMAP Phase 5);
// this entry point currently returns [ErrInterFrameUnsupported] when
// the frame header signals an inter frame even with ref supplied,
// but the API is exposed so DecodeAll can thread refs through as
// the implementation matures.
func DecodeWithRef(itemData []byte, seqHdr *obu.SequenceHeader, ref *Frame) (*Frame, error) {
	if seqHdr == nil {
		return nil, fmt.Errorf("av1/decoder: seqHdr is required")
	}
	_ = ref // placeholder for future inter integration

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

	// Inter frames require a reference frame. Gate early if the
	// caller didn't supply one — without a ref the motion-
	// compensation source is missing.
	if !frameHdr.FrameIsIntra && ref == nil {
		return nil, fmt.Errorf("%w: inter frame needs a reference (use DecodeWithRef)",
			ErrInterFrameUnsupported)
	}

	// HBD inter decode isn't on the Phase 5 path — the uint16
	// tile decoder doesn't have the inter integration yet.
	if !frameHdr.FrameIsIntra && int(seqHdr.Color.BitDepth) > 8 {
		return nil, fmt.Errorf("%w: HBD inter frames not yet supported",
			ErrInterFrameUnsupported)
	}

	// Run the tile decoder over the tile group payload.
	bd := int(seqHdr.Color.BitDepth)
	var fs *FrameState
	if bd > 8 {
		fs = NewFrameStateHBD(
			int(frameHdr.FrameWidth), int(frameHdr.FrameHeight),
			int(seqHdr.Color.SubsamplingX), int(seqHdr.Color.SubsamplingY),
			seqHdr.Color.Monochrome, bd,
		)
	} else {
		fs = NewFrameState(
			int(frameHdr.FrameWidth), int(frameHdr.FrameHeight),
			int(seqHdr.Color.SubsamplingX), int(seqHdr.Color.SubsamplingY),
			seqHdr.Color.Monochrome,
		)
	}
	if err := runTileGroup(fs, tileGroupPayload, frameHdr, seqHdr, ref); err != nil {
		return nil, err
	}

	f := &Frame{
		Width:      fs.Width,
		Height:     fs.Height,
		BitDepth:   bd,
		Monochrome: seqHdr.Color.Monochrome,
		Y:          fs.Y,
		U:          fs.U,
		V:          fs.V,
		Y16:        fs.Y16,
		U16:        fs.U16,
		V16:        fs.V16,
		YStride:    fs.YStride,
		CStride:    fs.UVStride,
		Header:     frameHdr,
		Seq:        seqHdr,
	}
	f.Subsampling.X = seqHdr.Color.SubsamplingX
	f.Subsampling.Y = seqHdr.Color.SubsamplingY
	return f, nil
}

// runTileGroup walks all tiles described by the tile group payload
// (spec §5.11.1). For a single-tile frame the whole payload is the
// tile bitstream. For multi-tile frames the payload starts with a
// tile_start_and_end_present_flag (inferred here) + per-tile
// tile_size_minus_1 leb128 prefixes of width TileSizeBytes (derived
// from the frame header's TileSizeBytesMinus1).
func runTileGroup(fs *FrameState, tileData []byte, fh *obu.FrameHeader, sh *obu.SequenceHeader, ref *Frame) error {
	if len(tileData) == 0 {
		return fmt.Errorf("av1/decoder: empty tile group payload")
	}
	totalTiles := int(fh.Tile.TileCols) * int(fh.Tile.TileRows)
	if totalTiles <= 0 {
		totalTiles = 1
	}
	sbSize := 64
	if sh.Use128x128Superblock {
		sbSize = 128
	}

	tiles, err := splitTileGroup(tileData, totalTiles, int(fh.Tile.TileSizeBytesMinus1)+1)
	if err != nil {
		return err
	}

	// Decode every tile in raster order. The frame's superblock grid is
	// partitioned by fh.Tile.MiColStarts / MiRowStarts.
	cols := int(fh.Tile.TileCols)
	miColStarts := fh.Tile.MiColStarts
	miRowStarts := fh.Tile.MiRowStarts
	for t := 0; t < totalTiles; t++ {
		td, err := NewTileDecoderWithRef(tiles[t], fh, sh, ref)
		if err != nil {
			return fmt.Errorf("tile %d init: %w", t, err)
		}
		col := t % cols
		row := t / cols
		var sbXStart, sbYStart, sbXEnd, sbYEnd int
		// MiColStarts / MiRowStarts are stored in 8-pixel units by the
		// tile_info parser (each entry = sb_index * (sbSize>>3)). Convert
		// to pixels by multiplying by 8. This is a goavif-internal
		// convention and differs from the AV1 spec's 4-pixel MI unit.
		if len(miColStarts) > col+1 {
			sbXStart = int(miColStarts[col]) * 8
			sbXEnd = int(miColStarts[col+1]) * 8
		} else {
			sbXEnd = fs.Width
		}
		if len(miRowStarts) > row+1 {
			sbYStart = int(miRowStarts[row]) * 8
			sbYEnd = int(miRowStarts[row+1]) * 8
		} else {
			sbYEnd = fs.Height
		}
		if sbXEnd > fs.Width {
			sbXEnd = fs.Width
		}
		if sbYEnd > fs.Height {
			sbYEnd = fs.Height
		}
		for sbY := sbYStart; sbY < sbYEnd; sbY += sbSize {
			for sbX := sbXStart; sbX < sbXEnd; sbX += sbSize {
				if err := td.DecodeSuperblock(fs, sbX, sbY); err != nil {
					return fmt.Errorf("tile %d: %w", t, err)
				}
			}
		}
	}

	applyLoopFilter(fs, fh)
	applyCDEF(fs, fh, sh)
	applyLoopRestoration(fs, fh, sh)
	applyFilmGrain(fs, fh, sh)
	return nil
}

// applyLoopRestoration runs AV1's loop restoration pass after CDEF.
// Per-unit filter params would normally come from bitstream syntax
// (use_wiener / use_sgrproj + delta-coded coefficients per spec
// §7.17.1) — our decoder doesn't yet parse that signaling, so when
// FrameRestorationType is NONE the pass short-circuits. For other
// settings the driver runs but with zero-magnitude defaults that
// produce output identical to the input (the CDFs to properly decode
// per-unit params are the next layer of work).
func applyLoopRestoration(fs *FrameState, fh *obu.FrameHeader, sh *obu.SequenceHeader) {
	if !sh.EnableRestoration || !fh.LR.UsesLR {
		return
	}
	unitSize := int(1) << uint(fh.LR.Log2RestorationUnitSize[0])
	if unitSize < 64 {
		unitSize = 64
	}
	// Luma pass.
	if fh.LR.FrameRestorationType[0] != obu.RestorationNone {
		if fs.BitDepth > 8 {
			lr.ApplyFrame16(lr.Plane16{
				Pix: fs.Y16, Stride: fs.YStride, Width: fs.Width, Height: fs.Height,
			}, unitSize, defaultUnitParams(fh.LR.FrameRestorationType[0]), fs.BitDepth)
		} else {
			lr.ApplyFrame(lr.Plane{
				Pix: fs.Y, Stride: fs.YStride, Width: fs.Width, Height: fs.Height,
			}, unitSize, defaultUnitParams(fh.LR.FrameRestorationType[0]))
		}
	}
	if fs.Monochrome {
		return
	}
	uvUnit := unitSize
	if fh.LR.Log2RestorationUnitSize[1] > 0 {
		uvUnit = int(1) << uint(fh.LR.Log2RestorationUnitSize[1])
	}
	if fs.BitDepth > 8 {
		for plane, pix := range [2][]uint16{fs.U16, fs.V16} {
			typ := fh.LR.FrameRestorationType[1+plane]
			if typ == obu.RestorationNone {
				continue
			}
			lr.ApplyFrame16(lr.Plane16{
				Pix: pix, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
			}, uvUnit, defaultUnitParams(typ), fs.BitDepth)
		}
	} else {
		for plane, pix := range [2][]uint8{fs.U, fs.V} {
			typ := fh.LR.FrameRestorationType[1+plane]
			if typ == obu.RestorationNone {
				continue
			}
			lr.ApplyFrame(lr.Plane{
				Pix: pix, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
			}, uvUnit, defaultUnitParams(typ))
		}
	}
}

// defaultUnitParams returns a no-effect param provider for the given
// restoration type. Once use_wiener / use_sgrproj bitstream reads land,
// this is the hook the tile decoder replaces with a real callback.
func defaultUnitParams(rt uint8) lr.UnitFn {
	switch rt {
	case obu.RestorationWiener:
		return func(x, y int) lr.UnitParams {
			// Identity Wiener taps: output == input.
			return lr.UnitParams{
				Filter:      lr.FilterWiener,
				WienerHoriz: lr.WienerTaps{0, 0, 0, 128},
				WienerVert:  lr.WienerTaps{0, 0, 0, 128},
			}
		}
	case obu.RestorationSGR:
		return func(x, y int) lr.UnitParams {
			// Both radii 0 → pass-through.
			return lr.UnitParams{Filter: lr.FilterSGR, SGR: lr.SGRParams{}}
		}
	}
	return func(x, y int) lr.UnitParams { return lr.UnitParams{Filter: lr.FilterNone} }
}

// applyFilmGrain runs film grain synthesis on the reconstructed frame
// when the bitstream's film_grain_params carry a non-zero grain seed.
// AV1 specifies a 73×73 luma template + 38×38 chroma templates shaped
// by AR coefficients then tiled as 32×32 patches over the output.
//
// The spec's tile-structure-dependent hashing step is a simplification
// here — see av1/filmgrain/patch.go for the details. The luma /
// chroma scaling curves, the grain seed, and AR shaping all come from
// the parsed FilmGrainParams.
func applyFilmGrain(fs *FrameState, fh *obu.FrameHeader, sh *obu.SequenceHeader) {
	if !sh.FilmGrainParamsPresent {
		return
	}
	g := &fh.FilmGrain
	if !g.ApplyGrain || g.GrainSeed == 0 {
		return
	}

	arLag := int(g.ARCoeffLag)
	arShift := uint8(g.ARCoeffShiftMinus6) + 6
	scalingShift := uint8(g.GrainScaleShift) + 8

	// Luma path.
	if g.NumYPoints > 0 {
		points := make([]filmgrain.Point, g.NumYPoints)
		for i := uint8(0); i < g.NumYPoints; i++ {
			points[i] = filmgrain.Point{Value: g.PointYValue[i], Scale: g.PointYScaling[i]}
		}
		lut := filmgrain.BuildLUT(points)
		yCoeffs := g.ARCoeffsY[:2*arLag*(arLag+1)]
		tpl := filmgrain.NewLumaTemplate(g.GrainSeed, arLag, yCoeffs, arShift)
		p := &filmgrain.Params{
			GrainSeed:             g.GrainSeed,
			ScalingY:              lut,
			ScalingShift:          scalingShift,
			ClipToRestrictedRange: g.ClipToRestrictedRange,
		}
		if fs.BitDepth > 8 {
			filmgrain.ApplyWithTemplate16(fs.Y16, fs.Width, fs.Height, fs.YStride, &lut, &tpl, p, fs.BitDepth)
		} else {
			filmgrain.ApplyWithTemplate(fs.Y, fs.Width, fs.Height, fs.YStride, &lut, &tpl, p)
		}
	}

	if fs.Monochrome {
		return
	}
	buildChromaTemplate := func(numPoints uint8, values, scales [10]uint8, arCoeffs [25]int8) (filmgrain.ScalingLUT, filmgrain.Template, *filmgrain.Params, bool) {
		if numPoints == 0 {
			return filmgrain.ScalingLUT{}, filmgrain.Template{}, nil, false
		}
		points := make([]filmgrain.Point, numPoints)
		for i := uint8(0); i < numPoints; i++ {
			points[i] = filmgrain.Point{Value: values[i], Scale: scales[i]}
		}
		lut := filmgrain.BuildLUT(points)
		nCoeffs := 2*arLag*(arLag+1) + 1
		if nCoeffs > len(arCoeffs) {
			nCoeffs = len(arCoeffs)
		}
		tpl := filmgrain.NewChromaTemplate(g.GrainSeed^0xA5A5,
			arLag, arCoeffs[:nCoeffs], arShift)
		p := &filmgrain.Params{
			GrainSeed:             g.GrainSeed ^ 0xA5A5,
			ScalingShift:          scalingShift,
			ClipToRestrictedRange: g.ClipToRestrictedRange,
		}
		return lut, tpl, p, true
	}
	applyChroma := func(plane8 []uint8, plane16 []uint16, numPoints uint8,
		values, scales [10]uint8, arCoeffs [25]int8) {
		lut, tpl, p, ok := buildChromaTemplate(numPoints, values, scales, arCoeffs)
		if !ok {
			return
		}
		if fs.BitDepth > 8 {
			filmgrain.ApplyWithTemplate16(plane16, fs.UVWidth, fs.UVHeight, fs.UVStride, &lut, &tpl, p, fs.BitDepth)
		} else {
			filmgrain.ApplyWithTemplate(plane8, fs.UVWidth, fs.UVHeight, fs.UVStride, &lut, &tpl, p)
		}
	}
	applyChroma(fs.U, fs.U16, g.NumCbPoints, g.PointCbValue, g.PointCbScaling, g.ARCoeffsCb)
	applyChroma(fs.V, fs.V16, g.NumCrPoints, g.PointCrValue, g.PointCrScaling, g.ARCoeffsCr)
}

// splitTileGroup separates a tile group payload into numTiles independent
// tile byte slices. For single-tile frames (numTiles == 1) the whole
// payload is the one tile. Multi-tile frames have (numTiles - 1)
// "tile_size_minus_1" prefixes of tileSizeBytes bytes; the final tile
// has no size prefix (its bytes run to end of payload).
func splitTileGroup(payload []byte, numTiles, tileSizeBytes int) ([][]byte, error) {
	if numTiles <= 1 {
		return [][]byte{payload}, nil
	}
	out := make([][]byte, 0, numTiles)
	pos := 0
	for t := 0; t < numTiles-1; t++ {
		if pos+tileSizeBytes > len(payload) {
			return nil, fmt.Errorf("tile_group truncated at size prefix for tile %d", t)
		}
		size := uint64(0)
		for i := 0; i < tileSizeBytes; i++ {
			size |= uint64(payload[pos+i]) << (uint(i) * 8)
		}
		size++ // stored as size_minus_1
		pos += tileSizeBytes
		if pos+int(size) > len(payload) {
			return nil, fmt.Errorf("tile %d extends past tile_group (want %d, have %d)", t, size, len(payload)-pos)
		}
		out = append(out, payload[pos:pos+int(size)])
		pos += int(size)
	}
	// Final tile: remaining bytes.
	out = append(out, payload[pos:])
	return out, nil
}

// applyCDEF runs the constrained directional enhancement filter after
// deblocking. AV1 signals per-superblock cdef_idx bits selecting one of
// up to eight (primary, secondary) strength pairs. Per-SB idx values
// are decoded into fs.CdefIdx during the partition walk; SBs with the
// sentinel 255 fall back to strengths[0].
func applyCDEF(fs *FrameState, fh *obu.FrameHeader, sh *obu.SequenceHeader) {
	if !sh.EnableCdef {
		return
	}
	damping := int(fh.Cdef.CdefDampingMinus3) + 3
	// Luma — resolve (pri, sec) from the cdef_idx at the containing 64×64 SB.
	yStrength := func(x, y int) (int, int) {
		idx := cdefIdxAt(fs, x, y)
		return scaleCDEFPriStrength(int(fh.Cdef.YPriStrengths[idx])),
			scaleCDEFSecStrength(int(fh.Cdef.YSecStrengths[idx]))
	}
	if fs.BitDepth > 8 {
		cdef.ApplyFramePerSB16(cdef.Plane16{
			Pix: fs.Y16, Stride: fs.YStride, Width: fs.Width, Height: fs.Height,
		}, cdef.StrengthFn16(yStrength), damping, fs.BitDepth)
	} else {
		cdef.ApplyFramePerSB(cdef.Plane{
			Pix: fs.Y, Stride: fs.YStride, Width: fs.Width, Height: fs.Height,
		}, yStrength, damping)
	}
	if fs.Monochrome {
		return
	}
	// Chroma uses damping - 1 per spec. Chroma block-to-SB mapping must
	// account for subsampling: the 64×64 SB footprint in chroma is
	// (64>>SubX)×(64>>SubY). Scale x/y back up when looking up the idx.
	dmp := damping - 1
	uvStrength := func(x, y int) (int, int) {
		idx := cdefIdxAt(fs, x<<fs.SubX, y<<fs.SubY)
		return scaleCDEFPriStrength(int(fh.Cdef.UVPriStrengths[idx])),
			scaleCDEFSecStrength(int(fh.Cdef.UVSecStrengths[idx]))
	}
	if fs.BitDepth > 8 {
		cdef.ApplyFramePerSB16(cdef.Plane16{
			Pix: fs.U16, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
		}, cdef.StrengthFn16(uvStrength), dmp, fs.BitDepth)
		cdef.ApplyFramePerSB16(cdef.Plane16{
			Pix: fs.V16, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
		}, cdef.StrengthFn16(uvStrength), dmp, fs.BitDepth)
	} else {
		cdef.ApplyFramePerSB(cdef.Plane{
			Pix: fs.U, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
		}, uvStrength, dmp)
		cdef.ApplyFramePerSB(cdef.Plane{
			Pix: fs.V, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
		}, uvStrength, dmp)
	}
}

// cdefIdxAt returns the cdef_idx for the 64×64 SB containing plane
// coordinate (x, y), or 0 when the SB wasn't signaled (sentinel 255).
func cdefIdxAt(fs *FrameState, x, y int) int {
	col := x >> 6
	row := y >> 6
	if col >= fs.SBCols || row >= fs.SBRows {
		return 0
	}
	idx := fs.CdefIdx[row*fs.SBCols+col]
	if idx == 255 {
		return 0
	}
	return int(idx)
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
//
// The uint8 and uint16 (10/12-bit) paths diverge only in the filter
// primitives — threshold derivation, edge-grid walk and block layout
// are shared.
func applyLoopFilter(fs *FrameState, fh *obu.FrameHeader) {
	if fh.LoopFilter.LevelY0 == 0 && fh.LoopFilter.LevelY1 == 0 {
		return
	}
	th := loopfilter.DeriveThresholds(int(fh.LoopFilter.LevelY0), int(fh.LoopFilter.Sharpness))
	grid := loopfilter.UniformGrid(fs.Width, fs.Height, 8, 8)
	if fs.BitDepth > 8 {
		th16 := loopfilter.ScaleThresholds16(th, fs.BitDepth)
		loopfilter.ApplyFrameNarrow16(loopfilter.Plane16{
			Pix: fs.Y16, Stride: fs.YStride, Width: fs.Width, Height: fs.Height,
		}, grid, th16)
	} else {
		loopfilter.ApplyFrameNarrow(loopfilter.Plane{
			Pix: fs.Y, Stride: fs.YStride, Width: fs.Width, Height: fs.Height,
		}, grid, th)
	}

	if !fs.Monochrome {
		uvLvl := int(fh.LoopFilter.LevelU)
		if uvLvl == 0 {
			return
		}
		thUV := loopfilter.DeriveThresholds(uvLvl, int(fh.LoopFilter.Sharpness))
		uvGrid := loopfilter.UniformGrid(fs.UVWidth, fs.UVHeight, 8, 8)
		if fs.BitDepth > 8 {
			th16 := loopfilter.ScaleThresholds16(thUV, fs.BitDepth)
			loopfilter.ApplyFrameNarrow16(loopfilter.Plane16{
				Pix: fs.U16, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
			}, uvGrid, th16)
			loopfilter.ApplyFrameNarrow16(loopfilter.Plane16{
				Pix: fs.V16, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
			}, uvGrid, th16)
		} else {
			loopfilter.ApplyFrameNarrow(loopfilter.Plane{
				Pix: fs.U, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
			}, uvGrid, thUV)
			loopfilter.ApplyFrameNarrow(loopfilter.Plane{
				Pix: fs.V, Stride: fs.UVStride, Width: fs.UVWidth, Height: fs.UVHeight,
			}, uvGrid, thUV)
		}
	}
}
