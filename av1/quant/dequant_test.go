package quant

import "testing"

func TestDCLookupExists(t *testing.T) {
	// Spot-check known values from spec Table 7.12.2: index 0 is 4, index 1 is 8.
	if DC8[0] != 4 {
		t.Errorf("DC8[0] = %d, want 4", DC8[0])
	}
	if DC8[1] != 8 {
		t.Errorf("DC8[1] = %d, want 8", DC8[1])
	}
	if AC8[0] != 4 {
		t.Errorf("AC8[0] = %d, want 4", AC8[0])
	}
	// DC tables are monotonically non-decreasing.
	prev := uint16(0)
	for i, v := range DC8 {
		if v < prev {
			t.Errorf("DC8 not monotonic at %d: %d < %d", i, v, prev)
		}
		prev = v
	}
}

func TestComputeYClipsNegative(t *testing.T) {
	p := Params{BaseQIndex: 5, DeltaQYDc: -100, BitDepth: 8}
	v := p.Compute(PlaneY)
	// Clipped q -> DC8[0] = 4
	if v.DC != 4 {
		t.Errorf("negative-delta DC=%d want 4", v.DC)
	}
	// AC uses BaseQIndex=5 → AC8[5] = 12
	if v.AC != AC8[5] {
		t.Errorf("AC=%d want %d", v.AC, AC8[5])
	}
}

func TestComputeCrUsesVDeltas(t *testing.T) {
	p := Params{BaseQIndex: 50, DeltaQVDc: 10, DeltaQVAc: -5, BitDepth: 8}
	v := p.Compute(PlaneV)
	if v.DC != DC8[60] {
		t.Errorf("V DC=%d want %d", v.DC, DC8[60])
	}
	if v.AC != AC8[45] {
		t.Errorf("V AC=%d want %d", v.AC, AC8[45])
	}
}

func TestUnsupportedBitDepth(t *testing.T) {
	p := Params{BaseQIndex: 100, BitDepth: 10}
	v := p.Compute(PlaneY)
	if v.DC != 0 || v.AC != 0 {
		t.Errorf("10-bit should return zero until tables are added: got %+v", v)
	}
}
