package cdef

import "testing"

func TestApplyFrame16NoopWhenStrengthsZero(t *testing.T) {
	pix := make([]uint16, 32*32)
	for i := range pix {
		pix[i] = uint16(i * 2)
	}
	orig := append([]uint16(nil), pix...)
	ApplyFrame16(Plane16{Pix: pix, Stride: 32, Width: 32, Height: 32}, 0, 0, 3, 10)
	for i := range pix {
		if pix[i] != orig[i] {
			t.Fatalf("zero-strength modified sample %d", i)
		}
	}
}

func TestApplyFrame16ClipsTo10Bit(t *testing.T) {
	pix := make([]uint16, 32*32)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			pix[y*32+x] = 1020
			if (x^y)&1 == 0 {
				pix[y*32+x] = 1023
			}
		}
	}
	ApplyFrame16(Plane16{Pix: pix, Stride: 32, Width: 32, Height: 32}, 32, 16, 3, 10)
	maxV := uint16(1023)
	for i, v := range pix {
		if v > maxV {
			t.Fatalf("sample %d = %d exceeds 10-bit max", i, v)
		}
	}
}

func TestApplyFrame16ClipsTo12Bit(t *testing.T) {
	pix := make([]uint16, 32*32)
	for y := 0; y < 32; y++ {
		for x := 0; x < 32; x++ {
			pix[y*32+x] = 4092
			if (x^y)&1 == 0 {
				pix[y*32+x] = 4095
			}
		}
	}
	ApplyFrame16(Plane16{Pix: pix, Stride: 32, Width: 32, Height: 32}, 32, 16, 3, 12)
	for i, v := range pix {
		if v > 4095 {
			t.Fatalf("sample %d = %d exceeds 12-bit max", i, v)
		}
	}
}

func TestFindDirection16HorizontalGradient(t *testing.T) {
	// Horizontal stripes: rows of equal value. Should pick dir 2 (horizontal).
	src := make([]uint16, 8*8)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			src[y*8+x] = uint16(500 + y*20) // same within a row
		}
	}
	dir, _ := FindDirection16(src, 8, 0, 0, 10)
	if dir != 2 && dir != 6 {
		t.Fatalf("horizontal gradient picked dir %d (want 2 or 6)", dir)
	}
}

func TestApplyFramePerSB16NilFnIsNoop(t *testing.T) {
	pix := make([]uint16, 16*16)
	for i := range pix {
		pix[i] = 512
	}
	orig := append([]uint16(nil), pix...)
	ApplyFramePerSB16(Plane16{Pix: pix, Stride: 16, Width: 16, Height: 16}, nil, 3, 10)
	for i := range pix {
		if pix[i] != orig[i] {
			t.Fatalf("nil StrengthFn changed pixel %d", i)
		}
	}
}
