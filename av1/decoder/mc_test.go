package decoder

import (
	"testing"

	"github.com/KarpelesLab/goavif/av1/predict"
)

// TestMotionCompensateIntegerPelCopy verifies zero-MV integer-pel
// MC reproduces the reference region exactly.
func TestMotionCompensateIntegerPelCopy(t *testing.T) {
	const w, h = 16, 16
	ref := make([]uint8, 64*64)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			ref[y*64+x] = uint8((x + y*7) & 0xFF)
		}
	}
	dst := make([]uint8, w*h)
	MotionCompensate(dst, w, h, ref, 64, 64, 64, 16, 16, MV{0, 0}, predict.InterpRegular)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			want := ref[(16+r)*64+(16+c)]
			if dst[r*w+c] != want {
				t.Fatalf("dst[%d,%d] = %d, want %d", r, c, dst[r*w+c], want)
			}
		}
	}
}

// TestMotionCompensateShiftedIntegerPel verifies a +2 integer-pel MV
// on both axes fetches from (block_x+2, block_y+2) in the reference.
func TestMotionCompensateShiftedIntegerPel(t *testing.T) {
	const w, h = 8, 8
	ref := make([]uint8, 64*64)
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			ref[y*64+x] = uint8(x)
		}
	}
	dst := make([]uint8, w*h)
	// MV eighth-pel: col=16 = +2 integer pel, row=16 = +2 integer pel.
	MotionCompensate(dst, w, h, ref, 64, 64, 64, 10, 10, MV{Row: 16, Col: 16}, predict.InterpRegular)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			// Block starts at (bx=10, by=10), MV shifts by +2 pixels.
			want := ref[(10+2+r)*64+(10+2+c)]
			if dst[r*w+c] != want {
				t.Fatalf("dst[%d,%d] = %d, want %d", r, c, dst[r*w+c], want)
			}
		}
	}
}

// TestMotionCompensateClampsPastEdge ensures MVs pointing past the
// frame boundary clamp to the edge pixel rather than crashing.
func TestMotionCompensateClampsPastEdge(t *testing.T) {
	const w, h = 8, 8
	ref := make([]uint8, 16*16)
	for i := range ref {
		ref[i] = 0x42
	}
	dst := make([]uint8, w*h)
	// Block at (0, 0), MV = -16 integer pel ≈ points into the
	// off-frame region. Expected: dst filled with 0x42 (edge repeat).
	MotionCompensate(dst, w, h, ref, 16, 16, 16, 0, 0, MV{Row: -128, Col: -128}, predict.InterpRegular)
	for _, v := range dst {
		if v != 0x42 {
			t.Fatalf("edge-clamp failed: got %#x, want 0x42", v)
		}
	}
}
