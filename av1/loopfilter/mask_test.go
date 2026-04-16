package loopfilter

import "testing"

func TestDeriveThresholdsZeroLevel(t *testing.T) {
	// filter_level = 0 → limit clamps to 1, blimit = 2*(0+2)+1 = 5, thresh = 0
	th := DeriveThresholds(0, 0)
	if th.Limit != 1 {
		t.Errorf("limit=%d want 1", th.Limit)
	}
	if th.Blimit != 5 {
		t.Errorf("blimit=%d want 5", th.Blimit)
	}
	if th.Thresh != 0 {
		t.Errorf("thresh=%d want 0", th.Thresh)
	}
}

func TestDeriveThresholdsHighLevel(t *testing.T) {
	// filter_level = 63, sharpness = 0
	// limit = 63 >> 1 = 31, blimit = 2*65 + 31 = 161, thresh = 63 >> 4 = 3
	th := DeriveThresholds(63, 0)
	if th.Limit != 31 {
		t.Errorf("limit=%d want 31", th.Limit)
	}
	if th.Blimit != 161 {
		t.Errorf("blimit=%d want 161", th.Blimit)
	}
	if th.Thresh != 3 {
		t.Errorf("thresh=%d want 3", th.Thresh)
	}
}

func TestDeriveThresholdsSharpnessCaps(t *testing.T) {
	// With sharpness=7: shift = 2, so limit = 63 >> 2 = 15
	// Cap = 63 / (7+1) = 7, so limit clamps to 7
	th := DeriveThresholds(63, 7)
	if th.Limit != 7 {
		t.Errorf("limit=%d want 7 (clamped by sharpness cap)", th.Limit)
	}
}

func TestDeriveThresholdsClamps(t *testing.T) {
	th := DeriveThresholds(200, 99)
	// filter_level clamped to 63, sharpness to 7. limit = 15 → cap 7.
	if th.Limit != 7 {
		t.Errorf("clamped limit=%d want 7", th.Limit)
	}
}
