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
	if alphaID := findAlphaItemID(ct, primaryID); alphaID != 0 {
		alpha, err := decodeAlphaFrame(ct, alphaID)
		if err != nil {
			return nil, fmt.Errorf("goavif: alpha decode: %w", err)
		}
		if frame.BitDepth > 8 {
			return compositeNRGBA64(frame, alpha)
		}
		return compositeNRGBA(frame, alpha)
	}

	return frameToImage(frame)
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
	if f.Subsampling.X != 1 || f.Subsampling.Y != 1 {
		return nil, fmt.Errorf("goavif: HBD non-4:2:0 subsampling %d/%d not yet implemented",
			f.Subsampling.X, f.Subsampling.Y)
	}
	img := image.NewRGBA64(image.Rect(0, 0, f.Width, f.Height))
	// Choose a matrix from the sequence header's CICP; fall back to
	// BT.709 when unspecified, matching common AVIF encoder defaults.
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
	colorspace.ConvertPlanar420_16(img.Pix, f.Y16, f.U16, f.V16, f.Width, f.Height, mc, rng, f.BitDepth)
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

// Encode writes m to w as an AVIF image using opts. Not implemented yet;
// returns [ErrUnsupported].
func Encode(w io.Writer, m image.Image, opts *Options) error {
	if m == nil {
		return fmt.Errorf("goavif: nil image")
	}
	bounds := m.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if width < 4 || height < 4 {
		return fmt.Errorf("goavif: image too small (%dx%d)", width, height)
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

	seqPayload := obu.WriteSequenceHeader(width, height)
	framePayload := obu.WriteKeyFrameHeader(width, height, baseQ)
	sh, err := obu.ParseSequenceHeader(seqPayload)
	if err != nil {
		return err
	}
	fh, _, err := obu.ParseFrameHeaderBytes(framePayload, sh, nil)
	if err != nil {
		return err
	}
	tilePayload, err := encoder.WriteIntraOnlyTile(width, height, fh, sh)
	if err != nil {
		return err
	}

	// Assemble AV1 bitstream: seq OBU + frame OBU.
	// Concat frame header and tile payload inside one FRAME OBU.
	seqOBU := obu.WrapOBU(1 /* OBU_SEQUENCE_HEADER */, seqPayload)
	frameBytes := append(append([]byte(nil), framePayload...), tilePayload...)
	frameOBU := obu.WrapOBU(6 /* OBU_FRAME */, frameBytes)

	// Wrap in ISOBMFF/AVIF container using BuildStillImage.
	container, err := isobmff.BuildStillImage(isobmff.StillImage{
		Width:              uint32(width),
		Height:             uint32(height),
		BitDepth:           sh.Color.BitDepth,
		Monochrome:         sh.Color.Monochrome,
		ChromaSubsamplingX: sh.Color.SubsamplingX,
		ChromaSubsamplingY: sh.Color.SubsamplingY,
		ConfigOBUs:         seqOBU,
		AV1Bitstream:       frameOBU,
	})
	if err != nil {
		return fmt.Errorf("goavif: build container: %w", err)
	}
	var outBuf []byte
	outBuf, err = container.Encode()
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
