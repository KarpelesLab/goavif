package decoder

import "testing"

func TestIntraModeIsDirectional(t *testing.T) {
	cases := map[IntraMode]bool{
		DCPred:      false,
		VPred:       false,
		HPred:       false,
		D45Pred:     true,
		D135Pred:    true,
		D113Pred:    true,
		D157Pred:    true,
		D203Pred:    true,
		D67Pred:     true,
		SmoothPred:  false,
		SmoothVPred: false,
		SmoothHPred: false,
		PaethPred:   false,
	}
	for m, want := range cases {
		if got := m.IsDirectional(); got != want {
			t.Errorf("%s.IsDirectional() = %v, want %v", m, got, want)
		}
	}
}

func TestMIWidthHeight(t *testing.T) {
	cases := []struct {
		bs           BlockSize
		mw, mh       int
	}{
		{Block4x4, 1, 1},
		{Block8x8, 2, 2},
		{Block16x16, 4, 4},
		{Block64x64, 16, 16},
		{Block16x32, 4, 8},
		{Block128x128, 32, 32},
	}
	for _, c := range cases {
		if c.bs.MIWidth() != c.mw || c.bs.MIHeight() != c.mh {
			t.Errorf("%v MI=(%d,%d), want (%d,%d)", c.bs, c.bs.MIWidth(), c.bs.MIHeight(), c.mw, c.mh)
		}
	}
}

func TestSubsampledMIDimsClamp(t *testing.T) {
	// 4x4 block in 4:2:0 chroma yields 1x1 MI (clamped from 0x0).
	mw, mh := Block4x4.SubsampledMIDims(1, 1)
	if mw != 1 || mh != 1 {
		t.Errorf("SubsampledMIDims(4x4, 1, 1) = (%d,%d), want (1,1)", mw, mh)
	}
}

func TestMaxTXSize(t *testing.T) {
	w, h := MaxTXSize(Block128x128)
	if w != 64 || h != 64 {
		t.Errorf("MaxTXSize(128x128) = (%d,%d), want (64,64)", w, h)
	}
	w, h = MaxTXSize(Block16x32)
	if w != 16 || h != 32 {
		t.Errorf("MaxTXSize(16x32) = (%d,%d), want (16,32)", w, h)
	}
}
