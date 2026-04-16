package goavif

import (
	"fmt"
	"image"
	"strings"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/colorspace"
	"github.com/KarpelesLab/goavif/isobmff"
)

// alphaURN is the auxC AuxType URN that identifies the alpha auxiliary
// image in AVIF (per ITU-T H.273 / CICP systems table).
const alphaURN = "urn:mpeg:mpegB:cicp:systems:auxiliary:alpha"

// findAlphaItemID returns the item ID of the alpha auxiliary image for
// the given primary item, or 0 when no alpha is attached.
//
// The search follows two signals per HEIF / AVIF §6:
//  1. an auxl iref whose FromID is the alpha candidate and ToIDs
//     contains primaryID.
//  2. the candidate item's associated auxC property carries the alpha URN.
func findAlphaItemID(ct *isobmff.Container, primaryID uint32) uint32 {
	if ct == nil || ct.Meta == nil {
		return 0
	}
	var iref *isobmff.Iref
	for _, ch := range ct.Meta.Children {
		if r, ok := ch.(*isobmff.Iref); ok {
			iref = r
			break
		}
	}
	if iref == nil {
		return 0
	}
	var candidate uint32
	for _, e := range iref.Entries {
		if e.Type != isobmff.TypeAuxl {
			continue
		}
		for _, to := range e.ToIDs {
			if to == primaryID {
				candidate = e.FromID
				break
			}
		}
		if candidate != 0 {
			break
		}
	}
	if candidate == 0 {
		return 0
	}
	// Verify the candidate has an auxC pointing at the alpha URN.
	iprp := findIprp(ct)
	if iprp == nil {
		return 0
	}
	for _, m := range iprp.Ipma {
		for _, e := range m.Entries {
			if e.ItemID != candidate {
				continue
			}
			for _, a := range e.Associations {
				if a.PropertyIndex == 0 || int(a.PropertyIndex) > len(iprp.Ipco.Properties) {
					continue
				}
				if aux, ok := iprp.Ipco.Properties[a.PropertyIndex-1].(*isobmff.AuxC); ok {
					if strings.HasPrefix(aux.AuxType, alphaURN) {
						return candidate
					}
				}
			}
		}
	}
	return 0
}

// decodeAlphaFrame decodes the auxiliary alpha AV1 item and returns its
// reconstructed plane. The alpha stream is a monochrome single-plane AV1
// encoding of the alpha channel.
func decodeAlphaFrame(ct *isobmff.Container, alphaID uint32) (*decoder.Frame, error) {
	seq, err := extractSequenceHeader(ct, alphaID)
	if err != nil {
		return nil, fmt.Errorf("alpha seq header: %w", err)
	}
	itemBytes, err := ct.ItemData(alphaID)
	if err != nil {
		return nil, fmt.Errorf("alpha item data: %w", err)
	}
	return decoder.Decode(itemBytes, seq)
}

// compositeNRGBA converts an 8-bit decoded color frame plus a decoded
// alpha frame into an image.NRGBA. The alpha plane's dimensions must
// match the color frame's; mismatched sizes return an error.
func compositeNRGBA(color, alpha *decoder.Frame) (image.Image, error) {
	if color.Width != alpha.Width || color.Height != alpha.Height {
		return nil, fmt.Errorf("goavif: alpha size %dx%d != color %dx%d",
			alpha.Width, alpha.Height, color.Width, color.Height)
	}
	if alpha.BitDepth != 8 {
		return nil, fmt.Errorf("goavif: 8-bit alpha composite requires 8-bit alpha plane, got %d", alpha.BitDepth)
	}
	img := image.NewNRGBA(image.Rect(0, 0, color.Width, color.Height))
	// Convert color YUV to RGB, then splice in alpha per-pixel.
	rgb := make([]byte, color.Width*color.Height*4)
	mc := colorspace.MCBT709
	rng := colorspace.Studio
	if color.Seq != nil {
		if color.Seq.Color.ColorRange {
			rng = colorspace.Full
		}
		if cicp := colorspace.MatrixCoefficients(color.Seq.Color.MatrixCoefficients); cicp != colorspace.MCUnspecified {
			mc = cicp
		}
	}
	if color.Monochrome {
		// Luma-only: replicate to RGB then splice alpha.
		for i, y := range color.Y {
			rgb[i*4+0] = y
			rgb[i*4+1] = y
			rgb[i*4+2] = y
		}
	} else {
		colorspace.ConvertPlanar420(rgb, color.Y, color.U, color.V, color.Width, color.Height, mc, rng)
	}
	for i := 0; i < color.Width*color.Height; i++ {
		img.Pix[i*4+0] = rgb[i*4+0]
		img.Pix[i*4+1] = rgb[i*4+1]
		img.Pix[i*4+2] = rgb[i*4+2]
		img.Pix[i*4+3] = alpha.Y[i]
	}
	return img, nil
}

// compositeNRGBA64 is the 10/12-bit counterpart of compositeNRGBA.
// Both color and alpha are assumed to carry uint16 Y plane data;
// non-Y16 alpha falls back to upshifting the uint8 plane.
func compositeNRGBA64(color, alpha *decoder.Frame) (image.Image, error) {
	if color.Width != alpha.Width || color.Height != alpha.Height {
		return nil, fmt.Errorf("goavif: alpha size %dx%d != color %dx%d",
			alpha.Width, alpha.Height, color.Width, color.Height)
	}
	img := image.NewNRGBA64(image.Rect(0, 0, color.Width, color.Height))
	mc := colorspace.MCBT709
	rng := colorspace.Studio
	if color.Seq != nil {
		if color.Seq.Color.ColorRange {
			rng = colorspace.Full
		}
		if cicp := colorspace.MatrixCoefficients(color.Seq.Color.MatrixCoefficients); cicp != colorspace.MCUnspecified {
			mc = cicp
		}
	}
	rgba := make([]byte, color.Width*color.Height*8)
	if color.Monochrome {
		shift := uint(16 - color.BitDepth)
		for i, y := range color.Y16 {
			s := y << shift
			rgba[i*8+0] = uint8(s >> 8)
			rgba[i*8+1] = uint8(s & 0xFF)
			rgba[i*8+2] = uint8(s >> 8)
			rgba[i*8+3] = uint8(s & 0xFF)
			rgba[i*8+4] = uint8(s >> 8)
			rgba[i*8+5] = uint8(s & 0xFF)
			rgba[i*8+6] = 0xFF
			rgba[i*8+7] = 0xFF
		}
	} else {
		colorspace.ConvertPlanar420_16(rgba, color.Y16, color.U16, color.V16,
			color.Width, color.Height, mc, rng, color.BitDepth)
	}
	aShift := uint(16 - alpha.BitDepth)
	for i := 0; i < color.Width*color.Height; i++ {
		img.Pix[i*8+0] = rgba[i*8+0]
		img.Pix[i*8+1] = rgba[i*8+1]
		img.Pix[i*8+2] = rgba[i*8+2]
		img.Pix[i*8+3] = rgba[i*8+3]
		img.Pix[i*8+4] = rgba[i*8+4]
		img.Pix[i*8+5] = rgba[i*8+5]
		var a uint16
		if alpha.BitDepth > 8 {
			a = alpha.Y16[i] << aShift
		} else {
			a = uint16(alpha.Y[i]) * 257 // broadcast 8-bit to 16-bit
		}
		img.Pix[i*8+6] = uint8(a >> 8)
		img.Pix[i*8+7] = uint8(a & 0xFF)
	}
	return img, nil
}
