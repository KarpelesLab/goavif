package cdef

import "testing"

func TestFindDirectionHorizontalPattern(t *testing.T) {
	// A block with horizontal stripes should score best on a horizontal
	// direction (2).
	stride := 8
	src := make([]uint8, 8*8)
	for r := 0; r < 8; r++ {
		v := uint8(40 + r*30%200)
		for c := 0; c < 8; c++ {
			src[r*stride+c] = v
		}
	}
	dir, _ := FindDirection(src, stride, 0, 0)
	// Direction 2 is horizontal lines; 6 is vertical. We expect
	// horizontal variance to dominate.
	if dir != 2 {
		t.Errorf("horizontal stripes → dir %d, expected 2", dir)
	}
}

func TestFindDirectionVerticalPattern(t *testing.T) {
	stride := 8
	src := make([]uint8, 8*8)
	for c := 0; c < 8; c++ {
		v := uint8(40 + c*30%200)
		for r := 0; r < 8; r++ {
			src[r*stride+c] = v
		}
	}
	dir, _ := FindDirection(src, stride, 0, 0)
	if dir != 6 {
		t.Errorf("vertical stripes → dir %d, expected 6", dir)
	}
}

func TestFindDirectionAntiDiagonalPattern(t *testing.T) {
	// A block where value = r + c (top-right and bottom-left corners are
	// extremes) has its lines of constant value running along the
	// anti-diagonal — direction 0 in CDEF's numbering.
	stride := 8
	src := make([]uint8, 8*8)
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			src[r*stride+c] = uint8((r + c) * 10)
		}
	}
	dir, _ := FindDirection(src, stride, 0, 0)
	if dir != 0 {
		t.Errorf("anti-diagonal gradient (r+c) → dir %d, expected 0", dir)
	}
}

func TestFindDirectionDiagonalPattern(t *testing.T) {
	// A block where value = r - c has lines of constant value running
	// along the top-left → bottom-right diagonal, which is CDEF direction 4.
	stride := 8
	src := make([]uint8, 8*8)
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			src[r*stride+c] = uint8(128 + (r-c)*10)
		}
	}
	dir, _ := FindDirection(src, stride, 0, 0)
	if dir != 4 {
		t.Errorf("diagonal gradient (r-c) → dir %d, expected 4", dir)
	}
}
