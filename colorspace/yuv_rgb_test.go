package colorspace

import "testing"

func TestIdentityMCMapsChannels(t *testing.T) {
	r, g, b := YUVToRGB8(10, 20, 30, MCIdentity, Full)
	// Identity: Y=G, U=B, V=R
	if r != 30 || g != 10 || b != 20 {
		t.Errorf("identity MC: got (%d,%d,%d), want (30,10,20)", r, g, b)
	}
}

func TestBT709FullRangeGray(t *testing.T) {
	// Full-range mid-gray: Y=128, U=V=128 → RGB ~ (128,128,128).
	r, g, b := YUVToRGB8(128, 128, 128, MCBT709, Full)
	if absI(int(r)-128) > 1 || absI(int(g)-128) > 1 || absI(int(b)-128) > 1 {
		t.Errorf("gray conversion off: (%d,%d,%d) want ~(128,128,128)", r, g, b)
	}
}

func TestBT709StudioRangeBlackWhite(t *testing.T) {
	// Studio-range black: Y=16, U=V=128 → ~(0,0,0)
	r, g, b := YUVToRGB8(16, 128, 128, MCBT709, Studio)
	if r > 1 || g > 1 || b > 1 {
		t.Errorf("studio black: (%d,%d,%d), want ~(0,0,0)", r, g, b)
	}
	// Studio-range white: Y=235 → ~(255,255,255)
	r, g, b = YUVToRGB8(235, 128, 128, MCBT709, Studio)
	if r < 254 || g < 254 || b < 254 {
		t.Errorf("studio white: (%d,%d,%d), want ~(255,255,255)", r, g, b)
	}
}

func TestBT601RedRange(t *testing.T) {
	// Full-range "pure red" in Y'CbCr under BT.601 is approximately
	// Y=76, Cb=85, Cr=255 (from the forward matrix). Round-trip to RGB
	// should give ~(255,0,0).
	r, g, b := YUVToRGB8(76, 85, 255, MCBT601, Full)
	if r < 245 {
		t.Errorf("red: R=%d want >=245", r)
	}
	if g > 10 || b > 10 {
		t.Errorf("red: G=%d B=%d want near zero", g, b)
	}
}

func TestConvertPlanar420Smoke(t *testing.T) {
	w, h := 4, 4
	yP := make([]uint8, w*h)
	uP := make([]uint8, (w/2)*(h/2))
	vP := make([]uint8, (w/2)*(h/2))
	for i := range yP {
		yP[i] = 128
	}
	for i := range uP {
		uP[i] = 128
		vP[i] = 128
	}
	dst := make([]uint8, w*h*4)
	ConvertPlanar420(dst, yP, uP, vP, w, h, MCBT709, Full)
	for i := 0; i < w*h; i++ {
		r, g, b, a := dst[i*4], dst[i*4+1], dst[i*4+2], dst[i*4+3]
		if absI(int(r)-128) > 1 || absI(int(g)-128) > 1 || absI(int(b)-128) > 1 {
			t.Errorf("pixel %d: (%d,%d,%d) want ~gray", i, r, g, b)
		}
		if a != 255 {
			t.Errorf("pixel %d: alpha=%d want 255", i, a)
		}
	}
}

func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
