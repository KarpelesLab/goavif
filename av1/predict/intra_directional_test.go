package predict

import "testing"

func TestD45PredDiagonalShift(t *testing.T) {
	// aboveExtended = [A, B, C, D, E, F, G]; D45 yields above[r+c+1].
	above := []uint8{10, 20, 30, 40, 50, 60, 70}
	dst := make([]uint8, 4*4)
	D45Pred(dst, 4, 4, above)
	// pred[r, c] = above[r+c+1]
	want := [][]uint8{
		{20, 30, 40, 50},
		{30, 40, 50, 60},
		{40, 50, 60, 70},
		{50, 60, 70, 70}, // last column saturates to above[6]=70
	}
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != want[r][c] {
				t.Errorf("D45[%d,%d]=%d want %d", r, c, dst[r*4+c], want[r][c])
			}
		}
	}
}

func TestD135PredCoreDiagonal(t *testing.T) {
	above := []uint8{100, 100, 100, 100}
	left := []uint8{200, 200, 200, 200}
	dst := make([]uint8, 4*4)
	D135Pred(dst, 4, 4, above, left, 150)
	// diagonal (r==c) = aboveLeft = 150
	for i := 0; i < 4; i++ {
		if dst[i*4+i] != 150 {
			t.Errorf("D135 diagonal[%d,%d]=%d want 150", i, i, dst[i*4+i])
		}
	}
	// r > c should pull from left[r-c-1] = 200
	if dst[1*4+0] != 200 {
		t.Errorf("D135[1,0]=%d want 200", dst[1*4+0])
	}
	// r < c should pull from above[c-r-1] = 100
	if dst[0*4+1] != 100 {
		t.Errorf("D135[0,1]=%d want 100", dst[0*4+1])
	}
}
