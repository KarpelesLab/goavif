package goavif

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"

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
// Not implemented yet; returns [ErrUnsupported]. The container path works
// today — see [DecodeConfig] — but the AV1 decoder is landing in a later
// milestone (Phase 2 of the implementation plan).
func Decode(r io.Reader) (image.Image, error) {
	if _, err := DecodeConfig(r); err != nil {
		return nil, err
	}
	return nil, fmt.Errorf("%w: AV1 decoder pending", ErrUnsupported)
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
	return fmt.Errorf("%w: AV1 encoder pending", ErrUnsupported)
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
