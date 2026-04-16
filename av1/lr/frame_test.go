package lr

import "testing"

func TestApplyFrameNoneIsIdentity(t *testing.T) {
	w, h := 16, 16
	pix := make([]uint8, w*h)
	for i := range pix {
		pix[i] = uint8(i & 0xFF)
	}
	orig := append([]uint8(nil), pix...)
	p := Plane{Pix: pix, Stride: w, Width: w, Height: h}
	ApplyFrame(p, 8, func(x, y int) UnitParams { return UnitParams{Filter: FilterNone} })
	for i, v := range pix {
		if v != orig[i] {
			t.Errorf("FilterNone changed pixel %d: %d → %d", i, orig[i], v)
		}
	}
}

func TestApplyFrameWienerIdentityPreserves(t *testing.T) {
	w, h := 16, 16
	pix := make([]uint8, w*h)
	for i := range pix {
		pix[i] = uint8((i * 17) & 0xFF)
	}
	orig := append([]uint8(nil), pix...)
	p := Plane{Pix: pix, Stride: w, Width: w, Height: h}
	ApplyFrame(p, 8, func(x, y int) UnitParams {
		return UnitParams{
			Filter:      FilterWiener,
			WienerHoriz: WienerTaps{0, 0, 0, 128},
			WienerVert:  WienerTaps{0, 0, 0, 128},
		}
	})
	for i, v := range pix {
		if v != orig[i] {
			t.Errorf("identity Wiener changed pixel %d: %d → %d", i, orig[i], v)
		}
	}
}

func TestApplyFrameSGRZeroRadiiIsIdentity(t *testing.T) {
	w, h := 16, 16
	pix := make([]uint8, w*h)
	for i := range pix {
		pix[i] = uint8((i * 41) & 0xFF)
	}
	orig := append([]uint8(nil), pix...)
	p := Plane{Pix: pix, Stride: w, Width: w, Height: h}
	ApplyFrame(p, 8, func(x, y int) UnitParams {
		return UnitParams{Filter: FilterSGR, SGR: SGRParams{R0: 0, R1: 0}}
	})
	for i, v := range pix {
		if v != orig[i] {
			t.Errorf("zero-radii SGR changed pixel %d: %d → %d", i, orig[i], v)
		}
	}
}
