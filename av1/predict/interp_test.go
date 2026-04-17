package predict

import "testing"

// TestInterpSubPelZeroPhasePassesInputThrough verifies that at
// integer-pel phase (hp=vp=0), the interpolator returns the integer
// samples at the block's top-left corner. The zero-phase filter
// has a single tap of 128 at index 3, so the result is identical
// (within rounding) to the input.
func TestInterpSubPelZeroPhasePassesInputThrough(t *testing.T) {
	// Reference region: 15×15 (for an 8×8 output block with 7 samples
	// of extension on each axis). Fill with a horizontal gradient.
	const srcW = 15
	src := make([]uint8, srcW*srcW)
	for r := 0; r < srcW; r++ {
		for c := 0; c < srcW; c++ {
			src[r*srcW+c] = uint8(c * 16) // 0, 16, 32, ...
		}
	}
	dst := make([]uint8, 8*8)
	// At hp=vp=0, the phase-0 filter has its single weight of 128
	// at tap index 3, so dst[r][c] = src[r+3][c+3].
	InterpSubPel(dst, 8, 8, src, srcW, 0, 0, InterpRegular)
	for r := 0; r < 8; r++ {
		for c := 0; c < 8; c++ {
			got := dst[r*8+c]
			want := src[(r+3)*srcW+(c+3)]
			if got != want {
				t.Fatalf("dst[%d,%d] = %d, want src[%d,%d] = %d",
					r, c, got, r+3, c+3, want)
			}
		}
	}
}

// TestInterpSubPelHorizontalShift checks that a half-pixel
// horizontal shift (hp=8) produces a luma value halfway between
// the two neighboring integer-pel samples.
func TestInterpSubPelHorizontalShift(t *testing.T) {
	const srcW = 15
	src := make([]uint8, srcW*srcW)
	for r := 0; r < srcW; r++ {
		for c := 0; c < srcW; c++ {
			src[r*srcW+c] = uint8(c * 16)
		}
	}
	dst := make([]uint8, 4*4)
	InterpSubPel(dst, 4, 4, src, srcW, 8, 0, InterpRegular)
	// Around r=0, c=0 the integer samples are src[3][3]=48, src[3][4]=64.
	// Phase 8 (exact mid) should produce ~56 after filtering and
	// rounding. The 8-tap regular filter integrates more neighbors
	// so the exact value differs slightly, but it should sit
	// between the two integers.
	v := dst[0]
	if v < 40 || v > 72 {
		t.Fatalf("half-pel horizontal dst[0,0] = %d, expected roughly 48..64", v)
	}
}

// TestInterpInteger verifies the integer-pel copy fast path.
func TestInterpInteger(t *testing.T) {
	src := make([]uint8, 10*10)
	for i := range src {
		src[i] = uint8(i)
	}
	dst := make([]uint8, 4*4)
	InterpInteger(dst, 4, 4, src, 10)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != src[r*10+c] {
				t.Fatalf("dst[%d,%d] = %d, want %d", r, c, dst[r*4+c], src[r*10+c])
			}
		}
	}
}
