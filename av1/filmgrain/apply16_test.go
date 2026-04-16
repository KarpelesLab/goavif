package filmgrain

import "testing"

func TestApplyWithTemplate16ClipsTo10Bit(t *testing.T) {
	const W, H = 64, 64
	plane := make([]uint16, W*H)
	for i := range plane {
		plane[i] = 1020
	}
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 255
	}
	tpl := NewLumaTemplate(0x4321, 0, nil, 7)
	p := &Params{GrainSeed: 0xBEEF, ScalingShift: 8}
	ApplyWithTemplate16(plane, W, H, W, &lut, &tpl, p, 10)
	for i, v := range plane {
		if v > 1023 {
			t.Fatalf("sample %d = %d exceeds 10-bit max", i, v)
		}
	}
}

func TestApplyWithTemplate16RestrictedRange10Bit(t *testing.T) {
	const W, H = 64, 64
	plane := make([]uint16, W*H)
	for i := range plane {
		plane[i] = 950
	}
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 255
	}
	tpl := NewLumaTemplate(0x0101, 0, nil, 7)
	p := &Params{GrainSeed: 0xCAFE, ScalingShift: 8, ClipToRestrictedRange: true}
	ApplyWithTemplate16(plane, W, H, W, &lut, &tpl, p, 10)
	// 10-bit restricted range: [16<<2, 235<<2] = [64, 940].
	for i, v := range plane {
		if v < 64 || v > 940 {
			t.Fatalf("sample %d = %d escaped restricted range [64, 940]", i, v)
		}
	}
}

func TestApplyWithTemplate16ZeroSeedNoop(t *testing.T) {
	plane := make([]uint16, 32*32)
	for i := range plane {
		plane[i] = 2000
	}
	tpl := NewLumaTemplate(0x9999, 0, nil, 7)
	p := &Params{GrainSeed: 0}
	ApplyWithTemplate16(plane, 32, 32, 32, &ScalingLUT{}, &tpl, p, 12)
	for _, v := range plane {
		if v != 2000 {
			t.Fatal("zero-seed modified a pixel")
		}
	}
}

func TestApplyWithTemplate16ProducesChanges12Bit(t *testing.T) {
	const W, H = 64, 64
	plane := make([]uint16, W*H)
	for i := range plane {
		plane[i] = 2048
	}
	var lut ScalingLUT
	for i := range lut {
		lut[i] = 200
	}
	tpl := NewLumaTemplate(0x8765, 0, nil, 7)
	p := &Params{GrainSeed: 0xABCD, ScalingShift: 8}
	ApplyWithTemplate16(plane, W, H, W, &lut, &tpl, p, 12)
	changed := 0
	for _, v := range plane {
		if v != 2048 {
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("12-bit film grain produced no changes")
	}
}
