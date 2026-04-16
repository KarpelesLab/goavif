package predict

import "testing"

func TestDirectionalPred16VerticalMatchesAbove(t *testing.T) {
	above := make([]uint16, 16)
	for i := range above {
		above[i] = uint16(i * 100)
	}
	left := make([]uint16, 16)
	dst := make([]uint16, 4*4)
	DirectionalPred16(dst, 4, 4, above, left, 90, 10)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != above[c] {
				t.Fatalf("D90 (%d,%d) got %d want %d", r, c, dst[r*4+c], above[c])
			}
		}
	}
}

func TestDirectionalPred16HorizontalMatchesLeft(t *testing.T) {
	above := make([]uint16, 16)
	left := make([]uint16, 16)
	for i := range left {
		left[i] = uint16(1000 + i*50)
	}
	dst := make([]uint16, 4*4)
	DirectionalPred16(dst, 4, 4, above, left, 180, 10)
	for r := 0; r < 4; r++ {
		for c := 0; c < 4; c++ {
			if dst[r*4+c] != left[r] {
				t.Fatalf("D180 (%d,%d) got %d want %d", r, c, dst[r*4+c], left[r])
			}
		}
	}
}

func TestDirectionalPred16ClipsToBitDepth(t *testing.T) {
	above := make([]uint16, 32)
	left := make([]uint16, 32)
	for i := range above {
		above[i] = 4095 // full-range 12-bit sample
		left[i] = 4095
	}
	dst := make([]uint16, 8*8)
	DirectionalPred16(dst, 8, 8, above, left, 67, 12)
	maxV := uint16((1 << 12) - 1)
	for i, v := range dst {
		if v > maxV {
			t.Fatalf("dst[%d]=%d exceeds 12-bit range", i, v)
		}
	}
}

func TestDirectionalPred16ConstantInputConstantOutput(t *testing.T) {
	const K = 900 // stays within 10-bit range (< 1024)
	above := make([]uint16, 32)
	left := make([]uint16, 32)
	for i := range above {
		above[i] = K
		left[i] = K
	}
	// Sweep all six base angles.
	for _, angle := range []int{45, 67, 113, 135, 157, 203} {
		dst := make([]uint16, 8*8)
		DirectionalPred16(dst, 8, 8, above, left, angle, 10)
		for i, v := range dst {
			if v != K {
				t.Fatalf("angle %d: non-constant output at %d: %d", angle, i, v)
			}
		}
	}
}
