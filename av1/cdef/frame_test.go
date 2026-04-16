package cdef

import "testing"

func TestApplyFrameNoopWhenStrengthsZero(t *testing.T) {
	pix := make([]uint8, 32*32)
	for i := range pix {
		pix[i] = uint8(i)
	}
	orig := append([]uint8(nil), pix...)
	ApplyFrame(Plane{Pix: pix, Stride: 32, Width: 32, Height: 32}, 0, 0, 3)
	for i := range pix {
		if pix[i] != orig[i] {
			t.Fatalf("zero-strength changed sample %d: %d -> %d", i, orig[i], pix[i])
		}
	}
}

func TestApplyFrameModifiesPixels(t *testing.T) {
	// Gentle gradient: CDEF smooths small differences, so the deltas
	// must fall inside the constrain threshold. Large contrast values
	// saturate the nonlinearity and produce zero output.
	pix := make([]uint8, 32*32)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			base := byte(128)
			if (x^y)&1 == 0 {
				base = 130
			}
			pix[y*32+x] = base
		}
	}
	orig := append([]uint8(nil), pix...)
	ApplyFrame(Plane{Pix: pix, Stride: 32, Width: 32, Height: 32}, 16, 8, 3)
	changed := 0
	for i := range pix {
		if pix[i] != orig[i] {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("CDEF didn't touch any pixels on a gentle-contrast pattern")
	}
}

func TestApplyFramePerSBNilFnIsNoop(t *testing.T) {
	pix := make([]uint8, 8*8)
	orig := append([]uint8(nil), pix...)
	ApplyFramePerSB(Plane{Pix: pix, Stride: 8, Width: 8, Height: 8}, nil, 3)
	for i := range pix {
		if pix[i] != orig[i] {
			t.Fatalf("nil StrengthFn changed pixel %d", i)
		}
	}
}

func TestApplyFramePerSBZeroStrengthSkipsBlock(t *testing.T) {
	pix := make([]uint8, 16*16)
	for i := range pix {
		pix[i] = 120
		if i%2 == 0 {
			pix[i] = 140
		}
	}
	orig := append([]uint8(nil), pix...)
	fn := func(x, y int) (int, int) {
		// Only activate the top-left 8×8.
		if x == 0 && y == 0 {
			return 4, 4
		}
		return 0, 0
	}
	ApplyFramePerSB(Plane{Pix: pix, Stride: 16, Width: 16, Height: 16}, fn, 3)
	// The non-(0,0) blocks must match orig.
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if x < 8 && y < 8 {
				continue
			}
			i := y*16 + x
			if pix[i] != orig[i] {
				t.Fatalf("pixel (%d,%d) in zero-strength block was touched", x, y)
			}
		}
	}
}

func TestApplyFramePerSBUsesPerBlockStrength(t *testing.T) {
	pix := make([]uint8, 16*16)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			base := byte(128)
			if (x^y)&1 == 0 {
				base = 131
			}
			pix[y*16+x] = base
		}
	}
	orig := append([]uint8(nil), pix...)
	fn := func(x, y int) (int, int) {
		// Activate every 8×8 block with non-zero strength.
		return 16, 8
	}
	ApplyFramePerSB(Plane{Pix: pix, Stride: 16, Width: 16, Height: 16}, fn, 3)
	different := 0
	for i := range pix {
		if pix[i] != orig[i] {
			different++
		}
	}
	if different == 0 {
		t.Fatal("per-SB CDEF produced no pixel changes")
	}
}
