package goavif

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/encoder"
	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/colorspace"
	"github.com/KarpelesLab/goavif/isobmff"
)

// ErrUnsupported is returned by entry points that are not yet implemented.
// It will be removed once the matching codec path lands.
var ErrUnsupported = errors.New("goavif: not yet implemented")

// Options tunes encoding behavior. It is not used by decoding.
type Options struct {
	// Quality 0..100, where 100 is highest quality. Ignored if Lossless.
	Quality int
	// Speed 0..10, where 10 is fastest. Trades encoding time for compression.
	Speed int
	// Lossless forces a lossless bitstream; overrides Quality.
	Lossless bool
	// BitDepth is one of 8, 10 or 12. Defaults to 8 when zero.
	BitDepth int
	// ChromaSubsampling selects the output chroma format.
	ChromaSubsampling ChromaSubsampling
	// Alpha, if true, includes the image's alpha channel as an auxiliary item.
	Alpha bool
	// InterEnabled, if true, enables inter-frame prediction for AVIS
	// image sequences — frames other than keyframes are coded as
	// INTER_FRAME against the previously decoded frame. Currently
	// restricted to 8-bit 4:2:0 color sequences; monochrome / HBD
	// fall back to the intra-only path. No effect on still-image
	// encoding.
	InterEnabled bool
	// KeyFrameInterval is the number of frames between keyframes
	// when InterEnabled is true. 0 or 1 means "every frame is a
	// keyframe" (intra-only behavior). A value of N means frames
	// 0, N, 2N, ... are keyframes. No effect when InterEnabled is
	// false.
	KeyFrameInterval int
	// TargetBytes enables target-size rate control: when non-zero,
	// [Encode] runs a Q-bisection loop and returns the best bitstream
	// within ±10% of the target (or the tightest quality-bounded
	// result if the target can't be hit). Overrides [Options.Quality]
	// when set. No effect on [EncodeAll] / [EncodeGrid] currently.
	TargetBytes int
}

// ChromaSubsampling identifies a YUV chroma sampling configuration.
type ChromaSubsampling int

const (
	// ChromaUnspecified lets the encoder pick based on the input image.
	ChromaUnspecified ChromaSubsampling = 0
	// Chroma420 = 4:2:0 (horizontal + vertical subsampling).
	Chroma420 ChromaSubsampling = 420
	// Chroma422 = 4:2:2 (horizontal subsampling only).
	Chroma422 ChromaSubsampling = 422
	// Chroma444 = 4:4:4 (no subsampling).
	Chroma444 ChromaSubsampling = 444
	// Chroma400 = monochrome.
	Chroma400 ChromaSubsampling = 400
)

// Decode reads an AVIF image from r and returns it as an [image.Image].
//
// The container and AV1 header parsing are implemented today. Pixel
// reconstruction is still landing; callers that hit an unimplemented code
// path receive an error wrapping [ErrUnsupported] or
// [decoder.ErrPixelDecodeUnimplemented].
func Decode(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	ct, err := isobmff.ParseContainer(data)
	if err != nil {
		return nil, err
	}
	if !ct.Ftyp.HasBrand("avif") && !ct.Ftyp.HasBrand("avis") {
		return nil, fmt.Errorf("goavif: ftyp has no avif/avis brand")
	}
	primaryID := ct.PrimaryItemID()
	if primaryID == 0 {
		return nil, fmt.Errorf("goavif: no primary item")
	}

	// Grid items (HEIF §6.6.2) split a large image into tile items
	// referenced via a dimg iref. Decode each tile and composite.
	if ct.ItemType(primaryID) == isobmff.TypeGridItem {
		img, err := decodeGridPrimary(ct, primaryID)
		if err != nil {
			return nil, err
		}
		return applyPrimaryPostTransforms(ct, primaryID, img), nil
	}

	seq, err := extractSequenceHeader(ct, primaryID)
	if err != nil {
		return nil, err
	}
	itemBytes, err := ct.ItemData(primaryID)
	if err != nil {
		return nil, err
	}
	frame, err := decoder.Decode(itemBytes, seq)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
	}

	// Alpha auxiliary: if the container signals an alpha item via auxl
	// iref + auxC alpha URN, decode it and composite into NRGBA /
	// NRGBA64. No alpha channel → passthrough.
	var img image.Image
	if alphaID := findAlphaItemID(ct, primaryID); alphaID != 0 {
		alpha, err := decodeAlphaFrame(ct, alphaID)
		if err != nil {
			return nil, fmt.Errorf("goavif: alpha decode: %w", err)
		}
		if frame.BitDepth > 8 {
			img, err = compositeNRGBA64(frame, alpha)
		} else {
			img, err = compositeNRGBA(frame, alpha)
		}
		if err != nil {
			return nil, err
		}
	} else {
		img, err = frameToImage(frame)
		if err != nil {
			return nil, err
		}
	}
	return applyPrimaryPostTransforms(ct, primaryID, img), nil
}

// applyPrimaryPostTransforms honors the primary item's ispe cropping,
// clap (clean-aperture) cropping, and irot / imir transform
// properties. Shared by the single-tile and grid decode paths.
//
// Per HEIF §6.5.10, the canonical application order is: clap → irot
// → imir. ispe-based cropping is applied first since it represents
// the author's declared size vs. our encoder's padded coded frame.
func applyPrimaryPostTransforms(ct *isobmff.Container, primaryID uint32, img image.Image) image.Image {
	if iw, ih, ok := primarySpatialExtents(ct, primaryID); ok {
		if int(iw) < img.Bounds().Dx() || int(ih) < img.Bounds().Dy() {
			img = cropToRect(img, image.Rect(0, 0, int(iw), int(ih)))
		}
	}
	if clap := primaryClap(ct, primaryID); clap != nil {
		img = applyClap(img, clap)
	}
	if props := primaryTransformProps(ct, primaryID); len(props) > 0 {
		img = applyTransforms(img, props)
	}
	return img
}

// primaryClap returns the clap (clean-aperture) property associated
// with itemID, or nil.
func primaryClap(ct *isobmff.Container, itemID uint32) *isobmff.Clap {
	iprp := findIprp(ct)
	if iprp == nil {
		return nil
	}
	for _, m := range iprp.Ipma {
		for _, e := range m.Entries {
			if e.ItemID != itemID {
				continue
			}
			for _, a := range e.Associations {
				if a.PropertyIndex == 0 || int(a.PropertyIndex) > len(iprp.Ipco.Properties) {
					continue
				}
				if c, ok := iprp.Ipco.Properties[a.PropertyIndex-1].(*isobmff.Clap); ok {
					return c
				}
			}
		}
	}
	return nil
}

// applyClap crops img per HEIF §6.5.10. clap carries rational values
// for crop width / height / horizontal-offset / vertical-offset; we
// evaluate them on pixel coordinates and return the resulting
// sub-image. Fractional results are rounded to the nearest integer.
func applyClap(img image.Image, clap *isobmff.Clap) image.Image {
	if clap.CleanApertureWidthD == 0 || clap.CleanApertureHeightD == 0 ||
		clap.HorizOffD == 0 || clap.VertOffD == 0 {
		return img
	}
	b := img.Bounds()
	W := b.Dx()
	H := b.Dy()
	// Crop width / height (rounded).
	cw := int((int64(clap.CleanApertureWidthN) + int64(clap.CleanApertureWidthD)/2) / int64(clap.CleanApertureWidthD))
	ch := int((int64(clap.CleanApertureHeightN) + int64(clap.CleanApertureHeightD)/2) / int64(clap.CleanApertureHeightD))
	if cw <= 0 || ch <= 0 || cw > W || ch > H {
		return img
	}
	// Crop center: (W-1)/2 + horizOff, (H-1)/2 + vertOff.
	centerX := (float64(W-1))/2.0 + float64(clap.HorizOffN)/float64(clap.HorizOffD)
	centerY := (float64(H-1))/2.0 + float64(clap.VertOffN)/float64(clap.VertOffD)
	x0 := int(centerX - float64(cw-1)/2.0 + 0.5)
	y0 := int(centerY - float64(ch-1)/2.0 + 0.5)
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x0+cw > W {
		x0 = W - cw
	}
	if y0+ch > H {
		y0 = H - ch
	}
	return cropToRect(img, image.Rect(b.Min.X+x0, b.Min.Y+y0, b.Min.X+x0+cw, b.Min.Y+y0+ch))
}

// decodeGridPrimary decodes a grid-type primary item by decoding each
// referenced tile (via dimg iref) and pasting them into an output
// image of size output_width × output_height. All tiles must share
// dimensions — the final row / column are cropped to fit output
// dimensions when tileW × columns > output_width.
func decodeGridPrimary(ct *isobmff.Container, gridID uint32) (image.Image, error) {
	gridBytes, err := ct.ItemData(gridID)
	if err != nil {
		return nil, fmt.Errorf("goavif: grid item data: %w", err)
	}
	grid, err := isobmff.ParseImageGrid(gridBytes)
	if err != nil {
		return nil, err
	}
	tileIDs := ct.FindDimgTargets(gridID)
	expected := int(grid.Rows) * int(grid.Columns)
	if len(tileIDs) != expected {
		return nil, fmt.Errorf("goavif: grid has %d tiles, dimg references %d", expected, len(tileIDs))
	}

	// Decode each tile via the single-item path, then paste into
	// the output canvas at its row/column offset.
	tiles := make([]image.Image, len(tileIDs))
	var tileW, tileH int
	for i, tid := range tileIDs {
		seq, err := extractSequenceHeader(ct, tid)
		if err != nil {
			return nil, fmt.Errorf("goavif: grid tile %d seq: %w", i, err)
		}
		itemBytes, err := ct.ItemData(tid)
		if err != nil {
			return nil, fmt.Errorf("goavif: grid tile %d item: %w", i, err)
		}
		frame, err := decoder.Decode(itemBytes, seq)
		if err != nil {
			return nil, fmt.Errorf("goavif: grid tile %d decode: %w", i, err)
		}
		img, err := frameToImage(frame)
		if err != nil {
			return nil, err
		}
		tiles[i] = img
		if i == 0 {
			tileW = img.Bounds().Dx()
			tileH = img.Bounds().Dy()
		} else if img.Bounds().Dx() != tileW || img.Bounds().Dy() != tileH {
			return nil, fmt.Errorf("goavif: grid tile %d dims %v differ from first %dx%d",
				i, img.Bounds(), tileW, tileH)
		}
	}
	out := image.NewRGBA(image.Rect(0, 0, int(grid.OutputWidth), int(grid.OutputHeight)))
	for i, t := range tiles {
		row := i / int(grid.Columns)
		col := i % int(grid.Columns)
		dstX := col * tileW
		dstY := row * tileH
		pasteClipped(out, t, dstX, dstY, int(grid.OutputWidth), int(grid.OutputHeight))
	}
	return out, nil
}

// pasteClipped copies tile pixels into dst at (dstX, dstY), clipped
// to (outW, outH).
func pasteClipped(dst *image.RGBA, tile image.Image, dstX, dstY, outW, outH int) {
	b := tile.Bounds()
	for y := 0; y < b.Dy(); y++ {
		if dstY+y >= outH {
			break
		}
		for x := 0; x < b.Dx(); x++ {
			if dstX+x >= outW {
				break
			}
			c := tile.At(b.Min.X+x, b.Min.Y+y)
			dst.Set(dstX+x, dstY+y, c)
		}
	}
}

// primaryTransformProps returns the ordered list of irot / imir
// transform properties associated with itemID, in the order they
// appear in the ipma entry.
func primaryTransformProps(ct *isobmff.Container, itemID uint32) []isobmff.Box {
	iprp := findIprp(ct)
	if iprp == nil {
		return nil
	}
	var out []isobmff.Box
	for _, m := range iprp.Ipma {
		for _, e := range m.Entries {
			if e.ItemID != itemID {
				continue
			}
			for _, a := range e.Associations {
				if a.PropertyIndex == 0 || int(a.PropertyIndex) > len(iprp.Ipco.Properties) {
					continue
				}
				switch p := iprp.Ipco.Properties[a.PropertyIndex-1].(type) {
				case *isobmff.Irot:
					if p.Angle != 0 {
						out = append(out, p)
					}
				case *isobmff.Imir:
					out = append(out, p)
				}
			}
		}
	}
	return out
}

// applyTransforms rebuilds img with irot / imir transforms applied in
// sequence. Each transform returns a freshly-allocated image of the
// rotated / mirrored pixels.
func applyTransforms(img image.Image, props []isobmff.Box) image.Image {
	for _, p := range props {
		switch t := p.(type) {
		case *isobmff.Irot:
			for i := uint8(0); i < t.Angle; i++ {
				img = rotate90CCW(img)
			}
		case *isobmff.Imir:
			if t.Axis == 0 {
				// vertical axis = mirror across horizontal axis (flip
				// top↔bottom). AVIF 1.1 errata redefined imir to the
				// same convention the HEIF spec always used.
				img = mirror(img, false)
			} else {
				img = mirror(img, true)
			}
		}
	}
	return img
}

// rotate90CCW returns a new RGBA image with img rotated 90° counter-
// clockwise.
func rotate90CCW(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// (x, y) → (y, w-1-x) for 90 CCW.
			out.Set(y, w-1-x, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return out
}

// mirror returns a new RGBA image with img flipped horizontally
// (flipH=true) or vertically (flipH=false).
func mirror(img image.Image, flipH bool) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx, sy := x, y
			if flipH {
				sx = w - 1 - x
			} else {
				sy = h - 1 - y
			}
			out.Set(x, y, img.At(b.Min.X+sx, b.Min.Y+sy))
		}
	}
	return out
}

// primarySpatialExtents returns the (width, height) from the primary
// item's ispe box. The third return value is false when ispe is
// absent or zero-valued.
func primarySpatialExtents(ct *isobmff.Container, itemID uint32) (uint32, uint32, bool) {
	iprp := findIprp(ct)
	if iprp == nil {
		return 0, 0, false
	}
	for _, m := range iprp.Ipma {
		for _, e := range m.Entries {
			if e.ItemID != itemID {
				continue
			}
			for _, a := range e.Associations {
				if a.PropertyIndex == 0 || int(a.PropertyIndex) > len(iprp.Ipco.Properties) {
					continue
				}
				if ispe, ok := iprp.Ipco.Properties[a.PropertyIndex-1].(*isobmff.Ispe); ok {
					if ispe.Width > 0 && ispe.Height > 0 {
						return ispe.Width, ispe.Height, true
					}
				}
			}
		}
	}
	return 0, 0, false
}

// cropToRect narrows img to rect. For standard library image types
// whose SubImage returns a shared-pixel view, we use that directly.
func cropToRect(img image.Image, rect image.Rectangle) image.Image {
	type subImager interface {
		SubImage(r image.Rectangle) image.Image
	}
	if s, ok := img.(subImager); ok {
		return s.SubImage(rect)
	}
	return img
}

// frameToImage builds an [image.Image] from a decoded [decoder.Frame].
//
// 8-bit frames return an image.YCbCr that shares the decoded Y/U/V
// planes to avoid copies. Callers needing RGB should use
// [colorspace.ConvertPlanar420] on the YCbCr planes directly.
//
// 10/12-bit frames return an image.RGBA64 produced by
// [colorspace.ConvertPlanar420_16] — the Go stdlib has no HBD planar
// image type, so RGB conversion happens on the decode path.
// Monochrome HBD returns image.Gray16 built from the high bits of the
// luma plane.
func frameToImage(f *decoder.Frame) (image.Image, error) {
	if f == nil {
		return nil, fmt.Errorf("goavif: nil frame")
	}
	if f.BitDepth > 8 {
		return frameToImage16(f)
	}
	if f.Monochrome {
		gray := image.NewGray(image.Rect(0, 0, f.Width, f.Height))
		if len(f.Y) != 0 {
			copy(gray.Pix, f.Y)
		}
		return gray, nil
	}
	// Planar YUV 4:2:0 is the only chroma sampling in the still-image AVIF
	// baseline we support today. 4:2:2 and 4:4:4 land with Phase 3.
	sub := image.YCbCrSubsampleRatio420
	switch {
	case f.Subsampling.X == 1 && f.Subsampling.Y == 1:
		sub = image.YCbCrSubsampleRatio420
	case f.Subsampling.X == 1 && f.Subsampling.Y == 0:
		sub = image.YCbCrSubsampleRatio422
	case f.Subsampling.X == 0 && f.Subsampling.Y == 0:
		sub = image.YCbCrSubsampleRatio444
	default:
		return nil, fmt.Errorf("goavif: unsupported chroma subsampling %d/%d",
			f.Subsampling.X, f.Subsampling.Y)
	}
	img := image.NewYCbCr(image.Rect(0, 0, f.Width, f.Height), sub)
	if len(f.Y) == len(img.Y) {
		copy(img.Y, f.Y)
	}
	if len(f.U) == len(img.Cb) {
		copy(img.Cb, f.U)
	}
	if len(f.V) == len(img.Cr) {
		copy(img.Cr, f.V)
	}
	return img, nil
}

// frameToImage16 converts a 10/12-bit decoded frame to an image.Image
// in 16-bit-per-channel space. Monochrome frames land in Gray16;
// 4:2:0 frames convert through YUV→RGB and return RGBA64. Other
// subsamplings (4:2:2, 4:4:4) are not yet implemented for HBD.
func frameToImage16(f *decoder.Frame) (image.Image, error) {
	if f.Monochrome {
		img := image.NewGray16(image.Rect(0, 0, f.Width, f.Height))
		shift := uint(16 - f.BitDepth)
		for i, v := range f.Y16 {
			s := uint16(v) << shift
			img.Pix[i*2+0] = uint8(s >> 8)
			img.Pix[i*2+1] = uint8(s & 0xFF)
		}
		return img, nil
	}
	img := image.NewRGBA64(image.Rect(0, 0, f.Width, f.Height))
	mc := colorspace.MCBT709
	rng := colorspace.Studio
	if f.Seq != nil {
		if f.Seq.Color.ColorRange {
			rng = colorspace.Full
		}
		if cicp := colorspace.MatrixCoefficients(f.Seq.Color.MatrixCoefficients); cicp != colorspace.MCUnspecified {
			mc = cicp
		}
	}
	colorspace.ConvertPlanar16(img.Pix, f.Y16, f.U16, f.V16, f.Width, f.Height,
		int(f.Subsampling.X), int(f.Subsampling.Y), mc, rng, f.BitDepth)
	return img, nil
}

// extractSequenceHeader finds the av1C property associated with itemID and
// parses its first OBU_SEQUENCE_HEADER OBU (there must be exactly one per
// AVIF spec). The av1C ConfigOBUs blob is encoded without OBU size fields,
// so we parse directly with an implicit length.
func extractSequenceHeader(ct *isobmff.Container, itemID uint32) (*obu.SequenceHeader, error) {
	iprp := findIprp(ct)
	if iprp == nil {
		return nil, fmt.Errorf("goavif: no iprp")
	}
	var av1c *isobmff.Av1C
	for _, m := range iprp.Ipma {
		for _, e := range m.Entries {
			if e.ItemID != itemID {
				continue
			}
			for _, a := range e.Associations {
				if a.PropertyIndex == 0 || int(a.PropertyIndex) > len(iprp.Ipco.Properties) {
					continue
				}
				if c, ok := iprp.Ipco.Properties[a.PropertyIndex-1].(*isobmff.Av1C); ok {
					av1c = c
				}
			}
		}
	}
	if av1c == nil {
		return nil, fmt.Errorf("goavif: item %d has no av1C", itemID)
	}
	// av1C's ConfigOBUs blob carries OBUs that do have a size field per the
	// AV1-in-ISOBMFF binding (§2.3), so Split works directly.
	obus, err := obu.Split(av1c.ConfigOBUs)
	if err != nil {
		return nil, fmt.Errorf("goavif: av1C OBU split: %w", err)
	}
	for _, u := range obus {
		if u.Header.Type == obu.TypeSequenceHeader {
			sh, err := obu.ParseSequenceHeader(u.Payload)
			if err != nil {
				return nil, fmt.Errorf("goavif: av1C sequence header: %w", err)
			}
			return sh, nil
		}
	}
	return nil, fmt.Errorf("goavif: av1C has no sequence header OBU")
}

// imageToYUV420 extracts BT.601 Y/Cb/Cr planes from an image at
// 4:2:0 subsampling. Convenience wrapper for [imageToYUV].
func imageToYUV420(m image.Image) (y, u, v []uint8) {
	return imageToYUV(m, 1, 1)
}

// imageToYUV extracts BT.601 Y/Cb/Cr planes at the given chroma
// subsampling factors (subX / subY ∈ {0, 1}):
//
//   - 4:2:0: subX=1, subY=1 → chroma is (w/2)×(h/2), 2×2 box-averaged
//   - 4:2:2: subX=1, subY=0 → chroma is (w/2)×h, 2×1 horizontal average
//   - 4:4:4: subX=0, subY=0 → chroma is w×h, no averaging
//
// Fast paths avoid m.At's per-pixel interface allocation for the
// common *image.RGBA / *image.NRGBA / *image.YCbCr types; other
// types fall through to the generic m.At path.
func imageToYUV(m image.Image, subX, subY int) (y, u, v []uint8) {
	// *image.YCbCr fast path: if the source's native subsampling
	// matches the requested output layout, copy planes directly
	// and skip the RGB round-trip entirely.
	if yc, ok := m.(*image.YCbCr); ok {
		if srcSubX, srcSubY, ok := ycbcrSubFactors(yc.SubsampleRatio); ok && srcSubX == subX && srcSubY == subY {
			return copyYCbCrPlanes(yc)
		}
	}

	bounds := m.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	y = make([]uint8, w*h)
	cw := w >> subX
	ch := h >> subY
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	u = make([]uint8, cw*ch)
	v = make([]uint8, cw*ch)
	uf := make([]int, w*h)
	vf := make([]int, w*h)

	readRGB := rgbReader(m)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			R, G, B := readRGB(bounds.Min.X+c, bounds.Min.Y+r)
			yv := (66*R + 129*G + 25*B + 128) >> 8
			uv := (-38*R - 74*G + 112*B + 128) >> 8
			vv := (112*R - 94*G - 18*B + 128) >> 8
			y[r*w+c] = clampByte(yv + 16)
			uf[r*w+c] = uv + 128
			vf[r*w+c] = vv + 128
		}
	}
	// Chroma box-average: (1<<subX) × (1<<subY) samples per chroma cell.
	sx := 1 << subX
	sy := 1 << subY
	for cr := 0; cr < ch; cr++ {
		for cc := 0; cc < cw; cc++ {
			su, sv := 0, 0
			n := 0
			for dy := 0; dy < sy && cr*sy+dy < h; dy++ {
				for dx := 0; dx < sx && cc*sx+dx < w; dx++ {
					idx := (cr*sy+dy)*w + (cc*sx + dx)
					su += uf[idx]
					sv += vf[idx]
					n++
				}
			}
			if n > 0 {
				u[cr*cw+cc] = clampByte(su / n)
				v[cr*cw+cc] = clampByte(sv / n)
			}
		}
	}
	return y, u, v
}

// ycbcrSubFactors maps a YCbCr SubsampleRatio to (subX, subY). Returns
// ok=false for ratios we don't handle (currently everything but
// 4:2:0 / 4:2:2 / 4:4:4).
func ycbcrSubFactors(r image.YCbCrSubsampleRatio) (subX, subY int, ok bool) {
	switch r {
	case image.YCbCrSubsampleRatio420:
		return 1, 1, true
	case image.YCbCrSubsampleRatio422:
		return 1, 0, true
	case image.YCbCrSubsampleRatio444:
		return 0, 0, true
	}
	return 0, 0, false
}

// copyYCbCrPlanes returns tightly-packed copies of yc's planes. The
// result matches the encoder tile writer's row-major convention
// (stride == width for every plane).
func copyYCbCrPlanes(yc *image.YCbCr) (y, u, v []uint8) {
	b := yc.Rect
	w, h := b.Dx(), b.Dy()
	y = make([]uint8, w*h)
	for r := 0; r < h; r++ {
		yiStart := yc.YOffset(b.Min.X, b.Min.Y+r)
		copy(y[r*w:r*w+w], yc.Y[yiStart:yiStart+w])
	}
	var chromaSubY int
	cw, ch := w, h
	switch yc.SubsampleRatio {
	case image.YCbCrSubsampleRatio420:
		cw, ch = w>>1, h>>1
		chromaSubY = 1
	case image.YCbCrSubsampleRatio422:
		cw = w >> 1
	}
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	u = make([]uint8, cw*ch)
	v = make([]uint8, cw*ch)
	for r := 0; r < ch; r++ {
		// Walk chroma rows at full-res Y intervals when subY=1.
		cOff := yc.COffset(b.Min.X, b.Min.Y+(r<<uint(chromaSubY)))
		copy(u[r*cw:r*cw+cw], yc.Cb[cOff:cOff+cw])
		copy(v[r*cw:r*cw+cw], yc.Cr[cOff:cOff+cw])
	}
	return y, u, v
}

// rgbReader returns a closure that reads 8-bit R/G/B components at
// image coordinates (x, y). It specialises on the concrete image
// type to avoid the Color-interface allocation that m.At does every
// pixel.
func rgbReader(m image.Image) func(x, y int) (R, G, B int) {
	switch src := m.(type) {
	case *image.RGBA:
		return func(x, y int) (int, int, int) {
			i := (y-src.Rect.Min.Y)*src.Stride + (x-src.Rect.Min.X)*4
			// RGBA stores premultiplied alpha; for a fully-opaque
			// source (which is the common case here) this equals the
			// non-premultiplied value. For translucent pixels the
			// chroma is computed from premultiplied samples, which is
			// consistent with how Color.RGBA() reports them.
			return int(src.Pix[i]), int(src.Pix[i+1]), int(src.Pix[i+2])
		}
	case *image.NRGBA:
		return func(x, y int) (int, int, int) {
			i := (y-src.Rect.Min.Y)*src.Stride + (x-src.Rect.Min.X)*4
			return int(src.Pix[i]), int(src.Pix[i+1]), int(src.Pix[i+2])
		}
	case *image.YCbCr:
		return func(x, y int) (int, int, int) {
			yi := src.YOffset(x, y)
			ci := src.COffset(x, y)
			Y := int(src.Y[yi])
			Cb := int(src.Cb[ci])
			Cr := int(src.Cr[ci])
			// BT.601 YCbCr → RGB. Standard library uses the same
			// formula in image.YCbCrToRGB.
			r := (298*(Y-16) + 409*(Cr-128) + 128) >> 8
			g := (298*(Y-16) - 100*(Cb-128) - 208*(Cr-128) + 128) >> 8
			b := (298*(Y-16) + 516*(Cb-128) + 128) >> 8
			return clampInt(r), clampInt(g), clampInt(b)
		}
	}
	// Fallback: generic m.At. Allocates a Color per pixel for
	// image types we don't specialise.
	bounds := m.Bounds()
	_ = bounds
	return func(x, y int) (int, int, int) {
		rr, gg, bb, _ := m.At(x, y).RGBA()
		return int(rr >> 8), int(gg >> 8), int(bb >> 8)
	}
}

func clampInt(v int) int {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// padToMultiple returns an image whose dimensions are the smallest
// multiples of align ≥ the source dimensions. Border rows/columns
// repeat the last source pixel (edge-extend). The original image is
// returned unchanged when already aligned.
//
// The returned image has the same concrete type as the input for
// the types we specialize on (*image.RGBA, *image.NRGBA,
// *image.Gray, *image.NRGBA64, *image.RGBA64, *image.Gray16,
// *image.YCbCr); other types drop through to a generic *image.RGBA
// container built via m.At.
func padToMultiple(m image.Image, align int) image.Image {
	bounds := m.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	pw := ((w + align - 1) / align) * align
	ph := ((h + align - 1) / align) * align
	if pw == w && ph == h {
		return m
	}
	// Build a fresh image of the padded size. Start by copying the
	// source into the top-left, then edge-extend.
	switch src := m.(type) {
	case *image.RGBA:
		return padRGBALike(src.Pix, src.Stride, 4, bounds, pw, ph, func(pix []uint8, stride, pw, ph int) image.Image {
			return &image.RGBA{Pix: pix, Stride: stride, Rect: image.Rect(0, 0, pw, ph)}
		})
	case *image.NRGBA:
		return padRGBALike(src.Pix, src.Stride, 4, bounds, pw, ph, func(pix []uint8, stride, pw, ph int) image.Image {
			return &image.NRGBA{Pix: pix, Stride: stride, Rect: image.Rect(0, 0, pw, ph)}
		})
	case *image.Gray:
		return padRGBALike(src.Pix, src.Stride, 1, bounds, pw, ph, func(pix []uint8, stride, pw, ph int) image.Image {
			return &image.Gray{Pix: pix, Stride: stride, Rect: image.Rect(0, 0, pw, ph)}
		})
	case *image.RGBA64:
		return padRGBALike(src.Pix, src.Stride, 8, bounds, pw, ph, func(pix []uint8, stride, pw, ph int) image.Image {
			return &image.RGBA64{Pix: pix, Stride: stride, Rect: image.Rect(0, 0, pw, ph)}
		})
	case *image.NRGBA64:
		return padRGBALike(src.Pix, src.Stride, 8, bounds, pw, ph, func(pix []uint8, stride, pw, ph int) image.Image {
			return &image.NRGBA64{Pix: pix, Stride: stride, Rect: image.Rect(0, 0, pw, ph)}
		})
	case *image.Gray16:
		return padRGBALike(src.Pix, src.Stride, 2, bounds, pw, ph, func(pix []uint8, stride, pw, ph int) image.Image {
			return &image.Gray16{Pix: pix, Stride: stride, Rect: image.Rect(0, 0, pw, ph)}
		})
	}
	// Generic fallback: rebuild as RGBA via m.At.
	dst := image.NewRGBA(image.Rect(0, 0, pw, ph))
	for y := 0; y < ph; y++ {
		sy := y
		if sy >= h {
			sy = h - 1
		}
		for x := 0; x < pw; x++ {
			sx := x
			if sx >= w {
				sx = w - 1
			}
			dst.Set(x, y, m.At(bounds.Min.X+sx, bounds.Min.Y+sy))
		}
	}
	return dst
}

// padRGBALike is the shared edge-extend routine for all Pix-based
// image types. bpp is the bytes-per-pixel for the concrete type.
func padRGBALike(srcPix []uint8, srcStride, bpp int, bounds image.Rectangle, pw, ph int,
	build func(pix []uint8, stride, pw, ph int) image.Image) image.Image {
	w, h := bounds.Dx(), bounds.Dy()
	dstStride := pw * bpp
	dst := make([]uint8, ph*dstStride)
	// Copy source rows into the top of dst.
	for y := 0; y < h; y++ {
		srcRow := (bounds.Min.Y+y)*srcStride + bounds.Min.X*bpp
		dstRow := y * dstStride
		copy(dst[dstRow:dstRow+w*bpp], srcPix[srcRow:srcRow+w*bpp])
		// Extend right edge.
		for x := w; x < pw; x++ {
			copy(dst[dstRow+x*bpp:dstRow+(x+1)*bpp], dst[dstRow+(w-1)*bpp:dstRow+w*bpp])
		}
	}
	// Extend bottom edge by repeating last source row.
	if h > 0 {
		lastRow := dst[(h-1)*dstStride : h*dstStride]
		for y := h; y < ph; y++ {
			copy(dst[y*dstStride:(y+1)*dstStride], lastRow)
		}
	}
	return build(dst, dstStride, pw, ph)
}

func clampByte(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}

// isGrayscale reports whether m should be encoded as a monochrome
// AV1 item. Currently recognizes image.Gray and image.Gray16.
func isGrayscale(m image.Image) bool {
	switch m.(type) {
	case *image.Gray, *image.Gray16:
		return true
	}
	return false
}

// hbdBitDepth picks a bit depth for the encoded primary item.
//
// Explicit opts.BitDepth wins when in {8, 10, 12}. Otherwise the
// decision comes from the input image type: 16-bit-per-channel
// Go types (NRGBA64 / RGBA64 / Gray16) opt in to 10-bit encoding;
// 8-bit types default to 8-bit.
func hbdBitDepth(m image.Image, opts *Options) int {
	if opts != nil && opts.BitDepth != 0 {
		switch opts.BitDepth {
		case 8:
			return 8
		case 10:
			return 10
		case 12:
			return 12
		}
	}
	switch m.(type) {
	case *image.NRGBA64, *image.RGBA64, *image.Gray16:
		return 10
	}
	return 8
}

// imageToLuma16 extracts a w*h HBD luma plane. Samples are in
// [0, (1<<bitDepth)-1], compressed from the input range into BT.601
// studio luma when applicable.
func imageToLuma16(m image.Image, bitDepth int) []uint16 {
	bounds := m.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := make([]uint16, w*h)
	// Studio luma range at N bits: [16 << (N-8), 235 << (N-8)].
	offset := 16 << uint(bitDepth-8)
	scale := 219 << uint(bitDepth-8)
	maxV := (1 << uint(bitDepth)) - 1
	switch src := m.(type) {
	case *image.Gray:
		for y := 0; y < h; y++ {
			base := (bounds.Min.Y+y-src.Rect.Min.Y)*src.Stride + (bounds.Min.X - src.Rect.Min.X)
			for x := 0; x < w; x++ {
				v := int(src.Pix[base+x])
				out[y*w+x] = clampU16(offset+(v*scale+128)>>8, maxV)
			}
		}
		return out
	case *image.Gray16:
		for y := 0; y < h; y++ {
			base := (bounds.Min.Y+y-src.Rect.Min.Y)*src.Stride + (bounds.Min.X-src.Rect.Min.X)*2
			for x := 0; x < w; x++ {
				// Downscale big-endian 16-bit Gray to bitDepth.
				v16 := int(src.Pix[base+x*2])<<8 | int(src.Pix[base+x*2+1])
				v := v16 >> uint(16-bitDepth)
				// Compress full-range to studio range.
				out[y*w+x] = clampU16(offset+(v*scale+((1<<uint(bitDepth-1))))>>uint(bitDepth), maxV)
			}
		}
		return out
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			yc, _, _, _ := m.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			v := int(yc) >> uint(16-bitDepth)
			out[y*w+x] = clampU16(offset+(v*scale+((1<<uint(bitDepth-1))))>>uint(bitDepth), maxV)
		}
	}
	return out
}

// imageToYUV420_16 extracts BT.601 Y/Cb/Cr planes at the given bit
// depth with 4:2:0 subsampling.
func imageToYUV420_16(m image.Image, bitDepth int) (y, u, v []uint16) {
	return imageToYUV16(m, bitDepth, 1, 1)
}

// imageToYUV16 is the HBD counterpart of [imageToYUV]. Output
// samples occupy [0, (1<<bitDepth)-1].
func imageToYUV16(m image.Image, bitDepth, subX, subY int) (y, u, v []uint16) {
	bounds := m.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	y = make([]uint16, w*h)
	cw := w >> subX
	ch := h >> subY
	if cw < 1 {
		cw = 1
	}
	if ch < 1 {
		ch = 1
	}
	u = make([]uint16, cw*ch)
	v = make([]uint16, cw*ch)

	uf := make([]int, w*h)
	vf := make([]int, w*h)

	readRGB := rgbReader16(m, bitDepth)
	maxV := (1 << uint(bitDepth)) - 1
	shift := uint(bitDepth - 8)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			R, G, B := readRGB(bounds.Min.X+c, bounds.Min.Y+r)
			yv := (66*R + 129*G + 25*B + (128 << shift)) >> 8
			uv := (-38*R - 74*G + 112*B + (128 << shift)) >> 8
			vv := (112*R - 94*G - 18*B + (128 << shift)) >> 8
			y[r*w+c] = clampU16(yv+(16<<shift), maxV)
			uf[r*w+c] = uv + (128 << shift)
			vf[r*w+c] = vv + (128 << shift)
		}
	}
	sx := 1 << subX
	sy := 1 << subY
	for cr := 0; cr < ch; cr++ {
		for cc := 0; cc < cw; cc++ {
			su, sv := 0, 0
			n := 0
			for dy := 0; dy < sy && cr*sy+dy < h; dy++ {
				for dx := 0; dx < sx && cc*sx+dx < w; dx++ {
					idx := (cr*sy+dy)*w + (cc*sx + dx)
					su += uf[idx]
					sv += vf[idx]
					n++
				}
			}
			if n > 0 {
				u[cr*cw+cc] = clampU16(su/n, maxV)
				v[cr*cw+cc] = clampU16(sv/n, maxV)
			}
		}
	}
	return y, u, v
}

// rgbReader16 returns a closure that reads RGB components scaled
// into the target bit-depth range.
func rgbReader16(m image.Image, bitDepth int) func(x, y int) (int, int, int) {
	shift := uint(16 - bitDepth)
	switch src := m.(type) {
	case *image.NRGBA64:
		return func(x, y int) (int, int, int) {
			i := (y-src.Rect.Min.Y)*src.Stride + (x-src.Rect.Min.X)*8
			r := (int(src.Pix[i])<<8 | int(src.Pix[i+1])) >> shift
			g := (int(src.Pix[i+2])<<8 | int(src.Pix[i+3])) >> shift
			b := (int(src.Pix[i+4])<<8 | int(src.Pix[i+5])) >> shift
			return r, g, b
		}
	case *image.RGBA64:
		return func(x, y int) (int, int, int) {
			i := (y-src.Rect.Min.Y)*src.Stride + (x-src.Rect.Min.X)*8
			r := (int(src.Pix[i])<<8 | int(src.Pix[i+1])) >> shift
			g := (int(src.Pix[i+2])<<8 | int(src.Pix[i+3])) >> shift
			b := (int(src.Pix[i+4])<<8 | int(src.Pix[i+5])) >> shift
			return r, g, b
		}
	case *image.RGBA:
		return func(x, y int) (int, int, int) {
			i := (y-src.Rect.Min.Y)*src.Stride + (x-src.Rect.Min.X)*4
			// 8-bit widened into HBD range.
			return int(src.Pix[i]) << uint(bitDepth-8),
				int(src.Pix[i+1]) << uint(bitDepth-8),
				int(src.Pix[i+2]) << uint(bitDepth-8)
		}
	case *image.NRGBA:
		return func(x, y int) (int, int, int) {
			i := (y-src.Rect.Min.Y)*src.Stride + (x-src.Rect.Min.X)*4
			return int(src.Pix[i]) << uint(bitDepth-8),
				int(src.Pix[i+1]) << uint(bitDepth-8),
				int(src.Pix[i+2]) << uint(bitDepth-8)
		}
	}
	return func(x, y int) (int, int, int) {
		rr, gg, bb, _ := m.At(x, y).RGBA()
		return int(rr) >> shift, int(gg) >> shift, int(bb) >> shift
	}
}

func clampU16(v, maxV int) uint16 {
	if v < 0 {
		return 0
	}
	if v > maxV {
		return uint16(maxV)
	}
	return uint16(v)
}

// imageToLuma extracts a w*h 8-bit luma plane for a monochrome
// encoder. The source's single channel is interpreted as studio-
// range luma (BT.601 Y = 16..235) rather than the full 0..255
// range to stay consistent with the color path; decoders use the
// sequence header's color_range flag to map samples back to RGB
// when needed.
func imageToLuma(m image.Image) []uint8 {
	bounds := m.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := make([]uint8, w*h)
	switch src := m.(type) {
	case *image.Gray:
		for y := 0; y < h; y++ {
			base := (bounds.Min.Y+y-src.Rect.Min.Y)*src.Stride + (bounds.Min.X - src.Rect.Min.X)
			for x := 0; x < w; x++ {
				// Compress full-range to studio-range: 0..255 → 16..235.
				v := int(src.Pix[base+x])
				out[y*w+x] = clampByte(16 + (v*219+128)>>8)
			}
		}
		return out
	case *image.Gray16:
		for y := 0; y < h; y++ {
			base := (bounds.Min.Y+y-src.Rect.Min.Y)*src.Stride + (bounds.Min.X-src.Rect.Min.X)*2
			for x := 0; x < w; x++ {
				// Gray16 pix is big-endian uint16. Downscale to 8-bit
				// then compress to studio range.
				hi := int(src.Pix[base+x*2])
				out[y*w+x] = clampByte(16 + (hi*219+128)>>8)
			}
		}
		return out
	}
	// Fallback: generic At.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			yc, _, _, _ := m.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			out[y*w+x] = clampByte(16 + (int(yc>>8)*219+128)>>8)
		}
	}
	return out
}

// wantAlpha returns true when the encoder should emit an alpha auxiliary
// item for this image. Explicit opts.Alpha forces it; otherwise we scan
// the input for a non-opaque pixel and opt in automatically.
func wantAlpha(m image.Image, opts *Options) bool {
	if opts != nil && opts.Alpha {
		return true
	}
	switch src := m.(type) {
	case *image.RGBA:
		for i := 3; i < len(src.Pix); i += 4 {
			if src.Pix[i] != 0xFF {
				return true
			}
		}
	case *image.NRGBA:
		for i := 3; i < len(src.Pix); i += 4 {
			if src.Pix[i] != 0xFF {
				return true
			}
		}
	case *image.RGBA64, *image.NRGBA64:
		// Fall through to generic At. Uncommon path.
		bounds := m.Bounds()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				_, _, _, a := m.At(x, y).RGBA()
				if a < 0xFFFF {
					return true
				}
			}
		}
	}
	return false
}

// imageToAlpha extracts a w*h alpha plane from an image as 8-bit full-
// range samples (0 = transparent, 255 = opaque). Non-alpha images return
// an all-opaque plane.
func imageToAlpha(m image.Image) []uint8 {
	bounds := m.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	out := make([]uint8, w*h)
	switch src := m.(type) {
	case *image.RGBA:
		for y := 0; y < h; y++ {
			row := (y-src.Rect.Min.Y+bounds.Min.Y-src.Rect.Min.Y)*src.Stride + (bounds.Min.X-src.Rect.Min.X)*4
			_ = row
			// Simpler: use bounds.Min directly.
			base := (bounds.Min.Y+y-src.Rect.Min.Y)*src.Stride + (bounds.Min.X-src.Rect.Min.X)*4
			for x := 0; x < w; x++ {
				out[y*w+x] = src.Pix[base+x*4+3]
			}
		}
		return out
	case *image.NRGBA:
		for y := 0; y < h; y++ {
			base := (bounds.Min.Y+y-src.Rect.Min.Y)*src.Stride + (bounds.Min.X-src.Rect.Min.X)*4
			for x := 0; x < w; x++ {
				out[y*w+x] = src.Pix[base+x*4+3]
			}
		}
		return out
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			_, _, _, a := m.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			out[y*w+x] = uint8(a >> 8)
		}
	}
	return out
}

// findIprp returns the iprp box from a parsed Container, or nil.
func findIprp(ct *isobmff.Container) *isobmff.Iprp {
	if ct.Meta == nil {
		return nil
	}
	for _, ch := range ct.Meta.Children {
		if p, ok := ch.(*isobmff.Iprp); ok {
			return p
		}
	}
	return nil
}

// DecodeConfig reads the AVIF container from r and returns the image
// dimensions and a best-effort color model. The full AV1 bitstream is not
// decoded.
func DecodeConfig(r io.Reader) (image.Config, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return image.Config{}, err
	}
	ct, err := isobmff.ParseContainer(data)
	if err != nil {
		return image.Config{}, err
	}
	if !ct.Ftyp.HasBrand("avif") && !ct.Ftyp.HasBrand("avis") {
		return image.Config{}, fmt.Errorf("goavif: ftyp has no avif/avis brand")
	}
	primaryID := ct.PrimaryItemID()
	if primaryID == 0 {
		return image.Config{}, fmt.Errorf("goavif: no primary item")
	}
	w, h, depth, channels, err := primaryDims(ct, primaryID)
	if err != nil {
		return image.Config{}, err
	}
	return image.Config{
		ColorModel: modelFor(depth, channels),
		Width:      int(w),
		Height:     int(h),
	}, nil
}

// Encode writes m to w as an AVIF image using opts.
//
// When opts.TargetBytes > 0, Encode runs a Q-bisection rate-control
// loop: it encodes the image at a series of Q values and returns the
// smallest-distortion bitstream whose size lands within ±10% of the
// target, falling back to the closest-below result when the exact
// target cannot be met. Otherwise a single-pass encode is performed
// at opts.Quality (default 50).
func Encode(w io.Writer, m image.Image, opts *Options) error {
	if opts != nil && opts.TargetBytes > 0 && !opts.Lossless {
		return encodeTargetSize(w, m, opts)
	}
	return encodeFixedQ(w, m, opts)
}

// encodeFixedQ performs a single-pass encode at opts.Quality.
func encodeFixedQ(w io.Writer, m image.Image, opts *Options) error {
	if m == nil {
		return fmt.Errorf("goavif: nil image")
	}
	bounds := m.Bounds()
	origW, origH := bounds.Dx(), bounds.Dy()
	width, height := origW, origH
	if width < 4 || height < 4 {
		return fmt.Errorf("goavif: image too small (%dx%d)", width, height)
	}
	// The current encoder emits full-SB partitions only; input that
	// isn't a multiple of 64 is pre-padded here to the next multiple
	// by repeating the bottom-right edge pixels. ispe stores the
	// original dimensions so Decode crops back on the way out.
	if width%64 != 0 || height%64 != 0 {
		m = padToMultiple(m, 64)
		bounds = m.Bounds()
		width, height = bounds.Dx(), bounds.Dy()
	}

	// Build the three AV1 OBUs: sequence header, frame header,
	// tile group. Current encoder is first-pass: PARTITION_NONE +
	// DC_PRED + skip=1 everywhere, so no residual is coded and
	// the decoder produces a constant-chroma mid-grey image.
	baseQ := uint8(32)
	if opts != nil && opts.Quality > 0 && opts.Quality <= 100 {
		// Map quality 0..100 to baseQ 255..0 roughly (not tuned).
		baseQ = uint8(255 - (opts.Quality*255)/100)
	}

	// Monochrome inputs (image.Gray / Gray16) are encoded with a
	// single-plane AV1 sequence header so the container declares
	// monochrome=1 and pixi has one channel. Saves both the chroma
	// bitstream and the chroma container metadata.
	monochrome := isGrayscale(m)

	// Detect HBD inputs (NRGBA64 / RGBA64 / Gray16, or explicit
	// opts.BitDepth > 8).
	bitDepth := hbdBitDepth(m, opts)
	hbd := bitDepth > 8

	// Pick chroma subsampling. For a *image.YCbCr input, honor its
	// native subsample ratio. For other input types, accept
	// opts.ChromaSubsampling and default to 4:2:0.
	subX, subY := pickSubsampling(m, opts)

	var seqPayload, framePayload []byte
	switch {
	case monochrome:
		if hbd {
			seqPayload = obu.WriteMonoSequenceHeaderHBD(width, height, bitDepth)
		} else {
			seqPayload = obu.WriteMonoSequenceHeader(width, height)
		}
		framePayload = obu.WriteMonoKeyFrameHeader(width, height, baseQ)
	default:
		seqPayload = obu.WriteSequenceHeaderFull(width, height, obu.SeqWriteOpts{
			BitDepth:     bitDepth,
			SubsamplingX: subX,
			SubsamplingY: subY,
		})
		framePayload = obu.WriteKeyFrameHeader(width, height, baseQ)
	}
	sh, err := obu.ParseSequenceHeader(seqPayload)
	if err != nil {
		return err
	}
	fh, _, err := obu.ParseFrameHeaderBytes(framePayload, sh, nil)
	if err != nil {
		return err
	}

	// HBD path: extract uint16 planes and run the HBD encoder tile.
	if hbd {
		var lumaY16, chromaU16, chromaV16 []uint16
		if monochrome {
			lumaY16 = imageToLuma16(m, bitDepth)
		} else {
			lumaY16, chromaU16, chromaV16 = imageToYUV16(m, bitDepth, subX, subY)
		}
		tilePayload, err := encoder.WriteIntraOnlyTile16(width, height, fh, sh, lumaY16, chromaU16, chromaV16)
		if err != nil {
			return err
		}
		return finishEncode(w, m, opts, width, height, origW, origH, baseQ, bitDepth, sh,
			seqPayload, framePayload, tilePayload)
	}

	// 8-bit path.
	var lumaY, chromaU, chromaV []uint8
	if monochrome {
		lumaY = imageToLuma(m)
	} else {
		lumaY, chromaU, chromaV = imageToYUV(m, subX, subY)
	}
	tilePayload, err := encoder.WriteIntraOnlyTile(width, height, fh, sh, lumaY, chromaU, chromaV)
	if err != nil {
		return err
	}
	return finishEncode(w, m, opts, width, height, origW, origH, baseQ, 8, sh,
		seqPayload, framePayload, tilePayload)
}

// encodeTargetSize drives a Q-bisection loop to land the encoded
// bitstream within ±10% of opts.TargetBytes. Starts with quality=50
// and adjusts up (smaller file) or down (larger file) until the
// window is hit or the quality range is exhausted.
func encodeTargetSize(w io.Writer, m image.Image, opts *Options) error {
	target := opts.TargetBytes
	tolerance := target / 10 // ±10%
	if tolerance < 256 {
		tolerance = 256
	}

	var bestBuf []byte
	bestErr := -1
	loQ := 1
	hiQ := 100
	q := 50
	if opts.Quality > 0 && opts.Quality <= 100 {
		q = opts.Quality
	}

	maxIters := 8
	for iter := 0; iter < maxIters && loQ <= hiQ; iter++ {
		var buf bytes.Buffer
		tryOpts := *opts
		tryOpts.Quality = q
		tryOpts.TargetBytes = 0 // avoid recursion
		if err := encodeFixedQ(&buf, m, &tryOpts); err != nil {
			return err
		}
		size := buf.Len()
		diff := size - target
		absDiff := diff
		if absDiff < 0 {
			absDiff = -absDiff
		}
		// Keep the closest-to-target result so far.
		if bestErr < 0 || absDiff < bestErr {
			bestErr = absDiff
			bestBuf = buf.Bytes()
		}
		if absDiff <= tolerance {
			// Within target window — prefer higher quality when tied.
			_, err := w.Write(buf.Bytes())
			return err
		}
		if size > target {
			// Too big — lower quality.
			hiQ = q - 1
		} else {
			// Too small — raise quality.
			loQ = q + 1
		}
		q = (loQ + hiQ) / 2
	}

	if bestBuf != nil {
		_, err := w.Write(bestBuf)
		return err
	}
	return fmt.Errorf("goavif: rate control failed to produce a bitstream")
}

// pickSubsampling selects (subX, subY) for the encoder output:
//   - YCbCr native subsampling matches its SubsampleRatio.
//   - opts.ChromaSubsampling overrides when set (for any input type).
//   - Default is 4:2:0 (1, 1) for other inputs.
func pickSubsampling(m image.Image, opts *Options) (subX, subY int) {
	if opts != nil {
		switch opts.ChromaSubsampling {
		case Chroma420:
			return 1, 1
		case Chroma422:
			return 1, 0
		case Chroma444:
			return 0, 0
		}
	}
	if yc, ok := m.(*image.YCbCr); ok {
		switch yc.SubsampleRatio {
		case image.YCbCrSubsampleRatio420:
			return 1, 1
		case image.YCbCrSubsampleRatio422:
			return 1, 0
		case image.YCbCrSubsampleRatio444:
			return 0, 0
		}
	}
	return 1, 1
}

// EncodeGrid writes a grid-structured AVIF to w. The primary item
// becomes a "grid"-type derived image that references rows × cols
// tiles in raster order via a dimg iref. All tiles must share
// dimensions; the output image is the tile mosaic cropped to
// outputWidth × outputHeight.
//
// This is the format Apple uses for iPhone HEIC / AVIF photos:
// a large sensor image split into smaller AV1-coded tiles. The
// decoder side (goavif.Decode) already composes grid-type primaries.
func EncodeGrid(w io.Writer, tiles []image.Image, rows, cols int, outputWidth, outputHeight int, opts *Options) error {
	if len(tiles) == 0 {
		return fmt.Errorf("goavif: EncodeGrid: no tiles")
	}
	if rows*cols != len(tiles) {
		return fmt.Errorf("goavif: EncodeGrid: rows×cols (%d) != tile count (%d)", rows*cols, len(tiles))
	}
	first := tiles[0]
	if first == nil {
		return fmt.Errorf("goavif: EncodeGrid: tile 0 is nil")
	}
	tw, th := first.Bounds().Dx(), first.Bounds().Dy()
	for i, t := range tiles {
		if t == nil {
			return fmt.Errorf("goavif: EncodeGrid: tile %d is nil", i)
		}
		if t.Bounds().Dx() != tw || t.Bounds().Dy() != th {
			return fmt.Errorf("goavif: EncodeGrid: tile %d size %v differs from tile 0 %dx%d",
				i, t.Bounds(), tw, th)
		}
	}
	if tw%64 != 0 || th%64 != 0 {
		return fmt.Errorf("goavif: EncodeGrid: tile dims %dx%d must be multiples of 64", tw, th)
	}
	if outputWidth > rows*0+cols*tw || outputHeight > rows*th {
		// outputWidth may be ≤ cols*tw (the encoder crops on decode).
	}

	baseQ := uint8(32)
	if opts != nil && opts.Quality > 0 && opts.Quality <= 100 {
		baseQ = uint8(255 - (opts.Quality*255)/100)
	}
	bitDepth := hbdBitDepth(first, opts)
	hbd := bitDepth > 8
	subX, subY := pickSubsampling(first, opts)

	// Shared sequence header across every tile.
	var seqPayload []byte
	if hbd {
		seqPayload = obu.WriteSequenceHeaderHBD(tw, th, bitDepth)
	} else if subX != 1 || subY != 1 {
		seqPayload = obu.WriteSequenceHeaderFull(tw, th, obu.SeqWriteOpts{
			BitDepth: bitDepth, SubsamplingX: subX, SubsamplingY: subY,
		})
	} else {
		seqPayload = obu.WriteSequenceHeader(tw, th)
	}
	sh, err := obu.ParseSequenceHeader(seqPayload)
	if err != nil {
		return err
	}
	framePayload := obu.WriteKeyFrameHeader(tw, th, baseQ)
	fh, _, err := obu.ParseFrameHeaderBytes(framePayload, sh, nil)
	if err != nil {
		return err
	}
	seqOBU := obu.WrapOBU(1, seqPayload)

	// Encode every tile as a standalone AV1 item.
	gridTiles := make([]isobmff.GridTile, len(tiles))
	for i, t := range tiles {
		tilePayload, err := encodeFrameTile(tw, th, fh, sh, t, bitDepth, hbd, false /* not monochrome */)
		if err != nil {
			return fmt.Errorf("goavif: EncodeGrid: tile %d: %w", i, err)
		}
		frameBytes := append(append([]byte(nil), framePayload...), tilePayload...)
		frameOBU := obu.WrapOBU(6, frameBytes)
		itemBytes := append(append([]byte(nil), seqOBU...), frameOBU...)
		gridTiles[i] = isobmff.GridTile{AV1Bitstream: itemBytes}
	}

	container, err := isobmff.BuildGrid(isobmff.GridImage{
		OutputWidth:        uint32(outputWidth),
		OutputHeight:       uint32(outputHeight),
		TileWidth:          uint32(tw),
		TileHeight:         uint32(th),
		Rows:               rows,
		Columns:            cols,
		BitDepth:           sh.Color.BitDepth,
		ChromaSubsamplingX: sh.Color.SubsamplingX,
		ChromaSubsamplingY: sh.Color.SubsamplingY,
		ConfigOBUs:         seqOBU,
		Tiles:              gridTiles,
	})
	if err != nil {
		return fmt.Errorf("goavif: EncodeGrid: build: %w", err)
	}
	_, err = container.WriteTo(w)
	return err
}

// finishEncode handles the container assembly shared between the
// 8-bit and HBD encode paths: OBU wrapping, optional alpha aux
// item, BuildStillImage, and serialization to the caller's io.Writer.
//
// width/height are the coded frame dimensions (post-padding);
// ispeW/ispeH are the caller-visible dimensions that end up in the
// ispe box so Decode can crop back to the requested size.
func finishEncode(w io.Writer, m image.Image, opts *Options,
	width, height, ispeW, ispeH int, baseQ uint8, bitDepth int, sh *obu.SequenceHeader,
	seqPayload, framePayload, tilePayload []byte) error {
	seqOBU := obu.WrapOBU(1 /* OBU_SEQUENCE_HEADER */, seqPayload)
	frameBytes := append(append([]byte(nil), framePayload...), tilePayload...)
	frameOBU := obu.WrapOBU(6 /* OBU_FRAME */, frameBytes)

	var alphaSeqOBU, alphaFrameOBU []byte
	if wantAlpha(m, opts) {
		// For a 10-bit primary the alpha is also 10-bit (matches the
		// primary's pixi/av1C declaration); otherwise stay 8-bit.
		alphaPlane := imageToAlpha(m)
		if bitDepth > 8 {
			widened := make([]uint16, len(alphaPlane))
			shift := uint(bitDepth - 8)
			for i, v := range alphaPlane {
				widened[i] = uint16(v) << shift
			}
			alphaSeqPayload := obu.WriteMonoSequenceHeaderHBD(width, height, bitDepth)
			alphaFramePayload := obu.WriteMonoKeyFrameHeader(width, height, baseQ)
			alphaSh, err := obu.ParseSequenceHeader(alphaSeqPayload)
			if err != nil {
				return err
			}
			alphaFh, _, err := obu.ParseFrameHeaderBytes(alphaFramePayload, alphaSh, nil)
			if err != nil {
				return err
			}
			alphaTile, err := encoder.WriteIntraOnlyTile16(width, height, alphaFh, alphaSh, widened, nil, nil)
			if err != nil {
				return err
			}
			alphaSeqOBU = obu.WrapOBU(1, alphaSeqPayload)
			alphaFrameBytes := append(append([]byte(nil), alphaFramePayload...), alphaTile...)
			alphaFrameOBU = obu.WrapOBU(6, alphaFrameBytes)
		} else {
			alphaSeqPayload := obu.WriteMonoSequenceHeader(width, height)
			alphaFramePayload := obu.WriteMonoKeyFrameHeader(width, height, baseQ)
			alphaSh, err := obu.ParseSequenceHeader(alphaSeqPayload)
			if err != nil {
				return err
			}
			alphaFh, _, err := obu.ParseFrameHeaderBytes(alphaFramePayload, alphaSh, nil)
			if err != nil {
				return err
			}
			alphaTile, err := encoder.WriteIntraOnlyTile(width, height, alphaFh, alphaSh, alphaPlane, nil, nil)
			if err != nil {
				return err
			}
			alphaSeqOBU = obu.WrapOBU(1, alphaSeqPayload)
			alphaFrameBytes := append(append([]byte(nil), alphaFramePayload...), alphaTile...)
			alphaFrameOBU = obu.WrapOBU(6, alphaFrameBytes)
		}
	}

	container, err := isobmff.BuildStillImage(isobmff.StillImage{
		Width:              uint32(ispeW),
		Height:             uint32(ispeH),
		BitDepth:           sh.Color.BitDepth,
		Monochrome:         sh.Color.Monochrome,
		ChromaSubsamplingX: sh.Color.SubsamplingX,
		ChromaSubsamplingY: sh.Color.SubsamplingY,
		ConfigOBUs:         seqOBU,
		AV1Bitstream:       frameOBU,
		AlphaBitstream:     alphaFrameOBU,
		AlphaConfigOBUs:    alphaSeqOBU,
		AlphaBitDepth:      sh.Color.BitDepth,
	})
	if err != nil {
		return fmt.Errorf("goavif: build container: %w", err)
	}
	outBuf, err := container.Encode()
	if err != nil {
		return fmt.Errorf("goavif: encode container: %w", err)
	}
	_, err = w.Write(outBuf)
	return err
}

// primaryDims returns the width, height, bit depth per component, and channel
// count for the given primary item by walking ipco/ipma.
func primaryDims(ct *isobmff.Container, itemID uint32) (w, h uint32, depth uint8, channels int, err error) {
	meta := ct.Meta
	if meta == nil {
		return 0, 0, 0, 0, fmt.Errorf("goavif: no meta box")
	}
	var iprp *isobmff.Iprp
	for _, ch := range meta.Children {
		if p, ok := ch.(*isobmff.Iprp); ok {
			iprp = p
			break
		}
	}
	if iprp == nil {
		return 0, 0, 0, 0, fmt.Errorf("goavif: no iprp box")
	}
	var assoc []isobmff.IpmaAssoc
	for _, m := range iprp.Ipma {
		for _, e := range m.Entries {
			if e.ItemID == itemID {
				assoc = e.Associations
				break
			}
		}
		if assoc != nil {
			break
		}
	}
	if assoc == nil {
		return 0, 0, 0, 0, fmt.Errorf("goavif: no property associations for item %d", itemID)
	}
	channels = 3
	for _, a := range assoc {
		if a.PropertyIndex == 0 || int(a.PropertyIndex) > len(iprp.Ipco.Properties) {
			continue
		}
		prop := iprp.Ipco.Properties[a.PropertyIndex-1]
		switch p := prop.(type) {
		case *isobmff.Ispe:
			w, h = p.Width, p.Height
		case *isobmff.Pixi:
			if len(p.ChannelBits) > 0 {
				depth = p.ChannelBits[0]
				channels = len(p.ChannelBits)
			}
		case *isobmff.Av1C:
			if depth == 0 {
				switch {
				case p.TwelveBit == 1:
					depth = 12
				case p.HighBitdepth == 1:
					depth = 10
				default:
					depth = 8
				}
			}
			if p.Monochrome == 1 {
				channels = 1
			}
		}
	}
	if w == 0 || h == 0 {
		return 0, 0, 0, 0, fmt.Errorf("goavif: ispe absent or zero")
	}
	if depth == 0 {
		depth = 8
	}
	return
}

// modelFor picks the best-matching image.ColorModel for the given bit depth
// and channel count.
func modelFor(depth uint8, channels int) color.Model {
	switch {
	case channels == 1 && depth <= 8:
		return color.GrayModel
	case channels == 1:
		return color.Gray16Model
	case depth <= 8:
		return color.NRGBAModel
	default:
		return color.NRGBA64Model
	}
}

// rawMagic values used for [image.RegisterFormat]. The '?' bytes wildcard any
// byte value; ftyp size varies between files so we can't pin the first four.
const (
	magicAvif = "????ftypavif"
	magicAvis = "????ftypavis"
)

func init() {
	image.RegisterFormat("avif", magicAvif, Decode, DecodeConfig)
	image.RegisterFormat("avif", magicAvis, Decode, DecodeConfig)
}

// BytesDecoder lets callers who already have the container bytes skip a copy.
// It is a thin convenience over [DecodeConfig].
func BytesDecodeConfig(data []byte) (image.Config, error) {
	return DecodeConfig(bytes.NewReader(data))
}
