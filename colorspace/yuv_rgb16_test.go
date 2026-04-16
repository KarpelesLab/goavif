package colorspace

import "testing"

func TestYUVToRGB16BlackTenBit(t *testing.T) {
	// Full range black: y=0, u=512, v=512 → R=G=B=0.
	r, g, b := YUVToRGB16(0, 512, 512, MCBT709, Full, 10)
	if r > 1 || g > 1 || b > 1 {
		t.Fatalf("full-range black got RGB(%d,%d,%d)", r, g, b)
	}
}

func TestYUVToRGB16WhiteTenBit(t *testing.T) {
	// Full range white: y=1023, u=512, v=512 → R=G=B=65535.
	r, g, b := YUVToRGB16(1023, 512, 512, MCBT709, Full, 10)
	// Allow a couple of units of rounding slack.
	if r < 65520 || g < 65520 || b < 65520 {
		t.Fatalf("full-range white got RGB(%d,%d,%d)", r, g, b)
	}
}

func TestYUVToRGB16LimitedRangeBlack(t *testing.T) {
	// Studio range black: y=64 (16<<2), u=512, v=512 → RGB ≈ 0.
	r, g, b := YUVToRGB16(64, 512, 512, MCBT709, Studio, 10)
	if r > 200 || g > 200 || b > 200 {
		t.Fatalf("studio-range black got RGB(%d,%d,%d) — should be near 0", r, g, b)
	}
}

func TestYUVToRGB16LimitedRangeWhite(t *testing.T) {
	// Studio range white: y=940 (235<<2), u=512, v=512 → RGB ≈ 65535.
	r, g, b := YUVToRGB16(940, 512, 512, MCBT709, Studio, 10)
	if r < 65300 || g < 65300 || b < 65300 {
		t.Fatalf("studio-range white got RGB(%d,%d,%d) — should be near 65535", r, g, b)
	}
}

func TestYUVToRGB16TwelveBit(t *testing.T) {
	// 12-bit full range mid-gray: y=2048, u=2048, v=2048 → RGB ≈ 32768.
	r, g, b := YUVToRGB16(2048, 2048, 2048, MCBT709, Full, 12)
	for _, c := range []uint16{r, g, b} {
		if c < 32000 || c > 33500 {
			t.Fatalf("12-bit mid-gray channel = %d, want ≈ 32768", c)
		}
	}
}

func TestYUVToRGB16IdentityPermutes(t *testing.T) {
	// MC_IDENTITY: G = Y, B = U, R = V (with bit-depth scaling).
	r, g, b := YUVToRGB16(100, 200, 300, MCIdentity, Full, 10)
	wantR := uint16(uint32(300) * 65535 / 1023)
	wantG := uint16(uint32(100) * 65535 / 1023)
	wantB := uint16(uint32(200) * 65535 / 1023)
	if r != wantR || g != wantG || b != wantB {
		t.Fatalf("identity permute got (%d,%d,%d) want (%d,%d,%d)", r, g, b, wantR, wantG, wantB)
	}
}

func TestConvertPlanar420_16ProducesRGBA64Layout(t *testing.T) {
	const W, H = 4, 4
	ySrc := make([]uint16, W*H)
	uSrc := make([]uint16, (W/2)*(H/2))
	vSrc := make([]uint16, (W/2)*(H/2))
	for i := range ySrc {
		ySrc[i] = 512
	}
	for i := range uSrc {
		uSrc[i] = 512
		vSrc[i] = 512
	}
	dst := make([]uint8, W*H*8)
	ConvertPlanar420_16(dst, ySrc, uSrc, vSrc, W, H, MCBT709, Full, 10)
	// Every pixel should be mid-gray ≈ 32767. Alpha == 0xFFFF.
	for i := 0; i < W*H; i++ {
		offset := i * 8
		a := (uint16(dst[offset+6]) << 8) | uint16(dst[offset+7])
		if a != 0xFFFF {
			t.Fatalf("alpha wrong at %d: %d", i, a)
		}
	}
}
