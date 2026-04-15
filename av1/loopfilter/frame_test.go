package loopfilter

import "testing"

func TestUniformGrid(t *testing.T) {
	g := UniformGrid(16, 16, 4, 4)
	wantX := []int{4, 8, 12}
	wantY := []int{4, 8, 12}
	if len(g.EdgeXs) != 3 || len(g.EdgeYs) != 3 {
		t.Fatalf("got %d edges X / %d edges Y", len(g.EdgeXs), len(g.EdgeYs))
	}
	for i := range wantX {
		if g.EdgeXs[i] != wantX[i] {
			t.Errorf("EdgeXs[%d]=%d want %d", i, g.EdgeXs[i], wantX[i])
		}
		if g.EdgeYs[i] != wantY[i] {
			t.Errorf("EdgeYs[%d]=%d want %d", i, g.EdgeYs[i], wantY[i])
		}
	}
}

func TestApplyFrameNarrowSoftensCheckerboard(t *testing.T) {
	// Build an 8x8 image with a sharp seam at column 4: left half = 110,
	// right half = 120. After ApplyFrameNarrow the inner samples around
	// the seam should have moved together.
	w, h := 8, 8
	pix := make([]uint8, w*h)
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if c < 4 {
				pix[r*w+c] = 110
			} else {
				pix[r*w+c] = 120
			}
		}
	}
	p := Plane{Pix: pix, Stride: w, Width: w, Height: h}
	grid := UniformGrid(w, h, 4, 8) // single vertical edge at x=4
	ApplyFrameNarrow(p, grid, Thresholds{Limit: 30, Blimit: 10, Thresh: 8})
	// Verify columns 3 and 4 moved.
	if pix[3] <= 110 {
		t.Errorf("col 3 not lifted: %d", pix[3])
	}
	if pix[4] >= 120 {
		t.Errorf("col 4 not lowered: %d", pix[4])
	}
}
