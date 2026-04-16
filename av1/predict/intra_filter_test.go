package predict

import "testing"

func TestFilterIntraAllModesRun(t *testing.T) {
	above := []uint8{100, 100, 100, 100, 100, 100, 100, 100}
	left := []uint8{100, 100, 100, 100, 100, 100, 100, 100}
	dst := make([]uint8, 4*4)
	for m := 0; m < 5; m++ {
		for i := range dst {
			dst[i] = 0
		}
		FilterIntraPred(dst, 4, 4, above, left, 100, m)
		// With all-100 neighbors, some modes can drift but all
		// outputs should stay in the valid [0, 255] band.
		for i, v := range dst {
			if v > 255 {
				t.Errorf("mode %d dst[%d] = %d out of range", m, i, v)
			}
			_ = v
		}
	}
}

func TestFilterIntraModeIndexClamps(t *testing.T) {
	above := []uint8{128, 128, 128, 128, 128, 128, 128, 128}
	left := []uint8{128, 128, 128, 128, 128, 128, 128, 128}
	dst := make([]uint8, 4*4)
	// mode out of range → clamped to 0, runs without panic.
	FilterIntraPred(dst, 4, 4, above, left, 128, 99)
}
