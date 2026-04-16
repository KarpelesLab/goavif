package goavif

import (
	"image"
	"testing"

	"github.com/KarpelesLab/goavif/av1/decoder"
	"github.com/KarpelesLab/goavif/av1/obu"
)

// TestFrameToImage16ProducesRGBA64 drives frameToImage on a synthetic
// 10-bit 4:2:0 frame and checks the output type + basic sample
// presence. The decoder here is standalone (no real bitstream), so we
// hand-assemble the Frame struct.
func TestFrameToImage16ProducesRGBA64(t *testing.T) {
	const W, H = 16, 16
	y16 := make([]uint16, W*H)
	u16 := make([]uint16, (W/2)*(H/2))
	v16 := make([]uint16, (W/2)*(H/2))
	// Mid-gray at 10-bit: Y=0.5*1023≈512, U=V=512 (neutral chroma).
	for i := range y16 {
		y16[i] = 512
	}
	for i := range u16 {
		u16[i] = 512
		v16[i] = 512
	}
	seq := &obu.SequenceHeader{}
	seq.Color.BitDepth = 10
	seq.Color.SubsamplingX = 1
	seq.Color.SubsamplingY = 1
	seq.Color.MatrixCoefficients = uint8(1) // BT.709
	seq.Color.ColorRange = true             // full range
	f := &decoder.Frame{
		Width:    W,
		Height:   H,
		BitDepth: 10,
		Y16:      y16,
		U16:      u16,
		V16:      v16,
		YStride:  W,
		CStride:  W / 2,
		Seq:      seq,
	}
	f.Subsampling.X = 1
	f.Subsampling.Y = 1
	img, err := frameToImage(f)
	if err != nil {
		t.Fatalf("frameToImage: %v", err)
	}
	rgba, ok := img.(*image.RGBA64)
	if !ok {
		t.Fatalf("expected *image.RGBA64, got %T", img)
	}
	if rgba.Bounds().Dx() != W || rgba.Bounds().Dy() != H {
		t.Fatalf("size mismatch: got %v want %dx%d", rgba.Bounds(), W, H)
	}
	// Spot-check: mid-gray → channels around 32768, alpha 65535.
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			c := rgba.RGBA64At(x, y)
			for _, ch := range []uint16{c.R, c.G, c.B} {
				if ch < 30000 || ch > 35000 {
					t.Fatalf("(%d,%d) channel = %d, want ~32768", x, y, ch)
				}
			}
			if c.A != 0xFFFF {
				t.Fatalf("(%d,%d) alpha = %d, want 65535", x, y, c.A)
			}
		}
	}
}

func TestFrameToImage16MonochromeReturnsGray16(t *testing.T) {
	const W, H = 8, 8
	y16 := make([]uint16, W*H)
	for i := range y16 {
		y16[i] = 256 // mid-low gray at 10-bit
	}
	seq := &obu.SequenceHeader{}
	seq.Color.BitDepth = 10
	seq.Color.Monochrome = true
	f := &decoder.Frame{
		Width:      W,
		Height:     H,
		BitDepth:   10,
		Monochrome: true,
		Y16:        y16,
		YStride:    W,
		Seq:        seq,
	}
	img, err := frameToImage(f)
	if err != nil {
		t.Fatalf("frameToImage: %v", err)
	}
	if _, ok := img.(*image.Gray16); !ok {
		t.Fatalf("expected *image.Gray16, got %T", img)
	}
}
