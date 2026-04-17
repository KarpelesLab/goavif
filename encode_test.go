package goavif

import (
	"bytes"
	"image"
	"image/color"
	"testing"

	"github.com/KarpelesLab/goavif/av1/obu"
	"github.com/KarpelesLab/goavif/isobmff"
)

func TestEncodeProducesValidContainer(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	var buf bytes.Buffer
	if err := Encode(&buf, src, nil); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	if buf.Len() == 0 {
		t.Fatal("Encode produced empty output")
	}
	// Parse the container and confirm it has the expected AVIF brand.
	ct, err := isobmff.ParseContainer(buf.Bytes())
	if err != nil {
		t.Fatalf("ParseContainer: %v", err)
	}
	if !ct.Ftyp.HasBrand("avif") {
		t.Fatalf("output lacks 'avif' brand; got %v", ct.Ftyp.CompatibleBrands)
	}
	if ct.PrimaryItemID() == 0 {
		t.Fatal("no primary item id in encoded output")
	}
}

func TestEncodeRejectsTinyImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	var buf bytes.Buffer
	if err := Encode(&buf, src, nil); err == nil {
		t.Fatal("expected error for image < 4x4")
	}
}

func TestEncodeNilImage(t *testing.T) {
	var buf bytes.Buffer
	if err := Encode(&buf, nil, nil); err == nil {
		t.Fatal("expected error for nil image")
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	for _, sz := range []int{64, 128, 256} {
		t.Run(dimName(sz), func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, sz, sz))
			var buf bytes.Buffer
			if err := Encode(&buf, src, nil); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			img, err := Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("Decode %dx%d: %v", sz, sz, err)
			}
			if img == nil {
				t.Fatal("Decode returned nil image")
			}
			if img.Bounds().Dx() != sz || img.Bounds().Dy() != sz {
				t.Fatalf("decoded size %v, want %dx%d", img.Bounds(), sz, sz)
			}
		})
	}
}

// TestEncodeQualityAffectsBaseQIndex verifies that the Quality option
// changes the encoded frame's base_q_index. High quality gives a low
// base_q; low quality gives a high base_q.
func TestEncodeQualityAffectsBaseQIndex(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))

	extractBaseQ := func(opts *Options) uint8 {
		var buf bytes.Buffer
		if err := Encode(&buf, src, opts); err != nil {
			t.Fatalf("Encode: %v", err)
		}
		ct, err := isobmff.ParseContainer(buf.Bytes())
		if err != nil {
			t.Fatalf("ParseContainer: %v", err)
		}
		itemData, err := ct.ItemData(ct.PrimaryItemID())
		if err != nil {
			t.Fatalf("ItemData: %v", err)
		}
		obus, err := obu.Split(itemData)
		if err != nil {
			t.Fatalf("OBU.Split: %v", err)
		}
		for _, u := range obus {
			if u.Header.Type != obu.TypeFrame {
				continue
			}
			// Extract the sequence header from the av1C associated with
			// the PRIMARY item. There may be additional av1Cs for alpha
			// or other auxiliaries; picking any would give a mismatched
			// sh for the primary's frame header.
			primaryID := ct.PrimaryItemID()
			var seqBytes []byte
			for _, c := range ct.Meta.Children {
				iprp, ok := c.(*isobmff.Iprp)
				if !ok {
					continue
				}
				for _, m := range iprp.Ipma {
					for _, e := range m.Entries {
						if e.ItemID != primaryID {
							continue
						}
						for _, a := range e.Associations {
							if a.PropertyIndex == 0 || int(a.PropertyIndex) > len(iprp.Ipco.Properties) {
								continue
							}
							if av1c, ok := iprp.Ipco.Properties[a.PropertyIndex-1].(*isobmff.Av1C); ok {
								seqBytes = av1c.ConfigOBUs
							}
						}
					}
				}
			}
			seqOBUs, err := obu.Split(seqBytes)
			if err != nil {
				t.Fatalf("seq split: %v", err)
			}
			sh, err := obu.ParseSequenceHeader(seqOBUs[0].Payload)
			if err != nil {
				t.Fatalf("ParseSequenceHeader: %v", err)
			}
			fh, _, err := obu.ParseFrameHeaderBytes(u.Payload, sh, nil)
			if err != nil {
				t.Fatalf("ParseFrameHeaderBytes: %v", err)
			}
			return fh.Quant.BaseQIndex
		}
		t.Fatal("no FRAME OBU found")
		return 0
	}

	hiQ := extractBaseQ(&Options{Quality: 90})
	loQ := extractBaseQ(&Options{Quality: 10})
	if hiQ >= loQ {
		t.Fatalf("high-quality base_q (%d) should be lower than low-quality (%d)", hiQ, loQ)
	}
}

func TestEncodeDecodeNonBlackImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	// Fill with bright red.
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i+0] = 255 // R
		src.Pix[i+1] = 0   // G
		src.Pix[i+2] = 0   // B
		src.Pix[i+3] = 255 // A
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 50}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if img.Bounds().Dx() != 64 || img.Bounds().Dy() != 64 {
		t.Fatalf("decoded size %v, want 64x64", img.Bounds())
	}
	t.Logf("encoded size: %d bytes", buf.Len())
}

func TestEncodeDecodeSolidColorApproximatelyCorrect(t *testing.T) {
	// Encode a solid white (255, 255, 255) image at quality=95
	// (low quantizer) so the DC residual survives quantization.
	// BT.601 Y for white ≈ 235 (studio range). Residual = 235-128 = 107.
	// At Q=95 (baseQ≈13), DC_q≈16, so |107*1.414/16|≈9 → non-zero.
	src := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i+0] = 255
		src.Pix[i+1] = 255
		src.Pix[i+2] = 255
		src.Pix[i+3] = 255
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 95}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	switch v := img.(type) {
	case *image.YCbCr:
		yVal := v.Y[32*v.YStride+32]
		t.Logf("center Y = %d (expected ~235, DC_PRED base 128)", yVal)
		// With full 2D coefficient encoding, Y should be close to 235.
		if yVal < 220 {
			t.Fatalf("center Y = %d — expected close to 235 for white at quality=95", yVal)
		}
	default:
		t.Logf("decoded image type: %T", img)
	}
}

// TestEncodeDecodeGradientPreservesVariation verifies the encoder emits
// AC coefficients that let the decoder reproduce a non-constant image.
// With DC-only encoding every block would collapse to a single value;
// with full AC coefficients the decoded gradient should keep its
// horizontal variation.
func TestEncodeDecodeGradientPreservesVariation(t *testing.T) {
	const dim = 64
	src := image.NewRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			// Horizontal gradient 0..255.
			g := uint8(x * 255 / (dim - 1))
			i := (y*dim + x) * 4
			src.Pix[i+0] = g
			src.Pix[i+1] = g
			src.Pix[i+2] = g
			src.Pix[i+3] = 255
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 90}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		t.Skipf("decoded type %T not YCbCr; can't inspect Y plane", img)
	}
	// Left column Y should be much less than right column Y.
	midRow := dim / 2
	leftY := int(ycbcr.Y[midRow*ycbcr.YStride+2])
	rightY := int(ycbcr.Y[midRow*ycbcr.YStride+dim-3])
	t.Logf("gradient: left Y = %d, right Y = %d", leftY, rightY)
	if rightY-leftY < 80 {
		t.Fatalf("gradient not preserved: left=%d right=%d (expected right-left > 80 for 0..255 ramp)",
			leftY, rightY)
	}
}

// TestEncodeDecodeTwoHalvesDistinguishable verifies a two-region image
// decodes as two distinguishable regions. Tests that blocks past the
// first row still reconstruct correctly (DC_PRED uses reconstructed
// neighbors).
func TestEncodeDecodeTwoHalvesDistinguishable(t *testing.T) {
	const dim = 128
	src := image.NewRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := (y*dim + x) * 4
			var v uint8 = 50
			if x >= dim/2 {
				v = 200
			}
			src.Pix[i+0] = v
			src.Pix[i+1] = v
			src.Pix[i+2] = v
			src.Pix[i+3] = 255
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 90}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		t.Skipf("decoded type %T not YCbCr", img)
	}
	midRow := dim / 2
	// Sample deep inside each half to avoid the boundary block.
	darkY := int(ycbcr.Y[midRow*ycbcr.YStride+10])
	brightY := int(ycbcr.Y[midRow*ycbcr.YStride+dim-10])
	t.Logf("two halves: dark Y = %d, bright Y = %d", darkY, brightY)
	if brightY-darkY < 100 {
		t.Fatalf("halves not distinguishable: dark=%d bright=%d (want diff > 100)",
			darkY, brightY)
	}
}

// TestEncodeDecodeHBD10Bit verifies that an image.NRGBA64 input is
// encoded as a 10-bit AVIF (high_bitdepth=1) and round-trips back
// through Decode as an image.RGBA64 with preserved gradient.
func TestEncodeDecodeHBD10Bit(t *testing.T) {
	const dim = 64
	src := image.NewNRGBA64(image.Rect(0, 0, dim, dim))
	// 10-bit gradient embedded in 16-bit storage (6-bit left-shifted).
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := (y*dim + x) * 8
			g10 := uint16(x * 1023 / (dim - 1))
			g16 := g10 << 6
			src.Pix[i+0] = uint8(g16 >> 8)
			src.Pix[i+1] = uint8(g16 & 0xFF)
			src.Pix[i+2] = uint8(g16 >> 8)
			src.Pix[i+3] = uint8(g16 & 0xFF)
			src.Pix[i+4] = uint8(g16 >> 8)
			src.Pix[i+5] = uint8(g16 & 0xFF)
			src.Pix[i+6] = 0xFF
			src.Pix[i+7] = 0xFF
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 90}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	cfg, err := DecodeConfig(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("DecodeConfig: %v", err)
	}
	t.Logf("decoded config: %dx%d color model %T", cfg.Width, cfg.Height, cfg.ColorModel)
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	rgba, ok := img.(*image.RGBA64)
	if !ok {
		t.Fatalf("expected *image.RGBA64 (HBD), got %T", img)
	}
	if rgba.Rect.Dx() != dim || rgba.Rect.Dy() != dim {
		t.Fatalf("decoded size %v, want %dx%d", rgba.Rect, dim, dim)
	}
	// Sample gradient endpoints.
	midRow := dim / 2
	leftIdx := (midRow*dim + 2) * 8
	rightIdx := (midRow*dim + dim - 3) * 8
	leftR := (uint16(rgba.Pix[leftIdx])<<8 | uint16(rgba.Pix[leftIdx+1])) >> 8
	rightR := (uint16(rgba.Pix[rightIdx])<<8 | uint16(rgba.Pix[rightIdx+1])) >> 8
	t.Logf("10-bit gradient: left R=%d right R=%d", leftR, rightR)
	if int(rightR)-int(leftR) < 100 {
		t.Fatalf("HBD gradient not preserved: left=%d right=%d", leftR, rightR)
	}
}

// TestEncodeDecodeOddDimensions verifies that non-multiple-of-64
// dimensions are auto-padded by the encoder (edge-extended) and
// round-trip back to the caller-visible size. ispe carries the
// original dimensions so Decode crops the coded frame back on the
// way out.
func TestEncodeDecodeOddDimensions(t *testing.T) {
	for _, dim := range []int{100, 133, 200} {
		t.Run(dimName(dim), func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, dim, dim))
			for y := 0; y < dim; y++ {
				for x := 0; x < dim; x++ {
					i := (y*dim + x) * 4
					src.Pix[i+0] = uint8(x * 255 / (dim - 1))
					src.Pix[i+1] = uint8(y * 255 / (dim - 1))
					src.Pix[i+2] = 128
					src.Pix[i+3] = 255
				}
			}
			var buf bytes.Buffer
			if err := Encode(&buf, src, &Options{Quality: 80}); err != nil {
				t.Fatalf("Encode: %v", err)
			}
			img, err := Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("Decode: %v", err)
			}
			if img.Bounds().Dx() != dim || img.Bounds().Dy() != dim {
				t.Fatalf("%dx%d: decoded %v, want %dx%d", dim, dim, img.Bounds(), dim, dim)
			}
		})
	}
}

// TestEncodeDecodeChromaSubsampling verifies that 4:2:0, 4:2:2, and
// 4:4:4 chroma layouts all encode and round-trip through Decode.
// Each mode uses a different profile (0, 2, 1) so this also
// exercises the profile-2 non-12-bit path.
func TestEncodeDecodeChromaSubsampling(t *testing.T) {
	cases := []struct {
		name string
		sub  ChromaSubsampling
	}{
		{"420", Chroma420},
		{"422", Chroma422},
		{"444", Chroma444},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			const dim = 64
			src := image.NewRGBA(image.Rect(0, 0, dim, dim))
			for y := 0; y < dim; y++ {
				for x := 0; x < dim; x++ {
					i := (y*dim + x) * 4
					src.Pix[i+0] = uint8(x * 255 / (dim - 1))
					src.Pix[i+1] = 128
					src.Pix[i+2] = uint8(y * 255 / (dim - 1))
					src.Pix[i+3] = 255
				}
			}
			var buf bytes.Buffer
			if err := Encode(&buf, src, &Options{Quality: 90, ChromaSubsampling: c.sub}); err != nil {
				t.Fatalf("%s Encode: %v", c.name, err)
			}
			img, err := Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("%s Decode: %v", c.name, err)
			}
			if img.Bounds().Dx() != dim || img.Bounds().Dy() != dim {
				t.Fatalf("%s decoded %v, want %dx%d", c.name, img.Bounds(), dim, dim)
			}
			t.Logf("%s: %d bytes → %T", c.name, buf.Len(), img)
		})
	}
}

// TestEncodeGridDecodeRoundTrip verifies EncodeGrid → Decode round-
// trips a simple 2×2 grid. Each tile is a different solid color so
// we can assert the composited image shows the expected spatial
// layout.
func TestEncodeGridDecodeRoundTrip(t *testing.T) {
	const tw, th = 64, 64
	shades := []uint8{50, 100, 150, 200}
	tiles := make([]image.Image, 4)
	for i, s := range shades {
		img := image.NewRGBA(image.Rect(0, 0, tw, th))
		for y := 0; y < th; y++ {
			for x := 0; x < tw; x++ {
				idx := (y*tw + x) * 4
				img.Pix[idx+0] = s
				img.Pix[idx+1] = s
				img.Pix[idx+2] = s
				img.Pix[idx+3] = 255
			}
		}
		tiles[i] = img
	}
	var buf bytes.Buffer
	if err := EncodeGrid(&buf, tiles, 2, 2, tw*2, th*2, &Options{Quality: 90}); err != nil {
		t.Fatalf("EncodeGrid: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if img.Bounds().Dx() != tw*2 || img.Bounds().Dy() != th*2 {
		t.Fatalf("grid size %v, want %dx%d", img.Bounds(), tw*2, th*2)
	}
	// Sample center of each tile quadrant. Tile 0 is top-left (shade 50),
	// tile 1 top-right (100), tile 2 bottom-left (150), tile 3 bottom-
	// right (200).
	pickY := func(x, y int) int {
		r, g, b, _ := img.At(x, y).RGBA()
		// Luma ≈ (66R + 129G + 25B + 128) >> 8 for BT.601, but we
		// simply average the three channels since the tiles are
		// grey.
		return (int(r) + int(g) + int(b)) / 3 >> 8
	}
	tl := pickY(tw/2, th/2)
	tr := pickY(tw+tw/2, th/2)
	bl := pickY(tw/2, th+th/2)
	br := pickY(tw+tw/2, th+th/2)
	t.Logf("grid corners: TL=%d TR=%d BL=%d BR=%d (src 50/100/150/200)", tl, tr, bl, br)
	if !(tl < tr && tr < bl && bl < br) {
		t.Fatalf("grid layout not preserved: TL=%d TR=%d BL=%d BR=%d", tl, tr, bl, br)
	}
}

// TestRotate90CCW verifies the rotate helper reorients an image.
func TestRotate90CCW(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	// Fill so row 0 is red, row 1 is green.
	for x := 0; x < 4; x++ {
		src.Set(x, 0, color.RGBA{R: 255, A: 255})
		src.Set(x, 1, color.RGBA{G: 255, A: 255})
	}
	out := rotate90CCW(src)
	if out.Bounds().Dx() != 2 || out.Bounds().Dy() != 4 {
		t.Fatalf("bounds %v, want 2x4", out.Bounds())
	}
	// 90° CCW: the top edge of src becomes the left edge of dst.
	// Source row 0 (red) → dst column 0; row 1 (green) → dst column 1.
	r0, _, _, _ := out.At(0, 0).RGBA()
	if r0>>8 != 255 {
		t.Errorf("col 0 should be red, got R=%d", r0>>8)
	}
	_, g1, _, _ := out.At(1, 0).RGBA()
	if g1>>8 != 255 {
		t.Errorf("col 1 should be green, got G=%d", g1>>8)
	}
}

// TestMirror verifies mirror flips the image on the requested axis.
func TestMirror(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 4, 2))
	for x := 0; x < 4; x++ {
		// Fill with column-based red gradient.
		src.Set(x, 0, color.RGBA{R: uint8(x * 64), A: 255})
		src.Set(x, 1, color.RGBA{R: uint8(x * 64), A: 255})
	}
	// Horizontal mirror: col 0 ↔ col 3.
	hm := mirror(src, true)
	c0R, _, _, _ := hm.At(0, 0).RGBA()
	c3R, _, _, _ := hm.At(3, 0).RGBA()
	if c0R>>8 != 192 {
		t.Errorf("horizontal mirror col 0 R=%d, want 192", c0R>>8)
	}
	if c3R>>8 != 0 {
		t.Errorf("horizontal mirror col 3 R=%d, want 0", c3R>>8)
	}
	// Vertical mirror: row 0 ↔ row 1. Columns unchanged, content
	// moves top↔bottom. Here both rows are identical so only
	// checking bounds is meaningful.
	vm := mirror(src, false)
	if vm.Bounds() != src.Bounds() {
		t.Errorf("vertical mirror bounds %v, want %v", vm.Bounds(), src.Bounds())
	}
}

// TestEncodeYCbCrDirect verifies that an *image.YCbCr input bypasses
// the RGB round-trip and preserves luma values nearly exactly when
// the requested subsampling matches the source. The decoded Y plane
// should stay close to the source Y values across a recognizable
// gradient.
func TestEncodeYCbCrDirect(t *testing.T) {
	const dim = 64
	ycbcr := image.NewYCbCr(image.Rect(0, 0, dim, dim), image.YCbCrSubsampleRatio420)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			ycbcr.Y[y*ycbcr.YStride+x] = uint8(16 + x*219/(dim-1))
		}
	}
	cw, ch := dim/2, dim/2
	for y := 0; y < ch; y++ {
		for x := 0; x < cw; x++ {
			ycbcr.Cb[y*ycbcr.CStride+x] = 128
			ycbcr.Cr[y*ycbcr.CStride+x] = 128
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, ycbcr, &Options{Quality: 95}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	out, ok := img.(*image.YCbCr)
	if !ok {
		t.Fatalf("expected YCbCr, got %T", img)
	}
	// Check gradient survived.
	row := dim / 2
	leftY := out.Y[row*out.YStride+2]
	rightY := out.Y[row*out.YStride+dim-3]
	t.Logf("YCbCr direct: source Y 16..235, decoded %d..%d", leftY, rightY)
	if int(rightY)-int(leftY) < 150 {
		t.Fatalf("YCbCr Y plane not preserved: left=%d right=%d", leftY, rightY)
	}
}

// TestEncodeDecodeHBDChromaSubsampling covers the 10-bit × non-4:2:0
// matrix now that the HBD decoder supports generalized chroma
// layouts via ConvertPlanar16.
func TestEncodeDecodeHBDChromaSubsampling(t *testing.T) {
	cases := []struct {
		name string
		sub  ChromaSubsampling
	}{
		{"420", Chroma420},
		{"422", Chroma422},
		{"444", Chroma444},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			const dim = 64
			src := image.NewNRGBA64(image.Rect(0, 0, dim, dim))
			for y := 0; y < dim; y++ {
				for x := 0; x < dim; x++ {
					i := (y*dim + x) * 8
					v := uint16(x * 1023 / (dim - 1))
					v16 := v << 6
					src.Pix[i+0] = uint8(v16 >> 8)
					src.Pix[i+1] = uint8(v16 & 0xFF)
					src.Pix[i+2] = uint8(v16 >> 8)
					src.Pix[i+3] = uint8(v16 & 0xFF)
					src.Pix[i+4] = uint8(v16 >> 8)
					src.Pix[i+5] = uint8(v16 & 0xFF)
					src.Pix[i+6] = 0xFF
					src.Pix[i+7] = 0xFF
				}
			}
			var buf bytes.Buffer
			if err := Encode(&buf, src, &Options{Quality: 90, ChromaSubsampling: c.sub}); err != nil {
				t.Fatalf("%s Encode: %v", c.name, err)
			}
			img, err := Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("%s Decode: %v", c.name, err)
			}
			if img.Bounds().Dx() != dim || img.Bounds().Dy() != dim {
				t.Fatalf("%s decoded %v, want %dx%d", c.name, img.Bounds(), dim, dim)
			}
			if _, ok := img.(*image.RGBA64); !ok {
				t.Fatalf("%s: expected RGBA64, got %T", c.name, img)
			}
			t.Logf("%s: %d bytes", c.name, buf.Len())
		})
	}
}

// TestEncodeDecodeLowQualityCoeffContext verifies the encoder honors
// the qCtx derived from base_q_index (spec §7.12.4) across the full
// quality range. Low quality (quality=10 → baseQ≈230, qCtx=3) uses a
// different coefficient CDF bucket than high quality; a mismatch
// would desynchronize the decoder's range coder.
func TestEncodeDecodeLowQualityCoeffContext(t *testing.T) {
	const dim = 64
	for _, q := range []int{10, 25, 50, 75, 95} {
		t.Run(dimName(q), func(t *testing.T) {
			src := image.NewRGBA(image.Rect(0, 0, dim, dim))
			for y := 0; y < dim; y++ {
				for x := 0; x < dim; x++ {
					i := (y*dim + x) * 4
					src.Pix[i+0] = uint8(x * 255 / (dim - 1))
					src.Pix[i+1] = uint8(y * 255 / (dim - 1))
					src.Pix[i+2] = 128
					src.Pix[i+3] = 255
				}
			}
			var buf bytes.Buffer
			if err := Encode(&buf, src, &Options{Quality: q}); err != nil {
				t.Fatalf("Encode(q=%d): %v", q, err)
			}
			img, err := Decode(bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("Decode(q=%d): %v", q, err)
			}
			if img.Bounds().Dx() != dim || img.Bounds().Dy() != dim {
				t.Fatalf("q=%d: wrong decoded size %v", q, img.Bounds())
			}
		})
	}
}

// TestEncodeDecodeHBD12Bit exercises the profile-2 (12-bit) encoder
// path. Samples above the 10-bit cap must survive round-trip so the
// profile switch is observable — BitDepth=12 is forced via Options.
func TestEncodeDecodeHBD12Bit(t *testing.T) {
	const dim = 64
	src := image.NewNRGBA64(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := (y*dim + x) * 8
			// 12-bit gradient stored in the 16-bit pix buffer.
			g12 := uint16(x * 4095 / (dim - 1))
			g16 := g12 << 4
			src.Pix[i+0] = uint8(g16 >> 8)
			src.Pix[i+1] = uint8(g16 & 0xFF)
			src.Pix[i+2] = uint8(g16 >> 8)
			src.Pix[i+3] = uint8(g16 & 0xFF)
			src.Pix[i+4] = uint8(g16 >> 8)
			src.Pix[i+5] = uint8(g16 & 0xFF)
			src.Pix[i+6] = 0xFF
			src.Pix[i+7] = 0xFF
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 90, BitDepth: 12}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	rgba, ok := img.(*image.RGBA64)
	if !ok {
		t.Fatalf("expected *image.RGBA64 (HBD), got %T", img)
	}
	midRow := dim / 2
	leftIdx := (midRow*dim + 2) * 8
	rightIdx := (midRow*dim + dim - 3) * 8
	leftR := (uint16(rgba.Pix[leftIdx])<<8 | uint16(rgba.Pix[leftIdx+1])) >> 8
	rightR := (uint16(rgba.Pix[rightIdx])<<8 | uint16(rgba.Pix[rightIdx+1])) >> 8
	t.Logf("12-bit gradient: left R=%d right R=%d", leftR, rightR)
	if int(rightR)-int(leftR) < 100 {
		t.Fatalf("12-bit HBD gradient not preserved: left=%d right=%d", leftR, rightR)
	}
}

// TestEncodeDecodeGrayscale verifies that an image.Gray input is
// encoded as a monochrome AVIF (1 channel in pixi) and round-trips
// back through Decode as image.Gray. No chroma bitstream is emitted,
// so the container should be noticeably smaller than the color path.
func TestEncodeDecodeGrayscale(t *testing.T) {
	const dim = 64
	src := image.NewGray(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			// Horizontal gradient 0..255.
			src.Pix[y*src.Stride+x] = uint8(x * 255 / (dim - 1))
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 90}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	gray, ok := img.(*image.Gray)
	if !ok {
		t.Fatalf("expected *image.Gray output, got %T", img)
	}
	if gray.Rect.Dx() != dim || gray.Rect.Dy() != dim {
		t.Fatalf("decoded size %v, want %dx%d", gray.Rect, dim, dim)
	}
	midRow := dim / 2
	leftY := int(gray.Pix[midRow*gray.Stride+2])
	rightY := int(gray.Pix[midRow*gray.Stride+dim-3])
	t.Logf("grayscale gradient: left=%d right=%d", leftY, rightY)
	if rightY-leftY < 100 {
		t.Fatalf("grayscale gradient not preserved: left=%d right=%d", leftY, rightY)
	}
}

// TestEncodeDecodeComplexTextureBenefitsFromSubSplit encodes an
// image with dense random-ish texture. At 32×32 partition granularity
// the block is too large to fit the local variation; highDetail32
// routes it to PARTITION_SPLIT → 16×16 leaves where each sub-block
// has a more uniform signal and quantization preserves more detail.
// We assert the decoded variance approximates the source's.
func TestEncodeDecodeComplexTextureBenefitsFromSubSplit(t *testing.T) {
	const dim = 64
	src := image.NewRGBA(image.Rect(0, 0, dim, dim))
	// Deterministic pseudo-random texture (linear congruential).
	state := uint32(0x12345678)
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			state = state*1664525 + 1013904223
			v := uint8(state >> 24)
			i := (y*dim + x) * 4
			src.Pix[i+0] = v
			src.Pix[i+1] = v
			src.Pix[i+2] = v
			src.Pix[i+3] = 255
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 90}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		t.Skipf("type %T", img)
	}
	// Compute variance of decoded Y across the whole frame. For
	// a random 0..255 source we expect variance close to 255²/12 ≈ 5418.
	n := dim * dim
	sum, sumSq := 0, 0
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			v := int(ycbcr.Y[y*ycbcr.YStride+x])
			sum += v
			sumSq += v * v
		}
	}
	mean := sum / n
	variance := sumSq/n - mean*mean
	t.Logf("texture: variance=%d (source variance ~5400)", variance)
	if variance < 1500 {
		t.Fatalf("texture variance collapsed: %d", variance)
	}
}

// TestEncodeDecodeHighQualityPreservesDetail verifies that at very
// high quality (low quantizer) we get a precise reproduction. This
// exercises the Golomb-rice coefficient tail, since at baseQ ~ 5 the
// quantized coefficients routinely exceed the base+BR saturation cap
// of 15.
func TestEncodeDecodeHighQualityPreservesDetail(t *testing.T) {
	const dim = 64
	src := image.NewRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := (y*dim + x) * 4
			// Sharp step function in luma.
			var v uint8 = 30
			if x >= dim/2 {
				v = 220
			}
			src.Pix[i+0] = v
			src.Pix[i+1] = v
			src.Pix[i+2] = v
			src.Pix[i+3] = 255
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 98}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		t.Skipf("type %T", img)
	}
	// Well inside each half (avoid boundary blur).
	midRow := dim / 2
	leftY := int(ycbcr.Y[midRow*ycbcr.YStride+4])
	rightY := int(ycbcr.Y[midRow*ycbcr.YStride+dim-5])
	t.Logf("high-Q step: left=%d right=%d (source y ~44 / ~209)", leftY, rightY)
	// Without the Golomb tail, large quantized coefs would saturate
	// and the step would flatten. With the tail, the diff should be
	// very close to the source.
	if rightY-leftY < 140 {
		t.Fatalf("high-Q step contrast insufficient: left=%d right=%d", leftY, rightY)
	}
}

// TestEncodeDecodeVerticalBarsBenefitFromVPred encodes an image of
// thick vertical bars. VPred copies the row above down, which is a
// near-perfect predictor for vertical bars once the first row lands —
// so blocks after the top row should need almost no residual coding.
// We assert the decoded output preserves the bar boundaries sharply.
func TestEncodeDecodeVerticalBarsBenefitFromVPred(t *testing.T) {
	const dim = 64
	src := image.NewRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := (y*dim + x) * 4
			var v uint8 = 50
			if (x/16)%2 == 1 {
				v = 200
			}
			src.Pix[i+0] = v
			src.Pix[i+1] = v
			src.Pix[i+2] = v
			src.Pix[i+3] = 255
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 90}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		t.Skipf("type %T", img)
	}
	// Sample in the middle of each bar. Bar 0 (x<16) should be dark,
	// bar 1 (16..31) bright, bar 2 (32..47) dark, bar 3 (48..63) bright.
	row := dim / 2
	darkA := int(ycbcr.Y[row*ycbcr.YStride+8])
	brightA := int(ycbcr.Y[row*ycbcr.YStride+24])
	darkB := int(ycbcr.Y[row*ycbcr.YStride+40])
	brightB := int(ycbcr.Y[row*ycbcr.YStride+56])
	t.Logf("bars: dark=%d/%d bright=%d/%d", darkA, darkB, brightA, brightB)
	// At quality=90 with PARTITION_SPLIT → 32×32 blocks and no inter-
	// partition RDO, the bar contrast is preserved but attenuated;
	// assert direction only. Tightening this threshold should wait for
	// finer partitions or a better quantization mapping.
	if brightA <= darkA || brightB <= darkB {
		t.Fatalf("bar contrast direction lost: darks=%d/%d brights=%d/%d",
			darkA, darkB, brightA, brightB)
	}
}

// TestEncodeDecodeCheckerboardPreservesHighFreq encodes a fine
// checkerboard and verifies that after decode the pattern still
// contains the high-frequency alternation. Before PARTITION_SPLIT,
// a 64×64 SB would use TX_64×64 with clamped scan (dropping
// frequencies outside the top-left 32×32) — a high-frequency
// checkerboard would be blurred into a uniform grey.
func TestEncodeDecodeCheckerboardPreservesHighFreq(t *testing.T) {
	const dim = 64
	src := image.NewRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := (y*dim + x) * 4
			var v uint8
			if (x+y)%2 == 0 {
				v = 255
			} else {
				v = 0
			}
			src.Pix[i+0] = v
			src.Pix[i+1] = v
			src.Pix[i+2] = v
			src.Pix[i+3] = 255
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 95}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	ycbcr, ok := img.(*image.YCbCr)
	if !ok {
		t.Skipf("decoded type %T not YCbCr", img)
	}
	// Measure luma variance across a row. Constant output → variance
	// ≈ 0 (old behavior). Preserved checkerboard → high variance.
	row := dim / 2
	var sum, sumSq int
	n := dim
	for c := 0; c < dim; c++ {
		v := int(ycbcr.Y[row*ycbcr.YStride+c])
		sum += v
		sumSq += v * v
	}
	mean := sum / n
	variance := sumSq/n - mean*mean
	t.Logf("checkerboard row variance = %d (mean=%d)", variance, mean)
	if variance < 1000 {
		t.Fatalf("checkerboard flattened to variance %d (mean=%d) — high-freq content lost", variance, mean)
	}
}

// TestEncodeDecodeWithAlphaRoundTrip encodes an RGBA image with a
// gradient alpha channel and verifies the decoder reads back an NRGBA
// with matching alpha values (within quantization tolerance).
func TestEncodeDecodeWithAlphaRoundTrip(t *testing.T) {
	const dim = 64
	src := image.NewNRGBA(image.Rect(0, 0, dim, dim))
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := (y*dim + x) * 4
			src.Pix[i+0] = 200
			src.Pix[i+1] = 100
			src.Pix[i+2] = 50
			// Horizontal alpha gradient 0..255.
			src.Pix[i+3] = uint8(x * 255 / (dim - 1))
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, &Options{Quality: 90, Alpha: true}); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	nrgba, ok := img.(*image.NRGBA)
	if !ok {
		t.Fatalf("expected *image.NRGBA output, got %T", img)
	}
	if nrgba.Rect.Dx() != dim || nrgba.Rect.Dy() != dim {
		t.Fatalf("decoded size %v, want %dx%d", nrgba.Rect, dim, dim)
	}
	// Sample alpha at a few positions. With lossy coding we allow ~50
	// ulps; what matters is that the gradient direction is preserved.
	midRow := dim / 2
	leftA := nrgba.Pix[(midRow*dim+2)*4+3]
	rightA := nrgba.Pix[(midRow*dim+dim-3)*4+3]
	t.Logf("alpha gradient: left=%d, right=%d", leftA, rightA)
	if int(rightA)-int(leftA) < 100 {
		t.Fatalf("alpha gradient not preserved: left=%d right=%d", leftA, rightA)
	}
}

// TestEncodeDetectsNonOpaqueAlpha verifies the encoder auto-enables
// the alpha item when the input contains non-opaque pixels, even
// without opts.Alpha set.
func TestEncodeDetectsNonOpaqueAlpha(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	// First half opaque, second half translucent.
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			i := (y*64 + x) * 4
			src.Pix[i+0] = 128
			src.Pix[i+1] = 128
			src.Pix[i+2] = 128
			if x < 32 {
				src.Pix[i+3] = 255
			} else {
				src.Pix[i+3] = 64
			}
		}
	}
	var buf bytes.Buffer
	if err := Encode(&buf, src, nil); err != nil {
		t.Fatalf("Encode: %v", err)
	}
	img, err := Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if _, ok := img.(*image.NRGBA); !ok {
		t.Fatalf("expected *image.NRGBA (alpha present), got %T", img)
	}
}

func dimName(n int) string {
	return "x" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
