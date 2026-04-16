package filmgrain

import (
	"testing"
)

func TestRNGDeterministic(t *testing.T) {
	a := NewRNG(0x1234)
	b := NewRNG(0x1234)
	for i := 0; i < 100; i++ {
		if a.Next() != b.Next() {
			t.Fatalf("rng diverged at step %d", i)
		}
	}
}

func TestRNGAdvances(t *testing.T) {
	r := NewRNG(0xACE1)
	prev := r.State()
	moved := false
	for i := 0; i < 16; i++ {
		v := r.Next()
		if v != prev {
			moved = true
		}
		prev = v
	}
	if !moved {
		t.Fatal("rng state never changed over 16 steps")
	}
}

func TestRNGByteInRange(t *testing.T) {
	r := NewRNG(0x4321)
	for i := 0; i < 1000; i++ {
		b := r.Byte()
		if b < -128 || b > 127 {
			t.Fatalf("byte out of signed range: %d", b)
		}
	}
}

func TestBuildLUTClampsBelowFirstPoint(t *testing.T) {
	lut := BuildLUT([]Point{
		{Value: 32, Scale: 40},
		{Value: 200, Scale: 10},
	})
	for i := 0; i < 32; i++ {
		if lut[i] != 40 {
			t.Fatalf("below-first-point @ %d want 40 got %d", i, lut[i])
		}
	}
}

func TestBuildLUTClampsAboveLastPoint(t *testing.T) {
	lut := BuildLUT([]Point{
		{Value: 32, Scale: 40},
		{Value: 200, Scale: 10},
	})
	for i := 201; i < 256; i++ {
		if lut[i] != 10 {
			t.Fatalf("above-last-point @ %d want 10 got %d", i, lut[i])
		}
	}
}

func TestBuildLUTInterpolates(t *testing.T) {
	lut := BuildLUT([]Point{
		{Value: 0, Scale: 0},
		{Value: 200, Scale: 200},
	})
	// At the midpoint, the LUT should be ~100.
	if lut[100] < 95 || lut[100] > 105 {
		t.Fatalf("midpoint interp want ~100 got %d", lut[100])
	}
	// Endpoints exact.
	if lut[0] != 0 || lut[200] != 200 {
		t.Fatalf("endpoints wrong: lut[0]=%d lut[200]=%d", lut[0], lut[200])
	}
}

func TestBuildLUTEmpty(t *testing.T) {
	lut := BuildLUT(nil)
	for i := 0; i < 256; i++ {
		if lut[i] != 0 {
			t.Fatalf("empty LUT non-zero at %d", i)
		}
	}
}

func TestApplyGrainZeroSeedIsNoop(t *testing.T) {
	plane := make([]uint8, 16*16)
	for i := range plane {
		plane[i] = 128
	}
	p := &Params{GrainSeed: 0}
	ApplyGrainPlane(plane, 16, 16, 16, &ScalingLUT{}, p)
	for i, v := range plane {
		if v != 128 {
			t.Fatalf("zero-seed modified pixel %d: %d", i, v)
		}
	}
}

func TestApplyGrainChangesPixels(t *testing.T) {
	plane := make([]uint8, 16*16)
	for i := range plane {
		plane[i] = 128
	}
	// Scale 255 everywhere means grain is fully applied.
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 255
	}
	p := &Params{GrainSeed: 0x1234, ScalingShift: 8}
	ApplyGrainPlane(plane, 16, 16, 16, &lut, p)
	changed := 0
	for _, v := range plane {
		if v != 128 {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("grain produced no pixel changes")
	}
}

func TestApplyGrainRespectsZeroScale(t *testing.T) {
	plane := make([]uint8, 8*8)
	for i := range plane {
		plane[i] = 100
	}
	// Zero scale everywhere means no perturbation applied.
	p := &Params{GrainSeed: 0xDEAD, ScalingShift: 8}
	ApplyGrainPlane(plane, 8, 8, 8, &ScalingLUT{}, p)
	for i, v := range plane {
		if v != 100 {
			t.Fatalf("zero-scale modified pixel %d: %d", i, v)
		}
	}
}

func TestApplyGrainClampsRestrictedRange(t *testing.T) {
	plane := make([]uint8, 32*32)
	for i := range plane {
		plane[i] = 235 // at the high clip boundary
	}
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 255
	}
	p := &Params{GrainSeed: 0xBEEF, ScalingShift: 8, ClipToRestrictedRange: true}
	ApplyGrainPlane(plane, 32, 32, 32, &lut, p)
	for i, v := range plane {
		if v > 235 || v < 16 {
			t.Fatalf("pixel %d escaped restricted range: %d", i, v)
		}
	}
}

func TestApplyFullFrame(t *testing.T) {
	const w, h = 32, 32
	y := make([]uint8, w*h)
	u := make([]uint8, (w/2)*(h/2))
	v := make([]uint8, (w/2)*(h/2))
	for i := range y {
		y[i] = 128
	}
	for i := range u {
		u[i] = 128
		v[i] = 128
	}
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 128
	}
	p := &Params{
		GrainSeed:    0xF00D,
		ScalingY:     lut,
		ScalingU:     lut,
		ScalingV:     lut,
		ScalingShift: 8,
	}
	Apply(y, w, h, w, u, v, w/2, h/2, w/2, p)
	// Just assert we touched at least some pixels on each plane.
	count := func(buf []uint8, base uint8) int {
		n := 0
		for _, p := range buf {
			if p != base {
				n++
			}
		}
		return n
	}
	if count(y, 128) == 0 {
		t.Fatal("luma unchanged")
	}
	if count(u, 128) == 0 {
		t.Fatal("U unchanged")
	}
	if count(v, 128) == 0 {
		t.Fatal("V unchanged")
	}
}
