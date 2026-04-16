package lr

import "testing"

// Identity taps sum to 128 with all weight at the center.
var identityTaps = WienerTaps{0, 0, 0, 128}

func TestWienerIdentityPreservesInput(t *testing.T) {
	w, h := 8, 8
	src := make([]uint8, w*h)
	for i := range src {
		src[i] = uint8((i * 17) & 0xFF)
	}
	dst := make([]uint8, w*h)
	ApplyWiener(dst, src, w, h, w, identityTaps, identityTaps)
	for i := range src {
		if dst[i] != src[i] {
			t.Errorf("identity Wiener changed sample %d: %d → %d", i, src[i], dst[i])
		}
	}
}

func TestWienerIdentityConstant(t *testing.T) {
	w, h := 8, 8
	src := make([]uint8, w*h)
	for i := range src {
		src[i] = 123
	}
	dst := make([]uint8, w*h)
	ApplyWiener(dst, src, w, h, w, identityTaps, identityTaps)
	for i, v := range dst {
		if v != 123 {
			t.Errorf("identity Wiener on flat: dst[%d]=%d want 123", i, v)
		}
	}
}

func TestWienerSmoothesNoise(t *testing.T) {
	w, h := 8, 8
	src := make([]uint8, w*h)
	// A checkerboard 40/160 pattern — high-frequency. A low-pass
	// Wiener should average values toward the midpoint.
	for r := 0; r < h; r++ {
		for c := 0; c < w; c++ {
			if (r+c)%2 == 0 {
				src[r*w+c] = 40
			} else {
				src[r*w+c] = 160
			}
		}
	}
	// A smooth low-pass: taps {−1, 2, 5, 116, 5, 2, −1} all scaled to
	// sum ≈ 128.
	taps := WienerTaps{-1, 2, 5, 116}
	dst := make([]uint8, w*h)
	ApplyWiener(dst, src, w, h, w, taps, taps)
	// Interior samples should be between the original extremes.
	for r := 2; r < h-2; r++ {
		for c := 2; c < w-2; c++ {
			v := dst[r*w+c]
			if v < 20 || v > 180 {
				t.Errorf("dst[%d,%d]=%d out of expected smoothed band", r, c, v)
			}
		}
	}
}
