package lr

import "testing"

func TestApplyWiener16IdentityTaps(t *testing.T) {
	const W, H = 8, 8
	src := make([]uint16, W*H)
	for i := range src {
		src[i] = uint16(i * 10)
	}
	dst := make([]uint16, W*H)
	// Identity: (0,0,0,128) → center tap = 128 / 128 = 1, all others 0.
	ApplyWiener16(dst, src, W, H, W, WienerTaps{0, 0, 0, 128}, WienerTaps{0, 0, 0, 128}, 10)
	for i := range dst {
		if dst[i] != src[i] {
			t.Fatalf("identity Wiener changed sample %d: %d -> %d", i, src[i], dst[i])
		}
	}
}

func TestApplyWiener16ClipsToBitDepth(t *testing.T) {
	const W, H = 16, 16
	src := make([]uint16, W*H)
	for i := range src {
		src[i] = 4090 // near 12-bit max
	}
	dst := make([]uint16, W*H)
	// Deliberately over-unity taps that would overflow: (0,32,0,128) sums
	// to 192 across taps, producing roughly 1.5× the input.
	ApplyWiener16(dst, src, W, H, W, WienerTaps{0, 32, 0, 128}, WienerTaps{0, 32, 0, 128}, 12)
	for i, v := range dst {
		if v > 4095 {
			t.Fatalf("dst[%d]=%d exceeded 12-bit max", i, v)
		}
	}
}

func TestApplySGR16CopyPassThroughAtZeroRadii(t *testing.T) {
	const W, H = 8, 8
	src := make([]uint16, W*H)
	for i := range src {
		src[i] = uint16(i * 5)
	}
	dst := make([]uint16, W*H)
	ApplySGR16(dst, src, W, H, W, SGRParams{}, 10)
	for i := range dst {
		if dst[i] != src[i] {
			t.Fatalf("zero-radii changed sample %d", i)
		}
	}
}

func TestApplySGR16ClipsToBitDepth(t *testing.T) {
	const W, H = 12, 12
	src := make([]uint16, W*H)
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			src[y*W+x] = 1020 + uint16((x+y)&3) // near 10-bit max
		}
	}
	dst := make([]uint16, W*H)
	p := SGRParams{R0: 1, Eps0: 12, Xq: [2]int{8, 0}}
	ApplySGR16(dst, src, W, H, W, p, 10)
	for i, v := range dst {
		if v > 1023 {
			t.Fatalf("dst[%d]=%d exceeded 10-bit max", i, v)
		}
	}
}

func TestApplyFrame16RunsWithoutPanic(t *testing.T) {
	const W, H = 32, 32
	pix := make([]uint16, W*H)
	for i := range pix {
		pix[i] = 512
	}
	p := Plane16{Pix: pix, Stride: W, Width: W, Height: H}
	fn := func(x, y int) UnitParams {
		return UnitParams{
			Filter:      FilterWiener,
			WienerHoriz: WienerTaps{0, 0, 0, 128},
			WienerVert:  WienerTaps{0, 0, 0, 128},
		}
	}
	ApplyFrame16(p, 16, fn, 10)
	for i, v := range pix {
		if v != 512 {
			t.Fatalf("identity ApplyFrame changed %d: %d", i, v)
		}
	}
}
